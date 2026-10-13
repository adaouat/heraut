package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// A tag named like a git option can only be created with update-ref or a fetch. It must never reach
// a git command as an argument: `git log --output=<file>..<tag>` would write that file.
func TestOptionShapedTagNeverReachesGit(t *testing.T) {
	bin := harness.Binary(t, "heraut_testclock")
	repo := harness.NewRepo(t)
	repo.WriteConfig(calverFmt("YYYY.MM.PATCH", "", "changelog:\n  output: CHANGELOG.md\n"))
	repo.Commit("feat: one")
	repo.Tag("2026.05.0")
	repo.Commit("fix: two")
	repo.Tag("2026.05.1")
	repo.Git("update-ref", "refs/tags/--output=pwned", "HEAD")
	env := at("2026-05-20T10:00:00Z")

	for _, args := range [][]string{
		{"changelog", "--offline"},
		{"notes", "2026.05.1", "--offline"},
		{"version", "next"},
	} {
		res := repo.Run(bin, env, args...)
		require.Equal(t, exitOK, res.ExitCode, "heraut %v\nstdout:\n%s\nstderr:\n%s", args, res.Stdout, res.Stderr)
	}

	matches, err := filepath.Glob(filepath.Join(repo.Dir, "*pwned*"))
	require.NoError(t, err)
	assert.Empty(t, matches, "no command wrote a file named after the tag")
	_, err = os.Stat(filepath.Join(repo.Dir, "pwned"))
	assert.True(t, os.IsNotExist(err))
}
