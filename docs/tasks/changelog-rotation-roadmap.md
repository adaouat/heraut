# Héraut — Changelog Rotation Roadmap

> Status: Active
> Design: [`docs/superpowers/specs/2026-08-28-changelog-rotation-design.md`](../superpowers/specs/2026-08-28-changelog-rotation-design.md)
> ADRs: new ADR-0047 ("Changelog output resolves after version, not at config-build time" — written in T249)
> Main roadmap: tracked as Phase 29 in [`roadmap.md`](roadmap.md)

This roadmap breaks the **rotating changelog file naming** design (T243) into incrementally
shippable tasks. It lives in its own file per the design doc's own placement decision, matching the
`release-config-roadmap.md` / `docs-audit-roadmap.md` precedent.

`changelog.output` (root and per-env) gains optional versioning-strategy tokens — `{YYYY}`, `{MM}`,
`{DD}`, `{WW}`, `{QQ}`, `{SS}`, `{SPRINT}` for CalVer strategies; `{MAJOR}`, `{MINOR}` for SemVer
strategies — so a project's changelog can rotate into one file per calendar period or per release
line (e.g. `CHANGELOG_{YYYY}.md`, `CHANGELOG_{MAJOR}.md`). A literal `output` with no tokens behaves
exactly as today. Tag-scoping for the rotated file is auto-derived from the same tokens, never a
second field to hand-maintain. Per-env strategies and the `heraut init` wizard are explicitly out of
scope for this pass (see design doc "Non-goals").

## Conventions

- Task IDs **continue the global sequence** (`T244+`) so they never collide with the main roadmap
  or any other dedicated roadmap.
- This file is the **single source of truth** for task status. Same checkbox markers as the other
  roadmaps: `[ ]` not started, `[x]` done. Follow the two-step flow
  ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD: failing test first), then flip
  `[ ]` → `[x]` and add a one-paragraph completion note.
- **No real data** anywhere (samples, docs, tests): synthetic placeholders only.
- Additive config change — no breaking migration, no removed keys. `output` keeps working literally
  for every project that doesn't opt into tokens.
- The main `roadmap.md` Phase 29 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Task | Description                                                                          | Status |
|------|---------------------------------------------------------------------------------------|--------|
| T244 | `calver`: generalize `periodKey` into a caller-scoped prefix-key + literal-prefix-regex helper | Done |
| T245 | `semver`: add `MAJOR`/`MINOR` extraction helper                                        | Done |
| T246 | `internal/config/validator.go`: static token-vocabulary + prefix-order + per-env-rejection checks | Done |
| T247 | `internal/app`: `port.Generator` rotation decorator + wiring into `buildChangelogPipelineConfig` | Done |
| T248 | Integration test: real-git-repo rotation run across a simulated period boundary        | Done |
| T249 | Docs: `schema.json`, `docs/heraut.sample.yml`, spec, new ADR-0047                       | Done |

Sequencing follows the design doc's "Roadmap placement": T244/T245 are pure-function unit tests with
no dependency on each other (can build in either order or in parallel). T246 depends on both (needs
the prefix-order/vocabulary logic they expose). T247 depends on T244–T246 (needs validated,
computable tokens to substitute). T248 depends on T247 (exercises the real wiring end to end). T249
documents the settled behavior and lands last, alongside ADR-0047.

---

## Archived task detail

Every task above is `Done`. Full task write-ups (implementation notes, decisions, deviations)
have been moved to
[`archive/changelog-rotation-roadmap.md`](archive/changelog-rotation-roadmap.md) to keep this
file lean — nothing is lost, just relocated.
