package native

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTemplateSet_InlineOverridesBlock(t *testing.T) {
	base := `{{ define "root" }}[{{ template "commit" . }}]{{ end }}{{ define "commit" }}builtin{{ end }}`
	ts, err := buildTemplateSet(base, map[string]string{"commit": "OVERRIDDEN"}, "")
	require.NoError(t, err)
	var sb strings.Builder
	require.NoError(t, ts.ExecuteTemplate(&sb, "root", nil))
	assert.Equal(t, "[OVERRIDDEN]", sb.String())
}

func TestBuildTemplateSet_BadSnippetErrors(t *testing.T) {
	base := `{{ define "root" }}{{ end }}`
	_, err := buildTemplateSet(base, map[string]string{"commit": "{{ .Bad "}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "commit")
}

func TestBuildTemplateSet_FileOverridesRoot(t *testing.T) {
	base := `{{ define "root" }}builtin-root{{ end }}`
	dir := t.TempDir()
	file := filepath.Join(dir, "custom.tmpl")
	require.NoError(t, os.WriteFile(file, []byte(`{{ define "root" }}file-root{{ end }}`), 0o644))

	ts, err := buildTemplateSet(base, nil, file)
	require.NoError(t, err)
	var sb strings.Builder
	require.NoError(t, ts.ExecuteTemplate(&sb, "root", nil))
	assert.Equal(t, "file-root", sb.String(), "the template file wins over the built-in root")
}

func TestBuildTemplateSet_MissingFileErrors(t *testing.T) {
	base := `{{ define "root" }}{{ end }}`
	_, err := buildTemplateSet(base, nil, "/nonexistent/does-not-exist.tmpl")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist.tmpl")
}

// TestBuildTemplateSet_EmptySnippetNullsBlock proves an explicit "" override actually nulls a
// non-preamble block. Before T251, only title/subtitle could be nulled (via the Go-level
// execPreambleBlock short-circuit) — every other block silently kept its built-in body, because
// Go's text/template treats an empty {{define}} body as a no-op redefinition.
func TestBuildTemplateSet_EmptySnippetNullsBlock(t *testing.T) {
	base := `{{ define "root" }}[{{ template "commit" . }}]{{ end }}{{ define "commit" }}builtin{{ end }}`
	ts, err := buildTemplateSet(base, map[string]string{"commit": ""}, "")
	require.NoError(t, err)
	var sb strings.Builder
	require.NoError(t, ts.ExecuteTemplate(&sb, "root", nil))
	assert.Equal(t, "[]", sb.String(), "an explicit empty override nulls the block")
}

// TestBuildTemplateSet_WhitespaceSnippetNullsBlock proves a whitespace-only override nulls a block
// the same way an exact "" does — the pre-T251 execPreambleBlock guard matched only the literal
// string "", so "   " hit the underlying Go quirk with no guard at all.
func TestBuildTemplateSet_WhitespaceSnippetNullsBlock(t *testing.T) {
	base := `{{ define "root" }}[{{ template "commit" . }}]{{ end }}{{ define "commit" }}builtin{{ end }}`
	ts, err := buildTemplateSet(base, map[string]string{"commit": "   "}, "")
	require.NoError(t, err)
	var sb strings.Builder
	require.NoError(t, ts.ExecuteTemplate(&sb, "root", nil))
	assert.Equal(t, "[]", sb.String(), "a whitespace-only override nulls the block same as an exact empty string")
}

// TestBuildTemplateSet_FileWinsOverNullSnippet proves a <driver>.template file redefining a block
// wins over an empty snippet override for that same block — restoring the documented "a template
// file always wins outright" precedence rule that the pre-T251 execPreambleBlock short-circuit
// inverted for title/subtitle.
func TestBuildTemplateSet_FileWinsOverNullSnippet(t *testing.T) {
	base := `{{ define "root" }}[{{ template "commit" . }}]{{ end }}{{ define "commit" }}builtin{{ end }}`
	dir := t.TempDir()
	file := filepath.Join(dir, "custom.tmpl")
	require.NoError(t, os.WriteFile(file, []byte(`{{ define "commit" }}from-file{{ end }}`), 0o644))

	ts, err := buildTemplateSet(base, map[string]string{"commit": ""}, file)
	require.NoError(t, err)
	var sb strings.Builder
	require.NoError(t, ts.ExecuteTemplate(&sb, "root", nil))
	assert.Equal(t, "[from-file]", sb.String(), "the template file wins over a null snippet override")
}

// TestRenderReleaseNotes_InlineCommitOverride proves an inline commit snippet flows through the
// render path (snippets -> execBlocks -> buildTemplateSet) and replaces the built-in commit line.
func TestRenderReleaseNotes_InlineCommitOverride(t *testing.T) {
	groups := []group{{name: "🚀 Features", order: 0, commits: []parsedCommit{parsedFrom("aaaaaaa", "feat: add thing")}}}
	snippets := map[string]string{"commit": "> {{ .Description }} [{{ .ShortHash }}]"}

	got, err := renderReleaseNotes(
		"v1.0.0", "", fixedDate1, groups, githubLC, nil, time.Time{}, 3, nil, nil, tplHeraut{}, snippets, "",
	)
	require.NoError(t, err)
	assert.Contains(t, got, "> Add thing [aaaaaaa]", "the inline commit snippet replaces the built-in line")
	assert.NotContains(t, got, "- *(", "the built-in commit format is gone")
}
