# T116: `heraut commit verify` — Built-in Conventional-Commit Checker — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `heraut commit verify`, an in-house conventional-commit grammar checker, replacing heraut's own dependency on `cog verify` for its commit-msg hook, and unifying SemVer bump detection onto the same grammar parser.

**Architecture:** A new pure domain package `internal/conventionalcommit/` owns grammar parsing (header + body + footers). `internal/versioning/semver` is refactored to consume it instead of its own ad hoc regexes. A new optional `commit_lint:` config block carries a type allow-list. `internal/app.VerifyCommit` layers the allow-list policy (and merge/fixup skipping) over the pure parser, per the existing hexagonal layering (`internal/cmd/` never calls domain packages directly). `internal/cmd/commit.go` exposes it as `heraut commit verify`. heraut's own `.config/hk/config.pkl` commit-msg hook switches from `cog verify` to this new command.

**Tech Stack:** Go 1.26, `regexp` (stdlib, package-level compiled patterns only), `spf13/cobra`, `stretchr/testify` (`assert`/`require`).

## Global Constraints

- TDD: write the failing test before the implementation, every task (`testing.md`, `.claude/rules/claude.md`).
- Every `if err != nil` wraps with `%w` when propagating an existing error (`coding.md`); a freshly originated validation error (e.g. `Parse`'s grammar error) does not need `%w` since there is no inner error to preserve.
- Never `os.Exit` below `cmd/heraut/`; commands return errors wrapped via `internal/exitcode.Wrap`.
- Layer rules (`coding.md`): `internal/cmd/` → `internal/app/`, `internal/ui/`, `internal/config/` only. `internal/app/` and `internal/versioning/*` gain `internal/conventionalcommit` as an allowed import in this plan (Task 2).
- No commitlint-style rule catalog (casing, length limits, `signed-off-by`, etc.) — grammar + type allow-list only, per ADR-0027's "Explicitly still out of scope" note. Resist the urge to add more.
- `regexp` patterns are package-level `var`s, compiled once, never per-call (ADR-0027 performance note).
- Never pass `--no-verify`/`--no-gpg-sign` to git; never bypass `hk` hooks. If a hook fails, fix the root cause.
- Conventional-commit subject lines for this work's own commits, per `workflow.md`'s type table (`feat`, `fix`, `refactor`, `chore`, `docs`, ...).
- Reference docs for this work: [ADR-0027](../../docs/adr/0027-builtin-conventional-commit-checker.md), [T116 in the roadmap](../../docs/tasks/roadmap.md).

---

### Task 1: `internal/conventionalcommit` — grammar parser

**Files:**
- Create: `internal/conventionalcommit/conventionalcommit.go`
- Test: `internal/conventionalcommit/conventionalcommit_test.go`

**Interfaces:**
- Produces: `type Footer struct { Token, Value string }`; `type Commit struct { Type, Scope string; Breaking bool; Description, Body string; Footers []Footer }`; `func Parse(message string) (*Commit, error)`; `func IsMergeCommit(message string) bool`; `func IsFixupCommit(message string) bool`. These four names/signatures are what Task 2 (semver) and Task 4 (app) consume.

- [ ] **Step 1: Write the failing test file**

```go
package conventionalcommit_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Valid(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    conventionalcommit.Commit
	}{
		{
			name:    "simple feat",
			message: "feat: add x",
			want:    conventionalcommit.Commit{Type: "feat", Description: "add x"},
		},
		{
			name:    "fix with scope",
			message: "fix(api): handle y",
			want:    conventionalcommit.Commit{Type: "fix", Scope: "api", Description: "handle y"},
		},
		{
			name:    "breaking bang, no scope",
			message: "feat!: breaking",
			want:    conventionalcommit.Commit{Type: "feat", Breaking: true, Description: "breaking"},
		},
		{
			name:    "breaking bang with scope",
			message: "feat(api)!: remove endpoint",
			want:    conventionalcommit.Commit{Type: "feat", Scope: "api", Breaking: true, Description: "remove endpoint"},
		},
		{
			name:    "bang in description, not type prefix, not breaking",
			message: "fix: handle the foo!: token",
			want:    conventionalcommit.Commit{Type: "fix", Description: "handle the foo!: token"},
		},
		{
			name:    "header with trailing newline (raw COMMIT_EDITMSG content)",
			message: "feat: add x\n",
			want:    conventionalcommit.Commit{Type: "feat", Description: "add x"},
		},
		{
			name:    "BREAKING CHANGE footer",
			message: "fix: y\n\nBREAKING CHANGE: boom",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y", Breaking: true,
				Footers: []conventionalcommit.Footer{{Token: "BREAKING CHANGE", Value: "boom"}},
			},
		},
		{
			name:    "BREAKING-CHANGE hyphenated footer",
			message: "fix: y\n\nBREAKING-CHANGE: boom",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y", Breaking: true,
				Footers: []conventionalcommit.Footer{{Token: "BREAKING-CHANGE", Value: "boom"}},
			},
		},
		{
			name:    "BREAKING CHANGE mentioned mid-sentence, not a footer, not breaking",
			message: "fix: y\n\nThis is not a BREAKING CHANGE: just a mention.",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y",
				Body: "This is not a BREAKING CHANGE: just a mention.",
			},
		},
		{
			name:    "BREAKING-CHANGE mentioned mid-sentence, not a footer, not breaking",
			message: "fix: y\n\nAlso recognize the hyphenated BREAKING-CHANGE: footer as a synonym.",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y",
				Body: "Also recognize the hyphenated BREAKING-CHANGE: footer as a synonym.",
			},
		},
		{
			name:    "BREAKING CHANGE starts a wrapped body line, not its paragraph, not breaking",
			message: "fix: y\n\nDiscussing isBreaking's\nBREAKING CHANGE: footer check here.",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y",
				Body: "Discussing isBreaking's\nBREAKING CHANGE: footer check here.",
			},
		},
		{
			name:    "body with no footers",
			message: "docs: update readme\n\nExplains the new flag in detail.",
			want: conventionalcommit.Commit{
				Type: "docs", Description: "update readme",
				Body: "Explains the new flag in detail.",
			},
		},
		{
			name:    "body paragraph then separate footer block",
			message: "fix: y\n\nSome body text.\n\nBREAKING CHANGE: boom",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y", Breaking: true,
				Body:    "Some body text.",
				Footers: []conventionalcommit.Footer{{Token: "BREAKING CHANGE", Value: "boom"}},
			},
		},
		{
			name:    "multiple footers, hyphenated token",
			message: "fix: y\n\nAcked-by: Alice\nBREAKING CHANGE: boom",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y", Breaking: true,
				Footers: []conventionalcommit.Footer{
					{Token: "Acked-by", Value: "Alice"},
					{Token: "BREAKING CHANGE", Value: "boom"},
				},
			},
		},
		{
			name:    "multi-line footer value (continuation line)",
			message: "fix: y\n\nSigned-off-by: Bob\nThis continues the previous footer.",
			want: conventionalcommit.Commit{
				Type: "fix", Description: "y",
				Footers: []conventionalcommit.Footer{
					{Token: "Signed-off-by", Value: "Bob\nThis continues the previous footer."},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := conventionalcommit.Parse(tc.message)
			require.NoError(t, err)
			assert.Equal(t, &tc.want, got)
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{"missing colon separator", "feat add x"},
		{"empty description", "feat:"},
		{"missing type", ": add x"},
		{"merge commit is not conventional grammar", "Merge branch 'main' into feature"},
		{"missing blank line before body", "feat: x\nBody text immediately"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := conventionalcommit.Parse(tc.message)
			require.Error(t, err)
		})
	}
}

func TestIsMergeCommit(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"merge branch", "Merge branch 'main' into feature/x", true},
		{"merge pull request", "Merge pull request #42 from org/feature", true},
		{"merge remote-tracking branch", "Merge remote-tracking branch 'origin/main'", true},
		{"not a merge", "feat: add x", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, conventionalcommit.IsMergeCommit(tc.message))
		})
	}
}

func TestIsFixupCommit(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{"fixup", "fixup! feat: add x", true},
		{"squash", "squash! feat: add x", true},
		{"not fixup", "feat: add x", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, conventionalcommit.IsFixupCommit(tc.message))
		})
	}
}

func BenchmarkParse(b *testing.B) {
	inputs := map[string]string{
		"header_only": "feat: add x",
		"with_body":   "fix(api): y\n\nSome explanatory body text describing the change in more detail.",
		"with_footers": "feat(api)!: z\n\nBody text.\n\n" +
			"Acked-by: Alice\nReviewed-by: Bob\nBREAKING CHANGE: boom",
	}
	for name, in := range inputs {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = conventionalcommit.Parse(in)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/conventionalcommit/... -v`
Expected: FAIL — `no required module provides package github.com/adaouat/heraut/internal/conventionalcommit` (the package doesn't exist yet).

- [ ] **Step 3: Write the implementation**

```go
// Package conventionalcommit parses and inspects commit messages against the
// Conventional Commits 1.0.0 grammar. It validates structure only — it does not enforce a
// type allow-list (that policy decision lives in internal/app.VerifyCommit) and it does
// not replicate the broader commitlint-style rule catalog (casing, length limits,
// signed-off-by, etc.) — see ADR-0027's "Explicitly still out of scope" note.
package conventionalcommit

import (
	"fmt"
	"regexp"
	"strings"
)

// headerPattern matches a conventional-commit header: type, optional (scope), optional
// breaking "!", then ": " and a non-empty description. Anchored and bounded — no
// nested-quantifier patterns — so it stays linear-time on the hot paths documented in
// ADR-0027 (the commit-msg hook and DetermineBump).
var headerPattern = regexp.MustCompile(`^(\w+)(\(([^)]*)\))?(!)?: (.+)$`)

// footerLinePattern matches one footer line: a token — either the literal, case-sensitive
// "BREAKING CHANGE" exception, or a generic hyphenated word-token per the spec — followed
// by ": " or " #", then the value. Reused both to decide whether a trailing paragraph is a
// footer block and to parse each line within it.
var footerLinePattern = regexp.MustCompile(`^(BREAKING CHANGE|[A-Za-z][A-Za-z0-9]*(?:-[A-Za-z0-9]+)*)(: | #)(.*)$`)

// mergeCommitPattern matches git's own merge-commit subject shapes.
var mergeCommitPattern = regexp.MustCompile(`^Merge (branch |pull request |remote-tracking branch )`)

// Footer is one trailer in a commit message's footer block, e.g. "Acked-by: Alice" or
// "BREAKING CHANGE: removes the old flag".
type Footer struct {
	Token string
	Value string
}

// Commit is the structural result of parsing a conventional-commit message.
type Commit struct {
	Type        string
	Scope       string
	Breaking    bool
	Description string
	Body        string
	Footers     []Footer
}

// Parse validates message against the Conventional Commits grammar and returns its
// structural components. It enforces grammar only — callers needing a type allow-list
// apply that policy themselves (see internal/app.VerifyCommit).
func Parse(message string) (*Commit, error) {
	lines := strings.Split(message, "\n")
	header := lines[0]

	m := headerPattern.FindStringSubmatch(header)
	if m == nil {
		return nil, fmt.Errorf(`invalid conventional commit header %q: expected "type(scope)!: description"`, header)
	}

	c := &Commit{
		Type:        m[1],
		Scope:       m[3],
		Breaking:    m[4] == "!",
		Description: m[5],
	}

	if len(lines) > 1 {
		rest := lines[1:]
		if rest[0] != "" {
			return nil, fmt.Errorf("invalid conventional commit: body/footer must be separated from the header by a blank line")
		}
		rest = rest[1:]

		body, footers := parseBodyAndFooters(rest)
		c.Body = body
		c.Footers = footers
		for _, f := range footers {
			if f.Token == "BREAKING CHANGE" || f.Token == "BREAKING-CHANGE" {
				c.Breaking = true
			}
		}
	}

	return c, nil
}

// IsMergeCommit reports whether message is a git-generated merge commit
// ("Merge branch ...", "Merge pull request ...", "Merge remote-tracking branch ...").
func IsMergeCommit(message string) bool {
	return mergeCommitPattern.MatchString(firstLine(message))
}

// IsFixupCommit reports whether message is a git "fixup!"/"squash!" autosquash commit.
func IsFixupCommit(message string) bool {
	line := firstLine(message)
	return strings.HasPrefix(line, "fixup! ") || strings.HasPrefix(line, "squash! ")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// parseBodyAndFooters splits the lines following the header (and its separating blank
// line) into a body string and a footer list. Paragraphs are groups of lines separated by
// blank lines; only the LAST paragraph is a candidate footer block, and only if its first
// line itself looks like a footer token — otherwise the whole remainder is body. This
// mirrors the spec's footer-placement rule and rejects a body paragraph that merely
// mentions footer-shaped text without being structurally a footer.
func parseBodyAndFooters(lines []string) (string, []Footer) {
	paragraphs := splitParagraphs(lines)
	if len(paragraphs) == 0 {
		return "", nil
	}

	last := paragraphs[len(paragraphs)-1]
	if !footerLinePattern.MatchString(last[0]) {
		return strings.Join(joinParagraphs(paragraphs), "\n\n"), nil
	}

	footers := parseFooterBlock(last)
	body := strings.Join(joinParagraphs(paragraphs[:len(paragraphs)-1]), "\n\n")
	return body, footers
}

func splitParagraphs(lines []string) [][]string {
	var paragraphs [][]string
	var current []string
	for _, l := range lines {
		if l == "" {
			if len(current) > 0 {
				paragraphs = append(paragraphs, current)
				current = nil
			}
			continue
		}
		current = append(current, l)
	}
	if len(current) > 0 {
		paragraphs = append(paragraphs, current)
	}
	return paragraphs
}

func joinParagraphs(paragraphs [][]string) []string {
	out := make([]string, len(paragraphs))
	for i, p := range paragraphs {
		out[i] = strings.Join(p, "\n")
	}
	return out
}

func parseFooterBlock(lines []string) []Footer {
	var footers []Footer
	for _, line := range lines {
		if m := footerLinePattern.FindStringSubmatch(line); m != nil {
			footers = append(footers, Footer{Token: m[1], Value: m[3]})
			continue
		}
		if len(footers) > 0 {
			footers[len(footers)-1].Value += "\n" + line
		}
	}
	return footers
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/conventionalcommit/... -v`
Expected: PASS — all `TestParse_Valid`, `TestParse_Invalid`, `TestIsMergeCommit`, `TestIsFixupCommit` subtests green.

- [ ] **Step 5: Run the benchmark once as a sanity check**

Run: `go test ./internal/conventionalcommit/... -bench=. -run=^$ -benchtime=10x`
Expected: three `BenchmarkParse/header_only`, `BenchmarkParse/with_body`, `BenchmarkParse/with_footers` lines print `ns/op` with no failures. (This is a smoke check that the benchmark compiles and runs — no specific threshold is gated yet.)

- [ ] **Step 6: Lint and commit**

```bash
hk fix
git add internal/conventionalcommit/conventionalcommit.go internal/conventionalcommit/conventionalcommit_test.go
git commit -m "$(cat <<'EOF'
feat(conventionalcommit): add Parse/IsMergeCommit/IsFixupCommit

New pure grammar package per ADR-0027: structural parsing of conventional-commit
headers, bodies, and footers (including BREAKING CHANGE/BREAKING-CHANGE footer
detection), with no type allow-list and no commitlint-style rule catalog.
EOF
)"
```

---

### Task 2: Refactor `bump.go` onto `conventionalcommit.Parse`

**Files:**
- Modify: `internal/versioning/semver/bump.go`
- Modify: `.claude/rules/coding.md` (layer table)
- Test: `internal/versioning/semver/resolver_test.go` (existing `TestDetermineBump` — unchanged, used as the regression gate)

**Interfaces:**
- Consumes: `conventionalcommit.Parse(message string) (*conventionalcommit.Commit, error)` from Task 1.
- Produces: `DetermineBump`'s public signature is unchanged (`func DetermineBump(commits []string) versioning.BumpType`) — no other task depends on its internals.

- [ ] **Step 1: Confirm the existing test still describes the desired behavior**

`internal/versioning/semver/resolver_test.go`'s `TestDetermineBump` (already in the repo) is the regression gate for this refactor — it is not modified. Run it now, before touching `bump.go`, to confirm it's currently green:

Run: `go test ./internal/versioning/semver/... -run TestDetermineBump -v`
Expected: PASS (baseline, using the current hand-rolled regex implementation).

- [ ] **Step 2: Replace `bump.go`'s implementation**

Replace the full contents of `internal/versioning/semver/bump.go` with:

```go
package semver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/adaouat/heraut/internal/versioning"
)

// DetermineBump scans conventional commit subjects and returns the highest applicable bump.
func DetermineBump(commits []string) versioning.BumpType {
	bump := versioning.BumpPatch // fallback
	for _, c := range commits {
		parsed, err := conventionalcommit.Parse(c)
		if err != nil {
			continue // not a conventional commit — ignore for bump purposes, same as before
		}
		if parsed.Breaking {
			return versioning.BumpMajor
		}
		if parsed.Type == "feat" && bump < versioning.BumpMinor {
			bump = versioning.BumpMinor
		}
	}
	return bump
}

// BumpVersion increments the appropriate SemVer component.
// current must be a bare version string without prefix (e.g. "1.2.3").
func BumpVersion(current string, bump versioning.BumpType) (string, error) {
	parts := strings.SplitN(current, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver %q: expected MAJOR.MINOR.PATCH", current)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major in %q: %w", current, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor in %q: %w", current, err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch in %q: %w", current, err)
	}

	switch bump {
	case versioning.BumpMajor:
		major++
		minor = 0
		patch = 0
	case versioning.BumpMinor:
		minor++
		patch = 0
	default:
		patch++
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

// IsBareVersion reports whether s is a bare MAJOR.MINOR.PATCH version with no
// pre-release or build metadata (e.g. "1.2.3", not "1.2.3-rc.1"). Used by the
// resolver, and by internal/versioning/perenv, to skip git tags that don't
// conform when locating the most recent release tag.
func IsBareVersion(s string) bool {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}
```

This deletes `breakingPrefixPattern`, `isBreaking`, `isFeat`, and `firstLine` — they have no other callers in the package (confirmed: `resolver.go` only calls `DetermineBump`).

- [ ] **Step 3: Run the regression gate**

Run: `go test ./internal/versioning/semver/... -v`
Expected: PASS — every existing test in the package, especially `TestDetermineBump`'s full table (feat/fix/breaking-bang/scoped-breaking/chore-fallback/multi-commit/BREAKING CHANGE footer/BREAKING-CHANGE footer/bang-in-description/mid-sentence mentions/wrapped-line false-positive), stays green with zero test edits — proving the refactor is behavior-preserving.

- [ ] **Step 4: Update the layer table**

In `.claude/rules/coding.md`, find:

```
| `internal/app/`      | `internal/{port,config,pipeline,versioning,generators,platforms,adapter,ui}/`      |
| `internal/pipeline/` | `internal/{port,config,versioning,ui}/`                                            |
| `internal/generators/*`, `internal/platforms/*` | `internal/{port,config}/`                              |
| `internal/versioning/*` | `internal/{port,config,versioning}/`                                            |
| `internal/config/`   | nothing from heraut (it is at the bottom)                                          |
| `internal/port/`     | nothing from heraut (it is the contract)                                           |
```

Replace with:

```
| `internal/app/`      | `internal/{port,config,pipeline,versioning,generators,platforms,adapter,ui,conventionalcommit}/` |
| `internal/pipeline/` | `internal/{port,config,versioning,ui}/`                                            |
| `internal/generators/*`, `internal/platforms/*` | `internal/{port,config}/`                              |
| `internal/versioning/*` | `internal/{port,config,versioning,conventionalcommit}/`                         |
| `internal/config/`   | nothing from heraut (it is at the bottom)                                          |
| `internal/port/`     | nothing from heraut (it is the contract)                                           |
| `internal/conventionalcommit/` | nothing from heraut (pure, like port/config)                             |
```

- [ ] **Step 5: Lint, build, commit**

```bash
go build ./...
hk fix
git add internal/versioning/semver/bump.go .claude/rules/coding.md
git commit -m "$(cat <<'EOF'
refactor(versioning/semver): use conventionalcommit.Parse in DetermineBump

Removes the second, divergent hand-rolled conventional-commit parser
(isBreaking/isFeat/breakingPrefixPattern) in favor of the shared grammar
package from T116, per ADR-0027. Behavior-preserving: TestDetermineBump's
existing table is unchanged and stays green.
EOF
)"
```

---

### Task 3: `commit_lint` config block

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/validator.go`
- Test: `internal/config/validator_test.go`
- Modify: `schema.json`
- Modify: `docs/heraut.sample.yml`
- Modify: `docs/specs/02-configuration.md`

**Interfaces:**
- Produces: `type CommitLint struct { Types []string }`; `Config.CommitLint *CommitLint`. Task 4 (`internal/app.VerifyCommit`) reads `cfg.CommitLint.Types`.

- [ ] **Step 1: Write the failing validator test**

`internal/config/validator_test.go` is package `config_test` (external) and already has two
helpers used by every test in the file: `mustLoad(t, yamlString) *config.Config` (parses
inline YAML, panics-via-`require` on parse error) and `findErr(errs, wantPath) *config.ValidationError`
(exact-match lookup by `Path`). Follow that exact convention — do not construct `config.Config`
struct literals directly, and do not duplicate the `assert`/`require`/`config` imports already
in the file's header. Append:

```go
func TestValidate_CommitLintValid(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: [feat, fix, docs]
`)
	assert.Empty(t, config.Validate(cfg))
}

