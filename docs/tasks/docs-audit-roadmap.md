# Héraut — Documentation Audit Roadmap

> Status: Active
> Source: four parallel Opus audit passes (2026-08-26) — specs vs. code, ADRs vs. code, and
> `CLAUDE.md`/`.claude/rules/`/`README.md`/`schema.json`/`heraut.sample.yml` vs. code.
> Main roadmap: tracked as Phase 27 in [`roadmap.md`](roadmap.md)

The audit found **142 mismatches** between documentation and current code. Most are pure
documentation drift — the codebase moved on (native-only generator, `forges:`/`release.targets:`,
release-block atomicity, `internal/forge/` HTTP clients) and the docs didn't fully follow. **Seven
are real code bugs** the audit surfaced incidentally, independent of any doc claim being wrong —
those are broken out first and kept separate from doc-only work so they can be prioritized,
reviewed, and tested on their own merits.

## Conventions

- Task IDs **continue the global sequence** (`T222+`) so they never collide with the main roadmap
  or the other dedicated roadmaps (`forge-abstraction-roadmap.md`, `native-generator-roadmap.md`,
  `release-config-roadmap.md`).
- This file is the **single source of truth** for task status. Same checkbox markers as the other
  roadmaps: `[ ]` not started, `[x]` done. Follow the two-step flow
  ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD for code tasks — failing test
  first; doc tasks are verified by re-reading the corresponding code, not by a test), then flip
  `[ ]` → `[x]` and add a one-paragraph completion note.
- **No real data** anywhere (samples, docs, tests): synthetic placeholders only.
- Code-bug tasks (T222–T228) and doc-reconciliation tasks (T229–T238) are independent of each
  other unless a task's own **Dependencies** line says otherwise — a doc task that describes a
  behavior a bug task is about to change should land *after* that bug task, so it documents the
  fixed behavior rather than the bug.
- The main `roadmap.md` Phase 27 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Task  | Description                                                                          | Status      |
|-------|---------------------------------------------------------------------------------------|-------------|
| T222  | `--version`/`--build` override ignores `tag_prefix` / per-env `tag_format`            | Done        |
| T223  | Per-environment `release:` never gets `Notes` default-populated (ADR-0046 gap)        | Done        |
| T224  | Per-driver `rendering.excludes` is never consumed                                     | Done        |
| T225  | Root `versioning.bump` enum and `sprint:` requiredness are unvalidated                | Done        |
| T226  | Environment promotion doesn't filter pre-release source tags like auto-resolve does   | Done        |
| T227  | `heraut init --defaults` overwrites an existing config with no confirmation           | Done        |
| T228  | Asset-glob failure semantics for `release.targets[].assets` (direction TBD)           | Done        |
| T229  | `docs/specs/01-overview.md` reconciliation                                            | Done        |
| T230  | `docs/specs/02-configuration.md` reconciliation                                       | Done        |
| T231  | `docs/specs/03-commands.md` reconciliation                                            | Done        |
| T232  | `docs/specs/04-versioning.md` reconciliation                                          | Done        |
| T233  | `docs/specs/05-generators-and-platforms.md` reconciliation                            | Done        |
| T234  | `docs/specs/06-dx-and-testing.md` reconciliation                                      | Done        |
| T235  | `schema.json` + `docs/heraut.sample.yml` cleanup                                      | Done        |
| T236  | `CLAUDE.md` + `README.md` reconciliation                                              | Done        |
| T237  | `.claude/rules/{coding,testing,workflow}.md` reconciliation                           | Done        |
| T238  | `docs/adr/README.md` + affected ADR bodies — status annotations and stale references  | Done        |

No fixed order within each group. Suggested sequencing: bug tasks first (T222–T228, any order,
disjoint code areas), then doc tasks (T229–T238) so they describe the post-fix behavior rather
than needing a second pass.

---

## Archived task detail

Every task above is `Done`. Full task write-ups (implementation notes, decisions, deviations)
have been moved to [`archive/docs-audit-roadmap.md`](archive/docs-audit-roadmap.md) to keep this
file lean — nothing is lost, just relocated.
