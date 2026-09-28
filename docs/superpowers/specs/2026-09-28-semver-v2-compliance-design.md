# SemVer v2 compliance — full grammar, §11 precedence, and a pre-release lifecycle

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-09-28
- **Author**: bchatard (with Claude)
- **Supersedes**: `2026-09-24-semver-v2-compliance-exploration.md` (untracked exploration notes —
  this design was re-derived from a blank page and deliberately narrows that scope: no
  auto-incrementing build counter, no pre-release minting under `semver-per-env`)
- **Related ADRs**: 0063 (`stay_at_v0` / `--allow-major` — the flag this design extends), 0008
  (per-env `source:` / promotion), 0007 (`--force` promotion guards), 0018 (`--set-version`
  bypass), 0038 (incremental changelog)
- **New ADR required**: yes — next available number (0064 as of this writing), "SemVer v2
  compliance and pre-release lifecycle."
- **Roadmap**: dedicated `docs/tasks/semver-v2-roadmap.md` (two phases), with a pointer from
  `docs/tasks/roadmap.md`. Not filed yet; filing happens in the implementation-planning step.
- **Origin**: no roadmap task — direct user question ("which SemVer do we follow, v1 or v2?").
  Motivation is twofold: a **real need** to ship pre-releases, both a human-driven RC phase and a
  CI-driven continuous channel; and a **correctness bar** — heraut says "semver," that should be
  strictly true.

---

## Problem

heraut correctly implements the bare `MAJOR.MINOR.PATCH` subset of SemVer v2 and nothing more:

- `semver.IsBareVersion` is the only version recognizer. Tags carrying a pre-release (`-rc.1`) or
  build metadata (`+5`) are silently skipped during resolution (`semver/resolver.go`,
  `perenv/auto.go`, `perenv/promote.go`).
- There is no §11 precedence comparator. Tag ordering relies on git's `--sort=-version:refname`,
  which misorders pre-releases relative to finals unless the user configured
  `versionsort.suffix` — the reason the skip policy exists (Spec 04 § Pre-release tags).
- heraut cannot mint a pre-release at all.
- The documented `{build}` usage is itself non-compliant: `tag_format: "{env}/{version}-{build}"`
  renders `uat/7.4.1-158404`, which SemVer reads as a **pre-release** of `7.4.1` (lower precedence
  than `7.4.1`), not as build metadata.
- `--set-build-id` only works for per-env strategies; plain `semver` has no equivalent.
- GitHub's pre-release flag is a static per-target config bool (`release.targets[].prerelease`)
  that can contradict the version being published.

## Goals

- heraut parses, orders, and emits versions exactly per SemVer v2 (§9 pre-release, §10 build
  metadata, §11 precedence) — for every strategy that uses SemVer (`semver`, `semver-per-env`).
- The plain `semver` strategy can mint pre-releases (`1.4.0-rc.1`), covering both a human-driven
  RC phase and a CI-driven continuous channel with **one** mechanism.
- Published pre-release tags are **always monotonic**: no tag is ever created with lower §11
  precedence than an existing tag of the same core.
- Build metadata has exactly one meaning (`+…`) and one entry point (`--set-build-id`).

## Non-goals

- **Pre-release minting under `semver-per-env`.** Environments already are per-env's staging
  model; stacking RC labels on `dev → uat → prod` doubles the concepts for an unrequested use case.
  Per-env gets the compliance core only. Additive later (e.g. a per-env gate on promoting
  pre-releases) if a real user needs it.
- **CalVer strategies.** No bump is computed from commits, so a pre-release channel doesn't map.
- **An auto-incrementing build counter.** Tags differing only in build metadata have identical
  precedence (§10) — a tag per CI build clutters the tag list with entries no SemVer tool can
  distinguish. CI run numbers belong on artifacts; `--set-build-id` covers the tag case.
- **Branch → label mapping in config.** The label is a CLI flag; CI workflows already know their
  branch. Additive later.
- **Pre-release sections in `CHANGELOG.md`.**
- **Lifting `--set-build-id`'s `--set-version` requirement.** Parity keeps today's rule.
- Any state outside git tags (existing invariant).

---

## Design

### 1. Versioning model (plain `semver`)

