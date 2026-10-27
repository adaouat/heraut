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
	dir := t.TempDir()
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
	dir := t.TempDir()
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
	dir := t.TempDir()
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
	dir := t.TempDir()
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
	dir := t.TempDir()
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
	dir := t.TempDir()
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
	dir := t.TempDir()
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
	dir := t.TempDir()
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