func TestValidate_CommitLintEmptyTypesList(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: []
`)
	e := findErr(config.Validate(cfg), "commit_lint.types")
	require.NotNil(t, e)
}

func TestValidate_CommitLintEmptyTypeEntry(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: ["feat", ""]
`)
	e := findErr(config.Validate(cfg), "commit_lint.types[1]")
	require.NotNil(t, e)
}

func TestValidate_CommitLintInvalidTypeName(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: ["feat", "not a type"]
`)
	e := findErr(config.Validate(cfg), "commit_lint.types[1]")
	require.NotNil(t, e)
}

func TestValidate_CommitLintDuplicateType(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: [feat, feat]
`)
	e := findErr(config.Validate(cfg), "commit_lint.types[1]")
	require.NotNil(t, e)
	assert.Contains(t, e.Message, "duplicate")
}

func TestValidate_CommitLintAbsent_NoError(t *testing.T) {
	cfg := mustLoad(t, `
version: "1"
versioning:
  strategy: semver
`)
	assert.Empty(t, config.Validate(cfg))
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/... -run TestValidate_CommitLint -v`
Expected: FAIL — compile error, `Config` has no field `CommitLint` and `CommitLint` type doesn't exist yet.

- [ ] **Step 3: Add the `CommitLint` type and config field**

In `internal/config/config.go`, after the `Ticket` struct (right after its closing `}`), insert:

```go

