# Changelog/Release-Notes Title & Subtitle Blocks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename the native generator's `header` template block to `release_header` (it fires once
per rendered release, not once per document) and `release-notes` to `release_notes` (separator
consistency), and add new `title`/`subtitle` blocks that fire exactly once per document — closing
the gap where the changelog's `# Changelog` line was a hardcoded Go constant, unreachable from any
template override.

**Architecture:** `title`/`subtitle` execute against a minimal `.Heraut`-only context (not the full
`Release`), via a new shared `renderPreamble` helper in `internal/generators/native/render.go`,
called once by `buildAllSections` (changelog: only on bootstrap/`--regenerate`, never on an
ordinary incremental splice) and once by `renderReleaseNotes` (release notes: every render, since
it's already single-release). No new config mechanism — both new blocks are ordinary
`rendering.templates` keys, subject to the existing four-layer precedence chain.

**Tech Stack:** Go `text/template`, existing `internal/generators/native` package, `internal/config`
validator, `schema.json`.

**Spec:** [`docs/superpowers/specs/2026-08-28-changelog-title-subtitle-design.md`](../specs/2026-08-28-changelog-title-subtitle-design.md)

## Global Constraints

- **Byte-identical default output.** `title`'s changelog built-in body is `"# Changelog"`
  (no trailing newline in the define — spacing is added by `renderPreamble`, not the block body),
  reproducing today's `changelogHeader` constant (`"# Changelog\n\n"`) exactly when nothing is
  overridden.
- **No backward-compat alias.** `header` and `release-notes` become config errors
  ("unknown template block") after this ships — no deprecated-alias shim (project is pre-v1.0,
  SemVer 0.x; see `.claude/rules/workflow.md`).
- **`title`/`subtitle` context is `.Heraut` directly** — a bare `tplHeraut` value, so snippets write
  `{{ .Version }}` / `{{ .URL }}` / `{{ .GeneratedAt }}`, **not** `{{ .Heraut.Version }}` (unlike
  `footer`/`release_header`, whose root is the full `Release` and reach the same data via
  `.Heraut.Version`). This asymmetry is deliberate and must be documented everywhere the blocks are
  documented (guide, spec, schema, ADR).
- **No refactor of unrelated code.** `execBlocks`/`buildTemplateSet`'s per-call re-parsing is
  pre-existing and out of scope — do not touch it.
- Every task must leave `go build ./...`, `go test ./...`, and `hk check` passing before its commit.

---

### Task 1: Rename `header` → `release_header`, `release-notes` → `release_notes`

**Files:**
- Modify: `docs/tasks/roadmap.md` (new phase heading, no test)
- Modify: `internal/config/validator.go:369-375`
- Modify: `internal/config/validator_test.go:853-864`
- Modify: `internal/config/commits.go:33-36` (doc comment)
- Modify: `internal/generators/native/changelog.tmpl`
- Modify: `internal/generators/native/release_notes.tmpl`
- Modify: `internal/generators/native/render.go:97` (the `execBlocks` root-name argument)
- Modify: `internal/generators/native/generator_internal_test.go:267,415`
- Modify: `schema.json` (the `templates` object's `properties`)

**Interfaces:**
- Produces: `validTemplateBlocks` (map, `internal/config/validator.go`) now contains
  `"release_header"` and `"release_notes"` in place of `"header"`/`"release-notes"`. Later tasks
  add `"title"`/`"subtitle"` to this same map.

- [ ] **Step 1: Add the roadmap phase heading**

Open `docs/tasks/roadmap.md`. Immediately before the `## Risks and mitigations` heading, insert:

```markdown
### Phase 30 — Changelog/release-notes title & subtitle blocks

Renames the native generator's `header` template block to `release_header` (it fires once per
rendered release, not once per document — the old name was confusing enough to hide a real gap)
and `release-notes` to `release_notes` (separator consistency across every block key). Adds new
`title`/`subtitle` blocks that fire exactly once per document, closing the gap where the
changelog's `# Changelog` line was a hardcoded Go constant unreachable from any template override.
New ADR-0048 (supersedes the block-set table in ADR-0037). Five tasks (rename → changelog
title/subtitle → release-notes title/subtitle → docs → roadmap close-out).

Design: [`docs/superpowers/specs/2026-08-28-changelog-title-subtitle-design.md`](../superpowers/specs/2026-08-28-changelog-title-subtitle-design.md).

---
```

- [ ] **Step 2: Write the failing validator tests (new key names)**

In `internal/config/validator_test.go`, replace the existing
`TestValidate_RenderingTemplatesHyphenatedBlockValid` (lines 853-864) with:

```go
func TestValidate_RenderingTemplatesRenamedBlocksValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    release_header: "## [{{ .Version }}]"
    release_notes: "{{range .Groups}}{{ template \"group\" . }}{{end}}"
`)
	assert.Empty(t, config.Validate(cfg))
}

// The pre-rename block names are config errors after ADR-0048 — no deprecated-alias shim.
func TestValidate_RenderingTemplatesOldHeaderKeyRejected(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    header: "## [{{ .Version }}]"
`)
	e := findErr(config.Validate(cfg), "rendering.templates.header")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "unknown template block")
	assert.Contains(t, e.Hint, "release_header")
}

