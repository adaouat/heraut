# Commit Wizard (`heraut commit create`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `heraut commit create` — an interactive, TTY-only wizard that builds a Conventional Commits message and runs `git commit`.

**Architecture:** A new pure serializer `conventionalcommit.Format()` plus a new `internal/commitwizard` package (the huh form, pure assembly, git helpers, and an orchestrator). The command reuses `app.AllowedCommitTypes`/`app.VerifyCommit` so wizard output is guaranteed to pass `heraut commit verify`. Git messages are passed via a temp file (`git commit -F`), because `port.Runner` has no stdin.

**Tech Stack:** Go, cobra, `charm.land/huh/v2`, `github.com/adaouat/forge/exec` (Runner) + `forge/exec/exectest` (MockRunner), `github.com/adaouat/forge/ui` (TTY detection), yaml.v3 / JSON Schema.

## Global Constraints

Copied verbatim from the design spec (`docs/superpowers/specs/2026-06-25-commit-wizard-design.md`) and the repo rules. Every task implicitly includes these:

- **TDD is mandatory.** Write the failing test first, watch it fail, then implement. Red → green → refactor.
- **Conventional Commits** for every commit. Subject line ≤ 72 chars. Scope matches the affected package/subcommand.
- **Never bypass hooks.** Never pass `--no-verify`, `--no-gpg-sign`. The repo's `commit-msg` (cog) and `prepare-commit-msg` (typos) hooks run on every commit — wizard-built and your own commits must pass them.
- **Lint via hk only.** `hk fix` / `hk fix -S golangci-lint` — never invoke `gofmt`/`yamlfmt` directly.
- **Layering (coding.md):** `cmd → app/ui/config`; `app` may import `port/config/pipeline/versioning/generators/platforms/adapter/ui/conventionalcommit`; `conventionalcommit` imports nothing from heraut. New package `commitwizard` imports `conventionalcommit, config, port, app, ui, huh` (the `commitwizard → app` edge is new and acyclic).
- **Config sync:** any change to a `internal/config` field must also update `schema.json` AND `docs/heraut.sample.yml` AND the relevant `docs/specs/` file.
- **`commit_lint.scopes` is wizard-only** — `verify`/`check` must NOT enforce it (ADR-0027 keeps `verify` to types only).
- **`scopes` value semantics:** unset/empty → free-text scope step; set → select menu with `(custom…)` + `(none)`.
- **Error exit codes:** `commit create` maps its errors to the **Usage** code (consistent with `commit verify`/`check`, which both wrap as `exitcode.Usage`). Clean no-ops (user cancels staging, declines the final confirm, or `--dry-run`) return `nil` → exit 0.
- **Don't change `conventionalcommit.parseFooterBlock` or its tests** — load-bearing. New parsing for the wizard goes in a new exported `ParseFooterLine`.

**Test commands:** targeted `go test ./internal/<pkg>/ -run <Name> -v`; full suite `mise run test`. Lint: `mise run lint:fix`.

---

### Task 1: `conventionalcommit.Format` + `ParseFooterLine`

Two new exported helpers in the grammar package: `Format` serializes a `Commit` back to canonical text (round-trips with `Parse`); `ParseFooterLine` parses one trailer line for the wizard's footer step. Neither touches the existing `Parse`/`parseFooterBlock`.

**Files:**
- Modify: `internal/conventionalcommit/conventionalcommit.go`
- Test: `internal/conventionalcommit/conventionalcommit_test.go` (append)

**Interfaces:**
- Consumes: existing `Commit{Type, Scope string; Breaking bool; Description, Body string; Footers []Footer}`, `Footer{Token, Value string}`, `Parse`.
- Produces:
  - `func (c *Commit) Format() string`
  - `func ParseFooterLine(line string) (Footer, bool)`

- [ ] **Step 1: Write the failing tests**

Append to `internal/conventionalcommit/conventionalcommit_test.go`:

```go
func TestCommit_Format(t *testing.T) {
	tests := []struct {
		name string
		in   conventionalcommit.Commit
		want string
	}{
		{
			name: "type + subject only",
			in:   conventionalcommit.Commit{Type: "feat", Description: "add wizard"},
			want: "feat: add wizard",
		},
		{
			name: "scope",
			in:   conventionalcommit.Commit{Type: "fix", Scope: "cmd", Description: "x"},
			want: "fix(cmd): x",
		},
		{
			name: "breaking bang",
			in:   conventionalcommit.Commit{Type: "feat", Scope: "cmd", Breaking: true, Description: "drop flag"},
			want: "feat(cmd)!: drop flag",
		},
		{
			name: "body",
			in:   conventionalcommit.Commit{Type: "feat", Description: "x", Body: "why line one\nwhy line two"},
			want: "feat: x\n\nwhy line one\nwhy line two",
		},
		{
			name: "breaking footer + user footer",
			in: conventionalcommit.Commit{
				Type: "feat", Description: "x", Breaking: true,
				Footers: []conventionalcommit.Footer{
					{Token: "BREAKING CHANGE", Value: "old removed"},
					{Token: "Closes", Value: "#42"},
				},
			},
			want: "feat!: x\n\nBREAKING CHANGE: old removed\nCloses: #42",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.in.Format())
		})
	}
}

func TestCommit_Format_RoundTripsThroughParse(t *testing.T) {
	original := conventionalcommit.Commit{
		Type: "feat", Scope: "cmd", Breaking: true, Description: "add the wizard",
		Body: "Guided prompts build the message.",
		Footers: []conventionalcommit.Footer{
			{Token: "BREAKING CHANGE", Value: "the old path is gone"},
			{Token: "Closes", Value: "#42"},
		},
	}
	reparsed, err := conventionalcommit.Parse(original.Format())
	require.NoError(t, err)
	assert.Equal(t, original.Type, reparsed.Type)
	assert.Equal(t, original.Scope, reparsed.Scope)
	assert.True(t, reparsed.Breaking)
	assert.Equal(t, original.Description, reparsed.Description)
	assert.Equal(t, original.Body, reparsed.Body)
	assert.Equal(t, original.Footers, reparsed.Footers)
}

func TestParseFooterLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		want      conventionalcommit.Footer
		wantOK    bool
	}{
		{"colon form", "Closes: #42", conventionalcommit.Footer{Token: "Closes", Value: "#42"}, true},
		{"hash form preserves #", "Closes #42", conventionalcommit.Footer{Token: "Closes", Value: "#42"}, true},
		{"breaking change token", "BREAKING CHANGE: gone", conventionalcommit.Footer{Token: "BREAKING CHANGE", Value: "gone"}, true},
		{"not a footer", "just prose here", conventionalcommit.Footer{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := conventionalcommit.ParseFooterLine(tc.line)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/conventionalcommit/ -run 'Format|ParseFooterLine' -v`
