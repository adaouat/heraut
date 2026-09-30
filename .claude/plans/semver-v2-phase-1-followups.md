# SemVer v2 — Phase 1 follow-ups (T334, T333) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** close the two Phase 1 code follow-ups: every changelog/release-notes/rotation/compare-link
boundary follows SemVer §11 under SemVer strategies (T334), and `semver-per-env` build IDs must be
valid SemVer build metadata (T333). T332 (manual gh/glab smoke test) is run by the controller, not
in this plan.

**Architecture:** `internal/generators/native` may not import `internal/versioning` (layering), so
the app layer injects a tag-ordering function into the generator via a new `native.WithTagOrder`
option. CalVer strategies inject nothing, so their output is byte-for-byte unchanged. T333 is a
strategy-aware check in `app.NewResolver` (the only place that knows both the strategy and the
build ID; `internal/cmd` checks run before config is loaded).

**Spec:** `docs/superpowers/specs/2026-09-28-semver-v2-compliance-design.md`, ADR-0064
(`docs/adr/0064-semver-v2-compliance.md`), roadmap `docs/tasks/semver-v2-roadmap.md` (T333, T334
entries). User decisions (2026-09-30): T334 approved as designed — pre-release tags no longer get
CHANGELOG sections under SemVer strategies; T333 enforced for SemVer strategies only.

## Global Constraints

- TDD: failing test first (capture RED output), then implementation, then GREEN.
- Never delete or weaken an existing test row; rows whose asserted behaviour deliberately changes
  are edited with a comment citing ADR-0064 and named in the commit body.
- calver / calver-per-env behaviour must not change (no tag order injected for them).
- Errors wrap with `%w`. `internal/cmd` never switches on strategy.
- Conventional commits, subjects ≤ 72 chars; trailer exactly
  `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>`; never a `Claude-Session:` line;
  never `--no-verify`. The `typos` hook rejects "m-i-s" prefixes such as "m-i-s-sorted" (write "out of order").
- Work on `main`. Ignore `.claude/worktrees/`.
- Each task flips its roadmap heading `[ ]` → `[x]`, sets its table row to Done, adds a
  one-paragraph completion note.
- `mise run test` once before each feature commit (final green run after any lint auto-fix);
  `hk check` clean.

## Review Focus

1. A repo with `v1.3.0`, `v1.4.0-rc.1` (on commit A), `v1.4.0+158404` (on commit B) and a new
   commit C: the incremental CHANGELOG section for the next release must list C only (today it
   re-lists B). Pinned in Task 1.
2. Release notes for a final cut after pre-releases must span back to the last final, not to the
   rc (`git describe` topology picks the rc today). Pinned in Task 1.
3. calver-per-env / calver changelogs unchanged. Pinned in Task 1 by existing calver rows staying
   green unmodified.
4. Per-env scoping (TagGlob) and explicit `tag_pattern` still apply before ordering. Pinned in
   Task 1.
5. `semver-per-env --set-build-id build_1` rejected; `calver-per-env --set-build-id build_1`
   still accepted. Pinned in Task 2.

---

### Task 1 (T334): Changelog, release notes, rotation and compare links bound by SemVer precedence

**Files:**
- Modify: `internal/generators/native/generator.go` (new `WithTagOrder` option + `tagOrder` field;
  `scopedTags`, `scopedPreviousTag`, and the oldest-in-scope fallback in `buildAllSections`)
- Modify: `internal/app/pipeline.go` (`buildGenerator` gains a `tagOrder func([]string) []string`
  parameter; its three call sites pass `tagOrderFor(cfg, env)`)
- Modify: `internal/app/changelog_rotation.go` (pass the order into `buildGenerator`; make
  `latestMatchingTag` use it for `semver`)
- Create or modify: an `internal/app` helper `tagOrderFor(cfg *config.Config, env string) func([]string) []string`
  (place it next to `semverExtractor` in `internal/app/current.go`, reusing it)
- Tests: `internal/generators/native/*_test.go` (unit/contract with `exectest.MockRunner`), an
  `internal/app` test for `tagOrderFor`, and one real-git regression test (use the existing
  `internal/testutil.RealGitRepo` helper or the pattern in
  `internal/app/changelog_rotation_realrepo_internal_test.go`)
- Docs: `docs/specs/04-versioning.md` § Pre-release tags, `docs/adr/0064-semver-v2-compliance.md`
  (remove the "until T334" qualifications; state the new behaviour), `docs/specs/05-generators-and-platforms.md`
  (wherever it describes how previous tags / sections are chosen — search for "version:refname"
  and "describe"), roadmap T334.

**Interfaces:**
- `native.WithTagOrder(order func(tags []string) []string) Option` — `order` receives the scoped
  tag list (after TagGlob / TagPattern filtering) and returns the tags to walk, newest-first.
  It may drop tags. nil = today's behaviour.
- `tagOrderFor(cfg, env)` returns nil for calver strategies; for `semver` / `semver-per-env` it
  returns `func(tags) []string` = the `Tag`s of `semver.SortTags(tags, semverExtractor(cfg, env))`
  **excluding pre-releases** (releases only, §11 order, stable ties).

