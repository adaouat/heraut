# SemVer v2 — Phase 1 (Compliance) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** heraut parses, orders, and emits versions strictly per SemVer 2.0.0 for the `semver` and
`semver-per-env` strategies — without minting any pre-release yet (that is Phase 2).

**Architecture:** A new strict `semver.Version` / `Parse` / `Compare` (§9–§11) plus a
`SortTags`/`Latest` pair become the single way SemVer-strategy code picks "the current tag".
Git still lists tags (its `--sort` flag stays in the call to avoid contract churn), but ordering is
decided in Go. `calver-per-env` shares `internal/versioning/perenv` and keeps its current lenient
path, because CalVer's zero-padded `2026.05.3` is not valid SemVer. Two clean config breaks land
here: `{build}` must follow `+`, and `--set-build-id` works on plain `semver`.

**Tech Stack:** Go (mise), cobra, testify, `github.com/adaouat/forge/exec/exectest` (`MockRunner`,
`FakeBin`), JSON Schema fixtures.

**Spec:** [`docs/superpowers/specs/2026-09-28-semver-v2-compliance-design.md`](../../docs/superpowers/specs/2026-09-28-semver-v2-compliance-design.md)
(Phase 1 = § Delivery → "Phase 1 — Compliance"; §3 `{build}` rule; §5 internals; §2
`--set-build-id` parity and `version current --include-pre-release`).

## Global Constraints

- Roadmap: dedicated `docs/tasks/semver-v2-roadmap.md`, task IDs **T324–T330**, main
  `roadmap.md` gets a **Phase 59** pointer block (no checkboxes there).
