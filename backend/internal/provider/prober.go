package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProbeConfig struct {
	Type           string
	BaseURL        string
	APIKey         string
	TimeoutSeconds int
}

type ProbeResult struct {
	Status     string
	DurationMs int64
	CheckedAt  time.Time
	HTTPStatus *int
	RequestID  string
	Message    string
}

type Prober interface {
	Test(ctx context.Context, config ProbeConfig) (ProbeResult, error)
}

type HTTPProber struct {
	client    *http.Client
	validator *URLValidator
	now       func() time.Time
}

func NewHTTPProber(client *http.Client, validator *URLValidator) *HTTPProber {
	if validator == nil {
		validator = NewURLValidator(nil)
	}
	if client == nil {
		client = &http.Client{}
	}
	prober := &HTTPProber{
		client:    client,
		validator: validator,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	prober.client.CheckRedirect = prober.checkRedirect
	return prober
}

func (p *HTTPProber) Test(ctx context.Context, config ProbeConfig) (ProbeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultRequestTimeout * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startedAt := p.now()
	result := ProbeResult{
		Status:    TestStatusFailure,
		CheckedAt: startedAt,
		Message:   "Provider test failed.",
	}

	probeURL, err := p.probeURL(ctx, config)
	if err != nil {
		result.DurationMs = time.Since(startedAt).Milliseconds()
		return result, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		result.DurationMs = time.Since(startedAt).Milliseconds()
		return result, ErrProviderTest
	}
	request.Header.Set("Accept", "application/json")
	switch config.Type {
	case TypeGemini:
		request.Header.Set("X-Goog-Api-Key", config.APIKey)
	default:
		request.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	response, err := p.client.Do(request)
	result.DurationMs = time.Since(startedAt).Milliseconds()
	result.CheckedAt = p.now()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			result.Message = "Provider test timed out."
		}
		return result, ErrProviderTest
	}
	defer response.Body.Close()
	_, _ = io.CopyN(io.Discard, response.Body, 1024)

	httpStatus := response.StatusCode
	result.HTTPStatus = &httpStatus
	result.RequestID = cleanProviderRequestID(response.Header.Get("X-Request-ID"))
	if result.RequestID == "" {
		result.RequestID = cleanProviderRequestID(response.Header.Get("OpenAI-Request-ID"))
	}

	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		result.Status = TestStatusSuccess
		result.Message = "Provider test succeeded."
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden:
		result.Message = "Provider authentication failed."
	default:
		result.Message = "Provider test returned a non-success status."
	}

	return result, nil
}

func (p *HTTPProber) probeURL(ctx context.Context, config ProbeConfig) (string, error) {
	base, err := url.Parse(strings.TrimSpace(config.BaseURL))
	if err != nil {
		return "", ErrValidation
	}
	if err := p.validator.ValidateParsed(ctx, base); err != nil {
		return "", err
	}

	path := strings.TrimRight(base.Path, "/") + "/models"
	base.Path = path
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func (p *HTTPProber) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return http.ErrUseLastResponse
	}
	if err := p.validator.ValidateParsed(req.Context(), req.URL); err != nil {
		return err
	}
	return nil
}

func cleanProviderRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, r := range value {
		if r < 33 || r > 126 {
			return ""
		}
	}
	return value
}
