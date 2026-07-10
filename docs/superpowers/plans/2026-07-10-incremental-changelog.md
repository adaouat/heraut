# Incremental Changelog Generation (native) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the native generator's changelog incremental by default — prepend only the new release's section into the existing file via a structural HTML-comment anchor, preserving history — with a `--regenerate` flag that rebuilds and re-enriches the whole file.

**Architecture:** A pure splice engine (`changelogfile.go`) parses the existing `CHANGELOG.md` by per-section anchors and inserts/replaces the new section. `generateChangelog` branches on a new `ContentDriver.RegenerateChangelog` flag: incremental (splice) vs full (rebuild all + enrich all). The flag is plumbed from a CLI option through `PipelineOpts` → `buildGenerator` onto the native driver (exactly like `HerautVersion` already is). A pipeline-level warning fires for GitLab full-regen (per-commit enrichment).

**Tech Stack:** Go, stdlib (`regexp`, `strings`, `os`); cobra (flags); existing `internal/{config,generators/native,app,pipeline,cmd}` packages. No new dependencies.

## Global Constraints

- **Native-only.** git-cliff/communique are untouched; the flag is a no-op for them.
- **No new Go dependencies.** stdlib + existing internal packages only.
- **Per-section render output is unchanged.** The anchor is emitted by the assembly layer (`generateChangelog`), never inside a template block, so `renderChangelogSection` output and its goldens (`internal/generators/native/testdata/*.golden`) stay byte-identical.
- **Anchor format (verbatim):** a line `<!-- heraut-release: <tag> -->` immediately before each section; `<tag>` is the release tag (e.g. `v0.49.0`), not the display version.
- **Foreign-file safety:** incremental mode on a non-empty anchorless file **errors** (naming `--regenerate`) and leaves the file byte-for-byte unchanged — never silently rewrites it.
- **Layer rules:** `internal/generators/native` imports only `internal/{port,config,conventionalcommit}` + stdlib; `internal/config` imports nothing from heraut.
- **TDD**: failing test first. **Lint** via `hk fix` (never gofmt directly). **Never** bypass hooks (`--no-verify` etc.).
- **Commit trailer:** `Co-Authored-By: Claude <model> <noreply@anthropic.com>` when Claude wrote the change.
- Enrichment batching is fixed: GitHub 50 SHAs/GraphQL query, Azure 1 query, GitLab per-commit.

---

### Task 1: Splice engine (pure functions)

**Files:**
- Create: `internal/generators/native/changelogfile.go`
- Test: `internal/generators/native/changelogfile_internal_test.go`

**Interfaces:**
- Produces:
  - `var ErrNoAnchors error`
  - `func anchorLine(tag string) string` → `"<!-- heraut-release: <tag> -->"`
  - `func spliceSection(existing, newBody, newTag string) (string, error)` — insert/replace the new section (body without anchor) into `existing`; `ErrNoAnchors` when `existing` is non-empty but anchorless.
  - `func parseChangelog(content string) (preamble string, sections []anchoredSection, hasAnchors bool)` (internal helper; `anchoredSection{tag, text string}`).

- [ ] **Step 1: Write the failing tests**

