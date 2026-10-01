# SemVer v2 — Phase 1.5 (remaining compliance) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** close the last two SemVer-compliance gaps, pulled forward from Phase 2 at the user's
request (2026-10-01): `--set-version` must be a valid SemVer v2 version under SemVer strategies
(T336), and GitHub's pre-release flag must be derived from the version instead of a static config
bool, which is removed (T337).

**Architecture:** T336 is a strategy-aware check in `app.NewResolver` (cmd-level checks run before
config is loaded and cannot see the strategy). T337 changes the `port.Platform` contract —
`CreateRelease(tag, notes string, prerelease bool)` — so the pipeline (which may import
`internal/versioning`) decides from the resolved version; platforms stay free of versioning
imports. `release.targets[].prerelease` is removed with a migration hint via the loader's existing
`removedKeys` mechanism.

**Spec:** `docs/superpowers/specs/2026-09-28-semver-v2-compliance-design.md` § 2 (`--set-version`
validation) and § 3–4 (derived GitHub flag, removed key); ADR-0064; roadmap
`docs/tasks/semver-v2-roadmap.md`.

## Global Constraints

- TDD: failing test first (RED output captured), implementation, GREEN.
- Never delete/weaken a test row; rows whose behaviour deliberately changes are edited with a
  comment citing ADR-0064 and named in the commit body.
- calver / calver-per-env: `--set-version` stays unvalidated beyond today's whitespace/empty check;
  a CalVer release is never marked pre-release.
- Layering: platforms import only port/config; pipeline may import versioning; cmd never switches
  on strategy. Changing `port.Platform` updates every implementor in the same commit.
- Config struct changes update `schema.json`, `docs/heraut.sample.yml` (if it shows the key) and
  `testdata/config/` fixtures in the same commit.
- Errors wrap with `%w`. Conventional commits, subjects ≤ 72, trailer exactly
  `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>`, never `Claude-Session:`, never
  `--no-verify`. The typos hook rejects the "m-i-s" prefix (write "out of order").
- Code comments state the *why*; never reference tasks-in-progress, review rounds or fix labels
  (`.claude/rules/coding.md` § Comments) — ADR/T-ids are fine.
- Work on `main`; ignore `.claude/worktrees/`. `mise run test` once before each commit (final run
  after any lint auto-fix); `hk check` clean.
- Each task flips its roadmap heading `[ ]` → `[x]`, sets its table row Done, adds a completion
  note.

## Review Focus

1. `--set-version v1.4.0` (with the configured prefix) and `--set-version 1.4.0-rc.1` keep working
   on plain semver; `uat`-style per-env overrides (`--set-version 7.4.1`) keep rendering through
   tag_format. Pinned in Task 1.
2. `--set-version 2026.05.0` on calver / calver-per-env still accepted unchanged. Pinned in Task 1.
3. A `--set-version 2.0.0-rc.1` release on GitHub passes `--prerelease`; `2.0.0` and every CalVer
   version do not; GitLab args unchanged. Pinned in Task 2.
4. A config still carrying `release.targets[].prerelease` (top-level or per-env) fails with the
   removed-key hint, not a generic strict-decode error. Pinned in Task 2.
5. `heraut init` no longer offers or emits `prerelease`. Pinned in Task 2.

---

### Task 1 (T336): `--set-version` must be valid SemVer v2 under SemVer strategies

**Files:**
- Modify: `internal/app/resolver.go` (`NewResolver`, override branch — before the build-ID checks)
- Tests: `internal/app/resolver_test.go`, `internal/cmd/version_override_test.go`
- Docs: `docs/specs/03-commands.md` (`--set-version` rows for release/changelog/version next),
  `docs/specs/04-versioning.md` § Manual mode, ADR-0064 (move "--set-version SemVer validation"
  from the Phase 2 sketch to Phase 1.5 delivered), roadmap.

**Behaviour:** when `versionOverride != ""` and strategy is `semver` or `semver-per-env`: strip the
prefix the same way the existing code does for that path (configured `tag_prefix` for plain semver
without tag_format; the leading-"v" heuristic on the tag_format path), then:
- if the stripped value contains `+` → error: build metadata is not accepted in `--set-version`;
  hint `pass it with --set-build-id`;
- else `semver.Parse` it; on failure → error wrapping it (`%w`) naming `--set-version`, expected
  `MAJOR.MINOR.PATCH[-pre-release]`.
Accepted: `1.4.0`, `v1.4.0`, `1.4.0-rc.1`. Rejected: `1.4`, `01.4.0`, `1.4.0+abc`.
Surfaced like sibling NewResolver errors (cmd wraps as `exitcode.Config`). CalVer strategies
unchanged.

