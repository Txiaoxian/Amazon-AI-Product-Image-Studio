package provideradapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
)

const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC"

func TestProviderTimeoutIsNotShortenedBySharedHTTPClientTimeout(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		select {
		case <-time.After(50 * time.Millisecond):
			body := `{"data":[{"b64_json":"` + tinyPNGBase64 + `"}]}`
			return jsonResponse(http.StatusOK, body, ""), nil
		case <-request.Context().Done():
			return nil, request.Context().Err()
		}
	})
	client := NewClient(ClientOptions{
		HTTPClient: &http.Client{Transport: transport, Timeout: 10 * time.Millisecond},
	})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "respect the provider timeout",
		Provider: ProviderConfig{
			Type:           provider.TypeOpenAICompatible,
			BaseURL:        "https://relay.example.com/v1",
			APIKey:         "relay-key",
			TimeoutSeconds: 1,
		},
		Model: ModelConfig{ModelName: "gpt-image-2"},
	})
	if err != nil {
		t.Fatalf("Execute returned error before provider timeout: %v", err)
	}
	if result.APICall.Status != APICallStatusSuccess {
		t.Fatalf("api call status = %q, want %q", result.APICall.Status, APICallStatusSuccess)
	}
	if len(result.Images) != 1 {
		t.Fatalf("image count = %d, want 1", len(result.Images))
	}
	defer os.Remove(result.Images[0].FilePath)
}

