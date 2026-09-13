# ADR-0055: `{{ .Env }}` hook template variable — no per-env `hooks:` config

- **Status**: Accepted
- **Date**: 2026-09-13
- **Deciders**: bchatard

---

## Context

[ADR-0053](0053-release-lifecycle-hooks.md) shipped `hooks:` as a flat, top-level block —
`docs/specs/02-configuration.md` states this explicitly ("A top-level, flat (not
per-environment) block"), and the design doc's own "Non-goals" section named per-env
overrides as out of scope for that pass. A user targeting `semver-per-env`/
`calver-per-env` strategies with `--env` has no way today to make a hook command behave
differently per environment — e.g. `npm publish` for `prod` but a dry-run-only echo for
`staging` — without duplicating the whole `hooks:` block conditionally outside heraut.

Two ways to close this were considered together, the same framing ADR-0054 already used
for the equivalent OS question:

1. **A second config axis**: `environments.<env>.hooks:`, overriding or merging with the
   root `hooks:` block per environment.
2. **A new template variable**: expose the active `--env` value as `{{ .Env }}`, letting a
   single hook command branch internally with `{{ if eq .Env "prod" }}...{{ end }}` — the
   same mechanism `{{ .Platform }}` already provides for `pre_release`/`post_release`.

## Decision

**Add `{{ .Env }}` as a new hook template variable. No `environments.<env>.hooks:` config
key.**

`hookVars` (`internal/pipeline/hooks.go`) gains an `Env` field, populated from the active
`--env` value (`pipeline.Config.Env` / `pipeline.ChangelogConfig.Env`, threaded from
`internal/app/pipeline.go`'s `env`/`opts.Env` parameters) — unlike `Platform`, it is set at
**all six hook points**, not only `pre_release`/`post_release`, since which environment is
active is meaningful throughout the run, not just at publish time. It is empty when the run
targets no environment (flat `semver`/`calver` strategies, or a per-env strategy invoked
without `--env`).

This mirrors ADR-0054's own reasoning for the equivalent Windows-vs-POSIX question almost
exactly: `{{ if eq .Platform ... }}` was already accepted there as proof the templating
engine can express per-axis branching inside one command string, without a second config
axis. `{{ .Env }}` extends that same proof to the per-environment case.

## Consequences

- **Additive, zero config-schema change.** `hooks:` stays exactly as ADR-0053 defined it —
  a flat, optional top-level block. `schema.json` and `docs/heraut.sample.yml` are
  untouched.
- **No merge-semantics question to answer.** A structural `environments.<env>.hooks:` key
  would have needed a decision on whether an env's hook list replaces or merges with root's
  — a real design question with no obvious default. A template variable sidesteps it
  entirely: there is still exactly one `hooks:` block, and branching is the author's choice
  inside a command string, same as `{{ if eq .Platform "github" }}` already is.
- **A future contributor wanting `environments.<env>.hooks:` is not blocked by this ADR** —
  it only says the simpler mechanism ships first. Revisit only if branching inside a single
  command string proves insufficient in practice (e.g. an env needing an entirely different
  *set* of hook points configured, not just different behavior at the same points).
- **Spec 02's "Template variables" table** gains one row; its "flat, not per-environment"
  framing for `hooks:` itself is unchanged and still accurate.

## Alternatives considered

- **`environments.<env>.hooks:` overriding/merging with root `hooks:`.** Rejected for this
  pass: real schema growth, a merge-semantics decision with no obvious default (replace per
  hook point vs. append root+env), and — per the reasoning above — no evidence yet that
  single-string branching via `{{ .Env }}` is insufficient.
- **Restricting `{{ .Env }}` to `pre_release`/`post_release` only, mirroring `Platform`'s
  scope.** Rejected: `Platform` is scoped that way because a publish *target* only exists
  at those two points — there is no "current platform" at `pre_tag`. `Env`, by contrast, is
  a property of the whole run from the moment `--env` is parsed, so restricting it to two
  hook points would be an arbitrary limitation with no equivalent justification.
