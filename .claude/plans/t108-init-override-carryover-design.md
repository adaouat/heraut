# T108 design spike — carry through per-platform/per-env overrides on `heraut init` update

## Objective

`heraut init`'s "Update it?" flow regenerates `.heraut.yml` from `scaffold.Answers`
populated by `ConfigToAnswers`. Three field categories are still silently dropped
(warned about by `scaffold.DroppedFields`, added in T99):

- `release.platforms.<name>.base_url` (when it overrides the type default)
- `release.platforms.<name>.draft` / `.prerelease`
- `environments.<name>.changelog` / `environments.<name>.release` (per-env content
  overrides)

T99 explicitly deferred carrying these through because `runPlatformWizard` and
`runEnvWizard` reset `a.Platforms` / `a.Environments` to `nil` and rebuild them from
wizard prompts — at the time, there seemed to be no stable identity to reattach
passthrough values to. T107 (done) already carried through the three fields that
*don't* have this problem (`release.assets`, `tickets`, `remote_metadata` — single
top-level values, no per-entry identity question).

This spike decides **how** (or whether) to solve the identity problem for the
remaining per-platform/per-env fields, without writing implementation code.

## Investigation: why T99 called this "no stable identity"

### Platforms (`internal/scaffold/wizard.go`, `runPlatformWizard`)

- `PlatformAnswer` (today) has no `Name` field at all. `answersToConfig` derives
  `config.Platform.Name` from `Type` plus a dedup counter: `"github"`, `"gitlab"`,
  `"gitlab-2"`, etc. (`internal/scaffold/generate.go`).
- Per ADR-0025, `config.Platform.Name` is a **required, user-meaningful, free-form
  label** — e.g. `gitlab-com` / `gitlab-internal` for the two-instance scenario the ADR
  introduced. A user who hand-edited `.heraut.yml` to use such names has names that
  `answersToConfig`'s `type`/`type-N` scheme will never reproduce.
- Consequence: **matching rebuilt entries to existing entries by `Name` is unreliable**
  whenever the existing config has a custom name — which is exactly the case ADR-0025
  was written for.
- However, `runPlatformWizard` rebuilds entries **in the order the user re-enters
  them**, one `PlatformAnswer{Type, Repository/Project, TokenEnv}` at a time. In the
  common case (no reordering, no add/remove, same set of platform types), the *n*-th
  entry of type `T` in the rebuilt list corresponds to the *n*-th entry of type `T` in
  the original list — a **positional, type-scoped** correspondence that doesn't depend
  on names matching.

### Environments (`internal/scaffold/wizard.go`, `runEnvWizard`)

- `EnvAnswer` **does** have `Name`, and it's the map key in both
  `cfg.Environments` and the rebuilt `a.Environments`. Per-env overrides
  (`Changelog`/`Release`) are keyed by that same name in `config.Environment`.
- So for environments, **name-based matching is the natural and already-available
  identity** — no new field needed. An env whose name is unchanged across the rebuild
  has an unambiguous match; a renamed/removed env has no match (today's drop+warn is
  correct for that case).

## Candidate approaches

### Approach A — type-scoped positional matching (platforms) + name matching (envs)

1. `PlatformAnswer` gains passthrough fields: `Name string`, `BaseURL string`,
   `Draft bool`, `Prerelease bool`. `ConfigToAnswers` populates them verbatim from
   `cfg.Release.Platforms[i]`.
2. Before `runPlatformWizard` resets `a.Platforms = nil`, it snapshots the incoming
   slice grouped by `Type` (preserving order within each type). As each new
   `PlatformAnswer` is built for type `T`, it's matched against the next unconsumed
   snapshot entry of type `T` (if any); on match, `Name`/`BaseURL`/`Draft`/`Prerelease`
   are copied onto the new entry. No match (new platform, or more entries of type `T`
   than before) → zero values, same as today (`answersToConfig` derives `Name`,
   `BaseURL` falls back to the type default, `Draft`/`Prerelease` default `false`).
3. `answersToConfig` uses `p.Name` when non-empty (falling back to the
   `type`/`type-N` derivation otherwise — needed for genuinely new entries) and writes
   `BaseURL`/`Draft`/`Prerelease` through unchanged.
4. `EnvAnswer` gains `Changelog *config.ContentDriver` and `Release *config.EnvRelease`.
   `ConfigToAnswers` populates them from `cfg.Environments[name]`. `runEnvWizard`
   rebuilds `a.Environments` keyed by the name the user enters; before resetting, it
   snapshots the incoming slice by `Name`. If the rebuilt entry's `Name` matches a
   snapshot entry, copy `Changelog`/`Release` across. Renamed/new envs get `nil`
   (dropped, same as today — `DroppedFields` keeps warning for *those* cases, but the
   common "edit bump mode, keep the name" case now round-trips).
5. `DroppedFields` narrows: a platform's `base_url`/`draft`/`prerelease` is only
   reported if it has **no positional counterpart of the same type** in... but
   `DroppedFields` runs on the *loaded config*, before the wizard runs, so it can't know
   the rebuild outcome. Simplify: drop the pre-wizard warning for these categories
   entirely (the passthrough now makes the *common* case lossless), and instead emit a
   **post-wizard** diff (`internal/cmd/init.go`, after `RunWizard` returns) comparing
   `DroppedFields(cfg)` (original) against what would be lost given the *new*
   `a.Platforms`/`a.Environments` lengths/types/names — i.e. only warn when the
   positional/name match actually failed (reorder, add, remove, rename, type change).

