package pipeline

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommitChangelog_StagedCommits verifies the normal path: add stages the file,
// the staged-check sees a change, and commit + push run. Reports committed=true.
func TestCommitChangelog_StagedCommits(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git commit
	mr.QueueResponse("", "", nil)               // git push

	g := gitHelper{runner: mr}
	committed, err := g.commitChangelog("CHANGELOG.md", "chore(release): 1.2.3", true)
	require.NoError(t, err)
	assert.True(t, committed)

	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[1].Args)
	assert.Equal(t, "commit", mr.Calls[2].Args[0])
	assert.Equal(t, []string{"push", "origin", "HEAD"}, mr.Calls[3].Args)
}

// TestCommitChangelog_NothingStagedSkips verifies that when `git add` stages nothing
// (the changelog is byte-identical to the last commit), commit and push are skipped
// and committed=false is reported without error.
func TestCommitChangelog_NothingStagedSkips(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git add
	mr.QueueResponse("", "", nil) // git diff --cached --name-only (empty: nothing staged)

	g := gitHelper{runner: mr}
	committed, err := g.commitChangelog("CHANGELOG.md", "chore(release): 1.2.3", true)
	require.NoError(t, err)
	assert.False(t, committed)

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[1].Args)
}

// TestCommitChangelog_DiffError propagates a genuine `git diff --cached` failure
// rather than misreading it as "nothing staged".
func TestCommitChangelog_DiffError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)                                 // git add
	mr.QueueResponse("", "", errors.New("fatal: not a git repo")) // git diff fails

	g := gitHelper{runner: mr}
	committed, err := g.commitChangelog("CHANGELOG.md", "chore(release): 1.2.3", true)
	require.Error(t, err)
	assert.False(t, committed)
	assert.Contains(t, err.Error(), "git diff --cached")

	require.Len(t, mr.Calls, 2)
}
