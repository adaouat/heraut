//go:build e2e_forge

package forgeharness

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// localForge is a Forge whose clone URL is a local bare repository; it records deletions.
type localForge struct {
	Forge
	url                                           string
	failRelease                                   map[string]bool
	deletedReleases, deletedTags, deletedBranches []string
	tags, releases, branches                      []string
}

func (l *localForge) CloneURL() string                     { return l.url }
func (l *localForge) GitAuthHeader() string                { return "X-Test: 1" }
func (l *localForge) Name() string                         { return "github" }
func (l *localForge) Platform() string                     { return "github" }
func (l *localForge) Coordinates() string                  { return "acme/widget-testing" }
func (l *localForge) CoordinatesKey() string               { return "repository" }
func (l *localForge) TokenEnv() string                     { return "GH_TOKEN" }
func (l *localForge) Token() string                        { return "tok" }
func (l *localForge) Tags(prefix string) ([]string, error) { return hasPrefix(l.tags, prefix), nil }
func (l *localForge) Releases(p string) ([]string, error)  { return hasPrefix(l.releases, p), nil }
func (l *localForge) Branches(p string) ([]string, error)  { return hasPrefix(l.branches, p), nil }
func (l *localForge) DeleteRelease(tag string) error {
	if l.failRelease[tag] {
		return errors.New("boom")
	}
	l.deletedReleases = append(l.deletedReleases, tag)
	return nil
}
func (l *localForge) DeleteTag(tag string) error {
	l.deletedTags = append(l.deletedTags, tag)
	return nil
}
func (l *localForge) DeleteBranch(name string) error {
	l.deletedBranches = append(l.deletedBranches, name)
	return nil
}

func newBare(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "--bare", "-b", "main", dir},
	} {
		out, err := exec.Command("git", args...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	seed := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "s@example.com"},
		{"config", "user.name", "s"},
		{"commit", "-q", "--allow-empty", "-m", "chore: seed"},
		{"push", "-q", dir, "main"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = seed
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	return dir
}

func TestWorkspaceClonesBranchesAndCleansUpWhatTheRunCreated(t *testing.T) {
	f := &localForge{url: newBare(t)}
	var runID string

	t.Run("run", func(t *testing.T) {
		w := NewWorkspace(t, f)
		runID = w.RunID

		assert.Regexp(t, `^e2e-[0-9]+-[0-9a-f]{4}$`, w.RunID)
		assert.Equal(t, "e2e/"+w.RunID, w.Branch)
		assert.Equal(t, w.RunID+"-v", w.TagPrefix)
		assert.Equal(t, w.Branch, w.Repo.Git("rev-parse", "--abbrev-ref", "HEAD"))
		assert.Equal(t, "chore: seed", w.Repo.Git("log", "-1", "--format=%s"), "the branch starts at the sandbox's main")
		assert.Equal(t, []string{"GH_TOKEN=tok"}, w.Env())
		assert.Contains(t, w.Config("versioning:\n  strategy: semver\n", ""), "repository: acme/widget-testing")
		assert.Contains(t, w.Config("versioning:\n  strategy: semver\n", ""), "token_env: GH_TOKEN")

		// the run "created" these; another run's resources must survive
		f.tags = []string{w.TagPrefix + "0.1.0", "e2e-1-ffff-v9.9.9", "v1.0.0"}
		f.releases = []string{w.TagPrefix + "0.1.0", "e2e-1-ffff-v9.9.9"}
		f.branches = []string{w.Branch, "e2e/e2e-1-ffff", "main"}
	})

	assert.Equal(t, []string{runID + "-v0.1.0"}, f.deletedReleases)
	assert.Equal(t, []string{runID + "-v0.1.0"}, f.deletedTags)
	assert.Equal(t, []string{"e2e/" + runID}, f.deletedBranches)
	for _, deleted := range append(append(f.deletedReleases, f.deletedTags...), f.deletedBranches...) {
		assert.True(t, strings.Contains(deleted, runID), "only this run's resources are deleted: %s", deleted)
	}
}
