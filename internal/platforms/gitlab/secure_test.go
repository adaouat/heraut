package gitlab

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSameGitLabHost(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"same scheme and host", "https://gitlab.example.com", "https://gitlab.example.com", true},
		{"trailing slash ignored", "https://gitlab.example.com/", "https://gitlab.example.com", true},
		{"scheme case ignored", "HTTPS://gitlab.example.com", "https://gitlab.example.com", true},
		{"host case ignored", "https://GitLab.Example.com", "https://gitlab.example.com", true},
		{"different scheme", "http://gitlab.example.com", "https://gitlab.example.com", false},
		{"different host", "https://gitlab.example.com", "https://other.example.com", false},
		{"different port", "https://gitlab.example.com:8443", "https://gitlab.example.com", false},
		{"missing side", "https://gitlab.example.com", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sameGitLabHost(tc.a, tc.b))
		})
	}
}

func TestNewClient_RefusesPlainHTTPToARemoteHost(t *testing.T) {
	for _, jobToken := range []bool{false, true} {
		_, err := NewClient("http://gitlab.example.com", "secret-token", jobToken)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "https")
	}
	_, err := NewClient("http://127.0.0.1:8080", "secret-token", false)
	assert.NoError(t, err, "loopback over http stays allowed")
}

func TestNewClient_RedirectPolicyDropsTheTokenOnHTTPSDowngrade(t *testing.T) {
	c, err := NewClient("https://gitlab.example.com", "secret-token", false)
	require.NoError(t, err)
	check := c.api.HTTPClient().CheckRedirect

	decide := func(from, to string) string {
		first, err := http.NewRequest(http.MethodGet, from, nil)
		require.NoError(t, err)
		first.Header.Set("Private-Token", "secret-token")
		next, err := http.NewRequest(http.MethodGet, to, nil)
		require.NoError(t, err)
		next.Header = first.Header.Clone()
		require.NoError(t, check(next, []*http.Request{first}))
		return next.Header.Get("Private-Token")
	}
	assert.Equal(t, "secret-token", decide("https://gitlab.example.com/a", "https://gitlab.example.com/b"))
	assert.Empty(t, decide("https://gitlab.example.com/a", "http://gitlab.example.com/b"))
	assert.Empty(t, decide("https://gitlab.example.com/a", "https://other.example.com/b"))
}
