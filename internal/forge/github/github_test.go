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

	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice", Email: "alice@example.com", Date: time.Now()}})
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
	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice"}})
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
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Bad credentials")
	})

	t.Run("non-2xx status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		f := github.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL, Project: "acme/widget"}, srv.Client())
		_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})
}

// More than one chunk's worth of SHAs must be fetched in multiple batched queries, and results
// merged — the legacy driver chunked at 50 to stay within GitHub's node limits.
func TestEnrich_ChunksLargeCommitSets(t *testing.T) {
	var queries int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"s0":{
			"author":{"user":{"login":"alice"}},"associatedPullRequests":{"nodes":[]}}}}}`))
	}))
	defer srv.Close()

	commits := make([]port.Commit, 0, 51)
	for i := 0; i < 51; i++ {
		commits = append(commits, port.Commit{Hash: fmt.Sprintf("sha%02d", i), Author: "Alice"})
	}
	f := github.New(port.ForgeIdentity{Host: srv.URL, APIURL: srv.URL, Project: "acme/widget"}, srv.Client())
	_, err := f.Enrich(commits)
	require.NoError(t, err)
	assert.Equal(t, 2, queries, "51 SHAs must be split into 2 chunks of at most 50")
}

func TestEnrich_NoCommits(t *testing.T) {
	f := github.New(port.ForgeIdentity{Host: "https://github.com", Project: "acme/widget"}, nil)
	en, err := f.Enrich(nil)
	require.NoError(t, err)
	assert.Empty(t, en.PRs)
	assert.Empty(t, en.Authors)
}