- New ADR: **ADR-0064** "SemVer v2 compliance and pre-release lifecycle" — written in Task 1,
  **before** any test row whose asserted behaviour deliberately changes (testing.md: "only after
  writing an ADR").
- TDD for every code task: failing test → run it red → implement → run it green → commit.
- Never delete a test row. Rows whose behaviour deliberately changes are **edited** (and say why in
  the commit body); typo fixes in existing tests go in their own `test:` commit.
- Error wrapping: every returned error wraps with `%w`; classify with `errors.Is`.
- Layering unchanged: `internal/cmd` never switches on strategy; `internal/app` may import
  `internal/versioning/*`; `internal/config` imports nothing from heraut.
- Config struct/validator changes update `schema.json` and `docs/heraut.sample.yml` in the same
  commit when they are affected.
- Conventional commits, subject ≤ 72 chars, scope = package. Breaking commits use `!`
  (heraut's own `.config/heraut.yml` has `stay_at_v0: true`, so `!` holds at a minor — precedent:
  `bad4968`, `366906b`).
- Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. **No**
  `Claude-Session:` trailer.
- Never pass `--no-verify`. Lint failures: `hk fix` / `hk fix -S golangci_lint`.
- No real data in fixtures/docs: synthetic tags/IDs only (`158404`, `acme/widget`, …).
- Commits land on `main` (pre-v1.0 trunk rule).
- Ignore `.claude/worktrees/` entirely (never read, edit, or grep-fix files there).

## Review Focus

1. **Build-metadata-only tags are releases.** `v1.4.0+158404` (plain semver, the new Task 6 output)
   must be the current release, not skipped — otherwise the next auto run re-releases `1.4.0`.
   Pinned in Task 3 (`TestResolve_BuildMetadataTagIsARelease`) and Task 4
   (`TestResolve_Promote_Semver_E002_BuildMetadataDestination`).
2. **Invalid-SemVer tags silently vanish.** `v1.02.0` used to pass `IsBareVersion`; it is now
   dropped, and if every tag is invalid the resolver falls back to `initial_version`. Expected:
   skipped consistently, like pre-releases today. Pinned in Task 3
   (`TestResolve_SkipsInvalidSemverTag`).
3. **CalVer through the shared per-env code.** `calver-per-env` must be byte-for-byte unaffected by
   strict parsing (zero-padded months). Pinned in Task 4 (`TestResolve_Auto_Calver_ZeroPaddedUnaffected`)
   plus the existing calver rows, which must stay green unmodified.
4. **Equal-precedence ties.** `v1.4.0+5` and `v1.4.0+6` compare equal; the pick must be
   deterministic (git's input order, stable sort). Pinned in Task 2 (`TestSortTags_TiesKeepInputOrder`).
5. **Oversized numeric identifiers.** `1.0.0-rc.99999999999999999999` must compare without
   overflow; an oversized MAJOR/MINOR/PATCH is an invalid version, not a panic. Pinned in Task 2.

---

## File map

| File | Task | Responsibility |
|------|------|----------------|
| `docs/tasks/semver-v2-roadmap.md` (new) | 1 | Task list + status for the epic |
| `docs/tasks/roadmap.md` | 1 | Phase 59 pointer block |
| `docs/adr/0064-semver-v2-compliance.md` (new), `docs/adr/README.md` | 1 | Decision record |
| `internal/versioning/semver/version.go` (new) | 2 | `Version`, `Parse`, `Compare`, `SortTags`, `Latest` |
| `internal/versioning/semver/version_test.go` (new) | 2 | §9–§11 unit tests |
| `internal/versioning/semver/resolver.go` | 3 | plain `semver` picks current tag via `SortTags`/`Latest` |
| `internal/versioning/perenv/order.go` (new), `auto.go`, `promote.go` | 4 | semver-per-env ordering; calver path unchanged |
| `docs/specs/04-versioning.md` | 4, 5 | § Pre-release tags rewrite; `{build}` examples |
| `internal/config/validator.go` | 5 | `{build}`-after-`+` rule (+ wizard) |
| `testdata/config/invalid/build_token_hyphen.yml` (new) | 5 | semantic-only fixture |
| `internal/app/resolver.go` | 6 | `--set-build-id` on plain semver |
| `internal/app/current.go`, `commit_check.go`, `internal/cmd/version.go` | 7 | final-by-default + `--include-pre-release` |

---

### Task 1 (T324): Roadmap, pointer, and ADR-0064

**Files:**
- Create: `docs/tasks/semver-v2-roadmap.md`
- Create: `docs/adr/0064-semver-v2-compliance.md`
- Modify: `docs/tasks/roadmap.md` (append Phase 59 block after the Phase 58 block, before the
  "Every Phase above marked `Done`…" paragraph)
- Modify: `docs/adr/README.md` (append row)

**Interfaces:** none (docs only). Produces the T-ids used by every later task.

- [ ] **Step 1: Create the dedicated roadmap**

`docs/tasks/semver-v2-roadmap.md`:

```markdown
# Héraut — SemVer v2 Roadmap

> Status: Active
> Design: [`docs/superpowers/specs/2026-09-28-semver-v2-compliance-design.md`](../superpowers/specs/2026-09-28-semver-v2-compliance-design.md)
> ADRs: new ADR-0064 ("SemVer v2 compliance and pre-release lifecycle" — written in T324)
> Main roadmap: tracked as Phase 59 in [`roadmap.md`](roadmap.md)

heraut implements the bare `MAJOR.MINOR.PATCH` subset of SemVer v2 only. This epic makes it
strictly compliant (full grammar, §11 precedence, `+`-only build metadata) and then adds a
pre-release lifecycle (`--pre-release <label>`) for the plain `semver` strategy.

## Conventions

- Task IDs **continue the global sequence** (`T324+`).
- This file is the **single source of truth** for task status: `[ ]` not started, `[x]` done.
  Two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD), then flip
  `[ ]` → `[x]` and add a one-paragraph completion note.
- **No real data** anywhere: synthetic placeholders only.
- Two deliberate clean breaks (ADR-0064): `{build}` must directly follow `+` in `tag_format`;
  `release.targets[].prerelease` is removed (Phase 2).
- The main `roadmap.md` Phase 59 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Task | Description | Status |
|------|-------------|--------|
| T324 | Roadmap, Phase 59 pointer, ADR-0064 | Not started |
| T325 | `semver.Version`: strict Parse, §11 Compare, SortTags/Latest | Not started |
| T326 | Plain `semver` resolver orders tags by §11 in Go | Not started |
| T327 | `semver-per-env` ordering + E002 via §11; calver-per-env unchanged (zero-padded CalVer is not SemVer) | Not started |
| T328 | `{build}` must directly follow `+` (validator, wizard, docs) | Not started |
| T329 | `--set-build-id` on plain `semver` (`v1.4.0+<id>`) | Not started |
| T330 | `version current`: latest final by default, `--include-pre-release` (+ `${version}` test-typo fix, own commit) | Not started |
| —    | Phase 2 (pre-release lifecycle) — planned after Phase 1 lands | Not planned |

## Phase 1 — Compliance

### [ ] T324 — Roadmap, Phase 59 pointer, ADR-0064
### [ ] T325 — `semver.Version`: strict Parse, §11 Compare, SortTags/Latest
### [ ] T326 — Plain `semver` resolver orders tags by §11 in Go
### [ ] T327 — `semver-per-env` ordering + E002 via §11; calver-per-env unchanged

- CalVer versions like `2026.05.0` have leading zeros, which strict SemVer parsing rejects:
  `calver-per-env` shares `internal/versioning/perenv` and must keep its lenient
  dotted-integer path (`IsBareVersion`, `compareVersionStrings`) — guarded by
  `TestResolve_Auto_Calver_ZeroPaddedUnaffected` plus the existing calver rows, unmodified.

### [ ] T328 — `{build}` must directly follow `+`
### [ ] T329 — `--set-build-id` on plain `semver`
### [ ] T330 — `version current`: latest final by default, `--include-pre-release`

- Test typo: an existing test (`TestCurrentTag_SemverPerEnv`) has a `${version}` typo that only
  passed because `version current` never parsed tags. It gets fixed in its own `test:` commit,
  before the feature commit.

## Phase 2 — Pre-release lifecycle

Not yet broken down. Scope per the design doc § Delivery → Phase 2: `--pre-release <label>`,
series rules, `--allow-major` second trigger, `--set-version` SemVer validation, changelog skip and
notes ranges, GitHub-derived `--prerelease`, removal of `release.targets[].prerelease`.
```

- [ ] **Step 2: Append the Phase 59 pointer to `docs/tasks/roadmap.md`**

Insert after the end of the Phase 58 block (keep the `---` separator style used between phases):

```markdown
### Phase 59 — SemVer v2 compliance and pre-release lifecycle

heraut only understood bare `MAJOR.MINOR.PATCH`: pre-release and build-metadata tags were skipped,
tag order came from git's `version:refname` sort, the documented `{version}-{build}` form produced
SemVer *pre-releases*, and heraut could not mint a pre-release at all. Phase 1 makes heraut
strictly SemVer v2 compliant (strict parser, §11 comparator, `+`-only `{build}`, `--set-build-id`
on plain `semver`, `version current --include-pre-release`); Phase 2 adds `--pre-release <label>`
for plain `semver`. New ADR-0064. The task breakdown and live `[ ] / [x]` status live in a
dedicated roadmap:

→ **[SemVer v2 Roadmap](semver-v2-roadmap.md)** — T324+

Design: [`docs/superpowers/specs/2026-09-28-semver-v2-compliance-design.md`](../superpowers/specs/2026-09-28-semver-v2-compliance-design.md).

---
```

Also add a Phase 59 row to the roadmap's status table if Phase 58 has one (search
`Phase 58` in the table near the top and mirror its row format, status `In progress`).

- [ ] **Step 3: Write ADR-0064**

`docs/adr/0064-semver-v2-compliance.md` — same header shape as ADR-0063, sections Context /
Decision / Consequences / Alternatives considered. Content (write it out in full prose, not
bullets only):

- **Context**: the four gaps from the design doc § Problem (bare-only recognizer; no §11
  comparator, git sort misorders pre-releases; `{version}-{build}` renders a pre-release; no
  minting; static GitHub `prerelease` bool).
- **Decision**:
  1. Strict SemVer 2.0.0 parsing and §11 ordering in Go for `semver` and `semver-per-env`;
     `calver`/`calver-per-env` keep their own lenient path (zero-padded CalVer is not SemVer).
  2. A tag carrying only build metadata (`v1.4.0+5`) is a *release*; pre-release tags are never the
     bump base.
  3. `{build}` must directly follow `+` in any `tag_format` — clean break, no deprecation window
     (no known real config uses `-{build}`).
  4. `--set-build-id` on plain `semver` renders `<prefix><version>+<id>`, still requiring
     `--set-version`.
  5. `version current` prints the latest **final**; `--include-pre-release` prints the highest §11
     tag. `commit check --from-latest-tag` keeps "latest tag of any kind" semantics.
  6. Pre-release lifecycle (Phase 2): floating core computed from the last final; minor/patch
     escalation warns; major escalation errors unless `--allow-major`; global monotonicity per core
     (a new pre-release must out-rank every existing tag of that core); label via
     `--pre-release <label>` only; one-commit rule for pre-releases, none for promotion to final;
     `semver` strategy only; no `CHANGELOG.md` write for pre-releases; final notes span back to the
     last final; GitHub `--prerelease` derived from the version and `release.targets[].prerelease`
     removed.
- **Consequences**: test rows that asserted `-{build}` tags or git-sort ordering are rewritten
  (not deleted); invalid-SemVer tags (`v1.02.0`) are ignored where they were accepted; a
  continuous channel on a core is blocked once a higher label exists for it (Phase 2).
- **Alternatives considered**: locked pre-release core (rejected — ships features in patches);
  per-label monotonicity (rejected — needs a "line" concept); auto-incrementing build counter
  (rejected — §10 makes such tags indistinguishable); keep `{build}` free-form (rejected —
  documented non-compliance); pre-release minting in `semver-per-env` (deferred — environments are
  already its staging model).

Design doc link: `../superpowers/specs/2026-09-28-semver-v2-compliance-design.md`.

- [ ] **Step 4: Add the ADR index row**

Append to `docs/adr/README.md`:

```markdown
| [0064](0064-semver-v2-compliance.md) | SemVer v2 compliance and pre-release lifecycle | Accepted |
```

- [ ] **Step 5: Flip T324 and commit**

In `docs/tasks/semver-v2-roadmap.md`: `### [ ] T324` → `### [x] T324`, status `Done`, and add a
one-paragraph note under the heading (what was filed, that Phase 2 is intentionally unplanned).

```bash
git add docs/tasks/semver-v2-roadmap.md docs/tasks/roadmap.md docs/adr/0064-semver-v2-compliance.md docs/adr/README.md
git commit -m "docs: file SemVer v2 roadmap (T324-T330) and ADR-0064

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2 (T325): `semver.Version` — strict Parse, §11 Compare, SortTags/Latest

**Files:**
- Create: `internal/versioning/semver/version.go`
- Test: `internal/versioning/semver/version_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces (used by Tasks 3, 4, 7):
  ```go
  var ErrInvalidVersion error
  type Version struct { Major, Minor, Patch uint64; Pre, Build []string }
  func Parse(s string) (Version, error)        // errors wrap ErrInvalidVersion
  func (v Version) String() string            // canonical form, incl. -pre and +build
  func (v Version) Core() string              // "MAJOR.MINOR.PATCH"
  func (v Version) IsPreRelease() bool
  func Compare(a, b Version) int              // -1/0/+1, SemVer §11, build ignored
  type TagVersion struct { Tag string; Version Version }
  func SortTags(tags []string, extract func(tag string) (string, bool)) []TagVersion
  func Latest(sorted []TagVersion, includePreRelease bool) (TagVersion, bool)
  ```

- [ ] **Step 1: Write the failing tests**

`internal/versioning/semver/version_test.go`:

```go
package semver_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, s string) semver.Version {
	t.Helper()
	v, err := semver.Parse(s)
	require.NoError(t, err, "parsing %q", s)
	return v
}

func TestParse_Valid(t *testing.T) {
	tests := []struct {
		in    string
		want  semver.Version
	}{
		{"0.0.0", semver.Version{}},
		{"1.2.3", semver.Version{Major: 1, Minor: 2, Patch: 3}},
		{"1.0.0-alpha", semver.Version{Major: 1, Pre: []string{"alpha"}}},
		{"1.0.0-alpha.1", semver.Version{Major: 1, Pre: []string{"alpha", "1"}}},
		{"1.0.0-0.3.7", semver.Version{Major: 1, Pre: []string{"0", "3", "7"}}},
		{"1.0.0-x-y-z.--", semver.Version{Major: 1, Pre: []string{"x-y-z", "--"}}},
		{"1.0.0-alpha+001", semver.Version{Major: 1, Pre: []string{"alpha"}, Build: []string{"001"}}},
		{"1.0.0+20130313144700", semver.Version{Major: 1, Build: []string{"20130313144700"}}},
		{"1.0.0-beta+exp.sha.5114f85", semver.Version{Major: 1, Pre: []string{"beta"}, Build: []string{"exp", "sha", "5114f85"}}},
		{"1.0.0+21AF26D3----117B344092BD", semver.Version{Major: 1, Build: []string{"21AF26D3----117B344092BD"}}},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := semver.Parse(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.in, got.String(), "String must round-trip")
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	for _, in := range []string{
		"", "1", "1.2", "1.2.3.4", "v1.2.3", " 1.2.3", "-1.2.3",
		"01.2.3", "1.02.3", "1.2.03", // leading zeros in core
		"1.2.3-", "1.2.3+", "1.2.3-rc.1+", // empty pre-release / build
		"1.2.3-01", "1.2.3-rc.01", // leading zero in numeric pre-release identifier
		"1.2.3-a..b", "1.2.3+a..b", // empty identifier
		"1.2.3-a_b", "1.2.3+a+b", "1.2.3-ä", // illegal characters
		"99999999999999999999.0.0", // overflows uint64
	} {
		t.Run(in, func(t *testing.T) {
			_, err := semver.Parse(in)
			require.Error(t, err)
			assert.True(t, errors.Is(err, semver.ErrInvalidVersion), "want ErrInvalidVersion, got %v", err)
		})
	}
}

func TestVersion_CoreAndIsPreRelease(t *testing.T) {
	v := mustParse(t, "1.4.0-rc.2+158404")
	assert.Equal(t, "1.4.0", v.Core())
	assert.True(t, v.IsPreRelease())
	assert.False(t, mustParse(t, "1.4.0+158404").IsPreRelease(), "build metadata alone is a release")
}

// TestCompare_SpecChain is the SemVer 2.0.0 §11 example chain, plus heraut's hard-won rows.
func TestCompare_SpecChain(t *testing.T) {
	chain := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0",
		"1.9.0", "1.10.0", "2.0.0", "2.1.0", "2.1.1",
	}
	for i := 0; i+1 < len(chain); i++ {
		lo, hi := mustParse(t, chain[i]), mustParse(t, chain[i+1])
		assert.Equal(t, -1, semver.Compare(lo, hi), "%s < %s", chain[i], chain[i+1])
		assert.Equal(t, 1, semver.Compare(hi, lo), "%s > %s", chain[i+1], chain[i])
	}
}

func TestCompare_Cases(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"build metadata ignored", "1.0.0+1", "1.0.0+2", 0},
		{"build metadata ignored on pre-release", "1.0.0-rc.1+a", "1.0.0-rc.1", 0},
		{"numeric identifier below alphanumeric", "1.0.0-1", "1.0.0-a", -1},
		{"numeric identifiers compare numerically", "1.0.0-rc.2", "1.0.0-rc.11", -1},
		{"ASCII order: dev above alpha", "1.0.0-dev.1", "1.0.0-alpha.1", 1},
		{"longer identifier list wins", "1.0.0-rc", "1.0.0-rc.1", -1},
		{"oversized numeric identifiers do not overflow", "1.0.0-rc.99999999999999999999", "1.0.0-rc.9999", 1},
		{"equal", "1.2.3", "1.2.3", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, semver.Compare(mustParse(t, tc.a), mustParse(t, tc.b)))
		})
	}
}

func stripV(tag string) (string, bool) { return strings.CutPrefix(tag, "v") }

func TestSortTags_OrdersByPrecedenceAndDropsInvalid(t *testing.T) {
	got := semver.SortTags([]string{"v1.2.3", "v1.10.0", "notatag", "v1.02.0", "v1.10.0-rc.1", "x1.0.0"}, stripV)
	tags := make([]string, len(got))
	for i, tv := range got {
		tags[i] = tv.Tag
	}
	assert.Equal(t, []string{"v1.10.0", "v1.10.0-rc.1", "v1.2.3"}, tags)
}

func TestSortTags_TiesKeepInputOrder(t *testing.T) {
	got := semver.SortTags([]string{"v1.4.0+6", "v1.4.0+5"}, stripV)
	require.Len(t, got, 2)
	assert.Equal(t, "v1.4.0+6", got[0].Tag)
	assert.Equal(t, "v1.4.0+5", got[1].Tag)
}

func TestLatest(t *testing.T) {
	sorted := semver.SortTags([]string{"v1.4.0-rc.1", "v1.3.0"}, stripV)

	final, ok := semver.Latest(sorted, false)
	require.True(t, ok)
	assert.Equal(t, "v1.3.0", final.Tag)

	highest, ok := semver.Latest(sorted, true)
	require.True(t, ok)
	assert.Equal(t, "v1.4.0-rc.1", highest.Tag)

	_, ok = semver.Latest(semver.SortTags([]string{"v1.0.0-rc.1"}, stripV), false)
	assert.False(t, ok, "only pre-releases → no final")

	_, ok = semver.Latest(nil, true)
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/versioning/semver/ -run 'TestParse|TestVersion_|TestCompare|TestSortTags|TestLatest'`
Expected: FAIL — build errors `undefined: semver.Parse`, `semver.Version`, …

- [ ] **Step 3: Implement**

`internal/versioning/semver/version.go`:

```go
package semver

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ErrInvalidVersion is returned (wrapped) by Parse for any string that is not a SemVer 2.0.0
// version.
var ErrInvalidVersion = errors.New("invalid SemVer version")

// Version is a parsed SemVer 2.0.0 version (https://semver.org/spec/v2.0.0.html).
type Version struct {
	Major, Minor, Patch uint64
	// Pre holds the pre-release identifiers (§9); empty for a release.
	Pre []string
	// Build holds the build-metadata identifiers (§10); Compare ignores it.
	Build []string
}

// Parse parses s strictly per the SemVer 2.0.0 grammar. s must not carry a tag prefix.
func Parse(s string) (Version, error) {
	rest, build, hasBuild := strings.Cut(s, "+")
	core, pre, hasPre := strings.Cut(rest, "-")

	var v Version
	nums := strings.Split(core, ".")
	if len(nums) != 3 {
		return Version{}, fmt.Errorf("%w %q: expected MAJOR.MINOR.PATCH", ErrInvalidVersion, s)
	}
	fields := []*uint64{&v.Major, &v.Minor, &v.Patch}
	for i, n := range nums {
		if !isNumericIdentifier(n) {
			return Version{}, fmt.Errorf("%w %q: %q is not a number without leading zeros", ErrInvalidVersion, s, n)
		}
		val, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return Version{}, fmt.Errorf("%w %q: %w", ErrInvalidVersion, s, err)
		}
		*fields[i] = val
	}
	if hasPre {
		ids, err := splitIdentifiers(pre, true)
		if err != nil {
			return Version{}, fmt.Errorf("%w %q: pre-release: %w", ErrInvalidVersion, s, err)
		}
		v.Pre = ids
	}
	if hasBuild {
		ids, err := splitIdentifiers(build, false)
		if err != nil {
			return Version{}, fmt.Errorf("%w %q: build metadata: %w", ErrInvalidVersion, s, err)
		}
		v.Build = ids
	}
	return v, nil
}

// String renders v in canonical SemVer form, including pre-release and build metadata.
func (v Version) String() string {
	s := v.Core()
	if len(v.Pre) > 0 {
		s += "-" + strings.Join(v.Pre, ".")
	}
	if len(v.Build) > 0 {
		s += "+" + strings.Join(v.Build, ".")
	}
	return s
}

// Core returns MAJOR.MINOR.PATCH without pre-release or build metadata.
func (v Version) Core() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// IsPreRelease reports whether v carries pre-release identifiers. Build metadata alone does not
// make a pre-release: 1.4.0+5 is the 1.4.0 release.
func (v Version) IsPreRelease() bool { return len(v.Pre) > 0 }

// Compare orders a and b by SemVer §11 precedence: -1 if a < b, 0 if equal, +1 if a > b. Build
// metadata is ignored, so versions differing only in it compare equal.
func Compare(a, b Version) int {
	if c := cmp.Compare(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Patch, b.Patch); c != 0 {
		return c
	}
	switch {
	case len(a.Pre) == 0 && len(b.Pre) == 0:
		return 0
	case len(a.Pre) == 0:
		return 1
	case len(b.Pre) == 0:
		return -1
	}
	for i := 0; i < len(a.Pre) && i < len(b.Pre); i++ {
		if c := compareIdentifier(a.Pre[i], b.Pre[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a.Pre), len(b.Pre))
}

// compareIdentifier compares two pre-release identifiers. Numeric identifiers have no leading
// zeros (Parse guarantees it), so a longer digit string is the larger number — comparing by
// length then lexically avoids overflowing on arbitrarily long identifiers.
func compareIdentifier(a, b string) int {
	aNum, bNum := allDigits(a), allDigits(b)
	switch {
	case aNum && bNum:
		if c := cmp.Compare(len(a), len(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	case aNum:
		return -1
	case bNum:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// TagVersion pairs a git tag with the SemVer version parsed from it.
type TagVersion struct {
	Tag     string
	Version Version
}

// SortTags parses every tag through extract — which returns the tag's bare version string and
// whether the tag belongs to this scheme at all — and returns the valid SemVer ones, highest §11
// precedence first. Tags that don't extract or don't parse are dropped. The sort is stable, so
// tags of equal precedence (differing only in build metadata) keep their input order.
func SortTags(tags []string, extract func(tag string) (string, bool)) []TagVersion {
	out := make([]TagVersion, 0, len(tags))
	for _, tag := range tags {
		bare, ok := extract(tag)
		if !ok {
			continue
		}
		v, err := Parse(bare)
		if err != nil {
			continue
		}
		out = append(out, TagVersion{Tag: tag, Version: v})
	}
	slices.SortStableFunc(out, func(a, b TagVersion) int { return Compare(b.Version, a.Version) })
	return out
}

// Latest returns the first entry of sorted (as returned by SortTags) that is a release — or, with
// includePreRelease, the first entry of any kind. ok is false when none qualifies.
func Latest(sorted []TagVersion, includePreRelease bool) (TagVersion, bool) {
	for _, tv := range sorted {
		if includePreRelease || !tv.Version.IsPreRelease() {
			return tv, true
		}
	}
	return TagVersion{}, false
}

func isNumericIdentifier(s string) bool {
	return allDigits(s) && (len(s) == 1 || s[0] != '0')
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// splitIdentifiers splits a dot-separated pre-release or build-metadata string into identifiers,
// enforcing [0-9A-Za-z-]+ each. Numeric pre-release identifiers must not have leading zeros
// (§9); build metadata allows them (§10).
func splitIdentifiers(s string, rejectLeadingZeros bool) ([]string, error) {
	ids := strings.Split(s, ".")
	for _, id := range ids {
		if id == "" {
			return nil, errors.New("empty identifier")
		}
		for _, r := range id {
			if !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '-') {
				return nil, fmt.Errorf("identifier %q contains %q (allowed: [0-9A-Za-z-])", id, r)
			}
		}
		if rejectLeadingZeros && allDigits(id) && !isNumericIdentifier(id) {
			return nil, fmt.Errorf("numeric identifier %q has a leading zero", id)
		}
	}
	return ids, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/versioning/semver/`
Expected: PASS (all existing resolver/bump tests unaffected).

- [ ] **Step 5: Lint, flip T325, commit**

Run: `hk check` (fix with `hk fix -S golangci_lint` if needed).
Flip `### [ ] T325` → `[x]`, status `Done`, one-paragraph note.

```bash
git add internal/versioning/semver/version.go internal/versioning/semver/version_test.go docs/tasks/semver-v2-roadmap.md
git commit -m "feat(versioning/semver): add strict SemVer v2 Parse and Compare (T325)

Implements the SemVer 2.0.0 grammar (§9/§10) and §11 precedence, plus
SortTags/Latest so every SemVer-strategy caller picks its current tag
the same way. Numeric identifiers compare by length then lexically, so
arbitrarily long ones never overflow. Not wired in yet.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3 (T326): Plain `semver` resolver orders tags by §11 in Go

**Files:**
- Modify: `internal/versioning/semver/resolver.go:124-156` (`resolveAuto`)
- Test: `internal/versioning/semver/resolver_test.go` (append)

**Interfaces:**
- Consumes: `SortTags`, `Latest`, `Version.Core()` from Task 2.
- Produces: no new API. `Result.CurrentTag` is now the highest-precedence *release* tag (build
  metadata allowed); the bump base is its `Core()`.

- [ ] **Step 1: Write the failing tests** (append to `resolver_test.go`)

```go
// Tag order comes from SemVer §11 in Go, not from git's version:refname sort (ADR-0064).
func TestResolve_OrdersTagsBySemverPrecedence(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.2.3\nv1.10.0\nv1.9.0\n", "", nil) // deliberately out of order
	mr.QueueResponse("fix: a small fix\x00", "", nil)

	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}
	result, err := semver.New(mr, cfg).Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v1.10.0", result.CurrentTag)
	assert.Equal(t, "1.10.1", result.Version)
	require.Len(t, mr.Calls, 2)
	assert.Equal(t, []string{"log", "v1.10.0..HEAD", "--format=%B%x00"}, mr.Calls[1].Args)
}

