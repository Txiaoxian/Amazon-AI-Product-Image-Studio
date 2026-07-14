package provideradapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

var (
	errProviderResponseTooLarge = errors.New("provider response exceeded safe size limit")
	errProviderImageTooLarge    = errors.New("provider image exceeded safe size limit")
	errProviderImageFetch       = errors.New("provider image URL could not be fetched")
)

type cappedResponseReader struct {
	reader    io.Reader
	remaining int64
}

func (r *cappedResponseReader) Read(buffer []byte) (int, error) {
	if r.remaining > 0 {
		if int64(len(buffer)) > r.remaining {
			buffer = buffer[:r.remaining]
		}
		count, err := r.reader.Read(buffer)
		r.remaining -= int64(count)
		return count, err
	}
	var probe [1]byte
	count, err := r.reader.Read(probe[:])
	if count > 0 {
		return 0, errProviderResponseTooLarge
	}
	return 0, err
}

func (c *Client) parseOpenAIResponseStream(ctx context.Context, reader io.Reader) (_ []Image, _ map[string]any, resultErr error) {
	limited := &cappedResponseReader{reader: reader, remaining: c.maxResponseBytes}
	decoder := json.NewDecoder(limited)
	start, err := decoder.Token()
	if err != nil {
		return nil, nil, err
	}
	if delimiter, ok := start.(json.Delim); !ok || delimiter != '{' {
		return nil, nil, errors.New("openai response must be an object")
	}

	images := []Image{}
	defer func() {
		if resultErr != nil {
			removeTemporaryImages(images)
		}
	}()
	usage := map[string]any{}
	for decoder.More() {
		fieldToken, err := decoder.Token()
		if err != nil {
			return nil, nil, err
		}
		field, ok := fieldToken.(string)
		if !ok {
			return nil, nil, errors.New("openai response field name is invalid")
		}
		switch field {
		case "data":
			parsed, err := c.decodeOpenAIData(ctx, decoder)
			if err != nil {
				return nil, nil, err
			}
			images = append(images, parsed...)
		case "usage":
			if err := decoder.Decode(&usage); err != nil {
				return nil, nil, err
			}
		default:
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return nil, nil, err
			}
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, nil, err
	}
	if token, err := decoder.Token(); err != io.EOF || token != nil {
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, errors.New("openai response contained trailing data")
	}
	if len(images) == 0 {
		return nil, nil, errors.New("openai response contained no images")
	}
	return images, usage, nil
}

func (c *Client) decodeOpenAIData(ctx context.Context, decoder *json.Decoder) ([]Image, error) {
	start, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := start.(json.Delim); !ok || delimiter != '[' {
		return nil, errors.New("openai response data must be an array")
	}
	images := []Image{}
	for decoder.More() {
		var item openAIImage
		if err := decoder.Decode(&item); err != nil {
			removeTemporaryImages(images)
			return nil, err
		}
		image, err := c.materializeOpenAIImage(ctx, item, len(images))
		if err != nil {
			removeTemporaryImages(images)
			return nil, err
		}
		images = append(images, image)
	}
	if _, err := decoder.Token(); err != nil {
		removeTemporaryImages(images)
		return nil, err
	}
	return images, nil
}

func (c *Client) materializeOpenAIImage(ctx context.Context, item openAIImage, index int) (Image, error) {
	mimeType := strings.TrimSpace(item.MIME)
	if mimeType == "" {
		mimeType = "image/png"
	}
	metadata := map[string]any{"providerOutputIndex": index}
	if encoded := strings.TrimSpace(item.B64JSON); encoded != "" {
		filePath, size, err := c.decodeBase64ImageToTemporaryFile(encoded)
		if err != nil {
			return Image{}, err
		}
		return Image{FilePath: filePath, SizeBytes: size, Temporary: true, MIMEType: mimeType, Metadata: metadata}, nil
	}
	if rawURL := strings.TrimSpace(item.URL); rawURL != "" {
		filePath, size, contentType, err := c.fetchImageURLToTemporaryFile(ctx, rawURL)
		if err != nil {
			return Image{}, fmt.Errorf("%w: %v", errProviderImageFetch, err)
		}
		if contentType != "" {
			mimeType = contentType
		}
		metadata["source"] = "provider_url"
		return Image{FilePath: filePath, SizeBytes: size, Temporary: true, MIMEType: mimeType, Metadata: metadata}, nil
	}
	return Image{}, errors.New("provider image contained no bytes")
}

func (c *Client) decodeBase64ImageToTemporaryFile(encoded string) (string, int64, error) {
	file, err := os.CreateTemp(c.tempDir, "provider-image-*.tmp")
	if err != nil {
		return "", 0, err
	}
	filePath := file.Name()
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(filePath)
		}
	}()

	decoded := base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded))
	size, err := io.Copy(file, io.LimitReader(decoded, c.maxImageBytes+1))
	if err != nil {
		return "", 0, err
	}
	if size > c.maxImageBytes {
		return "", 0, errProviderImageTooLarge
	}
	if size == 0 {
		return "", 0, errors.New("provider image was empty")
	}
	if err := file.Close(); err != nil {
		return "", 0, err
	}
	ok = true
	return filePath, size, nil
}

func (c *Client) fetchImageURLToTemporaryFile(ctx context.Context, rawURL string) (string, int64, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return "", 0, "", err
	}
	request.Header.Set("Accept", "image/*")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", 0, "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", 0, "", fmt.Errorf("provider image URL returned status %d", response.StatusCode)
	}
	if response.ContentLength > c.maxImageBytes {
		return "", 0, "", errProviderImageTooLarge
	}

	file, err := os.CreateTemp(c.tempDir, "provider-image-*.tmp")
	if err != nil {
		return "", 0, "", err
	}
	filePath := file.Name()
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(filePath)
		}
	}()
	size, err := io.Copy(file, io.LimitReader(response.Body, c.maxImageBytes+1))
	if err != nil {
		return "", 0, "", err
	}
	if size > c.maxImageBytes {
		return "", 0, "", errProviderImageTooLarge
	}
	if size == 0 {
		return "", 0, "", errors.New("provider image was empty")
	}
	if err := file.Close(); err != nil {
		return "", 0, "", err
	}
	ok = true
	contentType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	return filePath, size, contentType, nil
}

func removeTemporaryImages(images []Image) {
	for _, image := range images {
		if image.Temporary && strings.TrimSpace(image.FilePath) != "" {
			_ = os.Remove(image.FilePath)
		}
	}
}
