# SemVer v2 — Phase 2 (pre-release lifecycle) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** plain `semver` can mint pre-releases (`heraut release --pre-release rc` → `v1.4.0-rc.1`)
under the series rules of the design doc § 1. A pre-release run never writes `CHANGELOG.md`, and
its release notes span back to the previous tag of any kind that is in HEAD's history.

**Architecture:** the series rules live in `internal/versioning/semver` (new `prerelease.go`),
and the resolver gains a pre-release mode. `internal/app` validates the flag combinations and
decides **at build time** whether a run is a pre-release. A run is a pre-release when it has a
`--pre-release` label, or when a SemVer-strategy `--set-version` value has pre-release
identifiers. From that decision, app (1) reuses the `disable_changelog` skip path, so step totals
stay correct with no runtime branching, and (2) gives the release-notes generator an ordering
that keeps pre-releases, restricted to tags reachable from HEAD. `internal/generators/native`
gains one option (`WithReachableFromHead`) and one fix: the tag being released is added to the
list before ordering, so its predecessor is always found by precedence (roadmap note (b)).

**Tech Stack:** Go, cobra, `github.com/adaouat/forge/exec/exectest.MockRunner`,
`internal/testutil.RealGitRepo`.

**Spec:** `docs/superpowers/specs/2026-09-28-semver-v2-compliance-design.md` § 1, § 2, § 4, § 6,
§ 7; ADR-0064; roadmap `docs/tasks/semver-v2-roadmap.md` § Phase 2 (notes (a)–(d)).

## Decisions settled with the user (2026-10-03/04, do not reopen)

The spec's own decisions also stand: floating core, major escalation blocked unless
`--allow-major`, global per-core monotonicity, label from the CLI flag only, minting plain
`semver` only, no CHANGELOG for pre-releases, and a final's notes span back to the last final.

- **Previous-tag rule for a pre-release (note (c)):** the highest-precedence tag *of any kind*
  that sorts strictly below the version being released **and is reachable from HEAD**
  (`git tag --merged HEAD`). One rule drives both the commit requirement (§ 1 step 5) and the
  notes range. Finals keep T334's rule unchanged: the last final, precedence only. The core and
  "last final" computation stays global; maintenance branches are T344, filed for later.