func TestValidate_RenderingTemplatesOldHyphenatedReleaseNotesKeyRejected(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    release-notes: "{{range .Groups}}{{ template \"group\" . }}{{end}}"
`)
	e := findErr(config.Validate(cfg), "rendering.templates.release-notes")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "unknown template block")
	assert.Contains(t, e.Hint, "release_notes")
}
```

- [ ] **Step 3: Run the validator tests to verify they fail**

Run: `go test ./internal/config/... -run TestValidate_RenderingTemplates -v`
Expected: `TestValidate_RenderingTemplatesRenamedBlocksValid` FAILs (current validator still rejects
`release_header`/`release_notes` as unknown blocks); the two "OldKeyRejected" tests currently PASS
by coincidence (today's validator doesn't recognize those exact strings either, for a different
reason) — that's fine, they'll stay green once you flip the map in Step 4.

- [ ] **Step 4: Rename the block-key set in the validator**

In `internal/config/validator.go`, replace lines 369-375:

```go
var validTemplateBlocks = map[string]bool{
	"header": true, "footer": true, "group": true, "commit": true, "ticket": true,
	"contributor": true, "contributors": true, "stats": true,
	"changelog": true, "release-notes": true,
}

const validTemplateBlocksHint = "valid blocks: changelog, commit, contributor, contributors, footer, group, header, release-notes, stats, ticket"
```

with:

```go
var validTemplateBlocks = map[string]bool{
	"release_header": true, "footer": true, "group": true, "commit": true, "ticket": true,
	"contributor": true, "contributors": true, "stats": true,
	"changelog": true, "release_notes": true,
}

const validTemplateBlocksHint = "valid blocks: changelog, commit, contributor, contributors, footer, group, release_header, release_notes, stats, ticket"
```

- [ ] **Step 5: Rename the block-key doc comment in `internal/config/commits.go`**

Lines 33-36 currently read:

```go
	// Templates overrides built-in native template blocks by key (e.g. "commit", "group",
	// "contributor", "header", "footer"): each value is a Go text/template snippet. native
	// only — deep-merged global → per-driver → per-env (ADR-0037).
	Templates map[string]string `yaml:"templates,omitempty"`
```

Replace with:

```go
	// Templates overrides built-in native template blocks by key (e.g. "commit", "group",
	// "contributor", "release_header", "footer"): each value is a Go text/template snippet.
	// native only — deep-merged global → per-driver → per-env (ADR-0037, ADR-0048).
	Templates map[string]string `yaml:"templates,omitempty"`
```

- [ ] **Step 6: Rename the block in the embedded templates**

`internal/generators/native/changelog.tmpl` currently:

```gotemplate
{{define "header"}}## [{{ .Version }}]{{ if .CompareURL }}({{ .CompareURL }}){{ end }} - {{ date "2006-01-02" .Date }}{{end}}
{{define "changelog"}}{{ template "header" . }}
{{range .Groups}}
{{ template "group" . }}
{{range .Commits}}
{{ template "commit" . }}
{{end}}
{{end}}{{ template "footer" . }}{{end}}
```

Replace with:

```gotemplate
{{define "release_header"}}## [{{ .Version }}]{{ if .CompareURL }}({{ .CompareURL }}){{ end }} - {{ date "2006-01-02" .Date }}{{end}}
{{define "changelog"}}{{ template "release_header" . }}
{{range .Groups}}
{{ template "group" . }}
{{range .Commits}}
{{ template "commit" . }}
{{end}}
{{end}}{{ template "footer" . }}{{end}}
```

`internal/generators/native/release_notes.tmpl` currently:

```gotemplate
{{define "header"}}{{end}}
{{define "release-notes"}}{{ template "header" . }}{{range .Groups}}{{ template "group" . }}
{{range .Commits}}
{{ template "commit" . }}{{ if .Body }}

{{ indent 4 .Body }}{{ end }}{{ if .Footers }}

{{ range $i, $f := .Footers }}{{ if $i }}
{{ end }}  {{ $f.Token }}: {{ $f.Value }}{{ end }}{{ end }}
{{end}}
{{end}}
{{if .Contributors}}{{ template "contributors" . }}{{end}}{{ template "stats" . }}{{ template "footer" . }}{{end}}
```

Replace with:

```gotemplate
{{define "release_header"}}{{end}}
{{define "release_notes"}}{{ template "release_header" . }}{{range .Groups}}{{ template "group" . }}
{{range .Commits}}
{{ template "commit" . }}{{ if .Body }}

{{ indent 4 .Body }}{{ end }}{{ if .Footers }}

{{ range $i, $f := .Footers }}{{ if $i }}
{{ end }}  {{ $f.Token }}: {{ $f.Value }}{{ end }}{{ end }}
{{end}}
{{end}}
{{if .Contributors}}{{ template "contributors" . }}{{end}}{{ template "stats" . }}{{ template "footer" . }}{{end}}
```

- [ ] **Step 7: Rename the root-block-name argument in `render.go`**

In `internal/generators/native/render.go`, `renderReleaseNotes` (around line 97):

```go
	out, err := execBlocks("release-notes", releaseNotesTmpl, snippets, templateFile, rel)
```

becomes:

```go
	out, err := execBlocks("release_notes", releaseNotesTmpl, snippets, templateFile, rel)
```

- [ ] **Step 8: Update the two `generator_internal_test.go` call sites**

Line 267 currently:

```go
		`{{ define "release-notes" }}FILE-NOTES{{ range .Groups }}{{ range .Commits }}
```

becomes:

```go
		`{{ define "release_notes" }}FILE-NOTES{{ range .Groups }}{{ range .Commits }}
```

Line 415 currently:

```go
		EffectiveTemplates: map[string]string{"header": "=== {{ .Version }} ==="},
```

becomes:

```go
		EffectiveTemplates: map[string]string{"release_header": "=== {{ .Version }} ==="},
```

- [ ] **Step 9: Update `schema.json`**

In the `templates` object's `properties` (the block that starts at the `"templates": {` line
under `Rendering`), rename the `"header"` property key to `"release_header"` and update its
description; rename `"release-notes"` to `"release_notes"` and update its description; update the
object's own top-level `description` to mention ADR-0048. Full replacement of that `properties`
block:

```json
          "properties": {
            "release_header": {
              "type": "string",
              "description": "One release's version heading, fired once per rendered release. Changelog default: '## [version](compare-url) - date'. Release-notes default: empty."
            },
            "group": {
              "type": "string",
              "description": "Commit-type group heading (e.g. '### 🚀 Features')."
            },
            "commit": {
              "type": "string",
              "description": "One commit line: description, scope, breaking marker, hash link, 'by @author', PR/MR reference, ticket links."
            },
            "ticket": {
              "type": "string",
              "description": "One ticket link within a commit line (called once per match from the commit block). Default: '([TICKET](url))'."
            },
            "contributor": {
              "type": "string",
              "description": "One entry in the New Contributors block for a first-time contributor."
            },
            "contributors": {
              "type": "string",
              "description": "The 'New Contributors ❤️' block wrapping the contributor entries (release notes)."
            },
            "stats": {
              "type": "string",
              "description": "The Commit Statistics block (commit/issue counts, timespan) in release notes."
            },
            "footer": {
              "type": "string",
              "description": "Trailing content appended after the body. Empty by default."
            },
            "changelog": {
              "type": "string",
              "description": "Root changelog-section template composing release_header, group, and commit blocks."
            },
            "release_notes": {
              "type": "string",
              "description": "Root release-notes template composing release_header, group, commit, contributors, and stats blocks."
            }
          }
```

And the object's `"description"` field, one level up, from:

```json
          "description": "Override built-in native template blocks. Each value is a Go text/template snippet. native only; deep-merged global -> per-driver -> per-env (ADR-0037).",
```

to:

```json
          "description": "Override built-in native template blocks. Each value is a Go text/template snippet. native only; deep-merged global -> per-driver -> per-env (ADR-0037, ADR-0048).",
```

- [ ] **Step 10: Run the full test suite and lint**

Run: `go build ./... && go test ./... -count=1 && hk check`
Expected: all green. `TestValidate_RenderingTemplatesRenamedBlocksValid` and the two
"OldKeyRejected" tests pass; `TestGenerateChangelog_IncrementalWithCustomHeader` and the template-
file-override test in `generator_internal_test.go` pass with their updated key names.

- [ ] **Step 11: Commit**