func TestOpenAIGenerateRequestMappingAndUsage(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != "https://api.openai.com/v1/images/generations" {
			t.Fatalf("request = %s %s", request.Method, request.URL.String())
		}
		if got := request.Header.Get("Authorization"); got != "Bearer sk-test-openai" {
			t.Fatalf("Authorization header = %q", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["model"] != "gpt-image-1" || payload["prompt"] != "make a product image" || payload["size"] != "1024x1024" || payload["quality"] != "high" || payload["output_format"] != "png" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		if _, ok := payload["response_format"]; ok {
			t.Fatalf("official OpenAI payload should not force response_format: %#v", payload)
		}
		body := `{"data":[{"b64_json":"` + tinyPNGBase64 + `"}],"usage":{"input_tokens":12,"output_tokens":3,"image_count":1}}`
		return jsonResponse(http.StatusOK, body, "req-openai"), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "make a product image",
		Provider:  ProviderConfig{Type: provider.TypeOpenAI, BaseURL: "https://api.openai.com/v1", APIKey: "sk-test-openai"},
		Model:     ModelConfig{ModelName: "gpt-image-1"},
		Parameters: map[string]any{
			"size":         "1024x1024",
			"quality":      "high",
			"outputFormat": "png",
			"outputCount":  float64(1),
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(result.Images) != 1 || result.Images[0].FilePath == "" || result.Images[0].SizeBytes == 0 {
		t.Fatalf("normalized images = %#v", result.Images)
	}
	defer os.Remove(result.Images[0].FilePath)
	imageData, err := os.ReadFile(result.Images[0].FilePath)
	if err != nil || len(imageData) == 0 {
		t.Fatalf("read streamed provider image: len=%d err=%v", len(imageData), err)
	}
	if result.Usage.InputTokens != 12 || result.Usage.OutputTokens != 3 || result.Usage.ImageCount != 1 {
		t.Fatalf("usage = %#v", result.Usage)
	}
	if result.APICall.Status != APICallStatusSuccess || result.APICall.RequestID != "req-openai" {
		t.Fatalf("api call = %#v", result.APICall)
	}
}

func TestOpenAICompatibleStreamsLargeBase64ResponseToTemporaryFile(t *testing.T) {
	payloadBytes := bytes.Repeat([]byte("large-provider-image"), 32*1024)
	encoded := base64.StdEncoding.EncodeToString(payloadBytes)
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":[{"b64_json":"`+encoded+`"}]}`, ""), nil
	})
	tempDir := t.TempDir()
	client := NewClient(ClientOptions{
		HTTPClient:       &http.Client{Transport: transport},
		MaxResponseBytes: int64(len(encoded) + 1024),
		MaxImageBytes:    int64(len(payloadBytes) + 1),
		TempDir:          tempDir,
	})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "large image",
		Provider:  ProviderConfig{Type: provider.TypeOpenAICompatible, BaseURL: "https://relay.example.com/v1", APIKey: "relay-key"},
		Model:     ModelConfig{ModelName: "gpt-image-2"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(result.Images) != 1 || len(result.Images[0].Data) != 0 || !result.Images[0].Temporary {
		t.Fatalf("streamed image = %#v", result.Images)
	}
	defer os.Remove(result.Images[0].FilePath)
	got, err := os.ReadFile(result.Images[0].FilePath)
	if err != nil {
		t.Fatalf("read temporary image: %v", err)
	}
	if !bytes.Equal(got, payloadBytes) {
		t.Fatalf("temporary image bytes did not match provider payload: got=%d want=%d", len(got), len(payloadBytes))
	}
}

func TestOpenAICompatibleRejectsImageAboveConfiguredLimitWithoutLeavingTempFile(t *testing.T) {
	payloadBytes := bytes.Repeat([]byte("oversized"), 1024)
	encoded := base64.StdEncoding.EncodeToString(payloadBytes)
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"data":[{"b64_json":"`+encoded+`"}]}`, ""), nil
	})
	tempDir := t.TempDir()
	client := NewClient(ClientOptions{
		HTTPClient:       &http.Client{Transport: transport},
		MaxResponseBytes: int64(len(encoded) + 1024),
		MaxImageBytes:    int64(len(payloadBytes) - 1),
		TempDir:          tempDir,
	})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "oversized image",
		Provider:  ProviderConfig{Type: provider.TypeOpenAICompatible, BaseURL: "https://relay.example.com/v1", APIKey: "relay-key"},
		Model:     ModelConfig{ModelName: "gpt-image-2"},
	})
	if err == nil || result.APICall.ErrorCode != "PROVIDER_RESPONSE_TOO_LARGE" {
		t.Fatalf("result/error = %#v / %v", result, err)
	}
	entries, readErr := os.ReadDir(tempDir)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("temporary files leaked after rejection: entries=%v err=%v", entries, readErr)
	}
}

func TestOpenAIURLFetchFailureKeepsStableRetryableError(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPost {
			return jsonResponse(http.StatusOK, `{"data":[{"url":"https://cdn.example.com/image.png"}]}`, ""), nil
		}
		return jsonResponse(http.StatusBadGateway, `{}`, ""), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "image URL",
		Provider:  ProviderConfig{Type: provider.TypeOpenAI, BaseURL: "https://api.openai.com/v1", APIKey: "openai-key"},
		Model:     ModelConfig{ModelName: "gpt-image-1"},
	})
	var providerErr ProviderError
	if !errors.As(err, &providerErr) || !providerErr.Retryable {
		t.Fatalf("error = %#v, want retryable ProviderError", err)
	}
	if result.APICall.ErrorCode != "PROVIDER_IMAGE_FETCH_FAILED" || strings.Contains(result.APICall.ErrorMessage, "cdn.example.com") {
		t.Fatalf("api call = %#v", result.APICall)
	}
}

func TestOpenAICompatibleGenerateRequestsBaseURLAndB64ResponseFormat(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://relay.example.com/custom/v1/images/generations" {
			t.Fatalf("request URL = %s", request.URL.String())
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["response_format"] != "b64_json" {
			t.Fatalf("compatible payload missing b64 response format: %#v", payload)
		}
		if payload["size"] != "1024x1280" || payload["quality"] != "low" {
			t.Fatalf("compatible payload did not map official ratio and quality: %#v", payload)
		}
		body := `{"data":[{"b64_json":"` + tinyPNGBase64 + `"}]}`
		return jsonResponse(http.StatusOK, body, ""), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "compatible prompt",
		Provider:  ProviderConfig{Type: provider.TypeOpenAICompatible, BaseURL: "https://relay.example.com/custom/v1", APIKey: "relay-key"},
		Model:     ModelConfig{ModelName: "gpt-image-2"},
		Parameters: map[string]any{
			"size":    "4:5",
			"quality": "low",
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.APICall.RequestMetadata["size"] != "1024x1280" {
		t.Fatalf("request metadata size = %#v, want actual outbound size", result.APICall.RequestMetadata["size"])
	}
}

func TestOpenAICompatibleEditUsesImageArrayMultipartField(t *testing.T) {
	inputBytes, _ := base64.StdEncoding.DecodeString(tinyPNGBase64)
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != "https://relay.example.com/v1/images/edits" {
			t.Fatalf("request = %s %s", request.Method, request.URL.String())
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart request: %v", err)
		}
		if files := request.MultipartForm.File["image[]"]; len(files) != 1 {
			t.Fatalf("image[] files = %d, want 1; all files = %#v", len(files), request.MultipartForm.File)
		}
		if files := request.MultipartForm.File["image"]; len(files) != 0 {
			t.Fatalf("legacy image files = %d, want 0", len(files))
		}
		if request.FormValue("model") != "gpt-image-2" || request.FormValue("prompt") != "preserve the referenced product" {
			t.Fatalf("unexpected edit form values: %#v", request.MultipartForm.Value)
		}
		if request.FormValue("size") != "1296x800" || request.FormValue("quality") != "high" || request.FormValue("output_format") != "png" {
			t.Fatalf("unexpected image parameters: %#v", request.MultipartForm.Value)
		}
		if request.FormValue("response_format") != "b64_json" {
			t.Fatalf("compatible edit should request b64 response: %#v", request.MultipartForm.Value)
		}
		body := `{"data":[{"b64_json":"` + tinyPNGBase64 + `"}]}`
		return jsonResponse(http.StatusOK, body, ""), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	if _, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationEdit,
		Prompt:    "preserve the referenced product",
		Provider:  ProviderConfig{Type: provider.TypeOpenAICompatible, BaseURL: "https://relay.example.com/v1", APIKey: "relay-key"},
		Model:     ModelConfig{ModelName: "gpt-image-2"},
		Parameters: map[string]any{
			"size":         "1.62:1",
			"quality":      "high",
			"outputFormat": "png",
			"outputCount":  float64(1),
		},
		InputImages: []InputImage{{Data: inputBytes, MIMEType: "image/png", Filename: "reference.png"}},
	}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
}

func TestOpenAIGPTImage2MapsSupportedAspectRatiosToOfficialPixelSizes(t *testing.T) {
	want := map[string]string{
		"auto":   "auto",
		"1:1":    "1024x1024",
		"1.62:1": "1296x800",
		"2:3":    "1024x1536",
		"3:2":    "1536x1024",
		"3:4":    "1152x1536",
		"4:3":    "1536x1152",
		"4:5":    "1024x1280",
		"5:4":    "1280x1024",
		"9:16":   "864x1536",
		"16:9":   "1536x864",
		"21:9":   "1792x768",
	}

	for ratio, expectedSize := range want {
		t.Run(ratio, func(t *testing.T) {
			payload := openAIJSONPayload(ImageRequest{
				Model:      ModelConfig{ModelName: "gpt-image-2"},
				Parameters: map[string]any{"size": ratio, "quality": "medium"},
			}, false)
			if payload["size"] != expectedSize || payload["quality"] != "medium" {
				t.Fatalf("payload = %#v, want size %s and unchanged quality", payload, expectedSize)
			}
		})
	}
}

func TestOpenAIImageSizeLimitsMappingToGPTImage2Ratios(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
		size      string
		want      string
	}{
		{name: "other model ratio passes through", modelName: "custom-image-model", size: "4:5", want: "4:5"},
		{name: "legacy pixel size passes through", modelName: "gpt-image-2", size: "1024x1536", want: "1024x1536"},
		{name: "dated snapshot ratio is mapped", modelName: "gpt-image-2-2026-07-01", size: "16:9", want: "1536x864"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := openAIImageSize(ImageRequest{
				Model:      ModelConfig{ModelName: test.modelName},
				Parameters: map[string]any{"size": test.size},
			})
			if got != test.want {
				t.Fatalf("openAIImageSize() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGeminiEditRequestMappingAndUsage(t *testing.T) {
	inputBytes, _ := base64.StdEncoding.DecodeString(tinyPNGBase64)
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != "https://generativelanguage.googleapis.com/v1beta/models/gemini-image:generateContent" {
			t.Fatalf("request = %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("X-Goog-Api-Key") != "gemini-key" || request.Header.Get("Authorization") != "" {
			t.Fatalf("unexpected auth headers: %#v", request.Header)
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		encoded, _ := json.Marshal(payload)
		text := string(encoded)
		if !strings.Contains(text, "edit this image") || !strings.Contains(text, "inline_data") || !strings.Contains(text, "image/png") {
			t.Fatalf("unexpected gemini payload: %s", text)
		}
		generationConfig, ok := payload["generationConfig"].(map[string]any)
		if !ok {
			t.Fatalf("generationConfig = %#v", payload["generationConfig"])
		}
		imageConfig, ok := generationConfig["imageConfig"].(map[string]any)
		if !ok || imageConfig["aspectRatio"] != "16:9" || imageConfig["imageSize"] != "2K" {
			t.Fatalf("imageConfig = %#v, want aspectRatio 16:9 and imageSize 2K", generationConfig["imageConfig"])
		}
		body := `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"` + tinyPNGBase64 + `"}}]}}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":5,"totalTokenCount":12}}`
		return jsonResponse(http.StatusOK, body, "req-gemini"), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationEdit,
		Prompt:    "edit this image",
		Provider:  ProviderConfig{Type: provider.TypeGemini, BaseURL: "https://generativelanguage.googleapis.com/v1beta", APIKey: "gemini-key"},
		Model:     ModelConfig{ModelName: "gemini-image"},
		Parameters: map[string]any{
			"outputCount": float64(1),
			"size":        "16:9",
			"quality":     "2k",
		},
		InputImages: []InputImage{{Data: inputBytes, MIMEType: "image/png", Filename: "input.png"}},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(result.Images) != 1 || result.Usage.InputTokens != 7 || result.Usage.OutputTokens != 5 {
		t.Fatalf("result = %#v", result)
	}
}

