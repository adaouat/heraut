# `heraut notes` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** a read-only `heraut notes <tag>` command that prints the changelog-style groups and commit lines for a tag's range, in Markdown or plain text, through a configurable `notes:` template.

**Architecture:** a third native generator mode (`ModeNotes`) renders the changelog root with notes-specific default blocks layered in as the lowest inline snippets, so global and `notes` overrides win. The previous-tag bound is resolved from the tag's own ancestry. `app.GenerateNotes` builds it and converts to text with a goldmark AST walk when asked. `internal/cmd/notes.go` is a thin cobra wrapper.

**Tech Stack:** Go, cobra, `text/template` (existing native engine), `github.com/yuin/goldmark` (already in `go.sum` as an indirect requirement), testify, real-git test repos, the e2e Lane A harness.

**Spec:** `docs/superpowers/specs/2026-10-13-notes-command-design.md`

## Global Constraints

- TDD: every production line has a failing test first (project rule). Run Go under `mise exec --`.
- Conventional commits, subject ≤ 72 chars, trailer `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>` only (never `Claude-Session`); never `--no-verify`. Before each commit run `mise exec -- hk fix -S go_fmt` then `mise exec -- hk check`.
- Layering: `cmd` → `app` → `generators/native`; `generators/*` import only `port`/`config`/`conventionalcommit`. `cmd` never constructs a generator.
- Errors wrap with `%w`; sentinels matched with `errors.Is`; no `os.Exit` below `cmd/`.
- A config field change also updates `schema.json` and `docs/heraut.sample.yml`.
- No real project names, paths or hosts in fixtures or docs.
- Comments only for a non-obvious *why*; no task or history references in comments.
- Do not push; the user pushes.

## Rulings carried into the plan

- **Config type.** The spec said `Config.Notes` reuses `ContentDriver` and validation rejects `output` and `tag_pattern`. A dedicated `Notes{Template, Rendering}` struct gets the same result for free (strict parsing rejects any other key with a line number) and needs no validation code. Task 6 amends the spec.
- **Range bound.** The spec said "the bound the release itself uses". The release walks tags reachable from `HEAD`; a notes run for a tag cut on another line would then bound wrongly. Notes lists tags reachable from the *requested tag* (`git tag --merged <tag>`). Task 6 amends the spec.
- **Default blocks as snippets.** `ModeNotes` seeds `release.section` (empty) and `commit.message` (lean) as inline snippets, then overlays the effective `rendering.templates`. That gives the spec's precedence with no change to `buildTemplateSet`.

## Review Focus

1. **Tag cut on another line.** `heraut notes v1.3.2` from `main` must bound at `v1.3.1`, not at a higher tag from `main`. Pinned in Task 2.
2. **Empty range.** A range whose commits are all excluded prints nothing and exits 0 with a stderr notice. Pinned in Tasks 2 and 5.
3. **Markdown characters in a description, text mode.** `snake_case`, a backticked flag, and `*stars*` convert as CommonMark reads them. Pinned in Task 3.
4. **`--from` newer than the tag, or not a tag.** A runtime error naming the ref, nothing written. Pinned in Task 4.
5. **`--output` to a missing directory.** A runtime error, no partial file. Pinned in Task 5.

---

### Task 1: `notes:` config block, schema, sample, roadmap entry

