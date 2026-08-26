# Release Config Simplification — collapse notes+targets, `{}` as the defaults idiom

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-08-26
- **Author**: bchatard (with Claude)
- **Related ADRs**: 0043 (forge abstraction), 0044 (publishing config unification), 0045 (native
  sole generator — established the precedent this design extends: once a block has exactly one
  sensible default behavior, `{}` should mean "use it")
- **New ADR required**: yes — ADR-0046, working title "Release block is one intent, not two"
- **Roadmap**: own dedicated file — `docs/tasks/release-config-roadmap.md`, T216+, with a pointer
  from the main `docs/tasks/roadmap.md` (new Phase 25), matching the native-generator and
  forge-abstraction epics' pattern.
- **Timing**: lands before the v1.0.0 cut — user-confirmed.
- **Supersedes**: `docs/tasks/forge-abstraction-roadmap.md` → **T214**, shipped this session
  (commit `3452198`). T214's mechanism — skip implicit target synthesis when `release.notes` is
  configured — is the *opposite* of what this design does (synthesis should no longer depend on
  whether `notes` is set at all). T214 stays `[x]` there as a real, correct fix for the model that
  existed at the time; this design doc's implementation removes the specific gating it added — see
  Problem for why that's not undoing T214's value.

---

## Problem

Two things surfaced in the same investigation this session (originally: a user's `heraut check`
failure on a self-hosted GitLab CI config), and together they point at the same root cause.

**1. The minimal config is more boilerplate than it needs to be.** Since ADR-0045 removed the
`generator:` key (native is the only generator, so the key carried zero information), `changelog:
{}` and `release: { notes: {} }` are already meant to read as "use this feature, with defaults."
But `release:` needs two sub-decisions today — `notes` (generate text?) and `targets` (publish
where?) — each independently optional, so a user has to reason about four combinations to write
even a common-case config.

**2. Those four combinations don't behave the way they're documented, and the gap has real
consequences.** `docs/specs/02-configuration.md` documents `release.notes` set with no
`release.targets` as "notes are generated but no release is published... useful for previewing or
piping output to another tool." Tracing `internal/pipeline/release.go`'s `Run()` end to end: the
generated notes string is stored in a **local variable** whose only consumer is the loop over
`p.cfg.Platforms`. When that loop has zero iterations (no publish target), the string is discarded
when `Run()` returns — never printed, written to a file, or returned to the caller. `heraut
changelog` doesn't touch `release.notes` at all. **No command surfaces it.** The spec's own
justification for this state doesn't correspond to anything the code does, and — checked directly
— isn't a use case anyone building against heraut has actually needed either. The spec's mirror
case, "targets only, no notes" (publish with an empty body, CHANGELOG.md is the record), has the
same status: real code today, but no confirmed need behind it. Both get treated the same way below:
dropped, not preserved.

T214 (this session) made the "notes only" state *safe* — it stopped an empty `release.targets` from
silently publishing when the user's intent (signaled by setting `release.notes`) was "notes only."
That was the correct fix for the model as documented at the time. But it fixed the wrong layer: the
real problem isn't that "notes-only" published when it shouldn't have — it's that "notes with
nowhere to go" was never a state worth preserving as a *distinct, reachable* config shape in the
first place, since nothing consumes it. This design removes that state's precondition entirely
(there's no longer an independently-toggleable "notes only" or "publish only" axis to protect
against), which is why T214's specific gating mechanism gets removed rather than reused — the bug
it fixed can no longer occur.