```go
package native

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnchorLine(t *testing.T) {
	assert.Equal(t, "<!-- heraut-release: v1.2.3 -->", anchorLine("v1.2.3"))
}

func TestParseChangelog_NoAnchors(t *testing.T) {
	pre, secs, has := parseChangelog("# Changelog\n\n## [1.0.0] - 2026-01-01\n\n- x\n")
	assert.False(t, has)
	assert.Empty(t, secs)
	assert.Equal(t, "# Changelog\n\n## [1.0.0] - 2026-01-01\n\n- x\n", pre)
}

func TestParseChangelog_SplitsSections(t *testing.T) {
	content := "# Changelog\n\n" +
		"<!-- heraut-release: v2.0.0 -->\n## [2.0.0]\n\n- two\n\n" +
		"<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"
	pre, secs, has := parseChangelog(content)
	require.True(t, has)
	assert.Equal(t, "# Changelog\n\n", pre)
	require.Len(t, secs, 2)
	assert.Equal(t, "v2.0.0", secs[0].tag)
	assert.Equal(t, "v1.0.0", secs[1].tag)
	assert.Equal(t, "<!-- heraut-release: v2.0.0 -->\n## [2.0.0]\n\n- two", secs[0].text)
}

func TestSpliceSection_InsertsAboveTop(t *testing.T) {
	existing := "# Changelog\n\n" +
		"<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"
	got, err := spliceSection(existing, "## [2.0.0]\n\n- two", "v2.0.0")
	require.NoError(t, err)
	want := "# Changelog\n\n" +
		"<!-- heraut-release: v2.0.0 -->\n## [2.0.0]\n\n- two\n\n" +
		"<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"
	assert.Equal(t, want, got)
}

func TestSpliceSection_ReplacesSameTag(t *testing.T) {
	existing := "# Changelog\n\n" +
		"<!-- heraut-release: v2.0.0 -->\n## [2.0.0]\n\n- OLD\n\n" +
		"<!-- heraut-release: v1.0.0 -->\n## [1.0.0]\n\n- one\n"
	got, err := spliceSection(existing, "## [2.0.0]\n\n- NEW", "v2.0.0")
	require.NoError(t, err)
	assert.Contains(t, got, "- NEW")
	assert.NotContains(t, got, "- OLD")
	assert.Equal(t, 1, strings.Count(got, "heraut-release: v2.0.0"), "no duplicate section")
	assert.Contains(t, got, "<!-- heraut-release: v1.0.0 -->", "older section preserved")
}

func TestSpliceSection_ForeignFileErrors(t *testing.T) {
	_, err := spliceSection("# Changelog\n\n## [1.0.0]\n\n- x\n", "## [2.0.0]\n\n- y", "v2.0.0")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoAnchors))
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'AnchorLine|ParseChangelog|SpliceSection'`
Expected: FAIL — `undefined: anchorLine` / `parseChangelog` / `spliceSection` / `ErrNoAnchors`.

- [ ] **Step 3: Implement `changelogfile.go`**

```go
package native

import (
	"errors"
	"regexp"
	"strings"
)

// ErrNoAnchors reports that a non-empty changelog has no heraut-release anchors — it was produced
// by another tool and cannot be safely spliced incrementally (the caller directs to --regenerate).
var ErrNoAnchors = errors.New("no heraut-release anchors")

const (
	anchorOpen  = "<!-- heraut-release: "
	anchorClose = " -->"
)

// anchorRe matches a section anchor line and captures the release tag.
var anchorRe = regexp.MustCompile(`(?m)^<!-- heraut-release: (.+) -->$`)

// anchorLine returns the render-invisible section anchor for a release tag.
func anchorLine(tag string) string { return anchorOpen + tag + anchorClose }

// anchoredSection is one release section: its tag plus the full block text (anchor line + body).
type anchoredSection struct {
	tag  string
	text string
}

// parseChangelog splits content into the preamble (everything before the first anchor) and the
// ordered anchored sections. hasAnchors is false when content contains no anchor line — in which
// case preamble is the whole content and sections is nil.
func parseChangelog(content string) (preamble string, sections []anchoredSection, hasAnchors bool) {
	locs := anchorRe.FindAllStringSubmatchIndex(content, -1)
	if len(locs) == 0 {
		return content, nil, false
	}
	preamble = content[:locs[0][0]]
	for i, m := range locs {
		end := len(content)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		sections = append(sections, anchoredSection{
			tag:  content[m[2]:m[3]],
			text: strings.TrimRight(content[m[0]:end], "\n"),
		})
	}
	return preamble, sections, true
}

// spliceSection inserts a freshly rendered section (newBody, without its anchor) for newTag into
// existing changelog content. If the top section already carries newTag it is replaced
// (idempotent); otherwise the new section is inserted above it, preserving the rest verbatim.
// ErrNoAnchors is returned when existing is non-empty but anchorless.
func spliceSection(existing, newBody, newTag string) (string, error) {
	preamble, sections, hasAnchors := parseChangelog(existing)
	if !hasAnchors {
		return "", ErrNoAnchors
	}
	block := anchoredSection{tag: newTag, text: anchorLine(newTag) + "\n" + newBody}
	if len(sections) > 0 && sections[0].tag == newTag {
		sections[0] = block
	} else {
		sections = append([]anchoredSection{block}, sections...)
	}
	texts := make([]string, len(sections))
	for i, s := range sections {
		texts[i] = s.text
	}
	return preamble + strings.Join(texts, "\n\n") + "\n", nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ -run 'AnchorLine|ParseChangelog|SpliceSection'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/native/changelogfile.go internal/generators/native/changelogfile_internal_test.go
git commit -m "feat(generators/native): changelog splice engine (anchors)

Co-Authored-By: Claude <model> <noreply@anthropic.com>"
```