// CommitLint configures heraut commit verify's grammar/type-allow-list checking
// (ADR-0027). Optional — when nil, the default type list applies.
type CommitLint struct {
	// Types restricts which conventional-commit type words heraut commit verify accepts.
	// Replaces (does not extend) the default list when set: feat, fix, docs, chore,
	// refactor, test, style, perf, ci, build.
	Types []string `yaml:"types,omitempty"`
}
```

Then add the field to `Config`, after `Tickets`:

```go
	// Tickets configures issue-tracker links: each entry's regex is matched in commit
	// messages (subject/body/footer) and rendered as a link in the changelog and release
	// notes. git-cliff only (ADR-0024).
	Tickets []Ticket `yaml:"tickets,omitempty"`
	// CommitLint configures heraut commit verify's type allow-list. git-cliff/generator-
	// agnostic — unlike Tickets, this has nothing to do with changelog generation. ADR-0027.
	CommitLint *CommitLint `yaml:"commit_lint,omitempty"`
}
```

(That final `}` closes `Config` — i.e. `CommitLint` becomes the new last field.)

- [ ] **Step 4: Add the validator**

In `internal/config/validator.go`, add a package-level pattern next to the existing `var (...)` block at the top:

```go
	commitTypePattern = regexp.MustCompile(`^\w+$`)
