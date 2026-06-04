package provideradapter

import (
	"context"
	"errors"
)

const (
	OperationGenerate = "generate"
	OperationEdit     = "edit"

	APICallStatusSuccess    = "SUCCESS"
	APICallStatusFailure    = "FAILURE"
	APICallStatusAttempting = "ATTEMPTING"
	APICallStatusTimeout    = "TIMEOUT"
	APICallStatusCancelled  = "CANCELLED"
)

var (
	ErrUnsupportedProvider = errors.New("unsupported provider type")
	ErrUnsupportedTask     = errors.New("unsupported provider task")
	ErrInvalidRequest      = errors.New("invalid provider adapter request")
)

type ProviderConfig struct {
	ID             string
	Type           string
	BaseURL        string
	APIKey         string
	TimeoutSeconds int
}

type ModelConfig struct {
	ID        string
	ModelName string
}

type ImageRequest struct {
	TenantID    string
	ProjectID   string
	TaskID      string
	Operation   string
	Prompt      string
	ImageType   string
	Provider    ProviderConfig
	Model       ModelConfig
	Parameters  map[string]any
	InputImages []InputImage
}

type InputImage struct {
	Data     []byte
	MIMEType string
	Filename string
}

type ImageResult struct {
	Images  []Image
	Usage   Usage
	APICall APICall
}

type Image struct {
	Data     []byte
	MIMEType string
	Metadata map[string]any
}

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	ImageCount   int
	Raw          map[string]any
}

type APICall struct {
	ID               string
	Status           string
	DurationMs       int64
	RequestID        string
	HTTPStatus       *int
	ErrorCode        string
	ErrorMessage     string
	RequestMetadata  map[string]any
	ResponseMetadata map[string]any
}

type ProviderError struct {
	Code       string
	Message    string
	HTTPStatus *int
	Retryable  bool
}

func (e ProviderError) Error() string {
	if e.Message == "" {
		return "provider request failed"
	}
	return e.Message
}

type Runtime interface {
	Execute(ctx context.Context, req ImageRequest) (ImageResult, error)
}