Expected: FAIL — `c.Format undefined` / `ParseFooterLine undefined`.

- [ ] **Step 3: Implement `Format` and `ParseFooterLine`**

Add to `internal/conventionalcommit/conventionalcommit.go` (after `Parse`):

```go
// Format renders a Commit back to its canonical Conventional Commits message text.
// It round-trips with Parse: Parse(c.Format()) reproduces c's structural fields.
// Empty scope, body, and footer list are omitted; Breaking adds "!" to the header.
func (c *Commit) Format() string {
	var b strings.Builder
	b.WriteString(c.Type)
	if c.Scope != "" {
		b.WriteString("(" + c.Scope + ")")
	}
	if c.Breaking {
		b.WriteString("!")
	}
	b.WriteString(": " + c.Description)
	if c.Body != "" {
		b.WriteString("\n\n" + c.Body)
	}
	if len(c.Footers) > 0 {
		b.WriteString("\n\n")
		for i, f := range c.Footers {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(f.Token + ": " + f.Value)
		}
	}
	return b.String()
}

// ParseFooterLine parses a single footer trailer line for message construction
// (the commit wizard). ok is false when line is not a valid footer. Unlike the
// internal footer-block parser used by Parse, it preserves the leading "#" of the
// "Token #value" form so the result round-trips through Format.
func ParseFooterLine(line string) (Footer, bool) {
	m := footerLinePattern.FindStringSubmatch(line)
	if m == nil {
		return Footer{}, false
	}
	value := m[3]
	if m[2] == " #" {
		value = "#" + value
	}
	return Footer{Token: m[1], Value: value}, true
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/conventionalcommit/ -v`
Expected: PASS (new tests + all existing).

- [ ] **Step 5: Commit**

```bash
git add internal/conventionalcommit/conventionalcommit.go internal/conventionalcommit/conventionalcommit_test.go
git commit -m "feat(conventionalcommit): add Format and ParseFooterLine"
```

---

### Task 2: `app.AllowedCommitTypes` + `VerifyCommit` refactor

Extract the "config types or default list" decision into a reusable helper so the wizard and the verifier share one source of truth. Behavior-preserving.

**Files:**
- Modify: `internal/app/commit.go`
- Test: `internal/app/commit_test.go` (append)

**Interfaces:**
- Consumes: existing `DefaultCommitTypes []string`, `config.Config.CommitLint.Types`.
- Produces: `func AllowedCommitTypes(cfg *config.Config) []string`.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/commit_test.go`:

```go
func TestAllowedCommitTypes(t *testing.T) {
	t.Run("nil config returns defaults", func(t *testing.T) {
		assert.Equal(t, app.DefaultCommitTypes, app.AllowedCommitTypes(nil))
	})
	t.Run("empty commit_lint returns defaults", func(t *testing.T) {
		cfg := &config.Config{CommitLint: &config.CommitLint{}}
		assert.Equal(t, app.DefaultCommitTypes, app.AllowedCommitTypes(cfg))
	})
	t.Run("configured types override", func(t *testing.T) {
		cfg := &config.Config{CommitLint: &config.CommitLint{Types: []string{"feat", "fix"}}}
		assert.Equal(t, []string{"feat", "fix"}, app.AllowedCommitTypes(cfg))
	})
}
```

(If `commit_test.go` does not already import `config`, add `"github.com/adaouat/heraut/internal/config"`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestAllowedCommitTypes -v`
Expected: FAIL — `app.AllowedCommitTypes undefined`.

- [ ] **Step 3: Add the helper and refactor `VerifyCommit`**

In `internal/app/commit.go`, add:

```go
// AllowedCommitTypes returns the configured commit_lint.types when set, otherwise
// DefaultCommitTypes. Single source of truth shared by VerifyCommit and the commit wizard.
func AllowedCommitTypes(cfg *config.Config) []string {
	if cfg != nil && cfg.CommitLint != nil && len(cfg.CommitLint.Types) > 0 {
		return cfg.CommitLint.Types
	}
	return DefaultCommitTypes
}
```

Replace the inline type-selection block in `VerifyCommit`:

```go
	types := DefaultCommitTypes
	if cfg != nil && cfg.CommitLint != nil && len(cfg.CommitLint.Types) > 0 {
		types = cfg.CommitLint.Types
	}
```

with:

```go
	types := AllowedCommitTypes(cfg)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'AllowedCommitTypes|VerifyCommit' -v`
Expected: PASS (new test + all existing `VerifyCommit` tests still green).

- [ ] **Step 5: Commit**

```bash
git add internal/app/commit.go internal/app/commit_test.go
git commit -m "refactor(app): extract AllowedCommitTypes from VerifyCommit"
```

---

### Task 3: `commit_lint.scopes` config field + schema + sample + spec

Add the optional wizard-only `scopes` list and keep all four sync points consistent.

**Files:**
- Modify: `internal/config/config.go` (the `CommitLint` struct, ~line 35)
- Modify: `schema.json` (the `CommitLint` definition, ~line 71)
- Modify: `docs/heraut.sample.yml` (~line 101 block)
- Modify: `docs/specs/02-configuration.md` (commit_lint section)
- Test: `internal/config/loader_test.go` (append — confirm a config file with `scopes` round-trips)

**Interfaces:**
- Consumes: existing `config.CommitLint{Types []string}`.
- Produces: `config.CommitLint.Scopes []string` (yaml `scopes,omitempty`).

