package testutil

import (
	"os"
	"os/exec"
	"testing"
)

// RealGitRepo creates a temporary git repository with a single tagged conventional commit
// and chdirs into it (restored on test cleanup). Used by tests that need a real git binary
// rather than MockRunner/FakeBin — e.g. commit-verification integration tests. Skips the
// test when git is not on PATH.
func RealGitRepo(t *testing.T, tag string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	isolateGit(t.Setenv)
	dir := t.TempDir()
	t.Chdir(dir)

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "feat: a feature")
	run("tag", "-a", tag, "-m", tag)
}

// IsolateProcessEnv is for a package's TestMain when its tests run real git: the developer's global and
// system git config (commit signing, hooks, a default branch) and an exported HERAUT_FILE must not
// change what the tests see.
func IsolateProcessEnv() {
	isolateGit(func(k, v string) { _ = os.Setenv(k, v) })
}

func isolateGit(set func(key, value string)) {
	set("GIT_CONFIG_GLOBAL", os.DevNull)
	set("GIT_CONFIG_NOSYSTEM", "1")
	set("HERAUT_FILE", "")
}
