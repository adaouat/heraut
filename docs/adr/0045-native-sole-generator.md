# ADR-0045: Native as Heraut's Sole Content Generator

- **Status**: Accepted
- **Date**: 2026-08-14
- **Deciders**: bchatard

---

## Context

heraut shipped three changelog/release-notes generators: `native` (pure Go, no external binary),
`git-cliff` (embedded TOML defaults, shells out to the `git-cliff` binary), and `communique`
(fully opaque, shells out to `communique generate`). `native` has been heraut's **canonical**
renderer since [ADR-0032](0032-native-content-generator.md)/[ADR-0033](0033-native-config-model.md)
— git-cliff was explicitly named the *design anchor* to eventually drop then, with its own package
removal deliberately sequenced after native reached remote-enrichment parity, in a separate
follow-up ADR. That follow-up point arrived once
native reached full parity: remote enrichment across GitHub/GitLab/Azure DevOps
([ADR-0034](0034-native-remote-enrichment.md)/[ADR-0042](0042-gitlab-graphql-enrichment.md)),
user-customizable templates ([ADR-0037](0037-native-template-api.md)), incremental changelog
generation ([ADR-0038](0038-incremental-changelog.md)), and commit-author attribution
([ADR-0039](0039-commit-author-attribution.md)).

Keeping two more generators around cost real, ongoing tax with no corresponding benefit: every
enrichment/policy change had to be reasoned about **per generator**, because git-cliff and native
had already diverged on `enrichment_policy: required` semantics and author-fallback rendering.
`communique` was never brought into that parity effort at all — a pure passthrough with no
heraut-owned behavior to keep in sync, and unused.

This project's own precedent — [ADR-0028](0028-drop-cocogitto-generator.md), dropping the
`cocogitto` generator — established the cutover mechanics this ADR reuses: a hard, undeprecated
removal landed directly on `main` (pre-v1.0, no branch protection, no installed base of external
users to support through a transition window).

## Decision

Remove `git-cliff` and `communique` as a hard cutover, across two phases of one epic (not a
deprecation cycle): a config-cutover phase (T177-T184, "Phase A") that made `generator:`/`config:`
hard config-load errors and went schema/sample/fixtures native-only first, followed by a
package-and-docs removal phase (T185-T193, "Phase B") that deleted the now-unreachable code and
brought the remaining docs in line:

- `ContentDriver.Generator` and `ContentDriver.Config` are **removed from the Go struct entirely**
  — not enum-shrunk to a single valid value (T188). Once there is exactly one possible generator,
  the key carries zero information; keeping a boilerplate-only-required field would be dead weight
  in every `.heraut.yml` going forward. `changelog: {}` / `release: {notes: {}}` are valid and
  meaningful on their own: "generate with native, using defaults." (This is the first time the enum
  degenerates to one value — ADR-0028's cocogitto removal still left two generators standing, so its
  own decision enum-shrunk rather than field-removed; this decision does not carry that precedent
  forward, because the two situations differ in exactly this respect.)
- `generator:`/`config:` under `changelog:` and `release.notes:` (and their per-env variants)
  became hard config-load errors ahead of the package deletion, via the existing
  `ErrRemovedConfigKey` mechanism (T177) — an existing user's config gets a clear, actionable error
  on its next `heraut` invocation, before the underlying packages ever disappeared out from under it
  mid-transition. `schema.json`, `testdata/config/` fixtures, and `docs/heraut.sample.yml` went
  native-only in the same earlier phase (T183-T184), ahead of any code deletion.
- `internal/generators/gitcliff/` and `internal/generators/communique/` (packages, embedded TOML
  defaults, Tera merge logic, contract tests, and the real-CLI embedded-config smoke test) are
  deleted outright (T188), along with `internal/pipeline/release_integration_test.go` — a real
  functional `gitcliff.New` consumer proving `HERAUT_REMOTE_URL` propagation through a subprocess,
  structurally moot once native (which never shells out for its own generation) is the sole
  generator; the broader per-platform-distinct-notes behavior it also covered remains proven by
  `internal/pipeline/release_test.go`'s `TestRun_MultiPlatform_NotesPerPlatform`. Four
  `internal/scaffold` **test** files needed narrow compile fixes for the same deleted fields; the
  corresponding **production** files (`wizard.go`, `generate.go`, `cliff.go`, `dropped.go`) were
  deliberately left untouched, reserved for the wizard redesign in a later phase of this epic.