```

(Insert it as a new line inside the existing `var ( ... )` block, alongside `validStrategies`, `validGenerators`, etc.)

Wire it into `Validate`:

```go
func Validate(cfg *Config) ValidationErrors {
	if cfg == nil {
		return nil
	}
	var errs ValidationErrors
	errs = append(errs, validateRequired(cfg)...)
	errs = append(errs, validateEnums(cfg)...)
	errs = append(errs, validateStrategySpecific(cfg)...)
	errs = append(errs, validateEnvContradictions(cfg.Environments)...)
	errs = append(errs, validateTickets(cfg)...)
	errs = append(errs, validateCommitLint(cfg)...)
	return errs
}
```

Add the new function (place it after `validateTickets`):

```go
// validateCommitLint validates the optional commit_lint.types override: when present, the
// list must be non-empty (an empty list would silently allow zero types — anyone wanting
// "all default types" omits commit_lint entirely instead), each entry must be a non-empty
// single-word type name, and entries must be unique.
func validateCommitLint(cfg *Config) []ValidationError {
	if cfg.CommitLint == nil {
		return nil
	}
	var errs []ValidationError
	if len(cfg.CommitLint.Types) == 0 {
		return []ValidationError{{
			Path:    "commit_lint.types",
			Message: "must not be empty when commit_lint is set",
			Hint:    "list at least one allowed type, or remove commit_lint to use the default list",
		}}
	}
	seen := make(map[string]int)
	for i, t := range cfg.CommitLint.Types {
		path := fmt.Sprintf("commit_lint.types[%d]", i)
		if t == "" {
			errs = append(errs, ValidationError{Path: path, Message: "must not be empty"})
			continue
		}
		if !commitTypePattern.MatchString(t) {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("%q is not a valid type name", t),
				Hint:    "type names must be a single word (letters/digits/underscore), e.g. feat, fix, docs",
			})
			continue
		}
		if first, ok := seen[t]; ok {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("duplicate type %q (already listed at types[%d])", t, first),
			})
			continue
		}
		seen[t] = i
	}
	return errs
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/config/... -v`
Expected: PASS — the six new `TestValidate_CommitLint*` tests, and every pre-existing test in the package (unaffected).

- [ ] **Step 6: Update `schema.json`**

In `schema.json`, the top-level `"properties"` object currently ends with the `tickets` property (closing at the line with just `    }` before the `"definitions"` key). Change:

```json
    "tickets": {
      "type": "array",
      "description": "Issue-tracker links. Each entry's regex is matched in commit messages (subject/body/footer) and rendered as a link in the changelog and release notes. git-cliff only.",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": [
          "pattern",
          "url"
        ],
        "properties": {
          "pattern": {
            "type": "string",
            "description": "Regex matching ticket IDs, e.g. [A-Z]+-[0-9]+. The first capture group (or the full match if there is no group) becomes {ticket}; the link label is always the full match."
          },
          "url": {
            "type": "string",
            "description": "URL template containing {ticket}, e.g. https://jira.example.com/browse/{ticket}."
          }
        }
      }
    }
  },
