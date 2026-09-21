# ADR-0063: Hold major bumps at v0 — `versioning.bump.stay_at_v0` and `--allow-major`

- **Status**: Accepted
- **Date**: 2026-09-20
- **Deciders**: bchatard
- **Design doc**: [`docs/superpowers/specs/2026-09-20-stay-at-v0-design.md`](../superpowers/specs/2026-09-20-stay-at-v0-design.md)

---

## Context

`DetermineBump` maps any breaking commit — a `type!:` prefix or a `BREAKING CHANGE:` footer — to a
major bump. For a project that is deliberately pre-1.0, that turns a single `feat!:` into an
unintended `v1.0.0`: the next release after the commit declares 1.0 whether or not the maintainers
meant to.

[ADR-0052](0052-versioning-bump-object-and-overrides.md)'s `versioning.bump.overrides` can already
demote breaking commits (`{breaking: true, bump: minor}`), but it is a blunt rewrite rule. It is
silent — nothing tells the maintainer that a major was avoided or which commit forced it. It is
permanent — the rule keeps demoting after the project reaches 1.0, where a breaking change *must*
bump the major. And it has no per-run lift — the only way to release a real major afterwards is to
hand-type the version with `--set-version`.

## Decision

**Hold a major bump back to minor while the current major version is 0, warn about it, and let one
flag lift it for a single run.** `versioning.bump` gains an optional boolean `stay_at_v0` (default
`false`); `heraut release`, `heraut changelog` and `heraut version next` gain `--allow-major`.

- **The rule.** A bump is held back when all four conditions hold: `stay_at_v0` is true and
  `--allow-major` was not passed; the run is automatic (not `--set-version`, not `bump.mode:
  manual` — a `semver` setting that `semver-per-env` environments never read); the current version's major component is `0`; and the resolved bump is `major`. The bump
  then becomes `minor` — `v0.68.0` → `v0.69.0`, not `v1.0.0`. It is applied to the **release-level**
  result, after `bump.overrides`, so an override that yields `major` is held back too, and a project
  that already demotes breaking commits never triggers it. With no tags yet there is no current
  version and nothing to hold. `hold.go` (`internal/versioning/semver`) owns the clamp
  (`holdMajorAtZero`) and the list of commits that forced the major (`majorCommits`, which reuses
  the same per-commit level resolution as `DetermineBump`, so "breaking" has one definition).
- **One shared entry point.** The clamp is applied in a single private `(*Resolver).bumpAfterHold`
  that both `resolveAuto` (`semver`) and `BumpAuto` (the calculator `semver-per-env`'s `bump: auto`
  environments share) call — rather than each of them calling `holdMajorAtZero` itself — so
  `semver-per-env` auto environments get the hold with no separate code path and the two entry
  points cannot drift apart. `bump: promote` environments and the CalVer strategies compute no
  bump from commits, so they are unaffected.
- **The warning.** A held-back bump produces a multi-line warning: a headline, then up to five
  indented commit subjects, then `  … and N more` when there are more.

  ```
  major bump held back by versioning.bump.stay_at_v0: 1.0.0 → 0.69.0 (pass --allow-major to release 1.0.0)
    - feat(cmd)!: scope CLI flags to commands that use them, not root
    - feat(cmd)!: rename --version/--build override flags
  ```

  The versions in it are bare, with no tag prefix or environment: per-env resolvers only ever see
  bare versions (the tag format is applied afterwards), and the same text is then valid for every
  strategy that can hold.
- **Transport.** `versioning.Result` gains `Warnings []string` (one entry per warning; an entry may
  span several lines). The semver resolver records the warning of its last resolution and exposes
  it through `Warnings()`; it never prints, because the resolve step runs inside the pipeline under
  a spinner and a write from the resolver would corrupt it. `app.NewResolver` keeps a pointer to the
  semver calculator it builds — the resolver itself for `semver`, the calculator handed to `perenv`
  for `semver-per-env` — and wraps the result in a small resolver that copies `Warnings()` into
  `Result.Warnings` after a successful `Resolve()`. `perenv.VersionCalculator` is untouched. The
  `release` and `changelog` pipelines print each warning through `ui.WarnLines` (first line as a
  `!` warning, the rest verbatim) right after the "Resolve version" step completes, so it also shows
  under `--dry-run`; `heraut version next` prints it to **stderr**, so stdout stays exactly the tag.
