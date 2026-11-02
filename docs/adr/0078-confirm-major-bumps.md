# ADR-0078: Confirm major bumps — `versioning.bump.confirm_major`

- **Status**: Accepted
- **Date**: 2026-11-02
- **Deciders**: bchatard

---

## Context

[ADR-0063](0063-hold-major-at-v0.md) holds an automatic major bump back to a minor while the major
version is 0 (`versioning.bump.stay_at_v0`), and lets `--allow-major` lift the hold for one run. It
deliberately stops at 1.0: SemVer §8 requires a major bump for a backward-incompatible change from
1.0.0 on, so a demotion there would make the version number lie. ADR-0063 recorded the case it left
open as roadmap task T304: a project at 1.0 or above that wants every major release to be a
conscious act, so a single `feat!:` merged into an unattended release does not ship `2.0.0`. The
right shape there is a refusal, not a demotion.

[ADR-0064](0064-semver-v2-compliance.md) already gave `--allow-major` a second trigger: moving an
open pre-release series to a new major is an error unless it is passed.

## Decision

**An opt-in setting under which an automatic major bump is refused until the run is repeated with
`--allow-major`.** `versioning.bump` gains an optional boolean `confirm_major` (default `false`).

- **The rule.** A run is refused when `confirm_major` is true and `--allow-major` was not passed;
  the run is automatic (not `--set-version`, not `bump.mode: manual`); and the resolved
  release-level bump is `major`, after `bump.overrides`, as for the hold. It applies at every major
  version: with `confirm_major` alone, `0.9.0` → `1.0.0` is refused too.
- **`stay_at_v0` goes first.** With both settings on and a major version of 0, the hold lowers the
  bump to a minor, so nothing is left to confirm and the run goes on with the hold's warning. From
  1.0 on the hold does nothing and the refusal applies. `--allow-major` lifts both. The two
  settings stay separate keys: folding them into one would rename `stay_at_v0`, a breaking change
  for no gain.
- **Pre-releases.** Opening a pre-release series on a new major (`1.4.0` → `2.0.0-rc.1`) is an
  automatic major and is refused the same way. Moving an open series to a new major is already an
  error under ADR-0064, with or without the setting.
- **Strategies.** It is checked in the same shared entry point as the hold (`bumpAfterHold`), so
  `semver` and the `bump: auto` environments of `semver-per-env` get it with no separate path.
  `bump: promote` environments copy a version approved where it came from, and the CalVer
  strategies compute no bump from commits, so neither is affected. There is no per-environment
  override.
  On a version branch ([ADR-0065](0065-branch-aware-semver-resolution.md)) a major already leaves
  the branch's range and fails; that error stands.
- **Where it stops the run.** The resolver does not fail: it marks the result as an unconfirmed
  major and records the commits that forced it, as the hold does for its warning. `heraut release`
  and `heraut changelog` turn the mark into an error in their "Resolve version" step, before any
  hook, write, commit or tag, so `--dry-run` refuses too. The error names the would-be tag, lists up
  to five of the commits that forced the major, and says to pass `--allow-major`. It exits with the
  Config code (2), like the refusal of a branch missing from `versioning.branches`: a project rule
  stopped the run, nothing is wrong with the machine.
- **`heraut version next` previews.** It prints the major tag on stdout and exits 0, with a warning
  on stderr that `release` and `changelog` would refuse it without `--allow-major`. A preview that
  failed would hide the very number the maintainer wants to see.
- **`--allow-major` stays silent when nothing needs it**, as ADR-0063 decided.

## Consequences

- **Additive.** Omitting the key changes nothing. `config.go`, `schema.json`,
  `docs/heraut.sample.yml`, Spec 02, Spec 03 and Spec 04 carry it.
- **An unattended release with a breaking commit fails** when the setting is on. That is its
  purpose, and why it is opt-in: ADR-0063 rejected the same error at v0 as a default.
- **`versioning.Result` gains the unconfirmed-major mark**, which only the semver paths set.

## Alternatives considered

- **One enum replacing `stay_at_v0`** (for example `on_major: allow | hold_at_v0 | confirm`).
  Rejected: it renames a shipped key, and a project may want both behaviours, the hold below 1.0
  and the refusal above it.
- **Refuse in `heraut version next` too.** Rejected: the preview exists to show the number before
  a run commits to it.
- **The Promotion exit code (4).** Rejected: it means a per-environment promotion guard
  (E001–E003), which this is not.
