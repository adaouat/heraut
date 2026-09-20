# Stay at v0 — hold a breaking-change major bump back to minor while pre-1.0

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-09-20
- **Author**: bchatard (with Claude)
- **Related ADRs**: 0052 (`versioning.bump` object + per-commit overrides — the mechanism this sits
  on top of), 0007 (`--force` promotion-guard semantics — the flag this design deliberately does
  *not* extend)
- **New ADR required**: yes — next available number (0063 as of this writing), "Hold major bumps at
  v0 — `versioning.bump.stay_at_v0` and `--allow-major`."
- **Roadmap**: main `docs/tasks/roadmap.md`, a new Phase 53 with two tasks (T302, T303) — small
  enough that a dedicated roadmap file would be overhead. Not filed yet; filing happens in the
  implementation-planning step after this design is reviewed.
- **Origin**: no roadmap task — direct user request. heraut's own history contains `feat!` commits;
  the project wants to stay in `v0.x`, but the default rule (breaking → major) would resolve the
  next release to `v1.0.0`.

---

## Problem

`DetermineBump` resolves any breaking commit to a major bump. For a project that deliberately
stays pre-1.0, that means the next release after a `feat!:`/`BREAKING CHANGE:` commit is `v1.0.0`,
whether or not the maintainers meant to declare 1.0.

ADR-0052 already lets a project demote breaking changes with `versioning.bump.overrides:
[{breaking: true, bump: minor}]`. That works but is a blunt rewrite rule: it is silent, it never
turns itself off after the project reaches 1.0, and the only way to release a real major is to
hand-type the version with `--set-version`.

## Goals

- One switch, `versioning.bump.stay_at_v0: true`, expresses "I am pre-1.0; breaking changes bump
  minor."
- The hold-back is **visible**: a warning names the commits that would have forced a major, the
  version that was held back and the one released instead, and how to lift it.
- One dedicated flag, `--allow-major`, lifts it for a single run — including previewing it with
  `heraut version next --allow-major`.