```

to:

```json
    "tickets": {
      "type": "array",
      "description": "Issue-tracker links. Each entry's regex is matched in commit messages (subject/body/footer) and rendered as a link in the changelog and release notes. git-cliff only.",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": [
          "pattern",
          "url"
        ],
        "properties": {
          "pattern": {
            "type": "string",
            "description": "Regex matching ticket IDs, e.g. [A-Z]+-[0-9]+. The first capture group (or the full match if there is no group) becomes {ticket}; the link label is always the full match."
          },
          "url": {
            "type": "string",
            "description": "URL template containing {ticket}, e.g. https://jira.example.com/browse/{ticket}."
          }
        }
      }
    },
    "commit_lint": {
      "$ref": "#/definitions/CommitLint"
    }
  },
```

Then add a new entry to `"definitions"` (next to `"Versioning"` — exact placement among definitions doesn't matter, JSON objects are unordered):

```json
    "CommitLint": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "types": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "minItems": 1,
          "description": "Restricts which conventional-commit type words 'heraut commit verify' accepts. Replaces (does not extend) the default list when set: feat, fix, docs, chore, refactor, test, style, perf, ci, build."
        }
      },
      "description": "Configures 'heraut commit verify''s type allow-list (ADR-0027). Optional — omit entirely to use the default list."
    },
```

- [ ] **Step 7: Update `docs/heraut.sample.yml`**

Find the `tickets` comment block (it ends right before `# ── changelog ──...`). Insert a new block immediately after it:

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

# ── commit_lint ───────────────────────────────────────────────────────────────
#
# Restricts which conventional-commit type words `heraut commit verify` accepts.
# Optional — omit entirely to use the default list shown below. When set, this list
# replaces (does not extend) the default.
# commit_lint:
#   types: [feat, fix, docs, chore, refactor, test, style, perf, ci, build]

# ── changelog ─────────────────────────────────────────────────────────────────
```

(Only the blank line and the new `commit_lint` block are additions; the surrounding `tickets` and `changelog` blocks are unchanged context.)

- [ ] **Step 8: Update `docs/specs/02-configuration.md`**

Immediately after the `## tickets` section (which ends right before `## Content generators`), insert a new section:

```markdown
## `commit_lint`

A top-level, optional block configuring `heraut commit verify`'s type allow-list
(ADR-0027). Works with zero config — the default type list is the 10 types in
[`workflow.md`](../../.claude/rules/workflow.md)'s commit-type table:
`feat, fix, docs, chore, refactor, test, style, perf, ci, build`.

```yaml
commit_lint:
  types: [feat, fix, docs, chore, refactor, test, style, perf, ci, build]
```

| Field   | Meaning                                                                          |
|---------|-----------------------------------------------------------------------------------|
| `types` | The allowed conventional-commit type words. **Replaces**, does not extend, the default list. Must be non-empty and contain no duplicates. |

Not generator-specific (unlike `tickets`) — `commit_lint` has nothing to do with
changelog generation; it governs `heraut commit verify` only. Merge commits and
`fixup!`/`squash!` commits are always skipped, unconditionally, regardless of this
config.
```

