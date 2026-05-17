package provider

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SafeRoundTripper struct {
	validator *URLValidator
	transport *http.Transport
}

func NewSafeHTTPClient(validator *URLValidator, timeout time.Duration) *http.Client {
	if validator == nil {
		validator = NewURLValidator(nil)
	}
	if timeout <= 0 {
		timeout = defaultRequestTimeout * time.Second
	}
	roundTripper := NewSafeRoundTripper(validator)
	return &http.Client{
		Transport:     roundTripper,
		Timeout:       timeout,
		CheckRedirect: roundTripper.CheckRedirect,
	}
}

func NewSafeRoundTripper(validator *URLValidator) *SafeRoundTripper {
	if validator == nil {
		validator = NewURLValidator(nil)
	}
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network string, address string) (net.Conn, error) {
		return safeDialContext(ctx, validator, dialer, network, address)
	}
	return &SafeRoundTripper{validator: validator, transport: transport}
}

func (t *SafeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t == nil || t.transport == nil {
		t = NewSafeRoundTripper(nil)
	}
	if req == nil || req.URL == nil {
		return nil, ErrValidation
	}
	if err := validateOutboundURL(req.Context(), t.validator, req.URL); err != nil {
		return nil, err
	}
	return t.transport.RoundTrip(req)
}

func (t *SafeRoundTripper) CheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return http.ErrUseLastResponse
	}
	if t == nil || t.validator == nil {
		t = NewSafeRoundTripper(nil)
	}
	if req == nil || req.URL == nil {
		return ErrValidation
	}
	return validateOutboundURL(req.Context(), t.validator, req.URL)
}

func validateOutboundURL(ctx context.Context, validator *URLValidator, parsed *url.URL) error {
	if parsed == nil {
		return ErrValidation
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if validator == nil || validator.resolver == nil {
		validator = NewURLValidator(nil)
	}
	if parsed.Scheme != "https" {
		return ErrValidation
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return ErrValidation
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return ErrValidation
	}
	host := strings.TrimSpace(parsed.Hostname())
	if isBlockedHostname(host) {
		return ErrValidation
	}
	ips, err := resolveHost(ctx, validator.resolver, host)
	if err != nil || len(ips) == 0 {
		return ErrValidation
	}
	for _, ip := range ips {
		if blockedIP(ip) {
			return ErrValidation
		}
	}
	return nil
}

func safeDialContext(ctx context.Context, validator *URLValidator, dialer *net.Dialer, network string, address string) (net.Conn, error) {
	if validator == nil {
		validator = NewURLValidator(nil)
	}
	if dialer == nil {
		dialer = &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" {
		return nil, ErrValidation
	}
	if isBlockedHostname(host) {
		return nil, ErrValidation
	}

	ips, err := resolveHost(ctx, validator.resolver, host)
	if err != nil || len(ips) == 0 {
		return nil, ErrValidation
	}
	for _, ip := range ips {
		if blockedIP(ip) {
			return nil, ErrValidation
		}
	}

	target := net.JoinHostPort(ips[0].String(), port)
	return dialer.DialContext(ctx, network, target)
}