Definitions: the **last final** is the highest-precedence tag with no pre-release identifiers;
the **core** of a version is its `MAJOR.MINOR.PATCH`; a **series** is the set of pre-release tags
sharing a core.

**Final release (no `--pre-release`)** — unchanged from today in its computation: the core is the
bump over all commits since the last final (`stay_at_v0` applied). Pre-release tags are never used
as the bump base. Promoting `1.4.0-rc.2` → `1.4.0` needs **zero** new commits, because the commits
since the last final still justify the bump.

**Pre-release (`--pre-release <label>`)**:

1. **Compute the core** exactly as a final would: bump over all commits since the last final,
   `stay_at_v0` applied. (No final yet → seeded from `initial_version`, as today.)
2. **Compare against the open series.** Let *S* be the highest existing pre-release whose core is
   above the last final. If the new core is higher than *S*'s core, the bump level escalated:
   - escalation that introduces a **new major** → **error**, unless `--allow-major`;
   - escalation to a **minor or patch** → allowed, with a warning naming the commit(s) responsible,
     e.g. `! pre-release core escalated 1.3.1 → 1.4.0 (feat abc1234)`.

   Because the core is always computed from the last final, a fix or feat landing inside a
   `1.4.0` series keeps the core at `1.4.0` (`beta.1` → `beta.2`), never `1.4.1`. The core only
   moves when the whole batch's bump level rises — in practice, a patch series receiving a feat,
   or any series receiving a breaking change. Locking the core instead would ship a final patch
   release containing a feature, which SemVer forbids; so escalation is not configurable.
3. **Build the candidate**: `<core>-<label>.<N>`, where *N* is the highest existing counter for
   this core and label plus one, or `1` if none.
4. **Monotonicity check**: the candidate must have strictly higher §11 precedence than **every**
   existing tag with the same core — including the final of that core if it exists (no
   pre-releases of an already-shipped core). Violations error with a hint, e.g.
   `1.4.0-next.8 would sort below existing 1.4.0-rc.1 — ship 1.4.0 or use a label that sorts
   higher`. One rule covers label regression (`beta` → `alpha`), cross-channel collisions (a
   `next` channel after an `rc` of the same core is cut), and counter reuse. Consequence, accepted
   deliberately: a continuous channel on the same core is blocked during an RC phase until the
   final ships.
5. **Commit requirement**: at least one commit since the **previous tag** (pre-release or final).
   Re-cutting an identical pre-release is an error, mirroring today's "nothing to release" rule.
   (Promotion to final is exempt — see above.)

**Label grammar**: a single SemVer identifier — `[0-9A-Za-z-]+`, not purely numeric, no dots.
heraut always appends `.N`; the dotted numeric counter keeps `rc.2 < rc.11` correct under §11.
Label ordering is §11's ASCII ordering, not a hardcoded list: `alpha < beta < rc` works as
expected; `dev` sorts *above* `alpha` — that is SemVer's rule and heraut follows it.

**Worked examples** (last final `v1.3.0`):

| Commits since `v1.3.0`         | Existing tags       | Invocation              | Result                                  |
|--------------------------------|---------------------|-------------------------|-----------------------------------------|
| `feat: X`                      | —                   | `--pre-release beta`    | `v1.4.0-beta.1`                         |
| + `fix: bug`                   | `beta.1`            | `--pre-release beta`    | `v1.4.0-beta.2`                         |
| (same)                         | `beta.2`            | `--pre-release rc`      | `v1.4.0-rc.1`                           |
| (same)                         | `rc.1`              | `--pre-release beta`    | error: `beta.3` would sort below `rc.1` |
| (none new)                     | `rc.1`              | `--pre-release rc`      | error: no commits since `v1.4.0-rc.1`   |
| (none new)                     | `rc.1`              | *(final)*               | `v1.4.0`                                |
| `fix: A`                       | —                   | `--pre-release rc`      | `v1.3.1-rc.1`                           |
| + `feat: B`                    | `v1.3.1-rc.1`       | `--pre-release rc`      | `v1.4.0-rc.1` + escalation warning      |
| + `feat!: C`                   | `v1.4.0-rc.1`       | `--pre-release rc`      | error: major escalation                 |
| (same)                         | `v1.4.0-rc.1`       | `… --allow-major`       | `v2.0.0-rc.1` + escalation warning      |