- [ ] **Step 9: Validate the schema is still well-formed JSON, lint, commit**

Run: `python3 -m json.tool schema.json > /dev/null && echo "valid JSON"`
Expected: prints `valid JSON` (catches any stray comma/bracket mistake from the manual edit).

```bash
hk fix
git add internal/config/config.go internal/config/validator.go internal/config/validator_test.go schema.json docs/heraut.sample.yml docs/specs/02-configuration.md
git commit -m "$(cat <<'EOF'
feat(config): add optional commit_lint.types override

Adds the commit_lint block from ADR-0027/T116: an optional type allow-list for
heraut commit verify, with schema/sample/spec doc updates. No config = the
default 10-type list from workflow.md applies.
EOF
)"
```

---

### Task 4: `internal/app.VerifyCommit`

**Files:**
- Create: `internal/app/commit.go`
- Test: `internal/app/commit_test.go`

**Interfaces:**
- Consumes: `conventionalcommit.Parse`, `conventionalcommit.IsMergeCommit`, `conventionalcommit.IsFixupCommit` (Task 1); `config.Config.CommitLint` (Task 3).
- Produces: `var DefaultCommitTypes []string`; `func VerifyCommit(cfg *config.Config, message string) error`. Task 5 (`internal/cmd/commit.go`) calls `app.VerifyCommit`.

- [ ] **Step 1: Write the failing test**

```go
package app_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestVerifyCommit_DefaultTypes_Valid(t *testing.T) {
	err := app.VerifyCommit(nil, "feat: add x")
	assert.NoError(t, err)
}

func TestVerifyCommit_DefaultTypes_RejectsUnknownType(t *testing.T) {
	err := app.VerifyCommit(nil, "wip: not a real type")
	assert.Error(t, err)
}

func TestVerifyCommit_DefaultTypes_AllTenAccepted(t *testing.T) {
	for _, typ := range app.DefaultCommitTypes {
		err := app.VerifyCommit(nil, typ+": something")
		assert.NoError(t, err, "type %q should be accepted by default", typ)
	}
}

func TestVerifyCommit_ConfiguredTypes_OverridesDefault(t *testing.T) {
	cfg := &config.Config{CommitLint: &config.CommitLint{Types: []string{"feat", "fix"}}}

	assert.NoError(t, app.VerifyCommit(cfg, "feat: add x"))
	assert.Error(t, app.VerifyCommit(cfg, "docs: update readme")) // not in the configured list, even though it's a default type
}

func TestVerifyCommit_InvalidGrammar_Errors(t *testing.T) {
	err := app.VerifyCommit(nil, "not a conventional commit at all")
	assert.Error(t, err)
}

func TestVerifyCommit_MergeCommit_Skipped(t *testing.T) {
	cfg := &config.Config{CommitLint: &config.CommitLint{Types: []string{"feat"}}} // strict — would reject almost anything
	err := app.VerifyCommit(cfg, "Merge branch 'main' into feature/x")
	assert.NoError(t, err)
}

func TestVerifyCommit_FixupCommit_Skipped(t *testing.T) {
	cfg := &config.Config{CommitLint: &config.CommitLint{Types: []string{"feat"}}}
	err := app.VerifyCommit(cfg, "fixup! docs: typo")
	assert.NoError(t, err)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/app/... -run TestVerifyCommit -v`
Expected: FAIL — compile error, `app.VerifyCommit` and `app.DefaultCommitTypes` don't exist yet.

- [ ] **Step 3: Write the implementation**

```go
package app

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
)

// DefaultCommitTypes is the type allow-list VerifyCommit applies when no
// commit_lint.types override is configured — the 10 types documented in
// workflow.md's commit-type table.
var DefaultCommitTypes = []string{
	"feat", "fix", "docs", "chore", "refactor", "test", "style", "perf", "ci", "build",
}

// VerifyCommit validates message against the conventional-commit grammar and the
// configured (or default) type allow-list. Merge and fixup commits are always skipped,
// unconditionally. cfg may be nil (no .heraut.yml present) — the default type list applies.
func VerifyCommit(cfg *config.Config, message string) error {
	if conventionalcommit.IsMergeCommit(message) || conventionalcommit.IsFixupCommit(message) {
		return nil
	}

	c, err := conventionalcommit.Parse(message)
	if err != nil {
		return err
	}

	types := DefaultCommitTypes
	if cfg != nil && cfg.CommitLint != nil && len(cfg.CommitLint.Types) > 0 {
		types = cfg.CommitLint.Types
	}
	for _, t := range types {
		if c.Type == t {
			return nil
		}
	}
	return fmt.Errorf("commit type %q is not allowed (allowed: %s)", c.Type, strings.Join(types, ", "))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/... -run TestVerifyCommit -v`
Expected: PASS — all seven tests green.

- [ ] **Step 5: Run the full app package suite (regression check), lint, commit**

Run: `go test ./internal/app/... -v`
Expected: PASS — no other test in the package is affected by this addition.

```bash
hk fix
git add internal/app/commit.go internal/app/commit_test.go
git commit -m "$(cat <<'EOF'
feat(app): add VerifyCommit policy layer over conventionalcommit

Layers the type allow-list (default or commit_lint.types-configured) and
merge/fixup skipping on top of the pure conventionalcommit.Parse grammar,
per ADR-0027/T116. internal/cmd will call this, never the domain package
directly.
EOF
)"
```

---

### Task 5: `heraut commit verify` command

**Files:**
- Create: `internal/cmd/commit.go`
- Test: `internal/cmd/commit_test.go`
- Modify: `internal/cmd/root.go`
- Modify: `docs/specs/03-commands.md`

**Interfaces:**
- Consumes: `app.VerifyCommit` (Task 4); existing `config.ResolvePath`, `config.Load`, `config.Validate`, `exitcode.Wrap`, `exitcode.Usage`, `exitcode.Config`, and the existing `printConfigErrors` helper already defined in `internal/cmd/check.go` (same package — reusable as-is, no changes needed to it).
- Produces: `func NewCommitCmd() *cobra.Command`, registered on the root command.

