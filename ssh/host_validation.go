package ssh

import (
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

func validateOptionalHost(v interface{}, k string) ([]string, []error) {
	host, ok := v.(string)
	if !ok {
		return nil, []error{fmt.Errorf("%q must be a string", k)}
	}
	if host == "" {
		return nil, nil
	}
	return validateHost(host, k)
}

func validateHost(v interface{}, k string) ([]string, []error) {
	host, ok := v.(string)
	if !ok {
		return nil, []error{fmt.Errorf("%q must be a string", k)}
	}
	if host == "" {
		return nil, []error{fmt.Errorf("%q must not be empty", k)}
	}

	// Hostnames and IP addresses do not normally need a port here. A colon is
	// only valid for an unbracketed IPv6 address; host:port values belong in
	// the separate port argument.
	if _, err := netip.ParseAddr(host); err == nil {
		return nil, nil
	}
	if strings.Contains(host, ":") {
		return nil, []error{fmt.Errorf("%q must be a hostname or IP address without a port; use the separate 'port' argument", k)}
	}
	if strings.ContainsAny(host, " /\\?#@%") {
		return nil, []error{fmt.Errorf("%q must be a valid hostname or IP address", k)}
	}

	return nil, nil
}

// proxyFromEnvironment wraps http.ProxyFromEnvironment so an invalid host that
// prevents easyssh-proxy from constructing an HTTP request does not panic with
// a nil pointer dereference.
func proxyFromEnvironment(req *http.Request) (*url.URL, error) {
	if req == nil || req.URL == nil {
		return nil, nil
	}
	return http.ProxyFromEnvironment(req)
}
