# SemVer v2 — Phase 3 (maintenance branches) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** releasing from a declared maintenance branch (`release/1.3`) produces the next version
on that line (`v1.3.2`), with notes and changelog sections bounded by the branch's own history.
A commit that doesn't fit the line fails loudly, and changelog/notes bounds never use a tag
outside the bounded tag's history, on any branch.

**Architecture:** `internal/config` gains `versioning.branches` (`[]BranchRule`) plus pure range
parsing/derivation. `internal/app` detects the current branch (git, then CI variables), matches
it against the rules, and either sets a maintenance range on the semver resolver (inside
`NewResolver`, so `internal/cmd` never sees branch logic) or, for publishing commands, refuses an
unlisted branch. `internal/versioning/semver` gains a `Range` type and a maintenance mode in
`resolveAuto`, `resolvePreRelease` and `app.CurrentTag`. `internal/app.buildGenerator` always
restricts semver generators to tags reachable from HEAD, and native's `--regenerate` walk bounds
each section by its highest in-scope ancestor.

**Tech Stack:** Go, cobra, `github.com/adaouat/forge/exec/exectest.MockRunner`, real git in
`t.TempDir()` (pattern: `internal/app/tagorder_realrepo_internal_test.go`), `path.Match`.

**Spec:** `docs/superpowers/specs/2026-10-05-maintenance-branches-design.md` (approved
2026-10-06). Executors read the spec and this plan together.

## Decisions settled with the user (do not reopen)

From the spec: the semantic-release-style `versioning.branches` list; entry type by entry (range
→ maintenance; glob without range → maintenance with derived range; exact name without range →
release branch); ranges `N.x` / `N.N.x` only; unique explicit ranges; `semver` strategy only;
hard errors for out-of-range, tag collision and no in-range base; on an unlisted branch only
publishing (`release`, `changelog --tag`) is refused, `--force` bypasses it; previews keep today's
behaviour; `--set-version` is not range-checked, and a matched glob with no derivable range is not
an error under `--set-version`; pre-releases allowed on maintenance branches; history-aware
changelog/notes bounds for every semver repo, with or without `branches`; CalVer unchanged.

Clarifications made while planning (record each in the owning task's roadmap note):

- **`--set-version` collision guard:** `NewResolver`'s override path makes no git calls (it
  returns a `StaticResolver`). The spec's "collision guard still applies" is satisfied by the
  existing `git tag` failure when the tag exists. No new probe on that path.
- **`--dry-run`** on an unlisted branch is a preview, so it is not refused. Mirrors `CheckBranch`,
  which also runs only when `!dryRun`.
- **Per-section ancestry is scope-preserving:** a section's lower bound is the highest-precedence
  tag that is *both* in the scoped list and an ancestor of the section's tag. Only when no in-scope
  ancestor exists does today's oldest-in-scope fallback (unscoped ancestors) apply. Without this,
  under `semver-per-env` a `uat/1.3.0` ancestor could become `prod/1.3.0`'s bound.
- **Exit codes:** branch-rule errors raised while building the resolver (ambiguous match,
  underivable range) and the unlisted-branch refusal exit Config. Resolve-time errors (no in-range
  base, out of range, tag exists) exit Runtime through the existing `wrapRunErr` default.

## Global Constraints

- TDD: failing test first (capture the RED output), then implementation, then GREEN.
- Never delete or weaken a test row. A row whose behaviour deliberately changes is edited in place
  with a comment citing ADR-0065, and named in the commit body and the task's roadmap note.
- **No `branches` block → zero behaviour change in version resolution.** `NewResolver` must not
  call git for branch detection when `cfg.Versioning.Branches` is empty, so every existing
  `MockRunner` FIFO test passes unmodified.
- CalVer (`calver`, `calver-per-env`): zero behaviour change, no new git calls; every calver row
  unmodified.
- Linear-history semver repos: changelog and notes output byte-identical (the existing T334/T341
  real-repo tests must pass unmodified, apart from git call sequences in contract tests that gain
  `--merged HEAD`; those rows are edited, not deleted).
- Layering (`.claude/rules/coding.md`): `internal/config` imports nothing from heraut;
  `internal/versioning/semver` may import `config`; `internal/generators/native` must not import
  `internal/versioning`; `internal/cmd` never switches on strategy or branch type.
- Errors wrap with `%w`; new sentinels are checked with `errors.Is`, never string matching.
- When a field is added to `internal/config/config.go`, update `schema.json` and
  `docs/heraut.sample.yml` in the same commit.
- Conventional commits, subject ≤ 72 chars (**count it; no hook enforces length**), trailer
  `Co-Authored-By: Claude <implementing model name> <noreply@anthropic.com>`. Never a
  `Claude-Session:` line, never `--no-verify`.
- The `typos` hook rejects the token "m-i-s" (as in "m-i-s-sorted"): write "out of order".
- Code comments state the *why*. No session or review labels ("FIX-1", "review round",
  "previously/used to …"). ADR and T-ids are fine.
- The rtk shell hook can mangle some tools' flags: prefix with `rtk proxy` when a command
  misbehaves.
- Real-git tests: set `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_SYSTEM=/dev/null` plus author
  env vars (copy the `git` helper from `internal/app/tagorder_realrepo_internal_test.go`), and call
  `testutil.ClearCIEnv(t)` plus `t.Setenv` of `CI_COMMIT_BRANCH`, `GITHUB_REF_NAME`,
  `GITHUB_REF_TYPE`, `BUILD_SOURCEBRANCHNAME` to `""` so this repo's own CI can't leak a branch
  name into tests. Never use `printf '\x00'` in shell (dash on CI).