- [ ] **Step 1: Write the failing test**

Append to `internal/config/loader_test.go` (match the existing loader-test style — write a temp `.heraut.yml`, `config.Load`, assert):

```go
func TestLoad_CommitLintScopes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
versioning:
  strategy: semver
release:
  platforms:
    - type: github
commit_lint:
  types: [feat, fix]
  scopes: [cmd, config, versioning]
`), 0o644))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg.CommitLint)
	assert.Equal(t, []string{"cmd", "config", "versioning"}, cfg.CommitLint.Scopes)
}
```

(Add imports `os`, `path/filepath` if absent. If the minimal valid config in this repo differs, copy the shape from a neighbouring loader test in the same file.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoad_CommitLintScopes -v`
Expected: FAIL — strict parsing rejects the unknown key `scopes` (loader.go errors on unknown keys).

- [ ] **Step 3: Add the struct field**

In `internal/config/config.go`, extend `CommitLint`:

```go
type CommitLint struct {
	// Types restricts which conventional-commit type words heraut commit verify accepts.
	// Replaces (does not extend) the default list when set: feat, fix, docs, chore,
	// refactor, test, style, perf, ci, build.
	Types []string `yaml:"types,omitempty"`

	// Scopes is an optional allow-list used only by `heraut commit create` to offer a
	// scope picker. It is NOT enforced by `heraut commit verify`/`check` (ADR-0027 keeps
	// verify to types only). Empty/unset → the wizard uses a free-text scope step.
	Scopes []string `yaml:"scopes,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestLoad_CommitLintScopes -v`
Expected: PASS.

- [ ] **Step 5: Update schema.json**

In `schema.json`, inside `definitions.CommitLint.properties` (after the `types` property), add:

```json
        "scopes": {
          "type": "array",
          "items": {
            "type": "string"
          },
          "description": "Optional scope allow-list used only by 'heraut commit create' to offer a scope picker. NOT enforced by 'heraut commit verify'/'check'. Omit for a free-text scope step."
        }
```

- [ ] **Step 6: Update the sample config**

In `docs/heraut.sample.yml`, extend the `commit_lint` block (~line 106) to:

```yaml
# commit_lint:
#   types: [feat, fix, docs, chore, refactor, test, style, perf, ci, build]
#   # scopes: optional — used only by `heraut commit create` to offer a scope picker.
#   # Not enforced by `heraut commit verify`. Omit for a free-text scope step.
#   scopes: [cmd, config, versioning, ui]
```

- [ ] **Step 7: Update the configuration spec**

In `docs/specs/02-configuration.md`, in the `commit_lint` section, document `scopes`: optional list of strings; used only by `heraut commit create` to populate the scope picker; explicitly NOT enforced by `heraut commit verify`/`check`; unset → free-text scope step.

- [ ] **Step 8: Run validation + full config tests**

Run: `go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/config/config.go internal/config/loader_test.go schema.json docs/heraut.sample.yml docs/specs/02-configuration.md
git commit -m "feat(config): add wizard-only commit_lint.scopes"
```

---

### Task 4: `commitwizard` — `Answers`, `Options`, `Assemble`, `parseFooterLines`

Create the package with its pure core: the answer model and the mapping to a `conventionalcommit.Commit`.

**Files:**
- Create: `internal/commitwizard/commitwizard.go`
- Test: `internal/commitwizard/commitwizard_test.go`

**Interfaces:**
- Consumes: `conventionalcommit.{Commit, Footer, ParseFooterLine}` (Task 1).
- Produces:
  - `type Answers struct { Type, Scope, Subject, Body string; Breaking bool; BreakingDesc string; Footers []conventionalcommit.Footer }`
  - `type Options struct { All, DryRun bool; Out io.Writer }`
  - `func Assemble(a Answers) *conventionalcommit.Commit`
  - `func parseFooterLines(text string) ([]conventionalcommit.Footer, error)`

- [ ] **Step 1: Write the failing tests**

Create `internal/commitwizard/commitwizard_test.go`:

```go
package commitwizard

import (
	"testing"

	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssemble(t *testing.T) {
	t.Run("minimal", func(t *testing.T) {
		c := Assemble(Answers{Type: "feat", Subject: "add wizard"})
		assert.Equal(t, "feat: add wizard", c.Format())
	})
	t.Run("scope + body", func(t *testing.T) {
		c := Assemble(Answers{Type: "fix", Scope: "cmd", Subject: "x", Body: "why"})
		assert.Equal(t, "fix(cmd): x\n\nwhy", c.Format())
	})
	t.Run("breaking adds bang and BREAKING CHANGE footer first", func(t *testing.T) {
		c := Assemble(Answers{
			Type: "feat", Scope: "cmd", Subject: "drop flag",
			Breaking: true, BreakingDesc: "old removed",
			Footers: []conventionalcommit.Footer{{Token: "Closes", Value: "#42"}},
		})
		assert.True(t, c.Breaking)
		assert.Equal(t, "feat(cmd)!: drop flag\n\nBREAKING CHANGE: old removed\nCloses: #42", c.Format())
	})
	t.Run("breaking bang without description adds no footer", func(t *testing.T) {
		c := Assemble(Answers{Type: "feat", Subject: "x", Breaking: true})
		assert.Equal(t, "feat!: x", c.Format())
	})
}

func TestParseFooterLines(t *testing.T) {
	t.Run("parses and skips blanks", func(t *testing.T) {
		got, err := parseFooterLines("Closes: #42\n\nRefs: PROJ-1\n")
		require.NoError(t, err)
		assert.Equal(t, []conventionalcommit.Footer{
			{Token: "Closes", Value: "#42"},
			{Token: "Refs", Value: "PROJ-1"},
		}, got)
	})
	t.Run("rejects a non-footer line", func(t *testing.T) {
		_, err := parseFooterLines("Closes: #42\nnot a footer")
		require.Error(t, err)
	})
	t.Run("empty input yields nil", func(t *testing.T) {
		got, err := parseFooterLines("")
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/commitwizard/ -v`
Expected: FAIL — package/identifiers undefined.

