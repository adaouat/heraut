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
| T324 | Roadmap, Phase 59 pointer, ADR-0064 | Done |
| T325 | `semver.Version`: strict Parse, §11 Compare, SortTags/Latest | Done |
| T326 | Plain `semver` resolver orders tags by §11 in Go | Not started |
| T327 | `semver-per-env` ordering + E002 via §11; calver-per-env unchanged (zero-padded CalVer is not SemVer) | Not started |
| T328 | `{build}` must directly follow `+` (validator, wizard, docs) | Not started |
| T329 | `--set-build-id` on plain `semver` (`v1.4.0+<id>`) | Not started |
| T330 | `version current`: latest final by default, `--include-pre-release` (+ `${version}` test-typo fix, own commit) | Not started |
| —    | Phase 2 (pre-release lifecycle) — planned after Phase 1 lands | Not planned |

## Phase 1 — Compliance

### [x] T324 — Roadmap, Phase 59 pointer, ADR-0064

Filed this dedicated roadmap, the Phase 59 pointer block in `docs/tasks/roadmap.md` (status
table row + navigable index block, no checkboxes there), and
[ADR-0064](../adr/0064-semver-v2-compliance.md) recording the compliance decisions and the two
clean breaks. Phase 2 (pre-release lifecycle) is intentionally left unbroken-down in this file —
its scope is sketched under "Phase 2" below and will be decomposed into its own tasks once Phase
1 lands, per the design doc's delivery plan.

### [x] T325 — `semver.Version`: strict Parse, §11 Compare, SortTags/Latest

Added `internal/versioning/semver/version.go` with `Version`, `Parse` (strict SemVer 2.0.0
grammar per §9/§10, rejecting leading zeros in both the core and numeric pre-release
identifiers), `String`/`Core`/`IsPreRelease`, `Compare` (§11 precedence, build metadata
ignored), and `SortTags`/`Latest` for scheme-agnostic tag selection. Numeric pre-release
identifiers compare by digit-string length then lexically, so arbitrarily long identifiers
(e.g. `rc.99999999999999999999`) never overflow `uint64`, while a `MAJOR`/`MINOR`/`PATCH`
segment that large is a parse error per the fixed-width core grammar. `hk fix -S
golangci_lint` auto-applied a De Morgan's-law rewrite (QF1001/staticcheck) to the character
class check in `splitIdentifiers`; no other lint findings. Nothing is wired into the
resolvers yet — that starts at T326.

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
