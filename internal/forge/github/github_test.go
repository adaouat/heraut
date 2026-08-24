package github_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/forge/github"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForge_Links(t *testing.T) {
	f := github.New(port.ForgeIdentity{
		Type: "github", Host: "https://github.com", Project: "acme/widget",
	}, nil)
	assert.Equal(t, "github", f.Type())
	assert.Equal(t, "https://github.com/acme/widget/commit/deadbeef", f.CommitURL("deadbeef"))
	assert.Equal(t, "https://github.com/acme/widget/pull/42", f.ChangeURL(42))
	assert.Equal(t, "https://github.com/acme/widget/compare/v1.0.0...v1.1.0", f.CompareURL("v1.0.0", "v1.1.0"))
}

func TestEnrich_MapsPRsAndAuthors(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"s0":{
			"author":{"user":{"login":"alice"}},
			"associatedPullRequests":{"nodes":[{
				"number":42,"url":"https://github.com/acme/widget/pull/42","title":"Add widget",
				"author":{"login":"alice"},
				"labels":{"nodes":[{"name":"feature"}]},
				"createdAt":"2026-07-01T10:00:00Z","mergedAt":"2026-07-02T11:00:00Z",
				"mergedBy":{"login":"bob"},
				"latestReviews":{"nodes":[{"state":"APPROVED","author":{"login":"carol"}}]}
			}]}
		}}}}`))
	}))
	defer srv.Close()

	f := github.New(port.ForgeIdentity{
		Type: "github", Host: srv.URL, APIURL: srv.URL, Project: "acme/widget",
		Token: "ghtok", TokenKind: port.TokenPrivate,
	}, srv.Client())

	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice", Email: "alice@example.com", Date: time.Now()}}, "")
	require.NoError(t, err)

	assert.Equal(t, "bearer ghtok", gotAuth, "GitHub GraphQL authenticates with a bearer token")
	assert.Contains(t, gotBody, "abc123", "the commit SHA is queried")

	// GitHub resolves the LINKED commit-author handle (unlike GitLab REST / Azure).
	assert.Equal(t, "alice", en.Authors["abc123"])
	pr := en.PRs["abc123"]
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "#", pr.RefPrefix)
	assert.Equal(t, "Add widget", pr.Title)
	assert.Equal(t, []string{"feature"}, pr.Labels)
	assert.Equal(t, "bob", pr.MergedBy.Username)
	require.Len(t, pr.Approvers, 1)
	assert.Equal(t, "carol", pr.Approvers[0].Username)
}

// A commit whose author email isn't linked to a GitHub account yields no handle, and a commit
// with no associated PR yields no PR entry — neither is an error.
func TestEnrich_UnlinkedAuthorAndNoPR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"repository":{"s0":{
			"author":{"user":null},"associatedPullRequests":{"nodes":[]}}}}}`))
	}))
	defer srv.Close()

	f := github.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL, Project: "acme/widget"}, srv.Client())
	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice"}}, "")
	require.NoError(t, err)
	assert.Empty(t, en.Authors)
	assert.Empty(t, en.PRs)
}

func TestEnrich_GraphQLErrorAndStatus(t *testing.T) {
	t.Run("api error in body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"errors":[{"message":"Bad credentials"}]}`))
		}))
		defer srv.Close()
		f := github.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL, Project: "acme/widget"}, srv.Client())
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}}, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Bad credentials")
	})

	t.Run("non-2xx status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		f := github.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL, Project: "acme/widget"}, srv.Client())
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}}, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})
}

// More than one chunk's worth of SHAs must be fetched in multiple batched queries, and results
// merged — the legacy driver chunked at 50 to stay within GitHub's node limits. Aliases restart at
// s0 in each chunk, so a merge bug (e.g. the second chunk's response overwriting or shadowing the
// first's) hides exactly there: each request here returns a DIFFERENT PR number under the same s0
// alias, and both the first-chunk SHA (sha00, alias s0 of request 1) and the second-chunk SHA
// (sha50, alias s0 of request 2) must resolve to their own distinct, correct PR in the merged map.
func TestEnrich_ChunksLargeCommitSets(t *testing.T) {
	var queries int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries++
		prNumber := 100 + queries // request 1 → 101, request 2 → 102: distinct per chunk
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"data":{"repository":{"s0":{
			"author":{"user":{"login":"alice"}},
			"associatedPullRequests":{"nodes":[{"number":%d,"url":"https://github.com/acme/widget/pull/%d","title":"chunk %d"}]}}}}}`,
			prNumber, prNumber, queries)
	}))
	defer srv.Close()

	commits := make([]port.Commit, 0, 51)
	for i := 0; i < 51; i++ {
		commits = append(commits, port.Commit{Hash: fmt.Sprintf("sha%02d", i), Author: "Alice"})
	}
	f := github.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL, Project: "acme/widget"}, srv.Client())
	en, err := f.Enrich(commits, "")
	require.NoError(t, err)
	assert.Equal(t, 2, queries, "51 SHAs must be split into 2 chunks of at most 50")

	require.Contains(t, en.PRs, "sha00", "first-chunk SHA (alias s0 of request 1) must resolve")
	assert.Equal(t, 101, en.PRs["sha00"].Number, "first-chunk SHA must resolve to the FIRST request's PR, not the second's")
	require.Contains(t, en.PRs, "sha50", "second-chunk SHA (alias s0 of request 2) must resolve")
	assert.Equal(t, 102, en.PRs["sha50"].Number, "second-chunk SHA must resolve to the SECOND request's PR, not the first's")
}

func TestEnrich_NoCommits(t *testing.T) {
	f := github.New(port.ForgeIdentity{Host: "https://github.com", Project: "acme/widget"}, nil)
	en, err := f.Enrich(nil, "")
	require.NoError(t, err)
	assert.Empty(t, en.PRs)
	assert.Empty(t, en.Authors)
}

// The GraphQL endpoint must land on the correct path for a GHES identity carrying an explicit
// APIURL, as GitHub Actions sets GITHUB_API_URL on GHES runners: "{host}/api/v3". GitHub
// Enterprise Server serves GraphQL at {host}/api/graphql — a sibling of /api/v3 (REST), not nested
// under it. A httptest server can't stand in for github.com itself (apiBase special-cases that
// literal host string), so the github.com/host-only shapes are pinned in the internal,
// white-box TestGraphqlEndpoint instead.
func TestEnrich_GraphQLEndpointGHESExplicitAPIURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"s0":{"author":{"user":null},"associatedPullRequests":{"nodes":[]}}}}}`))
	}))
	defer srv.Close()

	f := github.New(port.ForgeIdentity{
		Host: "https://github.example.com", APIURL: srv.URL + "/api/v3", Project: "acme/widget",
	}, srv.Client())
	_, err := f.Enrich([]port.Commit{{Hash: "abc123"}}, "")
	require.NoError(t, err)
	assert.Equal(t, "/api/graphql", gotPath)
}