- [ ] **Step 3: Implement the pure core**

Create `internal/commitwizard/commitwizard.go`:

```go
// Package commitwizard implements `heraut commit create` — an interactive wizard that
// builds a Conventional Commits message and runs git commit. The interactive form lives
// in form.go (no unit tests, same as internal/scaffold/wizard.go); everything else is
// unit- or contract-tested.
package commitwizard

import (
	"fmt"
	"io"
	"strings"

	"github.com/adaouat/heraut/internal/conventionalcommit"
)

// Answers is the data the interactive form collects.
type Answers struct {
	Type         string
	Scope        string
	Subject      string
	Body         string
	Breaking     bool
	BreakingDesc string
	Footers      []conventionalcommit.Footer
}

// Options controls a wizard run.
type Options struct {
	All    bool      // pass -a to git commit (stage tracked modifications)
	DryRun bool      // print the assembled message, do not stage or commit
	Out    io.Writer // command stdout (also used for the TTY check)
}

// Assemble maps collected Answers to a conventional-commit Commit. A breaking change adds
// "!" to the header; a non-empty breaking description is prepended as a BREAKING CHANGE
// footer ahead of the user's footers.
func Assemble(a Answers) *conventionalcommit.Commit {
	c := &conventionalcommit.Commit{
		Type:        a.Type,
		Scope:       a.Scope,
		Breaking:    a.Breaking,
		Description: a.Subject,
		Body:        a.Body,
	}
	if a.Breaking && a.BreakingDesc != "" {
		c.Footers = append(c.Footers, conventionalcommit.Footer{
			Token: "BREAKING CHANGE",
			Value: a.BreakingDesc,
		})
	}
	c.Footers = append(c.Footers, a.Footers...)
	return c
}

// parseFooterLines converts a multi-line footer block into structured footers, skipping
// blank lines and erroring on any line that is not a valid trailer.
func parseFooterLines(text string) ([]conventionalcommit.Footer, error) {
	var footers []conventionalcommit.Footer
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f, ok := conventionalcommit.ParseFooterLine(line)
		if !ok {
			return nil, fmt.Errorf("invalid footer line %q: expected \"Token: value\"", line)
		}
		footers = append(footers, f)
	}
	return footers, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/commitwizard/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/commitwizard/commitwizard.go internal/commitwizard/commitwizard_test.go
git commit -m "feat(commitwizard): add Answers, Assemble, footer parsing"
```

---

### Task 5: `commitwizard` git helpers — `hasStaged`, `stageAll`, `commit`

The three git operations, contract-tested against `MockRunner`.

**Files:**
- Create: `internal/commitwizard/git.go`
- Test: `internal/commitwizard/git_test.go`

**Interfaces:**
- Consumes: `port.Runner` (`Run(name string, args ...string) (string, string, error)`).
- Produces:
  - `func hasStaged(r port.Runner) (bool, error)`
  - `func stageAll(r port.Runner) error`
  - `func commit(r port.Runner, message string, all bool) error`

- [ ] **Step 1: Write the failing tests**

Create `internal/commitwizard/git_test.go`:

```go
package commitwizard

import (
	"errors"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasStaged(t *testing.T) {
	t.Run("staged when name-only output is non-empty", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("internal/cmd/commit.go\n", "", nil)
		staged, err := hasStaged(mr)
		require.NoError(t, err)
		assert.True(t, staged)
		assert.Equal(t, "git", mr.Calls[0].Name)
		assert.Equal(t, []string{"diff", "--cached", "--name-only"}, mr.Calls[0].Args)
	})
	t.Run("not staged when output is empty", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", nil)
		staged, err := hasStaged(mr)
		require.NoError(t, err)
		assert.False(t, staged)
	})
	t.Run("propagates runner error", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", errors.New("boom"))
		_, err := hasStaged(mr)
		require.Error(t, err)
	})
}

func TestStageAll(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)
	require.NoError(t, stageAll(mr))
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"add", "-A"}, mr.Calls[0].Args)
}

func TestCommit(t *testing.T) {
	t.Run("git commit -F <tmpfile>", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", nil)
		require.NoError(t, commit(mr, "feat: x", false))
		require.Len(t, mr.Calls, 1)
		args := mr.Calls[0].Args
		require.Len(t, args, 3)
		assert.Equal(t, "commit", args[0])
		assert.Equal(t, "-F", args[1])
		assert.NotEmpty(t, args[2]) // dynamic tmpfile path
	})
	t.Run("all adds -a", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "", nil)
		require.NoError(t, commit(mr, "feat: x", true))
		args := mr.Calls[0].Args
		require.Len(t, args, 4)
		assert.Equal(t, []string{"commit", "-a", "-F"}, args[:3])
	})
	t.Run("propagates runner error", func(t *testing.T) {
		mr := exectest.NewMockRunner()
		mr.QueueResponse("", "rejected by hook", errors.New("exit 1"))
		require.Error(t, commit(mr, "feat: x", false))
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/commitwizard/ -run 'HasStaged|StageAll|Commit' -v`
Expected: FAIL — helpers undefined.

- [ ] **Step 3: Implement the git helpers**

Create `internal/commitwizard/git.go`:

```go
package commitwizard

import (
	"fmt"
	"os"
	"strings"

	"github.com/adaouat/heraut/internal/port"
)

// hasStaged reports whether the index has staged changes. Uses --name-only (empty output
// = nothing staged) rather than --quiet's exit code, which keeps the check trivial to
// model with MockRunner.
func hasStaged(r port.Runner) (bool, error) {
	out, _, err := r.Run("git", "diff", "--cached", "--name-only")
	if err != nil {
		return false, fmt.Errorf("checking staged changes: %w", err)
	}
	return strings.TrimSpace(out) != "", nil
}

// stageAll runs `git add -A`.
func stageAll(r port.Runner) error {
	if _, _, err := r.Run("git", "add", "-A"); err != nil {
		return fmt.Errorf("staging all changes: %w", err)
	}
	return nil
}

// commit writes message to a temp file and runs `git commit -F <file>` (plus -a when all),
// matching the temp-file pattern the gitcliff generator uses. The temp file is always
// removed, including on error paths.
func commit(r port.Runner, message string, all bool) error {
	f, err := os.CreateTemp("", "heraut-commit-*.txt")
	if err != nil {
		return fmt.Errorf("creating temp commit message file: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()

	if _, err := f.WriteString(message); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing temp commit message file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing temp commit message file: %w", err)
	}

	args := []string{"commit"}
	if all {
		args = append(args, "-a")
	}
	args = append(args, "-F", f.Name())
	if _, _, err := r.Run("git", args...); err != nil {
		return fmt.Errorf("running git commit: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/commitwizard/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/commitwizard/git.go internal/commitwizard/git_test.go
git commit -m "feat(commitwizard): add git staging and commit helpers"
```

