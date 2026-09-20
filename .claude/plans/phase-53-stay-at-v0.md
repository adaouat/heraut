# Stay at v0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `versioning.bump.stay_at_v0: true` holds a breaking-change major bump back to minor while the current major is 0, with a visible warning, and `--allow-major` lifts it for one run.

**Architecture:** One pure clamp function in `internal/versioning/semver` shared by `semver` and the `semver-per-env` "auto" calculator. The resolver records warnings and never prints; `app.NewResolver` wraps it to copy them into a new `versioning.Result.Warnings`, which the pipelines print via a new `ui.WarnLines` right after the "Resolve version" step (and `version next` prints to stderr). `pipeline`, `perenv` and the `VersionCalculator` interface are otherwise untouched.

**Tech Stack:** Go, cobra, testify, `exectest.MockRunner` / `exectest.FakeBin`, `internal/testutil.RealGitRepo`, JSON Schema, `hk` (golangci-lint, gofmt, typos).

**Spec:** `docs/superpowers/specs/2026-09-20-stay-at-v0-design.md` (read it first). Roadmap: `docs/tasks/roadmap.md` → Phase 53, T302 + T303. **T304 (the ≥ 1.0 major-bump gate) is explicitly out of scope — do not start it.**

## Global Constraints

- **TDD, always:** write the failing test, run it, confirm it fails *for the expected reason*, then implement the minimum, then run it green. Never write implementation first.
- **One roadmap task per session unless the user approves more.** T302 = Tasks 1–3 below; T303 = Tasks 4–9. Confirm the user wants both in one go before starting Task 4.
- **Layer rules** (`.claude/rules/coding.md`): `internal/versioning/*` imports only `port`, `config`, `versioning`, `conventionalcommit`; `internal/pipeline` may import `ui`; `internal/cmd` may import `app`, `ui`, `config`, `exitcode`; `internal/cmd` never imports `pipeline` or constructs resolvers directly.
- **Errors:** always wrap with `fmt.Errorf("doing X: %w", err)`; never string-match errors; never `os.Exit` below `cmd/`.
- **Comments:** none by default; add one only for a non-obvious *why*. Referencing `ADR-0063` is fine. Never reference task ids (`T302`) or callers in code comments.
- **Config field changes** update `schema.json` **and** `docs/heraut.sample.yml` in the same commit.
- **Never delete or loosen an existing test row.** Add rows; do not remove any.
- **Warning text uses bare versions** (`1.0.0 → 0.69.0`, no tag prefix) for both `semver` and `semver-per-env`, and the literal characters `→` and `…`.
- **Lint/format:** run `hk check` before each commit; fix with `hk fix -S <linter>` (e.g. `hk fix -S golangci_lint`). Never call `gofmt` directly. **Never** `--no-verify`.
- **Commits:** Conventional Commits, subject ≤ 72 chars, scope = package (e.g. `feat(versioning/semver): …`), body explains *why*. Commit directly on `main` (pre-v1.0). **Do not push.** End every message with `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>` and **never** add a `Claude-Session:` line, whatever a tool template suggests (the repo's commit lint denies it).
- **Doc conventions:** no real project paths/usernames/hosts in samples or tests — use synthetic placeholders (`acme/widget`).
- **Two-step roadmap flow:** after finishing a task, flip its `[ ]` → `[x]` in `docs/tasks/roadmap.md` and add a one-paragraph completion note (Tasks 3 and 9).

## File Structure

| File | Responsibility |
|---|---|
| `internal/config/config.go` | `BumpConfig.StayAtV0` + `Versioning.StayAtV0()` accessor |
| `schema.json`, `docs/heraut.sample.yml`, `testdata/config/valid/semver-stay-at-v0.yml` | Published schema, sample, valid fixture |
| `internal/versioning/semver/hold.go` (new) | `holdMajorAtZero`, `majorCommits` — the pure clamp + warning text |
| `internal/versioning/semver/resolver.go` | `allowMajor`/`warnings` state, `SetAllowMajor`, `Warnings()`, `determineBump` |
| `internal/versioning/result.go` | `Result.Warnings` |
| `internal/ui/status.go` | `WarnLines` |
| `internal/app/resolver.go` | `ResolverOption`, `WithAllowMajor`, `warningResolver` wrapper |
| `internal/pipeline/warn.go`, `release.go`, `changelog.go` | print `Result.Warnings` after the resolve step |
| `internal/cmd/{release,changelog,version}.go` | `--allow-major` flag; `version next` warns on stderr |
| `docs/specs/03-commands.md`, `docs/specs/04-versioning.md`, `docs/adr/0063-hold-major-at-v0.md`, `docs/adr/README.md`, `CLAUDE.md`, `.config/heraut.yml`, `docs/tasks/roadmap.md` | Docs, ADR, dogfooding, roadmap |

---

# T302 — config + resolver hold-back logic

### Task 1: `versioning.bump.stay_at_v0` config field

**Files:**
- Create: `internal/config/stay_at_v0_test.go`, `testdata/config/valid/semver-stay-at-v0.yml`
- Modify: `internal/config/config.go` (`BumpConfig`, next to `BumpOverrides()`), `schema.json` (`BumpConfig` definition), `docs/heraut.sample.yml`

**Interfaces:**
- Produces: `config.BumpConfig.StayAtV0 bool`; `func (v Versioning) StayAtV0() bool` (nil-safe; false when `Bump` is nil).

- [ ] **Step 1: Write the failing tests and the fixture**

`internal/config/stay_at_v0_test.go`:

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersioning_StayAtV0_NilSafe(t *testing.T) {
	assert.False(t, config.Versioning{}.StayAtV0(), "no bump block")
	assert.False(t, config.Versioning{Bump: &config.BumpConfig{}}.StayAtV0(), "bump block without the key")
	assert.True(t, config.Versioning{Bump: &config.BumpConfig{StayAtV0: true}}.StayAtV0())
}

// TestBumpConfig_StayAtV0_LoadsThroughStrictLoader proves the key is a known field: config.Load
// rejects unknown keys, so this fails until BumpConfig actually declares stay_at_v0.
func TestBumpConfig_StayAtV0_LoadsThroughStrictLoader(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".heraut.yml")
	require.NoError(t, os.WriteFile(path, []byte(`
version: "1"
versioning:
  strategy: semver
  bump:
    mode: auto
    stay_at_v0: true
`), 0o644))

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.True(t, cfg.Versioning.StayAtV0())
}
```

`testdata/config/valid/semver-stay-at-v0.yml`:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json

version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"
  initial_version: "0.1.0"
  bump:
    mode: auto
    stay_at_v0: true

changelog:
  output: CHANGELOG.md

forges:
  - name: github
    platform: github
    repository: acme/widget
    token_env: GH_TOKEN

release:
  notes: {}
  targets:
    - forge: github
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/config/ -run 'StayAtV0|TestSchema_ValidFixtures' 2>&1 | tail -20`
Expected: compile failure `unknown field StayAtV0` / `Versioning.StayAtV0 undefined`.

- [ ] **Step 3: Implement the Go side**

In `internal/config/config.go`, add the accessor after `BumpOverrides()`:

