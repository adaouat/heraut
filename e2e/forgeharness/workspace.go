//go:build e2e_forge

package forgeharness

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adaouat/heraut/e2e/harness"
)

// Workspace is a clone of a sandbox repository on a throwaway branch; everything the run creates
// carries RunID and is deleted by the cleanup registered in NewWorkspace.
type Workspace struct {
	Repo      *harness.Repo
	Forge     Forge
	RunID     string
	Branch    string
	TagPrefix string
}

// Require returns the named sandbox forge, skipping when it is not configured and failing when a
// guard refuses it.
func Require(t *testing.T, name string) Forge {
	t.Helper()
	c := LoadConfig()
	var f Forge
	switch name {
	case "github":
		if c.GitHubRepo == "" {
			t.Skip("HERAUT_E2E_GITHUB_REPO is not set: skipping the GitHub sandbox scenarios")
		}
		tok, err := GitHubToken()
		if err != nil {
			t.Skip(err.Error())
		}
		f = NewGitHub(c, tok)
	case "gitlab":
		if c.GitLabProject == "" {
			t.Skip("HERAUT_E2E_GITLAB_PROJECT is not set: skipping the GitLab sandbox scenarios")
		}
		tok, err := GitLabToken()
		if err != nil {
			t.Skip(err.Error())
		}
		f = NewGitLab(c, tok)
	default:
		t.Fatalf("unknown forge %q", name)
	}
	if err := f.Check(); err != nil {
		t.Fatalf("sandbox guard: %v", err)
	}
	return f
}

// NewWorkspace clones f's default branch into a temporary repository, creates e2e/<run-id> and
// registers the cleanup.
func NewWorkspace(t *testing.T, f Forge) *Workspace {
	t.Helper()
	id := NewRunID(time.Now(), RandomHex())
	w := &Workspace{Forge: f, RunID: id, Branch: "e2e/" + id, TagPrefix: id + "-v"}

	repo := harness.NewRepo(t)
	repo.Git("remote", "add", "origin", f.CloneURL())
	repo.Git("config", "credential.helper", "")
	repo.Git("config", "http.extraheader", f.GitAuthHeader())
	repo.Git("fetch", "-q", "origin", "main")
	repo.Git("checkout", "-q", "-b", w.Branch, "origin/main")
	w.Repo = repo

	t.Cleanup(func() { w.cleanup(t) })
	return w
}

// Env is the environment a heraut run needs for this forge: its token variable and nothing else.
func (w *Workspace) Env() []string { return []string{w.Forge.TokenEnv() + "=" + w.Forge.Token()} }

// Config renders a .heraut.yml for this forge: versioningBlock is the full `versioning:` section,
// releaseExtra is spliced into the single release target (for example assets).
func (w *Workspace) Config(versioningBlock, releaseExtra string) string {
	return fmt.Sprintf(`version: "1"
%schangelog:
  output: CHANGELOG.md
forges:
  - name: %s
    platform: %s
    %s: %s
    token_env: %s
release:
  targets:
    - forge: %s
%s`, versioningBlock, w.Forge.Name(), w.Forge.Platform(), w.Forge.CoordinatesKey(), w.Forge.Coordinates(),
		w.Forge.TokenEnv(), w.Forge.Name(), releaseExtra)
}

// cleanup deletes this run's releases, tags and branch, in that order. Failures fail the test:
// a leaked resource must be visible.
func (w *Workspace) cleanup(t *testing.T) {
	t.Helper()
	f := w.Forge
	if releases, err := f.Releases(w.RunID); err != nil {
		t.Errorf("cleanup: listing releases: %v", err)
	} else {
		for _, tag := range releases {
			w.delete(t, "release", tag, f.DeleteRelease)
		}
	}
	tags, err := f.Tags(w.RunID)
	if err != nil {
		t.Errorf("cleanup: listing tags: %v", err)
	}
	for _, tag := range tags {
		w.delete(t, "tag", tag, f.DeleteTag)
	}
	branches, err := f.Branches(w.Branch)
	if err != nil {
		t.Errorf("cleanup: listing branches: %v", err)
		return
	}
	for _, b := range branches {
		w.delete(t, "branch", b, f.DeleteBranch)
	}
}

func (w *Workspace) delete(t *testing.T, kind, name string, del func(string) error) {
	t.Helper()
	if !strings.Contains(name, w.RunID) {
		t.Errorf("cleanup: refusing to delete %s %q: it does not carry the run id %s", kind, name, w.RunID)
		return
	}
	if err := del(name); err != nil {
		t.Errorf("cleanup: deleting %s %q: %v", kind, name, err)
	}
}
