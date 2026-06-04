package task

type GeneratedImageOutput struct {
	Data     []byte
	MIMEType string
	Metadata map[string]any
}

type UsageResult struct {
	InputTokens  int64
	OutputTokens int64
	ImageCount   int
	Raw          map[string]any
}

type APICallResult struct {
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