---

### Task 2: generateChangelog — incremental / full / bootstrap

**Files:**
- Modify: `internal/config/config.go` (add `ContentDriver.RegenerateChangelog bool \`yaml:"-"\``)
- Modify: `internal/generators/native/generator.go` (`generateChangelog` branches; extract `buildAllSections`, add `generateIncremental`, `writeChangelog`)
- Test: `internal/generators/native/generator_internal_test.go`

**Interfaces:**
- Consumes: `spliceSection`, `anchorLine`, `ErrNoAnchors` (Task 1); `renderRelease`, `scopedTags`, `commitRange`, `changelogHeader` (existing).
- Produces: `config.ContentDriver.RegenerateChangelog`; `generateChangelog` now emits anchored sections and splices incrementally.

- [ ] **Step 1: Write the failing tests**

Add to `generator_internal_test.go` (helpers `record`, `New`, `exectest`, `config` already imported):

```go
func TestGenerateChangelog_BootstrapAnchored(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "CHANGELOG.md")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // scopedTags: no tags
	mr.QueueResponse(record("aaa1111111", "A", "a@x", "2026-01-01T00:00:00Z", "feat: initial", ""), "", nil) // latest..HEAD
	g := New(mr, &config.ContentDriver{Generator: "native", Output: out}, ModeChangelog)

	body, err := g.Generate("v0.1.0", nil)
	require.NoError(t, err)
	assert.Contains(t, body, "<!-- heraut-release: v0.1.0 -->", "bootstrap stamps an anchor")
	assert.Contains(t, body, "## [0.1.0]")
	written, _ := os.ReadFile(out)
	assert.Equal(t, body, string(written))
}

func TestGenerateChangelog_IncrementalInserts(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(out, []byte(
		"# Changelog\n\n<!-- heraut-release: v1.0.0 -->\n## [1.0.0] - 2026-01-01\n\n### 🚀 Features\n\n- old by @carol\n"), 0o644))
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil) // scopedTags
	mr.QueueResponse(record("bbb2222222", "B", "b@x", "2026-02-01T00:00:00Z", "feat: new thing", ""), "", nil) // v1.0.0..HEAD
	g := New(mr, &config.ContentDriver{Generator: "native", Output: out}, ModeChangelog)

	body, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)
	assert.Contains(t, body, "<!-- heraut-release: v1.1.0 -->")
	assert.Contains(t, body, "New thing")
	assert.Contains(t, body, "- old by @carol", "existing section preserved verbatim (attribution kept)")
	assert.Less(t, strings.Index(body, "v1.1.0"), strings.Index(body, "v1.0.0"), "new section on top")
}

func TestGenerateChangelog_ForeignFileErrors(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "CHANGELOG.md")
	foreign := "# Changelog\n\n## [1.0.0] - 2026-01-01\n\n- git-cliff line by @dave\n"
	require.NoError(t, os.WriteFile(out, []byte(foreign), 0o644))
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)
	mr.QueueResponse(record("bbb2222222", "B", "b@x", "2026-02-01T00:00:00Z", "feat: x", ""), "", nil)
	g := New(mr, &config.ContentDriver{Generator: "native", Output: out}, ModeChangelog)

	_, err := g.Generate("v1.1.0", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--regenerate")
	unchanged, _ := os.ReadFile(out)
	assert.Equal(t, foreign, string(unchanged), "foreign file left untouched")
}

func TestGenerateChangelog_RegenerateEnrichesAllSections(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "CHANGELOG.md")
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil) // scopedTags
	// new (v1.0.0..HEAD) — none classifiable → skipped
	mr.QueueResponse("", "", nil) // collectCommits latest..HEAD (empty)
	// existing tag v1.0.0 section: commits + enrichment (regenerate enriches history)
	mr.QueueResponse(record("ccc3333333", "C", "c@x", "2026-01-01T00:00:00Z", "feat: shipped", ""), "", nil) // collectCommits ""..v1.0.0
	mr.QueueResponse(ghGraphQLResponse(7, "https://github.com/o/r/pull/7", "carol"), "", nil) // gh enrich historical section
	g := New(mr, &config.ContentDriver{Generator: "native", Output: out, RegenerateChangelog: true}, ModeChangelog)

	body, err := g.Generate("v1.1.0", ghLC())
	require.NoError(t, err)
	assert.Contains(t, body, "by @carol", "regenerate enriches historical sections")
	assert.Contains(t, body, "<!-- heraut-release: v1.0.0 -->")
}

func TestGenerateChangelog_IncrementalWithCustomHeader(t *testing.T) {
	// A custom header block does not break splicing: the anchor is emitted by the assembly layer,
	// not the header, so the parser still finds section boundaries (ADR-0037 + ADR-0038).
	dir := t.TempDir()
	out := filepath.Join(dir, "CHANGELOG.md")
	require.NoError(t, os.WriteFile(out, []byte(
		"# Changelog\n\n<!-- heraut-release: v1.0.0 -->\n=== 1.0.0 ===\n\n- old\n"), 0o644))
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0\n", "", nil)
	mr.QueueResponse(record("bbb2222222", "B", "b@x", "2026-02-01T00:00:00Z", "feat: new", ""), "", nil)
	g := New(mr, &config.ContentDriver{
		Generator:          "native",
		Output:             out,
		EffectiveTemplates: map[string]string{"header": "=== {{ .Version }} ==="},
	}, ModeChangelog)

	body, err := g.Generate("v1.1.0", nil)
	require.NoError(t, err)
	assert.Contains(t, body, "<!-- heraut-release: v1.1.0 -->\n=== 1.1.0 ===", "custom header still splices under the structural anchor")
	assert.Contains(t, body, "- old", "history preserved")
}
```

