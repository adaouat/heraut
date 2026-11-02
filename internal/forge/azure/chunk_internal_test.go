package azure

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pullRequestQueryServer answers each pullrequestquery with one pull request per SHA asked, numbered
// from 1 in the order received, and records the SHAs of every request. failRequest > 0 answers
// that request (1-based) with a 500.
func pullRequestQueryServer(t *testing.T, failRequest int) (*httptest.Server, *[][]string) {
	t.Helper()
	var requests [][]string
	next := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var q prQuery
		require.NoError(t, json.Unmarshal(body, &q))
		requests = append(requests, q.Queries[0].Items)
		if len(requests) == failRequest {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		results := map[string][]azurePR{}
		for _, sha := range q.Queries[0].Items {
			next++
			results[sha] = []azurePR{{PullRequestID: next}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(prQueryResult{Results: []map[string][]azurePR{results}})
	}))
	t.Cleanup(srv.Close)
	return srv, &requests
}

func commitsN(n int) []port.Commit {
	commits := make([]port.Commit, n)
	for i := range commits {
		commits[i] = port.Commit{Hash: fmt.Sprintf("sha%04d", i)}
	}
	return commits
}

func TestEnrich_SplitsTheQueryIntoChunks(t *testing.T) {
	srv, requests := pullRequestQueryServer(t, 0)
	f := New(port.ForgeIdentity{Host: srv.URL, Project: "myorg/myproject", Repository: "myrepo"}, srv.Client())
	commits := commitsN(2*prQueryChunkSize + 1)

	en, err := f.Enrich(t.Context(), commits, "")
	require.NoError(t, err)

	require.Len(t, *requests, 3)
	var asked []string
	for _, r := range *requests {
		assert.LessOrEqual(t, len(r), prQueryChunkSize)
		asked = append(asked, r...)
	}
	want := make([]string, len(commits))
	for i, c := range commits {
		want[i] = c.Hash
	}
	assert.Equal(t, want, asked, "every SHA is asked once, in order")
	assert.Len(t, en.PRs, len(commits), "the chunks' results are merged")
}

func TestEnrich_AFailedChunkFailsTheEnrichment(t *testing.T) {
	srv, _ := pullRequestQueryServer(t, 2)
	f := New(port.ForgeIdentity{Host: srv.URL, Project: "myorg/myproject", Repository: "myrepo"}, srv.Client())

	_, err := f.Enrich(t.Context(), commitsN(prQueryChunkSize+1), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