```bash
git add docs/tasks/roadmap.md internal/config/validator.go internal/config/validator_test.go \
  internal/config/commits.go internal/generators/native/changelog.tmpl \
  internal/generators/native/release_notes.tmpl internal/generators/native/render.go \
  internal/generators/native/generator_internal_test.go schema.json
git commit -m "$(cat <<'EOF'
refactor(generators/native): rename header block to release_header, release-notes to release_notes

The header block fires once per rendered release, not once per document — the
old name read as document-scoped and hid the real gap (no way to override the
changelog's document title). release-notes is renamed to release_notes for
separator consistency across every block key. Breaking rename, no alias: both
old keys are now a config error naming the new one.
EOF
)"
```

---

### Task 2: Add `title`/`subtitle` blocks for the changelog driver

**Files:**
- Modify: `internal/generators/native/changelog.tmpl`
- Modify: `internal/generators/native/render.go` (new `renderPreamble` function)
- Modify: `internal/generators/native/generator.go` (`buildAllSections`)
- Modify: `internal/generators/native/render_internal_test.go` (replace `TestChangelogHeader`,
  add `renderPreamble` unit tests)
- Modify: `internal/generators/native/generator_internal_test.go` (new integration tests)
- Modify: `internal/config/validator.go` (`validTemplateBlocks` gains `title`/`subtitle`)
- Modify: `internal/config/validator_test.go` (new acceptance test)
- Modify: `schema.json` (new `title`/`subtitle` properties)

**Interfaces:**
- Consumes: `execBlocks(rootName, rootTmpl string, snippets map[string]string, templateFile string, data any) (string, error)` (existing, `internal/generators/native/render.go`); `tplHeraut` (existing, `internal/generators/native/templatemodel.go`); `g.herautMeta() tplHeraut` (existing method, `internal/generators/native/generator.go`); `changelogTmpl string` (existing package var, `internal/generators/native/render.go`).
- Produces: `renderPreamble(rootTmpl string, snippets map[string]string, templateFile string, heraut tplHeraut) (string, error)` — new function in `internal/generators/native/render.go`. Task 3 reuses it unchanged for the release-notes driver.

- [ ] **Step 1: Write the failing `renderPreamble` unit tests**

In `internal/generators/native/render_internal_test.go`, replace the existing
`TestChangelogHeader` (lines 508-512, including its `// ─── changelogHeader constant ───` banner
comment) with:

```go
// ─── renderPreamble (title/subtitle) ──────────────────────────────────────────

func TestRenderPreamble_ChangelogDefault(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, nil, "", tplHeraut{})
	require.NoError(t, err)
	assert.Equal(t, "# Changelog\n\n", out, "byte-identical to the former changelogHeader constant")
}

func TestRenderPreamble_ChangelogTitleOnly(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, map[string]string{"title": "# MyApp"}, "", tplHeraut{})
	require.NoError(t, err)
	assert.Equal(t, "# MyApp\n\n", out)
}

func TestRenderPreamble_ChangelogTitleAndSubtitle(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, map[string]string{
		"title":    "# MyApp",
		"subtitle": "All notable changes.",
	}, "", tplHeraut{})
	require.NoError(t, err)
	assert.Equal(t, "# MyApp\n\nAll notable changes.\n\n", out)
}

func TestRenderPreamble_ChangelogSubtitleOnly(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, map[string]string{
		"title":    "",
		"subtitle": "Just a subtitle.",
	}, "", tplHeraut{})
	require.NoError(t, err)
	assert.Equal(t, "Just a subtitle.\n\n", out, "no leading blank line when title is explicitly nulled")
}

func TestRenderPreamble_ChangelogBothEmpty(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, map[string]string{"title": ""}, "", tplHeraut{})
	require.NoError(t, err)
	assert.Empty(t, out, "an explicitly nulled title with no subtitle renders nothing, no stray blank line")
}

func TestRenderPreamble_HerautContextIsBareFields(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, map[string]string{
		"title": "Changelog (heraut v{{ .Version }})",
	}, "", tplHeraut{Version: "0.59.0"})
	require.NoError(t, err)
	assert.Equal(t, "Changelog (heraut v0.59.0)\n\n", out,
		"title's root IS tplHeraut directly — .Version, not .Heraut.Version")
}
```

- [ ] **Step 2: Run to verify the new tests fail**

