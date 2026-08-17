# Native-Only Generator, Phase C — wizard simplification design

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-08-17
- **Author**: bchatard (with Claude)
- **Related**: `docs/superpowers/specs/2026-08-08-native-only-generator-design.md` §4 (Phase C's
  original high-level scope: drop the generator-choice step, build the forge/target questions
  T164 originally scoped). This document is the detailed brainstorm output that pins down the
  exact wizard flow, field renames, and one new architectural dependency §4 left unresolved.
- **Supersedes**: `docs/tasks/forge-abstraction-roadmap.md` → T164 (`heraut init` generates forge
  config), same as the parent design doc already states. T164 stays `[ ]` there with a pointer
  note.
- **Roadmap**: `docs/tasks/native-generator-roadmap.md`, Phase C section (task breakdown left to
  `writing-plans`).

---

## Problem

The parent design doc's §4 correctly scopes *what* Phase C removes (the decorative
`git-cliff`/`communique`/`None` generator selects) and *what* it adds (forge/target questions),
but was written before this repo's wizard code was re-examined in detail. That re-examination
(2026-08-17) found the actual starting point is different from what §4 implies:

- `internal/scaffold/wizard.go` already has a substantial forge/target flow —
  `runPlatformWizard`/`PlatformAnswer` and the `ConfigToAnswers`/`generate.go` round-trip already
  build `forges:`/`release.targets:` from wizard answers. T164's scope is **not** greenfield; most
  of it already exists.
- What's actually missing is narrower and different in shape: an `api_mode` prompt, wizard-editable
  `commits.enrichment_forge`/`enrichment_policy` (currently passthrough-only), and — because
  `runPlatformWizard` is currently gated on `a.NotesGenerator != ""` (a pre-existing, semantically
  wrong coupling between "generate notes content" and "have somewhere to publish to") — a new
  trigger for the whole platform section once the generator selects are deleted.
- Neither the parent design doc nor T164's original one-paragraph scope note
  (`docs/tasks/forge-abstraction-roadmap.md:408-412`) specifies exact wizard screens, ordering, or
  how the new questions interact with each other.

This document resolves those specifics so `writing-plans` has an unambiguous starting point.

## Decisions

Five questions were brainstormed with the user; each answer below is final for this phase.

### 1. Auto-detection: export from `internal/forge`

The wizard's own git-remote detector (`detectRemoteProject`/`parseRemoteProject`,
`wizard.go:334-364`) is a weaker, independent reimplementation of `internal/forge`'s real
CI/git-origin detection (`detectCIForge`/`parseGitOrigin`, `internal/forge/detect.go`) — the
wizard's version only extracts a repo-path string from any host, with no platform-type or CI-env
detection, and no allowlist of known hosts. Phase C **exports a wrapper from `internal/forge`** so
the wizard can pre-fill platform *type* as well as repo/project — one detection source of truth
instead of two divergent ones.

This is the one decision in this phase that alters an interface other packages could come to
depend on (`internal/forge` gains public API) and requires a **layer-rule change**:
`.claude/rules/coding.md`'s layer table has no `internal/scaffold` row today at all — only
`internal/cmd` has a row, and it already lists `internal/scaffold` as one of its *allowed
imports*, not as an import*er* in its own right. This phase adds a new row for
`internal/scaffold`, permitting what it already imports (`internal/{config,ui,versioning}/`, per
`wizard.go`/`dropped.go`) plus the new `internal/forge` dependency. `internal/scaffold`'s own
`detectRemoteProject`/`parseRemoteProject` are deleted once the shared implementation replaces
them — no two competing detectors survive.

The exact exported shape (a single function returning type + host + project/repo + token-env, vs.
several smaller exports) is left to `writing-plans`/implementation — a mechanical decision, not a
design one, as long as it does not widen `internal/forge`'s already-careful unexported surface
(`resolveAuto`, `resolveExplicit`, `candidateTypes`, `defaultHostFor`, `defaultTokenEnvFor` stay
unexported; only the CI/origin-detection primitives the wizard needs become public).

### 2. Opt-out UX: explicit confirm toggle per block