The user who hit this originally expected `release:` to mean one thing — "create a release [on the
declared forge] with generated notes" — matching how it actually behaved pre-refactor (v0.51.1: a
single `platforms:` list, notes always included). Splitting "generate" and "deliver" into two
independently-optional axes is what created the ambiguity: omitting `release.targets` means
different things depending on whether `release.notes` happens to be set, which is not an obvious
signal to read, and is exactly what tripped the user up this session (twice, in opposite
directions — once via the pre-T214 accidental-publish bug, once via what would have been T214's
silent-no-op if they hadn't caught it).

## Goals

- **One intent, one block, fully atomic — root and per-environment alike.** `release:` presence
  (however minimal) always means "generate notes and publish them," together, unconditionally.
  There is no config-expressible way to get one without the other, anywhere. If a real need for
  splitting them surfaces later, that's new scope built against a real use case, not a corner of
  the current shape kept alive on spec.
- **`{}` is the standard "enable with defaults" idiom**, consistently, for every top-level feature
  block. `changelog: {}` already works this way; `release: {}` should too — it doesn't today
  (`Release.Notes` stays `nil` even when `Release` itself is present).
- **Omission still means off, always.** No block defaults to "on" when absent. Unchanged from the
  current model and from the pushback earlier in this same conversation — only *presence* semantics
  are being simplified, not the on/off default itself. This applies per-environment too: an
  environment can turn the whole `release:` behavior off, but not split it.
- **Don't foreclose a future non-forge delivery target.** The user raised pushing release notes to
  a notification channel (Slack/Teams/email) as a plausible future feature — a *different kind* of
  target, not a forge Release. The config shape should be able to grow a target `type`/`kind`
  discriminator later without another breaking change, even though building it is out of scope here.

## Non-goals

- **Building the notification-channel target type.** Explicitly future work. This design only
  needs to confirm the shape doesn't paint us into a corner — see Design §5.
- **Preserving "notes only" or "targets only, no notes."** Both checked directly against the
  people actually using heraut this session; neither has a confirmed use case. Dropped, not carried
  forward under a new name.
- **Changing `changelog:`'s behavior.** Already correct (`{}` → defaults; omitted → disabled). Not
  touched by this design.
- **Revisiting T215** (self-hosted CI autologin narrowing). Unrelated, already shipped, stays as-is.
- **Making any block default to enabled when fully omitted.** Considered and rejected earlier this
  session specifically for `release:`, given publishing is the highest-consequence action in the
  tool. This design doesn't reopen that — it only changes what *presence* (even `{}`) means.
- **Solving the null-vs-absent YAML distinction.** A bare `key:` and `key: ~` are the same YAML
  value and both already mean "absent" today; this design doesn't try to make them mean something
  else. `{}` (an explicit empty map) is the one form that's actually distinct, and is the one this
  design standardizes on.

---

## Design

### 1. `release.notes` stops being an on/off toggle; it becomes a customization sub-block

`ContentDriver` (the type behind `release.notes`) has real per-field value beyond "generate or
not" — `Template`, `TypesHeadingLevel`, `HeadingVersionPattern`, `Tickets`, `Rendering.Excludes`,
etc. all make sense as release-notes-specific overrides independent of whether notes generate at
all. Keep the sub-block for that purpose. Stop treating `Release.Notes == nil` as "notes
disabled" — instead:

- `cfg.Release == nil` (the whole block absent) → no release intent at all: no notes generated, no
  publish attempted. Unchanged from today.
- `cfg.Release != nil` (block present, however minimal — `release: {}`) → notes generate
  unconditionally, with any set `release.notes` fields as rendering overrides, and publish is
  attempted against the resolved target(s). No field anywhere suppresses one half while keeping the
  other.

### 2. Loader default-population for `Release.Notes`

Mirrors the existing pattern: `ContentDriver.Output` already defaults to `CHANGELOG.md` post-decode
for the changelog driver (confirmed: `changelog: {}` today produces a non-nil `ContentDriver` with
`Output: "CHANGELOG.md"` populated). Add the equivalent for `release.notes`: whenever `cfg.Release
!= nil` and `cfg.Release.Notes == nil`, the loader populates `Release.Notes = &ContentDriver{}`
(zero-value — every field defaults downstream exactly as an explicitly-written `notes: {}` already
does). This is the one piece of new *behavior*; everything else in this section either already
works or is a removal.

### 3. Target synthesis reverts to "does a forge resolve" — T214's `notesConfigured` gate is removed

`synthesizeDefaultTarget` (`internal/app/platforms.go`, added by T214) currently returns `nil` (no
synthesis) when `notesConfigured` is true, even if a forge resolves. Under §1/§2, `notesConfigured`
becomes true unconditionally whenever `release:` is present — so left as-is, T214's own logic would
now suppress synthesis in the *default* case, which is exactly backwards. Remove the
`notesConfigured` parameter and the branch entirely; synthesis reverts to checking only
`len(resolved.Forges) == 0` — the pre-T214 behavior, now correct again because there's no
independently-reachable "notes only" state left to protect against accidentally publishing into.

### 4. Per-environment: rename `disable_notes` → `disable_release`

Per-environment overrides keep the ability to turn `release:` **off entirely** for one environment
— a real, already-established pattern (`disable_changelog` on UAT environments, generating the
changelog only on the production release, is documented today) — but, matching §1's atomicity, can
no longer disable just the notes half while still publishing. `Environment.DisableNotes`
(`environments.<env>.disable_notes`) is renamed to `Environment.DisableRelease`
(`environments.<env>.disable_release`): it now turns off the entire `release:` behavior (notes and
publish together) for that environment, not just notes rendering.

This must be a **hard removed-key error**, not a silent reinterpretation of the old name — someone
with `disable_notes: true` today relies on it to mean "keep publishing, skip the notes text." If the
same key silently started meaning "stop publishing entirely" on the next heraut version, that's a
release-automation tool silently going quiet on an environment that used to publish, with no error
to catch it. `internal/config/loader.go`'s existing `checkRemovedKeys` mechanism (used for
`changelog.remote`, `release.platforms`, and the ADR-0045 `generator:`/`config:` removals) covers
exactly this shape: a clear, actionable error naming `disable_release` as the replacement, on the
next `heraut` invocation, no deprecation window — matching this project's own precedent (ADR-0028)
for a pre-v1.0 breaking config change. `disable_changelog` is untouched — a different, unrelated
block, never in question.