// A tag carrying only build metadata is the release of its core (SemVer §10): it must be the
// bump base, not skipped — otherwise the next run would re-release 1.4.0.
func TestResolve_BuildMetadataTagIsARelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.0+158404\nv1.3.0\n", "", nil)
	mr.QueueResponse("fix: a small fix\x00", "", nil)

	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}
	result, err := semver.New(mr, cfg).Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v1.4.0+158404", result.CurrentTag)
	assert.Equal(t, "1.4.1", result.Version)
	assert.Equal(t, []string{"log", "v1.4.0+158404..HEAD", "--format=%B%x00"}, mr.Calls[1].Args)
}

// Tags that are not valid SemVer (leading zeros, four components) are ignored, like pre-releases.
// Before ADR-0064, "1.02.0" passed IsBareVersion and would have been the bump base.
func TestResolve_SkipsInvalidSemverTag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v2.0.0.1\nv1.02.0\nv1.2.3\n", "", nil)
	mr.QueueResponse("fix: a small fix\x00", "", nil)

	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}
	result, err := semver.New(mr, cfg).Resolve()
	require.NoError(t, err)

	assert.Equal(t, "v1.2.3", result.CurrentTag)
	assert.Equal(t, "1.2.4", result.Version)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/versioning/semver/ -run 'TestResolve_OrdersTags|TestResolve_BuildMetadata|TestResolve_SkipsInvalid'`
Expected: FAIL — `OrdersTags` gets `v1.2.3` as CurrentTag; `BuildMetadata` gets `v1.3.0`;
`SkipsInvalid` gets `v1.02.0`.

- [ ] **Step 3: Implement** — replace the tag-selection block in `resolveAuto` (lines 132–147):

```go
	// Order by SemVer §11 in Go rather than trusting git's version:refname sort, which misorders
	// pre-releases without versionsort.suffix (ADR-0064). Pre-release tags are never the bump
	// base; a build-metadata-only tag (v1.4.0+5) is the release of its core.
	sorted := SortTags(parseTags(stdout), func(tag string) (string, bool) {
		return strings.CutPrefix(tag, prefix)
	})
	latest, ok := Latest(sorted, false)
	if !ok {
		iv := r.initialVersion()
		return versioning.Result{
			Version: iv,
			Tag:     prefix + iv,
			Bump:    versioning.BumpNone,
		}, nil
	}
	currentTag, currentVersion := latest.Tag, latest.Version.Core()