---

### Task 6: `commitwizard.finalize` — verify guard, dry-run, confirm-gated commit

The testable post-form pipeline. The final confirm is injected as a function so the whole thing is unit-testable without a TTY.

**Files:**
- Modify: `internal/commitwizard/commitwizard.go`
- Test: `internal/commitwizard/commitwizard_test.go` (append)

**Interfaces:**
- Consumes: `Assemble` (Task 4), `commit` (Task 5), `app.VerifyCommit` (existing), `ui.Info` (existing).
- Produces: `func finalize(r port.Runner, cfg *config.Config, a Answers, opts Options, confirm func(out io.Writer, msg string) (bool, error)) error`

- [ ] **Step 1: Write the failing tests**

Append to `internal/commitwizard/commitwizard_test.go`:

```go
import (
	// add to the existing import block:
	"bytes"

	"github.com/adaouat/forge/exec/exectest"
)

func alwaysConfirm(_ io.Writer, _ string) (bool, error) { return true, nil }
func neverConfirm(_ io.Writer, _ string) (bool, error)  { return false, nil }

func TestFinalize_CommitsOnConfirm(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil) // git commit
	var out bytes.Buffer
	err := finalize(mr, nil, Answers{Type: "feat", Subject: "x"},
		Options{Out: &out}, alwaysConfirm)
	require.NoError(t, err)
	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "commit", mr.Calls[0].Args[0])
}

func TestFinalize_CancelOnDecline(t *testing.T) {
	mr := exectest.NewMockRunner()
	var out bytes.Buffer
	err := finalize(mr, nil, Answers{Type: "feat", Subject: "x"},
		Options{Out: &out}, neverConfirm)
	require.NoError(t, err)
	assert.Empty(t, mr.Calls, "no git commit on decline")
	assert.Contains(t, out.String(), "feat: x")
}

func TestFinalize_DryRunPrintsAndSkipsCommit(t *testing.T) {
	mr := exectest.NewMockRunner()
	var out bytes.Buffer
	err := finalize(mr, nil, Answers{Type: "feat", Subject: "x"},
		Options{DryRun: true, Out: &out}, alwaysConfirm)
	require.NoError(t, err)
	assert.Empty(t, mr.Calls, "no git commit in dry-run")
	assert.Contains(t, out.String(), "feat: x")
	assert.Contains(t, out.String(), "dry-run")
}

func TestFinalize_GuardBlocksInvalidType(t *testing.T) {
	mr := exectest.NewMockRunner()
	var out bytes.Buffer
	// "wip" is not in DefaultCommitTypes → VerifyCommit fails → no commit.
	err := finalize(mr, nil, Answers{Type: "wip", Subject: "x"},
		Options{Out: &out}, alwaysConfirm)
	require.Error(t, err)
	assert.Empty(t, mr.Calls)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/commitwizard/ -run TestFinalize -v`
Expected: FAIL — `finalize undefined`.

- [ ] **Step 3: Implement `finalize`**

Add to `internal/commitwizard/commitwizard.go` (extend the import block with `"github.com/adaouat/heraut/internal/app"`, `"github.com/adaouat/heraut/internal/config"`, `"github.com/adaouat/heraut/internal/port"`, `"github.com/adaouat/heraut/internal/ui"`):

```go
// finalize assembles, verifies, and (unless dry-run) commits. confirm is injected so the
// pipeline is testable without a terminal; Run passes the interactive confirmCommit form.
func finalize(r port.Runner, cfg *config.Config, a Answers, opts Options, confirm func(out io.Writer, msg string) (bool, error)) error {
	msg := Assemble(a).Format()

	if err := app.VerifyCommit(cfg, msg); err != nil {
		return fmt.Errorf("assembled message failed validation: %w", err)
	}

	if opts.DryRun {
		fmt.Fprintln(opts.Out, msg)
		fmt.Fprintln(opts.Out, ui.Info(opts.Out, "[dry-run] would run: git commit"))
		return nil
	}

	ok, err := confirm(opts.Out, msg)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(opts.Out, msg)
		fmt.Fprintln(opts.Out, ui.Info(opts.Out, "commit cancelled"))
		return nil
	}

	return commit(r, msg, opts.All)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/commitwizard/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/commitwizard/commitwizard.go internal/commitwizard/commitwizard_test.go
git commit -m "feat(commitwizard): add finalize with verify guard and dry-run"
```

---

### Task 7: `commitwizard.Run` + the huh form + `ui.IsTTY`

The orchestrator (with the testable non-TTY early return) and the interactive form (the only untested file, matching `scaffold/wizard.go`).

**Files:**
- Modify: `internal/ui/status.go` (add `IsTTY` wrapper)
- Modify: `internal/commitwizard/commitwizard.go` (add `Run`, `typeOptionLabel`)
- Create: `internal/commitwizard/form.go` (huh form — no unit tests by design)
- Test: `internal/commitwizard/commitwizard_test.go` (append — `Run` non-TTY return, `typeOptionLabel`)

**Interfaces:**
- Consumes: `ui.IsTTY`, `hasStaged`/`stageAll` (Task 5), `finalize` (Task 6), `app.AllowedCommitTypes` (Task 2), `conventionalcommit.ParseFooterLine` (Task 1), `huh`, `ui.HuhTheme`.
- Produces:
  - `ui.IsTTY(w io.Writer) bool`
  - `func Run(r port.Runner, cfg *config.Config, opts Options) error`
  - `func typeOptionLabel(t string) string`
  - (form-internal, untested) `collectAnswers`, `confirmStageAll`, `confirmCommit`