- Work on `main`; ignore `.claude/worktrees/`. `mise run test` before each commit (again after any
  lint auto-fix); `hk check` clean. Fix lint with `hk fix -S <linter>`, not the raw tool.
- No real data in fixtures: synthetic tags, branches, messages and repo names only.
- Each task flips its roadmap heading `[ ]` → `[x]` in `docs/tasks/semver-v2-roadmap.md`, sets its
  table row to Done, and adds a one-paragraph completion note in the same commit.

## Review Focus

1. **Non-default `tag_prefix` and build-metadata finals on a maintenance branch**
   (`tag_prefix: "rel-"`, base `rel-1.3.1+5`): the base listing uses `rel-*`, a build-metadata-only
   tag is the release of its core, and the next tag is `rel-1.3.2`. Pinned in Task 3.
2. **`semver-per-env` changelog under the ancestry rule:** `uat/1.3.0` sits on an earlier commit
   that is an ancestor of `prod/1.3.0`; with `TagGlob: prod/*` the `prod/1.3.0` section must still
   bound at `prod/1.2.0`, never at a `uat/` tag. Pinned in Task 4.
3. **Branch names that look close but must not match:** `release/*` must not match
   `release/1.3/hotfix` (`path.Match`'s `*` stops at `/`), so that branch is unlisted; and on
   GitHub a tag pipeline (`GITHUB_REF_TYPE=tag`) must not use `GITHUB_REF_NAME` as a branch.
   Pinned in Task 2.
4. **A pre-release of the next core already on the maintenance branch:** `v1.3.2-rc.1` exists on
   `release/1.3`, then a final run: the base is still `v1.3.1` (pre-releases are never the base),
   the result is `v1.3.2`, and the collision guard does not fire (only the exact tag counts).
   Pinned in Task 3.
5. **`--regenerate` on a maintenance branch:** `CHANGELOG.md` on `release/1.3` lists only
   branch-history tags (`v1.3.2`, `v1.3.1`, `v1.3.0`), never `v1.4.0` or `v2.0.0`. Pinned in
   Task 6.

---

## File map

| File | Change | Task |
|---|---|---|
| `internal/pipeline/release_test.go`, `internal/platforms/gitlab/platform_test.go` | synthetic fixture hosts | 0 |
| `internal/config/config.go`, `internal/config/branches.go` (new) | `BranchRule`, `BranchRange`, parse/derive | 1 |
| `internal/config/validator.go` | `validateBranches` | 1 |
| `schema.json`, `docs/heraut.sample.yml`, `testdata/config/{valid,invalid}/` | schema + fixtures | 1 |
| `internal/app/branchrules.go` (new) | `CurrentBranch`, `MatchBranchRule`, `CheckReleaseBranch`, sentinels | 2 |
| `internal/cmd/release.go`, `internal/cmd/changelog.go` | call `CheckReleaseBranch` | 2 |
| `internal/versioning/semver/maintenance.go` (new) | `Range`, sentinels, base/cap/collision helpers | 3 |
| `internal/versioning/semver/resolver.go` | `SetMaintenanceRange`, maintenance path in `resolveAuto` | 3 |
| `internal/app/resolver.go` | branch match → `SetMaintenanceRange` in `NewResolver` | 3 |
| `internal/app/pipeline.go` (`buildGenerator`), `internal/app/changelog_rotation.go` | reachable-only semver generators | 4 |
| `internal/generators/native/generator.go` | scoped-ancestor section bound | 4 |
| `internal/versioning/semver/prerelease.go`, `internal/app/current.go` | maintenance base for pre-releases and `version current` | 5 |
| `internal/app/maintenance_realrepo_internal_test.go` (new) | worked examples #1–#11 | 6 |
| `docs/adr/0065-branch-aware-semver-resolution.md` (new), `docs/adr/README.md`, `docs/adr/0064-…` | ADR | 7 |
| `docs/specs/02,03,04,05`, `docs/guides/maintenance-branches.md` (new) | docs | per task, 7 |
| `docs/tasks/semver-v2-roadmap.md`, `docs/tasks/roadmap.md` | notes, Phase 3 close, CalVer follow-up | each task, 7 |

---

### Task 0: Synthetic fixtures (part of T344's recorded scope)

**Files:** `internal/pipeline/release_test.go` (lines using `git.adaouat.dev/bchatard/ecom-poc-release`),
`internal/platforms/gitlab/platform_test.go` (lines 56–95).

- [ ] **Step 1:** replace `https://git.adaouat.dev` → `https://git.example.com`, owner `bchatard` →
  `acme`, repo `ecom-poc-release` → `widget` (project `bchatard/ecom-poc-release` →
  `acme/widget`). Only the substituted strings change; every assertion keeps its shape.
  Verify: `rtk proxy grep -rn "adaouat.dev\|ecom-poc\|bchatard" internal testdata` → no output.
- [ ] **Step 2:** `mise run test` → all pass.
- [ ] **Step 3:** commit `test: replace real host and project in fixtures` (body: per the
  no-real-data rule, recorded on T344).

---

### Task 1 (T349): `versioning.branches` config, range parsing, validation

**Files:**
- Create: `internal/config/branches.go`, `internal/config/branches_test.go`
- Modify: `internal/config/config.go` (`Versioning.Branches`), `internal/config/validator.go`
  (`validateBranches`, called from `Validate`), `schema.json`, `docs/heraut.sample.yml`
- Fixtures: `testdata/config/valid/semver_branches.yml`,
  `testdata/config/invalid/branches_{non_semver,bad_range,duplicate_range,bad_glob,empty_name}.yml`
  (each picked up by the existing schema and loader fixture tests; check how
  `testdata/config/invalid/targets_prerelease_removed.yml` is wired and follow it)
- Docs: Spec 02 (new `versioning.branches` reference subsection, with the entry-type table from
  spec §1); roadmap.

**Interfaces — Produces:**
```go
// BranchRule is one versioning.branches entry (ADR-0065).
type BranchRule struct {
    Name  string `yaml:"name"`
    Range string `yaml:"range,omitempty"`
}
// in Versioning:
Branches []BranchRule `yaml:"branches,omitempty"`

// BranchRange is a maintenance line: Minor == nil means N.x (>=N.0.0 <N+1.0.0, patch+minor);
// Minor set means N.M.x (>=N.M.0 <N.M+1.0, patch only).
type BranchRange struct {
    Major uint64
    Minor *uint64
}
func (r BranchRange) String() string            // "1.x" / "1.3.x"
func ParseBranchRange(s string) (BranchRange, error)       // accepts "N.x", "N.N.x" only
func DeriveBranchRange(branch string) (BranchRange, bool)  // last "/" segment: N.x, N.N.x, N.N, optional leading v/V
func (b BranchRule) IsGlob() bool                // strings.ContainsAny(b.Name, "*?[")
```

`ParseBranchRange` grammar: `^(0|[1-9]\d*)\.x$` or `^(0|[1-9]\d*)\.(0|[1-9]\d*)\.x$`.
`DeriveBranchRange` takes `branch[strings.LastIndex(branch, "/")+1:]`, strips one leading `v`/`V`,
then accepts `N.x`, `N.N.x` or `N.N` (`N.N` → `Minor` set). Anything else (including `N.N.N`,
e.g. `7.8.0`) → `false`.

**`validateBranches`** (only when `len(cfg.Versioning.Branches) > 0`):
- strategy != `semver` → `ValidationError{Path: "versioning.branches", Message: "only valid with
  strategy: semver (current strategy: <s>)", Hint: "per-env strategies tie branches to
  environments via environments.<env>.branch; remove versioning.branches"}`.
- each entry `i`: empty `name` → `versioning.branches[i].name: required`; `path.Match(name, "")`
  returning `path.ErrBadPattern` → `versioning.branches[i].name: invalid glob`; non-empty `range`
  failing `ParseBranchRange` → `versioning.branches[i].range: "<r>" is not a valid range` with
  hint `use N.x (patch and minor) or N.N.x (patch only), e.g. 1.x or 1.3.x`.
- duplicate explicit ranges (compare `BranchRange.String()`) →
  `versioning.branches[j].range: duplicates versioning.branches[i].range (<r>)`.

`schema.json`: under `versioning.properties`, add `branches` (array of `BranchRule`), and a
`BranchRule` definition `{type: object, additionalProperties: false, required: [name], properties:
{name: {type: string, minLength: 1, description}, range: {type: string, pattern:
"^(0|[1-9][0-9]*)\\.((0|[1-9][0-9]*)\\.)?x$", description}}}`.

`docs/heraut.sample.yml`: a commented `branches:` example under `versioning:` (main + `release/*`
+ one explicit range), with a one-line comment pointing to the guide.

- [ ] **Step 1: failing tests.** `branches_test.go` tables:
```go
// ParseBranchRange
{"1.x", "1.x", ""}, {"1.3.x", "1.3.x", ""}, {"0.x", "0.x", ""},
{"1.3", "", "not a valid range"}, {"1.3.0", "", …}, {"v1.x", "", …}, {"01.x", "", …}, {"x", "", …}, {"", "", …},
// DeriveBranchRange
{"release/1.3", "1.3.x", true}, {"release/1.3.x", "1.3.x", true}, {"release/1.x", "1.x", true},
{"release/v1.3", "1.3.x", true}, {"1.3.x", "1.3.x", true}, {"support/2.x", "2.x", true},
{"release/legacy", "", false}, {"release/7.8.0", "", false}, {"release/1.3/hotfix", "", false}, {"main", "", false},
// IsGlob
{"release/*", true}, {"release/1.?", true}, {"main", false}, {"release/1.3", false},
```
  plus `validator_test.go` rows for each `validateBranches` error and a valid config (no errors),
  plus the fixtures above through the existing schema/loader fixture tests.
- [ ] **Step 2:** `go test ./internal/config/... -run 'Branch'` → RED (undefined symbols).
- [ ] **Step 3:** implement `branches.go`, the struct field, `validateBranches`, schema, sample.
- [ ] **Step 4:** GREEN; `mise run test`; `hk check`.
- [ ] **Step 5:** Spec 02 subsection; roadmap note (flip T349).
- [ ] **Step 6:** commit `feat(config): add versioning.branches for maintenance lines (T349)`.

---

### Task 2 (T350): Current-branch detection, rule matching, unlisted-branch publish guard

**Files:**
- Create: `internal/app/branchrules.go`, `internal/app/branchrules_test.go`
- Modify: `internal/cmd/release.go`, `internal/cmd/changelog.go`; cmd tests next to the existing
  `CheckBranch` cmd tests (find them with `rtk proxy grep -rn "must be operated from branch" internal/cmd`)
- Docs: Spec 03 (`--force` on `release` / `changelog` gains the unlisted-branch bypass, next to its
  existing per-env meaning); roadmap.

**Interfaces — Consumes:** `config.BranchRule`, `config.BranchRange`, `ParseBranchRange`,
`DeriveBranchRange`, `IsGlob` (Task 1).

**Interfaces — Produces:**
```go
var (
    ErrUnlistedBranch   = errors.New("branch matches no versioning.branches entry")
    ErrAmbiguousBranch  = errors.New("branch matches more than one versioning.branches entry")
    ErrUnderivableRange = errors.New("cannot derive a maintenance range from the branch name")
)

type BranchKind int
const (
    BranchRelease BranchKind = iota // exact name, no range: today's behaviour
    BranchMaintenance               // has a Range
    BranchUnlisted                  // no entry matches, or the branch is unknown
)

type BranchMatch struct {
    Branch string              // "" when unknown
    Kind   BranchKind
    Range  config.BranchRange  // set when Kind == BranchMaintenance
    Rule   int                 // index into cfg.Versioning.Branches; -1 when unlisted
}

// CurrentBranch: git rev-parse --abbrev-ref HEAD; on "HEAD" fall back to CI_COMMIT_BRANCH,
// then GITHUB_REF_NAME (only when GITHUB_REF_TYPE == "branch"), then BUILD_SOURCEBRANCHNAME.
// ok == false when none is available; err only when git itself fails.
func CurrentBranch(runner port.Runner) (branch string, ok bool, err error)

// MatchBranchRule matches branch against cfg.Versioning.Branches. needRange == false skips
// ErrUnderivableRange (the --set-version path): the match is then BranchMaintenance with a zero
// Range, which callers on that path never read.
func MatchBranchRule(cfg *config.Config, branch string, known, needRange bool) (BranchMatch, error)

// CheckReleaseBranch refuses publishing from an unlisted/unknown branch when
// cfg.Versioning.Branches is set. No-op when the block is absent or force is true.
func CheckReleaseBranch(runner port.Runner, cfg *config.Config, force bool) error
```

Matching: an entry matches when `IsGlob()` ? `path.Match(rule.Name, branch)` : `rule.Name ==
branch`. Zero matches → `BranchUnlisted`. Two or more → wrapped `ErrAmbiguousBranch` listing the
entry names (`entries "release/*", "release/1.3" all match branch "release/1.3"`). One match:
explicit `Range` → `ParseBranchRange` → `BranchMaintenance`; glob without range →
`DeriveBranchRange(branch)` → `BranchMaintenance`, or if not derivable and `needRange` → wrapped
`ErrUnderivableRange` (`branch "release/legacy" matches versioning.branches[1] ("release/*") but
its name carries no N.x / N.N.x / N.N version — add range: to the entry`); exact name without range
→ `BranchRelease`. `known == false` → `BranchUnlisted` with no error.

`CheckReleaseBranch` error text: `%w: branch "feature/x" (pass --force to release anyway)` with
the configured names listed in the message's second line, `ErrUnlistedBranch` wrapped. Unknown
branch: `%w: cannot determine the current branch (detached HEAD, no CI branch variable)`.

cmd wiring (both inside the existing `if !dryRun {` blocks, right after `app.CheckBranch`):
- `release.go`: `if err := app.CheckReleaseBranch(readRunner, cfg, force); err != nil { return exitcode.Wrap(exitcode.Config, err) }`
- `changelog.go`: the same, but only when `tag` is true.

- [ ] **Step 1: failing tests.** `branchrules_test.go` (MockRunner for git; `t.Setenv` for CI vars,
  all four cleared first):
```go
// CurrentBranch
{"attached", gitOut: "main\n", want: "main", ok: true},
{"detached, GitLab", gitOut: "HEAD\n", env: {"CI_COMMIT_BRANCH": "release/1.3"}, want: "release/1.3", ok: true},
{"detached, GitHub branch", gitOut: "HEAD\n", env: {"GITHUB_REF_NAME": "release/1.3", "GITHUB_REF_TYPE": "branch"}, want: "release/1.3", ok: true},
{"detached, GitHub tag ref ignored", gitOut: "HEAD\n", env: {"GITHUB_REF_NAME": "v1.3.1", "GITHUB_REF_TYPE": "tag"}, ok: false},   // Review Focus 3
{"detached, Azure", gitOut: "HEAD\n", env: {"BUILD_SOURCEBRANCHNAME": "release-1.3"}, want: "release-1.3", ok: true},
{"detached, nothing", gitOut: "HEAD\n", ok: false},
{"git error", gitErr: errors.New("boom"), wantErr: "determining current git branch"},
// MatchBranchRule, rules [{main}, {release/*}, {lts, range: 2.x}]
{"main", "main", BranchRelease},
{"derived", "release/1.3", BranchMaintenance, "1.3.x"},
{"explicit", "lts", BranchMaintenance, "2.x"},
{"nested not matched", "release/1.3/hotfix", BranchUnlisted},                 // Review Focus 3
{"unlisted", "feature/x", BranchUnlisted},
{"unknown", "", known: false, BranchUnlisted},
{"underivable", "release/legacy", needRange: true, ErrUnderivableRange},
{"underivable, set-version", "release/7.8.0", needRange: false, BranchMaintenance},
{"ambiguous", rules + {release/1.3}, "release/1.3", ErrAmbiguousBranch},
// CheckReleaseBranch
{"no block → no git call"}, {"listed → nil"}, {"unlisted → ErrUnlistedBranch"}, {"unlisted + force → nil, no git call"},
{"unknown → ErrUnlistedBranch, message names detached HEAD"},
```
  plus cmd tests: `heraut release` on an unlisted branch exits Config with the message;
  `--force` passes the guard; `--dry-run` passes the guard; `heraut changelog` (no `--tag`) on an
  unlisted branch is not refused; `heraut changelog --tag` is.
- [ ] **Step 2:** `go test ./internal/app/ ./internal/cmd/ -run 'Branch'` → RED.
- [ ] **Step 3:** implement `branchrules.go`; wire cmd.
- [ ] **Step 4:** GREEN; `mise run test`; `hk check`.
- [ ] **Step 5:** Spec 03; roadmap note (flip T350; record the dry-run clarification).
- [ ] **Step 6:** commit `feat(app): detect branch and guard unlisted publishes (T350)`.

---

### Task 3 (T351): Maintenance resolution in `semver.Resolver`

**Files:**
- Create: `internal/versioning/semver/maintenance.go`, `internal/versioning/semver/maintenance_test.go`
- Modify: `internal/versioning/semver/resolver.go`, `internal/app/resolver.go` (+ its tests)
- Docs: Spec 04, new § "Maintenance branches" (the rules of spec §4, the worked-examples table,
  the error table of spec §6); roadmap.

**Interfaces — Consumes:** `app.CurrentBranch`, `app.MatchBranchRule`, `config.BranchRange` (Tasks 1–2).

**Interfaces — Produces:**
```go
var (
    ErrOutOfRange       = errors.New("next version is outside the maintenance range")
    ErrTagExists        = errors.New("tag already exists")
    ErrNoInRangeRelease = errors.New("no release in the maintenance range is reachable from HEAD")
)

// Range is a maintenance line [Lo, Hi).
type Range struct{ Lo, Hi Version; Label string; Branch string } // Label "1.3.x", Branch "release/1.3"
func RangeFrom(r config.BranchRange, branch string) Range
func (r Range) Contains(v Version) bool // Compare(v, Lo) >= 0 && Compare(v, Hi) < 0, on v's core

func (r *Resolver) SetMaintenanceRange(rg *Range) // nil = off (today's path)
```
`RangeFrom`: `Minor == nil` → `Lo = N.0.0`, `Hi = (N+1).0.0`; else `Lo = N.M.0`, `Hi = N.(M+1).0`.

**`resolveAuto` with a range set.** Git calls in this exact order (contract tests assert it):
1. `git tag -l <prefix>* --merged HEAD --sort=-version:refname` (instead of the global listing).
2. Sort with `SortTags` (same extractor); `base` = the first *final* (`!IsPreRelease`) whose
   `Version` (core) satisfies `rg.Contains`. None → `fmt.Errorf("%w: no release in range %s in the
   history of %s — tag the branch's starting point or pass --set-version", ErrNoInRangeRelease,
   rg.Label, rg.Branch)`.
3. `git log <base.Tag>..HEAD --format=%B%x00`; empty → today's `no commits since <tag> — …` error.
4. `bump := r.bumpAfterHold(base.core, commits)`; `BumpNone` → `noReleasableCommitsError`.
   `next := BumpVersion(base.core, bump)`. `!rg.Contains(next)` →
   `fmt.Errorf("%w: %s would release %s, outside %s (%s) — land it on a branch whose range allows
   it, or on main", ErrOutOfRange, <first subject at the bump level>, next, rg.Branch, <">=Lo <Hi">)`
   (reuse `commitsAtLevel` from `hold.go` for the subject).
5. `git tag -l <prefix><next>`; non-empty → `fmt.Errorf("%w: %s<next> (cut on another branch) —
   pick the next free version with --set-version", ErrTagExists, prefix)`.
6. Return `Result{Version: next, Tag: prefix+next, CurrentTag: base.Tag, Bump: bump}`.

With `rg == nil`, `resolveAuto` is untouched (guard test: exact call sequence unchanged).

**`NewResolver` wiring** (`case "semver":` only, after the `versionOverride != ""` early return,
so `--set-version` never reaches it):
```go
if len(cfg.Versioning.Branches) > 0 {
    branch, known, err := CurrentBranch(runner)
    if err != nil { return nil, err }
    m, err := MatchBranchRule(cfg, branch, known, true)
    if err != nil { return nil, err }          // Config exit via the caller's wrap
    if m.Kind == BranchMaintenance {
        rg := semver.RangeFrom(m.Range, m.Branch)
        r.SetMaintenanceRange(&rg)
    }
}
```
Also under `bump.mode: manual` (no auto resolution) skip the block entirely: the version comes from
`--set-version`.

- [ ] **Step 1: failing tests.** `maintenance_test.go`, MockRunner FIFO, each row asserting the
  result *and* the exact git calls. Range `1.3.x` / branch `release/1.3` unless stated:
```go
// name | merged tags | log since base | tag-exists probe | range | want tag | want err
{"patch on line", "v1.3.1\nv1.3.0", "fix: x", "", "1.3.x", "v1.3.2", nil},
{"feat out of range", "v1.3.1\nv1.3.0", "feat: y", —, "1.3.x", "", ErrOutOfRange /* names "feat: y", 1.4.0, >=1.3.0 <1.4.0 */},
{"feat on 1.x", "v1.4.0\nv1.3.1", "feat: z", "", "1.x", "v1.5.0", nil},
{"breaking out of 1.x", "v1.4.0", "feat!: b", —, "1.x", "", ErrOutOfRange},
{"tag exists", "v1.3.1", "fix: x", "v1.3.2", "1.3.x", "", ErrTagExists},
{"no in-range base", "v1.2.5", —, —, "1.3.x", "", ErrNoInRangeRelease},
{"no commits", "v1.3.1", "", —, "1.3.x", "", "no commits since v1.3.1"},
{"rel- prefix + build metadata base", tag_prefix "rel-", "rel-1.3.1+5\nrel-1.3.0", "fix: x", "", "1.3.x", "rel-1.3.2", nil}, // Review Focus 1
{"pre-release of next core ignored as base", "v1.3.2-rc.1\nv1.3.1", "fix: x", "", "1.3.x", "v1.3.2", nil},                // Review Focus 4
{"higher main tags not reachable", "v1.3.1\nv1.3.0" /* v2.0.0 absent from --merged */, "fix: x", "", "1.3.x", "v1.3.2", nil},
{"stay_at_v0 on 0.x line", "v0.3.1", "feat!: b", —, "0.x", "v0.4.0", nil /* held to minor, warning recorded */},
```
  plus `RangeFrom`/`Contains` table, plus a guard that `SetMaintenanceRange(nil)` keeps
  `resolveAuto`'s call sequence byte-identical. `internal/app/resolver_test.go` rows: no `branches`
  → no `rev-parse` call; maintenance branch → resolver gets the range (observable via the first
  git call being `--merged HEAD`); release branch → global listing; ambiguous → error;
  `--set-version` on `release/7.8.0` with `release/*` → no git call, `v7.8.0`; manual mode → no
  `rev-parse` call.
- [ ] **Step 2:** `go test ./internal/versioning/semver/ ./internal/app/ -run 'Maintenance|Range|NewResolver'` → RED.
- [ ] **Step 3:** implement `maintenance.go`, the `resolveAuto` branch, `NewResolver` wiring.
- [ ] **Step 4:** GREEN; `mise run test`; `hk check`.
- [ ] **Step 5:** Spec 04 § Maintenance branches; roadmap note (flip T351; record the
  `--set-version` collision clarification).
- [ ] **Step 6:** commit `feat(versioning/semver): resolve versions on maintenance lines (T351)`.

---

### Task 4 (T352): History-aware changelog and notes bounds

**Files:**
- Modify: `internal/app/pipeline.go` (`buildGenerator`, and remove the now-redundant
  `WithReachableFromHead` append in the pre-release notes branch around line 318 — keep
  `notesTagOrderFor`), `internal/app/changelog_rotation.go` (`latestMatchingTag`),
  `internal/generators/native/generator.go` (`buildAllSections`)
- Test: `internal/generators/native/generator_internal_test.go`,
  `internal/app/changelog_rotation_internal_test.go`, a new real-git test
  `internal/app/historybounds_realrepo_internal_test.go`
- Docs: Spec 05 (changelog/notes bounds: reachable walk, per-section ancestry, CalVer unchanged);
  roadmap.

**Interfaces — Consumes:** `native.WithReachableFromHead()`, `listMergedTags` (existing).

**Changes:**
1. `buildGenerator`: when `tagOrder != nil`, append `native.WithReachableFromHead()` to `opts`
   (before `extra`). This covers changelog, notes, the changelog-only pipeline and
   `rotatingGenerator` (all go through `buildGenerator`).
2. `latestMatchingTag`: when `tagOrder != nil`, list with `git tag -l <prefix>* --merged HEAD
   --sort=-version:refname`; when nil (calver), the call stays byte-identical.
3. `buildAllSections`, for an existing tag `t` at index `i` with a tag order set:
   ```go
   merged, err := listMergedTags(g.runner, t)          // t's ancestors, t excluded
   // first entry after t in the (already ordered, already scoped) tags list that is an ancestor
   for _, cand := range tags[i+1:] { if slices.Contains(merged, cand) { prev = cand; break } }
   // none in scope → existing oldest-in-scope logic, unchanged:
   //   scoped (TagGlob/TagPattern) → first of g.tagOrder(merged); unscoped → "" (start of history)
   ```
   Without a tag order (calver), the loop stays exactly as today (`prev = tags[i+1]`, then the
   existing fallback). Keep the existing T257/T334 comment, extended to state the scope-preserving
   ancestry rule (ADR-0065).

- [ ] **Step 1: failing tests.**
  - native unit (MockRunner): with a tag order, a 3-tag walk issues one `git tag -l --merged <t>
    --no-contains <t> --sort=-version:refname` per section, and a section whose next-older scoped
    tag is not an ancestor skips to the next one that is. Without a tag order, the call sequence is
    byte-identical to today's (guard).
  - rotation: `latestMatchingTag` with an order → `--merged HEAD` argv; without → today's argv.
  - `buildGenerator`: with an order, the first `git tag -l` the changelog issues carries
    `--merged HEAD`; with nil order it doesn't.
  - real git (`historybounds_realrepo_internal_test.go`), each shown RED first:
    - **unmerged maintenance tag:** `main`: `v1.3.0`, `v1.3.1`, `v1.4.0`; branch `release/1.3` from
      `v1.3.1` with `fix: m` tagged `v1.3.2`; back on `main` a new `feat: n`; `--regenerate` changelog
      for `v1.5.0` → no `[v1.3.2]` section; `[v1.4.0]` section lists exactly the commits between
      `v1.3.1` and `v1.4.0`.
    - **merged forward:** same, then `git merge --no-ff release/1.3` on `main` → a `[v1.3.2]`
      section listing only `fix: m`; `[v1.4.0]` still bounded at `v1.3.1` (no `fix: m` in it).
    - **per-env scope (Review Focus 2):** `semver-per-env`, `tag_format: "{env}/{version}"`,
      tags `prod/1.2.0` (commit A), `uat/1.3.0` (commit B), `prod/1.3.0` (commit C, B ancestor of
      C); changelog driver with `TagGlob: "prod/*"`; `--regenerate` → `prod/1.3.0` section bounded
      at `prod/1.2.0` (lists commits B and C).
  - existing `tagorder_realrepo_internal_test.go` and every native/T334/T341 test: unmodified and
    green (linear history ⇒ byte-identical). Contract rows whose git argv gains `--merged HEAD`
    are edited in place with an ADR-0065 comment and named in the commit body.
- [ ] **Step 2:** `go test ./internal/generators/native/ ./internal/app/ -run 'Bound|Rotation|Merged|BuildGenerator'` → RED.
- [ ] **Step 3:** implement the three changes.
- [ ] **Step 4:** GREEN; `mise run test`; `hk check`.
- [ ] **Step 5:** Spec 05; roadmap note (flip T352; list edited rows; record the scope-preserving
  clarification).
- [ ] **Step 6:** commit `fix(generators/native): bound changelog sections by ancestry (T352)`.

---

### Task 5 (T353): `version current` and pre-releases on maintenance branches

**Files:**
- Modify: `internal/versioning/semver/prerelease.go` (base `L` under a range),
  `internal/app/current.go` (`CurrentTag` maintenance-aware), tests next to each
- Docs: Spec 04 (§ Maintenance branches: `version current`, pre-releases), Spec 03 (`version
  current` note); roadmap.

**Interfaces — Consumes:** `semver.Range`, `SetMaintenanceRange`, `ErrOutOfRange`,
`ErrNoInRangeRelease` (Task 3); `CurrentBranch`, `MatchBranchRule` (Task 2).

**`resolvePreRelease` with a range set:** step 1's global listing stays (series counter and
monotonicity are global by ADR-0064). Step 2's `L` becomes the highest in-range final from a
`git tag -l <prefix>* --merged HEAD --sort=-version:refname` listing, issued *before* the existing
`git log <L>..HEAD` call; no in-range final → `ErrNoInRangeRelease`. After computing `core`,
`!rg.Contains(core)` → `ErrOutOfRange` (same message shape as Task 3). The existing step-6 merged
listing is reused for the commit requirement (do not issue it twice: keep the parsed result from
the new call). Everything else (escalation, counter, monotonicity) is unchanged.