```

and delete the now-duplicated `if currentTag == "" { … }` block that followed. Keep the git
`tag -l … --sort=-version:refname` call unchanged (contract tests assert it; the flag is now just a
harmless pre-sort).

Also update `IsBareVersion`'s doc comment (`bump.go:153-156`), since the plain resolver no longer
uses it:

```go
// IsBareVersion reports whether s is a dotted MAJOR.MINOR.PATCH of integers with no pre-release
// or build suffix. It is deliberately lenient (it accepts leading zeros), because
// calver-per-env's zero-padded versions (2026.05.3) go through it — SemVer strategies use Parse
// instead (ADR-0064).
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/versioning/semver/`
Expected: PASS, including the unchanged `TestResolve_SkipsPreReleaseTag` and
`TestResolve_AllTagsPreRelease_UsesInitialVersion`.

- [ ] **Step 5: Run the whole suite** (pipeline and cmd tests exercise the resolver end to end)

Run: `mise run test`
Expected: PASS. If a test fails because it relied on git order over §11 order, it is a
deliberately changed row: edit its expectation, and name it in the commit body.

- [ ] **Step 6: Lint, flip T326, commit**

```bash
git add internal/versioning/semver/resolver.go internal/versioning/semver/bump.go internal/versioning/semver/resolver_test.go docs/tasks/semver-v2-roadmap.md
git commit -m "feat(versioning/semver): order release tags by SemVer precedence (T326)

The current tag is now the highest SemVer §11 release, decided in Go
instead of by git's version:refname sort. Build-metadata-only tags
(v1.4.0+5) count as the release of their core; invalid SemVer tags
such as v1.02.0 are ignored like pre-releases (ADR-0064).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4 (T327): `semver-per-env` ordering + E002 via §11; calver-per-env unchanged

**Files:**
- Create: `internal/versioning/perenv/order.go`
- Modify: `internal/versioning/perenv/auto.go:27-44`, `internal/versioning/perenv/promote.go:147-229`
- Modify: `docs/specs/04-versioning.md` § Pre-release tags (lines 148–159)
- Test: `internal/versioning/perenv/resolver_test.go` (append)

**Interfaces:**
- Consumes: `semver.SortTags`, `semver.Latest`, `semver.Parse`, `semver.Compare`,
  `Version.Core()`, `Version.String()` (Task 2); existing `semver.IsBareVersion`,
  `tagfmt.ParseVersion`, `compareVersionStrings`.
- Produces (package-internal): `releaseTags`, `latestTag`, `compareVersions` in `order.go`.

- [ ] **Step 1: Write the failing tests** (append to `perenv/resolver_test.go`)

```go
// ---- SemVer §11 ordering (ADR-0064) ----

func semverPerEnvCfg() *config.Config {
	return &config.Config{
		Versioning: config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{
			"dev":  {Bump: "auto", TagFormat: "dev/{version}"},
			"prod": {Bump: "promote", TagFormat: "prod/{version}"},
		},
	}
}

func TestResolve_Auto_Semver_OrdersTagsBySemverPrecedence(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/1.2.3\ndev/1.10.0\n", "", nil) // out of order
	mr.QueueResponse("fix: bugfix\x00", "", nil)

	result, err := perenv.New(mr, semverPerEnvCfg(), "dev", false, semverCalc("0.1.0")).Resolve()
	require.NoError(t, err)
	assert.Equal(t, "dev/1.10.0", result.CurrentTag)
	assert.Equal(t, "1.10.1", result.Version)
}

func TestResolve_Promote_Semver_PicksHighestSourceRelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/1.2.3\ndev/1.10.0\n", "", nil) // source, out of order
	mr.QueueResponse("", "", nil)                         // candidate prod/1.10.0 does not exist
	mr.QueueResponse("", "", nil)                         // no dest tags

	result, err := perenv.New(mr, semverPerEnvCfg(), "prod", false, semverCalc("0.1.0")).Resolve()
	require.NoError(t, err)
	assert.Equal(t, "prod/1.10.0", result.Tag)
}

// Before ADR-0064, compareVersionStrings read "1.2.4+7" as 1.2.0 ("4+7" is not an int), so a
// destination ahead of the candidate slipped past E002.
func TestResolve_Promote_Semver_E002_BuildMetadataDestination(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/1.2.3\n", "", nil)
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("prod/1.2.4+7\n", "", nil)

	_, err := perenv.New(mr, semverPerEnvCfg(), "prod", false, semverCalc("0.1.0")).Resolve()
	require.Error(t, err)
	assert.True(t, errors.Is(err, perenv.ErrDestinationAhead), "got %v", err)
}

// The destination's latest tag is chosen by §11, not by git's first line.
func TestResolve_Promote_Semver_E002_MisorderedDestination(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/1.2.4\n", "", nil)
	mr.QueueResponse("", "", nil)
	mr.QueueResponse("prod/1.2.3\nprod/1.2.10\n", "", nil)

	_, err := perenv.New(mr, semverPerEnvCfg(), "prod", false, semverCalc("0.1.0")).Resolve()
	require.Error(t, err)
	assert.True(t, errors.Is(err, perenv.ErrDestinationAhead), "got %v", err)
}

// calver-per-env shares this package; its zero-padded versions are not valid SemVer and must keep
// resolving exactly as before.
func TestResolve_Auto_Calver_ZeroPaddedUnaffected(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("dev/2026.05.3\ndev/2026.05.2\n", "", nil)

	cfg := &config.Config{
		Versioning:   config.Versioning{Strategy: "calver-per-env", Format: "YYYY.MM.PATCH"},
		Environments: map[string]config.Environment{"dev": {Bump: "auto", TagFormat: "dev/{version}"}},
	}
	result, err := perenv.New(mr, cfg, "dev", false, calverCalc("YYYY.MM.PATCH", fixedNow(2026, time.May, 24))).Resolve()
	require.NoError(t, err)
	assert.Equal(t, "dev/2026.05.4", result.Tag)
	assert.Equal(t, "dev/2026.05.3", result.CurrentTag)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/versioning/perenv/ -run 'OrdersTags|PicksHighest|E002_BuildMetadata|E002_Misordered|ZeroPadded'`
Expected: the four semver tests FAIL (git-order picks / missed E002); `ZeroPaddedUnaffected`
PASSES already (it is a guard, and must keep passing after Step 3).

- [ ] **Step 3: Implement**

`internal/versioning/perenv/order.go`:

```go
package perenv

import (
	"github.com/adaouat/heraut/internal/versioning/semver"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

const semverPerEnv = "semver-per-env"

// releaseTags returns env's release tags — never pre-releases — newest first, each paired with the
// bare version handed to the VersionCalculator. semver-per-env orders them by SemVer §11 in Go and
// passes each version's MAJOR.MINOR.PATCH core (ADR-0064). calver-per-env keeps git's
// version:refname order and the lenient dotted-integer check, because CalVer's zero-padded
// components (2026.05.3) are not valid SemVer.
func releaseTags(strategy, tf string, rawTags []string) (tags, versions []string) {
	if strategy == semverPerEnv {
		for _, tv := range semver.SortTags(rawTags, versionExtractor(tf)) {
			if tv.Version.IsPreRelease() {
				continue
			}
			tags = append(tags, tv.Tag)
			versions = append(versions, tv.Version.Core())
		}
		return tags, versions
	}
	for _, tag := range rawTags {
		bare, err := tagfmt.ParseVersion(tf, tag)
		if err != nil || !semver.IsBareVersion(bare) {
			continue
		}
		tags = append(tags, tag)
		versions = append(versions, bare)
	}
	return tags, versions
}

// latestTag returns the destination env's latest tag and its bare version. ok is false when no
// version could be parsed from it, in which case E002 cannot apply. semver-per-env picks the
// highest §11 tag of any kind; calver-per-env keeps git's first line.
func latestTag(strategy, tf string, rawTags []string) (tag, version string, ok bool) {
	if strategy == semverPerEnv {
		if tv, found := semver.Latest(semver.SortTags(rawTags, versionExtractor(tf)), true); found {
			return tv.Tag, tv.Version.String(), true
		}
	}
	if len(rawTags) == 0 {
		return "", "", false
	}
	v, err := tagfmt.ParseVersion(tf, rawTags[0])
	return rawTags[0], v, err == nil && strategy != semverPerEnv
}

// compareVersions orders two bare versions: SemVer §11 for semver-per-env when both parse, else
// the dotted-integer comparison calver-per-env has always used.
func compareVersions(strategy, a, b string) int {
	if strategy == semverPerEnv {
		va, errA := semver.Parse(a)
		vb, errB := semver.Parse(b)
		if errA == nil && errB == nil {
			return semver.Compare(va, vb)
		}
	}
	return compareVersionStrings(a, b)
}

func versionExtractor(tf string) func(string) (string, bool) {
	return func(tag string) (string, bool) {
		v, err := tagfmt.ParseVersion(tf, tag)
		return v, err == nil
	}
}
```

