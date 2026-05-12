package provider

import (
	"context"
	"net"
	"net/http"
	"testing"
)

type staticResolver map[string][]net.IPAddr

func (r staticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if addresses, ok := r[host]; ok {
		return addresses, nil
	}
	return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
}

func TestURLValidatorBlocksSSRFInputs(t *testing.T) {
	validator := NewURLValidator(staticResolver{
		"private.example.com":   {{IP: net.ParseIP("10.0.0.5")}},
		"linklocal.example.com": {{IP: net.ParseIP("169.254.1.1")}},
		"multicast.example.com": {{IP: net.ParseIP("224.0.0.1")}},
	})

	cases := []string{
		"http://api.openai.com/v1",
		"file:///etc/passwd",
		"https://user:pass@api.openai.com/v1",
		"https://localhost/v1",
		"https://127.0.0.1/v1",
		"https://[::1]/v1",
		"https://10.0.0.1/v1",
		"https://192.168.1.10/v1",
		"https://172.16.0.10/v1",
		"https://169.254.1.10/v1",
		"https://backend-api/v1",
		"https://mysql/v1",
		"https://redis/v1",
		"https://minio/v1",
		"https://host.docker.internal/v1",
		"https://private.example.com/v1",
		"https://linklocal.example.com/v1",
		"https://multicast.example.com/v1",
	}

	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if _, err := validator.Validate(context.Background(), raw); err == nil {
				t.Fatalf("Validate accepted blocked URL %s", raw)
			}
		})
	}
}

func TestURLValidatorAllowsPublicHTTPSURL(t *testing.T) {
	validator := NewURLValidator(staticResolver{
		"api.openai.com": {{IP: net.ParseIP("93.184.216.34")}},
	})

	normalized, err := validator.Validate(context.Background(), "https://api.openai.com/v1")
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if normalized != "https://api.openai.com/v1" {
		t.Fatalf("normalized URL = %q", normalized)
	}
}

func TestHTTPProberRejectsRedirectToBlockedTarget(t *testing.T) {
	validator := NewURLValidator(staticResolver{
		"api.openai.com": {{IP: net.ParseIP("93.184.216.34")}},
	})
	prober := NewHTTPProber(nil, validator)

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://127.0.0.1/v1", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if err := prober.checkRedirect(request, nil); err == nil {
		t.Fatal("redirect check accepted blocked target")
	}
}