**Behaviour when an order is set (native):**
1. `scopedTags()` applies the order after the existing glob/pattern filtering. This drives the
   historical walk, `newSectionBound` (when no `PreviousTagOverride`), and therefore compare links.
2. `scopedPreviousTag(tag)` returns `previousInList(tag, scopedTags())` instead of `git describe`
   — so release notes for a final span back to the previous *release* by §11. (`previousInList`
   already returns the newest tag when `tag` is not in the list.)
3. In `buildAllSections`, the oldest-in-scope fallback (currently `previousTag(g.runner, t, "")`,
   i.e. unscoped `git describe`) must use the unscoped *ordered* list instead:
   `previousInList(t, order(listTags(runner, "")))`. Keep the existing `git describe` path when no
   order is set.
4. Pre-release tags therefore get no CHANGELOG section and are never a range boundary; their
   commits fold into the next release's section (ADR-0064). Tags that are not valid SemVer are
   dropped from the walk (consistent with the resolver).

**Rotation:** `latestMatchingTag` (changelog_rotation.go) must return the first tag of
`order(list)` when an order applies (strategy `semver`), else today's git-first-line behaviour
(calver). Update its doc comment accordingly.

- [ ] **Step 1: Failing tests** — at minimum:
  - native unit test: with `WithTagOrder` returning a reordered/filtered list, `Generate` in
    changelog mode walks sections in the order returned and bounds each section by the next tag
    in that list (assert the `git log <prev>..<tag>` ranges via `MockRunner` calls).
  - native unit test: release-notes mode with an order set resolves prev via the list (no
    `git describe` call issued).
  - native unit test: no order set → call sequence identical to today (guard).
  - app unit test for `tagOrderFor`: semver drops `v1.4.0-rc.1` and `v1.02.0`, orders
    `v1.4.0+158404` above `v1.3.0`, keeps ties stable; semver-per-env parses through tag_format
    (`uat/1.4.0+5`); calver returns nil.
  - real-git regression (Review Focus 1 and 2): build the repo from Review Focus 1, run the
    changelog pipeline for the next release, assert the new section lists C only; and release
    notes for a final `v1.5.0` tagged after `v1.5.0-rc.1` span back to `v1.4.0+158404`.
  - rotation: `latestMatchingTag` under semver picks `v1.4.0+158404` over `v1.4.0-rc.1`.
- [ ] **Step 2: Run to verify failure** (capture output).
- [ ] **Step 3: Implement** per the behaviour above.
- [ ] **Step 4: GREEN** — focused packages, then `mise run test`; every existing calver and
  native row passes unmodified (if a semver-strategy row changes because it relied on git order
  or on a pre-release section, edit it with an ADR-0064 comment and name it in the commit body).
- [ ] **Step 5: Docs + roadmap** as listed under Files.
- [ ] **Step 6: Commit** e.g. `fix(app): bound changelog and notes by SemVer precedence (T334)`
  (≤ 72 chars), body explains the duplicate-entries bug and the pre-release-section change.

---

### Task 2 (T333): `semver-per-env` build IDs must be valid SemVer build metadata

**Files:**
- Modify: `internal/app/resolver.go` (`NewResolver`, override branch)
- Test: `internal/app/resolver_test.go`, and one `internal/cmd` test through `executeRoot`
- Docs: `docs/specs/03-commands.md` (`--set-build-id` rows), `docs/specs/02-configuration.md`
  (`{build}` section), roadmap T333

**Behaviour:** when `buildID != ""` and `cfg.Versioning.Strategy == "semver-per-env"`, validate
with `semver.Parse("0.0.0+" + buildID)`; on failure return an error wrapping it that says the
build ID must be dot-separated `[0-9A-Za-z-]` identifiers (SemVer build metadata) and names
`--set-build-id`. Classified the same way as sibling `NewResolver` errors (cmd wraps them as
`exitcode.Config`). Also validate the full composed version for semver-per-env the same way
FIX-1 does for plain semver: `semver.Parse(version + "+" + buildID)` (so `--set-version 1.4`
is rejected too). `calver-per-env` and `calver` keep today's lenient `tagfmt.ValidateBuildID`
only. Plain `semver` is already covered (FIX-1) — do not duplicate its check; if the natural
implementation is one shared check for both SemVer strategies, refactor FIX-1's branch into it
without changing its error text or tests.

- [ ] **Step 1: Failing tests:** semver-per-env `--set-version 7.4.1 --set-build-id build_1` →
  error mentioning `--set-build-id`; `7.4.1` + `158404` → `uat/7.4.1+158404`; `7.4.1` +
  `exp.sha.5114f85` accepted; `7.4` + `5` rejected; calver-per-env with build `build_1` still
  accepted (existing behaviour guard). One cmd-level test via `executeRoot("version", "next", …)`
  for the semver-per-env rejection, asserting `exitcode.Config`.
- [ ] **Step 2: RED**, **Step 3: implement**, **Step 4: GREEN + full suite**.
- [ ] **Step 5: Docs + roadmap.**
- [ ] **Step 6: Commit** — breaking for semver-per-env: e.g.
  `feat(app)!: require SemVer build IDs under semver-per-env (T333)` with a
  `BREAKING CHANGE:` footer ("--set-build-id values outside [0-9A-Za-z-] dot-separated
  identifiers are rejected under semver-per-env").
