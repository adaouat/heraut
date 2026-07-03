package native

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

func gitlabLC() *port.LinkContext {
	return &port.LinkContext{Platform: "gitlab", BaseURL: "https://gitlab.com", Owner: "g", Repo: "p", Token: "tok"}
}

func TestEnrichGitLab_MapsMR(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"https://gitlab.com/g/p/-/merge_requests/7","author":{"username":"alice"}}]`, "", nil)
	mr.QueueResponse(`[{"iid":3}]`, "", nil) // alice's earliest merged MR predates this release → not a first-timer

	result, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, prInfo{Number: 7, URL: "https://gitlab.com/g/p/-/merge_requests/7", AuthorLogin: "alice", RefPrefix: "!"}, result["abc123"])

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, "glab", mr.Calls[0].Name)
	assert.Equal(t, []string{"api", "projects/g%2Fp/repository/commits/abc123/merge_requests"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"GITLAB_TOKEN=tok"}, mr.Calls[0].Env)
}

// TestEnrichGitLab_FirstTimer: the author's earliest merged MR is the one in this release, so
// they are a first-time contributor (FirstTimer=true). Asserts the exact earliest-MR argv/env.
func TestEnrichGitLab_FirstTimer(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"https://gitlab.com/g/p/-/merge_requests/7","author":{"username":"alice"}}]`, "", nil)
	mr.QueueResponse(`[{"iid":7}]`, "", nil) // alice's earliest merged MR == this one → first-timer

	result, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.NoError(t, err)
	require.True(t, result["abc123"].FirstTimer)
	assert.Equal(t, "!", result["abc123"].RefPrefix, "RefPrefix preserved")

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, "glab", mr.Calls[1].Name)
	assert.Equal(t, []string{"api", "projects/g%2Fp/merge_requests?author_username=alice&state=merged&order_by=created_at&sort=asc&per_page=1"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"GITLAB_TOKEN=tok"}, mr.Calls[1].Env)
}

// TestEnrichGitLab_ReturningContributor: an earlier merged MR exists → not a first-timer.
func TestEnrichGitLab_ReturningContributor(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"u","author":{"username":"alice"}}]`, "", nil)
	mr.QueueResponse(`[{"iid":2}]`, "", nil) // earlier merged MR (iid 2 < 7) → returning contributor

	result, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.NoError(t, err)
	assert.False(t, result["abc123"].FirstTimer)
}

// TestEnrichGitLab_FirstTimer_DistinctAuthorQueriedOnce: an author on multiple commits triggers
// a single first-timer query; the min MR iid across their release commits is the comparison base.
func TestEnrichGitLab_FirstTimer_DistinctAuthorQueriedOnce(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"u1","author":{"username":"alice"}}]`, "", nil) // sha1 alice
	mr.QueueResponse(`[{"iid":9,"web_url":"u2","author":{"username":"alice"}}]`, "", nil) // sha2 alice again
	mr.QueueResponse(`[{"iid":7}]`, "", nil)                                              // one query; earliest 7 == min(7,9) → first-timer

	result, err := enrichGitLab(mr, gitlabLC(), []string{"sha1", "sha2"})
	require.NoError(t, err)
	assert.True(t, result["sha1"].FirstTimer)
	assert.True(t, result["sha2"].FirstTimer)
	require.Len(t, mr.Calls, 3, "alice queried once despite two commits")
}

// TestEnrichGitLab_FirstTimer_MultiAuthor: distinct authors are queried in sorted order.
func TestEnrichGitLab_FirstTimer_MultiAuthor(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"ua","author":{"username":"alice"}}]`, "", nil) // sha1
	mr.QueueResponse(`[{"iid":8,"web_url":"ub","author":{"username":"bob"}}]`, "", nil)   // sha2
	mr.QueueResponse(`[{"iid":7}]`, "", nil)                                              // alice earliest 7 → first-timer
	mr.QueueResponse(`[{"iid":1}]`, "", nil)                                              // bob earliest 1 → returning

	result, err := enrichGitLab(mr, gitlabLC(), []string{"sha1", "sha2"})
	require.NoError(t, err)
	assert.True(t, result["sha1"].FirstTimer, "alice is a first-timer")
	assert.False(t, result["sha2"].FirstTimer, "bob is returning")

	require.Len(t, mr.Calls, 4)
	assert.Contains(t, mr.Calls[2].Args[1], "author_username=alice")
	assert.Contains(t, mr.Calls[3].Args[1], "author_username=bob")
}

// TestEnrichGitLab_FirstTimerQueryError: a failed earliest-MR query propagates so the
// remote_metadata policy can decide (required=fatal, optional=degrade).
func TestEnrichGitLab_FirstTimerQueryError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":7,"web_url":"u","author":{"username":"alice"}}]`, "", nil)
	mr.QueueResponse("", "403 Forbidden", errors.New("exit status 1"))

	_, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge_requests")
}

func TestEnrichGitLab_NoMR_Absent(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[]`, "", nil)

	result, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestEnrichGitLab_ErrorWrapped(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "401 Unauthorized", errors.New("exit status 1"))

	_, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge_requests")
}

func TestEnrichGitLab_MalformedJSON(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("not json", "", nil)

	_, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing glab api merge_requests response")
}

func TestEnrichGitLab_Subgroup(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse(`[{"iid":3,"web_url":"u","author":{"username":"bob"}}]`, "", nil)
	mr.QueueResponse(`[{"iid":1}]`, "", nil) // bob's earliest merged MR predates this release
	lc := &port.LinkContext{Platform: "gitlab", BaseURL: "https://gitlab.com", Owner: "group/subgroup", Repo: "project", Token: "tok"}

	_, err := enrichGitLab(mr, lc, []string{"deadbeef"})
	require.NoError(t, err)
	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"api", "projects/group%2Fsubgroup%2Fproject/repository/commits/deadbeef/merge_requests"}, mr.Calls[0].Args)
}

// End-to-end: GitLab release-notes enrichment renders "!N" (not "#N") and, when the author's
// first merged MR is in this release, the "New Contributors" block with a [!N] reference.
func TestGenerate_Enrich_GitLab(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)                                                                        // previousTag
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil)                                                          // tagDate
	mr.QueueResponse(record("abc1234567", "A", "a@example.com", "2026-01-02T00:00:00Z", "feat: x", ""), "", nil) // collectCommits
	mr.QueueResponse(`[{"iid":7,"web_url":"https://gitlab.com/g/p/-/merge_requests/7","author":{"username":"alice"}}]`, "", nil)
	mr.QueueResponse(`[{"iid":7}]`, "", nil) // alice's earliest merged MR == this one → first-timer
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", gitlabLC())
	require.NoError(t, err)
	assert.Contains(t, out, "by @alice in [!7](https://gitlab.com/g/p/-/merge_requests/7)")
	assert.Contains(t, out, "### New Contributors ❤️")
	assert.Contains(t, out, "* @alice made their first contribution in [!7](https://gitlab.com/g/p/-/merge_requests/7)")
}
