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
| T217 | Rename per-env `disable_notes` → `disable_release` (removed-key migration)   | Not started |
| T218 | Docs: spec 02, sample config, schema                                          | Not started |
| T219 | ADR-0046 — "Release block is one intent, not two"                            | Not started |
| T220 | `heraut init` wizard: collapse the notes/publish questions, rename per-env `DisableNotes` | Not started |

Single pass, not phased — see the design doc's "Implementation sequencing." T216 and T217 touch
disjoint code (loader defaulting + target synthesis vs. a config-key rename) and can be built in
either order or in parallel; T218/T219/T220 document or reflect the result and land last, so they
describe real behavior rather than intent.

---

#### `[x]` T216: Release atomicity — default-populate `Release.Notes`, remove T214's synthesis gate

Two changes that must land together (an intermediate state with one but not the other is
incoherent — see design doc "Implementation sequencing"):

1. **Loader default-population.** Whenever `cfg.Release != nil` and `cfg.Release.Notes == nil`, the
   loader (`internal/config/loader.go`) populates `Release.Notes = &ContentDriver{}` — mirroring how
   `ContentDriver.Output` already defaults to `CHANGELOG.md` for the changelog driver. After this,
   `release: {}` produces a non-nil `Notes` exactly as an explicitly-written `notes: {}` already
   does.
2. **Remove the `notesConfigured` gate.** `synthesizeDefaultTarget`
   (`internal/app/platforms.go`, added by T214) drops its `notesConfigured bool` parameter and the
   branch that returns `nil` when it's true. Synthesis reverts to checking only
   `len(resolved.Forges) == 0` — the pre-T214 behavior. Both call sites
   (`buildTargetPlatforms` in `internal/app/pipeline.go`, `effectiveTargetPlatforms` in
   `internal/app/check.go`) drop the `notesConfigured`/`effectiveNotes != nil` argument they
   currently pass.

T214's own tests (`TestBuildReleasePipelineConfig_TargetsWiring`'s "release.notes set, no
release.targets" subtest in `internal/app/targets_internal_test.go`,
`TestRuntimeCheck_NotesOnlyNoTargets_SkipsPublishCheck` in `internal/app/check_test.go`) get
**replaced**, not just deleted — new assertions prove the reverted, fully-atomic behavior: a forge
resolves + `release: {}` → publishes with generated notes, with no configuration able to split the
two anymore.

**Files:** `internal/config/loader.go` (+ tests), `internal/app/platforms.go`,
`internal/app/pipeline.go`, `internal/app/check.go`, `internal/app/targets_internal_test.go`,
`internal/app/check_test.go`. **Scope:** M. **Dependencies:** none.

Implemented as designed, both halves in one commit. `normalize()` in `internal/config/loader.go`
now default-populates `Release.Notes` right after the existing `Changelog.Output` default, guarded
identically (`cfg.Release != nil && cfg.Release.Notes == nil`). `synthesizeDefaultTarget` dropped
the `notesConfigured bool` parameter entirely and reverted to `len(resolved.Forges) == 0`; both
call sites (`buildTargetPlatforms`, `effectiveTargetPlatforms`) dropped the argument they passed.
`check.go`'s `effectiveTargetPlatforms` also dropped its now-unused `config.EffectiveReleaseNotes`
call. Left `config.EffectiveReleaseNotes` itself in place (`internal/config/platforms.go`) even
though its only production call site is now gone — it wasn't in this task's file list, it's still
exported/tested, and deleting it would be a scope call belonging to whoever next audits for dead
exports, not this task. The two T214 tests were replaced with new assertions proving the reverted,
fully-atomic behavior (a forge resolves + `release.notes` set with no `release.targets` → publishes
regardless), matching the design doc's testing plan exactly. `go test ./...` and `hk check` both
clean.

---

#### `[ ]` T217: rename per-environment `disable_notes` → `disable_release`

`Environment.DisableNotes` (`environments.<env>.disable_notes`) is renamed to
`Environment.DisableRelease` (`environments.<env>.disable_release`) in
`internal/config/config.go`. Its meaning changes to match T216's atomicity: it now turns off the
entire `release:` behavior (notes generation *and* publishing) for that environment, not just notes
rendering while still publishing.

This is a **hard removed-key error**, not a silent reinterpretation of the old field name — a
config with `disable_notes: true` today relies on it to mean "keep publishing, skip the notes
text." If the same key silently started meaning "stop publishing entirely" on the next heraut
version, that's a release-automation tool going silently quiet on an environment that used to
publish, with nothing to catch it. Wire `environments.<env>.disable_notes` into the existing
`checkRemovedKeys` mechanism (`internal/config/loader.go`, the same one used for
`changelog.remote`, `release.platforms`, and the ADR-0045 `generator:`/`config:` removals) — a
clear, actionable error naming `disable_release` as the replacement, on the next `heraut`
invocation. `disable_changelog` is untouched — a different, unrelated toggle, never in question.

