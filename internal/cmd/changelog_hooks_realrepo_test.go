package cmd_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChangelog_RealGit_HooksExecuteInOrderWithTemplating is T272: a real-git-repo proof that
// pre_tag/post_tag hooks actually execute — not just that MockRunner recorded the expected
// `sh -c` args — in the right order relative to `git tag`, with real template substitution. The
// contract tests in internal/pipeline already prove the wiring logic against MockRunner; this
// closes the "does it actually run a real shell command against a real repo" gap the design
// doc's testing plan calls for.
func TestChangelog_RealGit_HooksExecuteInOrderWithTemplating(t *testing.T) {
	// Isolate from the host's global/system git config (e.g. tag.gpgSign=true would make
	// heraut attempt a GPG-signed tag and hang/fail on a machine without a usable key) so this
	// test's outcome never depends on the machine it runs on.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	testutil.RealGitRepo(t, "v0.1.0") // chdirs into a repo with one commit, tagged v0.1.0

	// A releasable commit since the tag, so the resolver has something to bump (fix -> patch).
	if out, err := exec.Command("git", "commit", "--allow-empty", "-m", "fix: something releasable").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	require.NoError(t, os.WriteFile(".heraut.yml", []byte(`
version: "1"
versioning:
  strategy: semver

hooks:
  pre_tag:
    - run: "echo pre_tag:{{ .Tag }} >> hooks.log"
  post_tag:
    - run: "echo post_tag:{{ .Tag }} >> hooks.log"
`), 0o644))

	out, err := executeRoot("changelog", "--tag", "--no-push")
	require.NoErrorf(t, err, "output:\n%s", out)

	logContent, err := os.ReadFile("hooks.log")
	require.NoError(t, err, "hooks.log must exist — the hooks must have actually executed")
	assert.Equal(t, "pre_tag:v0.1.1\npost_tag:v0.1.1\n", string(logContent),
		"pre_tag must fire before post_tag, and {{ .Tag }} must substitute the real resolved tag")

	tagOut, err := exec.Command("git", "tag", "-l", "v0.1.1").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(tagOut), "v0.1.1", "the tag itself must have been created for real")
}

// TestChangelog_RealGit_NoHooksSkipsRealExecution proves --no-hooks actually prevents the
// configured hooks from running against a real repo, not just in the MockRunner-based unit
// tests.
func TestChangelog_RealGit_NoHooksSkipsRealExecution(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	testutil.RealGitRepo(t, "v0.1.0")

	if out, err := exec.Command("git", "commit", "--allow-empty", "-m", "fix: something releasable").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	require.NoError(t, os.WriteFile(".heraut.yml", []byte(`
version: "1"
versioning:
  strategy: semver

hooks:
  pre_tag:
    - run: "echo pre_tag:{{ .Tag }} >> hooks.log"
  post_tag:
    - run: "echo post_tag:{{ .Tag }} >> hooks.log"
`), 0o644))

	out, err := executeRoot("changelog", "--tag", "--no-push", "--no-hooks")
	require.NoErrorf(t, err, "output:\n%s", out)

	_, err = os.Stat("hooks.log")
	assert.True(t, os.IsNotExist(err), "hooks.log must not exist — --no-hooks must skip real execution")
}

// TestChangelog_RealGit_PostBumpHookStagesDeclaredFile is T299: a real-git-repo proof that a
// post_bump hook's declared `stage` patterns (ADR-0061) actually land in the same commit as the
// changelog — not just that MockRunner recorded the right `git add` args (that's already covered
// at the pipeline contract-test layer, T295-T298), but that a real hook-written file really ends
// up in the real commit's tree.
func TestChangelog_RealGit_PostBumpHookStagesDeclaredFile(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	testutil.RealGitRepo(t, "v0.1.0")

	if out, err := exec.Command("git", "commit", "--allow-empty", "-m", "fix: something releasable").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	require.NoError(t, os.WriteFile(".heraut.yml", []byte(`
version: "1"
versioning:
  strategy: semver

changelog:
  output: CHANGELOG.md

hooks:
  post_bump:
    - run: 'echo "1.0.0" > version.txt'
      stage: ["version.txt"]
`), 0o644))

	out, err := executeRoot("changelog", "--tag", "--no-push")
	require.NoErrorf(t, err, "output:\n%s", out)

	showOut, err := exec.Command("git", "show", "--name-only", "HEAD").CombinedOutput()
	require.NoError(t, err)
	files := string(showOut)
	assert.Contains(t, files, "CHANGELOG.md", "the changelog must be part of the release commit's tree")
	assert.Contains(t, files, "version.txt", "the hook-declared stage pattern must be part of the same commit's tree")
}

// TestChangelog_RealGit_PostBumpHookStageMissingFileAbortsCommit is T299's second real-git-repo
// case: a post_bump hook that declares a `stage` pattern matching nothing on disk must fail the
// real `git add` (ADR-0061 Design §4 — a zero-match stage pattern is a `git add` failure like any
// other, no special-cased detection) before `git commit` ever runs, so no changelog commit is
// created.
func TestChangelog_RealGit_PostBumpHookStageMissingFileAbortsCommit(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	testutil.RealGitRepo(t, "v0.1.0")

	if out, err := exec.Command("git", "commit", "--allow-empty", "-m", "fix: something releasable").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	logBefore, err := exec.Command("git", "log", "--oneline").CombinedOutput()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(".heraut.yml", []byte(`
version: "1"
versioning:
  strategy: semver

changelog:
  output: CHANGELOG.md

hooks:
  post_bump:
    - run: "echo hook ran, but staged nothing"
      stage: ["does-not-exist.txt"]
`), 0o644))

	out, err := executeRoot("changelog", "--tag", "--no-push")
	require.Error(t, err, "output:\n%s", out)
	assert.Equal(t, exitcode.Runtime, cmd.ExitCode(err))
	// The detailed step failure — including the underlying `git add` error — is printed to out by
	// the spinner reporter; err itself carries only the short top-level summary (mirrors
	// TestChangelog_PreflightFail_GitIdentityMissing's convention in changelog_test.go).
	assert.Contains(t, out, "git add", "the failure must surface as a git add failure, not something else")

	logAfter, err := exec.Command("git", "log", "--oneline").CombinedOutput()
	require.NoError(t, err)
	assert.Equal(t, string(logBefore), string(logAfter),
		"git add failing must abort before git commit — no changelog commit must have been created")
}