**Files:**
- Modify: `internal/config/config.go` (add `Notes` type and `Config.Notes` field next to `Changelog`, line ~7)
- Modify: `internal/config/validator.go` (validate the block's templates, near line 615)
- Modify: `schema.json` (add `notes` property and `Notes` definition)
- Modify: `docs/heraut.sample.yml`
- Create: `testdata/config/valid/notes.yml`
- Modify: `docs/tasks/roadmap.md` (Phase 66 row and T371 entry, `[ ]`)
- Test: `internal/config/loader_test.go` (or the existing loader test file), `internal/config/validator_test.go`, `internal/config/schema_test.go` runs the valid fixtures already

**Interfaces:**
- Produces: `config.Notes{Template string; Rendering *Rendering}` and `Config.Notes *Notes` (yaml `notes,omitempty`).

- [ ] **Step 1: Write the failing tests**

In the loader test file add a test that loads this YAML through `config.Load` (write it to `t.TempDir()`):

```yaml
version: "1"
versioning:
  strategy: semver
notes:
  template: .config/heraut/notes.tmpl
  rendering:
    templates:
      release:
        section: "## {{ .Version }}"
```

Assert `cfg.Notes != nil`, `cfg.Notes.Template == ".config/heraut/notes.tmpl"`, and `cfg.Notes.Rendering.Templates["release.section"]` is set (check how the loader flattens `release:` blocks and assert the key it produces; follow the changelog equivalent test).

Add a second test: the same file with `notes:\n  output: NOTES.md` fails `config.Load` with an error containing `field output not found`.

In the validator test file add: a `notes.rendering.templates.commit.message` with an unparsable body (`"{{ .Description"`) yields a `ValidationError` whose `Path` starts with `notes.rendering.templates`; a `notes.template` pointing at a missing file yields `Path == "notes.template"`.

Create `testdata/config/valid/notes.yml` with the first YAML above minus the template path line (a missing file would fail validation) so the schema fixture test covers it.

- [ ] **Step 2: Run to verify they fail**

Run: `mise exec -- go test ./internal/config/ -run 'Notes|Schema' -count=1`
Expected: FAIL (`cfg.Notes` undefined / schema rejects `notes`).

- [ ] **Step 3: Implement**

`config.go`:

```go
// Notes configures `heraut notes`: the template layered over the built-in notes blocks. It has no
// output or tag_pattern, unlike ContentDriver, because the command writes no managed file and takes its range from the tag.
type Notes struct {
	Template  string     `yaml:"template,omitempty"`
	Rendering *Rendering `yaml:"rendering,omitempty"`
}
```

Add `Notes *Notes \`yaml:"notes,omitempty"\`` to `Config` after `Changelog`.

`validator.go`: after the changelog validation add

```go
if cfg.Notes != nil {
	errs = append(errs, validateContentDriverTemplates(&ContentDriver{Template: cfg.Notes.Template, Rendering: cfg.Notes.Rendering}, "notes")...)
}
```

`schema.json`: add `"notes": { "$ref": "#/definitions/Notes" }` beside `changelog`, and a `Notes` definition (`additionalProperties: false`) with the `template` and `rendering` properties copied from `ContentDriver` (same descriptions, `rendering` as `$ref` to the same `Rendering` definition `ContentDriver` uses; open `schema.json` around line 595 and copy exactly).

`docs/heraut.sample.yml`: add a commented `notes:` block showing `template:` and a one-line `rendering.templates` example, with the comment "used by `heraut notes`".

- [ ] **Step 4: Run to verify they pass**

Run: `mise exec -- go test ./internal/config/ -count=1`
Expected: PASS.

- [ ] **Step 5: Roadmap entry and commit**

In `docs/tasks/roadmap.md` add to the phase table `| 66 | heraut notes: release notes for a tag, for any consumer | Open — T371 |` and, before the "Every Phase above marked `Done`" paragraph, a `### Phase 66 — heraut notes` section with `#### \`[ ]\` T371: heraut notes command`, a short description pointing at the spec and this plan.

```bash
mise exec -- hk fix -S go_fmt && git add -A && mise exec -- git commit -m "feat(config): add the notes block

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 2: native `ModeNotes`

**Files:**
- Modify: `internal/generators/native/generator.go` (Mode, `fromTag` field, `WithFromTag`, `Generate` dispatch, `generateNotes`, `notesPreviousTag`)
- Modify: `internal/generators/native/render.go` (`notesDefaultBlocks`, `renderNotesSection`)
- Test: `internal/generators/native/notes_realrepo_test.go` (package `native_test`, reuse `gitFixture` from `integration_test.go`), `internal/generators/native/notes_internal_test.go`

**Interfaces:**
- Consumes: `buildRelease`, `execBlocks`, `changelogTmpl`, `collectCommits`, `commitRange`, `groupCommits`, `overlayAuthorHandles`, `releaseDate`, `listTags`, `filterByTagPattern`, `previousInList`, `previousTag`, `enrichForRelease` (all existing).
- Produces: `native.ModeNotes`; `native.WithFromTag(tag string) Option`; `(*Generator).Generate(tag, lc)` returns the notes text in `ModeNotes` (empty string when no group survives).

- [ ] **Step 1: Write the failing tests**

`notes_realrepo_test.go`:

```go
func TestNotes_RealRepo_DefaultLayout(t *testing.T) {
	gitFixture(t)
	require.NoError(t, exec.Command("git", "tag", "v0.2.0").Run())
	g := native.New(execadapter.New(false, false), &config.ContentDriver{}, native.ModeNotes)

	out, err := g.Generate("v0.1.0", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "### Features")
	assert.Contains(t, out, "- *(auth)* Add login")
	assert.Contains(t, out, "- *(auth)* Add logout")
	assert.Contains(t, out, "- Correct null check")
	assert.NotContains(t, out, "## [", "no release header")
	assert.NotContains(t, out, "by @", "no author")
	assert.NotContains(t, out, "](", "no hash or PR link")
	assert.NotContains(t, out, "bump some library", "rendering.excludes still apply")
}

func TestNotes_RealRepo_RangeIsThePreviousTagToTheTag(t *testing.T) {
	gitFixture(t)
	require.NoError(t, exec.Command("git", "tag", "v0.2.0").Run())
	g := native.New(execadapter.New(false, false), &config.ContentDriver{}, native.ModeNotes)

	out, err := g.Generate("v0.2.0", nil)
	require.NoError(t, err)

	assert.Contains(t, out, "Redesign the page")
	assert.NotContains(t, out, "Add login", "commits before v0.1.0 belong to v0.1.0")
}

func TestNotes_RealRepo_FromOverridesTheLowerBound(t *testing.T) {
	gitFixture(t)
	require.NoError(t, exec.Command("git", "tag", "v0.2.0").Run())
	g := native.New(execadapter.New(false, false), &config.ContentDriver{}, native.ModeNotes, native.WithFromTag("v0.1.0"))

	out, err := g.Generate("v0.2.0", nil)
	require.NoError(t, err)
	assert.Contains(t, out, "Redesign the page")

	g = native.New(execadapter.New(false, false), &config.ContentDriver{}, native.ModeNotes)
	out, err = g.Generate("v0.2.0", nil)
	require.NoError(t, err)
	assert.NotContains(t, out, "Add login", "the default bound is v0.1.0")
}

func TestNotes_RealRepo_NoSurvivingCommitIsEmpty(t *testing.T) {
	gitFixture(t)
	drv := &config.ContentDriver{Excludes: []config.Exclude{{Type: "feat"}, {Type: "revert"}}}
	require.NoError(t, exec.Command("git", "tag", "v0.2.0").Run())
	g := native.New(execadapter.New(false, false), drv, native.ModeNotes)

	out, err := g.Generate("v0.2.0", nil)
	require.NoError(t, err)
	assert.Empty(t, out)
}
```

Check the `config.Exclude` field names in `internal/config/config.go` and adjust the literal to match; the intent is "every commit between v0.1.0 and v0.2.0 is excluded".

`notes_internal_test.go` (package `native`): a `MockRunner` test asserting that, for `Generate("v1.2.0", nil)` with a `TagGlob` empty and no `tagOrder`, the generator calls `git tag -l --merged v1.2.0 …` (use `listTags`' exact args from `commits.go:154`) and never `--merged HEAD`. Queue the responses in call order: tag list, then the range log. Read `listTags`/`previousTag`/`collectCommits` to get the exact calls and queue matching responses.

Add the other-line test to the same file using a real repo via `historyRepo`-style setup is not available in this package; instead write it in Task 4 (`app` has `historyRepo`).

- [ ] **Step 2: Run to verify they fail**

Run: `mise exec -- go test ./internal/generators/native/ -run Notes -count=1`
Expected: FAIL (`native.ModeNotes` undefined).

- [ ] **Step 3: Implement**

`generator.go`: add `ModeNotes` after `ModeReleaseNotes`; field `fromTag string`; option

```go
// WithFromTag overrides the lower bound of a notes range (ModeNotes). An empty tag keeps the
// default bound.
func WithFromTag(tag string) Option { return func(g *Generator) { g.fromTag = tag } }
```

`Generate`: `if g.mode == ModeNotes { return g.generateNotes(tag, lc) }` first.

```go
// notesPreviousTag is the lower bound for tag: the override, else the previous release in tag's own
// history. It lists tags reachable from tag, not from HEAD, so a tag cut on another line is bounded by that line.
func (g *Generator) notesPreviousTag(tag string) (string, error) {
	if g.fromTag != "" {
		return g.fromTag, nil
	}
	tags, err := listTags(g.runner, g.cfg.TagGlob, tag)
	if err != nil {
		return "", err
	}
	if g.cfg.TagGlob == "" {
		if tags, err = filterByTagPattern(tags, g.cfg.TagPattern); err != nil {
			return "", err
		}
	}
	if g.tagOrder != nil {
		return previousInList(tag, g.tagOrder(tags)), nil
	}
	return previousInList(tag, tags), nil
}

func (g *Generator) generateNotes(tag string, lc *port.LinkContext) (string, error) {
	prev, err := g.notesPreviousTag(tag)
	if err != nil {
		return "", err
	}
	commits, err := collectCommits(g.runner, commitRange(prev, tag))
	if err != nil {
		return "", err
	}
	er, err := g.enrichForRelease(commits, tag)
	if err != nil {
		return "", err
	}
	groups := groupCommits(commits, g.cfg.Types, g.cfg.Excludes)
	overlayAuthorHandles(groups, er.authors)
	return renderNotesSection(tag, prev, releaseDate(commits), groups, lc, g.cfg.Tickets, g.cfg.TypesHeadingLevel, er.prs, g.herautMeta(), g.cfg.EffectiveTemplates, g.cfg.Template, g.cfg.EffectiveTrailerRules)
}
```

If `listTags` output order differs from what `previousInList` expects (newest-first), keep git's `--sort=-version:refname` as `listTags` already does; confirm by reading `listTags`.

`render.go`:

```go
// notesDefaultBlocks are the built-in blocks of `heraut notes`, seeded below every user snippet:
// no release heading and a commit line without hash, author or PR reference.
var notesDefaultBlocks = map[string]string{
	"release.section": "",
	"commit.message":  `- {{ if .Scope }}*({{ .Scope }})* {{ end }}{{ if .Breaking }}[**breaking**] {{ end }}{{ .Description }}{{ range .Tickets }} {{ template "commit.ticket" . }}{{ end }}`,
}

func renderNotesSection(version, previousVersion string, releaseDate time.Time, groups []group, lc *port.LinkContext, tickets []config.Ticket, typesHeadingLevel int, enrichment map[string]PullRequest, heraut tplHeraut, snippets map[string]string, templateFile string, trailerRules map[string]config.FooterRule) (string, error) {
	rel, err := buildRelease(version, previousVersion, releaseDate, time.Time{}, groups, lc, tickets, typesHeadingLevel, enrichment, nil, heraut, trailerRules)
	if err != nil {
		return "", fmt.Errorf("building notes model: %w", err)
	}
	merged := maps.Clone(notesDefaultBlocks)
	maps.Copy(merged, snippets)
	out, err := execBlocks("changelog", changelogTmpl, merged, templateFile, rel)
	if err != nil {
		return "", fmt.Errorf("rendering notes: %w", err)
	}
	return strings.TrimSpace(out), nil
}
```

Add `"maps"` to the imports. A user snippet for a block that `notesDefaultBlocks` also seeds replaces it (later parse wins), which is the precedence the spec wants.

- [ ] **Step 4: Run to verify they pass**

Run: `mise exec -- go test ./internal/generators/native/ -count=1`
Expected: PASS, including every pre-existing native test. If the default-layout test fails on whitespace only (blank lines between groups), compare with what `renderChangelogSection` produces for the same range minus its header and fix the assertion, not the template.

- [ ] **Step 5: Commit**

```bash
mise exec -- hk fix -S go_fmt && git add -A && mise exec -- git commit -m "feat(generators/native): add the notes mode

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Markdown to plain text

**Files:**
- Create: `internal/generators/native/plaintext.go`
- Modify: `go.mod` (`github.com/yuin/goldmark` moves from the indirect block to the direct block; `mise exec -- go mod tidy` does it)
- Test: `internal/generators/native/plaintext_test.go` (package `native_test`)

**Interfaces:**
- Produces: `native.PlainText(markdown string) string`.

- [ ] **Step 1: Write the failing test**

```go
func TestPlainText(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"heading is a plain line", "### Features\n\n- a", "Features\n\n- a"},
		{"scope emphasis", "- *(auth)* Add login", "- (auth) Add login"},
		{"breaking marker keeps the brackets", "- [**breaking**] Drop v1", "- [breaking] Drop v1"},
		{"link keeps its text only", "- Fix ([PROJ-12](https://example.com/PROJ-12))", "- Fix (PROJ-12)"},
		{"code span loses its backticks", "- Add `--force` flag", "- Add --force flag"},
		{"intraword underscore is untouched", "- Handle snake_case keys", "- Handle snake_case keys"},
		{"escaped punctuation is unescaped", `- Rename a\_b`, "- Rename a_b"},
		{"emphasis written as stars is formatting", "- Add *stars* support", "- Add stars support"},
		{"nested list keeps its depth", "- a\n  - b", "- a\n  - b"},
		{"groups are separated by one blank line", "### A\n\n- x\n\n### B\n\n- y", "A\n\n- x\n\nB\n\n- y"},
		{"bare url survives", "- See <https://example.com>", "- See https://example.com"},
		{"empty stays empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, native.PlainText(tc.in))
		})
	}
}

func TestPlainText_SampleRange(t *testing.T) {
	in := "### Features\n\n- *(auth)* Add biometric login\n- [**breaking**] Drop the v1 sync API ([PROJ-12](https://example.com/PROJ-12))\n\n### Bug Fixes\n\n- *(ui)* Fix crash when rotating the settings screen"
	want := "Features\n\n- (auth) Add biometric login\n- [breaking] Drop the v1 sync API (PROJ-12)\n\nBug Fixes\n\n- (ui) Fix crash when rotating the settings screen"
	assert.Equal(t, want, native.PlainText(in))
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `mise exec -- go test ./internal/generators/native/ -run PlainText -count=1`
Expected: FAIL (`native.PlainText` undefined).

- [ ] **Step 3: Implement**

```go
package native

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// PlainText converts Markdown to plain text by parsing it as CommonMark and writing the text a
// viewer would show: headings become plain lines, list items keep their "- " and depth, and
// emphasis, strong and code-span markers and link URLs are dropped. Parsing instead of
// stripping characters keeps text CommonMark does not treat as formatting, such as an intraword
// underscore, intact.
func PlainText(markdown string) string {
	src := []byte(markdown)
	doc := goldmark.New().Parser().Parse(text.NewReader(src))
	var sb strings.Builder
	writeBlocks(&sb, doc, src, 0)
	return strings.TrimSpace(sb.String())
}

func writeBlocks(sb *strings.Builder, parent ast.Node, src []byte, depth int) {
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Heading, *ast.Paragraph, *ast.TextBlock:
			sb.WriteString(strings.Repeat("  ", depth))
			writeInlines(sb, n, src)
			sb.WriteString("\n")
			if _, tight := n.(*ast.TextBlock); !tight {
				sb.WriteString("\n")
			}
		case *ast.List:
			writeList(sb, n, src, depth)
			if depth == 0 {
				sb.WriteString("\n")
			}
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			for i := 0; i < n.Lines().Len(); i++ {
				seg := n.Lines().At(i)
				sb.Write(seg.Value(src))
			}
			sb.WriteString("\n")
		default:
			writeBlocks(sb, n, src, depth)
		}
	}
}

