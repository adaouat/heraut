# Ticket Linking Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add first-class ticket linking to `.heraut.yml` — match ticket patterns (Jira/Linear/GitHub-issue) in commits and render them as links in the git-cliff changelog **and** release notes.

**Architecture:** A top-level `tickets:` list is validated in `internal/config`, propagated onto each content driver by the app layer (exactly like `remote_metadata`), and injected as git-cliff `link_parsers` into the effective merged TOML by the gitcliff generator. Both embedded templates render `commit.links`. git-cliff only.

**Tech Stack:** Go, `github.com/pelletier/go-toml/v2`, `gopkg.in/yaml.v3`, git-cliff 2.13, testify. Design spec: `.claude/plans/ticket-linking.md`.

**Conventions (read before starting):** TDD is mandatory (red→green). Conventional commits, end every commit message with `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`. Never pass `--no-verify`; if a hook fails, fix the cause (use `hk fix` for lint). Run tests with `go test ./...`.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `internal/config/config.go` | `Ticket` struct, `Config.Tickets`, driver carrier | modify |
| `internal/config/validator.go` | `validateTickets` (regex, url, generator) | modify |
| `internal/config/validator_test.go`, `loader_test.go` | config tests | modify |
| `internal/app/pipeline.go` | propagate `cfg.Tickets` onto the driver | modify |
| `internal/app/tickets_internal_test.go` | propagation unit test | create |
| `internal/generators/gitcliff/generator.go` | `injectLinkParsers`, wire into `effectiveConfig` | modify |
| `internal/generators/gitcliff/generator_test.go` | injection unit test | modify |
| `internal/generators/gitcliff/cliff.changelog.toml`, `cliff.release-notes.toml` | render `commit.links` | modify |
| `schema.json`, `docs/heraut.sample.yml` | config surface | modify |
| `testdata/config/valid/tickets.yml`, `testdata/config/invalid/invalid_tickets.yml` | schema fixtures | create |
| `internal/config/schema_test.go` | invalid-fixture row | modify |
| `docs/adr/0024-ticket-linking.md` | decision record | create |
| `docs/specs/02-configuration.md` | document `tickets` | modify |
| `docs/tasks/roadmap.md` | T79 + done note | modify |

---

## Task 1: Config — `Ticket` struct, `Config.Tickets`, driver carrier

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/loader_test.go`

- [ ] **Step 1: Write the failing test** — append to `internal/config/loader_test.go`:

```go
func TestLoad_Tickets(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
tickets:
  - pattern: '[A-Z]+-[0-9]+'
    url: 'https://acme.atlassian.net/browse/{ticket}'
`)
	require.Len(t, cfg.Tickets, 1)
	assert.Equal(t, "[A-Z]+-[0-9]+", cfg.Tickets[0].Pattern)
	assert.Equal(t, "https://acme.atlassian.net/browse/{ticket}", cfg.Tickets[0].URL)
}
```

(If `mustLoad`/imports aren't already in `loader_test.go`, mirror `validator_test.go`. Check first.)

- [ ] **Step 2: Run test — verify it fails**

Run: `go test ./internal/config/ -run TestLoad_Tickets`
Expected: FAIL — `field tickets not found in type config.Config` (strict loader rejects unknown key).

- [ ] **Step 3: Add the struct + fields** in `internal/config/config.go`.

Add the top-level field to `Config` (after `Environments`):
```go
	// Tickets configures issue-tracker links: each entry's regex is matched in commit
	// messages (subject/body/footer) and rendered as a link. git-cliff only (T79 / ADR-0024).
	Tickets []Ticket `yaml:"tickets,omitempty"`
```
Add the type (near `ContentDriver`):
```go
// Ticket maps a commit ticket-ID pattern to a URL template. {ticket} in URL is the first
// capture group of Pattern (or the full match if Pattern has no group); the link label is
// always the full match.
type Ticket struct {
	Pattern string `yaml:"pattern"`
	URL     string `yaml:"url"`
}
```
Add the programmatic carrier to `ContentDriver` (next to `RemoteMetadata`):
```go
	// Tickets is the effective top-level Config.Tickets, propagated onto the driver by the
	// app layer so the generator can inject link_parsers. Not user-configurable per-driver.
	Tickets []Ticket `yaml:"-"`
```

- [ ] **Step 4: Run test — verify it passes**

Run: `go test ./internal/config/ -run TestLoad_Tickets`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/loader_test.go
git commit -m "feat(config): add tickets field for issue-tracker links

Part of T79." -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: Config — `validateTickets`

**Files:**
- Modify: `internal/config/validator.go`
- Test: `internal/config/validator_test.go`

- [ ] **Step 1: Write the failing tests** — append to `internal/config/validator_test.go`:

```go
// ── tickets ──────────────────────────────────────────────────────────────────

func TestValidate_TicketsValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
tickets:
  - pattern: '[A-Z]+-[0-9]+'
    url: 'https://acme.atlassian.net/browse/{ticket}'
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_TicketsInvalidRegex(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
tickets:
  - pattern: '[A-Z'
    url: 'https://x.test/{ticket}'
`)
	e := findErr(config.Validate(cfg), "tickets[0].pattern")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "regex")
}