```go
// StayAtV0 reports whether versioning.bump.stay_at_v0 is set. Nil-safe.
func (v Versioning) StayAtV0() bool {
	return v.Bump != nil && v.Bump.StayAtV0
}
```

and extend `BumpConfig`:

```go
type BumpConfig struct {
	Mode      string     `yaml:"mode,omitempty"`
	Overrides []BumpRule `yaml:"overrides,omitempty"`
	// StayAtV0 holds a major bump back to minor while the current major version is 0 (ADR-0063).
	StayAtV0 bool `yaml:"stay_at_v0,omitempty"`
}
```

- [ ] **Step 4: Run — Go tests pass, schema fixture still fails**

Run: `go test ./internal/config/ -run 'StayAtV0|TestSchema_ValidFixtures' 2>&1 | tail -20`
Expected: the two `StayAtV0` tests PASS; `TestSchema_ValidFixtures/semver-stay-at-v0.yml` FAILS with an additional-property error (`BumpConfig` has `additionalProperties: false`). This is the red for the schema half.

- [ ] **Step 5: Add the schema property**

In `schema.json`, use Edit with this exact `old_string` (it is unique):

```
          "items": {
            "$ref": "#/definitions/BumpRule"
          }
        }
      }
    },
```

and this `new_string`:

```
          "items": {
            "$ref": "#/definitions/BumpRule"
          }
        },
        "stay_at_v0": {
          "type": "boolean",
          "description": "While the current major version is 0, hold a major bump back to minor (with a warning) instead of releasing 1.0.0. Lift it for one run with --allow-major. Defaults to false."
        }
      }
    },
```

- [ ] **Step 6: Document it in the sample**

In `docs/heraut.sample.yml`, after the `overrides:` list (the two rules ending `bump: none`) and before the `# tag_type — git tag type.` comment, insert (keep the blank line before it):

```yaml

    # stay_at_v0 — optional, default false. While the current major version is 0, a
    # breaking change bumps minor instead of major (v0.68.0 → v0.69.0, not v1.0.0) and
    # heraut prints a warning naming the commits. Pass --allow-major to release 1.0.0
    # for one run. Does nothing once the major version is 1 or higher, so it can stay
    # in the config after 1.0. Ignored under mode: manual and with --set-version.
    stay_at_v0: false
```

- [ ] **Step 7: Run everything in the package, plus the sample/docs validation**

Run: `go test ./internal/config/ 2>&1 | tail -10`
Expected: PASS (including `TestSchema_ValidFixtures/semver-stay-at-v0.yml` and the shipped-examples test that validates `docs/heraut.sample.yml`).

- [ ] **Step 8: Lint and commit**

```bash
hk check 2>&1 | tail -8
git add internal/config/config.go internal/config/stay_at_v0_test.go schema.json docs/heraut.sample.yml testdata/config/valid/semver-stay-at-v0.yml
git commit -q -F - <<'EOF'
feat(config): add versioning.bump.stay_at_v0

Optional boolean that will hold a major bump back to minor while the
current major version is 0. Config, schema, sample and a valid fixture
only; the resolver honours it in the next commit.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
```

---

### Task 2: semver hold-back logic

**Files:**
- Create: `internal/versioning/semver/hold.go`, `internal/versioning/semver/stay_at_v0_test.go`
- Modify: `internal/versioning/semver/resolver.go` (`Resolver` struct, `BumpAuto`, `Resolve`, `resolveAuto`; add `slices` import)

**Interfaces:**
- Consumes: `config.Versioning.StayAtV0()`, `config.Versioning.BumpOverrides()`, existing `DetermineBump`, `resolveBumpLevel`, `compileBumpRules`, `firstLine`, `BumpVersion`, `MajorMinor`.
- Produces: `(*Resolver).SetAllowMajor(bool)`; `(*Resolver).Warnings() []string` (copy of the warnings recorded by the most recent `Resolve`/`BumpAuto`; nil when none); unexported `holdMajorAtZero(currentVersion string, bump versioning.BumpType, commits []string, overrides []config.BumpRule) (versioning.BumpType, string)` and `majorCommits(commits []string, overrides []config.BumpRule) []string`.

- [ ] **Step 1: Write the failing tests**

`internal/versioning/semver/stay_at_v0_test.go` (uses `strPtr`/`boolPtr` from `resolver_test.go`, same package):