func TestProviderErrorIsSanitized(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusInternalServerError, `{"error":{"message":"Authorization Bearer sk-secret base64 AAAA"}}`, ""), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "prompt",
		Provider:  ProviderConfig{Type: provider.TypeOpenAI, BaseURL: "https://api.openai.com/v1", APIKey: "sk-secret"},
		Model:     ModelConfig{ModelName: "gpt-image-1"},
	})
	if err == nil {
		t.Fatal("Execute succeeded, want sanitized provider error")
	}
	lower := strings.ToLower(result.APICall.ErrorMessage + " " + result.APICall.RedactedTextForTest())
	for _, forbidden := range []string{"sk-secret", "authorization", "bearer", "base64"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("sanitized error leaked %q: %#v", forbidden, result.APICall)
		}
	}
}

func TestGeminiHTTPErrorRedactsCurrentAPIKeyValue(t *testing.T) {
	apiKey := "AIzaSyDUMMYVALUE1234567890"
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"error":{"message":"provider echoed ` + apiKey + `"}}`
		return jsonResponse(http.StatusForbidden, body, ""), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "prompt",
		Provider:  ProviderConfig{Type: provider.TypeGemini, BaseURL: "https://generativelanguage.googleapis.com/v1beta", APIKey: apiKey},
		Model:     ModelConfig{ModelName: "gemini-image"},
	})
	if err == nil {
		t.Fatal("Execute succeeded, want provider error")
	}
	combined := result.APICall.ErrorMessage + " " + err.Error()
	if strings.Contains(combined, apiKey) {
		t.Fatalf("Gemini provider error leaked API key: %q", combined)
	}
	if !strings.Contains(combined, redactedValue) {
		t.Fatalf("Gemini provider error did not contain redacted marker: %q", combined)
	}
}

func TestOpenAIHTTPErrorRedactsCurrentAPIKeyValue(t *testing.T) {
	apiKey := "openai_live_1234567890abcdef"
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"error":{"message":"provider echoed ` + apiKey + `"}}`
		return jsonResponse(http.StatusUnauthorized, body, ""), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "prompt",
		Provider:  ProviderConfig{Type: provider.TypeOpenAI, BaseURL: "https://api.openai.com/v1", APIKey: apiKey},
		Model:     ModelConfig{ModelName: "gpt-image-1"},
	})
	if err == nil {
		t.Fatal("Execute succeeded, want provider error")
	}
	combined := result.APICall.ErrorMessage + " " + err.Error()
	if strings.Contains(combined, apiKey) {
		t.Fatalf("OpenAI provider error leaked API key: %q", combined)
	}
	if !strings.Contains(combined, redactedValue) {
		t.Fatalf("OpenAI provider error did not contain redacted marker: %q", combined)
	}
}

