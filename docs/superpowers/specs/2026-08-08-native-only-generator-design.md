# Native-Only Generator — drop `git-cliff` and `communique`

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-08-08
- **Author**: bchatard (with Claude)
- **Related ADRs**: 0028 (drop cocogitto — direct precedent for the cutover mechanics), 0032/0033
  (native content generator, config model), 0010 (embedded cliff TOML default — superseded),
  0016 (bundled Docker image — living inventory, updated not left stale)
- **New ADR required**: yes — ADR-0045, "Native as heraut's sole content generator"
- **Roadmap**: lands as new phases in `docs/tasks/native-generator-roadmap.md` (Phase 2.5 was
  already stubbed there as "remove the git-cliff package (own ADR) — Deferred"; this un-defers it
  and adds sibling phases for communique and the wizard). No new roadmap file.
- **Supersedes**: `docs/tasks/forge-abstraction-roadmap.md` → T164 (`heraut init` generates forge
  config). T164 stays `[ ]` there with a pointer note; the actual work happens as Phase C below,
  built once against the final (forge + native-only) config shape instead of twice.

---

## Problem

heraut ships three changelog/release-notes generators today: `native` (pure Go, no external
binary), `git-cliff` (embedded TOML defaults, shells out to the `git-cliff` binary), and
`communique` (fully opaque, shells out to `communique generate`). `native` has been heraut's
**canonical** renderer since ADR-0032/0033 — git-cliff was explicitly dropped as the *design
anchor* then, with its own package removal "sequenced after native enrichment, in a follow-up
ADR." That follow-up point has arrived: native has full remote enrichment (GitHub/GitLab/Azure,
ADR-0034/0042/0043), user-customizable templates (ADR-0037), incremental changelog (ADR-0038), and
commit-author attribution (ADR-0039) — parity work the native-generator-roadmap's Phase 2.6–2.10
already completed. Keeping two more generators around now costs real, ongoing tax with no
corresponding benefit for this project: every enrichment/policy change (e.g. T174, T175 this
session) has to be reasoned about **per generator**, because git-cliff and native already diverge
on `enrichment_policy: required` semantics, author-fallback rendering, and more. `communique` was
never brought into that parity effort at all — it's a pure passthrough with no heraut-owned
behavior to keep in sync, and isn't used.

## Goals

- **One generator, no dispatch.** `native` is the only generator. No `generator:` config key, no
  `buildGenerator` switch, no per-generator behavioral divergence to reason about or document.
- **Hard cutover, matching this project's own precedent** (ADR-0028, dropping cocogitto): pre-v1.0,
  no deprecation window. A config referencing the old shape gets a clear, actionable error on its
  next `heraut` invocation.
