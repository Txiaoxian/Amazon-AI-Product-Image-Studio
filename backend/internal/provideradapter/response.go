package provideradapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type openAIResponse struct {
	Data  []openAIImage  `json:"data"`
	Usage map[string]any `json:"usage"`
	Raw   map[string]any `json:"-"`
}

type openAIImage struct {
	B64JSON string         `json:"b64_json"`
	URL     string         `json:"url"`
	MIME    string         `json:"mime_type"`
	Raw     map[string]any `json:"-"`
}

func parseOpenAIResponse(data []byte) (openAIResponse, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return openAIResponse{}, err
	}
	var parsed openAIResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return openAIResponse{}, err
	}
	if len(parsed.Data) == 0 {
		return openAIResponse{}, errors.New("openai response contained no images")
	}
	if usage, ok := raw["usage"].(map[string]any); ok {
		parsed.Usage = usage
	}
	if records, ok := raw["data"].([]any); ok {
		for index, record := range records {
			if index >= len(parsed.Data) {
				break
			}
			if item, ok := record.(map[string]any); ok {
				parsed.Data[index].Raw = item
			}
		}
	}
	parsed.Raw = raw
	return parsed, nil
}

func (c *Client) normalizeOpenAIImages(ctx context.Context, parsed openAIResponse) ([]Image, error) {
	images := make([]Image, 0, len(parsed.Data))
	for index, item := range parsed.Data {
		mimeType := strings.TrimSpace(item.MIME)
		if mimeType == "" {
			mimeType = "image/png"
		}
		switch {
		case strings.TrimSpace(item.B64JSON) != "":
			data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(item.B64JSON))
			if err != nil {
				return nil, err
			}
			images = append(images, Image{
				Data:     data,
				MIMEType: mimeType,
				Metadata: map[string]any{
					"providerOutputIndex": index,
				},
			})
		case strings.TrimSpace(item.URL) != "":
			data, contentType, err := c.fetchImageURL(ctx, item.URL)
			if err != nil {
				return nil, err
			}
			if contentType != "" {
				mimeType = contentType
			}
			images = append(images, Image{
				Data:     data,
				MIMEType: mimeType,
				Metadata: map[string]any{
					"providerOutputIndex": index,
					"source":              "provider_url",
				},
			})
		default:
			return nil, errors.New("provider image contained no bytes")
		}
	}
	return images, nil
}

func (c *Client) fetchImageURL(ctx context.Context, rawURL string) ([]byte, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Accept", "image/*")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", errors.New("provider image URL returned non-success status")
	}
	data, err := readLimited(response.Body, c.maxResponseBytes)
	if err != nil {
		return nil, "", err
	}
	contentType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	return data, contentType, nil
}

func normalizeOpenAIUsage(raw map[string]any) Usage {
	usage := Usage{Raw: SanitizeMetadata(raw)}
	usage.InputTokens = int64FromAny(raw["input_tokens"])
	if usage.InputTokens == 0 {
		usage.InputTokens = int64FromAny(raw["prompt_tokens"])
	}
	usage.OutputTokens = int64FromAny(raw["output_tokens"])
	if usage.OutputTokens == 0 {
		usage.OutputTokens = int64FromAny(raw["completion_tokens"])
	}
	usage.ImageCount = intFromAny(raw["image_count"])
	return usage
}

func parseGeminiResponse(data []byte) ([]Image, Usage, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, Usage{}, err
	}
	images := []Image{}
	candidates, _ := raw["candidates"].([]any)
	for candidateIndex, candidate := range candidates {
		candidateMap, _ := candidate.(map[string]any)
		content, _ := candidateMap["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		for partIndex, part := range parts {
			partMap, _ := part.(map[string]any)
			inlineData, _ := partMap["inlineData"].(map[string]any)
			if inlineData == nil {
				inlineData, _ = partMap["inline_data"].(map[string]any)
			}
			if inlineData == nil {
				continue
			}
			dataValue, _ := inlineData["data"].(string)
			if strings.TrimSpace(dataValue) == "" {
				continue
			}
			imageBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(dataValue))
			if err != nil {
				return nil, Usage{}, err
			}
			mimeType, _ := inlineData["mimeType"].(string)
			if mimeType == "" {
				mimeType, _ = inlineData["mime_type"].(string)
			}
			if strings.TrimSpace(mimeType) == "" {
				mimeType = "image/png"
			}
			images = append(images, Image{
				Data:     imageBytes,
				MIMEType: mimeType,
				Metadata: map[string]any{
					"providerCandidateIndex": candidateIndex,
					"providerPartIndex":      partIndex,
				},
			})
		}
	}
	if len(images) == 0 {
		return nil, Usage{}, errors.New("gemini response contained no images")
	}
	usageMetadata, _ := raw["usageMetadata"].(map[string]any)
	usage := Usage{
		InputTokens:  int64FromAny(usageMetadata["promptTokenCount"]),
		OutputTokens: int64FromAny(usageMetadata["candidatesTokenCount"]),
		Raw:          SanitizeMetadata(usageMetadata),
		ImageCount:   len(images),
	}
	return images, usage, nil
}

func int64FromAny(value any) int64 {
	switch typed := value.(type) {
	case int:
		if typed < 0 {
			return 0
		}
		return int64(typed)
	case int64:
		if typed < 0 {
			return 0
		}
		return typed
	case float64:
		if typed < 0 {
			return 0
		}
		return int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed < 0 {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func intFromAny(value any) int {
	parsed := int64FromAny(value)
	if parsed <= 0 {
		return 0
	}
	return int(parsed)
}