### 5. `release.targets[]` shape check: does this preclude a future non-forge target?

Today: `{forge, draft, prerelease, assets}` — `forge` is a bare name reference, `draft`/`prerelease`
are GitHub-only, `assets` is generic. Nothing here assumes every target is a forge Release *by
construction* — `forge:` is just a field name, not a type discriminator. A future
`{type: slack, webhook_url: ...}` entry alongside `{forge: gh}` entries is representable by adding a
`type` field (defaulting to `"forge"` for backward compatibility) without restructuring the existing
shape. No change needed now — confirmed the door is open, not opened.

### 6. Docs

- `docs/specs/02-configuration.md`'s `## release` section: the current four-shape enumeration
  (`targets` only / `notes` only / neither / release omitted) gets replaced with the two-state model
  — `release:` absent → nothing; `release:` present → notes + publish, unconditionally — plus the
  `release.targets[]` / `forges` reference sections updated to match, and the per-environment
  section updated for the `disable_notes` → `disable_release` rename.
- `docs/heraut.sample.yml`: minimal example simplifies to match the shape discussed this session
  (`changelog: {}`, `forges: [...]`, `release: {}`); any `disable_notes` example updated to
  `disable_release`.
- `schema.json`: rename the per-env `disable_notes` property to `disable_release`; no structural
  change to `release`/`forges`.

---

## New ADR-0046 outline

