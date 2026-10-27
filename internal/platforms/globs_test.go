package platforms_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/platforms"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func touch(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

// ---- Confinement to the repository root (SEC-009) ----------------------------

// inRepo makes a fresh directory the working directory, which is the root asset globs are
// confined to.
func inRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	t.Chdir(repo)
	return repo
}

func noWarn(t *testing.T) func(string) {
	return func(p string) { t.Errorf("unexpected no-match warning for %q", p) }
}

func TestCheckAssetGlobs_RefusesAnAssetOutsideTheRoot(t *testing.T) {
	inRepo(t)
	outside := t.TempDir()
	touch(t, outside, "secret.txt")

	err := platforms.CheckAssetGlobs([]string{filepath.Join(outside, "*.txt")}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
}

func TestCheckAssetGlobs_AcceptsInsideFilesAndStaysSilentOnNoMatchWhenAllowed(t *testing.T) {
	repo := inRepo(t)
	touch(t, repo, "app.zip")

	require.NoError(t, platforms.CheckAssetGlobs([]string{"*.zip", "missing/*"}, true))
}

func TestCheckAssetGlobs_InvalidGlobErrors(t *testing.T) {
	inRepo(t)
	require.Error(t, platforms.CheckAssetGlobs([]string{"["}, false))
	require.Error(t, platforms.CheckAssetGlobs([]string{"["}, true))
}

// ADR-0076: a pattern that matches nothing is refused up front, before anything is tagged.
func TestCheckAssetGlobs_RefusesAPatternMatchingNothing(t *testing.T) {
	repo := inRepo(t)
	touch(t, repo, "app.zip")

	err := platforms.CheckAssetGlobs([]string{"*.zip", "dist/app-*.tar.gz"}, false)

	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrAssetUnmatched)
	assert.Contains(t, err.Error(), `"dist/app-*.tar.gz"`, "names the pattern that matched nothing")
	assert.NotContains(t, err.Error(), `"*.zip"`, "the pattern that matched is not blamed")
	assert.Contains(t, err.Error(), "allow_missing_assets")
	assert.Contains(t, err.Error(), "post_bump")
}

func TestCheckAssetGlobs_ListsEveryUnmatchedPattern(t *testing.T) {
	inRepo(t)

	err := platforms.CheckAssetGlobs([]string{"a-*.zip", "b-*.zip"}, false)

	require.ErrorIs(t, err, port.ErrAssetUnmatched)
	assert.Contains(t, err.Error(), `"a-*.zip"`)
	assert.Contains(t, err.Error(), `"b-*.zip"`)
}

func TestCheckAssetGlobs_ADirectoryOnlyMatchIsNotAFile(t *testing.T) {
	repo := inRepo(t)
	require.NoError(t, os.Mkdir(filepath.Join(repo, "dist"), 0o755))

	err := platforms.CheckAssetGlobs([]string{"dist"}, false)

	require.ErrorIs(t, err, port.ErrAssetUnmatched, "a directory is skipped when publishing, so it is not an asset")
}

// Two matches published under one name are refused before anything is committed, as publishing
// would refuse them after the tag push. Never wrapped in ErrAssetUnmatched: a dry run must fail too.
func TestCheckAssetGlobs_RefusesDuplicateBasenames(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		patterns []string
		want     []string
	}{
		{name: "same name in two directories", files: []string{"linux/app.zip", "darwin/app.zip"},
			patterns: []string{"linux/*.zip", "darwin/*.zip"}, want: []string{`"app.zip"`, "linux/app.zip", "darwin/app.zip"}},
		{name: "one pattern matching both", files: []string{"linux/app.zip", "darwin/app.zip"},
			patterns: []string{"*/app.zip"}, want: []string{`"app.zip"`}},
		{name: "the same file matched twice", files: []string{"app.zip"},
			patterns: []string{"*.zip", "app.zip"}, want: []string{`"app.zip"`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := inRepo(t)
			for _, f := range tc.files {
				require.NoError(t, os.MkdirAll(filepath.Join(repo, filepath.Dir(f)), 0o755))
				touch(t, repo, f)
			}

			for _, allowMissing := range []bool{false, true} {
				err := platforms.CheckAssetGlobs(tc.patterns, allowMissing)
				require.Error(t, err)
				assert.NotErrorIs(t, err, port.ErrAssetUnmatched)
				for _, w := range tc.want {
					assert.Contains(t, err.Error(), w)
				}
			}
		})
	}
}

func TestCheckAssetGlobs_DistinctNamesPass(t *testing.T) {
	repo := inRepo(t)
	for _, d := range []string{"linux", "darwin"} {
		require.NoError(t, os.Mkdir(filepath.Join(repo, d), 0o755))
		touch(t, repo, d+"/app-"+d+".zip")
	}

	require.NoError(t, platforms.CheckAssetGlobs([]string{"linux/*.zip", "darwin/*.zip"}, false))
}