Run: `go test ./internal/generators/native/... -run TestRenderPreamble -v`
Expected: FAIL with `undefined: renderPreamble` (compile error — the function doesn't exist yet).

- [ ] **Step 3: Add `title`/`subtitle` to `changelog.tmpl`**

Prepend two new defines above the existing `release_header` define (added in Task 1):

```gotemplate
{{define "title"}}# Changelog{{end}}
{{define "subtitle"}}{{end}}
{{define "release_header"}}## [{{ .Version }}]{{ if .CompareURL }}({{ .CompareURL }}){{ end }} - {{ date "2006-01-02" .Date }}{{end}}
{{define "changelog"}}{{ template "release_header" . }}
{{range .Groups}}
{{ template "group" . }}
{{range .Commits}}
{{ template "commit" . }}
{{end}}
{{end}}{{ template "footer" . }}{{end}}
```

- [ ] **Step 4: Implement `renderPreamble` in `render.go`**

Add near `execBlocks` (end of the file, after the `execBlocks` function):

```go
// renderPreamble renders a driver's document-level title+subtitle blocks (ADR-0048): unlike every
// other block, these fire once per document, not once per rendered release, so they execute
// against a bare tplHeraut (not a Release) — snippets write .Version/.URL/.GeneratedAt directly,
// not .Heraut.Version. Each is independently trimmed and blank-line-joined: an unset block
// contributes nothing, so no stray blank line survives whether both, one, or neither is set.
func renderPreamble(rootTmpl string, snippets map[string]string, templateFile string, heraut tplHeraut) (string, error) {
	title, err := execBlocks("title", rootTmpl, snippets, templateFile, heraut)
	if err != nil {
		return "", fmt.Errorf("rendering title: %w", err)
	}
	subtitle, err := execBlocks("subtitle", rootTmpl, snippets, templateFile, heraut)
	if err != nil {
		return "", fmt.Errorf("rendering subtitle: %w", err)
	}
	var parts []string
	if t := strings.TrimSpace(title); t != "" {
		parts = append(parts, t)
	}
	if s := strings.TrimSpace(subtitle); s != "" {
		parts = append(parts, s)
	}
	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, "\n\n") + "\n\n", nil
}
```

Then delete the now-unused `changelogHeader` constant and its comment (lines 16-18):

```go
// changelogHeader is the fixed file-level header for a CHANGELOG.md.
// Release-notes have no header (the platform renders the version heading).
const changelogHeader = "# Changelog\n\n"
```

- [ ] **Step 5: Run the `renderPreamble` unit tests again**

Run: `go test ./internal/generators/native/... -run TestRenderPreamble -v`
Expected: PASS.

- [ ] **Step 6: Write the failing generator-level integration tests**

In `internal/generators/native/generator_internal_test.go`, add (near
`TestGenerator_GenerateChangelog_WritesFile`):

```go
func TestGenerator_GenerateChangelog_TitleSubtitleFireOncePerDocument(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil) // listTags: git tag -l (one existing tag)
	mr.QueueResponse(record("aaa1111111", "A", "a@example.com",
		"2026-02-01T00:00:00Z", "feat: brand new", ""), "", nil) // new release: v1.0.0..HEAD
	mr.QueueResponse(record("bbb2222222", "B", "b@example.com",
		"2026-01-01T00:00:00Z", "fix: an old bug", ""), "", nil) // existing v1.0.0

	dir := t.TempDir()
	g := New(mr, &config.ContentDriver{
		Output: filepath.Join(dir, "CHANGELOG.md"),
		EffectiveTemplates: map[string]string{
			"title":    "# MyApp Changelog",
			"subtitle": "All notable changes.",
		},
	}, ModeChangelog)

	body, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)

	assert.Equal(t, 1, strings.Count(body, "# MyApp Changelog"),
		"title renders once for the whole document, not once per section")
	assert.Equal(t, 1, strings.Count(body, "All notable changes."))
	assert.True(t, strings.HasPrefix(body, "# MyApp Changelog\n\nAll notable changes.\n\n"),
		"title+subtitle open the file")
}

func TestGenerator_GenerateChangelog_IncrementalPreservesExistingTitle(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil) // scopedTags for the splice path's latest lookup
	mr.QueueResponse(record("ccc3333333", "C", "c@example.com",
		"2026-03-01T00:00:00Z", "feat: another one", ""), "", nil) // new range v1.0.0..HEAD

	dir := t.TempDir()
	outPath := filepath.Join(dir, "CHANGELOG.md")
	existing := "# Totally Custom Title\n\nHand-edited subtitle.\n\n<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n### Features\n\n- first\n"
	require.NoError(t, os.WriteFile(outPath, []byte(existing), 0o644))

	g := New(mr, &config.ContentDriver{
		Output:             outPath,
		EffectiveTemplates: map[string]string{"title": "# Should Not Appear"},
	}, ModeChangelog)

	body, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(body, "# Totally Custom Title\n\nHand-edited subtitle.\n\n"),
		"an ordinary incremental splice never re-renders the preamble, even with a title override configured")
	assert.NotContains(t, body, "Should Not Appear")
}

func TestGenerator_GenerateChangelog_RegenerateRerendersTitle(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil) // listTags: git tag -l
	mr.QueueResponse(record("aaa1111111", "A", "a@example.com",
		"2026-02-01T00:00:00Z", "feat: brand new", ""), "", nil) // new release: v1.0.0..HEAD
	mr.QueueResponse(record("bbb2222222", "B", "b@example.com",
		"2026-01-01T00:00:00Z", "fix: an old bug", ""), "", nil) // existing v1.0.0

	dir := t.TempDir()
	outPath := filepath.Join(dir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(outPath, []byte(
		"# Stale Title\n\n<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n### Features\n\n- first\n"), 0o644))

	g := New(mr, &config.ContentDriver{
		Output:              outPath,
		RegenerateChangelog: true,
		EffectiveTemplates:  map[string]string{"title": "# Fresh Title"},
	}, ModeChangelog)

	body, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(body, "# Fresh Title\n\n"),
		"--regenerate re-renders the title from the current config, discarding the stale preamble")
	assert.NotContains(t, body, "Stale Title")
}

func TestGenerator_GenerateChangelog_TitleContextIsHerautOnly(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)
	mr.QueueResponse(record("aaa1111111", "A", "a@example.com",
		"2026-02-01T00:00:00Z", "feat: brand new", ""), "", nil)
	mr.QueueResponse(record("bbb2222222", "B", "b@example.com",
		"2026-01-01T00:00:00Z", "fix: an old bug", ""), "", nil)

	dir := t.TempDir()
	g := New(mr, &config.ContentDriver{
		Output:             filepath.Join(dir, "CHANGELOG.md"),
		EffectiveTemplates: map[string]string{"title": "{{ .Version }}"}, // .Version exists on Release, not tplHeraut
	}, ModeChangelog)
	_, err := g.Generate("v1.1.0", nil)
	require.Error(t, err, "title only receives .Heraut's own fields — .Version must fail to execute")
	assert.Contains(t, err.Error(), "title")
}
```

- [ ] **Step 7: Run to verify these fail**

Run: `go test ./internal/generators/native/... -run TestGenerator_GenerateChangelog -v`
Expected: the four new tests FAIL (title/subtitle not yet wired into `buildAllSections` — output
still starts with the old hardcoded `"# Changelog\n\n"`, ignoring the `EffectiveTemplates`
override; the "TitleContextIsHerautOnly" test currently fails differently, since `title` isn't
even a recognized key in `EffectiveTemplates` execution yet — either failure mode is acceptable
red, since the feature doesn't exist).

- [ ] **Step 8: Wire `renderPreamble` into `buildAllSections`**

In `internal/generators/native/generator.go`, replace the return line of `buildAllSections`
(currently `return changelogHeader + strings.Join(blocks, "\n\n") + "\n", nil`) — full new
function body:

```go
func (g *Generator) buildAllSections(tag string, lc *port.LinkContext, enrichAll bool) (string, error) {
	tags, err := g.scopedTags()
	if err != nil {
		return "", err
	}

	preamble, err := renderPreamble(changelogTmpl, g.cfg.EffectiveTemplates, g.cfg.Template, g.herautMeta())
	if err != nil {
		return "", fmt.Errorf("rendering changelog title: %w", err)
	}

	var blocks []string

	latest := g.newSectionBound(tags)
	if sec, err := g.renderRelease(tag, latest, "HEAD", lc, true); err != nil {
		return "", err
	} else if sec != "" {
		blocks = append(blocks, anchorLine(tag)+"\n"+sec)
	}

	// Existing releases, newest-first. prev is the next-older tag by version refname (listTags
	// is version-sorted); release-notes mode instead resolves prev via git-describe topology.
	// Equivalent for linear history — the common case.
	for i, t := range tags {
		prev := ""
		if i+1 < len(tags) {
			prev = tags[i+1]
		}
		if sec, err := g.renderRelease(t, prev, t, lc, enrichAll); err != nil {
			return "", err
		} else if sec != "" {
			blocks = append(blocks, anchorLine(t)+"\n"+sec)
		}
	}

	return preamble + strings.Join(blocks, "\n\n") + "\n", nil
}
```

(Only the new `preamble` block and the final `return` line actually change; the rest of the
function body is unchanged — reproduced in full here so the diff is unambiguous.)

- [ ] **Step 9: Run the full native package test suite**

Run: `go test ./internal/generators/native/... -v`
Expected: all PASS, including the four new integration tests and every pre-existing test
(`TestGenerator_GenerateChangelog_WritesFile`'s `assert.Contains(t, body, "# Changelog")` still
holds — the default title body is unchanged).

- [ ] **Step 10: Add `title`/`subtitle` to the validator's block set**

In `internal/config/validator.go`, replace the `validTemplateBlocks` map and hint (set by Task 1)
with:

```go
var validTemplateBlocks = map[string]bool{
	"title": true, "subtitle": true, "release_header": true, "footer": true, "group": true,
	"commit": true, "ticket": true, "contributor": true, "contributors": true, "stats": true,
	"changelog": true, "release_notes": true,
}

const validTemplateBlocksHint = "valid blocks: changelog, commit, contributor, contributors, footer, group, release_header, release_notes, stats, subtitle, ticket, title"
```

- [ ] **Step 11: Add the validator acceptance test**

In `internal/config/validator_test.go`, add:

```go
func TestValidate_RenderingTemplatesTitleSubtitleValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
rendering:
  templates:
    title: "# MyApp Changelog"
    subtitle: "All notable changes."
`)
	assert.Empty(t, config.Validate(cfg))
}
```

- [ ] **Step 12: Add `title`/`subtitle` to `schema.json`**

In the same `properties` object edited in Task 1, add two new entries at the top (before
`release_header`):

```json
            "title": {
              "type": "string",
              "description": "Document-level title, fired once per document (not once per release). Executes against .Heraut's own fields directly (.Version .URL .GeneratedAt), not the full release data — write {{ .Version }}, not {{ .Heraut.Version }}. Changelog default: '# Changelog'. Release-notes default: empty."
            },
            "subtitle": {
              "type": "string",
              "description": "Document-level subtitle rendered under title, fired once per document. Same .Heraut-only context as title. Empty by default on both drivers."
            },
