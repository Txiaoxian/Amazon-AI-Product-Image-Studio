package provideradapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
)

const (
	defaultMaxResponseBytes = 128 << 20
)

type ClientOptions struct {
	HTTPClient       *http.Client
	MaxResponseBytes int64
	Now              func() time.Time
}

type Client struct {
	httpClient       *http.Client
	maxResponseBytes int64
	now              func() time.Time
}

func NewClient(options ClientOptions) *Client {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = provider.NewSafeHTTPClient(nil, 0)
	}
	maxResponseBytes := options.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}
	now := options.Now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &Client{httpClient: httpClient, maxResponseBytes: maxResponseBytes, now: now}
}

func (c *Client) Execute(ctx context.Context, req ImageRequest) (ImageResult, error) {
	if c == nil {
		c = NewClient(ClientOptions{})
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if req.Provider.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.Provider.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	switch req.Provider.Type {
	case provider.TypeOpenAI:
		return c.executeOpenAI(ctx, req, false)
	case provider.TypeOpenAICompatible:
		return c.executeOpenAI(ctx, req, true)
	case provider.TypeGemini:
		return c.executeGemini(ctx, req)
	default:
		result := ImageResult{APICall: baseAPICall(req)}
		result.APICall.Status = APICallStatusFailure
		result.APICall.ErrorCode = "PROVIDER_UNSUPPORTED"
		result.APICall.ErrorMessage = "Provider type is not supported."
		return result, ProviderError{Code: result.APICall.ErrorCode, Message: result.APICall.ErrorMessage}
	}
}

func (c *Client) executeOpenAI(ctx context.Context, req ImageRequest, compatible bool) (ImageResult, error) {
	if err := validateRequest(req); err != nil {
		result := ImageResult{APICall: baseAPICall(req)}
		result.APICall.Status = APICallStatusFailure
		result.APICall.ErrorCode = "PROVIDER_REQUEST_INVALID"
		result.APICall.ErrorMessage = "Provider request is invalid."
		return result, err
	}

	endpoint := "/images/generations"
	if req.Operation == OperationEdit {
		endpoint = "/images/edits"
	}
	requestURL, err := appendEndpoint(req.Provider.BaseURL, endpoint)
	if err != nil {
		result := ImageResult{APICall: baseAPICall(req)}
		result.APICall.Status = APICallStatusFailure
		result.APICall.ErrorCode = "PROVIDER_URL_INVALID"
		result.APICall.ErrorMessage = "Provider base URL is invalid."
		return result, err
	}

	var body io.Reader
	contentType := "application/json"
	if req.Operation == OperationEdit {
		var buffer bytes.Buffer
		writer := multipart.NewWriter(&buffer)
		_ = writer.WriteField("model", req.Model.ModelName)
		_ = writer.WriteField("prompt", req.Prompt)
		writeOpenAIParameterFields(writer, req, compatible)
		for index, image := range req.InputImages {
			fieldName := "image"
			if index > 0 {
				fieldName = "image[]"
			}
			part, err := writer.CreateFormFile(fieldName, inputFilename(image, index))
			if err != nil {
				return ImageResult{}, err
			}
			if _, err := part.Write(image.Data); err != nil {
				return ImageResult{}, err
			}
		}
		if err := writer.Close(); err != nil {
			return ImageResult{}, err
		}
		body = &buffer
		contentType = writer.FormDataContentType()
	} else {
		payload := openAIJSONPayload(req, compatible)
		encoded, err := json.Marshal(payload)
		if err != nil {
			return ImageResult{}, err
		}
		body = bytes.NewReader(encoded)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, body)
	if err != nil {
		return ImageResult{}, err
	}
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Content-Type", contentType)
	httpRequest.Header.Set("Authorization", "Bearer "+req.Provider.APIKey)

	call := baseAPICall(req)
	call.RequestMetadata["endpointPath"] = endpoint
	call.RequestMetadata["compatible"] = compatible
	startedAt := c.now()
	response, err := c.httpClient.Do(httpRequest)
	call.DurationMs = c.now().Sub(startedAt).Milliseconds()
	if err != nil {
		call.Status = APICallStatusFailure
		call.ErrorCode = "PROVIDER_TRANSPORT_ERROR"
		call.ErrorMessage = SanitizeErrorMessage(err.Error())
		return ImageResult{APICall: call}, ProviderError{Code: call.ErrorCode, Message: call.ErrorMessage, Retryable: true}
	}
	defer response.Body.Close()
	call.HTTPStatus = statusCodePointer(response.StatusCode)
	call.RequestID = providerRequestID(response.Header)

	data, readErr := readLimited(response.Body, c.maxResponseBytes)
	if readErr != nil {
		call.Status = APICallStatusFailure
		call.ErrorCode = "PROVIDER_RESPONSE_TOO_LARGE"
		call.ErrorMessage = "Provider response could not be read safely."
		return ImageResult{APICall: call}, ProviderError{Code: call.ErrorCode, Message: call.ErrorMessage, HTTPStatus: call.HTTPStatus, Retryable: true}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		providerErr := providerHTTPError(response.StatusCode, data)
		call.Status = APICallStatusFailure
		call.ErrorCode = providerErr.Code
		call.ErrorMessage = providerErr.Message
		return ImageResult{APICall: call}, providerErr
	}

	parsed, err := parseOpenAIResponse(data)
	if err != nil {
		call.Status = APICallStatusFailure
		call.ErrorCode = "PROVIDER_RESPONSE_INVALID"
		call.ErrorMessage = "Provider response could not be normalized."
		return ImageResult{APICall: call}, ProviderError{Code: call.ErrorCode, Message: call.ErrorMessage, HTTPStatus: call.HTTPStatus}
	}
	images, err := c.normalizeOpenAIImages(ctx, parsed)
	if err != nil {
		call.Status = APICallStatusFailure
		call.ErrorCode = "PROVIDER_IMAGE_FETCH_FAILED"
		call.ErrorMessage = SanitizeErrorMessage(err.Error())
		return ImageResult{APICall: call}, ProviderError{Code: call.ErrorCode, Message: call.ErrorMessage, HTTPStatus: call.HTTPStatus, Retryable: true}
	}
	usage := normalizeOpenAIUsage(parsed.Usage)
	if usage.ImageCount == 0 {
		usage.ImageCount = len(images)
	}
	call.Status = APICallStatusSuccess
	call.ResponseMetadata = map[string]any{
		"outputCount": len(images),
		"usage":       usage.Raw,
	}
	return ImageResult{Images: images, Usage: usage, APICall: call}, nil
}

func (c *Client) executeGemini(ctx context.Context, req ImageRequest) (ImageResult, error) {
	if err := validateRequest(req); err != nil {
		result := ImageResult{APICall: baseAPICall(req)}
		result.APICall.Status = APICallStatusFailure
		result.APICall.ErrorCode = "PROVIDER_REQUEST_INVALID"
		result.APICall.ErrorMessage = "Provider request is invalid."
		return result, err
	}
	endpoint := "/models/" + url.PathEscape(req.Model.ModelName) + ":generateContent"
	requestURL, err := appendEndpoint(req.Provider.BaseURL, endpoint)
	if err != nil {
		result := ImageResult{APICall: baseAPICall(req)}
		result.APICall.Status = APICallStatusFailure
		result.APICall.ErrorCode = "PROVIDER_URL_INVALID"
		result.APICall.ErrorMessage = "Provider base URL is invalid."
		return result, err
	}

	payload := geminiPayload(req)
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ImageResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(encoded))
	if err != nil {
		return ImageResult{}, err
	}
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("X-Goog-Api-Key", req.Provider.APIKey)

	call := baseAPICall(req)
	call.RequestMetadata["endpointPath"] = endpoint
	startedAt := c.now()
	response, err := c.httpClient.Do(httpRequest)
	call.DurationMs = c.now().Sub(startedAt).Milliseconds()
	if err != nil {
		call.Status = APICallStatusFailure
		call.ErrorCode = "PROVIDER_TRANSPORT_ERROR"
		call.ErrorMessage = SanitizeErrorMessage(err.Error())
		return ImageResult{APICall: call}, ProviderError{Code: call.ErrorCode, Message: call.ErrorMessage, Retryable: true}
	}
	defer response.Body.Close()
	call.HTTPStatus = statusCodePointer(response.StatusCode)
	call.RequestID = providerRequestID(response.Header)

	data, readErr := readLimited(response.Body, c.maxResponseBytes)
	if readErr != nil {
		call.Status = APICallStatusFailure
		call.ErrorCode = "PROVIDER_RESPONSE_TOO_LARGE"
		call.ErrorMessage = "Provider response could not be read safely."
		return ImageResult{APICall: call}, ProviderError{Code: call.ErrorCode, Message: call.ErrorMessage, HTTPStatus: call.HTTPStatus, Retryable: true}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		providerErr := providerHTTPError(response.StatusCode, data)
		call.Status = APICallStatusFailure
		call.ErrorCode = providerErr.Code
		call.ErrorMessage = providerErr.Message
		return ImageResult{APICall: call}, providerErr
	}

	images, usage, err := parseGeminiResponse(data)
	if err != nil {
		call.Status = APICallStatusFailure
		call.ErrorCode = "PROVIDER_RESPONSE_INVALID"
		call.ErrorMessage = "Provider response could not be normalized."
		return ImageResult{APICall: call}, ProviderError{Code: call.ErrorCode, Message: call.ErrorMessage, HTTPStatus: call.HTTPStatus}
	}
	if usage.ImageCount == 0 {
		usage.ImageCount = len(images)
	}
	call.Status = APICallStatusSuccess
	call.ResponseMetadata = map[string]any{
		"outputCount": len(images),
		"usage":       usage.Raw,
	}
	return ImageResult{Images: images, Usage: usage, APICall: call}, nil
}