Note on `latestTag`'s last line: for semver-per-env, falling through means no tag parsed as SemVer;
the first raw tag is still reported as `CurrentTag` (today's behaviour) but is not comparable.

In `auto.go`, replace lines 27–44 (the `rawTags` loop) with:

```go
	tags, bareVersions := releaseTags(cfg.Versioning.Strategy, tf, splitLines(stdout))
	var latestTag string
	if len(tags) > 0 {
		latestTag = tags[0]
	}
```

and drop the now-unused `semver` import from `auto.go`.

In `promote.go`, replace step 3 (lines 147–168) with:

```go
	// 3. Pick the latest source release (never a pre-release) — the same selection resolveAuto
	// uses (T92, ADR-0064).
	srcTags, srcVersions := releaseTags(cfg.Versioning.Strategy, srcTF, splitLines(stdout))
	if len(srcTags) == 0 {
		return versioning.Result{}, &PromotionError{
			sentinel: ErrNoSourceTags,
			srcEnv:   srcEnv,
			destEnv:  env,
			srcGlob:  srcGlob,
		}
	}
	latestSrcTag, candidateVersion := srcTags[0], srcVersions[0]
```

and replace step 6's selection/compare (lines 203–229) with:

```go
	currentDestTag, latestDestVersion, destComparable := latestTag(cfg.Versioning.Strategy, destTF, splitLines(stdout))
	if destComparable && compareVersions(cfg.Versioning.Strategy, latestDestVersion, candidateVersion) > 0 && !force {
		suggested, renderErr := tagfmt.Render(srcTF, tagfmt.Tokens{Env: srcEnv, Version: latestDestVersion})
		if renderErr != nil {
			// srcTF needing a {build} token can't render a suggested tag from a promoted version alone
			// — no build ID survives promotion to reuse. Fall back to a placeholder instead of leaving
			// the hint with an empty tag name.
			suggested = fmt.Sprintf("<no suggested tag — %s's tag_format needs a build ID>", srcEnv)
		}
		return versioning.Result{}, &PromotionError{
			sentinel:          ErrDestinationAhead,
			srcEnv:            srcEnv,
			destEnv:           env,
			srcTag:            latestSrcTag,
			candidateTag:      candidateTag,
			latestDestTag:     currentDestTag,
			latestDestVersion: latestDestVersion,
			suggestedSrcTag:   suggested,
		}
	}
```

Drop the `semver` import from `promote.go` if no longer used. Keep `compareVersionStrings` (still
the calver path).

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/versioning/perenv/`
Expected: PASS — new tests plus every existing row, in particular
`TestResolve_Auto_Semver_SkipsPrereleaseTag`, `TestResolve_Promote_SkipsPrereleaseSourceTag`, all
`promoteBackends` calver rows, and `TestResolve_Promote_E002_NoForce` (both backends), unmodified.

- [ ] **Step 5: Rewrite Spec 04 § Pre-release tags** (`docs/specs/04-versioning.md:148-159`)

Replace the section body with:

```markdown
### Pre-release tags

heraut parses tags strictly per SemVer 2.0.0 and orders them by SemVer §11 precedence in Go — it
does not depend on git's `version:refname` sort or on `versionsort.suffix` (ADR-0064). When
resolving the next version, the current tag is the highest-precedence **release**: pre-release
tags (`v1.3.0-rc.1`) are never the bump base, and a tag carrying only build metadata
(`v1.4.0+158404`) counts as the release of its core (`1.4.0`). Tags that are not valid SemVer
(`v1.02.0`, `v1.2.3.4`) are ignored. If no tag qualifies, heraut behaves as if no tags exist and
returns `initial_version`.

The same rules apply to `semver-per-env` (source and destination selection, and the E002
comparison). `calver-per-env` keeps its dotted-integer handling — zero-padded CalVer versions are
not SemVer.

heraut does not produce pre-release tags itself yet (planned — see the SemVer v2 roadmap).
```

- [ ] **Step 6: Full suite, lint, flip T327, commit**

Run: `mise run test` then `hk check`. Expected: PASS.

```bash
git add internal/versioning/perenv/ docs/specs/04-versioning.md docs/tasks/semver-v2-roadmap.md
git commit -m "feat(versioning/perenv): order semver-per-env tags by precedence (T327)

semver-per-env now picks source, auto and destination tags by SemVer
§11 and compares them for E002 with semver.Compare, fixing a missed
E002 when the destination carries build metadata (1.2.4+7 read as
1.2.0). calver-per-env keeps its lenient dotted-integer path, since
zero-padded CalVer is not SemVer. Spec 04 § Pre-release tags updated.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5 (T328): `{build}` must directly follow `+`

**Files:**
- Modify: `internal/config/validator.go` (new helper near `tagFormatMissingVersion` ~line 792;
  call sites in `Validate` before the per-env `switch` ~line 781 and inside `validatePerEnv`'s env
  loop ~line 854; `ValidateTagFormatForWizard` ~line 799)
- Create: `testdata/config/invalid/build_token_hyphen.yml`
- Modify tests: `internal/config/validator_test.go`, `internal/config/schema_test.go`
- Convert `-{build}` → `+{build}` (and matching tag literals `X-158404` → `X+158404`, globs
  `*-*` → `*+*`) in: `internal/cmd/{changelog,release,version_override,version,build_id_exit}_test.go`,
  `internal/app/{errors,current,resolver}_test.go`, `internal/config/tagformat_test.go`,
  `internal/versioning/perenv/resolver_test.go`, `internal/versioning/tagfmt/tagfmt_test.go`
- Modify: `internal/versioning/tagfmt/tagfmt.go:86` (doc-comment example)
- Modify docs: `schema.json` (both `tag_format` descriptions), `docs/specs/02-configuration.md`
  (`{build}` section ~line 308–320 and the table at line 95), `docs/specs/03-commands.md` (lines
  103, 279 examples), `docs/specs/04-versioning.md:305-308`, `docs/guides/mobile-ci-tagging.md`,
  `docs/guides/README.md` (if it quotes the `-{build}` form)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `config.ValidateTagFormatForWizard` additionally rejects a misplaced `{build}`; new
  validation error message contains `must directly follow "+"`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/validator_test.go`:

```go
// ADR-0064: {build} is SemVer build metadata only when it directly follows "+"; "-{build}" would
// make the build ID a pre-release identifier.
func TestValidate_BuildTokenPlacement(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		wantPath string // "" = no build-token error expected
	}{
		{
			name:     "per-env common + is valid",
			cfg:      perEnvCfgWithFormats("{env}/{version}+{build}", ""),
			wantPath: "",
		},
		{
			name:     "per-env common hyphen",
			cfg:      perEnvCfgWithFormats("{env}/{version}-{build}", ""),
			wantPath: "versioning.tag_format",
		},
		{
			name:     "per-env env override hyphen",
			cfg:      perEnvCfgWithFormats("{env}/{version}", "uat/{version}-{build}"),
			wantPath: "environments.uat.tag_format",
		},
		{
			name:     "build first",
			cfg:      perEnvCfgWithFormats("{build}/{env}/{version}", ""),
			wantPath: "versioning.tag_format",
		},
		{
			name:     "second occurrence misplaced",
			cfg:      perEnvCfgWithFormats("{env}/{version}+{build}.{build}", ""),
			wantPath: "versioning.tag_format",
		},
		{
			name: "plain semver hyphen",
			cfg: &config.Config{Version: "1", Versioning: config.Versioning{
				Strategy: "semver", TagFormat: "v{version}-{build}",
			}},
			wantPath: "versioning.tag_format",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := config.Validate(tc.cfg)
			var buildErrs []config.ValidationError
			for _, e := range errs {
				if strings.Contains(e.Message, `must directly follow "+"`) {
					buildErrs = append(buildErrs, e)
				}
			}
			if tc.wantPath == "" {
				assert.Empty(t, buildErrs)
				return
			}
			require.Len(t, buildErrs, 1, "errs: %v", errs)
			assert.Equal(t, tc.wantPath, buildErrs[0].Path)
			assert.Contains(t, buildErrs[0].Hint, "{version}+{build}")
		})
	}
}

func perEnvCfgWithFormats(common, uatOverride string) *config.Config {
	return &config.Config{
		Version:    "1",
		Versioning: config.Versioning{Strategy: "semver-per-env", TagFormat: common},
		Environments: map[string]config.Environment{
			"uat": {Bump: "auto", TagFormat: uatOverride},
		},
	}
}
```

(If `strings` is not yet imported in `validator_test.go`, add it.)

Add rows to `TestValidateTagFormatForWizard`'s table:

```go
		{"build after plus", "{env}/{version}+{build}", ""},
		{"build after hyphen", "{env}/{version}-{build}", `must directly follow "+"`},
```

Create `testdata/config/invalid/build_token_hyphen.yml`:

```yaml
version: "1"

versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}-{build}"

environments:
  uat:
    bump: auto
```

Add to `TestValidate_invalidFixtures`:

```go
		{
			fixture:     "../../testdata/config/invalid/build_token_hyphen.yml",
			wantPath:    "versioning.tag_format",
			wantMessage: `must directly follow "+"`,
		},
```

and add `"build_token_hyphen.yml"` to the list in `TestSchema_SemanticOnlyFixturesPassSchema`
(the schema cannot express the rule; semantic validation owns it).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/config/ -run 'BuildTokenPlacement|TagFormatForWizard|invalidFixtures|SemanticOnly'`
Expected: FAIL — no build-token errors are produced yet.

- [ ] **Step 3: Implement** in `internal/config/validator.go`

Next to `tagFormatMissingVersion`:

```go
// buildTokenMisplaced reports whether a tag_format uses {build} anywhere other than directly
// after "+" — the only placement that makes it SemVer build metadata (§10) rather than a
// pre-release identifier (ADR-0064). Shared with ValidateTagFormatForWizard.
func buildTokenMisplaced(s string) bool {
	const token = "{build}"
	for i := 0; ; {
		j := strings.Index(s[i:], token)
		if j < 0 {
			return false
		}
		pos := i + j
		if pos == 0 || s[pos-1] != '+' {
			return true
		}
		i = pos + len(token)
	}
}

func buildTokenError(path string) ValidationError {
	return ValidationError{
		Path:    path,
		Message: `{build} must directly follow "+" (SemVer build metadata)`,
		Hint:    `write "{version}+{build}", e.g. "{env}/{version}+{build}" — "-{build}" would make the build ID a pre-release identifier`,
	}
}
```

In `Validate`, just before `switch cfg.Versioning.Strategy { case "semver-per-env", "calver-per-env":`
(~line 781) — applies to every strategy, since `--set-build-id` renders `versioning.tag_format`
for any strategy:

```go
	if buildTokenMisplaced(cfg.Versioning.TagFormat) {
		errs = append(errs, buildTokenError("versioning.tag_format"))
	}
```

In `validatePerEnv`'s env loop, after the `tagFormatMissingVersion(env.TagFormat)` block:

```go
		if buildTokenMisplaced(env.TagFormat) {
			errs = append(errs, buildTokenError(envPath+".tag_format"))
		}
```

`ValidateTagFormatForWizard`:

```go
func ValidateTagFormatForWizard(s string) error {
	if tagFormatMissingVersion(s) {
		return errors.New("must contain {version}")
	}
	if buildTokenMisplaced(s) {
		return errors.New(`{build} must directly follow "+" — use "{version}+{build}"`)
	}
	return nil
}
```

Update the doc comment on `ValidateTagFormatForWizard` to mention both rules.

- [ ] **Step 4: Run the config tests**

Run: `go test ./internal/config/`
Expected: the new tests PASS. Rows elsewhere in the repo that load or validate a `-{build}` config
now fail — they are converted in the next step.

- [ ] **Step 5: Convert every `-{build}` test row**

In each file listed under **Files**, switch the separator only — keep every row:
`{version}-{build}` → `{version}+{build}`; tag literals `uat/7.4.1-158404` → `uat/7.4.1+158404`,
`main/7.4.1-rc.1-158404` → `main/7.4.1-rc.1+158404`; FakeBin glob cases `"tag -l main/*-* …"` →
`"tag -l main/*+* …"`. Rows that already use `+{build}` stay as they are. Then confirm nothing is
left:

Run: `rtk grep -n -- '-{build}' internal testdata`
Expected: no matches.

Update `tagfmt.go:86`'s example line to
`//	{env}/{version}+{build}  [uat/7.4.1+158404]   → [7.4.1]` and the trailing sentence to
"…handle SemVer pre-release segments (e.g. 7.4.1-rc.1) under the "+" build separator…".

- [ ] **Step 6: Run the whole suite**

Run: `mise run test`
Expected: PASS.

- [ ] **Step 7: Docs**

- `schema.json`: append to both `tag_format` descriptions: `{build} (CI build ID) must directly
  follow "+", e.g. "{env}/{version}+{build}" — SemVer build metadata.`
- `docs/specs/02-configuration.md` `{build}` section: example becomes
  `tag_format: "{env}/{version}+{build}"  # e.g. uat/7.4.1+158404`; add a sentence: "`{build}` must
  directly follow `+`: SemVer treats anything after `-` as a pre-release, which sorts *below* the
  release. heraut rejects any other placement with a config error (ADR-0064)."
- `docs/specs/03-commands.md` lines 103 and 279, `docs/specs/04-versioning.md:305-308`: switch
  examples to `+`.
- `docs/guides/mobile-ci-tagging.md`: switch every tag and the `tag_format` to `+` (e.g.
  `uat/7.4.0+154392`), and the `--bare` table rows (`main/7.4.1+159001` → `7.4.1`,
  `main/7.4.1-rc.1+159001` → `7.4.1-rc.1`, `7.4.1+159001` → `7.4.1`). Add one line near the
  `tag_format` explaining why `+`.
- `docs/guides/README.md`: fix any quoted `-{build}`.
- `docs/heraut.sample.yml`: if it shows `{build}`, use `+`; otherwise leave it.

Run: `rtk grep -rn -- '-{build}' docs schema.json` — only the design spec, ADR-0064 and archived
roadmap/plan history may still match (they describe the old form on purpose).

- [ ] **Step 8: Lint, flip T328, commit**

```bash
git add internal testdata schema.json docs/specs docs/guides docs/heraut.sample.yml docs/tasks/semver-v2-roadmap.md
git commit -m "feat(config)!: require {build} to directly follow + in tag_format (T328)

\"{version}-{build}\" rendered SemVer pre-releases (uat/7.4.1-158404
sorts below 7.4.1). {build} is now only valid as build metadata; any
other placement is a config error with a hint, also in heraut init's
live validation. Existing -{build} test rows are converted, not
removed (ADR-0064).

BREAKING CHANGE: tag_format values with {build} not directly preceded
by + are rejected; change \"-{build}\" to \"+{build}\".

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6 (T329): `--set-build-id` on plain `semver`

**Files:**
- Modify: `internal/app/resolver.go:97-133` (`NewResolver`'s override branch)
- Modify: `internal/cmd/version_override.go:33` (flag help)
- Modify: `docs/specs/03-commands.md` (lines 93 and 188 flag rows)
- Test: `internal/app/resolver_test.go`, `internal/cmd/version_override_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `app.NewResolver(cfg{Strategy: semver, no tag_format}, "", …, "1.4.0", "158404", …)`
  resolves to `Tag: "v1.4.0+158404"`, `Version: "1.4.0"`.

- [ ] **Step 1: Write the failing tests** (append to `internal/app/resolver_test.go`)

```go
// ADR-0064: plain semver has no tag_format, so --set-build-id appends SemVer build metadata.
func TestNewResolver_BuildID_PlainSemver_AppendsBuildMetadata(t *testing.T) {
	custom := "rel-"
	tests := []struct {
		name     string
		prefix   *string
		override string
		wantTag  string
	}{
		{"default prefix", nil, "1.4.0", "v1.4.0+158404"},
		{"prefixed override", nil, "v1.4.0", "v1.4.0+158404"},
		{"custom prefix", &custom, "rel-1.4.0", "rel-1.4.0+158404"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			cfg := semverCfg()
			cfg.Versioning.TagPrefix = tc.prefix
			r, err := app.NewResolver(cfg, "", false, tc.override, "158404", mr)
			require.NoError(t, err)

			result, err := r.Resolve()
			require.NoError(t, err)
			assert.Equal(t, tc.wantTag, result.Tag)
			assert.Equal(t, "1.4.0", result.Version)
			assert.Empty(t, mr.Calls, "static resolver must not call git")
		})
	}
}
```

Edit `TestNewResolver_BuildID_NoTagFormat` (behaviour deliberately changed for plain semver by
ADR-0064 — the error path still exists for CalVer): replace `semverCfg()` with `calverCfg()` and
add a one-line comment saying so. Keep the `"tag_format"` assertion.

Append to `internal/cmd/version_override_test.go` (use the file's existing `writeConfig` /
`executeRoot` helpers):