func TestValidate_TicketsURLMissingPlaceholder(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
tickets:
  - pattern: '[A-Z]+-[0-9]+'
    url: 'https://x.test/browse/'
`)
	e := findErr(config.Validate(cfg), "tickets[0].url")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "{ticket}")
}

func TestValidate_TicketsNonGitCliffGenerator(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
changelog:
  generator: cocogitto
tickets:
  - pattern: '[A-Z]+-[0-9]+'
    url: 'https://x.test/{ticket}'
`)
	e := findErr(config.Validate(cfg), "tickets")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "git-cliff")
}
```

- [ ] **Step 2: Run tests — verify they fail**

Run: `go test ./internal/config/ -run TestValidate_Tickets`
Expected: FAIL — the four `findErr` lookups return nil / `TicketsValid` may pass vacuously. (No `validateTickets` yet.)

- [ ] **Step 3: Implement `validateTickets`** in `internal/config/validator.go`.

Add `"regexp"` to the imports. Add the function and wire it into `Validate` (add `errs = append(errs, validateTickets(cfg)...)` alongside the other layers):

```go
func validateTickets(cfg *Config) []ValidationError {
	if len(cfg.Tickets) == 0 {
		return nil
	}
	var errs []ValidationError
	if !ticketsGeneratorSupported(cfg) {
		errs = append(errs, ValidationError{
			Path:    "tickets",
			Message: "ticket linking requires the git-cliff generator",
			Hint:    "set changelog.generator / release.notes.generator to git-cliff, or remove tickets",
		})
	}
	for i, t := range cfg.Tickets {
		base := fmt.Sprintf("tickets[%d]", i)
		switch {
		case t.Pattern == "":
			errs = append(errs, ValidationError{Path: base + ".pattern", Message: "required", Hint: "a regex matching ticket IDs, e.g. [A-Z]+-[0-9]+"})
		default:
			if _, err := regexp.Compile(t.Pattern); err != nil {
				errs = append(errs, ValidationError{Path: base + ".pattern", Message: fmt.Sprintf("invalid regex: %v", err), Hint: "use a valid Go/git-cliff regex"})
			}
		}
		switch {
		case t.URL == "":
			errs = append(errs, ValidationError{Path: base + ".url", Message: "required", Hint: "an http(s) URL containing {ticket}, e.g. https://jira.example.com/browse/{ticket}"})
		default:
			if !strings.Contains(t.URL, "{ticket}") {
				errs = append(errs, ValidationError{Path: base + ".url", Message: "must contain the {ticket} placeholder", Hint: "e.g. https://jira.example.com/browse/{ticket}"})
			}
			if !isValidBaseURL(strings.ReplaceAll(t.URL, "{ticket}", "1")) {
				errs = append(errs, ValidationError{Path: base + ".url", Message: "must be an absolute http(s) URL", Hint: "include the scheme, e.g. https://…"})
			}
		}
	}
	return errs
}