Note: `ghLC()` and `ghGraphQLResponse` already exist in `enrich_internal_test.go`. Confirm the exact MockRunner call order for `RegenerateChangelog` against the real `renderRelease`/`enrichForRelease` sequence when running the test; adjust the queued responses to match (the assertions on `body` are the contract, the queue order is implementation-detail bookkeeping).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/generators/native/ -run 'GenerateChangelog_(BootstrapAnchored|IncrementalInserts|ForeignFileErrors|RegenerateEnrichesAllSections)'`
Expected: FAIL — `RegenerateChangelog` unknown field; no anchors emitted; foreign file overwritten instead of erroring.

- [ ] **Step 3a: Add the config field**

In `internal/config/config.go`, in the `ContentDriver` struct after `EffectiveTemplates`:

```go
	// RegenerateChangelog forces the native generator to rebuild the entire changelog and
	// re-enrich every section, instead of incrementally splicing only the new release's section.
	// Set by the app layer from the --regenerate / --regenerate-changelog flag. (native only.)
	RegenerateChangelog bool `yaml:"-"`
```

- [ ] **Step 3b: Rewrite `generateChangelog` in `generator.go`**

Replace the existing `generateChangelog` body with the branch + helpers (imports already include `os`, `fmt`, `strings`, `errors` — add `errors` if missing):

```go
func (g *Generator) generateChangelog(tag string, lc *port.LinkContext) (string, error) {
	if g.cfg.RegenerateChangelog {
		body, err := g.buildAllSections(tag, lc, true) // enrich every section
		if err != nil {
			return "", err
		}
		return g.writeChangelog(body)
	}
	return g.generateIncremental(tag, lc)
}