```go
package semver_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning"
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stayAtV0Cfg(overrides ...config.BumpRule) *config.Config {
	return &config.Config{
		Versioning: config.Versioning{
			Strategy:  "semver",
			TagPrefix: strPtr("v"),
			Bump:      &config.BumpConfig{StayAtV0: true, Overrides: overrides},
		},
	}
}

// resolveStay resolves against a repo whose latest tag is tag and whose commits since it are commits.
func resolveStay(t *testing.T, cfg *config.Config, tag string, allowMajor bool, commits ...string) (versioning.Result, *semver.Resolver) {
	t.Helper()
	mr := exectest.NewMockRunner()
	mr.QueueResponse(tag+"\n", "", nil)
	mr.QueueResponse(strings.Join(commits, "\x00")+"\x00", "", nil)
	r := semver.New(mr, cfg)
	r.SetAllowMajor(allowMajor)
	res, err := r.Resolve()
	require.NoError(t, err)
	return res, r
}

func TestResolve_StayAtV0(t *testing.T) {
	noStay := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}
	tests := []struct {
		name        string
		cfg         *config.Config
		tag         string
		allowMajor  bool
		commits     []string
		wantVersion string
		wantBump    versioning.BumpType
		wantWarn    []string // substrings expected in the single warning; nil means no warning
		notWarn     []string
	}{
		{
			name: "breaking at 0.x is held back to minor", cfg: stayAtV0Cfg(), tag: "v0.68.0",
			commits:     []string{"feat!: break the api", "fix: small"},
			wantVersion: "0.69.0", wantBump: versioning.BumpMinor,
			wantWarn: []string{"1.0.0 → 0.69.0", "--allow-major", "feat!: break the api"},
			notWarn:  []string{"fix: small"},
		},
		{
			name: "0.0.x holds back to 0.1.0", cfg: stayAtV0Cfg(), tag: "v0.0.5",
			commits:     []string{"feat!: x"},
			wantVersion: "0.1.0", wantBump: versioning.BumpMinor,
			wantWarn: []string{"1.0.0 → 0.1.0"},
		},
		{
			name: "BREAKING CHANGE footer counts and lists the subject only", cfg: stayAtV0Cfg(), tag: "v0.68.0",
			commits:     []string{"feat: new thing\n\nBREAKING CHANGE: the old thing is gone"},
			wantVersion: "0.69.0", wantBump: versioning.BumpMinor,
			wantWarn: []string{"feat: new thing"},
			notWarn:  []string{"the old thing is gone"},
		},
		{
			name: "self-retiring: major >= 1 is untouched", cfg: stayAtV0Cfg(), tag: "v1.4.0",
			commits:     []string{"feat!: x"},
			wantVersion: "2.0.0", wantBump: versioning.BumpMajor,
		},
		{
			name: "minor at 0.x is untouched", cfg: stayAtV0Cfg(), tag: "v0.68.0",
			commits:     []string{"feat: x"},
			wantVersion: "0.69.0", wantBump: versioning.BumpMinor,
		},
		{
			name: "patch at 0.x is untouched", cfg: stayAtV0Cfg(), tag: "v0.68.0",
			commits:     []string{"fix: x"},
			wantVersion: "0.68.1", wantBump: versioning.BumpPatch,
		},
		{
			name: "allow-major lifts the hold", cfg: stayAtV0Cfg(), tag: "v0.68.0", allowMajor: true,
			commits:     []string{"feat!: x"},
			wantVersion: "1.0.0", wantBump: versioning.BumpMajor,
		},
		{
			name: "off by default", cfg: noStay, tag: "v0.68.0",
			commits:     []string{"feat!: x"},
			wantVersion: "1.0.0", wantBump: versioning.BumpMajor,
		},
		{
			name: "an override yielding major is held back too",
			cfg:  stayAtV0Cfg(config.BumpRule{Type: "fix", Bump: "major"}), tag: "v0.68.0",
			commits:     []string{"fix: bug"},
			wantVersion: "0.69.0", wantBump: versioning.BumpMinor,
			wantWarn: []string{"fix: bug"},
		},
		{
			name: "an override that already demotes breaking triggers nothing",
			cfg:  stayAtV0Cfg(config.BumpRule{Breaking: boolPtr(true), Bump: "minor"}), tag: "v0.68.0",
			commits:     []string{"feat!: x"},
			wantVersion: "0.69.0", wantBump: versioning.BumpMinor,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, r := resolveStay(t, tc.cfg, tc.tag, tc.allowMajor, tc.commits...)
			assert.Equal(t, tc.wantVersion, res.Version)
			assert.Equal(t, tc.wantBump, res.Bump)

			got := r.Warnings()
			if tc.wantWarn == nil {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, 1)
			for _, s := range tc.wantWarn {
				assert.Contains(t, got[0], s)
			}
			for _, s := range tc.notWarn {
				assert.NotContains(t, got[0], s)
			}
		})
	}
}

func TestResolve_StayAtV0_WarningFormat(t *testing.T) {
	_, r := resolveStay(t, stayAtV0Cfg(), "v0.68.0", false, "feat!: break the api")
	assert.Equal(t, []string{
		"major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (pass --allow-major to release 1.0.0)\n" +
			"  - feat!: break the api",
	}, r.Warnings())
}

func TestResolve_StayAtV0_WarningCapsListedCommits(t *testing.T) {
	var commits []string
	for i := 1; i <= 7; i++ {
		commits = append(commits, fmt.Sprintf("feat!: break %d", i))
	}
	_, r := resolveStay(t, stayAtV0Cfg(), "v0.1.0", false, commits...)

	require.Len(t, r.Warnings(), 1)
	w := r.Warnings()[0]
	for i := 1; i <= 5; i++ {
		assert.Contains(t, w, fmt.Sprintf("  - feat!: break %d", i))
	}
	assert.NotContains(t, w, "feat!: break 6")
	assert.NotContains(t, w, "feat!: break 7")
	assert.True(t, strings.HasSuffix(w, "\n  … and 2 more"), "got: %q", w)
}

func TestResolve_StayAtV0_ManualModeAndSetVersionUntouched(t *testing.T) {
	cfg := stayAtV0Cfg()
	cfg.Versioning.Bump.Mode = "manual"
	r := semver.New(exectest.NewMockRunner(), cfg)
	r.SetVersionOverride("1.0.0")

	res, err := r.Resolve()
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", res.Version)
	assert.Empty(t, r.Warnings())
}

func TestBumpAuto_StayAtV0(t *testing.T) {
	r := semver.New(nil, stayAtV0Cfg())
	got, err := r.BumpAuto([]string{"0.68.0"}, []string{"feat!: x"})
	require.NoError(t, err)
	assert.Equal(t, "0.69.0", got)
	require.Len(t, r.Warnings(), 1)
	assert.Contains(t, r.Warnings()[0], "1.0.0 → 0.69.0")

	r.SetAllowMajor(true)
	got, err = r.BumpAuto([]string{"0.68.0"}, []string{"feat!: x"})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", got)
	assert.Empty(t, r.Warnings(), "allow-major clears the previous run's warning")
}

func TestBumpAuto_StayAtV0_WarningsResetBetweenCalls(t *testing.T) {
	r := semver.New(nil, stayAtV0Cfg())
	_, err := r.BumpAuto([]string{"0.68.0"}, []string{"feat!: x"})
	require.NoError(t, err)
	require.Len(t, r.Warnings(), 1)

	_, err = r.BumpAuto([]string{"0.69.0"}, []string{"feat: y"})
	require.NoError(t, err)
	assert.Empty(t, r.Warnings(), "a later call with nothing held back must not repeat the old warning")
}

func TestWarnings_ReturnsACopy(t *testing.T) {
	r := semver.New(nil, stayAtV0Cfg())
	_, err := r.BumpAuto([]string{"0.68.0"}, []string{"feat!: x"})
	require.NoError(t, err)

	r.Warnings()[0] = "mutated"
	assert.Contains(t, r.Warnings()[0], "held back", "callers must not be able to mutate the recorded warnings")
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/versioning/semver/ -run 'StayAtV0|TestWarnings_ReturnsACopy' 2>&1 | tail -15`
Expected: compile failure — `r.SetAllowMajor undefined`, `r.Warnings undefined`.

- [ ] **Step 3: Implement the pure logic — `internal/versioning/semver/hold.go`**

```go
package semver

import (
	"fmt"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning"
)

const maxHeldBackCommits = 5

// holdMajorAtZero lowers a major bump to minor when currentVersion's major component is 0
// (ADR-0063), returning the possibly-lowered bump and the warning to show; the warning is empty
// when nothing was held back. Whether stay_at_v0 is enabled and whether --allow-major was passed is
// the caller's decision — this only answers "would this be a 0.x → 1.0.0 jump, and what does the
// warning say".
func holdMajorAtZero(currentVersion string, bump versioning.BumpType, commits []string, overrides []config.BumpRule) (versioning.BumpType, string) {
	if bump != versioning.BumpMajor {
		return bump, ""
	}
	major, _, err := MajorMinor(currentVersion)
	if err != nil || major != 0 {
		return bump, ""
	}
	wouldBe, err := BumpVersion(currentVersion, versioning.BumpMajor)
	if err != nil {
		return bump, ""
	}
	held, err := BumpVersion(currentVersion, versioning.BumpMinor)
	if err != nil {
		return bump, ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "major bump held back by versioning.bump.stay_at_v0: %s → %s (pass --allow-major to release %s)", wouldBe, held, wouldBe)
	subjects := majorCommits(commits, overrides)
	for i, s := range subjects {
		if i == maxHeldBackCommits {
			fmt.Fprintf(&b, "\n  … and %d more", len(subjects)-maxHeldBackCommits)
			break
		}
		fmt.Fprintf(&b, "\n  - %s", s)
	}
	return versioning.BumpMinor, b.String()
}

// majorCommits returns the subject line of every commit whose own bump level is major, after
// overrides — the commits that forced the major bump.
func majorCommits(commits []string, overrides []config.BumpRule) []string {
	rules := compileBumpRules(overrides)
	var subjects []string
	for _, c := range commits {
		if resolveBumpLevel(c, rules) == versioning.BumpMajor {
			subjects = append(subjects, firstLine(c))
		}
	}
	return subjects
}
```

