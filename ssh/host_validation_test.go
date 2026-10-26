package ssh

import (
	"net/http"
	"testing"
)

func TestValidateHost(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{name: "hostname", host: "example.com"},
		{name: "localhost", host: "localhost"},
		{name: "ipv4", host: "192.0.2.1"},
		{name: "ipv6", host: "2001:db8::1"},
		{name: "ipv6 with zone", host: "fe80::1%eth0"},
		{name: "empty", host: "", wantErr: true},
		{name: "host and port", host: "any:localhost", wantErr: true},
		{name: "host with numeric port", host: "example.com:22", wantErr: true},
		{name: "bracketed ipv6", host: "[2001:db8::1]", wantErr: true},
		{name: "space", host: "bad host", wantErr: true},
		{name: "path", host: "example.com/path", wantErr: true},
		{name: "user prefix", host: "user@example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := validateHost(tt.host, "host")
			if tt.wantErr && len(errs) == 0 {
				t.Fatalf("expected validation error for %q", tt.host)
			}
			if !tt.wantErr && len(errs) != 0 {
				t.Fatalf("unexpected validation error for %q: %v", tt.host, errs)
			}
		})
	}
}

func TestValidateOptionalHost(t *testing.T) {
	if _, errs := validateOptionalHost("", "bastion_host"); len(errs) != 0 {
		t.Fatalf("unexpected validation error for empty optional host: %v", errs)
	}
	if _, errs := validateOptionalHost("bastion.example.com", "bastion_host"); len(errs) != 0 {
		t.Fatalf("unexpected validation error for optional host: %v", errs)
	}
	if _, errs := validateOptionalHost("any:localhost", "bastion_host"); len(errs) == 0 {
		t.Fatal("expected validation error for optional host with invalid port")
	}
}

func TestProxyFromEnvironmentHandlesNilRequest(t *testing.T) {
	proxy, err := proxyFromEnvironment(nil)
	if proxy != nil || err != nil {
		t.Fatalf("expected nil proxy and error, got %v, %v", proxy, err)
	}

	proxy, err = proxyFromEnvironment(&http.Request{})
	if proxy != nil || err != nil {
		t.Fatalf("expected nil proxy and error for request without URL, got %v, %v", proxy, err)
	}
}