- **Self-retiring**: once the current major is 1 or higher the setting does nothing.
- Never fails an unattended run (heraut's own release is a `workflow_dispatch`).

## Non-goals (v1)

- **Not** a hard-error mode. Considered and rejected: a breaking commit would break the pipeline on
  every run. See Resolved questions.
- **Not** an extension of `--force`. It already means two unrelated things (E001/E002 bypass and
  the `enrichment_policy` downgrade), and the repo has precedent for splitting rather than
  overloading (T40, `heraut init --overwrite`).
- **Not** a permanent ceiling (`max_bump: minor`). That would keep capping after 1.0.
- **Not** CalVer, and not `promote` environments — neither computes a bump from commits.
- **Not** a change to changelog/release-notes rendering. Breaking commits stay marked breaking;
  only the version number is held back.
- No per-commit or per-scope granularity — `bump.overrides` already covers that.

---

## Design

### 1. Config

```yaml
versioning:
  strategy: semver
  bump:
    mode: auto
    stay_at_v0: true      # optional, default false
```

`config.BumpConfig` gains `StayAtV0 bool \`yaml:"stay_at_v0,omitempty"\``, read through a nil-safe
accessor `Versioning.StayAtV0()` in the style of `BumpMode()`/`BumpOverrides()`. Additive: omitting
the key, or the whole `bump:` block, changes nothing. `schema.json`, `docs/heraut.sample.yml`,
Spec 02 and Spec 04 are updated together (per `.claude/rules/coding.md`).

No semantic validation beyond the type. It is inert under `bump.mode: manual` and on CalVer
strategies, like `bump.overrides` already is.

### 2. The rule

After the release-level bump is computed (`DetermineBump`, i.e. **after** `bump.overrides`), hold
it back when **all** of these hold:

1. `stay_at_v0` is true and `--allow-major` was not passed;
2. the run is automatic — not `--set-version`, not `bump.mode: manual`;
3. the current version's major component is `0`;
4. the resolved bump is `major`.

The bump becomes `minor`: `v0.68.0` → `v0.69.0`. Because it acts on the release-level result, a
per-commit override that yields `major` is held back too; a project that already demotes breaking
commits to minor never triggers it. With no tags yet there is no current version and nothing to
hold.

### 3. Where it lives

One function in `internal/versioning/semver` — `holdMajorAtZero(currentVersion, bump, ...)` —
called from both `resolveAuto` (`semver`) and `BumpAuto` (the calculator `semver-per-env`'s "auto"
environments share). That is the whole reason per-env needs no separate code path.

A helper `majorCommits(commits, overrides)` returns the subjects of the commits whose own level is
`major`, for the warning. It reuses `resolveBumpLevel`; no second definition of "breaking".

### 4. Warning transport

Two constraints: `perenv.VersionCalculator.BumpAuto` returns only `(string, error)` and ADR-0052
already declined to widen it for a display detail; and the resolve step runs inside the pipeline,
under a spinner, so printing straight to stderr from the resolver would corrupt it.

- `versioning.Result` gains `Warnings []string`.
- `semver.Resolver` records the warnings it produced during the last resolution and exposes them
  through `Warnings() []string`. It never prints.
- `app.NewResolver` keeps a pointer to the semver resolver it builds (directly for `semver`, as the
  calculator for `semver-per-env`) and returns it wrapped in a small resolver that copies
  `Warnings()` into `Result.Warnings` after a successful `Resolve()`. The `VersionCalculator`
  interface and `perenv` are untouched.
- The `release` and `changelog` pipelines already build the "Resolve version" step's sub-lines
  (`runStep` returns `(detail, subs, err)`); they return `Result.Warnings` as `subs`, the same
  mechanism the enrichment-degraded notes use. It therefore also appears under `--dry-run`, which
  runs the resolve step for real (read-only git calls only).
- `heraut version next` prints each warning to **stderr**, so stdout stays exactly the tag.

Warning shape (first line is the headline; the rest are indented commit subjects, at most five,
then "… and N more"):

```
major bump held back by versioning.bump.stay_at_v0: v1.0.0 → v0.69.0 (pass --allow-major to release v1.0.0)
  - feat(cmd)!: scope CLI flags to commands that use them, not root
  - feat(cmd)!: rename --version/--build override flags (T263)
```

### 5. `--allow-major`

A bool flag declared locally on `release`, `changelog` and `version next` — not `version current`,
which never bumps — following the "declare on exactly the commands that use it" rule (T266). It
reaches the resolver through a variadic option on `app.NewResolver`
(`app.WithAllowMajor(bool)`), so the existing six-parameter signature and its call sites/tests
are unchanged.

It is a silent no-op when nothing is held back: without `stay_at_v0`, with `--set-version`, under
`bump.mode: manual`, or once the major is ≥ 1. Making it an error would make a wrapper script that
always passes it fail on the day it becomes unnecessary.

### 6. Dogfooding

The last step of T303 adds `stay_at_v0: true` under `versioning.bump` in `.config/heraut.yml`, so
heraut's own next release is the first real use.

---

## New ADR-0063 outline

**Title:** Hold major bumps at v0 — `versioning.bump.stay_at_v0` and `--allow-major`.

**Context:** ADR-0052's overrides can demote breaking changes but silently, permanently, and with
no per-run lift.

**Decision:** the rule in §2, the flag in §5, the transport in §4, and the explicit non-decisions:
no hard-error mode, no `--force`, no permanent ceiling.

**Consequences:** additive; self-retiring at 1.0; `Result` gains `Warnings`; one more resolver
option; a release that would have been `v1.0.0` is `v0.x+1.0` unless the flag is passed, so
maintainers who *want* 1.0 must know to pass it (the warning says so).

**Alternatives considered:** hard error (breaks unattended pipelines); overloading `--force`
(already two unrelated meanings); a permanent `max_bump: minor` (outlives the pre-1.0 phase);
widening `VersionCalculator.BumpAuto` (ADR-0052 declined the same change); printing the warning
from the resolver (corrupts the spinner); doing nothing and documenting the ADR-0052 override.

## Roadmap placement

New **Phase 53 — Stay at v0** in `docs/tasks/roadmap.md`, two tasks:

- **T302** — `internal/config` + `internal/versioning/semver` + `internal/versioning`: `StayAtV0`
  field/accessor, `schema.json` and sample, `holdMajorAtZero` + `majorCommits`, `Result.Warnings`,
  the recording `Warnings()` accessor. TDD, unit level. The setting is functional but has no flag
  to lift it yet, so nothing wires it into a released binary's docs until T303.
- **T303** — `internal/app` + `internal/cmd` + `internal/pipeline` + docs: `WithAllowMajor`, the
  warning-copying resolver wrapper, `--allow-major` on the three commands, pipeline sub-lines,
  `version next` stderr, real-repo tests, Spec 02/03/04, ADR-0063 + index, `CLAUDE.md` counts,
  guide mention if any, and the `.config/heraut.yml` dogfood line.

Per `.claude/rules/claude.md`, one task per session unless the user approves more; these two are
sequential and T303 depends on T302.

## Testing plan

- **Unit, table-driven (`semver`):** `holdMajorAtZero` — `0.68.0` + major → minor; `0.0.5` + major
  → minor; `1.0.0` + major → major (self-retiring); `0.x` + minor/patch untouched; flag off;
  `--allow-major`; per-commit override yielding `major` is held back; override demoting breaking to
  minor triggers nothing; no-tags-yet returns the initial version untouched. Preserve the existing
  `v1.9.0 → v1.10.0` and override rows verbatim (project rule: never drop a hard-won row).
- **Unit (`semver`):** `majorCommits` returns only major-level subjects, honours overrides, and the
  warning caps at five plus "… and N more".
- **Config:** `StayAtV0` accessor is nil-safe; a `testdata/config/valid/` fixture with the key
  validates against `schema.json`.
- **Contract/integration (`internal/app`):** `NewResolver` + `WithAllowMajor` end to end against a
  `MockRunner`; `semver-per-env` "auto" environment holds back too and a `promote` environment is
  untouched; `Result.Warnings` populated only when held back.
- **Integration (`internal/cmd`, real git repo):** `version next` prints only the tag on stdout with
  the warning on stderr, and `--allow-major` prints `v1.0.0`; a `changelog --tag --no-push` run
  tags `v0.x+1.0` and shows the warning as a sub-line of the resolve step.
- **Mutation check:** removing the `holdMajorAtZero` call must fail at least the resolver and the
  real-repo tests.

## Resolved questions

- **Hold back vs. hard error?** Hold back to minor + warn (user's call). A hard error would fail
  every unattended release that contains a breaking commit — the exact situation the feature exists
  for.
- **`--force` or a new flag?** New: `--allow-major`. `--force` is already overloaded.
- **Permanent cap or v0-only?** v0-only, so it retires itself at 1.0 (precedent: release-please
  `bump-minor-pre-major`, commitizen `major_version_zero`).
- **Names.** `versioning.bump.stay_at_v0` and `--allow-major` — proposed and approved in
  conversation.
- **Per-env?** Yes for `semver-per-env` "auto" environments (they share `BumpAuto`); no for
  `promote` environments and CalVer.