- **Simplify the wizard.** No generator-choice step. `heraut init` asks forge questions instead
  (folding in T164's original scope), built once.
- **Shrink the Docker image and tool matrix.** `git-cliff` and `communique` drop out of the
  bundled image (ADR-0016) and `.config/mise/config.toml`. `git`/`gh`/`glab` stay — those are for
  git operations and publishing, unrelated to content generation.

## Non-goals

- **Feature parity audit.** Out of scope by the user's own call: git-cliff features not already in
  native are accepted losses, and communique (a pure opaque passthrough) isn't used. No gap
  analysis needed before proceeding.
- **Rewriting `gh`/`glab` publishing.** Unrelated — `internal/platforms/{github,gitlab}` are
  publish-side drivers, untouched by this epic (native-generator-roadmap's own "Phase 3 —
  raw-HTTP platform clients" is a separate, still-deferred concern).
- **Rewriting historical ADRs.** ADRs that mention git-cliff/communique in passing (0006, 0010,
  0011, 0012, 0021–0026, 0032–0043...) remain as accurate records of decisions made *at the time*
  — not rewritten, per ADR-0028's own explicit rule for this exact situation. ADR-0016's bundled-CLI
  table is the one standing exception (living inventory).

---

## Design

### 1. Config shape: drop the `generator` key entirely, not just its enum

`ContentDriver.Generator` and `ContentDriver.Config` (the external TOML/config-file path used only
by git-cliff and communique) are **removed from the struct**, not enum-shrunk to `{"native": true}`.
Once there is exactly one possible value, the key carries zero information — keeping it as
boilerplate-only-required-field would be dead weight in every `.heraut.yml` going forward.
`changelog: {}` / `release: {notes: {}}` become valid and meaningful on their own: "generate with
native, using defaults."

A leftover `generator:` or `config:` key becomes a **strict-parse "removed key" error** via the
existing `config.ErrRemovedConfigKey` / `checkRemovedKeys` mechanism (`internal/config/loader.go`)
— the same one T160 built for `changelog.remote` and T163 built for `release.platforms`. No new
mechanism needed; just new table entries:

| Removed key                                    | Hint                                                          |
|-------------------------------------------------|-----------------------------------------------------------------|
| `changelog.generator`                            | native is heraut's only generator now; remove this key         |
| `changelog.config`                               | generator-specific config files are gone; use `rendering.templates` (ADR-0037) |
| `release.notes.generator`                        | same as above                                                   |
| `release.notes.config`                           | same as above                                                   |
| `environments.<env>.changelog.generator`/`.config` | per-env variants of the above, same hints                     |
| `environments.<env>.release.notes.generator`/`.config` | per-env variants                                          |

This mirrors T160/T163's exact shape (top-level table entry + per-env probe message), so
`validateForges`-adjacent code isn't touched — this is purely a `loader.go` addition.

`internal/config/validator.go`'s `validGenerators` map and the `d.Generator == ""` / `!validGenerators[...]`
branches in `validateContentDriver` are deleted (nothing left to validate). The `tag_pattern`
cross-field check (`"tag_pattern requires the git-cliff or native generator"`) simplifies to just
validating the regex unconditionally, since native is the only generator left standing.

### 2. Package + command removal

- `internal/generators/gitcliff/` deleted outright: embedded TOML defaults, Tera merge logic
  (`merge.go`), contract tests, and the real-CLI embedded-config smoke test (`testing.md`'s
  documented exception) all go together.
- `internal/generators/communique/` deleted outright: generator + contract tests.
- `internal/cmd/cliff.go` and `internal/app/cliff.go` deleted. `heraut cliff <mode>` exists solely
  to show the effective merged git-cliff TOML (`EffectiveChangelogConfig`/`EffectiveReleaseNotesConfig`)
  — meaningless with no git-cliff config to merge. Removed from `internal/cmd/root.go`'s command
  wiring.
- `internal/app/pipeline.go`'s `buildGenerator` collapses: no more `switch driver.Generator`,
  it just constructs `native.New(...)` directly. `usesNative(...)` (used throughout
  `internal/app` to gate forge resolution) becomes unconditionally true and can be deleted, with
  its callers simplified — flagged for the implementation plan to confirm blast radius exactly
  (`resolveEnrichForgeIfNeeded`, `buildChangelogPipelineConfig`, `buildReleasePipelineConfig` all
  reference it).
- `internal/app/check.go`: the `git-cliff`/`communique` binary-presence probes and the
  `CheckCliff` (git-cliff config-acceptance check) function are removed; `heraut check runtime`
  probes only `git`, `gh`, `glab` going forward.
- `internal/pipeline/linkctx.go`: git-cliff's separate link-context/env-injection path
  (`linkEnv`, `HERAUT_REMOTE_URL`/`HERAUT_PLATFORM` for the git-cliff subprocess) is deleted —
  only native's `port.LinkContext` consumption path remains.
- `internal/testutil/constants.go`: `GitCliff`/`Communique` binary-name constants removed.
  `internal/testutil/realgit.go`: git-cliff-specific helpers (if any beyond the deleted smoke
  test) removed.
- **`.claude/rules/testing.md`** (project rule, not just docs): the "Real-CLI smoke tests (embedded
  config validation)" section loses its only remaining example (cocogitto's was already dropped by
  ADR-0028) — native has zero external-binary dependency for generation, so this whole exception
  category no longer applies. Section removed, with a one-line note in its place should the pattern
  ever be needed again for a future external dependency.
- `docs/specs/06-dx-and-testing.md`: same real-CLI-smoke-test section updated to match.

### 3. Docs + schema

- `schema.json`: `ContentDriver.generator` (required + enum) and `.config` properties removed.
- `docs/heraut.sample.yml`: both `generator:` occurrences (top-level example + per-env example)
  removed; the commented-out block referencing `generator: git-cliff` cleaned up.
- `docs/specs/02-configuration.md`: `ContentDriver` field table drops `generator`/`config` rows.
- `docs/specs/05-generators-and-platforms.md`: this is the biggest doc change. The `### git-cliff`
  and `### communique` sections are deleted outright (git-cliff's is ~50 lines: invocation shape,
  `[remote.*]` injection, TOML merge behavior; communique's is a short opaque-passthrough
  description). The "per generator" divergence prose T169 wrote this session (self-hosted
  enrichment behavior, `required` policy enforcement) collapses to native-only — most of that
  divergence text simply disappears since there's nothing left to diverge *from*. The generator
  comparison table at the top of the file drops to a single row.
- `docs/tasks/roadmap.md`'s intro line ("Héraut orchestrates `git-cliff`, `glab`, `gh`, `cog`, and
  `communique`...") drops `git-cliff` and `communique`.
- `testdata/config/valid/{enrichment-policy,platform-base-url,semver,tickets,calver,semver-per-env}.yml`
  — the six fixtures using `generator: git-cliff` or `generator: communique` as filler — migrated
  to omit the key entirely (native implicit). `TestSchema_ValidFixtures` continues to validate them
  against the trimmed schema.

### 4. Wizard simplification (Phase C, supersedes T164)