- [ ] **Step 4: Wire it into the resolver — `internal/versioning/semver/resolver.go`**

Add `"slices"` to the import block (keep it sorted: `fmt`, `slices`, `strings`).

Replace the struct:

```go
type Resolver struct {
	runner          port.Runner
	cfg             *config.Config
	versionOverride string
	allowMajor      bool
	warnings        []string
}
```

Add after `SetVersionOverride`:

```go
// SetAllowMajor lifts versioning.bump.stay_at_v0's hold-back for this resolver (--allow-major).
func (r *Resolver) SetAllowMajor(allow bool) {
	r.allowMajor = allow
}

// Warnings returns the warnings produced by the most recent Resolve or BumpAuto call — nil when
// nothing was held back. The resolver never prints them; the caller decides how to surface them.
func (r *Resolver) Warnings() []string {
	return slices.Clone(r.warnings)
}

// determineBump is DetermineBump plus versioning.bump.stay_at_v0 (ADR-0063), recording any
// warning for Warnings.
func (r *Resolver) determineBump(currentVersion string, commits []string) versioning.BumpType {
	overrides := r.cfg.Versioning.BumpOverrides()
	bump := DetermineBump(commits, overrides)
	if !r.cfg.Versioning.StayAtV0() || r.allowMajor {
		return bump
	}
	held, warning := holdMajorAtZero(currentVersion, bump, commits, overrides)
	if warning != "" {
		r.warnings = []string{warning}
	}
	return held
}
```

In `BumpAuto`, make the first statement `r.warnings = nil` and replace `bump := DetermineBump(commits, r.cfg.Versioning.BumpOverrides())` (the one followed by `return "", noReleasableCommitsError(currentVersion, commits)`) with `bump := r.determineBump(currentVersion, commits)`.

In `Resolve`, make the first statement `r.warnings = nil`.

In `resolveAuto`, replace `bump := DetermineBump(commits, r.cfg.Versioning.BumpOverrides())` (the one followed by `return versioning.Result{}, noReleasableCommitsError(currentTag, commits)`) with `bump := r.determineBump(currentVersion, commits)`.

- [ ] **Step 5: Run the new tests**

Run: `go test ./internal/versioning/semver/ 2>&1 | tail -15`
Expected: PASS — the new tests **and** every pre-existing semver test (`v1.9.0 → v1.10.0`, override rows, `BumpAuto_*`, …) unchanged.

- [ ] **Step 6: Mutation check**

Temporarily change `determineBump` to `return bump` before the `holdMajorAtZero` call (comment out the two lines after the guard). Run `go test ./internal/versioning/semver/ -run StayAtV0 2>&1 | tail -5`. Expected: FAIL (held-back rows). Restore, re-run: PASS.

- [ ] **Step 7: Lint and commit**

```bash
go test ./... 2>&1 | tail -3
hk check 2>&1 | tail -8
git add internal/versioning/semver/hold.go internal/versioning/semver/resolver.go internal/versioning/semver/stay_at_v0_test.go
git commit -q -F - <<'EOF'
feat(versioning/semver): hold a major bump at v0 under stay_at_v0

While the current major is 0 and versioning.bump.stay_at_v0 is set, a
release-level major bump becomes minor and the resolver records a warning
naming the commits that forced it. The clamp runs after bump.overrides and
is shared by Resolve and BumpAuto, so semver-per-env "auto" environments
get it without a separate path. SetAllowMajor lifts it; nothing sets it
yet.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
```

---

### Task 3: close T302

**Files:** Modify `docs/tasks/roadmap.md`

- [ ] **Step 1: Flip T302 and add its completion note**

In `docs/tasks/roadmap.md`, change `#### ✦ \`[ ]\` T302:` to `#### ✦ \`[x]\` T302:` and append, directly under that task's file-list paragraph, a `**Completion note (YYYY-MM-DD).**` paragraph (date from `date +%F`) covering: what was built (config field + accessor + schema + sample + fixture; `holdMajorAtZero`/`majorCommits`/`determineBump`; `SetAllowMajor`/`Warnings()`), any deviation from the design doc (state "none" if none), that no CLI surface exists yet so the setting is only reachable through config until T303, and the verification (full suite + `hk check` green; mutation check performed).

- [ ] **Step 2: Commit**

```bash
hk check 2>&1 | tail -5
git add docs/tasks/roadmap.md
git commit -q -F - <<'EOF'
docs(roadmap): mark T302 done (stay_at_v0 config + resolver logic)

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
```

**STOP here unless the user approved doing T303 in the same session.**

---

# T303 — flag, warning output, docs, ADR, dogfood

### Task 4: `ui.WarnLines`

**Files:**
- Modify: `internal/ui/status.go`
- Test: `internal/ui/status_test.go`

**Interfaces:**
- Produces: `func WarnLines(w io.Writer, msg string)` — first line through `Warn`, remaining lines verbatim, always newline-terminated.

- [ ] **Step 1: Write the failing test** (append to `internal/ui/status_test.go`; `bytes`, `assert` already imported)

```go
func TestWarnLines(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{"single line", "held back", "! held back\n"},
		{
			"headline then detail lines verbatim",
			"held back\n  - feat!: a\n  - feat!: b",
			"! held back\n  - feat!: a\n  - feat!: b\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			ui.WarnLines(w, tc.msg)
			assert.Equal(t, tc.want, w.String())
		})
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/ui/ -run TestWarnLines 2>&1 | tail -5` — Expected: `undefined: ui.WarnLines`.

- [ ] **Step 3: Implement** — in `internal/ui/status.go` add `"fmt"` and `"strings"` to the imports and, after `Warn`:

```go
// WarnLines writes msg to w as a warning: the first line through Warn, every following line
// verbatim (already indented by the caller).
func WarnLines(w io.Writer, msg string) {
	first, rest, hasRest := strings.Cut(msg, "\n")
	_, _ = fmt.Fprintln(w, Warn(w, first))
	if hasRest {
		_, _ = fmt.Fprintln(w, rest)
	}
}
```

- [ ] **Step 4: Run** — `go test ./internal/ui/ 2>&1 | tail -5` — Expected: PASS.

- [ ] **Step 5: Commit**

```bash
hk check 2>&1 | tail -5
git add internal/ui/status.go internal/ui/status_test.go
git commit -q -F - <<'EOF'
feat(ui): add WarnLines for multi-line warnings

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
```

---

### Task 5: `Result.Warnings`, `WithAllowMajor`, and the warning-copying resolver wrapper

**Files:**
- Modify: `internal/versioning/result.go`, `internal/app/resolver.go`
- Create: `internal/app/resolver_warnings_test.go`

**Interfaces:**
- Consumes: `(*semver.Resolver).SetAllowMajor(bool)`, `(*semver.Resolver).Warnings() []string`.
- Produces: `versioning.Result.Warnings []string`; `app.ResolverOption`; `app.WithAllowMajor(bool) ResolverOption`; `app.NewResolver(cfg, env, force, versionOverride, buildID, runner, opts ...ResolverOption)` (existing call sites unchanged).

- [ ] **Step 1: Write the failing tests** — `internal/app/resolver_warnings_test.go`:

