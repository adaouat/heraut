# ADR-0005: Config File Discovery — `.config/heraut.yml` and `.heraut.yml`

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

heraut could read its configuration from `.heraut.yml` at the project root only, but
that is inconsistent with the convention followed by many tools in this ecosystem —
`mise`, `hk`, `typos`, `yamlfmt`, `git-cliff`, and this repo's own tooling all keep their
configs under `.config/`.

For projects that already follow the `.config/` convention, being forced to place
heraut's config at the root creates an exception that needs explaining to every
contributor.

## Decision

heraut supports two config locations and resolves them in order:

| Priority | Location             | When used                       |
|----------|----------------------|---------------------------------|
| 1        | `--config <path>`    | Explicit flag — always honoured |
| 2        | `.config/heraut.yml` | When that file exists           |
| 3        | `.heraut.yml`        | Default fallback                |

`heraut init` applies the same logic to choose where to write:

- If the `.config/` directory exists → writes to `.config/heraut.yml`
- Otherwise → writes to `.heraut.yml`

The resolution is implemented in `internal/config.ResolvePath` and
`internal/config.InitDest`.

## Consequences

**Positive**
- Projects with a `.config/` convention can store heraut config there without friction.
- No breaking change relative to existing `.heraut.yml` users — both paths work.
- `heraut init` writes to the right place automatically; no flag required.
- Consistent with how `mise`, `hk`, and `typos` are configured.

**Negative / trade-offs**
- Two possible config locations can cause confusion if both exist simultaneously
  (`.config/heraut.yml` silently takes precedence; documented in
  [Spec 02 — Configuration § File discovery](../specs/02-configuration.md#file-discovery)).
- The resolution logic must be kept in sync across all commands; it is centralised in
  `internal/config.ResolvePath` to prevent drift.