**`CurrentTag`:** for `semver` with `len(cfg.Versioning.Branches) > 0`: detect + match
(`needRange: true`); maintenance → list `git tag -l <glob> --merged HEAD --sort=-version:refname`
and return the highest final in range (with `includePreRelease`, the highest tag of any kind whose
core is in range); no such tag → `errNoTagsFound`-wrapped message naming the range. Other kinds →
today's path. `CurrentVersion` inherits it.

- [ ] **Step 1: failing tests.**
```go
// resolvePreRelease, range 1.3.x
{"rc on line", global "v2.0.0\nv1.4.0\nv1.3.1", merged "v1.3.1\nv1.3.0", log "fix: x", "rc", "v1.3.2-rc.1", nil},
{"rc counter on line", global "+v1.3.2-rc.1", merged "v1.3.2-rc.1\nv1.3.1", logs…, "rc", "v1.3.2-rc.2", nil},
{"feat rc out of range", merged "v1.3.1", log "feat: y", "rc", "", ErrOutOfRange},
{"main's open series doesn't escalate the line", global "v2.1.0-rc.1\nv2.0.0\nv1.3.1", merged "v1.3.1", log "fix: x", "rc", "v1.3.2-rc.1", nil /* no escalation warning */},
// CurrentTag on release/1.3 (rules [{main},{release/*}])
{"current on line", merged "v1.3.1\nv1.3.0", "v1.3.1"},
{"current incl pre-release", merged "v1.3.2-rc.1\nv1.3.1", includePre, "v1.3.2-rc.1"},
{"current on main unchanged", branch main, global listing, "v2.0.0"},
{"no block → no rev-parse", …},
```
- [ ] **Step 2:** `go test ./internal/versioning/semver/ ./internal/app/ -run 'PreRelease|Current'` → RED.
- [ ] **Step 3:** implement.
- [ ] **Step 4:** GREEN; `mise run test`; `hk check`.
- [ ] **Step 5:** docs; roadmap note (flip T353).
- [ ] **Step 6:** commit `feat(versioning/semver): pre-releases and current on maintenance (T353)`.

