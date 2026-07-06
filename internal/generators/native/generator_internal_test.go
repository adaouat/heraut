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
	mr.QueueResponse("v1.0.0\n", "", nil)               // previousTag: git describe
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil) // tagDate: git log -1 --format=%cI v1.0.0
	mr.QueueResponse(record("abc1234567", "Alice", "alice@example.com",
		"2026-01-02T00:00:00Z", "feat: add the thing", ""), "", nil) // collectCommits: git log
	mr.QueueResponse("alice@example.com\n", "", nil) // authorsBefore: git log v1.0.0 --format=%ae

	g := New(mr, &config.ContentDriver{Generator: "native"}, ModeReleaseNotes)
	out, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "### 🚀 Features")
	assert.Contains(t, out, "Add the thing")
	assert.Contains(t, out, "Commit Statistics")

	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"describe", "--tags", "--abbrev=0", "v1.1.0^"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"log", "-1", "--format=%cI", "v1.0.0"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"log", "v1.0.0..v1.1.0", "--reverse", "--format=" + logFormat}, mr.Calls[2].Args)
	assert.Equal(t, []string{"log", "v1.0.0", "--format=%ae"}, mr.Calls[3].Args)
}

// TestGenerator_GenerateReleaseNotes_TagGlob verifies native scopes the previous-tag lookup to
// the env glob (per-env strategy support, T138): git describe gains --match <glob>.
func TestGenerator_GenerateReleaseNotes_TagGlob(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("prod/v1.0.0\n", "", nil)          // previousTag: git describe --match prod/v*
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil) // tagDate: git log -1 --format=%cI prod/v1.0.0
	mr.QueueResponse(record("abc1234567", "Alice", "alice@example.com",
		"2026-01-02T00:00:00Z", "feat: add the thing", ""), "", nil) // collectCommits
	mr.QueueResponse("alice@example.com\n", "", nil) // authorsBefore: git log prod/v1.0.0 --format=%ae

	g := New(mr, &config.ContentDriver{Generator: "native", TagGlob: "prod/v*"}, ModeReleaseNotes)
	_, err := g.Generate("prod/v1.1.0", nil)
	require.NoError(t, err)

	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"describe", "--tags", "--abbrev=0", "--match", "prod/v*", "prod/v1.1.0^"}, mr.Calls[0].Args)
	assert.Equal(t, []string{"log", "-1", "--format=%cI", "prod/v1.0.0"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"log", "prod/v1.0.0..prod/v1.1.0", "--reverse", "--format=" + logFormat}, mr.Calls[2].Args)
	assert.Equal(t, []string{"log", "prod/v1.0.0", "--format=%ae"}, mr.Calls[3].Args)
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

// TestGenerator_GenerateReleaseNotes_TagPatternRegex verifies native honours an explicit
// tag_pattern as a Go regex (T139): it lists ALL tags (no git-side glob) and Go-filters, then
// resolves the previous tag from the filtered list.
func TestGenerator_GenerateReleaseNotes_TagPatternRegex(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v2.0.0-prod\nv1.0.0-dev\nv1.0.0-prod\n", "", nil) // scopedTags: git tag -l (all)
	mr.QueueResponse("2026-02-01T00:00:00Z\n", "", nil)                 // tagDate: git log -1 --format=%cI v1.0.0-prod
	mr.QueueResponse(record("abc1234567", "A", "a@example.com",
		"2026-02-02T00:00:00Z", "feat: x", ""), "", nil) // collectCommits
	mr.QueueResponse("a@example.com\n", "", nil) // authorsBefore: git log v1.0.0-prod --format=%ae

	g := New(mr, &config.ContentDriver{Generator: "native", TagPattern: `-prod$`}, ModeReleaseNotes)
	_, err := g.Generate("v2.0.0-prod", nil)
	require.NoError(t, err)

	require.Len(t, mr.Calls, 4)
	assert.Equal(t, []string{"tag", "-l", "--sort=-version:refname"}, mr.Calls[0].Args,
		"regex mode lists all tags; filtering happens in Go")
	assert.Equal(t, []string{"log", "-1", "--format=%cI", "v1.0.0-prod"}, mr.Calls[1].Args,
		"prev (v1.0.0-prod) resolved from the -prod-filtered list, skipping v1.0.0-dev")
	assert.Equal(t, []string{"log", "v1.0.0-prod..v2.0.0-prod", "--reverse", "--format=" + logFormat}, mr.Calls[2].Args)
	assert.Equal(t, []string{"log", "v1.0.0-prod", "--format=%ae"}, mr.Calls[3].Args)
}