```

- [ ] **Step 13: Run the full suite and lint**

Run: `go build ./... && go test ./... -count=1 && hk check`
Expected: all green.

- [ ] **Step 14: Commit**

```bash
git add internal/generators/native/changelog.tmpl internal/generators/native/render.go \
  internal/generators/native/generator.go internal/generators/native/render_internal_test.go \
  internal/generators/native/generator_internal_test.go internal/config/validator.go \
  internal/config/validator_test.go schema.json
git commit -m "$(cat <<'EOF'
feat(generators/native): add title/subtitle document-level template blocks

Closes the gap where the changelog's "# Changelog" line was a hardcoded Go
constant unreachable from any template override. title/subtitle fire once
per document (not once per release, unlike every other block) via a new
renderPreamble helper, executed against a bare .Heraut context. Default
changelog output is byte-identical: title's built-in body is "# Changelog".
EOF
)"
```

---

### Task 3: Add `title`/`subtitle` blocks for the release-notes driver

**Files:**
- Modify: `internal/generators/native/release_notes.tmpl`
- Modify: `internal/generators/native/render.go` (`renderReleaseNotes`)
- Modify: `internal/generators/native/generator_internal_test.go` (new tests)

**Interfaces:**
- Consumes: `renderPreamble(rootTmpl string, snippets map[string]string, templateFile string, heraut tplHeraut) (string, error)` (Task 2, `internal/generators/native/render.go`) — reused unchanged.

- [ ] **Step 1: Write the failing tests**

In `internal/generators/native/generator_internal_test.go`, add (near
`TestGenerator_GenerateReleaseNotes`):

```go
func TestGenerator_GenerateReleaseNotes_TitleSubtitle(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil)
	mr.QueueResponse(record("abc1234567", "Alice", "alice@example.com",
		"2026-01-02T00:00:00Z", "feat: add the thing", ""), "", nil)
	mr.QueueResponse("alice@example.com\n", "", nil)

	g := New(mr, &config.ContentDriver{
		EffectiveTemplates: map[string]string{
			"title":    "MyApp v1.1.0 Release",
			"subtitle": "Highlights below.",
		},
	}, ModeReleaseNotes)
	out, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(out, "MyApp v1.1.0 Release\n\nHighlights below.\n\n"))
}

func TestGenerator_GenerateReleaseNotes_TitleSubtitleEmptyByDefault(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil)
	mr.QueueResponse(record("abc1234567", "Alice", "alice@example.com",
		"2026-01-02T00:00:00Z", "feat: add the thing", ""), "", nil)
	mr.QueueResponse("alice@example.com\n", "", nil)

	g := New(mr, &config.ContentDriver{}, ModeReleaseNotes)
	out, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(out, "### 🚀 Features"),
		"release notes has no document-level title by default — output starts with the first group heading, byte-identical to before this change")
}

func TestGenerator_GenerateReleaseNotes_TitleContextIsHerautOnly(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)
	mr.QueueResponse("2026-01-01T00:00:00Z\n", "", nil)
	mr.QueueResponse(record("abc1234567", "Alice", "alice@example.com",
		"2026-01-02T00:00:00Z", "feat: add the thing", ""), "", nil)
	mr.QueueResponse("alice@example.com\n", "", nil)

	g := New(mr, &config.ContentDriver{
		EffectiveTemplates: map[string]string{"title": "{{ .Version }}"}, // .Version exists on Release, not tplHeraut
	}, ModeReleaseNotes)
	_, err := g.Generate("v1.1.0", nil)
	require.Error(t, err, "title only receives .Heraut's own fields — .Version must fail to execute")
	assert.Contains(t, err.Error(), "title")
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/generators/native/... -run TestGenerator_GenerateReleaseNotes -v`
Expected: `TestGenerator_GenerateReleaseNotes_TitleSubtitle` and
`TestGenerator_GenerateReleaseNotes_TitleContextIsHerautOnly` FAIL (title/subtitle not yet wired
into the release-notes path); `TitleSubtitleEmptyByDefault` currently PASSES already (nothing to
regress) — that's fine, it locks in the byte-identical-by-default invariant going forward.

- [ ] **Step 3: Add `title`/`subtitle` defines to `release_notes.tmpl`**

Prepend two new defines above the existing `release_header` define (added in Task 1):

```gotemplate
{{define "title"}}{{end}}
{{define "subtitle"}}{{end}}
{{define "release_header"}}{{end}}
{{define "release_notes"}}{{ template "release_header" . }}{{range .Groups}}{{ template "group" . }}
{{range .Commits}}
{{ template "commit" . }}{{ if .Body }}

{{ indent 4 .Body }}{{ end }}{{ if .Footers }}