### 2. CLI surface

**`--pre-release <label>`** on `heraut release` and `heraut version next` (preview). Not on
`heraut changelog` — a pre-release never touches `CHANGELOG.md`.

Usage errors (config-error exit code):

- `--pre-release` with any strategy other than `semver`;
- `--pre-release` with `--set-version` (two ways to choose the version);
- `--pre-release` under `versioning.bump.mode: manual` (no computed core);
- an invalid label (grammar above).

**`--allow-major`** gains a second trigger: allowing a major escalation of an open pre-release
series. One flag lifts both `stay_at_v0` and the series block. The spec documents the asymmetry:
the `stay_at_v0` hold **downgrades with a warning**; the series block **errors**.

**`--set-version`** is validated as a SemVer v2 version (today it is accepted as-is):

- `1.4.0` and `1.4.0-rc.1` accepted — a pre-release value is the manual escape hatch (e.g.
  retargeting a series by hand);
- build metadata (`1.4.0+5`) rejected, with a hint pointing at `--set-build-id`;
- it remains a bypass: no monotonicity check, no git calls, as today.

**`--set-build-id` on plain `semver`** (parity): renders `<tag_prefix><version>+<id>`
(`v1.4.0+158404`). Still requires `--set-version`, exactly like per-env.

**`heraut version current`**: prints the latest **final** by default (unchanged meaning for
existing scripts). New boolean **`--include-pre-release`** prints the highest §11-precedence tag
including pre-releases. Available for `semver` and `semver-per-env` (per-env repos may carry
pre-release tags from other tooling). Composes with `--bare` (`1.4.0-rc.2`). Named distinctly from
`--pre-release` to avoid one flag name carrying a value on one command and a boolean on another.

### 3. Configuration (clean breaks — no known real config affected)

- **`{build}` placement**: in any `tag_format`, `{build}` must be immediately preceded by `+`
  (`"{env}/{version}+{build}"`). Any other placement is a semantic validation error with a hint.
  `ParseVersion` and the tag-listing glob follow the `+` form.
- **`release.targets[].prerelease` removed.** GitHub's pre-release flag is derived from the version
  (§4). A static bool can only contradict the version. Strict YAML parsing rejects the key; the
  validator/loader error carries a hint explaining the derivation.
- `schema.json`, `docs/heraut.sample.yml`, and `testdata/config/invalid/` updated in the same
  change as each struct/validator change.

### 4. Publishing a pre-release

A `heraut release --pre-release <label>` run: resolve → tag → push → publish. Specifically:

- **No `CHANGELOG.md` update and no changelog commit** — reuses the `disable_changelog` skip path.
  The tag lands on current `HEAD`.
- **Release-notes body**: commits since the **previous tag** (pre-release or final), rendered with
  the normal release-notes template — `rc.2`'s notes show only what changed since `rc.1`.
- **Final after pre-releases**: the `CHANGELOG.md` section and release notes for `1.4.0` span back
  to the **last final** (`v1.3.0`); pre-release tags are never range boundaries for a final.
- **GitHub**: `gh release create --prerelease` is passed iff the version has pre-release
  identifiers.
- **GitLab**: the Release is created as usual; GitLab has no pre-release flag — documented as a
  platform limitation.
- Hooks run as usual. No new template/hook variables (YAGNI).

### 5. Compliance core internals

In `internal/versioning/semver`:

- `Version` type and `Parse(s string) (Version, error)` implementing the full SemVer v2 grammar:
  numeric identifiers without leading zeros, no empty identifiers, `[0-9A-Za-z-]` only.
  `IsBareVersion` is replaced by parsing plus "no pre-release, no build metadata."
- `Compare(a, b Version) int` implementing §11: build metadata ignored; numeric identifiers
  compared numerically and always below alphanumeric; alphanumeric compared in ASCII order; a
  longer identifier list wins when the shared prefix is equal; a pre-release is below its final.
- **Tag ordering moves into Go**: tags are listed without relying on `--sort=-version:refname` and
  sorted with `Compare`. The `versionsort.suffix` caveat in Spec 04 § Pre-release tags goes away.