- [ ] **Step 1: Add the `ui.IsTTY` wrapper**

In `internal/ui/status.go`, add after `Header`:

```go
// IsTTY reports whether w is an interactive terminal.
func IsTTY(w io.Writer) bool { return forgeui.IsTTY(w) }
```

- [ ] **Step 2: Write the failing tests**

Append to `internal/commitwizard/commitwizard_test.go`:

```go
func TestRun_NonTTYErrors(t *testing.T) {
	mr := exectest.NewMockRunner()
	var out bytes.Buffer // a *bytes.Buffer is never a TTY
	err := Run(mr, nil, Options{Out: &out})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interactive terminal")
	assert.Empty(t, mr.Calls, "no git calls when there is no TTY")
}

func TestTypeOptionLabel(t *testing.T) {
	assert.Equal(t, "feat    A new feature", typeOptionLabel("feat"))
	assert.Equal(t, "custom", typeOptionLabel("custom")) // unknown type → bare label
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/commitwizard/ -run 'TestRun_NonTTYErrors|TestTypeOptionLabel' -v`
Expected: FAIL — `Run` / `typeOptionLabel` undefined.

- [ ] **Step 4: Implement `Run` and `typeOptionLabel`**

Add to `internal/commitwizard/commitwizard.go` (add `"errors"` to imports):

```go
// Run drives the interactive wizard: TTY check → staging (unless --all or --dry-run) →
// collect answers → finalize. Returns nil for clean no-ops (cancel/decline/dry-run).
func Run(r port.Runner, cfg *config.Config, opts Options) error {
	if !ui.IsTTY(opts.Out) {
		return errors.New("commit create requires an interactive terminal")
	}

	if !opts.DryRun && !opts.All {
		staged, err := hasStaged(r)
		if err != nil {
			return err
		}
		if !staged {
			stage, err := confirmStageAll()
			if err != nil {
				return err
			}
			if !stage {
				fmt.Fprintln(opts.Out, ui.Info(opts.Out, "nothing staged — cancelled"))
				return nil
			}
			if err := stageAll(r); err != nil {
				return err
			}
		}
	}

	a, err := collectAnswers(cfg)
	if err != nil {
		return err
	}
	return finalize(r, cfg, a, opts, confirmCommit)
}

// commitTypeDescriptions are the one-line hints shown beside the 10 built-in types.
var commitTypeDescriptions = map[string]string{
	"feat":     "A new feature",
	"fix":      "A bug fix",
	"docs":     "Documentation only",
	"chore":    "Tooling / housekeeping",
	"refactor": "Code change, no behaviour change",
	"test":     "Adding or fixing tests",
	"style":    "Formatting / whitespace",
	"perf":     "Performance improvement",
	"ci":       "CI / release tooling",
	"build":    "Build system / dependencies",
}

// typeOptionLabel renders the select-menu label for a commit type: "<type>  <description>"
// for the built-in types, or the bare type for custom configured ones.
func typeOptionLabel(t string) string {
	if d, ok := commitTypeDescriptions[t]; ok {
		return fmt.Sprintf("%-6s  %s", t, d)
	}
	return t
}
```

- [ ] **Step 5: Run the non-TTY + label tests to verify they pass**

Run: `go test ./internal/commitwizard/ -run 'TestRun_NonTTYErrors|TestTypeOptionLabel' -v`
Expected: PASS. (`Run` still won't compile until `collectAnswers`, `confirmStageAll`, `confirmCommit` exist — do Step 6 first if the package fails to build, then re-run.)

- [ ] **Step 6: Implement the interactive form (no unit tests by design)**

Create `internal/commitwizard/form.go`. This file mirrors `internal/scaffold/wizard.go`: it builds huh forms and is excluded from coverage (untestable without a VT100).

```go
package commitwizard

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"

	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/adaouat/heraut/internal/ui"
)

const (
	scopeCustom = "\x00custom"
	scopeNone   = "\x00none"
)

func themedForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(ui.HuhTheme())
}

// collectAnswers runs the field prompts (type, scope, subject, breaking, body, footers)
// and returns the assembled Answers.
func collectAnswers(cfg *config.Config) (Answers, error) {
	var a Answers
	var scopeChoice, customScope, footerText string

	typeOpts := make([]huh.Option[string], 0)
	for _, t := range app.AllowedCommitTypes(cfg) {
		typeOpts = append(typeOpts, huh.NewOption(typeOptionLabel(t), t))
	}

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewSelect[string]().Title("Type").Options(typeOpts...).Value(&a.Type),
		),
	}

	scopes := configuredScopes(cfg)
	if len(scopes) > 0 {
		scopeOpts := make([]huh.Option[string], 0, len(scopes)+2)
		for _, s := range scopes {
			scopeOpts = append(scopeOpts, huh.NewOption(s, s))
		}
		scopeOpts = append(scopeOpts, huh.NewOption("(custom…)", scopeCustom), huh.NewOption("(none)", scopeNone))
		groups = append(groups,
			huh.NewGroup(
				huh.NewSelect[string]().Title("Scope").Options(scopeOpts...).Value(&scopeChoice),
			),
			huh.NewGroup(
				huh.NewInput().Title("Custom scope").Value(&customScope),
			).WithHideFunc(func() bool { return scopeChoice != scopeCustom }),
		)
	} else {
		groups = append(groups,
			huh.NewGroup(
				huh.NewInput().Title("Scope").Description("optional — leave empty for none").Value(&a.Scope),
			),
		)
	}

	groups = append(groups,
		huh.NewGroup(
			huh.NewInput().Title("Subject").
				Description("short imperative summary").
				Value(&a.Subject).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("subject is required")
					}
					if strings.ContainsRune(s, '\n') {
						return fmt.Errorf("subject must be a single line")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewConfirm().Title("Breaking change?").Value(&a.Breaking),
		),
		huh.NewGroup(
			huh.NewInput().Title("Describe the breaking change").Value(&a.BreakingDesc),
		).WithHideFunc(func() bool { return !a.Breaking }),
		huh.NewGroup(
			huh.NewText().Title("Body").Description("optional — the why").Value(&a.Body),
		),
		huh.NewGroup(
			huh.NewText().Title("Footers").
				Description(`optional — one "Token: value" per line, e.g. Closes: #42`).
				Value(&footerText).
				Validate(func(s string) error {
					_, err := parseFooterLines(s)
					return err
				}),
		),
	)

	if err := themedForm(groups...).Run(); err != nil {
		return Answers{}, fmt.Errorf("collecting commit details: %w", err)
	}

	switch scopeChoice {
	case scopeCustom:
		a.Scope = strings.TrimSpace(customScope)
	case scopeNone, "":
		// a.Scope already set by the free-text path, or intentionally empty
	default:
		a.Scope = scopeChoice
	}

	footers, err := parseFooterLines(footerText)
	if err != nil {
		return Answers{}, err
	}
	a.Footers = footers
	return a, nil
}