**Consequences**
- *Positive*: the overwhelmingly common case — re-running `init` to tweak one field
  (e.g. change `token_env`, add a second env) without touching platform
  count/order/type or environment names — now round-trips `base_url`, `draft`,
  `prerelease`, and per-env `changelog`/`release` with **zero new prompts**.
- *Positive*: degrades gracefully — on any mismatch, behavior is identical to today
  (drop + warn), so this is strictly additive from a correctness standpoint.
- *Negative*: the "type-scoped positional" rule is an implicit, undocumented
  convention — if a user has two `gitlab` platforms and reorders them, their
  `base_url`/`draft`/`prerelease` silently swap rather than dropping (swap is *wrong*
  but not *warned*). Needs either: (a) accept this edge case (rare: requires 2+
  same-type entries *and* reordering *and* per-entry overrides), or (b) also compare a
  weak secondary key (`Repository`/`Project`) when matching same-type entries, only
  falling back to positional when that's ambiguous too.
- *Negative*: moving `DroppedFields` from pre-wizard to post-wizard changes
  `printDroppedFieldsWarning`'s call site and timing (after the wizard, before the
  "write this config?" confirm) — a UX change from T99, though arguably an
  improvement (the warning becomes accurate instead of "this *might* be dropped
  depending on what you do next").

### Approach B — skip the rebuild sub-flow by default on "Update it?"

`runPlatformWizard`/`runEnvWizard` only run if the user explicitly opts in (new
confirm prompts: "Edit release platforms?" / "Edit environments?"). If declined,
`a.Platforms`/`a.Environments` keep the values `ConfigToAnswers` already populated
(including, after this approach also extends `ConfigToAnswers` for the passthrough
fields, the full original entries byte-for-byte) — `answersToConfig` would need to
round-trip every field of `config.Platform`/`config.Environment`, not just the
wizard-editable subset.

**Consequences**
- *Positive*: trivially lossless when the user declines to edit platforms/envs —
  no identity-matching logic at all.
- *Negative*: as soon as the user *does* opt in to edit (e.g. to add one environment),
  we're back to the full rebuild-from-scratch problem for the *other* environments —
  Approach B alone doesn't solve the multi-entry case, it just avoids it when the user
  doesn't touch that section. Approach A's matching logic would still be needed for the
  "opted in" path, making B a strict superset of effort if both are wanted, not an
  alternative.
- *Negative*: two new confirm prompts on every "Update it?" run is wizard friction for
  the common case (most re-runs *do* want to tweak something in these sections, even if
  just adding a platform/env).

### Recommendation

**Approach A.** It's strictly additive (no regression vs. today on any path), requires
no new prompts, and handles the actual reported pain point (`release.assets` was T107;
`base_url`/`draft`/`prerelease`/per-env overrides are the realistic next complaint) for
the dominant case (stable platform types/order, stable env names). Approach B doesn't
avoid the matching problem, it only narrows when it's hit — and adds prompt friction
heraut's existing wizard has otherwise avoided (it pre-fills from git remote, defaults
aggressively, etc.).

Recommend deferring the "weak secondary key" refinement (A's negative, reorder of
same-type platforms) to a follow-up note in `DroppedFields`'s doc comment rather than
implementing it now — it requires `Repository`/`Project` to be present and unchanged,
which adds another layer of matching for an edge case (2+ same-type platforms *and*
reordering *and* overrides set) that hasn't been reported.

## Open questions for human review

1. **Post-wizard `DroppedFields` timing** (Approach A point 5): moving the warning to
   after `RunWizard` is a UX change beyond T99's "warn before the wizard runs"
   placement. OK to proceed, or keep the pre-wizard warning *in addition to* a
   post-wizard accuracy check (more conservative, two warnings in some runs)?
2. **`--defaults` path**: `heraut init --defaults` on an existing file calls
   `ConfigToAnswers` then `GenerateYAML` directly, *skipping* `RunWizard` entirely (see
   `internal/cmd/init.go`). Under Approach A, `ConfigToAnswers` already populates the
   passthrough fields and `answersToConfig` writes them straight through — so
   `--defaults` on an existing config becomes **lossless for all six T99 categories**
   with no rebuild step at all. Confirm this is desirable (it's a strict improvement,
   but worth flagging since `--defaults` is documented as "write opinionated defaults
   non-interactively").
3. Scope split for implementation: T108 as currently scoped (`Scope: L`) bundles
   platform passthrough + env passthrough + `DroppedFields` rework. Split into two
   roadmap tasks (platforms vs. environments) so each fits a single session, or keep
   as one task given they share the snapshot/match pattern?

## Out of scope for this spike

- No code changes.
- No task breakdown (per session scope: "decision only"). Once the open questions
  above are resolved, the T108 roadmap entry should be updated with the chosen
  approach and, if split per question 3, broken into T108a/T108b (or T108/T109)
  before implementation starts.