- **"Is a pre-release run"** is keyed on the version, not on the flag: `--pre-release <label>`,
  or a SemVer strategy (`semver`/`semver-per-env`) with a `--set-version` value carrying
  pre-release identifiers. This is the same gate as T337's derived GitHub flag. It covers
  `heraut changelog --set-version X-pre --tag`, which now tags without writing a section (closes
  ADR-0064's interim DOC-1 state).
- **Note (a), pre-release sections already on disk:** not handled. No user has pre-releases.
  Record it in the roadmap and do nothing in code or docs.

## Global Constraints

- TDD: failing test first (capture the RED output), then implementation, then GREEN.
- Never delete or weaken a test row. A row whose behaviour deliberately changes is edited, with
  a comment citing ADR-0064, and named in the commit body.
- `calver` / `calver-per-env`: zero behaviour change. No pre-release mode, no new git calls, and
  every existing calver row stays unmodified.
- Plain `semver` final releases: zero behaviour change. In particular, `resolveAuto`'s git call
  sequence and arguments (asserted by contract tests) stay byte-for-byte identical when
  `--pre-release` is not passed.
- Layering (`.claude/rules/coding.md`): cmd never switches on strategy, and validation that
  needs the strategy goes through `app.NewResolver`. `internal/generators/native` must not import
  `internal/versioning`, so app passes orderings down as functions.
- Errors wrap with `%w`. New sentinels in `internal/versioning/semver`: `ErrMajorEscalation`,
  `ErrPreReleaseRegression` (spec § 6). The commit-requirement error keeps today's unwrapped
  `no commits since <tag> — …` text shape.
- Conventional commits, subject ≤ 72 chars (**count it; no hook enforces length**), trailer
  exactly `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`. Never a `Claude-Session:`
  line, never `--no-verify`.
- The `typos` hook rejects the token "m-i-s" (as in "m-i-s-sorted"): write "out of order".
- Code comments state the *why*. Never write session or review labels ("FIX-1", "review round",
  "final review", "previously/used to …") or task-in-progress references. ADR and T-ids are fine.
- The rtk shell hook can mangle some tools' flags: prefix with `rtk proxy` when a command
  misbehaves (e.g. `rtk proxy typos …`).
- The user's global git config forces annotated tags. In scratch repos and `RealGitRepo`-style
  helpers use `git tag -a -m <msg>`, never a bare `git tag x`.
- Work on `main`; ignore `.claude/worktrees/`. Run `mise run test` before each commit (and again
  after any lint auto-fix); `hk check` must be clean. Fix lint with `hk fix -S <linter>`, not the
  raw tool.
- No real data in fixtures: synthetic tags, messages and repo names only.
- Each task flips its roadmap heading `[ ]` → `[x]`, sets its table row to Done, and adds a
  one-paragraph completion note (decisions, deviations, deferred items) in the same commit.

## Review Focus

1. **Non-default `tag_prefix`** (`tag_prefix: "rel-"`): `--pre-release rc` lists `rel-*`, renders
   `rel-1.4.0-rc.1`, and the counter/monotonicity checks only consider `rel-` tags. Pinned in T338.
2. **A pre-release core below an existing higher series**: a hand-cut `v2.0.0-rc.1` exists but
   the commits since `v1.3.0` only justify `1.4.0`. The spec only defines escalation *upward*, so
   heraut mints `v1.4.0-rc.1` with no error and no escalation warning: series are per core.
   Pinned in T338.
3. **Hand-made pre-release tags of odd shape** (`v1.4.0-rc`, `v1.4.0-rc.1.1`, `v1.4.0-RC.1`):
   the counter only counts the exact `<label>.<N>` shape. Every other same-core tag still takes
   part in the monotonicity check (`rc.1` > `rc`; `rc.2` > `rc.1.1`; `RC` < `rc` in ASCII order,
   so `--pre-release RC` after `rc.1` is a regression). Pinned in T338.
4. **`stay_at_v0` with a breaking commit on a 0.x repo**: without `--allow-major` the core is
   held to the next minor (the hold warning is printed) and the pre-release is minted on the held
   core. With `--allow-major` → `1.0.0-<label>.1`. Pinned in T338.
5. **`--dry-run` of a pre-release release**: no changelog lines and no `pre_changelog` hooks in
   either the plain or the reporter output, and the step total `N/M` matches the steps actually
   shown. Pinned in T340.

---

## File map

| File | Change | Task |
|---|---|---|
| `internal/versioning/semver/prerelease.go` (new) | label validation, sentinels, series resolution | T338 |
| `internal/versioning/semver/resolver.go` | `SetPreRelease`, dispatch in `Resolve` | T338 |
| `internal/versioning/semver/hold.go` | generalise `majorCommits` → `commitsAtLevel` | T338 |
| `internal/versioning/semver/prerelease_test.go` (new) | MockRunner unit + contract tests | T338 |
| `internal/app/resolver.go` | `WithPreRelease` option + usage errors; `ValidatePreReleaseLabel` | T339 |
| `internal/cmd/release.go`, `internal/cmd/version.go` | `--pre-release` flag; `--allow-major` help text | T339 |
| `internal/app/pipeline.go` (+ new `internal/app/prerelease.go`) | `PipelineOpts.PreReleaseLabel`, `isPreReleaseRun`, changelog skip, notes order | T340, T341 |
| `internal/pipeline/changelog.go` | `ChangelogConfig.PreRelease` message | T340 |
| `internal/generators/native/generator.go`, `commits.go` | insert-then-order; `WithReachableFromHead` | T341 |
| `internal/app/current.go` | `notesTagOrderFor` next to `tagOrderFor` | T341 |
| `internal/app/prerelease_realrepo_internal_test.go` (new) | end-to-end scenarios on a real repo | T342 |
| `docs/specs/03-commands.md`, `04-versioning.md`, `05-generators-and-platforms.md` | behaviour docs | per task |
| `docs/adr/0064-semver-v2-compliance.md` | Phase 2 status update | T343 |
| `docs/tasks/semver-v2-roadmap.md` | per-task notes; Phase 2 closed | each task, T343 |

---

### Task 1 (T338): Pre-release series resolution in `semver.Resolver`

**Files:**
- Create: `internal/versioning/semver/prerelease.go`, `internal/versioning/semver/prerelease_test.go`
- Modify: `internal/versioning/semver/resolver.go` (new field `preReleaseLabel`, `SetPreRelease`,
  `Resolve` dispatch), `internal/versioning/semver/hold.go` (`majorCommits` → `commitsAtLevel`;
  keep `holdMajorAtZero`'s output byte-identical)
- Docs: `docs/specs/04-versioning.md`, a new § "Pre-release lifecycle" (the § 1 model, label
  grammar, worked-examples table, the error messages, and the previous-tag rule with its
  reachable-from-HEAD bound); roadmap.

**Interfaces — Produces:**
```go
var ErrMajorEscalation = errors.New("pre-release series major escalation")
var ErrPreReleaseRegression = errors.New("pre-release would not sort above existing tags of its core")

// ValidatePreReleaseLabel: one SemVer identifier — [0-9A-Za-z-]+, not purely numeric, no dots.
func ValidatePreReleaseLabel(label string) error

// SetPreRelease switches Resolve into pre-release mode for label ("" = final release, today's path).
func (r *Resolver) SetPreRelease(label string)
```

**Algorithm (`resolvePreRelease`)**, given `prefix` and `label`. The git calls happen in this
exact order (contract tests assert it):

1. `git tag -l <prefix>* --sort=-version:refname`. Parse with `SortTags` (same extractor as
   `resolveAuto`) → `all`.
2. `L, hasFinal := Latest(all, false)`.
   - `hasFinal`: `git log <L.Tag>..HEAD --format=%B%x00`. If there are no commits, return today's
     error (`no commits since <L.Tag> — …`). Otherwise `bump := r.bumpAfterHold(L.core, commits)`,
     and if that is `BumpNone` return `noReleasableCommitsError`. Then
     `core := BumpVersion(L.core, bump)`.
   - `!hasFinal`: `core := r.initialVersion()`, with no commits read (matches today's no-final
     behaviour).
3. **Escalation.** `S` = the highest pre-release in `all` whose core is above `L.core` (any
   pre-release when there is no final). If `S` exists and `Compare(core, S.core) > 0`:
   - `core.Major > S.Major` and `!r.allowMajor` → return a wrapped `ErrMajorEscalation`:
     `pre-release series <S.Tag> would escalate to a new major <core>: breaking change(s) since
     <L.Tag> — pass --allow-major to open the <core> series, or ship <S.core> first`, followed by
     the major-level commit subjects as `\n  - <subject>` lines (max 5, then `… and N more`, the
     same shape as `holdMajorAtZero`).
   - otherwise (minor/patch escalation, or major with `--allow-major`), record the warning
     `pre-release core escalated <S.core> → <core>` with the subjects of the commits at the new
     bump level as `\n  - ` lines. Append `""` to `wouldBeVersions` so `app.warningResolver` does
     not rewrite it.
   - `Compare(core, S.core) <= 0`: no escalation (Review Focus 2).
4. **Candidate.** `N` = 1 + the highest `n` over the tags in `all` with core == `core` and
   `Pre == [label, n]` (n numeric). If there are none, `N` = 1. Candidate =
   `<core>-<label>.<N>`.
5. **Monotonicity.** For every tag in `all` with core == `core` (finals included), the candidate
   must satisfy `Compare(candidate, t) > 0`. Otherwise return a wrapped `ErrPreReleaseRegression`:
   `<prefix><candidate> would sort below existing <t.Tag> — ship <core> or use a label that sorts
   higher`. Report the highest offending tag.
6. **Commit requirement.** `git tag -l <prefix>* --merged HEAD --sort=-version:refname` → parse
   → `P` = the highest tag of any kind with `Compare(P, candidate) < 0`.
   - If `P` exists and `P.Tag != L.Tag`: `git log <P.Tag>..HEAD --format=%B%x00`. If there are
     no commits, return `no commits since <P.Tag> — create at least one commit before running
     heraut release`.
   - If `P.Tag == L.Tag`, step 2 already checked it, so make no extra call. If there is no `P`,
     make no check.
7. Return `Result{Version: candidate, Tag: prefix+candidate, CurrentTag: P.Tag (else L.Tag, else
   ""), Bump: bump}`. `CurrentTag` feeds the `previous_tag` hook variable, so for a pre-release
   it is the tag the notes span from.

`Resolve` order: `versionOverride != ""` or manual mode → `resolveManual` (unchanged; app rejects
the flag combinations in T339), else `preReleaseLabel != ""` → `resolvePreRelease`, else
`resolveAuto`.

- [ ] **Step 1: failing tests** in `prerelease_test.go`. They are table-driven over the MockRunner
  FIFO, and each row asserts the result *and* the exact git calls. Rows (last final `v1.3.0`
  unless stated):

```go
// name | tags (git tag -l output) | log since L | merged tags | log since P | label | allowMajor | want tag | want err / warning
{"first beta", "v1.3.0", "feat: x", "v1.3.0", "", "beta", false, "v1.4.0-beta.1", nil},
{"beta counter", "v1.4.0-beta.1\nv1.3.0", "feat: x\x00fix: y", "v1.4.0-beta.1\nv1.3.0", "fix: y", "beta", false, "v1.4.0-beta.2", nil},
{"switch to rc", "v1.4.0-beta.2\n…", …, "rc", false, "v1.4.0-rc.1", nil},
{"label regression", "v1.4.0-rc.1\nv1.4.0-beta.2\n…", …, "beta", false, "", ErrPreReleaseRegression /* names v1.4.0-rc.1 */},
{"no new commits", "v1.4.0-rc.1\n…", "feat: x", "v1.4.0-rc.1\n…", "" /* empty log */, "rc", false, "", "no commits since v1.4.0-rc.1"},
{"patch series", "v1.3.0", "fix: a", …, "rc", false, "v1.3.1-rc.1", nil},
{"minor escalation warns", "v1.3.1-rc.1\nv1.3.0", "fix: a\x00feat: b", …, "rc", false, "v1.4.0-rc.1", "pre-release core escalated 1.3.1 → 1.4.0\n  - feat: b"},
{"major escalation blocked", "v1.4.0-rc.1\nv1.3.0", "feat: b\x00feat!: c", …, "rc", false, "", ErrMajorEscalation},
{"major escalation allowed", same, …, "rc", true, "v2.0.0-rc.1", "pre-release core escalated 1.4.0 → 2.0.0\n  - feat!: c"},
{"no final yet", "", …, "alpha", false, "v0.1.0-alpha.1", nil},
// Review Focus 1–4: rel- prefix; lower core than an open higher series; odd-shaped tags
// (v1.4.0-rc, v1.4.0-rc.1.1, --pre-release RC after rc.1); stay_at_v0 0.x hold with/without allowMajor.
```

Also add: `ValidatePreReleaseLabel` table (accept `rc`, `beta`, `next`, `rc-x`, `0a`; reject `""`,
`1`, `007`, `rc.1`, `rc_x`, `rç`). Add a guard proving `SetPreRelease("")` leaves `resolveAuto`'s
call sequence unchanged. Add a `holdMajorAtZero` guard whose output is unchanged after the
`commitsAtLevel` refactor (the existing `stay_at_v0_test.go` rows must pass unmodified).

- [ ] **Step 2:** `go test ./internal/versioning/semver/ -run 'PreRelease|Label'` → RED (undefined symbols).
- [ ] **Step 3:** implement `prerelease.go` plus the `Resolve` dispatch and the `commitsAtLevel` generalisation.
- [ ] **Step 4:** GREEN, then `mise run test`.
- [ ] **Step 5:** Spec 04 § Pre-release lifecycle; roadmap note (flip T338).
- [ ] **Step 6:** commit `feat(versioning/semver): resolve pre-release series (T338)`.

---

### Task 2 (T339): `--pre-release <label>` on `release` and `version next`

**Files:**
- Modify: `internal/app/resolver.go`. Add `WithPreRelease(label string) ResolverOption` (field
  `preRelease` in `resolverOptions`). In `NewResolver`, apply options **first** (move the
  `for _, opt := range opts` loop to the top), then when `o.preRelease != ""` return these
  usage errors before any other branch:
  1. `versionOverride != ""` → `--pre-release cannot be combined with --set-version: --set-version already chooses the version (pass a pre-release value such as 1.4.0-rc.1 to it instead)`;
  2. strategy ≠ `semver` → `--pre-release requires versioning.strategy: semver (got %q): pre-releases are minted for plain semver only (ADR-0064)`;
  3. `cfg.Versioning.BumpMode() == "manual"` → `--pre-release requires versioning.bump.mode: auto — manual mode has no computed version to build a pre-release on`;
  4. `semver.ValidatePreReleaseLabel` fails → `--pre-release %q: <err>`, wrapped with `%w`.

  Then in the `"semver"` case call `r.SetPreRelease(o.preRelease)`.
- Modify: `internal/cmd/release.go`, `internal/cmd/version.go` (`version next` only). Add
  `cmd.Flags().String("pre-release", "", "mint a pre-release <core>-<label>.<N> (plain semver only; never writes CHANGELOG.md)")`
  and pass `app.WithPreRelease(label)` to `NewResolver`. On `release`, also put the label into
  `PipelineOpts.PreReleaseLabel` (the field is added in T340; this task adds the field to
  `PipelineOpts` and leaves it unused). Update the `--allow-major` help on both commands to:
  `lift versioning.bump.stay_at_v0 and the pre-release series major-escalation block for this run`.
- Map the new sentinels to an exit code: `ErrMajorEscalation` and `ErrPreReleaseRegression` come
  out of `Resolve` at run time. Keep them at the Runtime code via `wrapRunErr`'s default (no new
  mapping; spec § 6 defines none). A cmd test pins the exit code.
- Tests: `internal/app/resolver_test.go` (the four usage errors; `calver`/`semver-per-env`/
  `calver-per-env` reject; the option reaches the resolver: with MockRunner,
  `NewResolver(..., WithPreRelease("rc"))` then `Resolve()` → `v1.4.0-rc.1`);
  `internal/cmd/*_test.go` (`version next --pre-release rc` prints the tag on stdout and the
  escalation warning on stderr; `--pre-release` with `--set-version` → `exitcode.Config`;
  invalid label → `exitcode.Config`; `release --pre-release` flag is declared; `changelog` has no
  `--pre-release` flag).
- Docs: `docs/specs/03-commands.md`: `--pre-release` rows for `release` and `version next`, the
  usage-error list, and `--allow-major`'s second trigger with the documented asymmetry
  (`stay_at_v0` downgrades with a warning; the series block errors). Roadmap.

- [ ] Step 1: failing tests. Step 2: RED. Step 3: implement. Step 4: GREEN + full suite.
- [ ] Step 5: Spec 03 + roadmap (flip T339).
- [ ] Step 6: commit `feat(cmd): add --pre-release to release and version next (T339)`.

---

### Task 3 (T340): A pre-release run never writes `CHANGELOG.md`

**Files:**
- Create: `internal/app/prerelease.go` with
  ```go
  // isPreReleaseRun reports whether this run publishes a SemVer pre-release, decided before
  // resolution so step totals and the changelog skip are fixed at build time (ADR-0064).
  func isPreReleaseRun(cfg *config.Config, env string, opts PipelineOpts) bool
  ```
  It returns true when `opts.PreReleaseLabel != ""`, or when the strategy is `semver`/
  `semver-per-env` and `opts.VersionOverride` (stripped like `NewResolver` does: a leading `"v"`
  when `cfg.EffectiveTagFormat(env) != ""`, else `configuredTagPrefix(cfg)`) parses through
  `semver.Parse` with `IsPreRelease()`. It returns false on a parse error, for CalVer, and when
  both inputs are empty.
- Modify: `internal/app/pipeline.go`
  - `buildReleasePipelineConfig`: after the per-env block, `if isPreReleaseRun(...) { pCfg.DisableChangelog = true }`.
    This reuses the `disable_changelog` path the spec names, and `releaseStepTotal` then counts
    correctly with no change. Thread `opts` (or the bool) in; its signature takes individual
    values today, so add a `preRelease bool` parameter.
  - `buildChangelogPipelineConfig`: the same, plus `cCfg.PreRelease = true`.
- Modify: `internal/pipeline/changelog.go`: add `PreRelease bool` to `ChangelogConfig`
  (doc: "the run publishes a SemVer pre-release; set together with DisableChangelog so the skip
  message says why"). In the `DisableChangelog` branch, when `PreRelease`, print
  `ui.Warn(p.out, "pre-release: CHANGELOG.md not updated")` (reporter) /
  `pre-release %s: CHANGELOG.md not updated\n` (plain) instead of "changelog disabled". The tag
  behaviour is unchanged: `--tag` still tags.
- Tests:
  - `internal/app` unit: an `isPreReleaseRun` table (label set; `1.4.0-rc.1`, `v1.4.0-rc.1`,
    `rel-1.4.0-rc.1` with `tag_prefix: rel-`; `1.4.0`; `1.4.0+5`; semver-per-env `7.4.1-rc.1`
    with tag_format `{env}/{version}`; calver with `2026.10.2-0` → false; empty → false).
  - `releaseStepTotal`/`changelogStepTotal` via `BuildPipeline` with `PreReleaseLabel: "rc"` and
    a changelog configured: the total excludes the changelog steps (`steptotal_internal_test.go`).
  - `internal/pipeline` / `internal/app` integration with `MockGenerator`: a pre-release release
    run never calls the changelog generator and never commits; it still tags, pushes and
    publishes, with `CreateReleaseCalls[0].Prerelease == true`. A final run still writes the
    changelog.
  - Review Focus 5: a `--dry-run` pre-release (plain + reporter output) shows no changelog lines
    and no `pre_changelog` hook lines, and the step count matches.
  - `heraut changelog --set-version 1.4.0-rc.1 --tag` (cmd-level, FakeBin or MockRunner as
    existing changelog cmd tests do): tags, writes no section, prints the pre-release message.
    Without `--tag`: exit 0 with the message.
- Docs: Spec 05 § Changelog structure (pre-releases never get a section at cut time; the
  `--set-version X-pre` case is included; remove the interim DOC-1 wording); Spec 04
  § Pre-release tags (drop the "until Phase 2" interim note). Roadmap.

- [ ] Step 1: failing tests. Step 2: RED. Step 3: implement. Step 4: GREEN + full suite.
- [ ] Step 5: Specs 04/05 + roadmap (flip T340; resolve note (a) there as "not handled — no user
  has pre-release sections, decided 2026-10-04").
- [ ] Step 6: commit `feat(app): skip CHANGELOG.md for pre-release runs (T340)`.

---

### Task 4 (T341): Release-notes range for pre-releases; fix the absent-tag fallback

**Files:**
- Modify: `internal/generators/native/generator.go`
  - Split `scopedTags` into `rawScopedTags()` (glob/pattern only) plus the existing ordering
    (`scopedTags` = `tagOrder(rawScopedTags())` when set). The behaviour of `scopedTags` is
    unchanged.
  - `scopedPreviousTag(tag)` when `g.tagOrder != nil`: `raw := rawScopedTags()`; if
    `!slices.Contains(raw, tag)`, `raw = append(raw, tag)`; `return previousInList(tag, g.tagOrder(raw))`.
    Adding the tag being released before ordering means its predecessor is found by precedence
    even when the tag isn't in git yet. This fixes roadmap note (b): cutting `v1.3.1-rc.1` while
    `v2.0.0` exists gives `v1.3.0`, not `v2.0.0`. When the tag is already listed, the output is
    identical to today.
  - New option:
    ```go
    // WithReachableFromHead restricts the scoped tag list to tags reachable from HEAD
    // (git tag -l [glob] --merged HEAD) — a pre-release's notes span back to the previous tag
    // in its own history, never to a tag cut on another branch (ADR-0064).
    func WithReachableFromHead() Option
    ```
- Modify: `internal/generators/native/commits.go`: `listTags(runner, glob string, mergedInto string)`.
  When `mergedInto != ""` the args are `tag -l [glob] --merged <mergedInto> --sort=-version:refname`.
  Every existing call passes `""`, so their args stay identical.
- Modify: `internal/app/current.go`: add
  ```go
  // notesTagOrderFor is tagOrderFor for a pre-release run's release notes: same §11 sort, but
  // pre-release tags are kept — a pre-release's notes span back to the previous tag of any kind.
  func notesTagOrderFor(cfg *config.Config, env string) func([]string) []string
  ```
  (`nil` for CalVer, matching `tagOrderFor`).
- Modify: `internal/app/pipeline.go`: the release-notes generator, when `isPreReleaseRun`, uses
  `notesTagOrderFor` plus `native.WithReachableFromHead()`. Extend `buildGenerator` with a
  trailing `extra ...native.Option`. Finals keep `tagOrderFor` and no reachability filter.
- Tests:
  - native unit/contract (`generator_internal_test.go`): absent-tag insert picks the precedence
    predecessor (`v2.0.0`, `v1.3.0` listed; notes for `v1.3.1-rc.1` with an all-kinds order →
    prev `v1.3.0`); a present tag gives the same result as before (existing T334 rows unmodified);
    `WithReachableFromHead` sends `--merged HEAD` (exact args, with and without a glob); no
    option → args unchanged.
  - app real-repo (`tagorder_realrepo_internal_test.go` or a new
    `notes_prerelease_realrepo_internal_test.go`): `v1.3.0`, `v1.4.0-rc.1`, a new commit, cut
    `v1.4.0-rc.2` → notes range `v1.4.0-rc.1..v1.4.0-rc.2` (only the new commit). Then final
    `v1.4.0` → notes span `v1.3.0..v1.4.0` (both commits). A branch topology where `v1.3.2`
    exists only on a side branch: notes for `v1.4.0-rc.1` on main span from `v1.3.0`, not
    `v1.3.2`.
- Docs: Spec 05 § release-notes ranges (finals: last final; pre-releases: previous tag of any
  kind reachable from HEAD). Roadmap: flip T341, note that (b) and (c) are resolved.

- [ ] Step 1: failing tests. Step 2: RED. Step 3: implement. Step 4: GREEN + full suite.
- [ ] Step 5: Spec 05 + roadmap.
- [ ] Step 6: commit `feat(generators/native): pre-release notes span previous tag (T341)`
  (count: must be ≤ 72; shorten the scope or wording if not).

---

### Task 5 (T342): End-to-end pre-release scenarios on a real repo

Integration coverage from spec § 7, driven through `app.NewResolver` (with `WithPreRelease` /
`WithAllowMajor`) plus a real `exec` runner against `testutil.RealGitRepo`, creating tags the way
the pipeline does (`git tag -a -m`).

**Files:** Create `internal/app/prerelease_realrepo_internal_test.go`. No production code unless a
scenario exposes a bug: fix it under TDD in this task and name it in the roadmap note.

Scenarios (each a subtest that builds its own repo):
1. beta → rc → final: `feat` → `v1.4.0-beta.1`; `fix` → `beta.2`; `--pre-release rc` → `rc.1`;
   no new commit, final → `v1.4.0` (zero-commit promotion works).
2. patch → minor escalation: `fix` → `v1.3.1-rc.1`; `feat` → `v1.4.0-rc.1` plus the escalation
   warning in `Result.Warnings`.
3. blocked major: `feat!` inside the `v1.4.0` series → `errors.Is(err, semver.ErrMajorEscalation)`;
   with `--allow-major` → `v2.0.0-rc.1`.
4. `next` blocked after `rc`: `rc.1` exists, new commit, `--pre-release next` →
   `errors.Is(err, semver.ErrPreReleaseRegression)`.
5. re-cut with no commit: `rc.1` at HEAD, `--pre-release rc` → `no commits since v1.4.0-rc.1`.
6. side-branch tag ignored by the commit requirement: `v1.4.0-rc.1` on a side branch only; on
   main with a new commit since `v1.3.0`, `--pre-release rc` → `v1.4.0-rc.2` (monotonicity is
   global, so the counter continues), and the commit requirement uses `v1.3.0` (reachable), not
   the side-branch rc.
7. `version current` / `--include-pre-release` after scenario 1's rc phase (before the final):
   `v1.3.0` / `v1.4.0-rc.1`.

- [ ] Step 1: write the scenarios. Step 2: run them. Any RED here is a real bug: fix it with TDD
  in this task. Step 3: GREEN + full suite.
- [ ] Step 4: roadmap (flip T342).
- [ ] Step 5: commit `test(app): end-to-end pre-release lifecycle on a real repo (T342)`.

---

### Task 6 (T343): ADR-0064 Phase 2 status update and Phase 2 close

**Files:**
- `docs/adr/0064-semver-v2-compliance.md`: add `## Status update (Phase 2)`. It covers what
  shipped; the previous-tag rule for pre-releases (any kind, reachable from HEAD; finals
  unchanged); "is a pre-release run" keyed on the version, which also covers `--set-version
  X-pre`; the reuse of `disable_changelog` with step totals fixed at build time; the escalation
  warning listing commit subjects rather than hashes (deviation from the spec's
  `(feat abc1234)` example, matching the `stay_at_v0` warning shape); that note (a) was
  deliberately not handled; and the interim DOC-1 paragraph in § Decision now being historical
  (leave it, and point to this update).
- `docs/tasks/semver-v2-roadmap.md`: a "Phase 2 closed" paragraph; the progress table is complete.
- `README.md` / `docs/guides/`: `rtk proxy grep -rn "allow-major\|include-pre-release"`. Wherever
  release flags are listed, add `--pre-release` in the same style. Don't create a new guide.
- Commit: `docs(adr): record SemVer v2 Phase 2 delivery (T343)`.

---

## Roadmap filing (done by the controller before execution, not a task)

- `docs/tasks/semver-v2-roadmap.md` § Phase 2: replace "Not yet broken down" with the T338–T343
  headings (`[ ]`, one-line scope each, pointing to this plan), mark notes (a)–(d) as resolved by
  the decisions above, and add table rows.
- File **T344, "Maintenance-branch support: resolve last final and notes ranges from branch
  history"** (`[ ]`, needs design), in a new "Later" section. It covers: the resolver's
  `git tag -l` is global, so cutting `v1.3.2` on a maintenance branch while `v1.4.0` exists
  computes its core from `v1.4.0`; a final's notes (T334) pick their predecessor by precedence
  only; and the changelog walk. Phase 2's pre-release previous-tag rule is already history-aware
  and is the model.
