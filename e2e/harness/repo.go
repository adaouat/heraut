package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Repo is a throwaway git repository with deterministic identity and no signing.
type Repo struct {
	t      testing.TB
	Dir    string
	home   string
	remote string
	binDir string
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

// Switch moves to an existing branch.
func (r *Repo) Switch(branch string) {
	r.t.Helper()
	r.git("checkout", "-q", branch)
}

// Detach detaches HEAD at its current commit.
func (r *Repo) Detach() {
	r.t.Helper()
	r.git("checkout", "-q", "--detach")
}

// MergeNoFF merges branch into the current branch with a merge commit.
func (r *Repo) MergeNoFF(branch string) {
	r.t.Helper()
	r.git("merge", "-q", "--no-ff", "-m", "chore: merge "+branch, branch)
}

// AddRemote creates a bare repository, wires it as origin and pushes the current branch with
// upstream tracking. The repository needs at least one commit.
func (r *Repo) AddRemote() {
	r.t.Helper()
	r.remote = filepath.Join(r.t.TempDir(), "origin.git")
	cmd := exec.Command("git", "init", "-q", "--bare", r.remote)
	cmd.Env = baseEnv(r.home)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	r.git("remote", "add", "origin", r.remote)
	r.git("push", "-q", "-u", "origin", "main")
}

// RemoveRemote deletes the bare remote so a later push fails.
func (r *Repo) RemoveRemote() {
	r.t.Helper()
	if err := os.RemoveAll(r.remote); err != nil {
		r.t.Fatalf("removing the remote: %v", err)
	}
}

// RestoreRemote recreates the bare remote removed by RemoveRemote, holding only main at rev
// (typically the commit the remote had before the failed push), so a retry meets a remote that is
// reachable again but still behind.
func (r *Repo) RestoreRemote(rev string) {
	r.t.Helper()
	cmd := exec.Command("git", "init", "-q", "--bare", r.remote)
	cmd.Env = baseEnv(r.home)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	r.git("push", "-q", "origin", rev+":refs/heads/main")
}

// Git runs git in the repository and returns its trimmed output.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return strings.TrimSpace(r.git(args...))
}

// GitRemote runs git against the bare remote and returns its trimmed output.
func (r *Repo) GitRemote(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"--git-dir", r.remote}, args...)...)
	cmd.Env = baseEnv(r.home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git (remote) %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// WriteFile writes content to a path relative to the repository root, creating directories.
func (r *Repo) WriteFile(rel, content string) {
	r.t.Helper()
	path := filepath.Join(r.Dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatalf("creating directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatalf("writing %s: %v", rel, err)
	}
}

// Home is the HOME directory the repository's commands run with.
func (r *Repo) Home() string { return r.home }

// WriteHomeFile writes content to a path relative to HOME, creating directories.
func (r *Repo) WriteHomeFile(rel, content string) {
	r.t.Helper()
	path := filepath.Join(r.home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatalf("creating directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatalf("writing %s: %v", rel, err)
	}
}