func writeList(sb *strings.Builder, list *ast.List, src []byte, depth int) {
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		sb.WriteString(strings.Repeat("  ", depth) + "- ")
		first := true
		for c := item.FirstChild(); c != nil; c = c.NextSibling() {
			if sub, ok := c.(*ast.List); ok {
				if first {
					sb.WriteString("\n")
				}
				writeList(sb, sub, src, depth+1)
				first = false
				continue
			}
			writeInlines(sb, c, src)
			sb.WriteString("\n")
			first = false
		}
	}
}

func writeInlines(sb *strings.Builder, parent ast.Node, src []byte) {
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Text:
			sb.WriteString(string(util.UnescapePunctuations(n.Segment.Value(src))))
			if n.SoftLineBreak() || n.HardLineBreak() {
				sb.WriteString("\n")
			}
		case *ast.String:
			sb.Write(n.Value)
		case *ast.AutoLink:
			sb.Write(n.URL(src))
		case *ast.CodeSpan:
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					sb.Write(t.Segment.Value(src))
				}
			}
		case *ast.RawHTML:
		default:
			writeInlines(sb, n, src)
		}
	}
}
```

Run `mise exec -- go mod tidy` so goldmark is a direct requirement. Verify with `grep goldmark go.mod` that it is no longer marked `// indirect`.