The two `huh.NewSelect` "Changelog generator" / "Release notes generator" groups
(`wizard.go:246-269`), each offering git-cliff/communique/None, are replaced by two
`huh.NewConfirm()`s: **"Generate a changelog?"** and **"Generate release notes?"** (both default
`true`, matching today's `Defaults()` behavior). A "no" answer produces the same end state as
today's "None" choice — no `changelog:`/`release.notes:` block emitted — but the question is now
honest: no decorative options with zero live effect (per the memory note: today's selects don't
even affect emitted YAML, since `generator:` was removed from config entirely in Phase A).

The existing "Changelog output file" input stays, now hidden via `WithHideFunc` when "Generate a
changelog?" is answered `false`.

### 3. Platform trigger: new independent "Publish releases?" confirm

`runPlatformWizard`'s gate moves from `if a.NotesGenerator != ""` to a new, independent
`huh.NewConfirm()` — **"Publish releases to a platform (GitHub/GitLab)?"** — asked in the post-form
control flow at the same position the `NotesGenerator` gate occupies today (`wizard.go:289-293`):
after the sprint-wizard step, before the per-env branch. Only its *trigger condition* changes, not
its position in the flow. This decouples "generate notes content" from "have somewhere to publish
to," matching how `CLAUDE.md` already frames publishing as its own concern (`heraut release`
requires a resolvable publish destination independent of changelog/notes configuration). A "no"
answer leaves `a.Platforms` empty, same as `heraut init` producing a config with no
`forges:`/`release.targets:` block today when no platform is configured — valid, since heraut's
runtime auto-detects a forge from CI/git-origin at zero-config time
(`docs/specs/02-configuration.md:496-504`).

### 4. `api_mode` prompt: skip when `CI_JOB_TOKEN` is chosen

`api_mode: graphql` structurally cannot work with `CI_JOB_TOKEN` — GitLab's GraphQL API rejects
job tokens outright (`docs/specs/02-configuration.md:527-532`). Rather than let the wizard produce
a combination guaranteed to fail at enrichment time, the new `api_mode` select (rest/graphql,
default `rest`) is added as a new step inside `runPlatformWizard`'s GitLab branch, immediately
after the existing token-env step (`wizard.go:532-567`), and hidden via `WithHideFunc` when
`tokenChoice == "CI_JOB_TOKEN"` — `api_mode` silently stays `"rest"`, the only value that works,
mirroring the wizard's existing pattern of conditionally hiding groups (`isNotCalVer`, etc.)
rather than adding a new validation-error path for a combination that's simpler to just not offer.

### 5. Enrichment prompts: policy whenever content is generated, forge only if ambiguous

`commits.enrichment_forge`/`enrichment_policy` govern PR/MR enrichment of changelog/notes
*content* — conceptually independent of the "Publish releases?" toggle, since enrichment can work
off a zero-config auto-detected forge even with no explicit `forges:` block
(`docs/specs/02-configuration.md:534-550`). Two new prompts, run after the platform loop (or in
its place, when publishing is off):

- **"Enrichment policy"** select (`disabled`/`optional`/`required`, default `optional`) — shown
  whenever "Generate a changelog?" **or** "Generate release notes?" is `true`. Not gated on
  publishing.
