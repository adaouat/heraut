package native

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func newWriter(out string) *Generator {
	return New(exectest.NewMockRunner(), &config.ContentDriver{Output: out}, ModeChangelog)
}

// repoDir returns a temp directory made the working directory: the changelog is confined to the
// repository, which is the working directory.
func repoDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestWriteChangelog_ReplacesContentAndLeavesNoTempFile(t *testing.T) {
	dir := repoDir(t)
	out := filepath.Join(dir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(out, []byte("old\n"), 0o600))

	got, err := newWriter(out).writeChangelog("new\n")
	require.NoError(t, err)
	assert.Equal(t, "new\n", got)

	b, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, "new\n", string(b))
	assert.Equal(t, []string{"CHANGELOG.md"}, dirEntries(t, dir), "the temp file is renamed away, none is left behind")
}

func TestWriteChangelog_FileModeIs0644(t *testing.T) {
	dir := repoDir(t)
	out := filepath.Join(dir, "CHANGELOG.md")

	_, err := newWriter(out).writeChangelog("body\n")
	require.NoError(t, err)

	info, err := os.Stat(out)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	}
}

// A write that cannot complete must leave the previous changelog byte for byte intact. The
// directory is made read-only so the new content has nowhere to be staged: an in-place write would
// still succeed on the writable file and truncate it.
func TestWriteChangelog_FailedWriteKeepsOriginal(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions are not enforced for this user")
	}
	dir := repoDir(t)
	out := filepath.Join(dir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(out, []byte("original\n"), 0o644))
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := newWriter(out).writeChangelog("replacement\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "writing changelog")

	b, readErr := os.ReadFile(out)
	require.NoError(t, readErr)
	assert.Equal(t, "original\n", string(b))
	assert.Equal(t, []string{"CHANGELOG.md"}, dirEntries(t, dir))
}

func TestWriteChangelog_FollowsASymlinkedChangelog(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	dir := repoDir(t)
	target := filepath.Join(dir, "real.md")
	link := filepath.Join(dir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(target, []byte("old\n"), 0o644))
	require.NoError(t, os.Symlink("real.md", link))

	_, err := newWriter(link).writeChangelog("new\n")
	require.NoError(t, err)

	fi, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, fi.Mode()&os.ModeSymlink, "the link itself is kept")
	b, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new\n", string(b))
}

func TestWriteChangelog_KeepsTheModeOfAnExistingFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not enforced on windows")
	}
	dir := repoDir(t)
	out := filepath.Join(dir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(out, []byte("old\n"), 0o600))

	_, err := newWriter(out).writeChangelog("new\n")
	require.NoError(t, err)

	info, err := os.Stat(out)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "a stricter mode survives the rewrite")
}

func TestWriteChangelog_KeepsTheModeOfASymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	dir := repoDir(t)
	target := filepath.Join(dir, "real.md")
	link := filepath.Join(dir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(target, []byte("old\n"), 0o600))
	require.NoError(t, os.Symlink("real.md", link))

	_, err := newWriter(link).writeChangelog("new\n")
	require.NoError(t, err)

	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// A process killed between creating its temp file and renaming it leaves the temp file behind, where a
// hook staging everything could commit it. The next write removes such leftovers of its own target,
// and nothing else.
func TestWriteChangelog_RemovesTempFilesAnInterruptedWriteLeft(t *testing.T) {
	dir := repoDir(t)
	out := filepath.Join(dir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(out, []byte("old\n"), 0o644))
	for _, name := range []string{
		".CHANGELOG.md.tmp-123", ".CHANGELOG.md.tmp-", // leftovers of this target
		"CHANGELOG.md.tmp-1", ".CHANGELOG.md.tmpx", ".NOTES.md.tmp-1", ".changelog.md.tmp-1", // not ours
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644))
	}
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".CHANGELOG.md.tmp-dir"), 0o755))

	_, err := newWriter(out).writeChangelog("new\n")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		"CHANGELOG.md",
		"CHANGELOG.md.tmp-1", ".CHANGELOG.md.tmpx", ".NOTES.md.tmp-1", ".changelog.md.tmp-1",
		".CHANGELOG.md.tmp-dir",
	}, dirEntries(t, dir), "only regular files named like this target's temp files are removed")
}

func TestWriteChangelog_RemovesTheLeftoversOfASymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	dir := repoDir(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "real.md"), []byte("old\n"), 0o644))
	require.NoError(t, os.Symlink("real.md", filepath.Join(dir, "CHANGELOG.md")))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".real.md.tmp-1"), []byte("x"), 0o644))

	_, err := newWriter(filepath.Join(dir, "CHANGELOG.md")).writeChangelog("new\n")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"CHANGELOG.md", "real.md"}, dirEntries(t, dir))
}

func TestWriteChangelog_NoOutputWritesNothing(t *testing.T) {
	got, err := newWriter("").writeChangelog("body\n")
	require.NoError(t, err)
	assert.Equal(t, "body\n", got)
}

// The changelog is confined to the repository (the working directory): a path that leaves it is
// refused before anything is read or written.
func TestWriteChangelog_RefusesAPathOutsideTheRepository(t *testing.T) {
	top := t.TempDir()
	repo := filepath.Join(top, "a", "b")
	require.NoError(t, os.MkdirAll(repo, 0o755))
	t.Chdir(repo)

	_, err := newWriter("../../escaped.md").writeChangelog("new\n")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
	assert.NoFileExists(t, filepath.Join(top, "escaped.md"))
}

// A committed symlink pointing out of the repository is refused: the changelog is not written
// through it.
func TestWriteChangelog_RefusesASymlinkLeavingTheRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	top := t.TempDir()
	repo := filepath.Join(top, "repo")
	require.NoError(t, os.Mkdir(repo, 0o755))
	victim := filepath.Join(top, "victim.txt")
	require.NoError(t, os.WriteFile(victim, nil, 0o644))
	require.NoError(t, os.Symlink("../victim.txt", filepath.Join(repo, "CHANGELOG.md")))
	t.Chdir(repo)

	_, err := newWriter("CHANGELOG.md").writeChangelog("new\n")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
	b, readErr := os.ReadFile(victim)
	require.NoError(t, readErr)
	assert.Empty(t, b, "the file outside the repository is untouched")
}

// The existing changelog is read under the same rule, so a symlink out of the repository fails
// before any rendering.
func TestReadChangelog_RefusesASymlinkLeavingTheRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	top := t.TempDir()
	repo := filepath.Join(top, "repo")
	require.NoError(t, os.Mkdir(repo, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(top, "secret.txt"), []byte("secret\n"), 0o644))
	require.NoError(t, os.Symlink("../secret.txt", filepath.Join(repo, "CHANGELOG.md")))
	t.Chdir(repo)

	got, err := newWriter("CHANGELOG.md").readChangelog()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the repository")
	assert.Empty(t, got)
}

// A missing changelog reads as empty, so the first run bootstraps it.
func TestReadChangelog_MissingFileIsEmpty(t *testing.T) {
	t.Chdir(t.TempDir())

	got, err := newWriter("CHANGELOG.md").readChangelog()

	require.NoError(t, err)
	assert.Empty(t, got)
}

// A changelog in a directory that does not exist yet is not refused for that alone.
func TestReadChangelog_MissingDirectoryIsEmpty(t *testing.T) {
	t.Chdir(t.TempDir())

	got, err := newWriter("docs/CHANGELOG.md").readChangelog()

	require.NoError(t, err)
	assert.Empty(t, got)
}
