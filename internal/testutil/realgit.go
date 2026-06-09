package testutil

import (
	"os/exec"
	"testing"
)

// RealGitRepo creates a temporary git repository with a single tagged conventional commit
// and chdirs into it (restored on test cleanup). It is for the real-CLI smoke tests (T77)
// that run the actual git-cliff / cog against heraut's embedded default configs — the one
// place the suite uses real external binaries rather than MockRunner/FakeBin, because that
// is the only way to catch an embedded config the real tool rejects. Skips the test when
// git is not on PATH.
func RealGitRepo(t *testing.T, tag string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
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