- [ ] **Step 1: Write the failing test**

```go
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeRootWithStdin is like executeRoot (defined in version_test.go, same package)
// but also wires stdin, for the --file - case.
func executeRootWithStdin(stdin string, args ...string) (string, error) {
	root := cmd.NewRootCmd("dev")
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(args)
	_, err := root.ExecuteC()
	return buf.String(), err
}

func TestCommitCmd_Exists(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var commitCmd, verifyCmd bool
	for _, c := range root.Commands() {
		if c.Use == "commit" {
			commitCmd = true
			for _, sc := range c.Commands() {
				if strings.HasPrefix(sc.Use, "verify") {
					verifyCmd = true
				}
			}
		}
	}
	assert.True(t, commitCmd, "commit command missing")
	assert.True(t, verifyCmd, "commit verify missing")
}

func TestCommitVerify_PositionalArg_Valid_NoConfig(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml") // deliberately does not exist
	_, err := executeRoot("commit", "verify", "feat: add x", "--config", missingCfg)
	require.NoError(t, err)
}

func TestCommitVerify_PositionalArg_InvalidGrammar(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	_, err := executeRoot("commit", "verify", "not conventional at all", "--config", missingCfg)
	require.Error(t, err)
}

func TestCommitVerify_ConfiguredTypes_RejectsOutOfList(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: [feat, fix]
`)
	_, err := executeRoot("commit", "verify", "docs: update readme", "--config", cfgPath)
	require.Error(t, err)
}

func TestCommitVerify_FileFlag_Valid(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	msgPath := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: add x\n"), 0o644))

	_, err := executeRoot("commit", "verify", "--file", msgPath, "--config", missingCfg)
	require.NoError(t, err)
}

func TestCommitVerify_FileFlagStdin_Valid(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	_, err := executeRootWithStdin("feat: add x\n", "commit", "verify", "--file", "-", "--config", missingCfg)
	require.NoError(t, err)
}

func TestCommitVerify_BothArgAndFile_Errors(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	msgPath := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	require.NoError(t, os.WriteFile(msgPath, []byte("feat: add x\n"), 0o644))

	_, err := executeRoot("commit", "verify", "feat: add x", "--file", msgPath, "--config", missingCfg)
	require.Error(t, err)
}

func TestCommitVerify_NoInput_Errors(t *testing.T) {
	missingCfg := filepath.Join(t.TempDir(), ".heraut.yml")
	_, err := executeRoot("commit", "verify", "--config", missingCfg)
	require.Error(t, err)
}

func TestCommitVerify_MergeCommit_SkippedEvenWithStrictConfig(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: [feat]
`)
	_, err := executeRoot("commit", "verify", "Merge branch 'main' into feature/x", "--config", cfgPath)
	require.NoError(t, err)
}

func TestCommitVerify_MalformedConfig_IsConfigError(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
commit_lint:
  types: []
`)
	_, err := executeRoot("commit", "verify", "feat: add x", "--config", cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error(s) in config")
}
```

(`executeRoot` and `writeConfig` are already defined in `internal/cmd/version_test.go`, same `cmd_test` package — reused here, not redefined.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cmd/... -run TestCommitCmd -v` and `go test ./internal/cmd/... -run TestCommitVerify -v`
Expected: FAIL — `cmd.NewCommitCmd` doesn't exist, "commit" command not registered.

- [ ] **Step 3: Write the implementation**

```go
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/exitcode"
)

// NewCommitCmd constructs the `heraut commit` parent command and its subcommands.
func NewCommitCmd() *cobra.Command {
	commitCmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit message tooling",
	}
	commitCmd.AddCommand(newCommitVerifyCmd())
	return commitCmd
}

func newCommitVerifyCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "verify [message]",
		Short: "Validate a commit message against the conventional-commit grammar",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			message, err := readCommitMessage(cmd, args, file)
			if err != nil {
				return exitcode.Wrap(exitcode.Usage, err)
			}

			cfgPath, _ := cmd.Flags().GetString("config")
			path := config.ResolvePath(cfgPath)
			cfg, err := config.Load(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("loading config: %w", err))
				}
				cfg = nil
			}
			if cfg != nil {
				if errs := config.Validate(cfg); len(errs) > 0 {
					printConfigErrors(errs, cmd.OutOrStdout())
					return exitcode.Wrap(exitcode.Config, fmt.Errorf("%d error(s) in config", len(errs)))
				}
			}

			if err := app.VerifyCommit(cfg, message); err != nil {
				return exitcode.Wrap(exitcode.Usage, err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "read the commit message from a file (use - for stdin)")
	return cmd
}

func readCommitMessage(cmd *cobra.Command, args []string, file string) (string, error) {
	if file != "" && len(args) == 1 {
		return "", errors.New("provide a commit message as an argument or via --file, not both")
	}
	if file != "" {
		if file == "-" {
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return "", fmt.Errorf("reading commit message from stdin: %w", err)
			}
			return string(data), nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading commit message from %s: %w", file, err)
		}
		return string(data), nil
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return "", errors.New("provide a commit message as an argument or via --file")
}
```

In `internal/cmd/root.go`, add the new command to the root (next to the other `root.AddCommand(...)` calls):

```go
	root.AddCommand(NewReleaseCmd())
	root.AddCommand(NewChangelogCmd())
	root.AddCommand(NewCheckCmd())
	root.AddCommand(NewCliffCmd())
	root.AddCommand(NewVersionCmd())
	root.AddCommand(NewCommitCmd())
	root.AddCommand(NewInitCmd(version))
```

(Just the `NewCommitCmd()` line is new; the others are existing context to locate the insertion point.)

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cmd/... -run "TestCommitCmd|TestCommitVerify" -v`
Expected: PASS — all eleven tests green.

- [ ] **Step 5: Run the full cmd package suite (regression check)**

Run: `go test ./internal/cmd/... -v`
Expected: PASS — no other command's tests are affected by the new registration.

- [ ] **Step 6: Document the command**

In `docs/specs/03-commands.md`, insert a new section right after `## heraut version sprint bump` (which ends right before `## heraut check`):

