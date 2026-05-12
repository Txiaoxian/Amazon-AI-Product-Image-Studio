package provider

import (
	"context"
	"net"
	"net/url"
	"strings"
)

type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type URLValidator struct {
	resolver Resolver
}

func NewURLValidator(resolver Resolver) *URLValidator {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &URLValidator{resolver: resolver}
}

func (v *URLValidator) Validate(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len([]rune(raw)) > maxProviderURLRunes {
		return "", ErrValidation
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", ErrValidation
	}
	if err := v.ValidateParsed(ctx, parsed); err != nil {
		return "", err
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (v *URLValidator) ValidateParsed(ctx context.Context, parsed *url.URL) error {
	if parsed == nil {
		return ErrValidation
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if v == nil || v.resolver == nil {
		v = NewURLValidator(nil)
	}

	if parsed.Scheme != "https" {
		return ErrValidation
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return ErrValidation
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ErrValidation
	}

	host := strings.TrimSpace(parsed.Hostname())
	if isBlockedHostname(host) {
		return ErrValidation
	}

	ips, err := resolveHost(ctx, v.resolver, host)
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

func resolveHost(ctx context.Context, resolver Resolver, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	addresses, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		if address.IP != nil {
			ips = append(ips, address.IP)
		}
	}
	return ips, nil
}

func isBlockedHostname(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if !strings.Contains(host, ".") && net.ParseIP(host) == nil {
		return true
	}

	blocked := map[string]struct{}{
		"backend-api":              {},
		"backend-worker":           {},
		"frontend":                 {},
		"mysql":                    {},
		"redis":                    {},
		"minio":                    {},
		"host.docker.internal":     {},
		"gateway.docker.internal":  {},
		"docker.for.mac.localhost": {},
		"docker.for.win.localhost": {},
	}
	_, ok := blocked[host]
	return ok
}

func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}

	for _, cidr := range blockedCIDRs() {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func blockedCIDRs() []*net.IPNet {
	cidrs := []string{
		"100.64.0.0/10",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
	}
	blocks := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		_, block, err := net.ParseCIDR(raw)
		if err == nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}
