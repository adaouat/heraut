package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// shallowRepo has v0.1.0 released, two fixes since, and a history cut to the remote's tip: the
// default checkout of GitHub Actions and GitLab CI. heraut would see no tag and start over at
// v0.1.0.
func shallowRepo(t *testing.T) *harness.Repo {
	t.Helper()
	repo := flowRepo(t, flowCfg)
	repo.Commit("feat: one")
	repo.Tag("v0.1.0")
	repo.Commit("fix: two")
	repo.Git("push", "-q", "origin", "main", "v0.1.0")
	repo.Git("fetch", "-q", "--depth=1", "origin")
	require.Equal(t, "true", repo.Git("rev-parse", "--is-shallow-repository"))
	return repo
}

// release and changelog refuse a shallow clone before resolving, a dry run and --set-version
// included, and leave the repo and the remote untouched.
func TestShallowClone_WritesAreRefused(t *testing.T) {
	for _, args := range [][]string{
		{"changelog", "--commit", "--tag", "--offline"},
		{"changelog", "--dry-run", "--offline"},
		{"changelog", "--offline", "--set-version", "0.2.0"},
		{"release", "--dry-run", "--offline"},
	} {
		t.Run(args[0]+" "+args[1], func(t *testing.T) {
			bin := harness.Binary(t)
			repo := shallowRepo(t)
			head := repo.Git("rev-parse", "HEAD")

			res := repo.Run(bin, nil, args...)

			assert.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			assert.Contains(t, res.Stderr, "shallow clone")
			assert.Contains(t, res.Stderr, "git fetch --unshallow --tags")
			assert.NotContains(t, res.Stdout, "Resolve version", "stopped before resolving")
			assert.Equal(t, head, repo.Git("rev-parse", "HEAD"), "nothing committed")
			assert.Equal(t, "v0.1.0", repo.Git("tag", "-l"), "nothing tagged")
			assert.NoFileExists(t, repo.Dir+"/CHANGELOG.md")
			assert.Equal(t, "v0.1.0", repo.GitRemote("tag", "-l"), "nothing pushed")
		})
	}
}

// After `git fetch --unshallow --tags` the same repository releases normally.
func TestShallowClone_UnshallowFixesIt(t *testing.T) {
	bin := harness.Binary(t)
	repo := shallowRepo(t)
	repo.Git("fetch", "-q", "--unshallow", "--tags", "origin")

	res := runOK(t, repo, bin, "version", "next")

	assert.Equal(t, "v0.1.1\n", res.Stdout)
	assert.Empty(t, res.Stderr)
}

// version next and version current still answer, with a warning on stderr; stdout stays the tag.
func TestShallowClone_ReadsWarn(t *testing.T) {
	bin := harness.Binary(t)
	repo := shallowRepo(t)

	next := runOK(t, repo, bin, "version", "next")
	assert.Regexp(t, `^v\d+\.\d+\.\d+\n$`, next.Stdout, "stdout stays exactly the tag")
	assert.Contains(t, next.Stderr, "shallow clone")

	nextJSON := runOK(t, repo, bin, "version", "next", "--json")
	assert.Contains(t, nextJSON.Stdout, "shallow clone", "in --json the warning goes in warnings")

	current := runOK(t, repo, bin, "version", "current")
	assert.Equal(t, "v0.1.0\n", current.Stdout)
	assert.Contains(t, current.Stderr, "shallow clone")

	currentJSON := runOK(t, repo, bin, "version", "current", "--json")
	assert.Contains(t, currentJSON.Stdout, "shallow clone")
}

// check runtime fails on a shallow clone, naming the fix.
func TestShallowClone_CheckRuntimeFails(t *testing.T) {
	bin := harness.Binary(t)
	repo := shallowRepo(t)

	res := repo.Run(bin, nil, "check", "runtime")

	assert.NotEqual(t, exitOK, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout+res.Stderr, "history")
	assert.Contains(t, res.Stdout+res.Stderr, "git fetch --unshallow --tags")
}
