# ADR-0046: Release block is one intent, not two

- **Status**: Accepted
- **Date**: 2026-08-26
- **Deciders**: bchatard
- **Extends**: [ADR-0043](0043-forge-abstraction.md) (forge abstraction),
  [ADR-0044](0044-publishing-config-unification.md) (publishing config unification),
  [ADR-0045](0045-native-sole-generator.md) (native sole generator — established the precedent this
  ADR extends: once a block has exactly one sensible default behavior, `{}` should mean "use it")
- **Supersedes**: [`docs/tasks/forge-abstraction-roadmap.md` T214](../tasks/forge-abstraction-roadmap.md)
  (shipped 2026-08-25, commit `3452198`) — see Context below for why this is a removal of T214's
  precondition, not a reversal of its correctness at the time

---

## Context

`release:` had two independently-optional sub-fields: `notes` (generate release-notes text?) and
`targets` (publish to a forge?). That produced four reachable config shapes — both set, `targets`
only, `notes` only, or `release:` omitted entirely — and the "notes only" shape did not do what
`docs/specs/02-configuration.md` documented. The spec said it meant "notes are generated but no
release is published... useful for previewing or piping output to another tool." Tracing
`internal/pipeline/release.go`'s `Run()` end to end: the generated notes string is assigned to a
local variable whose only consumer is the loop over `p.cfg.Platforms`. With zero platforms, that
loop never runs, and the string is discarded when `Run()` returns — never printed, written to a
file, or handed back to the caller. No command surfaces it: `heraut changelog` never touches
`release.notes` at all. Checked directly against how heraut is actually being used this session:
neither this "notes only" shape nor its mirror ("targets only, no notes" — a real, working shape,
just with no confirmed use behind it either) corresponds to a need anyone has.

T214, shipped the session before this one, made the "notes only" shape *safe*: it stopped an empty
`release.targets` from silently publishing when `release.notes` was set, by adding a
`notesConfigured` gate to `synthesizeDefaultTarget`. That was the correct fix for the config model
as it existed at the time — a real accidental-publish bug, fixed at the layer where it manifested.
But it fixed the shape's *symptom*, not its cause: the actual problem was never that "notes only"
published when it shouldn't have. It was that "notes with nowhere to go" was never a state worth
preserving as a distinct, reachable config shape in the first place, since nothing downstream
consumes it. This ADR removes that state's precondition — once `release:` presence always means
"generate and publish, together," there is no more independently-reachable "notes only" corner for
a gate to protect against — which is why T214's specific mechanism is removed rather than reused,
not because T214 was wrong when it shipped.

The user who hit this originally expected `release:` to mean one thing — "create a release on the
declared forge with generated notes" — matching how heraut actually behaved before the forge/
publishing-config refactor (v0.51.1: a single `platforms:` list, notes always included). Splitting
"generate" and "deliver" into two independently-optional axes is what introduced the ambiguity:
omitting `release.targets` means different things depending on whether `release.notes` happens to
be set, which is not an obvious signal to read from a config file, and is exactly what tripped this
session up twice, in opposite directions — once via the pre-T214 accidental-publish bug, once via
what would have been T214's own silent no-op had it not been caught.

## Decision

`release:` presence, however minimal (`release: {}`), always means "generate release notes and
publish them" — together, unconditionally, root and per-environment alike. There is no
config-expressible way to get one without the other anywhere in the schema.

- **Loader default-population.** `cfg.Release != nil && cfg.Release.Notes == nil` now populates
  `Release.Notes = &ContentDriver{}` in `internal/config/loader.go`'s `normalize()`, mirroring how
  `ContentDriver.Output` already defaults to `CHANGELOG.md`. `release: {}` produces a non-nil
  `Notes` exactly as an explicitly-written `notes: {}` already did — `release.notes` stops being an
  on/off toggle and becomes a rendering-customization sub-block only (`template`,
  `HeadingVersionPattern`, `Tickets`, `Rendering.Excludes`, etc. still apply; nothing under it can
  suppress notes generation or publishing).
- **Target synthesis reverts to pre-T214 behavior.** `synthesizeDefaultTarget`
  (`internal/app/platforms.go`) drops the `notesConfigured bool` parameter T214 added and the
  branch that returned `nil` when it was true. Synthesis is once again just
  `len(resolved.Forges) == 0` — there is no longer a "notes only" state to protect against.