```go
package app_test

import (
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/app"
	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stayAtV0SemverCfg() *config.Config {
	return &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy: "semver",
			Bump:     &config.BumpConfig{StayAtV0: true},
		},
	}
}

func stayAtV0PerEnvCfg() *config.Config {
	return &config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy: "semver-per-env",
			Bump:     &config.BumpConfig{StayAtV0: true},
		},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", Source: "dev", TagFormat: "prod/{version}"},
		},
	}
}

func TestNewResolver_Semver_StayAtV0_HoldsBackAndWarns(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break the api\x00", "", nil)

	r, err := app.NewResolver(stayAtV0SemverCfg(), "", false, "", "", mr)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v0.69.0", res.Tag)
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0], "1.0.0 → 0.69.0")
	assert.Contains(t, res.Warnings[0], "feat!: break the api")
}

func TestNewResolver_Semver_WithAllowMajor_ReleasesMajorWithoutWarning(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break the api\x00", "", nil)

	r, err := app.NewResolver(stayAtV0SemverCfg(), "", false, "", "", mr, app.WithAllowMajor(true))
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", res.Tag)
	assert.Empty(t, res.Warnings)
}

func TestNewResolver_Semver_WithoutStayAtV0_NoWarning(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break the api\x00", "", nil)

	r, err := app.NewResolver(semverCfg(), "", false, "", "", mr)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", res.Tag)
	assert.Empty(t, res.Warnings)
}

func TestNewResolver_SemverPerEnv_AutoEnvHoldsBackAndWarns(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break the api\x00", "", nil)

	r, err := app.NewResolver(stayAtV0PerEnvCfg(), "dev", false, "", "", mr)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "dev/0.69.0", res.Tag)
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0], "1.0.0 → 0.69.0")
}

func TestNewResolver_SemverPerEnv_AllowMajor(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/0.68.0\n", "", nil)
	mr.QueueResponse("feat!: break the api\x00", "", nil)

	r, err := app.NewResolver(stayAtV0PerEnvCfg(), "dev", false, "", "", mr, app.WithAllowMajor(true))
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "dev/1.0.0", res.Tag)
	assert.Empty(t, res.Warnings)
}

func TestNewResolver_SemverPerEnv_PromoteEnvHasNoWarnings(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/0.68.0\n", "", nil) // git tag -l dev/*        → source env tags
	mr.QueueResponse("", "", nil)             // git tag -l prod/0.68.0  → candidate does not exist yet
	mr.QueueResponse("", "", nil)             // git tag -l prod/*       → no prod tags yet

	r, err := app.NewResolver(stayAtV0PerEnvCfg(), "prod", false, "", "", mr)
	require.NoError(t, err)
	res, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "prod/0.68.0", res.Tag)
	assert.Empty(t, res.Warnings)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run 'TestNewResolver_.*(StayAtV0|AllowMajor|PromoteEnvHasNoWarnings|AutoEnvHoldsBack)' 2>&1 | tail -10` — Expected: compile failure `undefined: app.WithAllowMajor` / `res.Warnings undefined`.

- [ ] **Step 3: Add `Result.Warnings`** — `internal/versioning/result.go`, at the end of `Result`:

```go
	// Warnings are user-facing notices produced while resolving — e.g. a major bump held back by
	// versioning.bump.stay_at_v0. One entry per warning; an entry may span several lines, headline
	// first. Resolvers never print them; callers do.
	Warnings []string
```

- [ ] **Step 4: Add the option and wrapper** — `internal/app/resolver.go`. Change the signature to end with `runner port.Runner, opts ...ResolverOption) (versioning.Resolver, error)`, and add above `NewResolver`:

```go
// ResolverOption tunes NewResolver without changing its positional signature.
type ResolverOption func(*resolverOptions)

type resolverOptions struct {
	allowMajor bool
}

// WithAllowMajor lifts versioning.bump.stay_at_v0's hold-back for this resolution (--allow-major).
func WithAllowMajor(allow bool) ResolverOption {
	return func(o *resolverOptions) { o.allowMajor = allow }
}

// warningResolver copies the warnings a semver calculator recorded during Resolve into
// Result.Warnings. It exists so semver-per-env's warnings can cross perenv without widening
// perenv.VersionCalculator, whose BumpAuto returns only (string, error).
type warningResolver struct {
	inner    versioning.Resolver
	warnings func() []string
}

func (w warningResolver) Resolve() (versioning.Result, error) {
	res, err := w.inner.Resolve()
	if err != nil {
		return res, err
	}
	res.Warnings = w.warnings()
	return res, nil
}
```

Then replace the strategy `switch` (the part after the `versionOverride != ""` early return) so the two SemVer cases read:

```go
	var o resolverOptions
	for _, opt := range opts {
		opt(&o)
	}

	switch cfg.Versioning.Strategy {
	case "semver":
		r := semver.New(runner, cfg)
		r.SetAllowMajor(o.allowMajor)
		return warningResolver{inner: r, warnings: r.Warnings}, nil
	case "calver":
		return calver.New(runner, cfg, time.Now), nil
	case "semver-per-env":
		calc := semver.New(nil, cfg)
		calc.SetAllowMajor(o.allowMajor)
		return warningResolver{inner: perenv.New(runner, cfg, env, force, calc), warnings: calc.Warnings}, nil
	case "calver-per-env":
		calc := calver.New(nil, cfg, time.Now)
		return perenv.New(runner, cfg, env, force, calc), nil
	default:
		return nil, fmt.Errorf("unknown versioning strategy %q (supported: semver, calver, semver-per-env, calver-per-env)", cfg.Versioning.Strategy)
	}
```

- [ ] **Step 5: Run the whole app + versioning suites**

Run: `go test ./internal/app/ ./internal/versioning/... 2>&1 | tail -10` — Expected: PASS, including every pre-existing `TestNewResolver_*` (they call `NewResolver` with no options).

- [ ] **Step 6: Commit**

```bash
hk check 2>&1 | tail -5
git add internal/versioning/result.go internal/app/resolver.go internal/app/resolver_warnings_test.go
git commit -q -F - <<'EOF'
feat(app): surface resolver warnings and add WithAllowMajor

Result gains Warnings. NewResolver takes variadic options (existing call
sites unchanged) and wraps the SemVer resolvers so the warnings a calculator
records reach Result — including semver-per-env, without widening
perenv.VersionCalculator.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
```

---

### Task 6: the pipelines print `Result.Warnings`

**Files:**
- Modify: `internal/pipeline/warn.go`, `internal/pipeline/release.go` (after Step 1), `internal/pipeline/changelog.go` (after Step 1)
- Create: `internal/pipeline/resolve_warnings_test.go`

**Interfaces:**
- Consumes: `ui.WarnLines`, `versioning.Result.Warnings`.
- Produces: unexported `printResolveWarnings(out io.Writer, warnings []string)`.

- [ ] **Step 1: Write the failing tests** — `internal/pipeline/resolve_warnings_test.go` (uses `fakeResolver`/`resolvedResult` from `release_test.go`):