func configuredScopes(cfg *config.Config) []string {
	if cfg != nil && cfg.CommitLint != nil {
		return cfg.CommitLint.Scopes
	}
	return nil
}

// confirmStageAll prompts when nothing is staged. Returns true to `git add -A`, false to cancel.
func confirmStageAll() (bool, error) {
	var stage bool
	err := themedForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Nothing staged").
				Description("Stage all changes (git add -A) before committing?").
				Affirmative("Stage all").
				Negative("Cancel").
				Value(&stage),
		),
	).Run()
	if err != nil {
		return false, fmt.Errorf("confirming staging: %w", err)
	}
	return stage, nil
}

// confirmCommit shows the assembled message and asks for final confirmation.
func confirmCommit(_ io.Writer, msg string) (bool, error) {
	var ok bool
	err := themedForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Commit this message?").
				Description(msg).
				Value(&ok),
		),
	).Run()
	if err != nil {
		return false, fmt.Errorf("confirming commit: %w", err)
	}
	return ok, nil
}
```

Note: `confirmCommit` needs `io` — add `"io"` to `form.go` imports. (`conventionalcommit` import is used via `parseFooterLines`'s return type only indirectly; if `goimports`/lint flags it unused, remove it — `hk fix` will sort imports.)

- [ ] **Step 7: Run the full package + build**

Run: `go build ./... && go test ./internal/commitwizard/ ./internal/ui/ -v`
Expected: builds; PASS.

- [ ] **Step 8: Lint**

Run: `mise run lint:go:fix` then `mise run lint:go:check`
Expected: clean (fixes import ordering, etc.).

- [ ] **Step 9: Commit**

```bash
git add internal/ui/status.go internal/commitwizard/commitwizard.go internal/commitwizard/commitwizard_test.go internal/commitwizard/form.go
git commit -m "feat(commitwizard): add interactive form and Run orchestrator"
```

---

### Task 8: Wire `heraut commit create` + command spec

Add the thin cobra command and document it.

**Files:**
- Modify: `internal/cmd/commit.go` (register subcommand + `newCommitCreateCmd`)
- Modify: `docs/specs/03-commands.md` (document `heraut commit create`)
- Test: `internal/cmd/commit_test.go` (append — black-box cobra tests)

**Interfaces:**
- Consumes: `commitwizard.{Run, Options}` (Task 7), `execadapter.New` (existing import in commit.go), `config.{ResolvePath, Load, Validate}`, `exitcode.{Wrap, Usage, Config}`, `printConfigErrors` (existing in commit.go).
- Produces: `heraut commit create` with `--all/-a`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/cmd/commit_test.go`:

```go
func TestCommitCreate_Registered(t *testing.T) {
	root := cmd.NewRootCmd("dev")
	var found, hasAll bool
	for _, c := range root.Commands() {
		if c.Use != "commit" {
			continue
		}
		for _, sc := range c.Commands() {
			if strings.HasPrefix(sc.Use, "create") {
				found = true
				hasAll = sc.Flags().Lookup("all") != nil
			}
		}
	}
	assert.True(t, found, "commit create subcommand registered")
	assert.True(t, hasAll, "--all flag present")
}

func TestCommitCreate_NonTTYErrors(t *testing.T) {
	// executeRoot writes to a *bytes.Buffer (never a TTY) → wizard must refuse.
	out, err := executeRoot("commit", "create")
	require.Error(t, err)
	assert.Contains(t, out+err.Error(), "interactive terminal")
}
```

