package gitlab_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/forge/gitlab"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForge_Links(t *testing.T) {
	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: "https://gitlab.example.com", Project: "group/subgroup/project",
	}, nil)
	assert.Equal(t, "gitlab", f.Type())
	assert.Equal(t, "https://gitlab.example.com/group/subgroup/project/-/commit/deadbeef", f.CommitURL("deadbeef"))
	assert.Equal(t, "https://gitlab.example.com/group/subgroup/project/-/merge_requests/42", f.ChangeURL(42))
	assert.Equal(t, "https://gitlab.example.com/group/subgroup/project/-/compare/v1.0.0...v1.1.0", f.CompareURL("v1.0.0", "v1.1.0"))
	assert.Equal(t, "https://gitlab.example.com/group/subgroup/project/-/compare/v1.0.0%2B1...v1.1.0%2B2", f.CompareURL("v1.0.0+1", "v1.1.0+2"))
}

// TestEnrichREST_JobToken is the epic's core scenario: a CI_JOB_TOKEN must be sent as JOB-TOKEN
// (GitLab rejects it as PRIVATE-TOKEN) against the job-token-allowed per-commit MR endpoint.
func TestEnrichREST_JobToken(t *testing.T) {
	var gotPath, gotJob, gotPrivate string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotJob, gotPrivate = r.URL.EscapedPath(), r.Header.Get("JOB-TOKEN"), r.Header.Get("PRIVATE-TOKEN")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"iid": 42, "web_url": "https://gitlab.example.com/group/project/-/merge_requests/42",
			"title": "Add widget", "labels": ["feature"],
			"author": {"username": "alice"},
			"created_at": "2026-07-01T10:00:00Z", "merged_at": "2026-07-02T11:00:00Z",
			"merged_by": {"username": "bob"}
		}]`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "jobtok", TokenKind: port.TokenJob, APIMode: "rest",
	}, srv.Client())

	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice", Email: "alice@example.com", Date: time.Now()}}, "")
	require.NoError(t, err)

	assert.Equal(t, "/api/v4/projects/group%2Fproject/repository/commits/abc123/merge_requests", gotPath)
	assert.Equal(t, "jobtok", gotJob, "a job token must be sent as JOB-TOKEN")
	assert.Empty(t, gotPrivate, "a job token must NOT be sent as PRIVATE-TOKEN")

	pr := en.PRs["abc123"]
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "!", pr.RefPrefix)
	assert.Equal(t, "Add widget", pr.Title)
	assert.Equal(t, "alice", pr.AuthorLogin)
	assert.Equal(t, []string{"feature"}, pr.Labels)
	assert.Equal(t, "bob", pr.MergedBy.Username)
	assert.False(t, pr.MergedAt.IsZero())
	// REST commits carry no linked handle: `by @` falls back to the git author email's local-part.
	assert.Equal(t, "alice", en.Authors["abc123"])
}

func TestEnrichREST_PrivateTokenHeader(t *testing.T) {
	var gotJob, gotPrivate string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotJob, gotPrivate = r.Header.Get("JOB-TOKEN"), r.Header.Get("PRIVATE-TOKEN")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "pat", TokenKind: port.TokenPrivate,
	}, srv.Client())
	_, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice"}}, "")
	require.NoError(t, err)
	assert.Equal(t, "pat", gotPrivate, "a PAT must be sent as PRIVATE-TOKEN")
	assert.Empty(t, gotJob)
}

func TestEnrichREST_NoMR_And_ErrorStatus(t *testing.T) {
	t.Run("no MR for commit", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`[]`))
		}))
		defer srv.Close()
		f := gitlab.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "g/p"}, srv.Client())
		en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice"}}, "")
		require.NoError(t, err)
		assert.Empty(t, en.PRs, "a commit with no MR yields no PR entry")
		assert.Equal(t, "Alice", en.Authors["abc123"], "author handle still resolves offline-style")
	})

	t.Run("non-2xx is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		f := gitlab.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "g/p"}, srv.Client())
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}}, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})
}

func TestEnrich_NoCommits(t *testing.T) {
	f := gitlab.New(port.ForgeIdentity{Host: "https://gitlab.example.com", Project: "g/p"}, nil)
	en, err := f.Enrich(nil, "")
	require.NoError(t, err)
	assert.Empty(t, en.PRs)
	assert.Empty(t, en.Authors)
}

// redirectTarget starts a second server that records the token headers it receives and answers
// with an empty MR list.
func redirectTarget(t *testing.T, seen *http.Header) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestEnrich_RedirectDropsTokenHeadersOnHostChange pins SEC-003: Go forwards custom headers on a
// redirect, so a redirect to another host would otherwise receive the GitLab token.
func TestEnrich_RedirectDropsTokenHeadersOnHostChange(t *testing.T) {
	tests := []struct {
		name   string
		kind   port.TokenKind
		header string
	}{
		{"job token", port.TokenJob, "JOB-TOKEN"},
		{"private token", port.TokenPrivate, "PRIVATE-TOKEN"},
	}
	for _, tc := range tests {
		for _, withClient := range []bool{false, true} {
			name := tc.name + "/default client"
			if withClient {
				name = tc.name + "/provided client"
			}
			t.Run(name, func(t *testing.T) {
				var seen http.Header
				other := redirectTarget(t, &seen)
				origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, other.URL+"/elsewhere", http.StatusFound)
				}))
				defer origin.Close()

				var client *http.Client
				if withClient {
					client = &http.Client{Timeout: 5 * time.Second}
				}
				f := gitlab.New(port.ForgeIdentity{
					Type: "gitlab", Host: origin.URL, APIURL: origin.URL + "/api/v4", Project: "group/project",
					Token: "secret-token", TokenKind: tc.kind, APIMode: "rest",
				}, client)

				_, err := f.Enrich([]port.Commit{{Hash: "abc123"}}, "")
				require.NoError(t, err)
				require.NotNil(t, seen, "the redirect target must have been reached")
				assert.Empty(t, seen.Get(tc.header), "the token must not follow a redirect to another host")
				assert.Equal(t, "application/json", seen.Get("Accept"), "other headers are still forwarded")
			})
		}
	}
}

func TestEnrich_RedirectKeepsTokenOnSameHost(t *testing.T) {
	var gotToken string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/moved", http.StatusFound)
	})
	mux.HandleFunc("/moved", func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("PRIVATE-TOKEN")
		_, _ = w.Write([]byte(`[]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "secret-token", TokenKind: port.TokenPrivate, APIMode: "rest",
	}, nil)
	_, err := f.Enrich([]port.Commit{{Hash: "abc123"}}, "")
	require.NoError(t, err)
	assert.Equal(t, "secret-token", gotToken, "a same-host redirect keeps the token")
}

func TestEnrich_RedirectLoopStopsAtNetHTTPDefaultLimit(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "secret-token", TokenKind: port.TokenPrivate, APIMode: "rest",
	}, nil)
	_, err := f.Enrich([]port.Commit{{Hash: "abc123"}}, "")
	require.Error(t, err)
	assert.Equal(t, 10, hits, "the limit net/http applies by default must survive a custom CheckRedirect")
}