func TestOpenAICompatibleErrorsRedactCurrentAPIKeyValue(t *testing.T) {
	apiKey := "relay_live_1234567890abcdef"
	tests := []struct {
		name      string
		transport http.RoundTripper
	}{
		{
			name: "transport error",
			transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("relay rejected API key " + apiKey)
			}),
		},
		{
			name: "http error body",
			transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				body := `{"error":{"message":"relay rejected ` + apiKey + `"}}`
				return jsonResponse(http.StatusBadGateway, body, ""), nil
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: tc.transport}})
			result, err := client.Execute(context.Background(), ImageRequest{
				Operation: OperationGenerate,
				Prompt:    "prompt",
				Provider:  ProviderConfig{Type: provider.TypeOpenAICompatible, BaseURL: "https://relay.example.com/v1", APIKey: apiKey},
				Model:     ModelConfig{ModelName: "custom-image"},
			})
			if err == nil {
				t.Fatal("Execute succeeded, want provider error")
			}
			combined := result.APICall.ErrorMessage + " " + err.Error()
			if strings.Contains(combined, apiKey) {
				t.Fatalf("compatible provider error leaked API key: %q", combined)
			}
			if !strings.Contains(combined, redactedValue) {
				t.Fatalf("compatible provider error did not contain redacted marker: %q", combined)
			}
		})
	}
}

