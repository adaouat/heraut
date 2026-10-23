package github

import (
	"net/http"
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redirectDecision runs c's CheckRedirect for a redirect from the first URL to the second, with the
// Authorization header set on the request, and returns the header the redirected request keeps.
func redirectDecision(t *testing.T, c *http.Client, from, to string) string {
	t.Helper()
	first, err := http.NewRequest(http.MethodGet, from, nil)
	require.NoError(t, err)
	first.Header.Set("Authorization", "secret-token")
	next, err := http.NewRequest(http.MethodGet, to, nil)
	require.NoError(t, err)
	next.Header = first.Header.Clone()
	require.NoError(t, c.CheckRedirect(next, []*http.Request{first}))
	return next.Header.Get("Authorization")
}

func TestRedirectPolicy_HTTPSDowngradeDropsTheToken(t *testing.T) {
	c := New(port.ForgeIdentity{}, nil).client
	tests := []struct {
		name     string
		from, to string
		want     string
	}{
		{"same host and scheme keeps it", "https://github.example.com/a", "https://github.example.com/b", "secret-token"},
		{"https to http on the same host drops it", "https://github.example.com/a", "http://github.example.com/b", ""},
		{"another host drops it", "https://github.example.com/a", "https://other.example.com/b", ""},
		{"http to https on the same host keeps it", "http://127.0.0.1/a", "https://127.0.0.1/b", "secret-token"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, redirectDecision(t, c, tc.from, tc.to))
		})
	}
}

func TestRedirectPolicy_StopsAfterTenRedirects(t *testing.T) {
	c := New(port.ForgeIdentity{}, nil).client
	req, err := http.NewRequest(http.MethodGet, "https://github.example.com/x", nil)
	require.NoError(t, err)
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = req
	}
	assert.Error(t, c.CheckRedirect(req, via))
}
