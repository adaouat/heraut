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

	result, err := enrichGitLab(mr, gitlabLC(), []string{"abc123"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, prInfo{Number: 7, URL: "https://gitlab.com/g/p/-/merge_requests/7", AuthorLogin: "alice", RefPrefix: "!"}, result["abc123"])

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "glab", mr.Calls[0].Name)
	assert.Equal(t, []string{"api", "projects/g%2Fp/repository/commits/abc123/merge_requests"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"GITLAB_TOKEN=tok"}, mr.Calls[0].Env)
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

// End-to-end: GitLab release-notes enrichment renders "!N" (not "#N").
func TestGenerate_Enrich_GitLab(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)                                                                        // previousTag
	mr.QueueResponse(record("abc1234567", "A", "a@example.com", "2026-01-02T00:00:00Z", "feat: x", ""), "", nil) // collectCommits
	mr.QueueResponse(`[{"iid":7,"web_url":"https://gitlab.com/g/p/-/merge_requests/7","author":{"username":"alice"}}]`, "", nil)
	g := New(mr, &config.ContentDriver{Generator: "native", RemoteMetadata: "optional"}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", gitlabLC())
	require.NoError(t, err)
	assert.Contains(t, out, "by @alice in [!7](https://gitlab.com/g/p/-/merge_requests/7)")
}
