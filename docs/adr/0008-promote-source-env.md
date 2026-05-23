# ADR-0008: Explicit `source` Field for `bump: promote` Environments

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

The `semver-per-env` and `calver-per-env` strategies support a `bump: promote` mode
where one environment's latest tag is copied into another environment's tag format.

A naive design would find the source environment by scanning the config map for the
first entry with `bump: auto`. Two problems emerge:

1. **Non-determinism with multiple `auto` environments.** Go map iteration order is
   randomised per process. A config with two `bump: auto` environments (e.g. `dev` and
   `hotfix`) would cause `promote` environments to pick their source randomly on each
   run. A latent data-correctness bug.

2. **No chaining.** The only valid promotion source would be an `auto` environment.
   There would be no way to express a gated pipeline where `prod` promotes from
   `preprod` (which itself promotes from `dev`), forcing all `promote` environments to
   point to the same `auto` source.

## Options Considered

### Option A — `order` field on each environment

Assign an integer priority to each env; `promote` picks the nearest lower-order `auto`
env.

Rejected because:
- "Nearest lower-order auto" is ambiguous when multiple `auto` envs exist below the
  current one.
- Ordering does not express chaining (promoting from a `promote` env).
- Go maps don't preserve YAML insertion order, so implicit ordering would require a
  breaking schema change (`[]EnvEntry` instead of a map).
- An extra field that must be kept globally consistent is more error-prone than a
  single local pointer.

### Option B — explicit `source` field on `bump: promote` environments (chosen)

Add an optional `source: <env-name>` field to `EnvVersioning`. When set, the named
environment is used as the source regardless of its `bump` mode.

Selected because:
- **Explicit beats implicit.** The dependency is declared at the point of use, visible
  without cross-referencing the full env list.
- **Backward-compatible.** Omitting `source` preserves the single-`auto` default.
  Existing configs with one `auto` env continue to work without changes.
- **Enables chaining.** `source` may point to a `promote` env (e.g. `prod` → `preprod`
  → `dev`), impossible with `auto`-only lookup.
- **Statically verifiable.** Unknown source name, self-reference, and cycles are caught
  at `heraut check config` time, not at release time.
- **Familiar pattern.** The same `needs:` / `depends_on:` idiom is used in GitHub
  Actions, GitLab CI, and most DAG-based systems.

## Decision

Add an optional `source` string field to `EnvVersioning`. The resolution rules:

| Condition                                | Behaviour                                                              |
|------------------------------------------|------------------------------------------------------------------------|
| `source` set                             | Use the named environment as the promotion source (any `bump` mode)    |
| `source` omitted, exactly one `auto` env | Use that env (backward-compatible)                                     |
| `source` omitted, zero `auto` envs       | Config validation error                                                |
| `source` omitted, multiple `auto` envs   | Config validation error — must set `source:`                           |

Config validation (in `internal/config/validator.go`) enforces:

- `source` on a `bump: auto` env → error (source is only meaningful for promote)
- `source` pointing to a non-existent env → error
- `source` equal to the env's own name → error
- Cycles in the source chain → error

`--force` does **not** bypass these validation errors; they are config-level mistakes,
not runtime edge cases. The runtime guards E001/E002/E003 (see
[ADR-0007](0007-version-promotion-error-handling.md)) are bypassable by `--force`.

## Consequences

- **Positive**: non-determinism is eliminated; chaining is supported; the config is
  self-documenting.
- **Positive**: configs with exactly one `auto` env never need to set `source:`
  explicitly — the single-`auto` case stays the simple default.
- **Neutral**: configs with multiple `auto` environments must add `source:` to their
  `promote` envs — a one-line change per env, caught by `heraut check config`.
- **Neutral**: `source` can point to a `promote` env, meaning the E001/E002/E003 guards
  compare the destination's tags against the source `promote` env's latest tag (not
  transitively against the ultimate `auto` source). This is intentional: each hop in
  the chain is an independent promotion decision.
- **Implementation**: cycle detection lives in `internal/config/validator.go`; source
  resolution lives in `internal/versioning/perenv/promote.go`.