- [ ] **Step 4: Run to verify it passes**

Run: `mise exec -- go test ./internal/generators/native/ -run PlainText -count=1`
Expected: PASS. If a nested-list or blank-line row fails, adjust the writer (not the expectation) until the table and the sample range both hold; the tight-list newline handling is the usual culprit.

- [ ] **Step 5: Commit**

```bash
mise exec -- hk fix -S go_fmt && git add -A && mise exec -- git commit -m "feat(generators/native): convert markdown to plain text

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 4: `app.GenerateNotes`

**Files:**
- Create: `internal/app/notes.go`
- Modify: `internal/pipeline/linkctx.go` (export `LinkContext(id *port.ForgeIdentity)`)
- Test: `internal/app/notes_realrepo_internal_test.go` (reuse `historyRepo` from `historybounds_realrepo_internal_test.go`)

**Interfaces:**
- Consumes: `native.New`/`ModeNotes`/`WithFromTag`/`PlainText` (Tasks 2–3), `withEnvDerivations`, `buildGenerator`, `resolveEnrichForgeIfNeeded`, `tagOrderFor`.
- Produces:

```go
type NotesOpts struct {
	Tag          string
	From         string
	Env          string
	TemplateFile string
	Format       string // "markdown" (default) or "text"
	Force        bool
}
type NotesResult struct {
	Text     string
	Warnings []string
}
var ErrNotesFormat = errors.New("invalid --format")
var ErrNotesTagNotFound = errors.New("tag not found")
var ErrNotesRange = errors.New("invalid notes range")
func GenerateNotes(runner port.Runner, cfg *config.Config, opts NotesOpts) (NotesResult, error)
```

- [ ] **Step 1: Write the failing tests**

Using `historyRepo(t)` (returns a `git` func; read the file for its signature and for a commit/tag helper pattern) and `execadapter.New(false, false)`, with a base config `&config.Config{Version: "1", Versioning: config.Versioning{Strategy: "semver"}}`. Scenarios:

1. **default text**: history `feat(auth): add login`, tag `v1.0.0`, `fix: stop crash`, tag `v1.1.0`. `GenerateNotes(..., NotesOpts{Tag: "v1.1.0"})` has `Text` containing `### Bug Fixes` and `- Stop crash` and not `Add login`.
2. **final skips its rc builds**: `v1.0.0`, commit, `v1.1.0-rc.1`, commit, `v1.1.0`. Notes for `v1.1.0` contain both commits (bound is `v1.0.0`).
3. **`--from` spans the rc builds**: notes for `v1.1.0-rc.1` with `From: "v1.0.0"` contain the first commit; notes for the same tag without `From` contain only the commits since `v1.0.0` (it is the first pre-release, so its bound is `v1.0.0` too: choose instead `v1.1.0-rc.2`, whose default bound is `v1.1.0-rc.1`, and `From: "v1.0.0"` widens it).
4. **other line is bounded by its own line (Review Focus 1)**: `v1.0.0`, `release/1.3` branch with `fix: on the line` and tag `v1.3.1`, then back on `main` a `feat: next` and tag `v1.4.0`, then on `release/1.3` another `fix: later` and tag `v1.3.2`. Run from `main` (checked out): notes for `v1.3.2` contain `Later` and not `Next` and not `On the line`. Config sets `versioning.branches` with `release/*` as in `versionBranchFixture`.
5. **format text**: `NotesOpts{Tag: ..., Format: "text"}` contains `Bug Fixes` without `###`.
6. **errors**: unknown tag wraps `ErrNotesTagNotFound` and names the tag; unknown `From` wraps it too; `Format: "html"` wraps `ErrNotesFormat`; a per-env strategy config with no `Env` returns an error containing `--env`; a `From` that is not in the history of `Tag` (for example a newer tag) wraps `ErrNotesRange`; the check is `git merge-base --is-ancestor <from> <tag>`.
7. **empty range** (Review Focus 2): a config whose `rendering.excludes` drops every type used between the tags yields `Text == ""` and one warning containing `no notable changes`.
8. **precedence**: with `cfg.Rendering.Templates["commit.message"] = "* {{ .Description }}"` globally and `cfg.Notes.Rendering.Templates["commit.message"] = "> {{ .Description }}"`, the text contains `> ` and not `* `; with only the global one, `* `.
9. **offline**: with `cfg.Commits = &config.Commits{EnrichmentPolicy: "disabled"}` no forge is resolved (test passes with no remote configured).

