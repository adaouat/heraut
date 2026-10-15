package config_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestIsSecureURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"https host", "https://gitlab.example.com", true},
		{"https with path", "https://gitlab.example.com/api/v4", true},
		{"https upper-case scheme", "HTTPS://gitlab.example.com", true},
		{"http remote host", "http://gitlab.example.com", false},
		{"http remote host with port", "http://gitlab.example.com:8080", false},
		{"http localhost", "http://localhost:8080", true},
		{"http IPv4 loopback", "http://127.0.0.1:34567", true},
		{"http other 127/8 address", "http://127.1.2.3", true},
		{"http IPv6 loopback", "http://[::1]:8080", true},
		{"http name that only starts with localhost", "http://localhost.evil.example", false},
		{"http private address is not loopback", "http://10.0.0.5", false},
		{"other scheme", "ftp://gitlab.example.com", false},
		{"no scheme", "gitlab.example.com", false},
		{"no host", "https://", false},
		{"empty", "", false},
		{"not a url", "not a url", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, config.IsSecureURL(tc.url))
		})
	}
}