```markdown
## `heraut commit verify`

Validate a single commit message against the Conventional Commits grammar
(`type(scope)!: description`, with structural body/footer parsing — see
[ADR-0027](../adr/0027-builtin-conventional-commit-checker.md)).

```
heraut commit verify [message] [--file <path>]
```

| Flag     | Description                                                                          |
|----------|---------------------------------------------------------------------------------------|
| `--file` | Read the commit message from a file instead of the positional argument. `--file -` reads from stdin. |

Exactly one of a positional `message` argument or `--file` must be given — both or
neither is a usage error.

Validates grammar, then checks the parsed type against `commit_lint.types` (or the
default 10-type list — see [Spec 02 § `commit_lint`](02-configuration.md#commit_lint))
— unless the message is a git-generated merge commit or a `fixup!`/`squash!` commit,
which are always skipped. An invalid message exits with the Usage code (1); an invalid
`.heraut.yml` (if one is present) exits with the Config code (2) — same semantic
validation `heraut check config` runs.

heraut's own `.config/hk/config.pkl` `commit-msg` hook runs this command via
`go run ./cmd/heraut commit verify --file {{ commit_msg_file }}` instead of `cog verify`.
```

- [ ] **Step 7: Lint, commit**

```bash
hk fix
git add internal/cmd/commit.go internal/cmd/commit_test.go internal/cmd/root.go docs/specs/03-commands.md
git commit -m "$(cat <<'EOF'
feat(cmd): add heraut commit verify

Wires app.VerifyCommit up as a new heraut commit verify subcommand, mirroring
cog verify's three input modes (positional, --file <path>, --file - for
stdin). Per ADR-0027/T116.
EOF
)"
```

---

### Task 6: Dogfooding — drop heraut's own dependency on `cog` for linting

**Files:**
- Modify: `.config/hk/config.pkl`
- Delete: `.config/cocogitto/config.toml`
- Modify: `.config/mise/config.toml`

**Interfaces:** None (dev-tooling only; no Go code).

- [ ] **Step 1: Switch the commit-msg hook**

In `.config/hk/config.pkl`, change:

```pkl
  ["commit-msg"] {
    fix = false
    steps {
      // your commit message should follow conventional commit
      ["cocogitto"] {
        check = "cog --config .config/cocogitto/config.toml verify --file {{ commit_msg_file }}"
      }
    }
  }
```

to:

```pkl
  ["commit-msg"] {
    fix = false
    steps {
      // your commit message should follow conventional commit
      ["heraut-commit-lint"] {
        check = "go run ./cmd/heraut commit verify --file {{ commit_msg_file }}"
      }
    }
  }
```

- [ ] **Step 2: Delete the now-unused cocogitto dev-tooling config**

`git rm` both deletes the file from disk and stages the removal in one step, so the
commit in Step 5 only needs `git add` for the other modified files:

```bash
git rm .config/cocogitto/config.toml
```

(Leave the `.config/cocogitto/` directory itself only if anything else lives in it — check first: `ls .config/cocogitto/`. If now empty, remove the directory too: `rmdir .config/cocogitto/`.)

- [ ] **Step 3: Remove the now-unused `cog` shell alias**

In `.config/mise/config.toml`, remove this line from `[shell_alias]`:

```toml
cog = "cog --config {{ config_root }}/.config/cocogitto/config.toml"
```

(Leave `cocogitto = "7.0.0"` in `[tools]` untouched — it's still required by the `generator: cocogitto` feature's own contract/smoke tests until T117.)

- [ ] **Step 4: Manually verify the new hook end-to-end**

This step has no automated test — it's a one-time manual check that the hook actually fires correctly with the new command, since `hk` hook wiring isn't something `go test` exercises.

Run:
```bash
echo "this is not conventional" > /tmp/test-msg
go run ./cmd/heraut commit verify --file /tmp/test-msg; echo "exit: $?"
```
Expected: a grammar error printed, `exit: 1`.

Run:
```bash
echo "test: verify the new hk hook wiring" > /tmp/test-msg
go run ./cmd/heraut commit verify --file /tmp/test-msg; echo "exit: $?"
```
Expected: no error printed, `exit: 0`.

- [ ] **Step 5: Commit**

This commit's own message must pass the *new* hook (it lints itself on creation — a fitting first real-world exercise of the new command):

```bash
git add .config/hk/config.pkl .config/mise/config.toml
git commit -m "$(cat <<'EOF'
chore(hk): switch commit-msg hook from cog to heraut commit verify

heraut no longer depends on cog/cocogitto being installed or configured for
its own commit-msg linting (ADR-0027/T116). cocogitto itself stays installed
via mise — it's still required by the generator: cocogitto feature's own
tests until T117.
EOF
)"
```

---

### Task 7: Close out T116 in the roadmap

**Files:**
- Modify: `docs/tasks/roadmap.md`

- [ ] **Step 1: Run the full test suite and linters one more time**

```bash
go build ./...
go test ./...
hk check
```
Expected: everything green. This is the final gate before marking the task done.

- [ ] **Step 2: Flip the checkbox and add the closing note**

In `docs/tasks/roadmap.md`, change `#### \`[ ]\` T116:` to `#### \`[x]\` T116:`, and append a closing paragraph after the existing `**Scope:** M. **Dependencies:** none.` line (mirroring the style of T114/T115's closing notes — actual decisions made, any deviations from the original plan), e.g.:

```markdown
Implemented across seven commits (conventionalcommit package → semver refactor →
commit_lint config → app.VerifyCommit → cmd wiring → hk/mise dogfooding cleanup →
this note), per ADR-0027. `DetermineBump`'s existing test table required zero
changes, confirming the refactor was behavior-preserving. [Note any deviations
encountered during implementation here, or state "implemented exactly as planned"
if none.]
```

(Fill in the bracketed sentence honestly based on what actually happened during Tasks 1-6 — if a step deviated from this plan, say so here, per `workflow.md`'s "the note is what makes the roadmap a living document.")

- [ ] **Step 3: Commit**

```bash
git add docs/tasks/roadmap.md
git commit -m "$(cat <<'EOF'
docs(roadmap): mark T116 complete
EOF
)"
```