// buildAllSections renders every release section (newest-first), each prefixed with its anchor.
// The newest (unreleased) section is always enriched; historical sections are enriched only when
// enrichAll is true (--regenerate). Matches the pre-incremental full-regen layout plus anchors.
func (g *Generator) buildAllSections(tag string, lc *port.LinkContext, enrichAll bool) (string, error) {
	tags, err := g.scopedTags()
	if err != nil {
		return "", err
	}
	var blocks []string
	latest := ""
	if len(tags) > 0 {
		latest = tags[0]
	}
	if sec, err := g.renderRelease(tag, latest, commitRange(latest, "HEAD"), lc, true); err != nil {
		return "", err
	} else if sec != "" {
		blocks = append(blocks, anchorLine(tag)+"\n"+sec)
	}
	for i, t := range tags {
		prev := ""
		if i+1 < len(tags) {
			prev = tags[i+1]
		}
		if sec, err := g.renderRelease(t, prev, commitRange(prev, t), lc, enrichAll); err != nil {
			return "", err
		} else if sec != "" {
			blocks = append(blocks, anchorLine(t)+"\n"+sec)
		}
	}
	return changelogHeader + strings.Join(blocks, "\n\n") + "\n", nil
}

// generateIncremental splices only the new release's section into the existing changelog,
// preserving all historical sections (and their attribution). It bootstraps a full build when the
// file is missing/empty, and errors when the file is non-empty but has no anchors.
func (g *Generator) generateIncremental(tag string, lc *port.LinkContext) (string, error) {
	var existing string
	if g.cfg.Output != "" {
		b, err := os.ReadFile(g.cfg.Output)
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("reading changelog %q: %w", g.cfg.Output, err)
		}
		existing = string(b)
	}
	if strings.TrimSpace(existing) == "" {
		body, err := g.buildAllSections(tag, lc, false) // bootstrap: enrich newest only
		if err != nil {
			return "", err
		}
		return g.writeChangelog(body)
	}

	tags, err := g.scopedTags()
	if err != nil {
		return "", err
	}
	latest := ""
	if len(tags) > 0 {
		latest = tags[0]
	}
	newBody, err := g.renderRelease(tag, latest, commitRange(latest, "HEAD"), lc, true)
	if err != nil {
		return "", err
	}
	if newBody == "" {
		return existing, nil // nothing new to add; leave the file untouched
	}
	body, err := spliceSection(existing, newBody, tag)
	if errors.Is(err, ErrNoAnchors) {
		return "", fmt.Errorf("changelog %q has no heraut-release anchors (generated by another tool?); run with --regenerate to rebuild it with anchors and full PR attribution: %w", g.cfg.Output, err)
	}
	if err != nil {
		return "", err
	}
	return g.writeChangelog(body)
}

