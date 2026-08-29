# Héraut — Release Config Roadmap

> Status: Active
> Design: [`docs/superpowers/specs/2026-08-26-release-config-simplification-design.md`](../superpowers/specs/2026-08-26-release-config-simplification-design.md)
> ADRs: ADR-0046 (release block atomicity — written in T219) · extends 0043, 0044, 0045
> Main roadmap: tracked as Phase 25 in [`roadmap.md`](roadmap.md)

This roadmap breaks the **release config simplification** epic into incrementally shippable tasks.
It lives in its own file (rather than as a `forge-abstraction-roadmap.md` follow-up) per the design
doc's own placement decision.

`release:` collapses from two independently-optional axes (`notes`, `targets`) into one atomic
intent: block presence (even `release: {}`) means "generate notes and publish them," root and
per-environment alike — no config shape splits the two anymore. `release.notes` stops being an
on/off toggle and becomes a rendering-customization sub-block, default-populated the same way
`changelog: {}` already defaults `Output` to `CHANGELOG.md`. This supersedes T214
(`docs/tasks/forge-abstraction-roadmap.md`, shipped 2026-08-25): its `notesConfigured` synthesis
gate protected a "notes only, no publish" state that traced to nothing observable — `heraut release`
generated the notes string and discarded it whenever there was no publish target, confirmed by
tracing `Run()` end to end, and no command ever surfaced it. T214 remains a correct fix for the
model that existed at the time; this epic removes that model's precondition rather than reusing
T214's mechanism.

## Conventions

- Task IDs **continue the global sequence** (`T216+`) so they never collide with the main roadmap,
  the native-generator roadmap, or the forge-abstraction roadmap.
- This file is the **single source of truth** for task status. Same checkbox markers as the other
  roadmaps: `[ ]` not started, `[x]` done. Follow the two-step flow
  ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD: failing test first), then flip
  `[ ]` → `[x]` and add a one-paragraph completion note.
- **No real data** anywhere (samples, docs, tests): synthetic placeholders only.
- Breaking config change, no deprecation window, matching this project's own pre-v1.0 precedent
  (ADR-0028, T163, ADR-0045) — every task below assumes a hard cutover, not a soft transition.
- The main `roadmap.md` Phase 25 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Task | Description                                                                 | Status      |
|------|-------------------------------------------------------------------------------|-------------|
| T216 | Release atomicity: default-populate `Release.Notes`, remove T214's synthesis gate | Done |
| T217 | Rename per-env `disable_notes` → `disable_release` (removed-key migration)   | Done |
| T218 | Docs: spec 02, sample config, schema                                          | Done |
| T219 | ADR-0046 — "Release block is one intent, not two"                            | Done |
| T220 | `heraut init` wizard: collapse the notes/publish questions, rename per-env `DisableNotes` | Done |

Single pass, not phased — see the design doc's "Implementation sequencing." T216 and T217 touch
disjoint code (loader defaulting + target synthesis vs. a config-key rename) and can be built in
either order or in parallel; T218/T219/T220 document or reflect the result and land last, so they
describe real behavior rather than intent.

---

## Archived task detail

Every task above is `Done`. Full task write-ups (implementation notes, decisions, deviations)
have been moved to [`archive/release-config-roadmap.md`](archive/release-config-roadmap.md) to
keep this file lean — nothing is lost, just relocated.
