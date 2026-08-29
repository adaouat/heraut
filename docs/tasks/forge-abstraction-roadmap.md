# Héraut — Forge Abstraction Roadmap

> Status: Active
> Design: [`docs/superpowers/specs/2026-07-24-forge-abstraction-design.md`](../superpowers/specs/2026-07-24-forge-abstraction-design.md)
> ADRs: ADR-0043 (forge abstraction + config unification — written in T154) · extends/supersedes
> 0006, 0020, 0023, 0025, 0026, 0034, 0035, 0039, 0040, 0041, 0042
> Main roadmap: tracked as Phase 24 in [`roadmap.md`](roadmap.md)

This roadmap breaks the **forge abstraction** epic into incrementally shippable tasks. It lives in
its own file because the work is heavy and multi-phase; the main roadmap keeps only a Phase 24
pointer here.

A single top-level **`forges:`** list (a forge = one code-hosting platform heraut talks to) replaces
`changelog.remote` + `release.platforms`. A new **`port.Forge`** owns three responsibilities —
resolve its identity from the environment, build links, and fetch enrichment metadata. Identity
**auto-configures from CI env or git `origin`** (fail loud on ambiguity), so a changelog / release
notes render fully enriched **with zero config in CI**. GitLab gains a native `net/http` enricher
(REST default, `JOB-TOKEN`-aware) so `CI_JOB_TOKEN` enriches without a manually-created PAT — with
an opt-in GraphQL path (`api_mode: graphql`) for linked commit-author handles. Consumers reference a
forge by name: `commits.enrichment_forge` (enrichment source) and `release.targets[].forge` (publish
targets); `commits.remote_metadata` is renamed `commits.enrichment_policy`.

## Conventions

- Task IDs **continue the global sequence** (`T154+`) so they never collide with the main roadmap or
  the native-generator roadmap.
- This file is the **single source of truth** for task status. Same checkbox markers: `[ ]` not
  started, `[x]` done. Follow the two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)):
  implement (TDD: failing test first), then flip `[ ]` → `[x]` and add a one-paragraph completion
  note.
- **No real data** anywhere (samples, docs, tests): synthetic placeholders only
  (`gitlab.example.com`, `group/subgroup/project`, `alice`).
- The main `roadmap.md` Phase 24 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Phase                                                    | Tasks       | Status      |
|----------------------------------------------------------|-------------|-------------|
| P1 — GitLab-first: `port.Forge` + config + resolution + native REST/GraphQL + links | T154–T160 | Complete |
| P2 — migrate GitHub + Azure onto `port.Forge`            | T161, T162  | Complete    |
| P3 — publishing via `release.targets` (config unification, not transport) | T163 | Complete |
| P4 (last) — `heraut init` wizard                         | T164        | Superseded  |

Phases run in order. P2–P4 tasks are stubs to be fleshed out when reached; **P1 is the phase to
plan first** (via `superpowers:writing-plans` → subagent-driven execution).

---

## Open items

### T164: `heraut init` generates the forge config (P4, superseded)

Deliberately last in the original phasing: the wizard codifies the config shape, so it lands only
after the new config is **battle-tested in real pipelines** (P1–P3), to avoid churning it against
a moving schema.

Update the scaffold wizard (`internal/scaffold`) to generate `forges:` / `release.targets:` /
`commits.enrichment_forge` / `commits.enrichment_policy`, with auto-detection defaults and an
`api_mode` prompt. Tests: wizard-output fixtures validate against `schema.json`.

**Superseded by `docs/tasks/native-generator-roadmap.md`'s Phase C (T195-T202, 2026-08-19).** The
native-only-generator epic's wizard simplification (dropping the now-decorative generator-choice
step) needed this exact same forge/target work done at the same time, built once against the
final generator-free config shape instead of twice. Stays `[ ]` here as a pointer, not
implemented in this file.

## Archived task detail

Every other task in this epic (P1–P3, and the "Follow-ups" section) is `Done`. Full task
write-ups have been moved to
[`archive/forge-abstraction-roadmap.md`](archive/forge-abstraction-roadmap.md) to keep this file
lean — nothing is lost, just relocated.