{{ range $i, $f := .Footers }}{{ if $i }}
{{ end }}  {{ $f.Token }}: {{ $f.Value }}{{ end }}{{ end }}
{{end}}
{{end}}
{{if .Contributors}}{{ template "contributors" . }}{{end}}{{ template "stats" . }}{{ template "footer" . }}{{end}}
```

(`title`/`subtitle` are defined here — so the template set includes them, making them
overridable via `rendering.templates`/`<driver>.template` — but never *called* from within the
`release_notes` root; they're rendered separately by `renderPreamble` in Go, then prepended.)

- [ ] **Step 4: Wire `renderPreamble` into `renderReleaseNotes`**

In `internal/generators/native/render.go`, replace the body of `renderReleaseNotes` (around lines
82-102):

```go
func renderReleaseNotes(
	version, previousVersion string,
	releaseDate time.Time,
	groups []group,
	lc *port.LinkContext,
	tickets []config.Ticket,
	prevReleaseDate time.Time,
	typesHeadingLevel int,
	prs map[string]PullRequest,
	contributors []Contributor,
	heraut tplHeraut,
	snippets map[string]string,
	templateFile string,
) (string, error) {
	rel := buildRelease(version, previousVersion, releaseDate, prevReleaseDate, groups, lc, tickets, typesHeadingLevel, prs, contributors, heraut)
	preamble, err := renderPreamble(releaseNotesTmpl, snippets, templateFile, heraut)
	if err != nil {
		return "", fmt.Errorf("rendering release notes title: %w", err)
	}
	out, err := execBlocks("release_notes", releaseNotesTmpl, snippets, templateFile, rel)
	if err != nil {
		return "", fmt.Errorf("rendering release notes: %w", err)
	}
	return strings.TrimSpace(preamble + out), nil
}
```

(Only the two new lines — the `renderPreamble` call and its error check — and the final `return`
statement change; reproduced in full for an unambiguous diff.)

- [ ] **Step 5: Run the release-notes tests again**

Run: `go test ./internal/generators/native/... -run TestGenerator_GenerateReleaseNotes -v`
Expected: all PASS, including the two new tests and the pre-existing golden-output tests
(`TestGenerator_GenerateReleaseNotes`, `TestGenerator_GenerateReleaseNotes_DaysBetweenReleases`,
etc. — unaffected since `renderPreamble` returns `""` when title/subtitle are both unset).

- [ ] **Step 6: Run the full suite and lint**

Run: `go build ./... && go test ./... -count=1 && hk check`
Expected: all green.

- [ ] **Step 7: Commit**

```bash
git add internal/generators/native/release_notes.tmpl internal/generators/native/render.go \
  internal/generators/native/generator_internal_test.go
git commit -m "$(cat <<'EOF'
feat(generators/native): extend title/subtitle blocks to release notes

Symmetric with the changelog driver (previous commit): title/subtitle fire
once per release-notes render, executed against the same .Heraut-only
context via the shared renderPreamble helper. Default output is
byte-identical — both blocks default to empty on this driver, matching
today's empty header default.
EOF
)"
```

---

### Task 4: Update documentation

**Files:**
- Create: `docs/adr/0048-changelog-title-subtitle-blocks.md`
- Modify: `docs/adr/README.md`
- Modify: `docs/adr/0037-native-template-api.md`
- Modify: `docs/specs/05-generators-and-platforms.md`
- Modify: `docs/heraut.sample.yml`
- Modify: `docs/guides/template-customization.md`

No tests — documentation only. Each step is a self-contained edit.

- [ ] **Step 1: Write ADR-0048**

Create `docs/adr/0048-changelog-title-subtitle-blocks.md`:

```markdown
# ADR-0048: Changelog/release-notes title & subtitle template blocks

- **Status**: Accepted
- **Date**: 2026-08-28
- **Deciders**: bchatard
- **Supersedes**: the block-set table in [ADR-0037](0037-native-template-api.md) (`header` →
  `release_header`, `release-notes` → `release_notes`; adds `title`/`subtitle`)

---

## Context

[ADR-0037](0037-native-template-api.md) exposed the native generator's template blocks as public
config, but the changelog's document-level title was never one of them: `internal/generators/
native/render.go` hardcoded `const changelogHeader = "# Changelog\n\n"`, prepended once per file
by `buildAllSections` on bootstrap or `--regenerate` only — unreachable from
`rendering.templates.header` (which is the **per-release** version heading, e.g.
`## [1.2.3] - 2026-08-28`, rendered once per section) or from a `<driver>.template` file. The
`header` block's name read as "the document header," which hid this gap until a user asked
directly whether the changelog's `# Changelog` title could be customized.

Full design: [`docs/superpowers/specs/2026-08-28-changelog-title-subtitle-design.md`](../superpowers/specs/2026-08-28-changelog-title-subtitle-design.md).

## Decision

1. **Renamed** the per-release heading block from `header` to `release_header` — the name now
   says what varies (fires again for every release/section).
2. **Added** `title`/`subtitle` — new blocks that fire exactly **once per document**, on both the
   changelog and release-notes drivers. They execute against a bare `tplHeraut` context
   (`.Version` `.URL` `.GeneratedAt` directly — not `.Heraut.Version`), not the full `Release`,
   since they are not tied to any single release.
3. **Renamed** `release-notes` (root block) to `release_notes`, so every block key uses a
   consistent `snake_case` separator.

### Block set (supersedes ADR-0037's table)

| Key | Was | Context (`.`) | Fires |
|---|---|---|---|
| `title` | *(new)* | bare `tplHeraut` | once per document |
| `subtitle` | *(new)* | bare `tplHeraut` | once per document |
| `release_header` | `header` | `Release` | once per rendered release |
| `group`, `commit`, `ticket`, `contributor`, `contributors`, `stats`, `footer` | unchanged | unchanged | unchanged |
| `changelog` | unchanged | `Release` | once per rendered release (the per-section root) |
| `release_notes` | `release-notes` | `Release` | once |

### Defaults

`changelog`'s `title` built-in is `"# Changelog"` — byte-identical to the former
`changelogHeader` constant when unset. `changelog`'s `subtitle`, and both `release_notes` blocks,
default to empty — contributing zero bytes, no stray blank line, same pattern `footer` already
used.

### The rendering-scope wrinkle

Every other block executes once per rendered release. `title`/`subtitle` cannot follow that path
for the **changelog**: `buildAllSections` calls the per-section render once per section (newest,
then every historical tag) — title/subtitle must render exactly once regardless of section count.
The new `renderPreamble` helper (`internal/generators/native/render.go`) executes `title` and
`subtitle` as independent named-block executions against `tplHeraut`, trims each, and joins them
with a blank line — contributing nothing when both are unset. `buildAllSections` calls it once and
prepends the result; the ordinary incremental splice path is untouched (it never re-renders the
preamble, exactly as before). Release notes has no such wrinkle — `renderReleaseNotes` calls the
same `renderPreamble` helper once per render, prepending its result to the per-release body.

## Consequences

- **Byte-identical default output.** Nobody's rendered `CHANGELOG.md`/release notes change unless
  they explicitly set `title`/`subtitle`, or already had `rendering.templates.header`/
  `release-notes` set — those now fail config validation with an actionable "unknown template
  block" error naming the new key.
- **Breaking rename, no alias.** Consistent with the project's pre-v1.0 stance (SemVer 0.x).
- **A documented context asymmetry.** `title`/`subtitle` reach heraut's own metadata via
  `.Version`/`.URL`/`.GeneratedAt` directly; every other block reaches the same data via
  `.Heraut.Version` etc. This is called out explicitly in the guide and schema descriptions to
  avoid the obvious footgun.

## Alternatives considered

- **A plain (non-templated) string config field** instead of a template block. Rejected: no
  `.Heraut`/funcs access, and inconsistent with every other piece of native's output being
  template-driven.
- **A backward-compatible alias** for the renamed keys. Rejected: no other block rename in this
  codebase has one; pre-v1.0 breaking changes are the established norm here.