`pCfg.DisableNotes`/`buildReleasePipelineConfig`'s reference to `envCfg.DisableNotes`
(`internal/app/pipeline.go`) updates to the renamed field.

**Files:** `internal/config/config.go`, `internal/config/loader.go` (+ removed-key migration test,
mirroring `TestLoad_RemovedKey_ReleasePlatforms`'s shape), `internal/app/pipeline.go`.
**Scope:** S. **Dependencies:** none.

---

#### `[ ]` T220: `heraut init` wizard — collapse the notes/publish questions, rename per-env `DisableNotes`

`internal/scaffold/wizard.go`'s main form asks three independent `huh.NewConfirm()` questions today:
"Generate a changelog?" (`EnableChangelog`), "Generate release notes?" (`EnableReleaseNotes`), and
(in a later form) "Publish releases to a platform (GitHub/GitLab)?" (`PublishReleases`). The last two
are exactly the split T216 removes — `generate.go`'s `hasNotes := a.EnableReleaseNotes` /
`hasPlatforms := len(a.Platforms) > 0` are independently checked, and a user can currently answer
"yes" to notes and "no" to publish (or vice versa), producing a config shape that's no longer
meaningful once `release:` is atomic.

Collapse `EnableReleaseNotes` and `PublishReleases` into one question — something like "Create a
release (generate notes and publish) on your forge?" — driving both the platform-selection sub-flow
(`runPlatformWizard`, unchanged) and whether `cfg.Release`/`cfg.Release.Notes` get emitted at all in
`generate.go`. `Answers.EnableReleaseNotes` is removed as a separate field (or repurposed — decide at
implementation time whether one bool suffices or the field is dropped in favor of `PublishReleases`
alone, now that they mean the same thing). `generate.go`'s `hasNotes || hasPlatforms || hasAssets`
condition simplifies accordingly — likely `hasPlatforms || hasAssets`, since "notes" is no longer an
independent trigger for emitting `cfg.Release`.

`EnvAnswer.DisableNotes` (the per-environment wizard field, wired straight through to
`config.Environment.DisableNotes` in `generate.go`) renames to `DisableRelease`, matching T217.
Whatever per-env prompt currently sets it (if any — confirm at implementation time whether this is
question-driven or only reachable via the edit-existing-config round trip) updates its label to
match the new meaning ("turn off the whole release for this environment," not "skip notes text").

`runEnrichmentWizard`'s gate (`if !a.EnableChangelog && !a.EnableReleaseNotes`) updates to whatever
the collapsed field ends up being named.

**Files:** `internal/scaffold/wizard.go`, `internal/scaffold/generate.go` (+
`internal/scaffold/wizard_internal_test.go`, `internal/scaffold/generate_test.go`).
**Scope:** M. **Dependencies:** T216, T217 (the wizard must emit the final, post-atomicity config
shape, not the old one).

---

#### `[ ]` T218: docs — spec 02, sample config, schema

- `docs/specs/02-configuration.md`'s `## release` section: the current four-shape enumeration
  (`targets` only / `notes` only / neither / release omitted) is replaced with the two-state model
  — `release:` absent → nothing; `release:` present → notes + publish, unconditionally — plus the
  `release.targets[]` / `forges` reference sections updated to match, and the per-environment
  section updated for the `disable_notes` → `disable_release` rename.
- `docs/heraut.sample.yml`: minimal example simplifies to `changelog: {}` / `forges: [...]` /
  `release: {}`; any `disable_notes` example updated to `disable_release`.
- `schema.json`: rename the per-env `disable_notes` property to `disable_release`; no structural
  change to `release`/`forges`.
- `testdata/config/valid/*.yml`: any fixture using `disable_notes` migrates to `disable_release`.

**Files:** `docs/specs/02-configuration.md`, `docs/heraut.sample.yml`, `schema.json`,
`testdata/config/valid/*.yml`. **Scope:** S. **Dependencies:** T216, T217 (documents their actual
behavior, not intent ahead of it).

---

#### `[ ]` T219: ADR-0046 — "Release block is one intent, not two"

Context: `release.notes` and `release.targets` as independently-optional axes created an ambiguous
config shape whose "notes only" corner produced no observable output (traced via `Run()`); the
mirror "targets only, no notes" shape had the same status — real code, no confirmed use, per direct
confirmation this session. Decision: `release:` presence means "generate and deliver," fully atomic,
root and per-environment; the per-environment `disable_notes` escape hatch is renamed
`disable_release` via a hard removed-key error, no deprecation window, matching ADR-0028's
precedent. Consequences: existing configs setting `release.notes` without `release.targets` (T214's
"notes only" shape, shipped but never released to a version tag) start publishing; existing configs
setting `release.targets` without `release.notes` now also get notes generated unconditionally, with
no opt-out short of disabling the whole environment's release behavior; existing `disable_notes`
configs hard-fail with an actionable rename hint rather than silently changing meaning.

**Files:** `docs/adr/0046-release-block-atomicity.md` (or final chosen filename), `docs/adr/README.md`.
**Scope:** S. **Dependencies:** T216, T217 (records the decision as implemented).
