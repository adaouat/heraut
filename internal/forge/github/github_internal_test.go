package github

import (
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
)

// graphqlEndpoint must land on the exact GraphQL path for every identity shape GitHub can present:
// github.com routes through api.github.com; GitHub Enterprise Server serves GraphQL at
// {host}/api/graphql — a sibling of /api/v3 (REST), never nested under it. GITHUB_API_URL on GHES
// is "{host}/api/v3" and must normalize to the same {host}/api/graphql, not {host}/api/v3/graphql.
func TestGraphqlEndpoint(t *testing.T) {
	tests := []struct {
		name string
		id   port.ForgeIdentity
		want string
	}{
		{
			name: "github.com host-only",
			id:   port.ForgeIdentity{Host: "https://github.com"},
			want: "https://api.github.com/graphql",
		},
		{
			name: "github.com explicit APIURL",
			id:   port.ForgeIdentity{Host: "https://github.com", APIURL: "https://api.github.com"},
			want: "https://api.github.com/graphql",
		},
		{
			name: "GHES host-only",
			id:   port.ForgeIdentity{Host: "https://github.example.com"},
			want: "https://github.example.com/api/graphql",
		},
		{
			name: "GHES explicit APIURL (GITHUB_API_URL style, .../api/v3)",
			id:   port.ForgeIdentity{Host: "https://github.example.com", APIURL: "https://github.example.com/api/v3"},
			want: "https://github.example.com/api/graphql",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &Forge{id: tc.id}
			assert.Equal(t, tc.want, f.graphqlEndpoint())
		})
	}
}

// I2: pins the generated GraphQL query byte-for-byte — the aliasing scheme (s0, s1, …), the
// object(oid:) selector, and prFragment's placement — so a malformed query silently passing an
// httptest server (which accepts any body) can't ship unnoticed again.
func TestBuildGitHubQuery(t *testing.T) {
	got := buildGitHubQuery("myorg", "myrepo", []string{"abc123", "def456"})
	want := `{repository(owner:"myorg",name:"myrepo"){` +
		`s0:object(oid:"abc123"){` + prFragment + `}` +
		`s1:object(oid:"def456"){` + prFragment + `}` +
		`}}`
	assert.Equal(t, want, got)
}
