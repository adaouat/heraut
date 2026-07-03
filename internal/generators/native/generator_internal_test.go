package native

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
)

func TestGenerator_CheckValidateDegraded(t *testing.T) {
	g := New(exectest.NewMockRunner(), &config.ContentDriver{Generator: "native"}, ModeChangelog)
	assert.NoError(t, g.Check(), "no external binary → Check always succeeds")
	assert.NoError(t, g.Validate())
	assert.False(t, g.Degraded(), "Phase 1 has no enrichment → never degraded")
}

func TestGenerator_GenerateReleaseNotes(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil) // previousTag: git describe
	mr.QueueResponse(record("abc1234567", "Alice", "alice@example.com",
		"2026-01-02T00:00:00Z", "feat: add the thing", ""), "", nil) // collectCommits: git log

	g := New(mr, &config.ContentDriver{Generator: "native"}, ModeReleaseNotes)
	out, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "### 🚀 Features")
	assert.Contains(t, out, "Add the thing")
	assert.Contains(t, out, "Commit Statistics")

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"describe", "--tags", "--abbrev=0", "v1.1.0^"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"log", "v1.0.0..v1.1.0", "--reverse", "--format=" + logFormat}, mr.Calls[1].Args)
}

// TestGenerator_GenerateReleaseNotes_TagGlob verifies native scopes the previous-tag lookup to
// the env glob (per-env strategy support, T138): git describe gains --match <glob>.
func TestGenerator_GenerateReleaseNotes_TagGlob(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("prod/v1.0.0\n", "", nil) // previousTag: git describe --match prod/v*
	mr.QueueResponse(record("abc1234567", "Alice", "alice@example.com",
		"2026-01-02T00:00:00Z", "feat: add the thing", ""), "", nil) // collectCommits

	g := New(mr, &config.ContentDriver{Generator: "native", TagGlob: "prod/v*"}, ModeReleaseNotes)
	_, err := g.Generate("prod/v1.1.0", nil)
	require.NoError(t, err)

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"describe", "--tags", "--abbrev=0", "--match", "prod/v*", "prod/v1.1.0^"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"log", "prod/v1.0.0..prod/v1.1.0", "--reverse", "--format=" + logFormat}, mr.Calls[1].Args)
}

// TestGenerator_GenerateChangelog_TagGlob verifies native scopes tag listing to the env glob.
func TestGenerator_GenerateChangelog_TagGlob(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("prod/v1.0.0\n", "", nil) // listTags: git tag -l prod/v*
	mr.QueueResponse(record("aaa1111111", "A", "a@example.com",
		"2026-02-01T00:00:00Z", "feat: brand new", ""), "", nil) // new release: prod/v1.0.0..HEAD
	mr.QueueResponse(record("bbb2222222", "B", "b@example.com",
		"2026-01-01T00:00:00Z", "fix: an old bug", ""), "", nil) // existing prod/v1.0.0

	g := New(mr, &config.ContentDriver{Generator: "native", TagGlob: "prod/v*"}, ModeChangelog)
	_, err := g.Generate("prod/v1.1.0", nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"tag", "-l", "prod/v*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestGenerator_GenerateChangelog_WritesFile(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil) // listTags: git tag -l (one existing tag)
	mr.QueueResponse(record("aaa1111111", "A", "a@example.com",
		"2026-02-01T00:00:00Z", "feat: brand new", ""), "", nil) // new release: v1.0.0..HEAD
	mr.QueueResponse(record("bbb2222222", "B", "b@example.com",
		"2026-01-01T00:00:00Z", "fix: an old bug", ""), "", nil) // existing v1.0.0

	dir := t.TempDir()
	outPath := filepath.Join(dir, "CHANGELOG.md")
	g := New(mr, &config.ContentDriver{Generator: "native", Output: outPath}, ModeChangelog)

	body, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)

	assert.Contains(t, body, "# Changelog")
	assert.Contains(t, body, "## [1.1.0]", "the unreleased section uses the new tag")
	assert.Contains(t, body, "## [1.0.0]", "plus a section per existing tag")

	written, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, body, string(written), "changelog is also written to cfg.Output")
}

func TestGenerator_GenerateChangelog_FirstRelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // listTags: no tags yet
	mr.QueueResponse(record("ccc3333333", "C", "c@example.com",
		"2026-01-01T00:00:00Z", "feat: initial", ""), "", nil) // new release: full history (HEAD)

	g := New(mr, &config.ContentDriver{Generator: "native"}, ModeChangelog)
	body, err := g.Generate("v0.1.0", nil)
	require.NoError(t, err)
	assert.Contains(t, body, "## [0.1.0]")

	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"tag", "-l", "--sort=-version:refname"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"log", "HEAD", "--reverse", "--format=" + logFormat}, mr.Calls[1].Args)
}