---

### Task 6 (T354): End-to-end maintenance scenarios on a real repo

**Files:** Create `internal/app/maintenance_realrepo_internal_test.go` (pattern:
`prerelease_realrepo_internal_test.go` / `tagorder_realrepo_internal_test.go`). Drive
`NewResolver(...).Resolve()`, `CurrentTag`, and the changelog generator through the production
wiring (`BuildChangelogPipeline` or `buildGenerator` + `tagOrderFor`), with the real
`execadapter.New(false, false)` runner.

Fixture (built once per subtest via a helper): `main` has `v1.3.0 → v1.3.1 → v1.4.0 → v2.0.0`
(annotated or lightweight tags with the `GIT_CONFIG_GLOBAL=/dev/null` helper); `release/1.3`
from `v1.3.1`; `release/1.x` from `v1.4.0`. Config: `strategy: semver`,
`branches: [{name: main}, {name: "release/*"}]`.

- [ ] **Step 1: tests**, one subtest per spec worked-example row, asserting the exact outcome:
  1. `release/1.3` + `fix: x` → `v1.3.2`; release notes for it span `v1.3.1..v1.3.2`.
  2. `release/1.3` + `feat: y` → `errors.Is(err, semver.ErrOutOfRange)`.
  3. `release/1.x` + `feat: z` → `v1.5.0`.
  4. `release/1.x` + `feat!: b` → `ErrOutOfRange`.
  5. `main` + `fix: w` with `v1.3.2` tagged on `release/1.3` → `v2.0.1`.
  6. `release/1.3`, `v1.3.2` already tagged on a side branch → `ErrTagExists`.
  7. detached HEAD at `release/1.3`'s tip, `CI_COMMIT_BRANCH=release/1.3` → `v1.3.2`.
  8. `feature/foo` → `CheckReleaseBranch` returns `ErrUnlistedBranch`; with `force` → nil; and
     8b. `NewResolver(...).Resolve()` (preview) → `v2.0.1`.
  9. `release/1.3` → `CurrentTag` = `v1.3.1`.
  10. `release/1.3` + `fix: x` + pre-release `rc` → `v1.3.2-rc.1`.
  11. `release/7.8.0` + `--set-version 7.8.0` → `v7.8.0`, no error; 11b. without
      `--set-version` → `ErrUnderivableRange`.
  - **Review Focus 5:** on `release/1.3` after `v1.3.2` is tagged, a `--regenerate` changelog run
    lists exactly sections `v1.3.2`, `v1.3.1`, `v1.3.0` (no `v1.4.0`, no `v2.0.0`).
