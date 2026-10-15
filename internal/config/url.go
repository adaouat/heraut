package config

import (
	"net"
	"net/url"
	"strings"
)

// IsSecureURL reports whether raw is an absolute URL a credential may be sent to: https, or http
// to a loopback host (a local server or a test double). Any other plain-http host would expose the
// forge token on the wire, or let a changed base_url aim it at an attacker.
func IsSecureURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		return isLoopbackHost(u.Hostname())
	}
	return false
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
