package native

import (
	"errors"
	"fmt"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/port"
)

// makeGitHubLC returns a test LinkContext for github.com with the given credentials.
func makeGitHubLC(owner, repo, token string) *port.LinkContext {
	return &port.LinkContext{
		BaseURL:  "https://github.com",
		Owner:    owner,
		Repo:     repo,
		Platform: "github",
		Token:    token,
	}
}

func TestBuildGitHubQuery(t *testing.T) {
	got := buildGitHubQuery("myorg", "myrepo", []string{"abc123", "def456"})
	want := `{repository(owner:"myorg",name:"myrepo"){` +
		`s0:object(oid:"abc123"){` + prFragment + `}` +
		`s1:object(oid:"def456"){` + prFragment + `}` +
		`}}`
	assert.Equal(t, want, got)
}

func TestEnrichGitHub_TwoCommitsWithPRs(t *testing.T) {
	mr := exectest.NewMockRunner()
	sha1 := "aabbccddee1122334455667788990011aabbccdd"
	sha2 := "bbccddee1122334455667788990011aabbccddee"
	lc := makeGitHubLC("owner", "repo", "my-token")

	mr.QueueResponse(`{
		"data": {
			"repository": {
				"s0": {"associatedPullRequests": {"nodes": [
					{"number": 42, "url": "https://github.com/owner/repo/pull/42",
					 "author": {"login": "alice"}, "authorAssociation": "CONTRIBUTOR"}
				]}},
				"s1": {"associatedPullRequests": {"nodes": [
					{"number": 43, "url": "https://github.com/owner/repo/pull/43",
					 "author": {"login": "bob"}, "authorAssociation": "FIRST_TIME_CONTRIBUTOR"}
				]}}
			}
		}
	}`, "", nil)

	result, err := enrichGitHub(mr, lc, []string{sha1, sha2})
	require.NoError(t, err)
	require.Len(t, result, 2)

	pi1 := result[sha1]
	assert.Equal(t, 42, pi1.Number)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", pi1.URL)
	assert.Equal(t, "alice", pi1.AuthorLogin)
	assert.False(t, pi1.FirstTimer)

	pi2 := result[sha2]
	assert.Equal(t, 43, pi2.Number)
	assert.Equal(t, "bob", pi2.AuthorLogin)
	assert.True(t, pi2.FirstTimer)

	// Contract: exact argv and env (single batched call).
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "gh", mr.Calls[0].Name)
	assert.Equal(t, []string{"api", "graphql", "-f",
		"query=" + buildGitHubQuery("owner", "repo", []string{sha1, sha2}),
	}, mr.Calls[0].Args)
	assert.Equal(t, []string{"GH_TOKEN=my-token"}, mr.Calls[0].Env)
}

func TestEnrichGitHub_CommitNoPR(t *testing.T) {
	mr := exectest.NewMockRunner()
	sha := "ccddee1122334455667788990011aabbccddeeff"
	lc := makeGitHubLC("owner", "repo", "tok")

	mr.QueueResponse(`{"data":{"repository":{"s0":{"associatedPullRequests":{"nodes":[]}}}}}`, "", nil)

	result, err := enrichGitHub(mr, lc, []string{sha})
	require.NoError(t, err)
	assert.Empty(t, result, "commit with no associated PR must be absent from the map")
}

func TestEnrichGitHub_Chunking(t *testing.T) {
	mr := exectest.NewMockRunner()
	lc := makeGitHubLC("owner", "repo", "tok")

	// 51 SHAs → two chunks (50 + 1).
	shas := make([]string, 51)
	for i := range shas {
		shas[i] = fmt.Sprintf("%040x", i+1)
	}

	// First chunk (50 SHAs): only shas[0] has a PR; absent aliases are treated as no-PR.
	mr.QueueResponse(`{"data":{"repository":{`+
		`"s0":{"associatedPullRequests":{"nodes":[{"number":1,"url":"https://github.com/owner/repo/pull/1","author":{"login":"alice"},"authorAssociation":"CONTRIBUTOR"}]}}`+
		`}}}`, "", nil)
	// Second chunk (1 SHA): shas[50] has PR #51 (aliased as s0 in its own chunk).
	mr.QueueResponse(`{"data":{"repository":{`+
		`"s0":{"associatedPullRequests":{"nodes":[{"number":51,"url":"https://github.com/owner/repo/pull/51","author":{"login":"carol"},"authorAssociation":"CONTRIBUTOR"}]}}`+
		`}}}`, "", nil)

	result, err := enrichGitHub(mr, lc, shas)
	require.NoError(t, err)

	assert.Len(t, mr.Calls, 2, "exactly two gh api graphql calls for 51 SHAs")
	assert.Contains(t, result, shas[0], "shas[0] PR from first chunk")
	assert.Contains(t, result, shas[50], "shas[50] PR from second chunk")
	assert.Len(t, result, 2, "only two commits had PRs")
}

func TestEnrichGitHub_GhError(t *testing.T) {
	mr := exectest.NewMockRunner()
	lc := makeGitHubLC("owner", "repo", "tok")
	mr.QueueResponse("", "error from gh", errors.New("exit status 1"))

	_, err := enrichGitHub(mr, lc, []string{"deadbeef1234567890abcdef1234567890abcdef"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh api graphql")
}

func TestEnrichGitHub_MalformedJSON(t *testing.T) {
	mr := exectest.NewMockRunner()
	lc := makeGitHubLC("owner", "repo", "tok")
	mr.QueueResponse("not-valid-json", "", nil)

	_, err := enrichGitHub(mr, lc, []string{"deadbeef1234567890abcdef1234567890abcdef"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing")
}