```

- [ ] **Step 2: Update `docs/adr/README.md`**

Change the ADR-0037 row (currently `| Accepted |`) to:

```markdown
| [0037](0037-native-template-api.md) | Native Generator Public Template API | Accepted (block-set table superseded by 0048 — `header`→`release_header`, `release-notes`→`release_notes`, new `title`/`subtitle` blocks; see body note) |
```

Add a new row after the ADR-0047 row (the current last row):

```markdown
| [0048](0048-changelog-title-subtitle-blocks.md) | Changelog/release-notes title & subtitle template blocks | Accepted |
```

- [ ] **Step 3: Add a body note to ADR-0037**

In `docs/adr/0037-native-template-api.md`, immediately after the `### Block set` heading (before
its table), insert:

> **Superseded by [ADR-0048](0048-changelog-title-subtitle-blocks.md):** `header` → `release_header`,
> `release-notes` → `release_notes`, and new `title`/`subtitle` blocks. The table below reflects the
> block set as originally shipped; see ADR-0048 for the current set.

- [ ] **Step 4: Update `docs/specs/05-generators-and-platforms.md`**

Change the section heading (line 56) from `### User-customizable templates (ADR-0037)` to
`### User-customizable templates (ADR-0037, ADR-0048)`.

Replace the YAML example (lines 66-77):

```yaml
rendering:
  templates:
    commit: "- {{ upperFirst .Description }} ({{ .ShortHash }})"
    contributor: "* @{{ .Author.Username }} — first contribution 🎉"

changelog:
  template: .config/heraut/changelog.tmpl   # optional full template file
  rendering:
    templates:
      header: "# Changelog\n\nAll notable changes.\n"
```

with:

```yaml
rendering:
  templates:
    commit: "- {{ upperFirst .Description }} ({{ .ShortHash }})"
    contributor: "* @{{ .Author.Username }} — first contribution 🎉"

changelog:
  template: .config/heraut/changelog.tmpl   # optional full template file
  rendering:
    templates:
      title: "# Changelog"
      subtitle: "All notable changes."
```

Replace the "Overridable blocks" paragraph (lines 79-86):

```markdown
**Overridable blocks:** `header`, `group`, `commit`, `ticket`, `contributor`, `contributors`,
`stats`, `footer`, and the roots `changelog` / `release-notes`. The changelog renders a one-line
commit; the release-notes root wraps the shared `commit` block with indented body/footers.
`ticket` renders one matched ticket link (`commits.tickets`) within a commit line — `commit`
calls it once per match, so overriding just `ticket` customizes ticket rendering without
restating the whole commit-line template. Any other key under `rendering.templates` is a
**config error** (a misspelled block would otherwise be silently ignored); `schema.json`
enumerates the same set for editor autocompletion.
```

with:

```markdown
**Overridable blocks:** `title`, `subtitle`, `release_header`, `group`, `commit`, `ticket`,
`contributor`, `contributors`, `stats`, `footer`, and the roots `changelog` / `release_notes`.
`title`/`subtitle` are document-level — they fire exactly **once per document**, unlike every
other block (which fires once per rendered release); they execute against `.Heraut`'s own fields
directly (`.Version` `.URL` `.GeneratedAt`), **not** `.Heraut.Version` like `footer`/
`release_header` do, since their root isn't a `Release`. The changelog renders a one-line
commit; the release-notes root wraps the shared `commit` block with indented body/footers.
`ticket` renders one matched ticket link (`commits.tickets`) within a commit line — `commit`
calls it once per match, so overriding just `ticket` customizes ticket rendering without
restating the whole commit-line template. Any other key under `rendering.templates` is a
**config error** (a misspelled block would otherwise be silently ignored); `schema.json`
enumerates the same set for editor autocompletion.
```

- [ ] **Step 5: Update `docs/heraut.sample.yml`**

In the `rendering:` commented block (around lines 196-209), update the `templates:` example to
use the new key names and add `title`/`subtitle`:

```yaml
#   # templates — override built-in native template blocks (ADR-0037, ADR-0048). native only.
#   # Each value is a Go text/template snippet; unset blocks keep their built-in.
#   # Overridable keys: title, subtitle (document-level, fire once — see below), release_header,
#   # group, commit, ticket, contributor, footer (+ the structural contributors, stats,
#   # changelog, release_notes for whole-section control). ticket renders one commits.tickets
#   # match within a commit line — override it alone to restyle ticket links without restating
#   # "commit". title/subtitle execute against .Heraut's own fields directly (.Version .URL
#   # .GeneratedAt) — not the full release data, and not .Heraut.Version like every other block.
#   # Data & funcs: see the native template contract in the docs. Precedence:
#   # built-in -> rendering.templates (global) -> <driver>.rendering.templates ->
#   # environments.<env>.<driver>.rendering.templates -> <driver>.template file.
#   templates:
#     title: "# MyApp Changelog"
#     subtitle: "All notable changes, by version."
#     release_header: '## [{{ .Version }}]'
#     commit: '- {{ .Description }} ({{ .ShortHash }})'
#     ticket: '🎫[{{ .Text }}]({{ .Href }})'
#     contributor: '* @{{ .Author.Username }} — first contribution 🎉'
#     footer: "\n_Generated by [heraut]({{ .Heraut.URL }}) {{ .Heraut.Version }}._"
```

Also update the `changelog.rendering` example (around lines 296-301), which currently uses
`header:` for a changelog-only override — change to `title:` since that's now the correct block
for "reformat the changelog's own document title without touching release notes":

```yaml
  # rendering — per-driver overrides (template snippets + excludes), deep-merged over the
  # global rendering block. native only. Use this to reformat one block for the changelog
  # without touching release notes (or vice-versa). See the global rendering block above.
  # rendering:
  #   templates:
  #     title: "# Changelog\n\nAll notable changes to this project.\n"
```

- [ ] **Step 6: Update `docs/guides/template-customization.md`**

This guide (published earlier this session) needs several updates for consistency with the new
design. Replace the "Overridable blocks" table:

```markdown
| Key | Context (data passed in) | Renders | Built-in default |
|---|---|---|---|
| `header` | root `Release` | Version heading line | Changelog: `## [version](compare-url) - date`. Release-notes default: empty. |
| `group` | `Group` | One commit-type group's heading | `{{ .HeadingPrefix }} {{ .Name }}` (e.g. `### 🚀 Features`) |
| `commit` | `Commit` | One commit's line — description, scope, breaking marker, hash link, `by @author`, PR/MR ref, ticket links | See [built-in `commit`](#worked-examples) below |
| `ticket` | `Link` (one `.Tickets` entry) | One matched ticket reference *within* a commit line | `([TICKET](url))` |
| `contributor` | `Contributor` | One "New Contributors" entry | `* @user made their first contribution in [#42](url)` |
| `contributors` | root `Release` | The whole "New Contributors ❤️" section (release notes only) | Heading + a loop over `contributor` |
| `stats` | root `Release` | The Commit Statistics block (release notes only) | Commit/conventional counts, timespan, linked tickets |
| `footer` | root `Release` | Trailing content after the body | Empty |
| `changelog` | root `Release` | The whole changelog-section document (whole-document control) | Composes `header` → `group` → `commit` |
| `release-notes` | root `Release` | The whole release-notes document (whole-document control) | Composes `header` → `group` → `commit` (+body/footers) → `contributors` → `stats` → `footer` |
```

with:

```markdown
| Key | Context (data passed in) | Renders | Built-in default |
|---|---|---|---|
| `title` | bare `.Heraut` (`.Version` `.URL` `.GeneratedAt` — **not** `.Heraut.Version`) | Document-level title, once per document | Changelog: `# Changelog`. Release-notes default: empty. |
| `subtitle` | bare `.Heraut` (same as `title`) | Document-level subtitle rendered under `title`, once per document | Empty on both drivers |
| `release_header` | root `Release` | One release's version heading, once per rendered release | Changelog: `## [version](compare-url) - date`. Release-notes default: empty. |
| `group` | `Group` | One commit-type group's heading | `{{ .HeadingPrefix }} {{ .Name }}` (e.g. `### 🚀 Features`) |
| `commit` | `Commit` | One commit's line — description, scope, breaking marker, hash link, `by @author`, PR/MR ref, ticket links | See [built-in `commit`](#worked-examples) below |
| `ticket` | `Link` (one `.Tickets` entry) | One matched ticket reference *within* a commit line | `([TICKET](url))` |
| `contributor` | `Contributor` | One "New Contributors" entry | `* @user made their first contribution in [#42](url)` |
| `contributors` | root `Release` | The whole "New Contributors ❤️" section (release notes only) | Heading + a loop over `contributor` |
| `stats` | root `Release` | The Commit Statistics block (release notes only) | Commit/conventional counts, timespan, linked tickets |
| `footer` | root `Release` | Trailing content after the body | Empty |
| `changelog` | root `Release` | The whole changelog-section document (whole-document control) | Composes `release_header` → `group` → `commit` |
| `release_notes` | root `Release` | The whole release-notes document (whole-document control) | Composes `release_header` → `group` → `commit` (+body/footers) → `contributors` → `stats` → `footer` |
```

Note the table's own footnote-style callout right after it should gain a line: `title`/`subtitle`
fire once per document, not once per rendered release — the one exception to every other block's
scope — and reach heraut's own metadata via bare `.Version`/`.URL`/`.GeneratedAt`, unlike every
other block which reaches the same data via `.Heraut.Version` since its root is a full `Release`.

- **"Two commit renderings, one block" / "contributors/stats are release-notes-only" callouts**:
  unaffected, leave as-is.
- **"Where to set overrides" section**: no precedence-chain changes — `title`/`subtitle` follow
  the exact same four-layer chain as every other block. Add one line noting they're the only
  blocks that fire once per document rather than once per release.
- **"Full template file mode" example**: rename `{{define "header"}}` to
  `{{define "release_header"}}` if the example uses it.
- **"Data contract" section**: add a short callout that `title`/`subtitle` are the one exception
  to the documented `Release`-rooted contract — their root is `tplHeraut` directly.
- **"Worked examples" section**: update the "Plainer changelog header" example to use `title`
  instead of `header`, and add a new worked example showing a `title`+`subtitle` pair.
- **"Gotchas" section**: replace the now-resolved gap ("no document-level title override") with a
  note documenting the `.Heraut` vs `.Heraut.Version` addressing asymmetry, since that's now the
  guide's own footgun to warn about.

Apply these as direct edits to the existing file content (already read into context from earlier
in this session) rather than a rewrite — the file's structure and worked-example format stay the
same.

- [ ] **Step 7: Run `hk check`**

Run: `hk check`
Expected: clean (typos, markdown formatting).

- [ ] **Step 8: Commit**

```bash
git add docs/adr/0048-changelog-title-subtitle-blocks.md docs/adr/README.md \
  docs/adr/0037-native-template-api.md docs/specs/05-generators-and-platforms.md \
  docs/heraut.sample.yml docs/guides/template-customization.md
git commit -m "$(cat <<'EOF'
docs: document title/subtitle blocks and the header/release-notes rename

New ADR-0048 records the decision (supersedes ADR-0037's block-set table).
Updates Spec 05, the annotated sample config, and the template-customization
guide for the renamed release_header/release_notes keys and the new
document-level title/subtitle blocks.
EOF
)"
```

---

### Task 5: Close out the roadmap entry

**Files:**
- Modify: `docs/tasks/roadmap.md`

No tests — this is the project's required roadmap completion step
(`.claude/rules/workflow.md` § Two-step roadmap flow).

- [ ] **Step 1: Run the full verification suite one more time**

Run: `go build ./... && go test ./... -count=1 && hk check`
Expected: all green — this is the final gate before marking the phase complete.

- [ ] **Step 2: Flip the phase checkbox and add the completion note**

In `docs/tasks/roadmap.md`, change the Phase 30 heading added in Task 1 from a plain heading to
include the completion marker, and add a one-paragraph note underneath describing what was
actually built, matching the style of the other completed phases in this file:

```markdown
### Phase 30 — Changelog/release-notes title & subtitle blocks ✅

Renames the native generator's `header` template block to `release_header` (it fires once per
rendered release, not once per document — the old name was confusing enough to hide a real gap)
and `release-notes` to `release_notes` (separator consistency across every block key). Adds new
`title`/`subtitle` blocks that fire exactly once per document, closing the gap where the
changelog's `# Changelog` line was a hardcoded Go constant unreachable from any template override.
New ADR-0048 (supersedes the block-set table in ADR-0037). Five tasks (rename → changelog
title/subtitle → release-notes title/subtitle → docs → roadmap close-out).

Design: [`docs/superpowers/specs/2026-08-28-changelog-title-subtitle-design.md`](../superpowers/specs/2026-08-28-changelog-title-subtitle-design.md).

Shipped as designed, no deviations. `title`/`subtitle` execute against a bare `tplHeraut`
(`.Version`/`.URL`/`.GeneratedAt` directly, not `.Heraut.Version` — a deliberate, documented
asymmetry with every other block) via a new shared `renderPreamble` helper
(`internal/generators/native/render.go`), called once by `buildAllSections` for the changelog
driver (bootstrap/`--regenerate` only — an ordinary incremental splice never touches the preamble,
unchanged from before) and once per render by `renderReleaseNotes` for the release-notes driver.
Default output is byte-identical on both drivers. The rename (`header`→`release_header`,
`release-notes`→`release_notes`) is breaking with no alias, consistent with the project's pre-v1.0
stance; both old keys now fail config validation with an actionable error naming the new one.

---
```

- [ ] **Step 3: Commit**

```bash
git add docs/tasks/roadmap.md
git commit -m "docs(roadmap): close out Phase 30 — title/subtitle template blocks"
```