func TestFilterByTagPattern(t *testing.T) {
	tags := []string{"v2.0.0-prod", "v1.0.0-dev", "v1.0.0-prod"}
	got, err := filterByTagPattern(tags, `-prod$`)
	require.NoError(t, err)
	assert.Equal(t, []string{"v2.0.0-prod", "v1.0.0-prod"}, got)

	all, err := filterByTagPattern(tags, "")
	require.NoError(t, err)
	assert.Equal(t, tags, all, "empty pattern keeps every tag")

	_, err = filterByTagPattern(tags, `[`)
	require.Error(t, err, "invalid regex is an error")
}

func TestPreviousInList(t *testing.T) {
	tags := []string{"v3-prod", "v2-prod", "v1-prod"} // newest-first
	assert.Equal(t, "v2-prod", previousInList("v3-prod", tags))
	assert.Equal(t, "", previousInList("v1-prod", tags), "oldest has no predecessor")
	assert.Equal(t, "v3-prod", previousInList("v4-prod", tags), "new release → newest existing")
	assert.Equal(t, "", previousInList("v1", nil), "empty list")
}

// TestGenerator_GenerateReleaseNotes_DaysBetweenReleases verifies native resolves the previous
// tag's date and emits the "days passed between releases" stat (T140).
func TestGenerator_GenerateReleaseNotes_DaysBetweenReleases(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)               // previousTag: git describe
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil) // tagDate: git log -1 --format=%cI v1.0.0
	mr.QueueResponse(record("abc1234567", "A", "a@example.com",
		"2026-01-11T00:00:00Z", "feat: x", ""), "", nil) // collectCommits (release date +10 days)
	mr.QueueResponse("a@example.com\n", "", nil) // authorsBefore: git log v1.0.0 --format=%ae

	g := New(mr, &config.ContentDriver{Generator: "native"}, ModeReleaseNotes)
	out, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"log", "-1", "--format=%cI", "v1.0.0"}, mr.Calls[1].Args)
	assert.Equal(t, []string{"log", "v1.0.0", "--format=%ae"}, mr.Calls[3].Args)
	assert.Contains(t, out, "10 day(s) passed between releases")
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

func TestGenerate_InlineCommitOverride(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)               // previousTag
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil) // tagDate
	mr.QueueResponse(record("abc1234567", "A", "a@example.com", "2026-01-02T00:00:00Z", "feat: add x", ""), "", nil)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore
	g := New(mr, &config.ContentDriver{
		Generator:          "native",
		EffectiveTemplates: map[string]string{"commit": "CUSTOM {{ .Description }}"},
	}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "CUSTOM Add x", "inline commit override is applied")
	assert.NotContains(t, out, "- *(", "built-in commit format is replaced")
}

func TestGenerate_TemplateFileOverride(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.tmpl")
	require.NoError(t, os.WriteFile(file, []byte(
		`{{ define "release-notes" }}FILE-NOTES{{ range .Groups }}{{ range .Commits }}
- {{ .Description }}{{ end }}{{ end }}{{ end }}`), 0o644))

	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)               // previousTag
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil) // tagDate
	mr.QueueResponse(record("abc1234567", "A", "a@example.com", "2026-01-02T00:00:00Z", "feat: add x", ""), "", nil)
	mr.QueueResponse("bob@x\n", "", nil) // authorsBefore
	g := New(mr, &config.ContentDriver{Generator: "native", Template: file}, ModeReleaseNotes)

	out, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "FILE-NOTES", "the template file replaces the release-notes root")
	assert.Contains(t, out, "- Add x")
	assert.NotContains(t, out, "Commit Statistics", "the file root drops the built-in stats block")
}