**Deliberate behaviour change:** `internal/cmd/version_override_test.go` has a row
"set-version already carrying build metadata, no --set-build-id, still works" asserting
`v1.4.0+abc`. Under the spec (§ 2: "build metadata (`1.4.0+5`) rejected, with a hint pointing at
`--set-build-id`") it must now fail: edit the row to assert the error (exit Config, message names
`--set-build-id`), cite ADR-0064 in a comment, name it in the commit body.

- [ ] Step 1: failing tests (table in resolver_test.go covering accepted/rejected values for plain
  semver and semver-per-env; calver and calver-per-env guard rows accepting `2026.05.0`; one cmd
  test via `executeRoot("version", "next", "--set-version", "1.4", …)` asserting exitcode.Config).
- [ ] Step 2: RED. Step 3: implement. Step 4: GREEN + full suite.
- [ ] Step 5: docs + roadmap: add a "Phase 1.5 — Remaining compliance" section to
  `docs/tasks/semver-v2-roadmap.md` (between Phase 1 and Phase 2) with T336 and T337 headings and
  table rows, remove those two items from the Phase 2 scope paragraph, flip T336.
- [ ] Step 6: commit `feat(app)!: validate --set-version as SemVer under SemVer strategies (T336)`
  (≤ 72 chars — shorten if needed) with a `BREAKING CHANGE:` footer.

---

### Task 2 (T337): Derive GitHub `--prerelease` from the version; remove `release.targets[].prerelease`

**Files:**
- Modify: `internal/port/platform.go` — `CreateRelease(tag, notes string, prerelease bool) error`
- Modify every implementor (grep `CreateRelease(` in internal/, including test doubles in
  `internal/testutil/` and any fakes in `_test.go` files): `internal/platforms/github` (pass
  `--prerelease` iff the argument is true; drop the `p.cfg.Prerelease` read), `internal/platforms/gitlab`
  (accept and ignore — GitLab has no pre-release flag; comment says so).
- Modify: `internal/pipeline/release.go` (~line 333): compute
  `prerelease := isPreRelease(result.Version)` once — `semver.Parse(result.Version)` succeeds and
  `IsPreRelease()`; parse failure (CalVer) → false — and pass it to every target.
- Modify: `internal/config/config.go` — remove `Target.Prerelease` and `Platform.Prerelease`;
  `internal/app/platforms.go` stop copying it.
- Modify: `internal/config/loader.go` `removedKeys`/`checkRemovedKeys` — report
  `release.targets[N].prerelease` and `environments.<env>.release.targets[N].prerelease` with a
  hint: "removed — GitHub's pre-release flag is now derived from the version (a SemVer pre-release
  like 1.4.0-rc.1 is published as a pre-release; ADR-0064)". Use the same `ErrRemovedConfigKey`.
- Modify: `internal/scaffold/` (wizard.go, generate.go, dropped.go) — drop the Prerelease field,
  any prompt, emission and the dropped-field report entry; keep the wizard's tests green by editing
  (not deleting) rows that referenced it.
- Modify: `schema.json` (remove `prerelease` from the target definition), `docs/heraut.sample.yml`
  if it shows it, `testdata/config/` — add `invalid/targets_prerelease_removed.yml`; make the
  schema reject it (additionalProperties) and `config.Load` return `ErrRemovedConfigKey`.
- Tests: `internal/platforms/github/*_test.go` (contract: `--prerelease` present only when true),
  `internal/platforms/gitlab/*_test.go` (args unchanged for both values),
  `internal/pipeline/*_test.go` (MockPlatform records the flag: `2.0.0-rc.1` → true, `2.0.0` →
  false, `2026.05.0` → false), `internal/config/migration_test.go` (`TestLoad_RemovedKeys` rows for
  top-level and per-env), schema fixture test, scaffold tests.
- Docs: Spec 02 (remove the key from `release.targets` table/examples), Spec 05 (GitHub driver:
  derived flag; GitLab limitation), ADR-0064 (Phase 1.5 delivered), roadmap.

- [ ] Step 1: failing tests (as listed). Step 2: RED. Step 3: implement (port + all implementors in
  one commit). Step 4: GREEN + full suite.
- [ ] Step 5: docs + roadmap (flip T337; note Phase 1.5 closed).
- [ ] Step 6: commit `feat!: derive GitHub pre-release flag from the version (T337)` with a
  `BREAKING CHANGE:` footer ("release.targets[].prerelease is removed; …").