Per-env (`perenv/auto.go`, `perenv/promote.go`): still skips pre-release tags for resolution (it
does not mint them), but uses `Compare` for E001/E002 and for `--include-pre-release`, and parses
tags through the `+`-only `{build}` rule.

Layering is unchanged: everything lives in `internal/versioning/*`, `internal/config/`,
`internal/pipeline/`, `internal/platforms/github`, and `internal/cmd/`, within the existing import
rules.

### 6. Error handling

All new failures are wrapped with `%w` and classified with `errors.Is`/`errors.As`:

- `ErrMajorEscalation` (series major escalation without `--allow-major`) — message names the
  current series, the escalated core, the breaking commit(s), and the flag.
- `ErrPreReleaseRegression` (monotonicity) — names the candidate and the existing tag it would
  sort below, with the "ship the final or use a higher label" hint.
- Commit requirement: reuses today's unwrapped `no commits since <tag> — …` error from
  `semver/resolver.go`, now naming the previous pre-release tag when there is one. No sentinel
  exists today and none is added — nothing needs to classify it.
- Invalid label / invalid `--set-version` / incompatible flags → usage/config errors with hints.

### 7. Testing

TDD throughout, following the four layers:

- **Unit**: table-driven §11 cases including the spec's own chain (`1.0.0-alpha < 1.0.0-alpha.1 <
  1.0.0-alpha.beta < 1.0.0-beta < 1.0.0-beta.2 < 1.0.0-beta.11 < 1.0.0-rc.1 < 1.0.0`), build
  metadata ignored for precedence, grammar rejections (`1.02.0`, `1.0.0-01`, `1.0.0-`, `1.0.0+`,
  `1.0.0-a..b`); series rules — counter, escalation warning, major block and `--allow-major`,
  monotonicity error, commit requirement; label validation; `--set-version` validation; `{build}`
  placement validation.
- **Integration** (`RealGitRepo`): beta → rc → final; patch → minor escalation; blocked major plus
  `--allow-major`; `next` blocked after `rc`; zero-commit promotion to final; final notes spanning
  back to the last final; `version current` with and without `--include-pre-release`.
- **Contract** (`MockRunner`): `gh release create` receives `--prerelease` only for a hyphenated
  version; `glab` args unchanged; git tag-listing args updated for Go-side sorting.
- **Schema**: invalid fixtures for `{version}-{build}` and for the removed
  `release.targets[].prerelease`.
- Existing edge-case rows (e.g. `v1.9.0` → `v1.10.0`, current pre-release skip behaviour) are kept;
  any row whose asserted behaviour deliberately changes is changed under the new ADR.

### 8. Documentation

- New ADR (0064): SemVer v2 compliance and pre-release lifecycle — records the floating core,
  major-escalation block, global monotonicity, label-by-flag, per-env/CalVer exclusion, no build
  counter, and the two clean breaks.
- Spec 02 (`{build}` rule, removed `prerelease` key), Spec 03 (`--pre-release`,
  `--include-pre-release`, `--allow-major`, `--set-version`, `--set-build-id`), Spec 04 (rewrite of
  § Pre-release tags, new § Pre-release lifecycle, corrected `{build}` examples), Spec 05 (GitHub
  derived flag, GitLab limitation, notes ranges).
- `schema.json`, `docs/heraut.sample.yml`.

---

## Delivery

Dedicated roadmap `docs/tasks/semver-v2-roadmap.md`, two phases. Each phase ships on its own.

**Phase 1 — Compliance** (no new tags minted; existing behaviour unchanged apart from the clean
breaks):

- `semver.Version`, `Parse`, `Compare`; Go-side tag sorting.
- Per-env uses `Compare`; `{build}` `+`-only rule in validator, `tagfmt`, and globs.
- `--set-build-id` parity on plain `semver`.
- `heraut version current --include-pre-release`.
- ADR-0064 and the Spec 02/03/04 updates for this phase.

**Phase 2 — Pre-release lifecycle**:

- `--pre-release <label>` on `release` and `version next`; series rules (§1).
- `--allow-major` second trigger.
- `--set-version` SemVer validation, accepting pre-release values.
- Pipeline: changelog skip for pre-releases, notes ranges (previous tag vs. last final).
- GitHub derived `--prerelease`; removal of `release.targets[].prerelease`.
- Spec 03/04/05 updates for this phase.