- **`--allow-major`.** A bool flag declared locally on `release`, `changelog` and `version next` —
  not `version current`, which never bumps — reaching the resolver through a variadic option
  (`app.WithAllowMajor`), so `app.NewResolver`'s existing six positional parameters and call sites
  are unchanged. `heraut version next --allow-major` previews the major.
- **A silent no-op when nothing is held back.** Without `stay_at_v0`, with `--set-version`, under
  `bump.mode: manual` (`semver` only), or once the major version is 1 or higher, `--allow-major`
  does nothing and says nothing. Making it an error would fail a wrapper script that always passes it on the day it
  stops being necessary.
- **Rendering is untouched.** The changelog and release notes still mark the commits as breaking;
  only the version number is held back.

## Consequences

- **Additive.** Omitting the key, or the whole `bump:` block, changes nothing. `config.go`,
  `schema.json`, `docs/heraut.sample.yml`, Spec 03 and Spec 04 carry the new key and flag together;
  `versioning.Result` gains a field that only the semver paths populate.
- **Self-retiring.** Once the current major is 1 or higher the setting does nothing, so it can stay
  in the config after the project ships 1.0. heraut's own `.config/heraut.yml` turns it on, so its
  next release is the first real use.
- **A release that would have been `v1.0.0` is `v0.x+1.0`** unless `--allow-major` is passed.
  Maintainers who *want* 1.0 must know to pass it; the warning says so, and `heraut version next
  --allow-major` shows the tag first.
- **The SemVer basis is why the hold is v0-only.** SemVer §4 says anything may change at any time in
  `0.y.z`, so releasing a breaking change as a minor there is legitimate; §8 requires a major bump
  for a backward-incompatible change from 1.0.0 on, so holding one back above 1.0 would make the
  version number lie. That asymmetry is also why a major-bump gate for versions ≥ 1 is a different
  feature with a different shape — an *error* until the run is repeated with `--allow-major`, not a
  demotion. It is recorded as roadmap task T304, deliberately deferred and needing its own design
  pass (setting name and placement, interaction with `stay_at_v0`, per-env behaviour); this ADR does
  not decide it.
- **One more resolver option and one more result field.** Small, but every future resolver that can
  produce warnings goes through the same `Result.Warnings` → `ui.WarnLines` path rather than
  printing on its own.

## Alternatives considered

- **A hard error instead of a hold.** Fail the run when a breaking commit would force `1.0.0`
  unless `--allow-major` is passed. Rejected: it breaks every unattended release containing a
  breaking commit — heraut's own release is a `workflow_dispatch` — which is exactly the situation
  the feature exists for. (For versions ≥ 1 an error *is* the right shape; see T304 above.)
- **Extend `--force`.** Rejected: `--force` already means two unrelated things (bypass E001/E002
  promotion guards, and downgrade `commits.enrichment_policy: required` to `optional`), and the
  repo has precedent for splitting a flag rather than overloading it (T264, `heraut init
  --overwrite`). Someone passing `--force` to recover a promotion should not release a `1.0.0`.
- **A permanent `max_bump: minor` ceiling.** Rejected: it keeps capping after 1.0, contradicting
  SemVer §8, and the maintainer must remember to remove it. `stay_at_v0` retires itself.
- **Widen `perenv.VersionCalculator.BumpAuto`** to return the warning alongside the version.
  Rejected: ADR-0052 declined to widen the same interface for a display detail; the pointer kept by
  `app.NewResolver` reaches the same information without touching the contract every calculator
  implements.
- **Print the warning from the resolver.** Rejected: the resolve step runs under a spinner and a
  direct write corrupts it; the resolver stays side-effect free and the caller decides where and
  when to show it.
- **Do nothing and document the ADR-0052 override.** Rejected: the override is silent, permanent
  and has no per-run lift — the three gaps this ADR exists to close.
