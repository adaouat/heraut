package gitlab_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// The release window must bound both connections: an unbounded mergeRequests(first:100) returns
// whatever GitLab orders first, which on a long-lived project can exclude the release's own MRs.
func TestEnrichGraphQL_SendsReleaseWindowBounds(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"project":{
			"repository":{"commits":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":false}}},
			"mergeRequests":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":false}}
		}}}`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/subgroup/project",
		Token: "pat", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())

	oldest := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	_, err := f.Enrich([]port.Commit{
		{Hash: "aaa", Author: "Alice", Date: oldest},
		{Hash: "bbb", Author: "Bob", Date: oldest.Add(48 * time.Hour)},
	})
	require.NoError(t, err)

	all := strings.Join(bodies, "\n")
	assert.Contains(t, all, "committedAfter", "commits must be bounded to the release window")
	assert.Contains(t, all, "mergedAfter", "merged MRs must be bounded to the release window")
	// The bound is the OLDEST commit (minus a small buffer), not the newest.
	assert.Contains(t, all, "2026-07-01T11:59:00Z")
}

// Both connections must follow cursors until exhausted, so a release with >100 commits/MRs
// resolves fully instead of silently truncating at the first page.
func TestEnrichGraphQL_PaginatesBothConnections(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		w.Header().Set("Content-Type", "application/json")
		switch {
		// commits, page 1 → one more page
		case strings.Contains(body, "commits(") && !strings.Contains(body, "CURSOR-C"):
			_, _ = w.Write([]byte(`{"data":{"project":{"repository":{"commits":{
				"nodes":[{"sha":"aaa","author":{"username":"alice-gl"}}],
				"pageInfo":{"endCursor":"CURSOR-C","hasNextPage":true}}}}}}`))
		// commits, page 2 → done
		case strings.Contains(body, "commits("):
			_, _ = w.Write([]byte(`{"data":{"project":{"repository":{"commits":{
				"nodes":[{"sha":"bbb","author":{"username":"bob-gl"}}],
				"pageInfo":{"endCursor":"","hasNextPage":false}}}}}}`))
		// MRs, page 1 → one more page
		case !strings.Contains(body, "CURSOR-M"):
			_, _ = w.Write([]byte(`{"data":{"project":{"mergeRequests":{
				"nodes":[{"iid":"1","title":"first","mergeCommitSha":"aaa","commits":{"nodes":[]},
				          "labels":{"nodes":[]},"author":{"username":"alice-gl"}}],
				"pageInfo":{"endCursor":"CURSOR-M","hasNextPage":true}}}}}`))
		// MRs, page 2 → done
		default:
			_, _ = w.Write([]byte(`{"data":{"project":{"mergeRequests":{
				"nodes":[{"iid":"2","title":"second","mergeCommitSha":"bbb","commits":{"nodes":[]},
				          "labels":{"nodes":[]},"author":{"username":"bob-gl"}}],
				"pageInfo":{"endCursor":"","hasNextPage":false}}}}}`))
		}
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/subgroup/project",
		Token: "pat", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())

	en, err := f.Enrich([]port.Commit{
		{Hash: "aaa", Author: "Alice", Date: time.Now().Add(-48 * time.Hour)},
		{Hash: "bbb", Author: "Bob", Date: time.Now()},
	})
	require.NoError(t, err)

	assert.Equal(t, "alice-gl", en.Authors["aaa"], "page 1 authors")
	assert.Equal(t, "bob-gl", en.Authors["bbb"], "page 2 authors must not be lost to truncation")
	assert.Equal(t, 1, en.PRs["aaa"].Number)
	assert.Equal(t, 2, en.PRs["bbb"].Number, "page 2 MRs must not be lost to truncation")
	assert.GreaterOrEqual(t, calls, 4, "both connections paginate")
}

// A malformed hasNextPage:true with no cursor must stop, not refetch page 1 forever.
func TestEnrichGraphQL_StopsOnEmptyCursor(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"project":{
			"repository":{"commits":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":true}}},
			"mergeRequests":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":true}}
		}}}`))
	}))
	defer srv.Close()

	f := gitlab.New(port.ForgeIdentity{
		Host: srv.URL, APIURL: srv.URL + "/api/v4", Project: "group/subgroup/project",
		Token: "pat", TokenKind: port.TokenPrivate, APIMode: "graphql",
	}, srv.Client())

	done := make(chan struct{})
	go func() {
		_, _ = f.Enrich([]port.Commit{{Hash: "aaa", Author: "Alice", Date: time.Now()}})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Enrich did not terminate on an empty cursor — infinite pagination loop")
	}
	assert.Less(t, calls, 10, "must stop promptly, not spin")
}
