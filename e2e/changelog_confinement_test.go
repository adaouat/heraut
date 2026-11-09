package e2e_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

// A committed CHANGELOG.md symlink to a file outside the repository is refused: heraut never writes
// through it.
func TestChangelog_ASymlinkOutOfTheRepositoryIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	bin := harness.Binary(t)
	repo := flowRepo(t, flowCfg)
	victim := filepath.Join(filepath.Dir(repo.Dir), "victim.txt")
	require.NoError(t, os.WriteFile(victim, nil, 0o644))
	require.NoError(t, os.Symlink("../victim.txt", filepath.Join(repo.Dir, "CHANGELOG.md")))
	repo.Git("add", "CHANGELOG.md")
	repo.Commit("feat: one")

	res := repo.Run(bin, nil, "changelog", "--offline")

	assert.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout+res.Stderr, "resolves outside the repository")
	b, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Empty(t, b, "the file outside the repository is untouched")
}

// changelog.output naming a path above the repository is refused the same way.
func TestChangelog_AnOutputAboveTheRepositoryIsRefused(t *testing.T) {
	bin := harness.Binary(t)
	repo := flowRepo(t, "version: \"1\"\nversioning:\n  strategy: semver\nchangelog:\n  output: ../escaped.md\n")
	repo.Commit("feat: one")

	res := repo.Run(bin, nil, "changelog", "--offline")

	assert.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
	assert.Contains(t, res.Stdout+res.Stderr, "resolves outside the repository")
	assert.NoFileExists(t, filepath.Join(filepath.Dir(repo.Dir), "escaped.md"))
}