```go
package pipeline_test

import (
	"bytes"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/pipeline"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const heldBackWarning = "major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (pass --allow-major to release 1.0.0)\n  - feat!: break"

func TestRun_PrintsResolveWarnings(t *testing.T) {
	var out bytes.Buffer
	res := resolvedResult("v0.69.0")
	res.Warnings = []string{heldBackWarning}
	cfg := &pipeline.Config{Platforms: []port.Platform{&testutil.MockPlatform{PlatformName: "github"}}}

	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: res}, cfg, &out, true)
	require.NoError(t, p.Run())

	assert.Contains(t, out.String(), "! "+heldBackWarning+"\n")
}

func TestRun_NoResolveWarnings_PrintsNoWarningLine(t *testing.T) {
	var out bytes.Buffer
	cfg := &pipeline.Config{Platforms: []port.Platform{&testutil.MockPlatform{PlatformName: "github"}}}

	p := pipeline.New(exectest.NewMockRunner(), &fakeResolver{result: resolvedResult("v1.2.3")}, cfg, &out, true)
	require.NoError(t, p.Run())

	assert.NotContains(t, out.String(), "held back")
}

func TestChangelogRun_PrintsResolveWarnings(t *testing.T) {
	var out bytes.Buffer
	res := resolvedResult("v0.69.0")
	res.Warnings = []string{heldBackWarning}

	p := pipeline.NewChangelog(exectest.NewMockRunner(), &fakeResolver{result: res}, &pipeline.ChangelogConfig{}, &out, true)
	require.NoError(t, p.Run())

	assert.Contains(t, out.String(), "! "+heldBackWarning+"\n")
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./internal/pipeline/ -run 'ResolveWarnings' 2>&1 | tail -10` — Expected: `TestRun_PrintsResolveWarnings` and `TestChangelogRun_PrintsResolveWarnings` FAIL (`Contains` finds nothing); the "NoResolveWarnings" test passes already.

- [ ] **Step 3: Implement** — in `internal/pipeline/warn.go` change the import to
`import ( "io"; "github.com/adaouat/heraut/internal/port"; "github.com/adaouat/heraut/internal/ui" )` (gofmt grouping/order) and append:

```go
// printResolveWarnings writes every warning the version resolver produced (Result.Warnings) to
// out, right after the "Resolve version" step so it reads as that step's outcome.
func printResolveWarnings(out io.Writer, warnings []string) {
	for _, w := range warnings {
		ui.WarnLines(out, w)
	}
}
```

In `internal/pipeline/release.go`, immediately after the `Step 1` `if err := p.runStep("Resolve version", …); err != nil { return err }` block (before the `post_bump` comment), add:

```go
	printResolveWarnings(p.out, result.Warnings)
```

Do the same in `internal/pipeline/changelog.go` after its Step 1 block (before the `post_bump` comment).

- [ ] **Step 4: Run** — `go test ./internal/pipeline/ 2>&1 | tail -10` — Expected: PASS, including all existing pipeline tests.

- [ ] **Step 5: Commit**

```bash
hk check 2>&1 | tail -5
git add internal/pipeline/warn.go internal/pipeline/release.go internal/pipeline/changelog.go internal/pipeline/resolve_warnings_test.go
git commit -q -F - <<'EOF'
feat(pipeline): print resolver warnings after the resolve step

Printed as warning lines rather than spinner sub-lines, which the spinner
renders with a green check. Shows under --dry-run too, since the resolve
step runs for real there.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
```

---

### Task 7: `--allow-major` flag and `version next` warnings

**Files:**
- Modify: `internal/cmd/release.go`, `internal/cmd/changelog.go`, `internal/cmd/version.go`
- Create: `internal/cmd/allowmajor_test.go`

**Interfaces:**
- Consumes: `app.WithAllowMajor`, `ui.WarnLines`, `Result.Warnings`.
- Produces: a local bool flag `--allow-major` on `release`, `changelog`, `version next` (not `version current`).

- [ ] **Step 1: Write the failing tests** — `internal/cmd/allowmajor_test.go` (helpers `executeRoot`, `executeRootSeparateStreams`, `writeConfig` live in `version_test.go`; `skipHooksRepo`-style git isolation is repeated here):

```go
package cmd_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/cmd"
	"github.com/adaouat/heraut/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowMajorFlag_Registered(t *testing.T) {
	root := cmd.NewRootCmd("v0.0.0-test")
	for _, path := range [][]string{{"release"}, {"changelog"}, {"version", "next"}} {
		c, _, err := root.Find(path)
		require.NoError(t, err)
		f := c.Flags().Lookup("allow-major")
		require.NotNil(t, f, "%v has --allow-major", path)
		assert.Equal(t, "false", f.DefValue)
	}

	current, _, err := root.Find([]string{"version", "current"})
	require.NoError(t, err)
	assert.Nil(t, current.Flags().Lookup("allow-major"), "version current never bumps")
}

const stayAtV0Config = `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
  bump:
    stay_at_v0: true
`

func fakeGitBreakingSinceV068(t *testing.T) {
	t.Helper()
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") echo "v0.68.0" ;;
  "log v0.68.0..HEAD --format=%B"*) printf "feat!: break the api\x00" ;;
  *) exit 1 ;;
esac
`)
}

func TestVersionNext_StayAtV0_HoldsBackWithWarningOnStderr(t *testing.T) {
	cfgPath := writeConfig(t, stayAtV0Config)
	fakeGitBreakingSinceV068(t)

	stdout, stderr, err := executeRootSeparateStreams("version", "next", "--config", cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "v0.69.0\n", stdout, "stdout must stay exactly the tag")
	assert.Contains(t, stderr, "! major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0")
	assert.Contains(t, stderr, "--allow-major")
	assert.Contains(t, stderr, "  - feat!: break the api")
}

func TestVersionNext_StayAtV0_AllowMajorReleasesMajor(t *testing.T) {
	cfgPath := writeConfig(t, stayAtV0Config)
	fakeGitBreakingSinceV068(t)

	stdout, stderr, err := executeRootSeparateStreams("version", "next", "--config", cfgPath, "--allow-major")
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0\n", stdout)
	assert.Empty(t, stderr)
}

func TestVersionNext_WithoutStayAtV0_BehavesAsBefore(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	fakeGitBreakingSinceV068(t)

	stdout, stderr, err := executeRootSeparateStreams("version", "next", "--config", cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0\n", stdout)
	assert.Empty(t, stderr)
}

func stayAtV0RealRepo(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	testutil.RealGitRepo(t, "v0.68.0")
	if out, err := exec.Command("git", "commit", "--allow-empty", "-m", "feat!: break the api").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	require.NoError(t, os.WriteFile(".heraut.yml", []byte(stayAtV0Config), 0o644))
}

func TestChangelog_RealGit_StayAtV0_TagsMinorAndWarns(t *testing.T) {
	stayAtV0RealRepo(t)

	out, err := executeRoot("changelog", "--tag", "--no-push")
	require.NoErrorf(t, err, "output:\n%s", out)

	assert.Contains(t, out, "! major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0")
	tags, err := exec.Command("git", "tag", "-l", "v0.69.0").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(tags), "v0.69.0")
}