- [ ] **Step 2: Run to verify they fail**

Run: `mise exec -- go test ./internal/app/ -run Notes -count=1`
Expected: FAIL (`GenerateNotes` undefined).

- [ ] **Step 3: Implement**

`pipeline/linkctx.go`: add

```go
// LinkContext is the link context of the enrichment forge identity when it is complete, else the ambient CI host.
func LinkContext(id *port.ForgeIdentity) *port.LinkContext {
	if id != nil {
		if lc := linkContextFromIdentity(*id); lc != nil {
			return lc
		}
	}
	return ambientLinkContext()
}
```

and make `changelogLinkContext` return `LinkContext(p.cfg.ForgeIdentity)` (a pure refactor; the existing tests cover it).

`app/notes.go`:

```go
func GenerateNotes(runner port.Runner, cfg *config.Config, opts NotesOpts) (NotesResult, error) {
	format := opts.Format
	if format == "" {
		format = "markdown"
	}
	if format != "markdown" && format != "text" {
		return NotesResult{}, fmt.Errorf("%w %q: valid formats are markdown, text", ErrNotesFormat, opts.Format)
	}
	if (cfg.Versioning.Strategy == "semver-per-env" || cfg.Versioning.Strategy == "calver-per-env") && opts.Env == "" {
		return NotesResult{}, fmt.Errorf("--env is required for %s strategy", cfg.Versioning.Strategy)
	}
	if err := requireTag(runner, opts.Tag); err != nil {
		return NotesResult{}, err
	}
	if opts.From != "" {
		if err := requireTag(runner, opts.From); err != nil {
			return NotesResult{}, err
		}
		if _, _, err := runner.Run("git", "merge-base", "--is-ancestor", opts.From, opts.Tag); err != nil {
			return NotesResult{}, fmt.Errorf("%w: %s is not in the history of %s", ErrNotesRange, opts.From, opts.Tag)
		}
	}

	driver := &config.ContentDriver{}
	if cfg.Notes != nil {
		driver.Template, driver.Rendering = cfg.Notes.Template, cfg.Notes.Rendering
	}
	if opts.TemplateFile != "" {
		driver.Template = opts.TemplateFile
	}
	enrichForge, forgeID, degraded, err := resolveEnrichForgeIfNeeded(runner, os.Getenv, cfg, opts.Force)
	if err != nil {
		return NotesResult{}, err
	}
	driver = withEnvDerivations(driver, cfg, opts.Env)
	var extra []native.Option
	if opts.From != "" {
		extra = append(extra, native.WithFromTag(opts.From))
	}
	gen := buildGenerator(runner, driver, native.ModeNotes, "", false, opts.Force, enrichForge, degraded, tagOrderFor(cfg, opts.Env), extra...)
	text, err := gen.Generate(opts.Tag, pipeline.LinkContext(forgeID))
	if err != nil {
		return NotesResult{}, fmt.Errorf("generating notes: %w", err)
	}
	var res NotesResult
	if d, ok := gen.(interface{ Degraded() bool; DegradedReason() string }); ok && d.Degraded() {
		res.Warnings = append(res.Warnings, "release notes rendered without PR/MR metadata: "+d.DegradedReason())
	}
	if format == "text" {
		text = native.PlainText(text)
	}
	if text == "" {
		res.Warnings = append(res.Warnings, "no notable changes in "+rangeLabel(opts))
	}
	res.Text = text
	return res, nil
}

func requireTag(runner port.Runner, tag string) error {
	if _, _, err := runner.Run("git", "rev-parse", "--verify", "--quiet", "refs/tags/"+tag+"^{commit}"); err != nil {
		return fmt.Errorf("%w: %q", ErrNotesTagNotFound, tag)
	}
	return nil
}
```

