package cmd_test

import (
	"os"
	"os/exec"
	"testing"

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
    - "echo pre_tag:{{ .Tag }} >> hooks.log"
  post_tag:
    - "echo post_tag:{{ .Tag }} >> hooks.log"
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
    - "echo pre_tag:{{ .Tag }} >> hooks.log"
  post_tag:
    - "echo post_tag:{{ .Tag }} >> hooks.log"
`), 0o644))

	out, err := executeRoot("changelog", "--tag", "--no-push", "--no-hooks")
	require.NoErrorf(t, err, "output:\n%s", out)

	_, err = os.Stat("hooks.log")
	assert.True(t, os.IsNotExist(err), "hooks.log must not exist — --no-hooks must skip real execution")
}
