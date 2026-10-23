package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/harness"
)

const nestedChangelogCfg = `version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
changelog:
  output: docs/CHANGELOG.md
`

// leftoverTempFiles lists the temp files writeFileAtomic would leave beside the changelog.
func leftoverTempFiles(t *testing.T, dir string) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(dir, ".CHANGELOG.md.tmp-*"))
	require.NoError(t, err)
	return found
}

// The changelog is replaced through a temp file and a rename (T387), so a write that cannot create
// the temp file fails before the existing file is touched.
func TestChangelogWrite_IsAtomic(t *testing.T) {
	bin := harness.Binary(t)

	t.Run("a normal run leaves no temp file behind", func(t *testing.T) {
		repo := flowRepo(t, nestedChangelogCfg)
		require.NoError(t, os.MkdirAll(filepath.Join(repo.Dir, "docs"), 0o755))
		repo.Commit("feat: one")

		runOK(t, repo, bin, "changelog", "--offline")

		assert.Contains(t, repo.ReadFile("docs/CHANGELOG.md"), "One")
		assert.Empty(t, leftoverTempFiles(t, filepath.Join(repo.Dir, "docs")))
	})

	t.Run("a stricter mode on the changelog survives the rewrite", func(t *testing.T) {
		repo := flowRepo(t, nestedChangelogCfg)
		require.NoError(t, os.MkdirAll(filepath.Join(repo.Dir, "docs"), 0o755))
		repo.Commit("feat: one")
		runOK(t, repo, bin, "changelog", "--offline")
		changelog := filepath.Join(repo.Dir, "docs", "CHANGELOG.md")
		require.NoError(t, os.Chmod(changelog, 0o600))
		repo.Commit("feat: two")

		runOK(t, repo, bin, "changelog", "--offline")

		info, err := os.Stat(changelog)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		assert.Contains(t, repo.ReadFile("docs/CHANGELOG.md"), "Two", "the file was rewritten")
	})

	t.Run("a read-only directory is refused and the file is untouched", func(t *testing.T) {
		repo := flowRepo(t, nestedChangelogCfg)
		require.NoError(t, os.MkdirAll(filepath.Join(repo.Dir, "docs"), 0o755))
		repo.Commit("feat: one")
		runOK(t, repo, bin, "changelog", "--offline")
		docs := filepath.Join(repo.Dir, "docs")
		before := repo.ReadFile("docs/CHANGELOG.md")

		require.NoError(t, os.Chmod(docs, 0o555))
		t.Cleanup(func() { _ = os.Chmod(docs, 0o755) })
		if probe, err := os.Create(filepath.Join(docs, ".probe")); err == nil {
			_ = probe.Close()
			t.Skip("the process can write into a 0555 directory (running as root)")
		}
		repo.Commit("feat: two")

		res := repo.Run(bin, nil, "changelog", "--offline")

		require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
		assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), "writing changelog")
		assert.Equal(t, before, repo.ReadFile("docs/CHANGELOG.md"), "the previous changelog is byte-identical")
		assert.False(t, strings.Contains(repo.ReadFile("docs/CHANGELOG.md"), "Two"))
		assert.Empty(t, leftoverTempFiles(t, docs))
	})
}