Add `ErrNotesRange` and `rangeLabel` (`"<from>..<tag>"` when `From` is set, else the tag). Check that `buildGenerator`'s doc says `tagOrder` also adds `WithReachableFromHead`: that option is ignored by `ModeNotes`, which resolves its own bound; keep it.

- [ ] **Step 4: Run to verify they pass**

Run: `mise exec -- go test ./internal/app/ ./internal/pipeline/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
mise exec -- hk fix -S go_fmt && git add -A && mise exec -- git commit -m "feat(app): generate release notes for a tag

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 5: the `heraut notes` command and e2e scenarios

**Files:**
- Create: `internal/cmd/notes.go`
- Modify: `internal/cmd/root.go` (`root.AddCommand(NewNotesCmd())`)
- Test: `internal/cmd/notes_test.go`, `e2e/notes_test.go`

**Interfaces:**
- Consumes: `app.GenerateNotes`, `app.NotesOpts`, `app.ErrNotes*`, `applyOfflineOverride`, `config.Load`/`Validate`, `exitcode.Wrap`.
- Produces: `cmd.NewNotesCmd() *cobra.Command` with flags `--from`, `--format` (default `markdown`), `--template`, `--output`, `--env`, `--offline`.

- [ ] **Step 1: Write the failing tests**

`internal/cmd/notes_test.go`: follow `changelog_test.go` for how a command runs in-process against a real repo (read its first test and copy the setup). Cases: stdout carries the notes and stderr is empty; `--output <file>` writes the file and prints nothing; `--output` into a missing directory fails with the runtime exit code and leaves no file (Review Focus 5); `--format html` exits with `exitcode.Usage`; unknown tag exits with `exitcode.Runtime`; the exit-code assertions use `exitcode.Resolve(err)` as the other cmd tests do; `--template <file>` changes the output; an empty range prints nothing on stdout and the notice on stderr.

`e2e/notes_test.go` using `runScenarios` (see `maintenance_test.go`):

```go
func TestNotes(t *testing.T) {
	bin := harness.Binary(t)
	cfg := "version: \"1\"\nversioning:\n  strategy: semver\n"
	history := []step{commit("feat(auth): add login"), tag("v1.0.0"), commit("fix: stop crash"), tag("v1.1.0")}
	runScenarios(t, bin, nil, []scenario{
		{name: "markdown by default", config: cfg, history: history, args: []string{"notes", "v1.1.0", "--offline"},
			wantOut: "### Bug Fixes\n\n- Stop crash"},
		{name: "text format has no markdown characters", config: cfg, history: history, args: []string{"notes", "v1.1.0", "--offline", "--format", "text"},
			wantOut: "Bug Fixes\n\n- Stop crash"},
		{name: "from widens the range", config: cfg, history: history, args: []string{"notes", "v1.1.0", "--offline", "--from", "v1.0.0"},
			wantOut: "### Bug Fixes\n\n- Stop crash"},
		{name: "unknown tag", config: cfg, history: history, args: []string{"notes", "v9.9.9", "--offline"},
			wantExit: exitRuntime, wantText: []string{"v9.9.9"}},
		{name: "bad format is a usage error", config: cfg, history: history, args: []string{"notes", "v1.1.0", "--offline", "--format", "html"},
			wantExit: exitUsage, wantText: []string{"markdown", "text"}},
	})
}
```

Add one `scenario` with a `notes:` block in `config` that restores a heading (`release.section`) and asserts `wantOut` begins with it. Add a test (outside the table) that `--output` writes the file and stdout is empty, using `repo.Run` and `repo.ReadFile`. Confirm `step`/`commit`/`tag` helper names in `e2e/scenario_test.go` and adjust the literals if they differ.

- [ ] **Step 2: Run to verify they fail**

Run: `mise exec -- go test ./internal/cmd/ ./e2e/ -run 'Notes' -count=1`
Expected: FAIL (`unknown command "notes"`).

- [ ] **Step 3: Implement**

`internal/cmd/notes.go` follows `changelog.go`: `RunE`, flags declared in the constructor, `cobra.ExactArgs(1)`, `cfgPath, _ := cmd.Flags().GetString("config")` (check how `changelog.go` reads the persistent flags), `config.Load` → `applyOfflineOverride` → `config.Validate` (print config errors the way `changelog.go` does) → `env, err = app.ResolveEnv(env, cfg, runner)` → `app.GenerateNotes(runner, cfg, app.NotesOpts{...})`.

Exit mapping:

```go
switch {
case errors.Is(err, app.ErrNotesFormat):
	return exitcode.Wrap(exitcode.Usage, err)
case errors.Is(err, app.ErrNotesTagNotFound), errors.Is(err, app.ErrNotesRange):
	return exitcode.Wrap(exitcode.Runtime, err)
default:
	return wrapRunErr(err, "notes failed")
}
```

(Reuse `wrapRunErr`; open it to confirm its signature.) After success: print each `res.Warnings` entry to `cmd.ErrOrStderr()` via the existing UI warning helper `changelog.go` uses (or a plain `Fprintln` if none), then write `res.Text`: with `--output`, `os.WriteFile(path, []byte(res.Text+"\n"), 0o644)` wrapped in `exitcode.Wrap(exitcode.Runtime, …)`; otherwise `fmt.Fprintln(cmd.OutOrStdout(), res.Text)` only when `res.Text != ""`.

Use `execadapter.New(false, verbose)` as the runner (read-only, no dry-run). Register in `root.go`.

- [ ] **Step 4: Run to verify they pass**

Run: `mise exec -- go test ./... -count=1`
Expected: PASS (whole suite, including the e2e Lane A package).

- [ ] **Step 5: Commit**

```bash
mise exec -- hk fix -S go_fmt && git add -A && mise exec -- git commit -m "feat(cmd): add the notes command

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 6: docs, ADR, guide, roadmap close