- `heraut cliff <mode>` (`internal/cmd/cliff.go`, `internal/app/cliff.go`) is removed (T186) — it
  existed solely to show the effective merged git-cliff TOML, meaningless with no git-cliff config
  to merge.
- `heraut check cliff` and its default-output "Cliff" section, plus `internal/app/check.go`'s
  git-cliff/communique binary-presence probes (`configuredGenerators`, `CheckCliff`), are removed
  (T187) — `heraut check runtime` now probes only `git`, `gh`, `glab`.
- `internal/app/pipeline.go`'s `buildGenerator` collapses from a three-way switch to an
  unconditional `native.New(...)` call; `usesNative` (used throughout `internal/app` to gate forge
  resolution) is deleted, since every driver is native by construction now (T185).
- `README.md`, `docs/specs/02-configuration.md`, and `docs/specs/05-generators-and-platforms.md`
  drop `generator`/`config` from the documented `ContentDriver` shape and the generator-comparison
  content built around it (T190-T192): README's prerequisites table and tool list, spec 02's
  "Content generators" field table (replaced with a "Content generation" section), and spec 05's
  three-generator comparison (replaced with a single `## Generator` section describing `native`,
  with the `forges:`/auto-detection content that was never git-cliff-exclusive kept and re-homed).
- `.claude/rules/testing.md`'s "Real-CLI smoke tests" exception loses its only remaining example
  (cocogitto's was already dropped by ADR-0028) — native has zero external-binary dependency for
  generation, so the category currently has no live example; `docs/specs/06-dx-and-testing.md`'s
  matching unit/contract-layer lists and "hard-won edge cases" were updated to agree (T189).

### What does not change

Historical ADRs that mention git-cliff/communique in passing (0006, 0011, 0012, 0021, 0023-0026,
0032-0043...) remain as accurate records of the decisions made *at the time* — they are not
rewritten, per ADR-0028's own explicit rule for this exact situation.
[ADR-0010](0010-embedded-cliff-toml-default.md) and [ADR-0022](0022-fat-injection-thin-templates.md)
are the two deliberate exceptions: each one's entire subject — which embedded TOML defaults heraut
ships for git-cliff (0010), and the fat-injection env-var mechanism feeding git-cliff's own
templates (0022) — becomes moot, not just tangentially mentioned, so both are marked `Superseded
by ADR-0045` in `docs/adr/README.md` (see also T238, 2026-08-27).

## Consequences

- Anyone with `generator: git-cliff` or `generator: communique` in an existing `.heraut.yml` gets a
  hard config error on their next `heraut` invocation (already true as of the earlier config-cutover
  phase of this epic, T177) — this phase removes the underlying packages the error was warning them
  away from. No deprecation warning period precedes this.
- heraut ships with zero external-binary dependency for changelog/release-notes generation —
  `native` is pure Go, embedded in the `heraut` binary. `git`/`gh`/`glab` remain required for git
  operations and publishing, unrelated to content generation.
- The Docker image and `.config/mise/config.toml` tool matrix shrink (a later, separate phase of
  this epic).
- `internal/scaffold/wizard.go`'s generator-choice prompt (still offering `git-cliff`/`communique`
  as live options today, a known-stale leftover this phase deliberately does not touch beyond the
  minimum test-file compile fixes described above) is simplified in a later, separate phase of this
  epic — see that phase's own scope note in `docs/tasks/native-generator-roadmap.md`.
  **Update (T238, 2026-08-27): resolved.** Phase C (T195-T202,
  `docs/tasks/native-generator-roadmap.md`) landed the wizard simplification; `wizard.go` has no
  generator-choice prompt at all today, and `internal/scaffold/cliff.go` (cited above as a
  production file left untouched) no longer exists — Phase C removed it along with the rest of the
  generator-choice machinery.
- Future changelog-generator additions (if any) are evaluated against `native`'s existing coverage
  first, to avoid reintroducing a generator-dispatch tax this epic just removed.