- **Per-environment `disable_notes` → `disable_release`.** The escape hatch to turn off release
  behavior for one environment (paralleling `disable_changelog`) is renamed and its meaning
  broadens: it now disables the whole `release:` block for that environment — notes and publish
  together — not just the notes text while still publishing. The old key is a **hard removed-key
  error** via the existing `checkRemovedKeys` mechanism (`internal/config/loader.go`), not a silent
  reinterpretation: a config relying on `disable_notes: true` to mean "keep publishing, skip the
  notes text" would otherwise start publishing nothing on its next `heraut` invocation, with no
  error to catch it. `disable_changelog` is untouched.
- **`release.targets[]`'s shape does not foreclose a future non-forge target.** `forge:` on a
  target is a bare name reference, not a type discriminator by construction — nothing assumes
  every target is a forge Release. A future `{type: slack, webhook_url: ...}` entry alongside
  `{forge: gh}` entries is representable later by adding a `type` field (defaulting to `"forge"`
  for compatibility) without another breaking change. Building that is explicitly out of scope
  here; this decision only confirms the door stays open.

### What does not change

`changelog:`'s behavior is untouched — `{}` already meant "use defaults," omitted already meant
"disabled," and this decision does not touch it. `disable_changelog` keeps its existing meaning and
precedence rule. No block is made to default to *enabled* when fully omitted — omission still means
off everywhere; only what *presence* (even `{}`) means is simplified. T215 (self-hosted CI autologin
narrowing) is unrelated and unaffected.

## Consequences

- **Breaking config change, acceptable pre-v1.0** — same precedent as
  [ADR-0028](0028-drop-cocogitto-generator.md) (cocogitto removal), T163, and ADR-0045 (native sole
  generator): a hard cutover lands directly on `main`, no deprecation window, no branch protection
  to route around.
- **Existing configs setting `release.notes` without `release.targets`** (T214's "notes only"
  shape — shipped but never released to a version tag) start publishing to the resolved forge on
  their next `heraut release`, since target synthesis no longer treats "notes configured" as a
  reason to withhold it.
- **Existing configs setting `release.targets` without `release.notes`** now also get notes
  generated unconditionally on publish, with no config-level opt-out short of
  `disable_release: true` for the whole environment.
- **Existing `disable_notes` configs hard-fail** on their next `heraut` invocation with an
  actionable error naming `disable_release` as the replacement, rather than silently changing what
  the config does.
- **`heraut init`'s wizard** still asks its pre-atomicity notes/publish questions independently
  until a later, separately-tracked task (T220) collapses them — tracked, not silently left stale.

## Alternatives considered

- **Keep the two independently-optional axes, and just improve the spec's accuracy about what
  "notes only" does.** Rejected: the shape traces to no observable output regardless of how well
  it's documented — fixing the docs without fixing the code leaves the same dead corner in place,
  just described more precisely.
- **Reuse T214's `notesConfigured` gate, inverted, as the new atomicity guard.** Rejected: once
  `release:` presence always populates `Notes`, `notesConfigured` becomes true unconditionally in
  the default case — keeping the gate (even inverted) would suppress zero-config publishing exactly
  when it should fire. The gate's precondition is gone, not just its polarity.
- **Make `release:` default to "on" when fully omitted, matching the old pre-refactor
  `platforms:` shape more closely.** Rejected, revisiting a decision already made earlier the same
  session: publishing is the highest-consequence action heraut takes, and an implicit config
  default should never be the thing that decides whether something ships. Omission still means off.

## References

- Design spec: [`docs/superpowers/specs/2026-08-26-release-config-simplification-design.md`](../superpowers/specs/2026-08-26-release-config-simplification-design.md)
- Roadmap: [`docs/tasks/release-config-roadmap.md`](../tasks/release-config-roadmap.md) (T216–T220)
- [ADR-0043](0043-forge-abstraction.md) — forge identity resolution this decision's zero-config
  synthesis depends on
- [ADR-0044](0044-publishing-config-unification.md) — `release.targets[]` as the sole publishing
  surface, unchanged by this decision
- [ADR-0045](0045-native-sole-generator.md) — established the `{}`-means-defaults precedent this
  decision extends to `release:`
- [ADR-0028](0028-drop-cocogitto-generator.md) — hard-cutover-no-deprecation precedent this
  decision follows for the `disable_notes` removal
