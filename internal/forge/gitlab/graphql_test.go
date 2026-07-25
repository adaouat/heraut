package gitlab_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adaouat/heraut/internal/forge/gitlab"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A job token can never authenticate GraphQL — fail fast, with no request issued.
func TestEnrichGraphQL_JobTokenRejected(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "jobtok", TokenKind: port.TokenJob, APIMode: "graphql",
	}, srv.Client())

	_, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice"}})
	require.Error(t, err)
	assert.True(t, errors.Is(err, gitlab.ErrJobTokenGraphQL))
	assert.Contains(t, err.Error(), "api_mode: rest", "the error must point at the fix")
	assert.False(t, called, "no network call may be made when the guard trips")
}

func TestEnrichGraphQL_LinkedUsernameAndHeader(t *testing.T) {
	var gotPrivate, gotJob string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrivate, gotJob = r.Header.Get("PRIVATE-TOKEN"), r.Header.Get("JOB-TOKEN")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"project":{
			"repository":{"commits":{"nodes":[{"sha":"abc123","author":{"username":"alice-gl"}}]}},
			"mergeRequests":{"nodes":[{
				"iid":"42","webUrl":"https://gitlab.example.com/group/project/-/merge_requests/42",
				"title":"Add widget","author":{"username":"alice-gl"},
				"createdAt":"2026-07-01T10:00:00Z","mergedAt":"2026-07-02T11:00:00Z",
				"mergeUser":{"username":"bob-gl"},
				"labels":{"nodes":[{"title":"feature"}]},
				"mergeCommitSha":"abc123","commits":{"nodes":[]}
			}]}
		}}}`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Type: "gitlab", Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "pat", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())

	en, err := f.Enrich([]port.Commit{{Hash: "abc123", Author: "Alice", Email: "alice@example.com"}})
	require.NoError(t, err)

	assert.Equal(t, "pat", gotPrivate, "GraphQL uses PRIVATE-TOKEN")
	assert.Empty(t, gotJob)
	// GraphQL's whole advantage: the LINKED handle, not the local git name.
	assert.Equal(t, "alice-gl", en.Authors["abc123"])
	pr := en.PRs["abc123"]
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "!", pr.RefPrefix)
	assert.Equal(t, "Add widget", pr.Title)
	assert.Equal(t, []string{"feature"}, pr.Labels)
	assert.Equal(t, "bob-gl", pr.MergedBy.Username)
}

func TestEnrichGraphQL_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"insufficient scope"}]}`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/project",
		Token: "pat", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())
	_, err := f.Enrich([]port.Commit{{Hash: "abc123"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient scope")
}
