# Héraut — Native Content Generator Roadmap

> Status: Active
> ADRs: [ADR-0032](../adr/0032-native-content-generator.md) (generator) · [ADR-0033](../adr/0033-native-config-model.md) (config model) · [ADR-0034](../adr/0034-native-remote-enrichment.md) (enrichment)
> Main roadmap: tracked as Phase 23 in [`roadmap.md`](roadmap.md)

This roadmap breaks down the **native content generator** epic into incrementally shippable
tasks. It lives in its own file because the work is heavy and multi-phase; the main roadmap
keeps only a Phase 23 pointer here.

`native` (`generator: native`) renders changelogs / release notes in pure Go, with no external
`git-cliff` binary. Per [ADR-0033](../adr/0033-native-config-model.md) it is heraut's
**canonical** renderer — git-cliff is dropped as the design anchor (the git-cliff *package*
removal is sequenced after native enrichment, in a follow-up ADR), and rendering is **driven
by config**: a unified `commits:` block (the single source of truth for commit types, also
consumed by `heraut commit verify`/`create`) plus a `rendering:` block. git-cliff's output is
no longer a parity target — heraut's rendering is its own spec, validated by golden snapshots.

## Conventions

- Task IDs **continue the global sequence** (`T122+`) so they never collide with the main
  roadmap's IDs.
- This file is the **single source of truth** for task status. Same checkbox markers as the
  main roadmap: `[ ]` not started, `[x]` done. Follow the same two-step flow
  ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD: failing test first),
  then flip `[ ]` → `[x]` and add a one-paragraph completion note.
- The main `roadmap.md` Phase 23 block is a navigable index only; it carries no checkboxes
  to keep status in one place.

## Progress at a glance

| Phase                                                | Tasks                  | Status      |
|------------------------------------------------------|------------------------|-------------|
| Phase 1 — config model + native canonical renderer   | T122–T126, T130–T136   | Complete    |
| Phase 2 — remote enrichment (GitHub/GitLab CLI, Azure HTTP) | T127, T128, T137, T129 | Complete    |
| Phase 2.6 — native ↔ git-cliff parity (prereq for 2.5) | T138 – T141            | Complete    |
| Phase 2.7 — unified enrichment model                 | T142 – T148            | Complete    |
| Phase 2.8 — user-customizable templates (ADR-0037)   | TT1 – TT11             | Complete    |
| Phase 2.9 — incremental changelog (ADR-0038)          | —                      | Complete    |
| Phase 2.10 — commit-author attribution (ADR-0039)    | T151 (follow-up)       | Complete — GitHub, GitLab, Azure |
| Phase 2.5 — remove the git-cliff package (own ADR)   | T177–T194              | Done        |
| Phase C — wizard simplification (supersedes T164)    | T195–T202              | Done        |
| Phase D — infra housekeeping (Dockerfile / mise / ADR-0016) | T205–T208       | Done        |
| Phase 3 — raw-HTTP clients (drop `gh` / `glab`)       | —                      | Deferred    |

---

## Deferred

### Phase 3 — Raw-HTTP platform clients

**Not scheduled.** Replacing `gh` / `glab` with direct `net/http` platform clients — which
would let heraut drop those binaries entirely and reach a fully self-contained generator +
publisher — is **explicitly deferred behind its own ADR** (see ADR-0032, "Phase 3"). It
reimplements asset upload, pagination, and rate-limit handling the CLIs currently absorb, and
shifts ongoing API-churn maintenance onto heraut. Listed here only so the epic's full arc is
visible; do **not** start it without a follow-up ADR.

## Archived task detail

Every other task in this epic (Phases 1–2.10, 2.5, C, D) is `Done`. Full task write-ups have
been moved to
[`archive/native-generator-roadmap.md`](archive/native-generator-roadmap.md) to keep this file
lean — nothing is lost, just relocated.