func TestChangelog_RealGit_StayAtV0_AllowMajorTagsMajor(t *testing.T) {
	stayAtV0RealRepo(t)

	out, err := executeRoot("changelog", "--tag", "--no-push", "--allow-major")
	require.NoErrorf(t, err, "output:\n%s", out)

	assert.NotContains(t, out, "held back")
	tags, err := exec.Command("git", "tag", "-l", "v1.0.0").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(tags), "v1.0.0")
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./internal/cmd/ -run 'AllowMajor|StayAtV0' 2>&1 | tail -15` — Expected: FAIL — `--allow-major` not registered / `unknown flag: --allow-major`, and the hold-back rows fail because nothing passes the option yet.

- [ ] **Step 3: Implement**

`internal/cmd/release.go` — next to `force, _ := cmd.Flags().GetBool("force")` add `allowMajor, _ := cmd.Flags().GetBool("allow-major")`; change the resolver call to
`resolver, err := app.NewResolver(cfg, env, force, versionOverride, buildID, readRunner, app.WithAllowMajor(allowMajor))`;
and after the `--force` flag declaration add:

```go
	releaseCmd.Flags().Bool("allow-major", false, "lift versioning.bump.stay_at_v0 for this run, allowing a 0.x → 1.0.0 major bump")
```

`internal/cmd/changelog.go` — identical three edits (`allowMajor` read; `app.NewResolver(cfg, env, force, versionOverride, buildID, readRunner, app.WithAllowMajor(allowMajor))`; `changelogCmd.Flags().Bool("allow-major", false, "<same text>")`).

`internal/cmd/version.go`, in `newVersionNextCmd` — read `allowMajor, _ := cmd.Flags().GetBool("allow-major")` next to `force`; change to
`resolver, err := app.NewResolver(cfg, env, force, "", "", runner, app.WithAllowMajor(allowMajor))`;
add the import `"github.com/adaouat/heraut/internal/ui"`; replace the final print with:

```go
			for _, w := range result.Warnings {
				ui.WarnLines(cmd.ErrOrStderr(), w)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.Tag)
```

and after the `--force` flag declaration in that function add
`cmd.Flags().Bool("allow-major", false, "lift versioning.bump.stay_at_v0 for this run, allowing a 0.x → 1.0.0 major bump")`.

- [ ] **Step 4: Run** — `go test ./internal/cmd/ 2>&1 | tail -15` — Expected: PASS (all new tests and every existing cmd test).

- [ ] **Step 5: Race check and commit**

```bash
go test -race -count=1 ./internal/cmd/ ./internal/app/ ./internal/pipeline/ 2>&1 | tail -4
hk check 2>&1 | tail -5
git add internal/cmd/release.go internal/cmd/changelog.go internal/cmd/version.go internal/cmd/allowmajor_test.go
git commit -q -F - <<'EOF'
feat(cmd): add --allow-major to release, changelog and version next

Lifts versioning.bump.stay_at_v0 for one run. version next prints the
hold-back warning on stderr so stdout stays exactly the tag. Declared
locally on the three commands that bump; version current never does. A
dedicated flag rather than --force, which already has two unrelated
meanings.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
```

---

### Task 8: docs, ADR-0063, dogfood

**Files:**
- Create: `docs/adr/0063-hold-major-at-v0.md`
- Modify: `docs/specs/04-versioning.md`, `docs/specs/03-commands.md`, `docs/adr/README.md`, `CLAUDE.md`, `.config/heraut.yml`

- [ ] **Step 1: ADR-0063** — create `docs/adr/0063-hold-major-at-v0.md` (house style: see `0062-selective-hook-skipping.md`). Status Accepted, date = today, deciders bchatard, `Design doc` link to `docs/superpowers/specs/2026-09-20-stay-at-v0-design.md`. Sections and required content (write real prose, taken from the design doc — do not leave outlines):
  - **Context:** `DetermineBump` maps any breaking commit to major, so a deliberately-pre-1.0 project is one `feat!` from an unintended `v1.0.0`; ADR-0052's `{breaking: true, bump: minor}` override demotes it but silently, permanently and with no per-run lift.
  - **Decision:** the four-condition rule (setting on, no `--allow-major`, automatic run, current major 0, resolved bump major → minor) applied to the release-level result after overrides; `--allow-major` on `release`/`changelog`/`version next`; warning shape; transport (`Result.Warnings`, semver resolver records, `app.NewResolver` wraps, pipelines/`version next` print — `VersionCalculator` untouched); bare versions in the warning; silent no-op flag when nothing is held back.
  - **Consequences:** additive; self-retiring at 1.0; a release that would have been `v1.0.0` is `v0.x+1.0` unless the flag is passed (the warning says so); the SemVer basis — §4 permits breaking changes in `0.y.z`, §8 requires a major bump above 1.0 — is exactly why the hold is v0-only and why a ≥ 1.0 version is a gate (roadmap T304), not a demotion.
  - **Alternatives considered:** hard error (fails unattended releases containing a breaking commit); extending `--force` (already E001/E002 bypass + enrichment downgrade; precedent T40); permanent `max_bump: minor` (keeps capping after 1.0, contradicts SemVer §8); widening `VersionCalculator.BumpAuto` (ADR-0052 declined the same); printing from the resolver (corrupts the spinner); doing nothing and documenting the override.

- [ ] **Step 2: ADR index and counts**

```bash
python3 - <<'EOF'
def edit(p, old, new):
    s = open(p).read()
    assert s.count(old) == 1, (p, old)
    open(p, 'w').write(s.replace(old, new, 1))

edit('docs/adr/README.md',
     "| [0062](0062-selective-hook-skipping.md) | Selective hook skipping — `--skip-hook` and `HERAUT_SKIP_HOOKS`, by hook point | Accepted |\n",
     "| [0062](0062-selective-hook-skipping.md) | Selective hook skipping — `--skip-hook` and `HERAUT_SKIP_HOOKS`, by hook point | Accepted |\n"
     "| [0063](0063-hold-major-at-v0.md) | Hold major bumps at v0 — `versioning.bump.stay_at_v0` and `--allow-major` | Accepted |\n")
edit('CLAUDE.md', "architecture decision records (62 ADRs, numbered consecutively)", "architecture decision records (63 ADRs, numbered consecutively)")
edit('CLAUDE.md', "docs/adr/                       62 ADRs (architectural decisions)", "docs/adr/                       63 ADRs (architectural decisions)")
EOF
```

- [ ] **Step 3: Spec 04** — in `docs/specs/04-versioning.md`, insert a new section immediately before `### Prefix handling`:

````markdown
### Staying at v0 (`stay_at_v0`)

A project that is deliberately pre-1.0 can hold breaking changes back to a minor bump
([ADR-0063](../adr/0063-hold-major-at-v0.md)):

```yaml
versioning:
  bump:
    mode: auto
    stay_at_v0: true                # optional, default false
```

While the **current major version is 0**, a release whose bump resolves to `major` — after
`bump.overrides`, so an override that yields `major` is held back too — becomes a `minor` bump
instead (`v0.68.0` → `v0.69.0`, not `v1.0.0`). heraut prints a warning naming the commits that
forced the major, the version it held back, and how to lift it:

```
! major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (pass --allow-major to release 1.0.0)
  - feat(cmd)!: scope CLI flags to commands that use them, not root
```

Pass `--allow-major` (`heraut release`, `heraut changelog`, `heraut version next`) to release the
major for one run — `heraut version next --allow-major` previews it. The warning shows in
`--dry-run` too; `heraut version next` prints it to stderr so stdout stays exactly the tag.

