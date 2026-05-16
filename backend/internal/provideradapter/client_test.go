package provideradapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Txiaoxian/Amazon-AI-Product-Image-Studio/backend/internal/provider"
)

const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC"

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
	if len(result.Images) != 1 || len(result.Images[0].Data) == 0 {
		t.Fatalf("normalized images = %#v", result.Images)
	}
	if result.Usage.InputTokens != 12 || result.Usage.OutputTokens != 3 || result.Usage.ImageCount != 1 {
		t.Fatalf("usage = %#v", result.Usage)
	}
	if result.APICall.Status != APICallStatusSuccess || result.APICall.RequestID != "req-openai" {
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
		body := `{"data":[{"b64_json":"` + tinyPNGBase64 + `"}]}`
		return jsonResponse(http.StatusOK, body, ""), nil
	})
	client := NewClient(ClientOptions{HTTPClient: &http.Client{Transport: transport}})

	if _, err := client.Execute(context.Background(), ImageRequest{
		Operation: OperationGenerate,
		Prompt:    "compatible prompt",
		Provider:  ProviderConfig{Type: provider.TypeOpenAICompatible, BaseURL: "https://relay.example.com/custom/v1", APIKey: "relay-key"},
		Model:     ModelConfig{ModelName: "custom-image-model"},
	}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
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