func TestOpenAICompatibleHTTPErrorRedactsCurrentAPIKeyMapKey(t *testing.T) {
	apiKey := "relay_live_1234567890abcdef"
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"error":{"` + apiKey + `":"rejected"}}`
		return jsonResponse(http.StatusBadGateway, body, ""), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "prompt",
		Provider:  ProviderConfig{Type: provider.TypeOpenAICompatible, BaseURL: "https://relay.example.com/v1", APIKey: apiKey},
		Model:     ModelConfig{ModelName: "custom-image"},
	})
	if err == nil {
		t.Fatal("Execute succeeded, want provider error")
	}
	combined := result.APICall.ErrorMessage + " " + err.Error() + " " + result.APICall.RedactedTextForTest()
	if strings.Contains(combined, apiKey) {
		t.Fatalf("compatible provider error map key leaked API key: %q", combined)
	}
}

func TestOpenAICompatibleInsufficientQuotaReturnsStableNonRetryableError(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusForbidden, `{"error":{"code":"insufficient_user_quota","message":"remaining balance 0.99947, required 1.0"}}`, ""), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	result, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "prompt",
		Provider:  ProviderConfig{Type: provider.TypeOpenAICompatible, BaseURL: "https://relay.example.com/v1", APIKey: "relay-key"},
		Model:     ModelConfig{ModelName: "gpt-image-2"},
	})
	if err == nil {
		t.Fatal("Execute succeeded, want quota error")
	}
	var providerErr ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error type = %T, want ProviderError", err)
	}
	if providerErr.Code != "PROVIDER_INSUFFICIENT_QUOTA" || providerErr.Retryable {
		t.Fatalf("provider error = %#v", providerErr)
	}
	if providerErr.Message != "Provider account quota is insufficient." {
		t.Fatalf("provider error message = %q", providerErr.Message)
	}
	if result.APICall.ErrorCode != providerErr.Code || result.APICall.ErrorMessage != providerErr.Message {
		t.Fatalf("api call = %#v, want stable quota error", result.APICall)
	}
	if strings.Contains(result.APICall.ErrorMessage, "0.99947") || strings.Contains(result.APICall.ErrorMessage, "1.0") {
		t.Fatalf("quota error leaked provider account details: %q", result.APICall.ErrorMessage)
	}
}

func (c APICall) RedactedTextForTest() string {
	requestJSON, _ := json.Marshal(c.RequestMetadata)
	responseJSON, _ := json.Marshal(c.ResponseMetadata)
	return string(requestJSON) + string(responseJSON)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponse(status int, body string, requestID string) *http.Response {
	response := &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
	response.Header.Set("Content-Type", "application/json")
	if requestID != "" {
		response.Header.Set("X-Request-ID", requestID)
	}
	return response
}
