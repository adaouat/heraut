# ADR-0068: A missing or unknown `--env` and the branch guard exit with the configuration code

- **Status**: Accepted
- **Date**: 2026-10-09
- **Deciders**: bchatard

---

## Context

With a per-env strategy, `heraut version next` without `--env`, `--env` naming an environment that
is not in `.heraut.yml`, and the per-env branch guard (`environment "prod" must be operated from
branch "main"`) all exited **3** (runtime). Spec 01 reserves 3 for binary, token, network and git
failures, and 2 for configuration problems. Sibling `--env` mistakes (`--env auto` on an unlinked
branch, on a non-per-env strategy) and a missing `{build}` ID already exited 2. The e2e suite found
the inconsistency (T359).

## Decision

All three exit **2**. Which environments exist, and which branch each is linked to, are defined by
the configuration, so naming one that does not exist, or operating one from a branch the config
does not link to it, is a configuration problem rather than an environmental failure. A failure to
*read* the current branch (git itself failing) stays 3.

Implementation: `perenv.ErrEnvNotFound` / `ErrEnvRequired` sentinels and an `app.BranchMismatchError`
type, classified by `app.IsEnvSelection` / `app.IsBranchMismatch` in the cmd layer. Error messages are
unchanged.

## Alternatives considered

- **Usage (1) for a missing `--env`:** the flag is a required argument in a per-env setup, but the
  run cannot tell a forgotten flag from a config that lost its environments, and a single code per
  cause is simpler for scripts.
- **Leave it at 3:** contradicts Spec 01 and the neighbouring `--env auto` errors.

## Consequences

- Scripts that treated exit 3 from these cases as a transient failure and retried will now see 2.
- `version current` classifies the same errors as 2, and `version next` without `--env` now says
  `--env is required for …` like `version current` does.
- `TestVersionNext_SetVersion_StillEnforcesBranchGuard` and the e2e branch-guard row, which pinned
  or left open the old code, now assert 2.
