package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Repo is a throwaway git repository with deterministic identity and no signing.
type Repo struct {
	t    testing.TB
	Dir  string
	home string
}

// NewRepo initialises an empty repository on branch main; it skips when git is not on PATH.
func NewRepo(t testing.TB) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	r := &Repo{t: t, Dir: t.TempDir(), home: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	r.git("config", "user.email", "e2e@example.com")
	r.git("config", "user.name", "e2e")
	r.git("config", "commit.gpgsign", "false")
	r.git("config", "tag.gpgsign", "false")
	return r
}

// Commit records an empty commit with the given message.
func (r *Repo) Commit(msg string) {
	r.t.Helper()
	r.git("commit", "-q", "--allow-empty", "-m", msg)
}

// Tag creates an annotated tag on HEAD.
func (r *Repo) Tag(name string) {
	r.t.Helper()
	r.git("tag", "-a", name, "-m", name)
}

// WriteConfig writes .heraut.yml at the repository root.
func (r *Repo) WriteConfig(yaml string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.Dir, ".heraut.yml"), []byte(yaml), 0o644); err != nil {
		r.t.Fatalf("writing .heraut.yml: %v", err)
	}
}

func (r *Repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = baseEnv(r.home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func baseEnv(home string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"GIT_CONFIG_NOSYSTEM=1",
		"NO_COLOR=1",
		"HERAUT_CHECK_UPDATE=false",
		"LC_ALL=C",
	}
}

// ReadFile returns the contents of a file relative to the repository root.
func (r *Repo) ReadFile(rel string) string {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join(r.Dir, rel))
	if err != nil {
		r.t.Fatalf("reading %s: %v", rel, err)
	}
	return string(b)
}

// Checkout creates branch at HEAD and switches to it.
func (r *Repo) Checkout(branch string) {
	r.t.Helper()
	r.git("checkout", "-q", "-b", branch)
}