func validateRequest(req ImageRequest) error {
	if strings.TrimSpace(req.Provider.BaseURL) == "" || strings.TrimSpace(req.Provider.APIKey) == "" || strings.TrimSpace(req.Model.ModelName) == "" || strings.TrimSpace(req.Prompt) == "" {
		return ErrInvalidRequest
	}
	switch req.Operation {
	case OperationGenerate:
		return nil
	case OperationEdit:
		if len(req.InputImages) == 0 {
			return ErrInvalidRequest
		}
		return nil
	default:
		return ErrUnsupportedTask
	}
}

func openAIJSONPayload(req ImageRequest, compatible bool) map[string]any {
	payload := map[string]any{
		"model":  req.Model.ModelName,
		"prompt": req.Prompt,
	}
	if count := outputCount(req.Parameters); count > 0 {
		payload["n"] = count
	}
	if size := parameterString(req.Parameters, "size"); size != "" {
		payload["size"] = size
	}
	if quality := parameterString(req.Parameters, "quality"); quality != "" {
		payload["quality"] = quality
	}
	if format := parameterString(req.Parameters, "outputFormat"); format != "" {
		payload["output_format"] = format
	}
	if compatible {
		payload["response_format"] = "b64_json"
	}
	return payload
}

func writeOpenAIParameterFields(writer *multipart.Writer, req ImageRequest, compatible bool) {
	if count := outputCount(req.Parameters); count > 0 {
		_ = writer.WriteField("n", strconv.Itoa(count))
	}
	if size := parameterString(req.Parameters, "size"); size != "" {
		_ = writer.WriteField("size", size)
	}
	if quality := parameterString(req.Parameters, "quality"); quality != "" {
		_ = writer.WriteField("quality", quality)
	}
	if format := parameterString(req.Parameters, "outputFormat"); format != "" {
		_ = writer.WriteField("output_format", format)
	}
	if compatible {
		_ = writer.WriteField("response_format", "b64_json")
	}
}

