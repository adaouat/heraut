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

// TestCommitChangelog_UsesInteractiveRunnerForCommit covers T260 (forge v0.19.0's CmdRunner.
// Interactive mode): `git commit` is the one call that can trigger a GPG pinentry prompt, so it
// must run through the interactive runner (stdin/stdout/stderr connected directly to the
// terminal) when one is configured — every other call (add, diff --cached, push) keeps using the
// regular, captured-output runner, since Interactive mode returns empty stdout/stderr and those
// calls depend on what they capture.
func TestCommitChangelog_UsesInteractiveRunnerForCommit(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)               // git add
	mr.QueueResponse("CHANGELOG.md\n", "", nil) // git diff --cached --name-only (staged)
	mr.QueueResponse("", "", nil)               // git push

	interactive := exectest.NewMockRunner()
	interactive.QueueResponse("", "", nil) // git commit

	g := gitHelper{runner: mr, interactiveRunner: interactive}
	committed, err := g.commitChangelog("CHANGELOG.md", "chore(release): 1.2.3", true)
	require.NoError(t, err)
	assert.True(t, committed)

	require.Len(t, mr.Calls, 3, "add/diff/push go through the regular runner")
	assert.Equal(t, []string{"add", "CHANGELOG.md"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"push", "origin", "HEAD"}, mr.Calls[2].Args)
	require.Len(t, interactive.Calls, 1, "commit goes through the interactive runner")
	assert.Equal(t, "commit", interactive.Calls[0].Args[0])
}

// TestCommitChangelog_NoInteractiveRunnerFallsBackToRegular verifies that when no interactive
// runner is configured (the zero value — every existing gitHelper{runner: mr} test construction),
// the commit call falls back to the regular runner exactly as it always has. This is what keeps
// every pre-existing test in this file passing unchanged.
func TestCommitChangelog_NoInteractiveRunnerFallsBackToRegular(t *testing.T) {
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
}

// TestTag_Signed_UsesInteractiveRunner covers T260: `git tag -s` also invokes GPG, so it needs the
// same interactive-runner treatment as commit.
func TestTag_Signed_UsesInteractiveRunner(t *testing.T) {
	mr := exectest.NewMockRunner()
	interactive := exectest.NewMockRunner()
	interactive.QueueResponse("", "", nil) // git tag -s

	g := gitHelper{runner: mr, interactiveRunner: interactive}
	err := g.tag("v1.2.3", "chore(release): 1.2.3", true, true)
	require.NoError(t, err)

	assert.Empty(t, mr.Calls, "a signed tag never touches the regular runner")
	require.Len(t, interactive.Calls, 1)
	assert.Equal(t, []string{"tag", "-s", "v1.2.3", "-m", "chore(release): 1.2.3"}, interactive.Calls[0].Args)
}

// TestTag_Unsigned_UsesRegularRunner verifies an unsigned (annotated) tag — no GPG involved —
// keeps using the regular runner even when an interactive one is configured.
func TestTag_Unsigned_UsesRegularRunner(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)
	interactive := exectest.NewMockRunner()

	g := gitHelper{runner: mr, interactiveRunner: interactive}
	err := g.tag("v1.2.3", "chore(release): 1.2.3", true, false)
	require.NoError(t, err)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, []string{"tag", "-a", "v1.2.3", "-m", "chore(release): 1.2.3"}, mr.Calls[0].Args)
	assert.Empty(t, interactive.Calls)
}