// writeChangelog writes body to cfg.Output when set and returns it.
func (g *Generator) writeChangelog(body string) (string, error) {
	if g.cfg.Output != "" {
		if err := os.WriteFile(g.cfg.Output, []byte(body), 0o644); err != nil {
			return "", fmt.Errorf("writing changelog %q: %w", g.cfg.Output, err)
		}
	}
	return body, nil
}
```

Ensure `errors` is imported in `generator.go`.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/generators/native/ ./internal/config/`
Expected: PASS (existing changelog generator tests use `Contains`, so anchors don't break them). If a pre-existing test asserts an exact full-changelog body, update it to expect the anchor lines (search for such assertions first).

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/generators/native/generator.go internal/generators/native/generator_internal_test.go
git commit -m "feat(generators/native): incremental changelog with --regenerate rebuild

Co-Authored-By: Claude <model> <noreply@anthropic.com>"
```

---

### Task 3: Flag plumbing (opts → commands → generator)

**Files:**
- Modify: `internal/app/pipeline.go` (`PipelineOpts.RegenerateChangelog`; thread through `buildReleasePipelineConfig`, `buildChangelogPipelineConfig`, `buildGenerator`)
- Modify: `internal/cmd/changelog.go` (`--regenerate` flag → `opts.RegenerateChangelog`)
- Modify: `internal/cmd/release.go` (`--regenerate-changelog` flag → `opts.RegenerateChangelog`)
- Test: `internal/cmd/changelog_test.go`, `internal/cmd/release_test.go`

**Interfaces:**
- Consumes: `config.ContentDriver.RegenerateChangelog` (Task 2).
- Produces: `PipelineOpts.RegenerateChangelog bool`; `buildGenerator(runner, driver, mode, herautVersion string, regenerateChangelog bool)`.

- [ ] **Step 1: Write the failing tests**

In `internal/cmd/changelog_test.go`:

```go
func TestNewChangelogCmd_RegenerateFlag(t *testing.T) {
	c := cmd.NewChangelogCmd("v0.0.0-test")
	f := c.Flags().Lookup("regenerate")
	require.NotNil(t, f, "changelog has a --regenerate flag")
	assert.Equal(t, "false", f.DefValue)
}
```

In `internal/cmd/release_test.go`:

```go
func TestNewReleaseCmd_RegenerateChangelogFlag(t *testing.T) {
	c := cmd.NewReleaseCmd("v0.0.0-test")
	f := c.Flags().Lookup("regenerate-changelog")
	require.NotNil(t, f, "release has a --regenerate-changelog flag")
	assert.Equal(t, "false", f.DefValue)
}
```

(Ensure `require`/`assert` are imported in both test files.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cmd/ -run 'RegenerateFlag|RegenerateChangelogFlag'`
Expected: FAIL — the flags are nil.

- [ ] **Step 3a: `PipelineOpts` + app plumbing (`internal/app/pipeline.go`)**

Add to `PipelineOpts` (after `HerautVersion`):

```go
	// RegenerateChangelog forces the native changelog generator to rebuild + re-enrich the whole
	// file instead of incrementally splicing the new section. Native only.
	RegenerateChangelog bool
```

Change `buildGenerator` to accept and apply the flag (native clone only):

```go
func buildGenerator(runner port.Runner, driver *config.ContentDriver, defaultMode gitcliff.Mode, herautVersion string, regenerateChangelog bool) (port.Generator, error) {
	switch driver.Generator {
	case "git-cliff":
		return gitcliff.New(runner, driver, defaultMode), nil
	case "communique":
		return communique.New(runner, driver), nil
	case "native":
		nativeDriver := *driver
		nativeDriver.HerautVersion = herautVersion
		nativeDriver.RegenerateChangelog = regenerateChangelog
		return native.New(runner, &nativeDriver, nativeMode(defaultMode)), nil
	default:
		return nil, fmt.Errorf("unsupported generator %q (supported: native, git-cliff, communique)", driver.Generator)
	}
}
```

Update `buildReleasePipelineConfig` signature to `(runner, cfg, env, herautVersion string, regenerateChangelog bool)` and pass `regenerateChangelog` to both `buildGenerator` calls (changelog + notes). In `BuildPipeline`, call it with `opts.HerautVersion, opts.RegenerateChangelog`.

In `buildChangelogPipelineConfig`, pass `opts.HerautVersion, opts.RegenerateChangelog` to its `buildGenerator` call.

- [ ] **Step 3b: `--regenerate` on changelog (`internal/cmd/changelog.go`)**

In `NewChangelogCmd`, declare a `var regenerate bool` alongside the other flag vars, set `RegenerateChangelog: regenerate` in the `app.PipelineOpts{…}` literal, and register the flag near the other `changelogCmd.Flags()` calls:

```go
	changelogCmd.Flags().BoolVar(&regenerate, "regenerate", false,
		"rebuild the entire changelog and re-fetch PR attribution (instead of incrementally adding the new section)")
```

- [ ] **Step 3c: `--regenerate-changelog` on release (`internal/cmd/release.go`)**

In `NewReleaseCmd`, declare `var regenerateChangelog bool`, set `RegenerateChangelog: regenerateChangelog` in the `app.PipelineOpts{…}` literal, and register:

```go
	releaseCmd.Flags().BoolVar(&regenerateChangelog, "regenerate-changelog", false,
		"rebuild the entire changelog and re-fetch PR attribution (needed once when migrating a changelog to the native generator)")
```

- [ ] **Step 4: Run to verify it passes**

Run: `go build ./... && go test ./internal/cmd/ ./internal/app/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/pipeline.go internal/cmd/changelog.go internal/cmd/release.go internal/cmd/changelog_test.go internal/cmd/release_test.go
git commit -m "feat(cmd): --regenerate / --regenerate-changelog flags for native changelog

Co-Authored-By: Claude <model> <noreply@anthropic.com>"
```

---

### Task 4: GitLab full-regen warning

**Files:**
- Modify: `internal/pipeline/changelog.go` (`ChangelogConfig.RegenerateChangelog`; warn in the generate step)
- Modify: `internal/pipeline/release.go` (`Config.RegenerateChangelog`; warn in the changelog step)
- Modify: `internal/app/pipeline.go` (set the pipeline-config field from `opts.RegenerateChangelog`)
- Create: `internal/pipeline/warn.go` (pure `gitlabRegenWarning` helper)
- Test: `internal/pipeline/warn_internal_test.go`

**Interfaces:**
- Produces: `func gitlabRegenWarning(regenerate bool, lc *port.LinkContext) []string` — a one-line warning slice when `regenerate` and the remote is GitLab, else nil.

- [ ] **Step 1: Write the failing test**

`internal/pipeline/warn_internal_test.go`:

```go
package pipeline

import (
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
)

func TestGitlabRegenWarning(t *testing.T) {
	gl := &port.LinkContext{Platform: "gitlab"}
	gh := &port.LinkContext{Platform: "github"}
	assert.Nil(t, gitlabRegenWarning(false, gl), "no warning without --regenerate")
	assert.Nil(t, gitlabRegenWarning(true, gh), "no warning on github (batched)")
	assert.Nil(t, gitlabRegenWarning(true, nil), "no warning without a remote")
	w := gitlabRegenWarning(true, gl)
	assert.Len(t, w, 1)
	assert.Contains(t, w[0], "GitLab")
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/pipeline/ -run 'GitlabRegenWarning'`
Expected: FAIL — `undefined: gitlabRegenWarning`.

- [ ] **Step 3a: Implement the helper (`internal/pipeline/warn.go`)**

```go
package pipeline

import "github.com/adaouat/heraut/internal/port"

// gitlabRegenWarning returns a one-line caution when a full changelog regeneration will enrich
// every section against a GitLab remote — GitLab has no batched per-commit-MR primitive, so it is
// one API call per commit (slow / rate-limited). GitHub and Azure batch, so they need no warning.
func gitlabRegenWarning(regenerate bool, lc *port.LinkContext) []string {
	if regenerate && lc != nil && lc.Platform == "gitlab" {
		return []string{"--regenerate re-fetches PR metadata one call per commit on GitLab; this may be slow and hit rate limits"}
	}
	return nil
}
```

- [ ] **Step 3b: Wire into the two changelog steps**

In `internal/pipeline/changelog.go`, add `RegenerateChangelog bool` to `ChangelogConfig`, and in the "Generate changelog" step return the warning as sub-results:

```go
		if err := p.runStep("Generate changelog", func() (string, []string, error) {
			if _, err := p.cfg.Changelog.Generate(result.Tag, changelogCtx); err != nil {
				return "", nil, fmt.Errorf("generating changelog: %w", err)
			}
			subs := append(gitlabRegenWarning(p.cfg.RegenerateChangelog, changelogCtx), degradedSubResult(p.cfg.Changelog)...)
			return "", subs, nil
		}); err != nil {
```

In `internal/pipeline/release.go`, add `RegenerateChangelog bool` to `Config`, and merge `gitlabRegenWarning(p.cfg.RegenerateChangelog, changelogCtx)` into that step's sub-results the same way (find the `p.cfg.Changelog.Generate(result.Tag, changelogCtx)` call ~line 122).

- [ ] **Step 3c: Set the field in the app layer**

In `internal/app/pipeline.go`, in `buildChangelogPipelineConfig` set `cCfg.RegenerateChangelog = opts.RegenerateChangelog`; in `buildReleasePipelineConfig` set `pCfg.RegenerateChangelog = <regenerateChangelog param>` (add the field assignment near where the changelog generator is built).

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/pipeline/ ./internal/app/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/pipeline/ internal/app/pipeline.go
git commit -m "feat(pipeline): warn on GitLab full changelog regeneration

Co-Authored-By: Claude <model> <noreply@anthropic.com>"
```

---

### Task 5: Docs, ADR, and CI migration

**Files:**
- Create: `docs/adr/0038-incremental-changelog.md`; Modify `docs/adr/README.md`
- Modify: `docs/specs/05-generators-and-platforms.md` (changelog file structure + incremental/full modes)
- Modify: `docs/specs/03-commands.md` (the `--regenerate` / `--regenerate-changelog` flags)
- Modify: `.github/workflows/release.yml` (one-time `regenerate_changelog` workflow_dispatch input)
- Modify: `docs/tasks/native-generator-roadmap.md` (Phase 2.9 note, `[x]`)

- [ ] **Step 1: Write ADR-0038**

Create `docs/adr/0038-incremental-changelog.md` (status Accepted, date 2026-07-10) capturing: the incremental default (anchor-based splice), the `--regenerate` full rebuild + re-enrich (batched; GitLab warning), the anchor format/contract (`<!-- heraut-release: <tag> -->`, emitted by the assembly layer, non-overridable, decoupled from the ADR-0037 customizable header), the foreign-anchorless stop-error, native-only scope, and the reference to the design spec `docs/superpowers/specs/2026-07-10-incremental-changelog-design.md`. Add the row to `docs/adr/README.md`:

```markdown
| [0038](0038-incremental-changelog.md) | Incremental Changelog Generation (native) | Accepted |
```

- [ ] **Step 2: Update the specs**

In `docs/specs/05-generators-and-platforms.md`, under the native section, add a "Changelog structure & incremental generation" subsection: the file is a preamble + anchored sections; incremental (default) splices the new section preserving history; `--regenerate` rebuilds + re-enriches all; foreign/anchorless files stop with a directive to `--regenerate`. In `docs/specs/03-commands.md`, document `heraut changelog --regenerate` and `heraut release --regenerate-changelog`.

- [ ] **Step 3: Add the CI migration input**

In `.github/workflows/release.yml`, add to `workflow_dispatch.inputs`:

```yaml
      regenerate_changelog:
        description: "Rebuild the whole changelog + re-fetch PR attribution (once, when migrating to the native generator)."
        type: boolean
        default: false
```

and change the Release step command to pass the flag conditionally:

```yaml
      - name: Release
        run: '"./$FRESH_BIN" release --version "$VERSION" ${{ inputs.regenerate_changelog && ''--regenerate-changelog'' || '''' }}'
```

- [ ] **Step 4: Roadmap note + verify**

Add a `Phase 2.9 — incremental changelog (ADR-0038)` row + `[x]` note to `docs/tasks/native-generator-roadmap.md`. Then:

Run: `go test ./... && hk fix`
Expected: all green; lint clean.

- [ ] **Step 5: Commit**

```bash
git add docs/ .github/workflows/release.yml
git commit -m "docs(adr): 0038 incremental changelog; specs, roadmap, CI migration

Co-Authored-By: Claude <model> <noreply@anthropic.com>"
```

---

## Notes for the implementer

- **The anchor is an assembly concern, never a template block.** Do not add it to `changelog.tmpl`'s `header` block — that would make it user-overridable and break splicing. It is added in `buildAllSections` / `spliceSection` only.
- **`renderChangelogSection` and its goldens must not change.** If a golden diff appears, the anchor leaked into the section renderer — back it out.
- **Foreign-file safety is load-bearing.** The `ForeignFileErrors` test asserts the file is byte-for-byte unchanged; never write in that path.
- **MockRunner queue order** in Task 2's `--regenerate` test is bookkeeping — the `body` assertions are the real contract. Run the test, read the actual call sequence from the failure, and align the queued responses; do not weaken the `body` assertions to make the queue simpler.
- **buildGenerator now carries two app-computed native scalars** (`herautVersion`, `regenerateChangelog`). If a third appears later, bundle them into a struct — for two, positional params match the existing style.