"Release block is one intent, not two." Context: §Problem above — `release.notes` and
`release.targets` as independently-optional axes created an ambiguous config shape whose "notes
only" corner produced no observable output, confirmed by tracing `Run()`; the mirror "targets only,
no notes" shape has the same status — real code, no confirmed use. Decision: `release:` presence
means "generate and deliver," fully atomic, root and per-environment; the per-environment
`disable_notes` escape hatch is renamed `disable_release` (turns off the whole block, not half of
it) via a hard removed-key error, no deprecation window, matching ADR-0028's precedent.
Consequences: existing configs setting `release.notes` without `release.targets` (T214's "notes
only" shape, shipped but never released to a version tag) start publishing; existing configs
setting `release.targets` without `release.notes` now also get notes generated unconditionally,
with no opt-out short of disabling the whole environment's release behavior; existing
`disable_notes` configs hard-fail with an actionable rename hint rather than silently changing
meaning.

## Roadmap placement

New dedicated file, `docs/tasks/release-config-roadmap.md`, starting at **T216** (continuing the
global sequence from T215), with a Phase 25 pointer added to the main `docs/tasks/roadmap.md` —
user-confirmed, matching how native-generator and forge-abstraction each got their own file. Rough
task shape (exact boundaries and TDD red/green splits are a Plan-phase concern, not this doc's):

1. `Release.Notes` default-population in the loader (§2).
2. Remove T214's `notesConfigured` gate from `synthesizeDefaultTarget` (§3) — this is a revert of a
   very recent, well-tested change; the removal itself should be small, but needs its own tests
   proving the *new* two-state model (not just deleting T214's tests).
3. Rename per-env `disable_notes` → `disable_release`, with a removed-key migration error for the
   old name (§4).
4. Docs: spec 02, sample config, schema (§6).
5. ADR-0046.
6. `heraut init` wizard: collapse the independent "generate release notes?" / "publish releases?"
   questions into one (`internal/scaffold/wizard.go` currently asks both separately, and
   `generate.go` wires `hasNotes`/`hasPlatforms` independently — exactly the split this design
   removes), and rename the wizard's per-env `DisableNotes` field to match §4.

## Implementation sequencing

Single pass, not phased — the pieces are small and interdependent. §2's default-population and §3's
gate removal must land together (an intermediate state populating `Notes` by default while T214's
gate still keys off it would suppress synthesis unconditionally — worse than either end state). §4's
rename is independent of §2/§3 but small enough not to warrant its own phase. One plan, one task
breakdown, TDD throughout as usual.

## Testing plan

- `internal/config`: new loader tests for `Release.Notes` default-population (`release: {}` →
  non-nil `Notes`); a removed-key migration test for `environments.<env>.disable_notes` (mirroring
  `TestLoad_RemovedKey_ReleasePlatforms`'s shape) asserting the error names `disable_release`; a
  parse/effective-value test for the renamed `disable_release` field, mirroring the existing
  `disable_changelog` per-env test shape.
- `internal/app`: `synthesizeDefaultTarget`'s T214-era tests
  (`TestBuildReleasePipelineConfig_TargetsWiring`'s "release.notes set, no release.targets" subtest,
  `TestRuntimeCheck_NotesOnlyNoTargets_SkipsPublishCheck`) get **replaced**, not just deleted — new
  assertions prove the reverted, fully-atomic behavior: a forge resolves + `release: {}` → publishes
  with generated notes, no configuration able to split the two.
- `docs/specs/02-configuration.md`'s updated examples should stay synced with `testdata/config/valid/`
  fixtures the way existing spec examples do; any fixture using `disable_notes` migrates to
  `disable_release`.
- `go test ./...` and `hk check` clean throughout, per this project's standing discipline.

## Resolved questions

1. **Roadmap placement**: own dedicated file (`docs/tasks/release-config-roadmap.md`) — user-confirmed.
2. **Migration note for the "targets without notes now generates notes anyway" behavior change**:
   none beyond the `disable_release` removed-key error — user-confirmed acceptable to break
   pre-v1.0, no additional soft-warning mechanism needed.
3. **Timing**: lands before the v1.0.0 cut — user-confirmed.