func geminiPayload(req ImageRequest) map[string]any {
	parts := []map[string]any{{"text": req.Prompt}}
	for _, image := range req.InputImages {
		if len(image.Data) == 0 || strings.TrimSpace(image.MIMEType) == "" {
			continue
		}
		parts = append(parts, map[string]any{
			"inline_data": map[string]any{
				"mime_type": image.MIMEType,
				"data":      base64.StdEncoding.EncodeToString(image.Data),
			},
		})
	}
	payload := map[string]any{
		"contents": []map[string]any{{
			"role":  "user",
			"parts": parts,
		}},
		"generationConfig": map[string]any{
			"responseModalities": []string{"IMAGE"},
		},
	}
	if count := outputCount(req.Parameters); count > 0 {
		payload["generationConfig"].(map[string]any)["candidateCount"] = count
	}
	return payload
}

func baseAPICall(req ImageRequest) APICall {
	count := outputCount(req.Parameters)
	if count == 0 {
		count = 1
	}
	return APICall{
		Status: APICallStatusFailure,
		RequestMetadata: map[string]any{
			"providerType":        req.Provider.Type,
			"operation":           req.Operation,
			"modelName":           req.Model.ModelName,
			"size":                parameterString(req.Parameters, "size"),
			"quality":             parameterString(req.Parameters, "quality"),
			"outputFormat":        parameterString(req.Parameters, "outputFormat"),
			"outputCount":         count,
			"referenceImageCount": len(req.InputImages),
		},
		ResponseMetadata: map[string]any{},
	}
}

func parameterString(parameters map[string]any, key string) string {
	if parameters == nil {
		return ""
	}
	value, ok := parameters[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func outputCount(parameters map[string]any) int {
	if parameters == nil {
		return 1
	}
	for _, key := range []string{"outputCount", "n"} {
		value, ok := parameters[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed
		case int64:
			return int(typed)
		case float64:
			return int(typed)
		case json.Number:
			parsed, err := typed.Int64()
			if err == nil {
				return int(parsed)
			}
		}
	}
	return 1
}

func inputFilename(image InputImage, index int) string {
	name := strings.TrimSpace(image.Filename)
	if name != "" {
		return name
	}
	ext := "img"
	switch strings.ToLower(strings.TrimSpace(image.MIMEType)) {
	case "image/jpeg":
		ext = "jpg"
	case "image/png":
		ext = "png"
	case "image/webp":
		ext = "webp"
	}
	return fmt.Sprintf("input-%d.%s", index+1, ext)
}

func appendEndpoint(baseURL string, endpoint string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", ErrInvalidRequest
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	endpoint = "/" + strings.TrimLeft(endpoint, "/")
	parsed.Path = path.Clean(basePath + endpoint)
	if strings.HasSuffix(endpoint, ":generateContent") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "%3AgenerateContent")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = defaultMaxResponseBytes
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("provider response exceeded safe size limit")
	}
	return data, nil
}

func providerRequestID(header http.Header) string {
	for _, key := range []string{"X-Request-ID", "OpenAI-Request-ID", "X-Goog-Request-ID"} {
		if requestID := cleanRequestID(header.Get(key)); requestID != "" {
			return requestID
		}
	}
	return ""
}