// ticketsGeneratorSupported reports whether every configured top-level content generator is
// git-cliff (the only generator with link support). Empty generator (inherits default) is OK.
func ticketsGeneratorSupported(cfg *Config) bool {
	drivers := []*ContentDriver{cfg.Changelog}
	if cfg.Release != nil {
		drivers = append(drivers, cfg.Release.Notes)
	}
	for _, d := range drivers {
		if d != nil && d.Generator != "" && !strings.EqualFold(d.Generator, "git-cliff") {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run tests — verify they pass**

Run: `go test ./internal/config/ -run TestValidate_Tickets`
Expected: PASS (all four).

- [ ] **Step 5: Run the full config package — no regressions**

Run: `go test ./internal/config/`
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/config/validator.go internal/config/validator_test.go
git commit -m "feat(config): validate tickets (regex, {ticket} url, git-cliff only)

Part of T79." -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: gitcliff — inject `link_parsers`

**Files:**
- Modify: `internal/generators/gitcliff/generator.go`
- Test: `internal/generators/gitcliff/generator_test.go`

- [ ] **Step 1: Write the failing test** — append to `internal/generators/gitcliff/generator_test.go`:

```go
func TestEffectiveConfig_InjectsTicketLinkParsers(t *testing.T) {
	cfg := &config.ContentDriver{Generator: "git-cliff", Tickets: []config.Ticket{
		{Pattern: "[A-Z]+-[0-9]+", URL: "https://acme.atlassian.net/browse/{ticket}"}, // no group → wrapped
		{Pattern: "GH-([0-9]+)", URL: "https://github.com/acme/app/issues/{ticket}"},  // group → as-is
	}}
	gen := gitcliff.New(nil, cfg, gitcliff.ModeReleaseNotes)

	toml, err := gen.EffectiveReleaseNotesConfig()
	require.NoError(t, err)
	// no-group pattern wrapped in a capture group; {ticket} → $1
	assert.Contains(t, toml, `pattern = '([A-Z]+-[0-9]+)'`)
	assert.Contains(t, toml, `href = 'https://acme.atlassian.net/browse/$1'`)
	// already-grouped pattern left as-is
	assert.Contains(t, toml, `pattern = 'GH-([0-9]+)'`)
	assert.Contains(t, toml, `href = 'https://github.com/acme/app/issues/$1'`)
}

func TestEffectiveConfig_NoTickets_NoLinkParsers(t *testing.T) {
	cfg := &config.ContentDriver{Generator: "git-cliff"}
	gen := gitcliff.New(nil, cfg, gitcliff.ModeChangelog)
	toml, err := gen.EffectiveChangelogConfig()
	require.NoError(t, err)
	assert.NotContains(t, toml, "atlassian")
}
```

(The exact quote style git-cliff/go-toml emits — `'...'` vs `"..."` — may differ; if the `Contains` assertions miss on quote style after Step 3, adjust the expected substrings to the emitted form, do **not** weaken them to ignore the value.)

- [ ] **Step 2: Run test — verify it fails**

Run: `go test ./internal/generators/gitcliff/ -run TestEffectiveConfig_InjectsTicketLinkParsers`
Expected: FAIL — no `atlassian`/`href` in the output (no injection yet).

- [ ] **Step 3: Implement `injectLinkParsers`** in `internal/generators/gitcliff/generator.go`.

Add `"regexp"` to imports. Add the function (mirror `injectHeadingPostprocessor`, but target `[git].link_parsers` and **append**):

```go
// injectLinkParsers appends one git-cliff link_parser per ticket to the [git] table of the
// merged TOML: { pattern = <P>, href = <url with {ticket}→$1> }. <P> is the user pattern
// wrapped in a capture group only when it has none, so $1 is the URL value; the link label
// defaults to the full match. Existing user link_parsers are preserved (appended after).
func injectLinkParsers(merged string, tickets []config.Ticket) (string, error) {
	if len(tickets) == 0 {
		return merged, nil
	}
	var doc map[string]any
	if err := toml.Unmarshal([]byte(merged), &doc); err != nil {
		return "", fmt.Errorf("parsing merged TOML for link_parsers injection: %w", err)
	}
	git, _ := doc["git"].(map[string]any)
	if git == nil {
		git = make(map[string]any)
		doc["git"] = git
	}
	var entries []any
	for _, t := range tickets {
		pattern := t.Pattern
		re, err := regexp.Compile(pattern)
		if err != nil {
			return "", fmt.Errorf("ticket pattern %q: %w", t.Pattern, err)
		}
		if re.NumSubexp() == 0 {
			pattern = "(" + pattern + ")"
		}
		href := strings.ReplaceAll(t.URL, "{ticket}", "$1")
		entries = append(entries, map[string]any{"pattern": pattern, "href": href})
	}
	switch existing := git["link_parsers"].(type) {
	case []any:
		git["link_parsers"] = append(existing, entries...)
	default:
		git["link_parsers"] = entries
	}
	out, err := toml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshalling TOML after link_parsers injection: %w", err)
	}
	return string(out), nil
}
```

Wire it into `effectiveConfig` — change the final return to chain both injections:
```go
	withHeading, err := injectHeadingPostprocessor(merged, g.cfg.HeadingVersionPattern)
	if err != nil {
		return "", err
	}
	return injectLinkParsers(withHeading, g.cfg.Tickets)
```

- [ ] **Step 4: Run test — verify it passes**

Run: `go test ./internal/generators/gitcliff/ -run TestEffectiveConfig`
Expected: PASS (adjust expected quote style per the Step 1 note if needed).

- [ ] **Step 5: Run the gitcliff package — no regressions**

Run: `go test ./internal/generators/gitcliff/`
Expected: `ok` (including the real-CLI `TestEmbeddedConfig_RealGitCliff` if git-cliff is on PATH — confirms the injected TOML still parses).

- [ ] **Step 6: Commit**

```bash
git add internal/generators/gitcliff/generator.go internal/generators/gitcliff/generator_test.go
git commit -m "feat(generators/gitcliff): inject ticket link_parsers into effective config

Part of T79." -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: app — propagate `cfg.Tickets` onto the driver

**Files:**
- Modify: `internal/app/pipeline.go` (`withEnvDerivations`)
- Test: `internal/app/tickets_internal_test.go` (create)

- [ ] **Step 1: Write the failing test** — create `internal/app/tickets_internal_test.go`:

```go
package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/config"
)

func TestWithEnvDerivations_CarriesTickets(t *testing.T) {
	driver := &config.ContentDriver{Generator: "git-cliff"}
	cfg := &config.Config{
		Changelog: driver,
		Tickets:   []config.Ticket{{Pattern: "[A-Z]+-[0-9]+", URL: "https://x.test/{ticket}"}},
	}
	got := withEnvDerivations(driver, cfg, "")
	assert.Len(t, got.Tickets, 1)
	assert.Empty(t, driver.Tickets) // original never mutated
}
```

- [ ] **Step 2: Run test — verify it fails**

Run: `go test ./internal/app/ -run TestWithEnvDerivations_CarriesTickets`
Expected: FAIL — `got.Tickets` is empty (not propagated).

- [ ] **Step 3: Propagate in `withEnvDerivations`** (`internal/app/pipeline.go`).

Add `len(cfg.Tickets) == 0` to the early-return guard and set the field on the clone:
```go
	if headingPat == "" && tagPat == "" && cfg.RemoteMetadata == "" && len(cfg.Tickets) == 0 {
		return driver
	}
	clone := *driver
	// ... existing headingPat / tagPat / RemoteMetadata assignments ...
	if len(cfg.Tickets) > 0 {
		clone.Tickets = cfg.Tickets
	}
	return &clone
```
Also update the doc comment to mention Tickets (mirroring the RemoteMetadata bullet).

- [ ] **Step 4: Run test — verify it passes**

Run: `go test ./internal/app/ -run TestWithEnvDerivations_CarriesTickets`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/pipeline.go internal/app/tickets_internal_test.go
git commit -m "feat(app): propagate tickets config onto content drivers

Part of T79." -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5: Templates — render `commit.links` (both changelog + release notes)

**Files:**
- Modify: `internal/generators/gitcliff/cliff.changelog.toml`, `internal/generators/gitcliff/cliff.release-notes.toml`

> This is the whitespace-sensitive task. Tests cover config-*acceptance* only; the *rendered* output is verified by a real-CLI render check (no byte-assertion test), the same method used for the New Contributors section.

- [ ] **Step 1: Add the link rendering to both `print_commit` macros.**

In each file's `print_commit` macro, immediately after the PR-number line:
```
        {%- if commit.remote.pr_number %} in {{ self::pr_link(number=commit.remote.pr_number) }}{%- endif %}
```
append:
```
        {%- for link in commit.links %} ([{{ link.text }}]({{ link.href }})){%- endfor %}
```

- [ ] **Step 2: Verify git-cliff still accepts both embedded configs**

Run: `go test ./internal/generators/gitcliff/ -run TestEmbeddedConfig_RealGitCliff`
Expected: PASS (skips if git-cliff absent). Confirms both templates still parse.

- [ ] **Step 3: Real-CLI render check (manual) — link appears**

```bash
tmp=$(mktemp -d) && cd "$tmp" && git init -q && git config user.email t@t.t && git config user.name t
git commit -q --allow-empty -m "feat(auth): add SSO login PROJ-123"
cat > c.toml <<'TOML'
[changelog]
body = """{% macro print_commit(commit) -%}
- {{ commit.message }}{%- for link in commit.links %} ([{{ link.text }}]({{ link.href }})){%- endfor %}
{% endmacro -%}
{% for commit in commits %}{{ self::print_commit(commit=commit) }}{% endfor %}"""
[git]
conventional_commits = true
commit_parsers = [{ message = ".*", group = "x" }]
link_parsers = [{ pattern = '([A-Z]+-[0-9]+)', href = 'https://acme.atlassian.net/browse/$1' }]
TOML
git-cliff --config c.toml --latest --offline 2>/dev/null
cd - && rm -rf "$tmp"
```
Expected output: `- add SSO login PROJ-123 ([PROJ-123](https://acme.atlassian.net/browse/PROJ-123))`
If the spacing looks wrong against the real embedded template, render the embedded config the way the generator does (`HERAUT_*` env + `--from-context` with an injected `commit.links`) and adjust the `{%- … -%}` until clean — do not commit until the render is correct.

- [ ] **Step 4: Run the gitcliff package**

Run: `go test ./internal/generators/gitcliff/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/generators/gitcliff/cliff.changelog.toml internal/generators/gitcliff/cliff.release-notes.toml
git commit -m "feat(generators/gitcliff): render ticket links in changelog and release notes

Part of T79." -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 6: Schema, sample, fixtures

**Files:**
- Modify: `schema.json`, `docs/heraut.sample.yml`, `internal/config/schema_test.go`
- Create: `testdata/config/valid/tickets.yml`, `testdata/config/invalid/invalid_tickets.yml`

- [ ] **Step 1: Add `tickets` to `schema.json`** — add to the top-level `properties` (after `environments`):

```json
    "tickets": {
      "type": "array",
      "description": "Issue-tracker links. Each entry's regex is matched in commit messages (subject/body/footer) and rendered as a link. git-cliff only.",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["pattern", "url"],
        "properties": {
          "pattern": { "type": "string", "description": "Regex for ticket IDs, e.g. [A-Z]+-[0-9]+. The first capture group (or full match) becomes {ticket}." },
          "url": { "type": "string", "description": "URL template containing {ticket}, e.g. https://jira.example.com/browse/{ticket}." }
        }
      }
    }
```

- [ ] **Step 2: Create fixtures.**

`testdata/config/valid/tickets.yml`:
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json
version: "1"
versioning:
  strategy: semver
changelog:
  generator: git-cliff
tickets:
  - pattern: '[A-Z]+-[0-9]+'
    url: 'https://acme.atlassian.net/browse/{ticket}'
```
`testdata/config/invalid/invalid_tickets.yml` (missing required `url`):
```yaml
version: "1"
versioning:
  strategy: semver
tickets:
  - pattern: '[A-Z]+-[0-9]+'
```

- [ ] **Step 3: Add the invalid-fixture row** in `internal/config/schema_test.go` (the `TestSchema_InvalidFixtures` table):
```go
		{"invalid_tickets.yml", "tickets item missing required url"},
```

- [ ] **Step 4: Run schema tests**

Run: `go test ./internal/config/ -run TestSchema`
Expected: PASS (valid fixture accepted, invalid rejected).

- [ ] **Step 5: Document in `docs/heraut.sample.yml`** — add a `# ── tickets ──` section (top-level) modeled on the existing sections:
```yaml
# ── tickets ───────────────────────────────────────────────────────────────────
#
# Link issue-tracker references found in commit messages (subject, body, or footer)
# in the changelog and release notes. git-cliff only. Each entry's regex is matched;
# {ticket} in the URL is the first capture group (or the whole match if there is no
# group), and the link label is always the matched text.
# tickets:
#   - pattern: '[A-Z]+-[0-9]+'                          # Jira/Linear: PROJ-123 → /browse/PROJ-123
#     url: 'https://acme.atlassian.net/browse/{ticket}'
#   - pattern: 'GH-([0-9]+)'                            # GitHub issues: GH-123 → /issues/123
#     url: 'https://github.com/acme/app/issues/{ticket}'
```

- [ ] **Step 6: Commit**

```bash
git add schema.json docs/heraut.sample.yml internal/config/schema_test.go testdata/config/valid/tickets.yml testdata/config/invalid/invalid_tickets.yml
git commit -m "feat(config): add tickets to schema + sample + fixtures

Part of T79." -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 7: Docs — ADR-0024, spec, roadmap

**Files:**
- Create: `docs/adr/0024-ticket-linking.md`
- Modify: `docs/adr/README.md`, `docs/specs/02-configuration.md`, `docs/tasks/roadmap.md`

- [ ] **Step 1: Write `docs/adr/0024-ticket-linking.md`** following the ADR-0023 format (Status: Accepted; Date; Deciders: bchatard; Context / Decision / Consequences). Record: git-cliff `link_parsers` over `commit_preprocessors` (inline only links the rendered subject — footer/body tickets are linked but discarded; demo-verified), top-level `tickets` list governing both generators, `{ticket}` = first capture group / full match with the label as the full match, append-only, git-cliff-only. Add the index row to `docs/adr/README.md`.

- [ ] **Step 2: Document `tickets` in `docs/specs/02-configuration.md`** — a short subsection: the config shape, the `{ticket}`/label semantics, that matching covers subject/body/footer, and the git-cliff-only constraint.

- [ ] **Step 3: Add `[x]` T79 to `docs/tasks/roadmap.md`** with a Done note: mechanism (link_parsers injection), the `{ticket}`=capture-group decision, append-only, git-cliff-only, files touched, tests.

- [ ] **Step 4: Full suite + lint**

Run: `go test ./...` → expect ALL ok. Then `hk check` (or `mise run lint:check`) → expect clean.

- [ ] **Step 5: Commit**

```bash
git add docs/adr/0024-ticket-linking.md docs/adr/README.md docs/specs/02-configuration.md docs/tasks/roadmap.md
git commit -m "docs: record ADR-0024 ticket linking; document tickets; complete T79

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Self-Review notes (verify during execution)

- **Spec coverage:** config field (T1) · validation incl. git-cliff-only (T2) · link_parsers injection with capture-group rule (T3) · driver propagation (T4) · both templates (T5) · schema/sample/fixtures (T6) · ADR/spec/roadmap (T7). All spec sections mapped.
- **Type consistency:** `Ticket{Pattern, URL}`, `Config.Tickets`, `ContentDriver.Tickets`, `injectLinkParsers`, `ticketsGeneratorSupported`, `validateTickets` used consistently across tasks.
- **Watch item:** go-toml may emit double- vs single-quoted strings; Task 3 Step 1 notes to align the expected substrings to the emitted form without weakening the value assertion.