- [ ] **Step 2: regression proof.** Tasks 3–5 already landed, so prove the scenarios against the
  pre-Task-3 code in a throwaway worktree outside the repo:
  `git worktree add "$TMPDIR/heraut-pre-t351" <commit before Task 3>`, copy the new test file in,
  stub what doesn't compile there (rows needing `ErrOutOfRange`/`CheckReleaseBranch` are skipped
  in the copy), run rows 1, 9 and Review Focus 5 → expect FAIL (`v2.0.1` instead of `v1.3.2`,
  `v2.0.0` instead of `v1.3.1`, a `[v1.4.0]`/`[v2.0.0]` section present); row 5 must PASS there too
  (it pins unchanged behaviour on `main`). Paste the failure lines
  into the commit body, then `git worktree remove "$TMPDIR/heraut-pre-t351"`. Then GREEN on `main`.
- [ ] **Step 3:** `mise run test`; `hk check`.
- [ ] **Step 4:** roadmap note (flip T354).
- [ ] **Step 5:** commit `test(app): end-to-end maintenance-branch scenarios (T354)`.

---

### Task 7 (T355): ADR-0065, guide, Phase 3 close

**Files:**
- Create: `docs/adr/0065-branch-aware-semver-resolution.md` (follow the section layout of
  `docs/adr/0064-semver-v2-compliance.md`: Context, Decision, Consequences, Alternatives
  considered). Content from the spec: the model and entry-type table, prior art with the
  semantic-release and GitVersion links, the branch-type resolution table, why hard errors, the
  history-aware bounds and their one reach into configs without `branches`, the `--set-version`
  rule, rejected alternatives B and C, and the clarifications listed at the top of this plan.