```go
func TestVersionNext_PlainSemver_SetBuildID(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
`)
	out, err := executeRoot("version", "next", "--config", cfgPath, "--set-version", "1.4.0", "--set-build-id", "158404")
	require.NoError(t, err)
	assert.Equal(t, "v1.4.0+158404\n", out)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ ./internal/cmd/ -run 'BuildID|SetBuildID'`
Expected: FAIL — `--set-build-id requires versioning.tag_format to contain a {build} token…`.

- [ ] **Step 3: Implement** in `internal/app/resolver.go`

Extract the prefix computation that currently sits in the `else` branch into a helper, and add the
plain-semver path at the top of the override branch:

```go
	if versionOverride != "" {
		if buildID != "" && cfg.Versioning.Strategy == "semver" && cfg.EffectiveTagFormat(env) == "" {
			// Plain semver has no tag_format to carry {build}: append the ID as SemVer build
			// metadata (ADR-0064).
			prefix := configuredTagPrefix(cfg)
			version := strings.TrimPrefix(versionOverride, prefix)
			return versioning.NewStaticResolver(prefix+version+"+"+buildID, version), nil
		}
		// … existing code, with the else branch now using configuredTagPrefix(cfg):
		// prefix := configuredTagPrefix(cfg)
```

```go
// configuredTagPrefix is versioning.tag_prefix when set, else the strategy's default.
func configuredTagPrefix(cfg *config.Config) string {
	if cfg.Versioning.TagPrefix != nil {
		return *cfg.Versioning.TagPrefix
	}
	return defaultTagPrefix(cfg.Versioning.Strategy)
}
```

Flag help in `internal/cmd/version_override.go:33`:

```go
	c.Flags().StringVar(buildID, "set-build-id", "", "build ID for the tag: the {build} token in tag_format, or +<id> build metadata under plain semver (requires --set-version)")
```

- [ ] **Step 4: Run tests**

Run: `mise run test`
Expected: PASS.

- [ ] **Step 5: Docs** — `docs/specs/03-commands.md` rows at lines 93 and 188:

`| --set-build-id | CI build ID for the tag. Per-env strategies: substituted into the {build} token of tag_format (which must be "+{build}"). Plain semver without tag_format: appended as SemVer build metadata (v1.4.0+158404). Requires --set-version. |`
(keep each table's existing column alignment style).

- [ ] **Step 6: Lint, flip T329, commit**

```bash
git add internal/app/resolver.go internal/app/resolver_test.go internal/cmd/version_override.go internal/cmd/version_override_test.go docs/specs/03-commands.md docs/tasks/semver-v2-roadmap.md
git commit -m "feat(app): support --set-build-id on plain semver as build metadata (T329)

Plain semver has no tag_format, so --set-build-id used to fail. It now
renders <prefix><version>+<id> (v1.4.0+158404), still requiring
--set-version like the per-env strategies. The no-tag_format error row
moves to a CalVer config, where that error still applies (ADR-0064).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7 (T330): `version current` — latest final by default, `--include-pre-release`

**Files:**
- Fix first (own commit): `internal/app/current_test.go:53` typo `prod/${version}` → `prod/{version}`
- Modify: `internal/app/current.go` (`CurrentTag`, `CurrentVersion` signatures + selection)
- Modify: `internal/app/commit_check.go:19` (pass `true`)
- Modify: `internal/cmd/version.go:89-139` (flag + pass-through)
- Modify: `docs/specs/03-commands.md` § `heraut version current` (lines 283–304)
- Test: `internal/app/current_test.go`, `internal/cmd/version_test.go`,
  `internal/app/commit_check_test.go` (only if it calls `CurrentTag` directly)

**Interfaces:**
- Consumes: `semver.SortTags`, `semver.Latest`, `Version.String()` (Task 2).
- Produces:
  ```go
  func CurrentTag(runner port.Runner, cfg *config.Config, env string, includePreRelease bool) (string, error)
  func CurrentVersion(runner port.Runner, cfg *config.Config, env string, includePreRelease bool) (string, error)
  ```
  `includePreRelease` has no effect for `calver` / `calver-per-env`.

- [ ] **Step 1: Fix the test typo in its own commit**

`TestCurrentTag_SemverPerEnv` uses `TagFormat: "prod/${version}"`, which only passed because
`CurrentTag` never parsed tags. Change it to `"prod/{version}"`.

Run: `go test ./internal/app/ -run TestCurrentTag_SemverPerEnv` — Expected: PASS.

```bash
git add internal/app/current_test.go
git commit -m "test(app): fix \${version} typo in per-env CurrentTag fixture

The tag_format had a stray \$, so the fixture could never parse a tag;
it only passed because CurrentTag returned git's first line unparsed.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 2: Write the failing tests**

In `internal/app/current_test.go`, add `false` as the new last argument to every existing
`app.CurrentTag(…)` / `app.CurrentVersion(…)` call, then append:

```go
func TestCurrentTag_Semver_SkipsPreReleaseByDefault(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.0-rc.1\nv1.3.0\n", "", nil)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}

	got, err := app.CurrentTag(mr, cfg, "", false)
	require.NoError(t, err)
	assert.Equal(t, "v1.3.0", got)
}

func TestCurrentTag_Semver_IncludePreRelease(t *testing.T) {
	tests := []struct {
		name, tags, want string
	}{
		{"pre-release above older final", "v1.4.0-rc.1\nv1.3.0\n", "v1.4.0-rc.1"},
		{"final above its own pre-release", "v1.4.0-rc.1\nv1.4.0\n", "v1.4.0"},
		{"numeric identifiers by value", "v1.4.0-rc.2\nv1.4.0-rc.10\n", "v1.4.0-rc.10"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.tags, "", nil)
			cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}

			got, err := app.CurrentTag(mr, cfg, "", true)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCurrentTag_SemverPerEnv_IncludePreRelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("prod/1.4.0-rc.2\nprod/1.4.0-rc.10\nprod/1.3.0\n", "", nil)
	cfg := &config.Config{
		Versioning:   config.Versioning{Strategy: "semver-per-env"},
		Environments: map[string]config.Environment{"prod": {TagFormat: "prod/{version}"}},
	}

	got, err := app.CurrentTag(mr, cfg, "prod", true)
	require.NoError(t, err)
	assert.Equal(t, "prod/1.4.0-rc.10", got)
}

func TestCurrentTag_Semver_OnlyPreReleases_HintsAtFlag(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.0.0-rc.1\n", "", nil)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}

	_, err := app.CurrentTag(mr, cfg, "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--include-pre-release")
}

func TestCurrentTag_Calver_IgnoresIncludePreRelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.05.1\n", "", nil)
	empty := ""
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", TagPrefix: &empty}}

	got, err := app.CurrentTag(mr, cfg, "", true)
	require.NoError(t, err)
	assert.Equal(t, "2026.05.1", got)
}

func TestCurrentVersion_Semver_IncludePreRelease_Bare(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("v1.4.0-rc.2\nv1.3.0\n", "", nil)
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "semver", TagPrefix: strPtr("v")}}

	got, err := app.CurrentVersion(mr, cfg, "", true)
	require.NoError(t, err)
	assert.Equal(t, "1.4.0-rc.2", got)
}
```

Append to `internal/cmd/version_test.go`:

```go
func TestVersionCurrent_IncludePreRelease(t *testing.T) {
	cfgPath := writeConfig(t, `
version: "1"
versioning:
  strategy: semver
  tag_prefix: "v"
`)
	exectest.FakeBin(t, "git", `#!/bin/sh
case "$*" in
  "tag -l v* --sort=-version:refname") printf "v1.4.0-rc.1\nv1.3.0\n" ;;
  *) exit 1 ;;
esac
`)
	out, err := executeRoot("version", "current", "--config", cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "v1.3.0\n", out)

	out, err = executeRoot("version", "current", "--config", cfgPath, "--include-pre-release")
	require.NoError(t, err)
	assert.Equal(t, "v1.4.0-rc.1\n", out)
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/app/ ./internal/cmd/ -run 'CurrentTag|CurrentVersion|VersionCurrent'`
Expected: FAIL — compile errors (too many arguments to `CurrentTag`) and unknown flag
`--include-pre-release`.

- [ ] **Step 4: Implement** `internal/app/current.go`

```go
// CurrentTag returns the latest existing git tag for the given strategy and environment. For
// SemVer strategies it is the highest SemVer §11 release, or — with includePreRelease — the
// highest tag including pre-releases (ADR-0064). CalVer strategies ignore includePreRelease and
// keep git's version:refname order. For single-env strategies, env is ignored; for per-env
// strategies, env is required.
func CurrentTag(runner port.Runner, cfg *config.Config, env string, includePreRelease bool) (string, error) {
	glob, err := currentTagGlob(cfg, env)
	if err != nil {
		return "", err
	}

	stdout, _, err := runner.Run("git", "tag", "-l", glob, "--sort=-version:refname")
	if err != nil {
		return "", fmt.Errorf("listing git tags: %w", err)
	}

	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}

	switch cfg.Versioning.Strategy {
	case "semver", "semver-per-env":
		sorted := semver.SortTags(lines, semverExtractor(cfg, env))
		if tv, ok := semver.Latest(sorted, includePreRelease); ok {
			return tv.Tag, nil
		}
		if len(sorted) > 0 {
			return "", fmt.Errorf("%w for %q: only pre-release tags exist (pass --include-pre-release to show them)", errNoTagsFound, glob)
		}
		return "", fmt.Errorf("%w for %q", errNoTagsFound, glob)
	default:
		if len(lines) > 0 {
			return lines[0], nil
		}
		return "", fmt.Errorf("%w for %q", errNoTagsFound, glob)
	}
}

// semverExtractor returns how to read a tag's bare version: strip tag_prefix for plain semver,
// parse through the effective tag_format for semver-per-env.
func semverExtractor(cfg *config.Config, env string) func(string) (string, bool) {
	if cfg.Versioning.Strategy == "semver-per-env" {
		tf := cfg.EffectiveTagFormat(env)
		return func(tag string) (string, bool) {
			v, err := tagfmt.ParseVersion(tf, tag)
			return v, err == nil
		}
	}
	prefix := configuredTagPrefix(cfg)
	return func(tag string) (string, bool) { return strings.CutPrefix(tag, prefix) }
}
```

(`configuredTagPrefix` is the helper added in Task 6, in the same package.)

`CurrentVersion` gains the same `includePreRelease bool` parameter and passes it to `CurrentTag`;
its body is otherwise unchanged. Add the `semver` import
(`github.com/adaouat/heraut/internal/versioning/semver`).

`internal/app/commit_check.go:19`: `tag, err := CurrentTag(runner, cfg, env, true)` — `commit check
--from-latest-tag` keeps "latest tag of any kind" semantics, now correctly ordered. Add a short
comment saying so.

`internal/cmd/version.go`, in `newVersionCurrentCmd`:

```go
	var bare, includePreRelease bool
	…
			value, err := out(runner, cfg, env, includePreRelease)
	…
	cmd.Flags().BoolVar(&includePreRelease, "include-pre-release", false, "print the highest SemVer tag including pre-releases (SemVer strategies only; default: latest release)")
```

- [ ] **Step 5: Run tests**

Run: `mise run test`
Expected: PASS. Existing `TestVersionCurrent_*` cmd tests pass unchanged (their tags are finals).

- [ ] **Step 6: Docs** — `docs/specs/03-commands.md` § `heraut version current`:

- usage line: `heraut version current [--env <name>] [--bare] [--include-pre-release] [--force]`
- replace "For single-env strategies, prints the latest tag overall." with: "For SemVer strategies
  (`semver`, `semver-per-env`), prints the highest-precedence **release** tag (SemVer §11, decided
  by heraut, not git's sort); pre-release tags are skipped. `--include-pre-release` prints the
  highest tag including pre-releases (`v1.4.0-rc.2`). CalVer strategies print the latest tag and
  ignore `--include-pre-release`."
- `--bare` examples switch to `main/7.4.1+158404` → `7.4.1` and `main/7.4.1-rc.1+158404` →
  `7.4.1-rc.1`.
- "Exits non-zero if no tags exist." → "Exits non-zero if no qualifying tag exists; when only
  pre-release tags exist, the error suggests `--include-pre-release`."

- [ ] **Step 7: Lint, flip T330, close Phase 1, commit**

Flip T330; update the "Progress at a glance" table; add a short "Phase 1 closed" note under the
Phase 1 heading of `docs/tasks/semver-v2-roadmap.md` (deviations from the design doc: `--sort` flag
kept in git calls as a harmless pre-sort; `IsBareVersion` kept for `calver-per-env`; build-metadata
tags count as releases — see Review Focus 1).

```bash
git add internal/app/current.go internal/app/current_test.go internal/app/commit_check.go internal/cmd/version.go internal/cmd/version_test.go docs/specs/03-commands.md docs/tasks/semver-v2-roadmap.md
git commit -m "feat(cmd): version current prints latest release, add --include-pre-release (T330)

For SemVer strategies version current now returns the highest SemVer
release, chosen in Go; --include-pre-release returns the highest tag
including pre-releases. CalVer is unchanged. commit check
--from-latest-tag keeps latest-tag-of-any-kind semantics (ADR-0064).

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8 (T331): Escape `+` in tag names inside every generated URL