- **"Enrichment forge"** select, offering the names of the platforms just configured — shown
  **only when `len(a.Platforms) >= 2`**. With 0 or 1 configured platform, resolution is
  unambiguous (matches `docs/specs/02-configuration.md:506-508`: "disambiguation is only needed
  with multiple forges") and the field stays unset, relying on runtime auto-detection or the
  single configured forge. This replaces `generate.go`'s current default-to-first-forge stopgap
  (lines 124-140, explicitly flagged there as the T164/P4 placeholder) with a real choice exactly
  when the choice is actually ambiguous.

If publishing is off (0 platforms) and content generation is on, "Enrichment policy" is still
asked (governs whether zero-config runtime enrichment happens at all); "Enrichment forge" is
skipped (nothing to choose among).

## Mechanical changes

### `internal/scaffold/wizard.go`

- `mainForm` groups 5–6 (`wizard.go:246-269`) become two confirms per Decision 2.
- Post-form control flow (`wizard.go:289-293`) gains the "Publish releases?" confirm (Decision 3)
  in place of the `NotesGenerator` gate.
- `runPlatformWizard`'s GitLab branch gains the `api_mode` step (Decision 4).
- A new function (name left to implementation) runs the enrichment prompts (Decision 5) after the
  platform loop returns.
- `detectRemoteProject`/`parseRemoteProject` deleted; call sites use the new `internal/forge`
  export (Decision 1).
- `Answers` struct: `ChangelogGenerator string` / `NotesGenerator string` → `EnableChangelog bool`
  / `EnableReleaseNotes bool`. `RemoteMetadata string` → renamed `EnrichmentPolicy string` (its
  current name is a stale holdover from the pre-ADR-0043 `remote_metadata` key and collides
  confusingly with the unrelated `config.ContentDriver`-adjacent `RemoteMetadata` field in
  `internal/config/config.go:104`). `EnrichmentForge` becomes wizard-editable; its "not
  wizard-editable... T164/P4" comment (`wizard.go:44-48`) is deleted, not edited, since this phase
  is that redesign.
- `Defaults()` (`wizard.go:99-110`) sets `EnableChangelog: true, EnableReleaseNotes: true` in
  place of the old generator strings; the `Platforms: []PlatformAnswer{{Type: "gitlab"}}` default
  is unaffected by this phase.
- `ConfigToAnswers` (`wizard.go:113-190`): the sentinel-string workaround at lines 132-137
  (`a.ChangelogGenerator = "git-cliff"` because "cfg.Changelog.Generator is always \"\" post-T177")
  is deleted outright — `a.EnableChangelog = cfg.Changelog != nil` needs no sentinel or comment.
  Same for `NotesGenerator`/`EnableReleaseNotes` at line 147. `EnrichmentPolicy` is populated the
  same way `RemoteMetadata` was (line 119: `cfg.EnrichmentPolicy()`), just renamed.

### `internal/scaffold/generate.go`

- Gates at lines 82/92 switch from `!= ""` string checks on the deleted fields to the new bools.
  No change to emitted YAML shape — no `generator:` key existed to remove; these were always
  presence gates only.
- `EnrichmentForge`/`EnrichmentPolicy` feed `cfg.Commits` the same way `Tickets` already does
  (line 48-49) — extended, not restructured.
- The default-to-first-forge stopgap (lines 124-140) is deleted per Decision 5.

### `internal/scaffold/cliff.go`

Deleted. Confirmed clean: the file is one string-equality helper (`IsCliffGenerator`), called only
by its own test (`cliff_test.go`), not referenced anywhere else in the wizard flow or elsewhere in
the repo.

### `internal/forge`

Gains one new exported wrapper (exact function signature left to `writing-plans`) built on top of
the existing unexported `detectCIForge`/`parseGitOrigin` (`internal/forge/detect.go`). No behavior
change to the existing `Resolve` entry point or its callers (`internal/app/pipeline.go`,
`internal/app/check.go`, `internal/app/platforms.go`) — this is a pure additive export.

### `.claude/rules/coding.md`

Layer-rules table gains a new row: `internal/scaffold` → `internal/{config,ui,versioning,forge}/`
(the first three already reflect current imports in `wizard.go`/`dropped.go`; `forge` is the one
new addition this phase makes).

### Cleanup sweep

Per the roadmap's carried-forward Phase B note, a raw `grep -rn "gitcliff\|git-cliff\|communique"`
sweep (not just the symbol-focused `\.Generator\b|GitCliff|Communique` pattern, which misses
plain-comment prose) runs across `internal/scaffold/` — `wizard.go`, `generate.go`, and their test
files — as part of this phase's own plan, not discovered reactively afterward. The 2026-08-17
survey found the live, in-scope hits are: `wizard.go:29,32,100,105,107,137,147,250,251,264,265`;
`cliff.go:3,5` (deleted with the file); test files `cliff_test.go`, `wizard_test.go`,
`generate_test.go` (multiple lines each, using `"git-cliff"`/`"communique"` as filler generator
values — migrated to the new bool/select-based fixtures as part of updating those tests for the
struct changes above, not a separate pass).

Out of scope for this sweep (confirmed comment-only, no gate, no wizard connection): the
plain-prose hits in `internal/pipeline/linkctx.go:15`, `internal/app/pipeline.go:393`,
`internal/generators/native/{commits,group}.go`, `internal/config/loader.go:36`, `CLAUDE.md:122`,
`schema.json:386`, and `docs/specs/*` comparison-table prose — none of these touch
`internal/scaffold` and none are part of Phase C's brief. `CLAUDE.md:122`'s mise-tooling line and
the Docker/mise pins are Phase D's job (design doc §5), not this phase's.

## Non-goals

- No change to `internal/app`, `internal/pipeline`, or any runtime config-consumption code —
  Phase C only touches what `heraut init` *emits*; Phases A/B already built the runtime that
  consumes `forges:`/`release.targets:`/`commits.enrichment_forge`/`enrichment_policy`.
- No change to `internal/forge`'s existing `Resolve` behavior or its three existing exported
  symbols (`Resolve`, `ErrAmbiguousForge`, `Resolved`) — purely additive.
- No new config schema fields — `api_mode`, `enrichment_forge`, `enrichment_policy` all already
  exist in `schema.json`/`internal/config`; this phase only adds wizard prompts that populate
  fields the config model already supports.
- `docs/tasks/forge-abstraction-roadmap.md`'s T164 is not deleted, only annotated with a pointer
  to this phase — matching the parent design doc's existing precedent for T164.

## Testing plan

- `internal/scaffold`: existing wizard/generate/cliff tests updated for the struct and control-flow
  changes above (TDD: failing test first per this repo's standing rule). New tests for the
  `api_mode`-hidden-on-`CI_JOB_TOKEN` behavior and the enrichment-forge-shown-only-when-2+
  condition, since both are new conditional logic with real failure modes if inverted.
- `internal/forge`: new export gets its own unit test(s), independent of the wizard.
- Wizard-output fixtures continue to validate against `schema.json` (`TestSchema_ValidFixtures` or
  equivalent), per T164's original acceptance criterion.
- `go test ./...` and `hk check` clean at the end of the phase, per this project's standing
  discipline.
