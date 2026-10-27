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

func TestResolveGlobsLenient_MultiplePatterns(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	touch(t, dir, "app.tar.gz")
	touch(t, dir, "app.zip")

	files, err := platforms.ResolveGlobsLenient([]string{
		filepath.Join(dir, "*.tar.gz"),
		filepath.Join(dir, "*.zip"),
	}, func(p string) { t.Errorf("unexpected no-match warning for %q", p) })
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		filepath.Join(dir, "app.tar.gz"),
		filepath.Join(dir, "app.zip"),
	}, files)
}

func TestResolveGlobsLenient_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	touch(t, dir, "file.bin")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755))

	files, err := platforms.ResolveGlobsLenient([]string{filepath.Join(dir, "*")}, func(p string) {
		t.Errorf("unexpected no-match warning for %q", p)
	})
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(dir, "file.bin")}, files)
}

func TestResolveGlobsLenient_Matches(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	touch(t, dir, "a.bin")
	touch(t, dir, "b.bin")

	var warned []string
	files, err := platforms.ResolveGlobsLenient(
		[]string{filepath.Join(dir, "*.bin")},
		func(p string) { warned = append(warned, p) },
	)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		filepath.Join(dir, "a.bin"),
		filepath.Join(dir, "b.bin"),
	}, files)
	assert.Empty(t, warned)
}

func TestResolveGlobsLenient_NoMatch_Warns(t *testing.T) {
	dir := t.TempDir()

	var warned []string
	files, err := platforms.ResolveGlobsLenient(
		[]string{filepath.Join(dir, "*.zip")},
		func(p string) { warned = append(warned, p) },
	)
	require.NoError(t, err)
	assert.Empty(t, files)
	require.Len(t, warned, 1)
	assert.Contains(t, warned[0], "*.zip")
}

func TestResolveGlobsLenient_InvalidGlob_Errors(t *testing.T) {
	_, err := platforms.ResolveGlobsLenient([]string{"[invalid"}, func(string) {})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid glob pattern")
}

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

func TestResolveGlobsLenient_RefusesAbsolutePathOutsideTheRoot(t *testing.T) {
	inRepo(t)
	outside := t.TempDir()
	touch(t, outside, "secret.txt")

	_, err := platforms.ResolveGlobsLenient([]string{filepath.Join(outside, "*.txt")}, noWarn(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
	assert.Contains(t, err.Error(), "secret.txt")
}

func TestResolveGlobsLenient_RefusesParentTraversal(t *testing.T) {
	repo := inRepo(t)
	touch(t, filepath.Dir(repo), "sibling.bin")
	t.Cleanup(func() { _ = os.Remove(filepath.Join(filepath.Dir(repo), "sibling.bin")) })

	_, err := platforms.ResolveGlobsLenient([]string{"../sibling.bin"}, noWarn(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
}

func TestResolveGlobsLenient_RefusesSymlinkLeavingTheRoot(t *testing.T) {
	repo := inRepo(t)
	outside := t.TempDir()
	touch(t, outside, "secret.txt")
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(repo, "innocent.txt")))

	_, err := platforms.ResolveGlobsLenient([]string{"innocent.txt"}, noWarn(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
}

func TestResolveGlobsLenient_AllowsFilesAndSymlinksInsideTheRoot(t *testing.T) {
	repo := inRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "dist"), 0o755))
	touch(t, filepath.Join(repo, "dist"), "app.zip")
	require.NoError(t, os.Symlink(filepath.Join(repo, "dist", "app.zip"), filepath.Join(repo, "latest.zip")))

	files, err := platforms.ResolveGlobsLenient([]string{"dist/*.zip", filepath.Join(repo, "dist", "*.zip"), "latest.zip"}, noWarn(t))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"dist/app.zip", filepath.Join(repo, "dist", "app.zip"), "latest.zip"}, files)
}

func TestResolveGlobsLenient_NoMatchOutsideTheRootStillJustWarns(t *testing.T) {
	inRepo(t)
	var warned []string
	files, err := platforms.ResolveGlobsLenient([]string{filepath.Join(t.TempDir(), "*.zip")}, func(p string) { warned = append(warned, p) })
	require.NoError(t, err)
	assert.Empty(t, files)
	assert.Len(t, warned, 1)
}

func TestResolveGlobsLenient_PrefixesDotSlashOnNamesStartingWithADash(t *testing.T) {
	repo := inRepo(t)
	touch(t, repo, "-rf.zip")
	touch(t, repo, "ok.zip")

	files, err := platforms.ResolveGlobsLenient([]string{"*.zip"}, noWarn(t))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"./-rf.zip", "ok.zip"}, files, "a leading dash must not read as an option to gh")
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