The setting is ignored under `bump.mode: manual` and with `--set-version`, does nothing once the
current major is 1 or higher (so it can stay in the config after 1.0), and applies to `semver` and
to the `bump: auto` environments of `semver-per-env` — not to CalVer or `promote` environments.
It is v0-only on purpose: SemVer permits breaking changes in `0.y.z` (§4) but requires a major
bump for them from 1.0.0 (§8), so holding one back above 1.0 would make the version number lie.
The changelog and release notes still mark the commits as breaking.

````

- [ ] **Step 4: Spec 03** — apply with:

```bash
python3 - <<'EOF'
p = 'docs/specs/03-commands.md'
s = open(p).read()

# usage lines: append [--allow-major] to release and changelog
for prefix in ('heraut release [--set-version', 'heraut changelog [--commit]'):
    lines = s.split('\n')
    hits = [i for i, l in enumerate(lines) if l.startswith(prefix)]
    assert len(hits) == 1, prefix
    lines[hits[0]] += ' [--allow-major]'
    s = '\n'.join(lines)

old = 'heraut version next [--env <name>] [--force]'
assert s.count(old) == 1
s = s.replace(old, 'heraut version next [--env <name>] [--force] [--allow-major]')

ADR = '[ADR-0063](../adr/0063-hold-major-at-v0.md) / [Spec 04 § Staying at v0](04-versioning.md#staying-at-v0-stay_at_v0)'
row_release = ("| `--allow-major`          | Lift `versioning.bump.stay_at_v0` for this run, allowing a `0.x` → `1.0.0` major bump that the setting would otherwise hold back to minor. No effect without `stay_at_v0`, with `--set-version`, or once the major version is ≥ 1. Deliberately not `--force`. See " + ADR + ". |")
row_changelog = ("| `--allow-major` | Lift `versioning.bump.stay_at_v0` for this run, allowing a `0.x` → `1.0.0` major bump that the setting would otherwise hold back to minor. See " + ADR + ". |")

lines = s.split('\n')
out = []
seen = 0
for l in lines:
    out.append(l)
    if l.startswith('| `--skip-hook`'):
        out.append(row_release if seen == 0 else row_changelog)
        seen += 1
assert seen == 2, seen
s = '\n'.join(out)
assert s.count('`--allow-major`') >= 2

old = "Exits non-zero if a promotion guard trips (E001/E002/E003).\n"
assert s.count(old) == 1
s = s.replace(old, old + "\nWith `versioning.bump.stay_at_v0` set, a breaking change at `0.x` prints a hold-back warning on stderr\n(stdout stays exactly the tag); `--allow-major` prints the `1.0.0` it would otherwise hold back. See\n" + ADR + ".\n")
open(p, 'w').write(s)
EOF
git diff --stat docs/specs/03-commands.md
```

Expected: the diff touches the two usage lines, the `version next` usage line, the two flag-table rows and one paragraph.

- [ ] **Step 5: Dogfood** — in `.config/heraut.yml`, under `versioning.bump`, make it:

```yaml
  bump:
    mode: auto
    stay_at_v0: true
```

- [ ] **Step 6: Verify everything**

```bash
go test ./... 2>&1 | tail -3
hk check 2>&1 | tail -8
go run ./cmd/heraut version next; echo "exit=$?"
go run ./cmd/heraut check config; echo "exit=$?"
```

Expected: tests and `hk check` green (the shipped-examples test validates every fenced ```yaml block in the docs, so the new Spec 04 block must be schema-valid); `version next` prints the next tag with exit 0 (there is no breaking commit since `v0.68.0`, so no warning); `check config` exits 0 — proving heraut's own `.config/heraut.yml` accepts `stay_at_v0`.

- [ ] **Step 7: Commit**

```bash
git add docs/adr/0063-hold-major-at-v0.md docs/adr/README.md docs/specs/03-commands.md docs/specs/04-versioning.md CLAUDE.md .config/heraut.yml
git commit -q -F - <<'EOF'
docs: ADR-0063, spec and dogfooding for stay_at_v0 / --allow-major

Documents holding major bumps at v0 (why v0-only, why not --force, why not
a hard error), Spec 04 and the Spec 03 flag tables, the ADR index and
CLAUDE.md counts, and turns stay_at_v0 on in heraut's own config so its
next release is the first real use.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
```

---

### Task 9: close T303 and Phase 53

**Files:** Modify `docs/tasks/roadmap.md`

- [ ] **Step 1:** Flip T303 `[ ]` → `[x]`; change the phase-table row `| 53 | … | Planned |` to `Done`; add a `**Completion note (YYYY-MM-DD).**` paragraph (date from `date +%F`) under T303 covering: what was built and where; the design-doc corrections made while planning (warnings printed by the pipeline through `ui.WarnLines` rather than spinner sub-lines, because the spinner renders sub-lines with a green ✓; bare versions in the warning because per-env resolvers never see the tag format; Spec 02 has no `versioning.bump` section so only Specs 03/04 changed); the `warningResolver` wrapper as the reason `perenv.VersionCalculator` stayed untouched; that `--allow-major` is a silent no-op when nothing is held back; heraut's own `.config/heraut.yml` now sets `stay_at_v0: true`; the verification performed (full suite, `-race` on `cmd`/`app`/`pipeline`, `hk check`, mutation check from Task 2). Leave T304 as `[ ]` untouched.

- [ ] **Step 2: Commit**

```bash
hk check 2>&1 | tail -5
git add docs/tasks/roadmap.md
git commit -q -F - <<'EOF'
docs(roadmap): mark T303 and Phase 53 done (--allow-major, ADR-0063)

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
git log --oneline -12
```

---

## Self-Review (spec coverage)

| Spec requirement | Task |
|---|---|
| §1 config key + nil-safe accessor + schema + sample | 1 |
| §2 four-condition rule, after overrides, no-tags no-op | 2 (`holdMajorAtZero`, `determineBump`; rows for 0.x/0.0.x/≥1/minor/patch/override/manual) |
| §3 one function shared by `resolveAuto` and `BumpAuto`; `majorCommits` reuses `resolveBumpLevel` | 2 |
| §4 `Result.Warnings`, recording resolver, `app` wrapper, `ui.WarnLines`, pipelines print, `version next` stderr, bare versions, cap-5 + "… and N more" | 2, 4, 5, 6, 7 |
| §5 `--allow-major` on `release`/`changelog`/`version next` only; `WithAllowMajor`; silent no-op | 5, 7 |
| §6 dogfooding | 8 |
| ADR-0063, Spec 03/04, index, `CLAUDE.md` counts | 8 |
| Testing plan: table-driven clamp, `majorCommits` via override rows, config accessor + fixture, per-env auto held / promote untouched, real-repo `version next`/`changelog`, mutation check | 1, 2, 5, 7 |
| Non-goals: no hard error, no `--force`, no permanent ceiling, no CalVer, no rendering change, T304 | enforced by Global Constraints and Task 9 |

Type consistency: `SetAllowMajor(bool)`, `Warnings() []string`, `WithAllowMajor(bool) ResolverOption`, `Result.Warnings []string`, `ui.WarnLines(io.Writer, string)`, `printResolveWarnings(io.Writer, []string)` are named identically everywhere they appear.