`internal/scaffold/wizard.go`'s "Changelog generator" / "Release notes generator" `huh.NewSelect`
groups (currently offering git-cliff/communique/none — notably **not even offering native today**,
a pre-existing gap) are deleted. In their place: the forge/target questions T164 originally scoped
(`forges:` entry, `release.targets:`, `commits.enrichment_forge`/`enrichment_policy`, an `api_mode`
prompt, auto-detection defaults) — built once, since there's no generator branching left to design
around. `internal/scaffold/cliff.go` (git-cliff-specific scaffold helpers) is deleted.
`internal/scaffold/generate.go`'s YAML emission drops `generator:` lines. `Answers` struct drops
`ChangelogGenerator`/`NotesGenerator` fields.

`docs/tasks/forge-abstraction-roadmap.md`'s T164 gets a note pointing at this phase instead of
being implemented redundantly — not silently dropped, not duplicated.

### 5. Infra housekeeping (Phase D)

- `Dockerfile`: drop the `git-cliff@${GIT_CLIFF_VERSION}` and `communique@${COMMUNIQUE_VERSION}`
  mise-install lines and their `cp` steps. `glab`/`gh` stay (publishing, ADR-0044 — unrelated to
  this epic). The musl/glibc comment referencing communique's dynamic linking is removed along
  with communique itself (worth checking whether git-cliff also drove the glibc-base-image choice,
  or whether `gh`/`glab` alone still require it — implementation-time check, not a design blocker).
- `.config/mise/config.toml`: drop the `git-cliff = "2.13"` tool pin.
- `docs/adr/0016-bundled-docker-image.md`: bundled-CLI table updated (git-cliff/communique rows
  removed) — living inventory, per ADR-0028's own precedent for this exact table.
- `docs/adr/README.md`: new ADR-0045 row added.
- `docs/adr/0010-embedded-cliff-toml-default.md`: this is the one targeted exception to the
  "historical ADRs stay as-is" non-goal above — its entire subject (which embedded TOML defaults
  heraut ships for git-cliff) becomes moot, not just tangentially mentioned. Gets the full
  `Status: Superseded by ADR-0045` treatment plus a `> **Superseded (date):**` blockquote, matching
  the convention this repo already uses for a fully-replaced decision (ADR-0014, ADR-0020) — as
  opposed to the lighter `> **Update (date):**` blockquote used for a partially-refined one
  (ADR-0034, ADR-0043, both left at `Status: Accepted`). Everything else stays untouched.

---

## New ADR-0045 outline

"Native as heraut's sole content generator." Context: native has reached parity (link to
Phase 2.6–2.10 completion), git-cliff/communique now cost more to keep than they return, this
project's own Phase 2.5 already anticipated the git-cliff half. Decision: hard cutover (both
generators), `generator:`/`config:` keys removed entirely (not enum-shrunk — the fresh reasoning
from §1 above, since this ADR is the first time the enum degenerates to one value). Consequences:
mirrors ADR-0028's consequences section shape (config error on next invocation, no deprecation
window, Docker image + tool matrix shrink, `heraut cliff` command removed).

## Roadmap placement

`docs/tasks/native-generator-roadmap.md` gains:

- **Phase 2.5 — Remove the git-cliff package** (currently a stub row marked "Deferred" in the
  progress table, no section body written yet) — fleshed out fully, status flips to Active/tasks
  assigned.
- **New phase — Remove the communique package** (wasn't anticipated in this file at all; added
  fresh).
- **New phase — Wizard simplification** (Phase C above, superseding
  `forge-abstraction-roadmap.md`'s T164).

Task IDs continue the global sequence from **T177** (T176 was the last one used, in
`forge-abstraction-roadmap.md`). Exact task boundaries (how many tasks per phase, TDD red/green
splits) are an implementation-plan concern, not a design-doc concern — left to `writing-plans`.

## Implementation sequencing

Each phase (A/B/C/D) gets its **own** implementation plan and execution pass, run in order —
mirroring how the forge-abstraction epic split P1–P4 into separate planned phases rather than one
giant plan. Phase A (config cutover) is small and self-contained: plan and land it first. Phase B
(package/command deletion) depends on Phase A's removed-key errors already being in place, so
existing users hit a clear config error before the underlying packages disappear from under them
mid-transition (both land in the same overall epic regardless, but Phase A first keeps every
intermediate commit's behavior coherent). Phase C (wizard) depends on Phase A/B being done, since
it's built against the final, generator-free config shape. Phase D (infra) can happen any time
after Phase B, or alongside it.

## Testing plan

- Every deleted package takes its tests with it — no orphaned test files.
- `internal/config`: new removed-key migration tests (mirroring
  `TestLoad_RemovedKey_ReleasePlatforms`'s shape) for `changelog.generator`/`.config` and the
  release-notes/per-env variants.
- `internal/app/pipeline_test.go` and friends: existing tests referencing `generator: git-cliff`/
  `generator: communique` as filler config are migrated to omit the key (matching the testdata
  fixture migration in §3).
- `TestSchema_ValidFixtures` continues to pass against the trimmed schema — proves the six migrated
  fixtures round-trip.
- `go test ./...` and `hk check` clean at the end of every phase, per this project's standing
  discipline — no phase lands with a broken build even temporarily.