- Modify: `docs/adr/README.md` (index row), `docs/adr/0064-semver-v2-compliance.md` (one line in
  § Status update (Phase 2) pointing to ADR-0065 for T344), `CLAUDE.md` ("63 ADRs" count → 64 in
  both places), Spec 04 links to the ADR.
- Create: `docs/guides/maintenance-branches.md`: when to use it, the config, the worked-examples
  table, the forward-merge/backport workflow (git, not heraut), and the "branch is the version"
  case (`--set-version`, link T348).
- `docs/tasks/semver-v2-roadmap.md`: "Phase 3 closed" paragraph; flip T344 `[ ]` → `[x]` with a
  completion note pointing to T349–T355; table rows; header `Status:` → `Done` if nothing else is
  open; the epic note at the end.
- `docs/tasks/roadmap.md`: Phase 59 row → Done; file the CalVer follow-up task (next free T-id):
  "History-aware changelog bounds for CalVer" (`[ ]`, the §5 rule applied when no tag order is
  set; CalVer's zero-padded tags need their own ordering for the ancestor pool).
- Commit: `docs(adr): add 0065 branch-aware SemVer resolution (T355)`.

---

## Roadmap filing (done by the controller before execution, not a task)

- `docs/tasks/semver-v2-roadmap.md`: add "## Phase 3 — Maintenance branches" with the T349–T355
  headings (`[ ]`, one-line scope each, pointing to this plan) and table rows; T344's entry gains
  "Broken down into T349–T355 (Phase 3)".
- Set the header `Status:` line to note Phase 3 in progress.
