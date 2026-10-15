package gitlab

import (
	"net/http"
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitAuthors_PrefersEmailLocalPart(t *testing.T) {
	got := gitAuthors([]port.Commit{
		{Hash: "aaa", Author: "Alice Smith", Email: "alice@example.com"}, // local-part wins over the name
		{Hash: "bbb", Author: "Bob", Email: ""},                          // no email → git name
		{Hash: "ccc", Author: "", Email: "carol@example.com"},            // no name → local-part
		{Hash: "ddd", Author: "", Email: ""},                             // nothing → omitted
	})
	assert.Equal(t, map[string]string{
		"aaa": "alice",
		"bbb": "Bob",
		"ccc": "carol",
	}, got, "an @handle must not contain spaces when an email local-part is available")
}

// redirectDecision runs c's CheckRedirect for a redirect from the first URL to the second, with the
// token header set on the request, and returns the token header the redirected request keeps.
func redirectDecision(t *testing.T, c *http.Client, header, from, to string) string {
	t.Helper()
	first, err := http.NewRequest(http.MethodGet, from, nil)
	require.NoError(t, err)
	first.Header.Set(header, "secret-token")
	next, err := http.NewRequest(http.MethodGet, to, nil)
	require.NoError(t, err)
	next.Header = first.Header.Clone()
	require.NoError(t, c.CheckRedirect(next, []*http.Request{first}))
	return next.Header.Get(header)
}

func TestRedirectPolicy_HTTPSDowngradeDropsTheToken(t *testing.T) {
	c := withTokenSafeRedirects(&http.Client{})
	tests := []struct {
		name     string
		from, to string
		want     string
	}{
		{"same host and scheme keeps it", "https://gitlab.example.com/a", "https://gitlab.example.com/b", "secret-token"},
		{"https to http on the same host drops it", "https://gitlab.example.com/a", "http://gitlab.example.com/b", ""},
		{"another host drops it", "https://gitlab.example.com/a", "https://other.example.com/b", ""},
		{"http to https on the same host keeps it", "http://127.0.0.1/a", "https://127.0.0.1/b", "secret-token"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, redirectDecision(t, c, "PRIVATE-TOKEN", tc.from, tc.to))
		})
	}
}