Added mid-execution at the user's request: heraut used to reject `+` in tags, and Phase 1 now
produces `+` tags (T328's `{version}+{build}`, T329's `v1.4.0+<id>`). A `+` in a URL query string
means a space, and some servers/tools decode it that way in paths too. Every place heraut puts a
tag name into a URL must write `%2B` instead of `+`. Only `+` is escaped — `/` in per-env tags
(`uat/7.4.1`) stays literal, so every existing non-`+` URL is byte-for-byte unchanged.

Run order: execute right after Task 5 (T328), before Task 6.

**Files:**
- Modify: `internal/port/generator.go` (add `URLTag` next to `LinkContext`)
- Test: `internal/port/generator_test.go` (new)
- Modify: `internal/platforms/github/platform.go:36-57` (`ReleaseURL`, `ReleaseURLFromContext`)
- Modify: `internal/platforms/gitlab/platform.go:36-53` (same two)
- Modify: `internal/forge/github/github.go:43-45`, `internal/forge/gitlab/gitlab.go:43-45` (`CompareURL`)
- Modify: `internal/generators/native/links.go:33-44` (`buildCompareURL`, all three platforms)
- Tests: `internal/platforms/{github,gitlab}/platform_test.go`, `internal/forge/{github,gitlab}/*_test.go`,
  `internal/generators/native/render_internal_test.go`
- Modify: `docs/tasks/semver-v2-roadmap.md` (add T331 + T332 entries, flip T331)

**Interfaces:**
- Produces: `func URLTag(tag string) string` in package `port` — `+` → `%2B`, nothing else.
  `port` is the one package every URL builder (platforms, forge, generators) may import, and it
  already owns `LinkContext`, the link-building contract.

- [ ] **Step 1: Write the failing tests**

`internal/port/generator_test.go`:

```go
package port_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
)

func TestURLTag(t *testing.T) {
	tests := []struct{ in, want string }{
		{"v1.2.3", "v1.2.3"},
		{"v1.4.0+158404", "v1.4.0%2B158404"},
		{"uat/7.4.1+158404", "uat/7.4.1%2B158404"}, // "/" stays literal
		{"v1.4.0-rc.1+a+b", "v1.4.0-rc.1%2Ba%2Bb"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) { assert.Equal(t, tc.want, port.URLTag(tc.in)) })
	}
}
```

`internal/platforms/github/platform_test.go` (append):

```go
// Tags may carry SemVer build metadata ("+"), which must reach the URL as %2B (ADR-0064).
func TestReleaseURL_EscapesPlusInTag(t *testing.T) {
	p := github.New(exectest.NewMockRunner(), &config.Platform{Repository: "acme/widget"})
	assert.Equal(t, "https://github.com/acme/widget/releases/tag/v1.4.0%2B158404", p.ReleaseURL("v1.4.0+158404"))

	ambient := &port.LinkContext{BaseURL: "https://github.com/acme/widget", Platform: "github"}
	assert.Equal(t, "https://github.com/acme/widget/releases/tag/uat/7.4.1%2B158404", p.ReleaseURLFromContext("uat/7.4.1+158404", ambient))

	platform := &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"}
	assert.Equal(t, "https://github.com/acme/widget/releases/tag/v1.4.0%2B158404", p.ReleaseURLFromContext("v1.4.0+158404", platform))
}
```

`internal/platforms/gitlab/platform_test.go` (append):

```go
func TestReleaseURL_EscapesPlusInTag(t *testing.T) {
	t.Setenv("GITLAB_CI", "")
	p := gitlab.New(exectest.NewMockRunner(), &config.Platform{Project: "group/proj"})
	assert.Equal(t, "https://gitlab.com/group/proj/-/releases/v1.4.0%2B158404", p.ReleaseURL("v1.4.0+158404"))

	ambient := &port.LinkContext{BaseURL: "https://gitlab.com/group/proj", Platform: "gitlab"}
	assert.Equal(t, "https://gitlab.com/group/proj/-/releases/uat/7.4.1%2B158404", p.ReleaseURLFromContext("uat/7.4.1+158404", ambient))

	platform := &port.LinkContext{BaseURL: "https://gitlab.com", Owner: "group", Repo: "proj", Platform: "gitlab"}
	assert.Equal(t, "https://gitlab.com/group/proj/-/releases/v1.4.0%2B158404", p.ReleaseURLFromContext("v1.4.0+158404", platform))
}
```

`internal/forge/github/github_test.go`, inside `TestForge_Links` (add one assertion after the
existing `CompareURL` line):

```go
	assert.Equal(t, "https://github.com/acme/widget/compare/v1.0.0%2B1...v1.1.0%2B2", f.CompareURL("v1.0.0+1", "v1.1.0+2"))
```

`internal/forge/gitlab/gitlab_test.go`, inside `TestForge_Links`:

```go
	assert.Equal(t, "https://gitlab.example.com/group/subgroup/project/-/compare/v1.0.0%2B1...v1.1.0%2B2", f.CompareURL("v1.0.0+1", "v1.1.0+2"))
```

`internal/generators/native/render_internal_test.go`, add rows to the `buildCompareURL` table
(before the `ambient (no owner/repo)` row):

```go
		{
			name:    "github escapes + in tags",
			lc:      &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"},
			prev:    "v1.0.0+1",
			version: "v1.1.0+2",
			want:    "https://github.com/acme/widget/compare/v1.0.0%2B1..v1.1.0%2B2",
		},
		{
			name:    "gitlab escapes + in tags",
			lc:      &port.LinkContext{BaseURL: "https://gitlab.com", Owner: "group/sub", Repo: "proj", Platform: "gitlab"},
			prev:    "v1.0.0+1",
			version: "v1.1.0+2",
			want:    "https://gitlab.com/group/sub/proj/-/compare/v1.0.0%2B1..v1.1.0%2B2",
		},
		{
			name:    "azure_devops escapes + in query string",
			lc:      &port.LinkContext{BaseURL: "https://dev.azure.com", Owner: "org/proj", Repo: "repo", Platform: "azure_devops"},
			prev:    "v1.0.0+1",
			version: "v1.1.0+2",
			want:    "https://dev.azure.com/org/proj/_git/repo/branchCompare?baseVersion=GTv1.0.0%2B1&targetVersion=GTv1.1.0%2B2",
		},
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/port/ ./internal/platforms/... ./internal/forge/... ./internal/generators/native/ -run 'URLTag|EscapesPlus|TestForge_Links|BuildCompareURL|buildCompareURL'`
(if the native table test has a different name, find it with `grep -n "buildCompareURL(tc" internal/generators/native/*_test.go` and use that name).
Expected: FAIL — `undefined: port.URLTag`; URLs contain a literal `+`.

- [ ] **Step 3: Implement**

`internal/port/generator.go`, below `LinkContext`:

```go
// URLTag returns tag ready to embed in a URL path or query string. Only "+" is escaped (as
// %2B): SemVer build metadata puts "+" in tag names (ADR-0064), and a literal "+" reads as a
// space in query strings and on some servers' paths. "/" (per-env tags such as uat/7.4.1) is
// left literal so existing URLs are unchanged. url.PathEscape does not escape "+".
func URLTag(tag string) string { return strings.ReplaceAll(tag, "+", "%2B") }
```

(add `"strings"` to the imports).

Then wrap every tag at the five sites with `port.URLTag(...)`:

- `platforms/github/platform.go`: `ReleaseURL` → `…/releases/tag/%s", …, port.URLTag(tag))`;
  both `ReleaseURLFromContext` returns → `+ "/releases/tag/" + port.URLTag(tag)`.
- `platforms/gitlab/platform.go`: same for `/-/releases/`.
- `forge/github/github.go` / `forge/gitlab/gitlab.go` `CompareURL`: pass
  `port.URLTag(from), port.URLTag(to)`.
- `generators/native/links.go` `buildCompareURL`: compute `p, v := port.URLTag(prev), port.URLTag(version)`
  once after the nil/empty guard and use them in all three branches. Update the URL-shapes doc
  comment to say tags are passed through `port.URLTag`.

- [ ] **Step 4: Run tests**

Run: `mise run test` — Expected: PASS, and every pre-existing URL assertion unchanged.

- [ ] **Step 5: Roadmap entries**

In `docs/tasks/semver-v2-roadmap.md`, add two rows to "Progress at a glance" and two headings
under Phase 1:

```markdown
### [x] T331 — Escape `+` in tag names inside generated URLs

(completion note)

### [ ] T332 — Manual smoke test: `gh` / `glab` with a `+` tag

heraut passes tag names to `gh release create/upload` and `glab release create/upload` as argv;
how those CLIs encode `+` when they call their APIs cannot be checked offline (testing.md: no
network in tests). Before the first release that ships T328/T329, create a throwaway release with
a `v0.0.0+smoke` tag on a scratch GitHub repo and a scratch GitLab project (create + upload an
asset + open the printed release URL), then delete both. Record the outcome here.
```

Rows: `| T331 | Escape `+` in tag names inside generated URLs | Done |` and
`| T332 | Manual smoke test: gh/glab with a `+` tag | Not started |`.

- [ ] **Step 6: Lint, commit**

```bash
git add internal/port internal/platforms internal/forge internal/generators/native docs/tasks/semver-v2-roadmap.md
git commit -m "fix(port): escape + in tag names inside generated URLs (T331)

SemVer build metadata now puts + in tags (v1.4.0+158404). A literal +
reads as a space in query strings (Azure DevOps compare links) and on
some servers' paths. Release and compare URLs for GitHub, GitLab and
Azure DevOps now write %2B via port.URLTag; / stays literal so existing
URLs are unchanged. Adds T332, a manual gh/glab smoke test.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Deviations from the design doc (record in the roadmap notes)

- **`--sort=-version:refname` stays in the git calls.** The design says tags are "listed without
  relying on" it; ordering is now decided by `semver.Compare`, and keeping the flag avoids
  rewriting every `MockRunner`/`FakeBin` contract assertion for no behavioural gain.
- **`IsBareVersion` is kept, not replaced**, because `calver-per-env` shares `perenv` and its
  zero-padded versions are not SemVer. Its doc comment now says so.
- **Build-metadata-only tags are releases.** Design §5 said "no pre-release, no build metadata";
  that would make the Task 6 output (`v1.4.0+158404`) invisible to the next resolution and
  re-release `1.4.0`. SemVer §10 agrees they are the same release.