**Files:**
- Modify: `docs/specs/03-commands.md` (new `## heraut notes` section: synopsis, flags table, behaviour table from the spec, exit codes), `docs/specs/02-configuration.md` (§ `notes`), `docs/specs/05-generators-and-platforms.md` (template precedence names the `notes` driver; the Modes sentence)
- Create: `docs/adr/0070-notes-command.md`
- Create: `docs/guides/store-release-notes.md`
- Modify: `README.md`, `CLAUDE.md` (command list; ADR count 70), `docs/superpowers/specs/2026-10-13-notes-command-design.md` (Status: Implemented; the two rulings), `docs/tasks/roadmap.md` (T371 `[x]`, completion note, Phase 66 `Done`)

**Interfaces:** none.

- [ ] **Step 1: Write the docs**

ADR-0070 (follow the layout of `docs/adr/0069-*.md`): context (store text needs the groups without forge-oriented decoration), decision (read-only `notes` command, `notes:` block, default lean blocks seeded below user snippets, ancestry-of-the-tag bound, `--format text` via a CommonMark parse), consequences (goldmark becomes a direct dependency; a global `commit.message` override reaches the notes; text mode drops markers CommonMark reads as formatting), rejected alternatives (character stripping, a second text template set).

Spec 03 must carry the behaviour table (tag/`--from`/format/empty-range/per-env rows), the exit codes (usage 1 for `--format`, runtime 3 for an unknown tag or range), and an example pipeline: `heraut release && heraut notes "$(heraut version current)" --format text --output store.txt`.

The guide shows the mobile workflow end to end: release, `heraut notes <tag> --format text`, a `notes.rendering.templates` example that adds a heading or the short hash, and the `--from` recipe for spanning release candidates.

Amend the design spec: Status Implemented; decision 2 and the Design section now say the bound is the previous release reachable from the requested tag; the config type is a dedicated `Notes` struct whose unknown keys fail strict parsing.

- [ ] **Step 2: Verify docs are consistent**

Run: `grep -rn "heraut notes" docs README.md CLAUDE.md | head -30` and read the hits for contradictions with the implementation; run `mise exec -- hk check`.
Expected: no contradictions; `hk check` clean.

- [ ] **Step 3: Close the roadmap and commit**

Flip T371 `[ ]` to `[x]`, mark Phase 66 Done, and add the completion note (what shipped, the two rulings, deferred: per-store limits, locales, a `notes.format` key, named profiles).

```bash
mise exec -- hk fix -S go_fmt && git add -A && mise exec -- git commit -m "docs: document the notes command and close T371

Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```