(`executeRoot` is the existing helper in `version_test.go`, same `cmd_test` package. If it returns only `(string, error)`, the assertion above already accounts for that.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cmd/ -run TestCommitCreate -v`
Expected: FAIL — subcommand not registered.

- [ ] **Step 3: Register and implement the command**

In `internal/cmd/commit.go`, add the registration in `NewCommitCmd`:

```go
	commitCmd.AddCommand(newCommitVerifyCmd())
	commitCmd.AddCommand(newCommitCheckCmd())
	commitCmd.AddCommand(newCommitCreateCmd())
```

Add the constructor (and `"github.com/adaouat/heraut/internal/commitwizard"` to the imports):

```go
func newCommitCreateCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Interactively author a Conventional Commits message and commit it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			dryRun, _ := cmd.Flags().GetBool("dry-run")
			verbose, _ := cmd.Flags().GetBool("verbose")
			// Always a real runner: the wizard's only mutation (git commit) is gated by
			// Options.DryRun in finalize, and read-only staging checks must really run.
			runner := execadapter.New(false, verbose)

			opts := commitwizard.Options{All: all, DryRun: dryRun, Out: cmd.OutOrStdout()}
			if err := commitwizard.Run(runner, cfg, opts); err != nil {
				return exitcode.Wrap(exitcode.Usage, err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "stage all tracked modifications before committing (git commit -a)")
	return cmd
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cmd/ -run TestCommitCreate -v`
Expected: PASS.

- [ ] **Step 5: Document the command in the spec**

In `docs/specs/03-commands.md`, add a `heraut commit create` entry near `commit verify`/`commit check`. Cover: interactive TTY-only wizard; flow (type → scope → subject → breaking → body → footers → preview-confirm); `--all/-a`; honours global `--dry-run` (prints the message, no commit) and `--config`; sources types from `commit_lint.types` (else defaults) and scopes from `commit_lint.scopes` (else free text); output is guaranteed to pass `heraut commit verify`; errors (non-TTY, guard failure) map to the Usage exit code.

- [ ] **Step 6: Full suite + lint**

Run: `mise run test && mise run lint:check`
Expected: PASS / clean.

- [ ] **Step 7: Commit**

```bash
git add internal/cmd/commit.go internal/cmd/commit_test.go docs/specs/03-commands.md
git commit -m "feat(cmd): add heraut commit create wizard"
```

---

### Task 9: ADR-0031 + roadmap T120

Record the decision and close the loop on the roadmap (two-step flow).

**Files:**
- Create: `docs/adr/0031-interactive-commit-wizard.md`
- Modify: `docs/adr/README.md` (ADR index — add 0031 row)
- Modify: `docs/tasks/roadmap.md` (add T120, marked `[x]`, with completion note)

**Interfaces:** none (docs).

- [ ] **Step 1: Write ADR-0031**

Create `docs/adr/0031-interactive-commit-wizard.md` following the format of `docs/adr/0030-commit-check-rev-range-validation.md`. Record:
- **Decision:** `heraut commit create` — interactive, TTY-only wizard building a Conventional Commits message and running `git commit`.
- **Mechanism:** temp file `git commit -F <tmpfile>` (Runner has no stdin); new `internal/commitwizard` package; `conventionalcommit.Format`/`ParseFooterLine`; reuse of `app.AllowedCommitTypes`/`app.VerifyCommit` as the final guard (the new `commitwizard → app` import edge, acyclic).
- **Config:** new wizard-only `commit_lint.scopes`; explicitly NOT enforced by `verify`/`check`.
- **Staging UX:** lightweight "stage all / cancel" prompt + `--all/-a`; per-file picker deferred.
- **Errors:** map to the Usage exit code (consistent with `verify`/`check`).
- **Deferred to v2:** ticket-pattern integration (ADR-0024), per-file staging picker, `--amend`, structured footer add-loop.
- **Supersedes:** the "future work" notes in ADR-0027 and ADR-0030.

- [ ] **Step 2: Add 0031 to the ADR index**

In `docs/adr/README.md`, add the `0031` row matching the existing table format.

- [ ] **Step 3: Add T120 to the roadmap**

In `docs/tasks/roadmap.md` (after the T119 block, ~line 5090+), add:

```markdown
#### `[x]` T120: `heraut commit create` — interactive commit wizard

Per [ADR-0031](../adr/0031-interactive-commit-wizard.md). Interactive, TTY-only wizard
(type → scope → subject → breaking → body → footers → preview-confirm) that assembles a
Conventional Commits message via the new `conventionalcommit.Format`, validates it through
the existing `app.VerifyCommit` guard, and runs `git commit -F <tmpfile>`. New
`internal/commitwizard` package; new wizard-only `commit_lint.scopes`; lightweight
stage-all prompt + `--all/-a`. Tickets / per-file staging / `--amend` deferred to v2.

**Completion note:** <fill in actual decisions/deviations after implementation — e.g. the
huh form file `internal/commitwizard/form.go` carries no unit tests, same precedent as
`internal/scaffold/wizard.go`; errors map to Usage like the sibling commit subcommands.>

**Files:** `internal/conventionalcommit/conventionalcommit.go`, `internal/app/commit.go`,
`internal/config/config.go`, `schema.json`, `docs/heraut.sample.yml`,
`docs/specs/02-configuration.md`, `docs/specs/03-commands.md`,
`internal/commitwizard/{commitwizard,git,form}.go` (+ tests), `internal/ui/status.go`,
`internal/cmd/commit.go`, `docs/adr/0031-interactive-commit-wizard.md`.
```

- [ ] **Step 4: Fill in the completion note**

Replace the `<fill in…>` placeholder with the actual decisions/deviations encountered during Tasks 1–8.

- [ ] **Step 5: Commit**

```bash
git add docs/adr/0031-interactive-commit-wizard.md docs/adr/README.md docs/tasks/roadmap.md
git commit -m "docs(adr): add 0031 interactive commit wizard; complete T120"
```

---

## Self-Review

**Spec coverage** — every spec section maps to a task:
- Command surface (`create`, `--all`, global flags) → Task 8.
- UX flow / fields → form in Task 7; pure assembly in Task 4.
- `conventionalcommit.Format` → Task 1.
- Reuse of `VerifyCommit`/`AllowedCommitTypes` → Task 2 (helper), Task 6 (guard).
- `commit_lint.scopes` + sync points (schema/sample/spec) → Task 3.
- Git mechanism (`-F` temp file) + staging helpers → Task 5.
- Error handling (non-TTY, guard, dry-run, cancel/decline) → Tasks 6 (finalize) + 7 (Run) + 8 (exit mapping).
- Testing strategy (unit/contract/excluded form) → tests in Tasks 1–8; exclusion documented in Task 7/9.
- ADR + roadmap → Task 9.
- v2 backlog → recorded in ADR (Task 9), not built. ✓

**Placeholder scan:** one intentional `<fill in…>` exists in Task 9 Step 3 and is explicitly resolved in Task 9 Step 4 (completion note must reflect real implementation). No other TBD/TODO.

**Type consistency:** `Answers`, `Options`, `Assemble`, `finalize(r, cfg, a, opts, confirm)`, `Run(r, cfg, opts)`, `hasStaged`/`stageAll`/`commit(r, message, all)`, `AllowedCommitTypes(cfg)`, `Format()`, `ParseFooterLine(line) (Footer, bool)`, `typeOptionLabel(t)`, `ui.IsTTY(w)` are referenced identically across tasks. The `confirm` func signature `func(out io.Writer, msg string) (bool, error)` matches `confirmCommit` in Task 7.

## Execution Handoff

(Stated after the user reviews this plan.)
