# ADR-0047: Changelog output resolves after version, not at config-build time

- **Status**: Accepted
- **Date**: 2026-08-28
- **Deciders**: bchatard
- **Related**: [ADR-0032](0032-native-content-generator.md) (native content generator),
  [ADR-0037](0037-native-template-api.md) (template overridable blocks — precedent for "a single
  string field grows optional token syntax, literal values keep working unchanged"),
  [ADR-0045](0045-native-sole-generator.md) (native sole generator)
- **Design doc**: [`docs/superpowers/specs/2026-08-28-changelog-rotation-design.md`](../superpowers/specs/2026-08-28-changelog-rotation-design.md)
- **Roadmap**: [`docs/tasks/changelog-rotation-roadmap.md`](../tasks/changelog-rotation-roadmap.md), T244–T249

---

## Context

`changelog.output` (and its per-environment override) gained optional versioning-strategy tokens —
`{YYYY}`, `{MM}`, `{DD}`, `{WW}`, `{QQ}`, `{SS}`, `{SPRINT}` for CalVer strategies; `{MAJOR}`,
`{MINOR}` for SemVer — so a project's changelog can rotate into one file per calendar period or per
release line (e.g. `CHANGELOG_{YYYY}.md`, `CHANGELOG_{MAJOR}.md`) instead of one ever-growing file.

Substituting those tokens into a concrete path requires the *resolved* version — the actual `YYYY`
or `MAJOR` a release carries. Two placements looked natural and are both wrong:

1. **At `buildChangelogPipelineConfig`/`buildReleasePipelineConfig` time** (`internal/app/pipeline.go`),
   alongside where the rest of `native.Config` — really `*config.ContentDriver`, native has no
   separate config type — is assembled today. This is where a reader unfamiliar with the rest of
   the pipeline would look first. It's wrong because `ChangelogPipeline.Run()` /
   `Pipeline.Run()` (`internal/pipeline/{changelog,release}.go`) call `p.resolver.Resolve()`
   **after** this point, and a manual `--version` override bypasses the resolver's own date/bump
   computation entirely (confirmed: `tagfmt.ValidateVersionOverride` only checks for emptiness and
   whitespace — heraut is deliberately strategy-agnostic about override shape). Computing "what the
   version will be" ahead of resolution, just to fill in a filename, would mean duplicating logic
   that could disagree with the resolver's actual answer.
2. **Inside `native` itself**, since it already owns the file I/O `Output` drives. Wrong for a
   different reason: `internal/generators/native` may only import `internal/{port,config,
   conventionalcommit}` (`.claude/rules/coding.md`'s layer table) — never `internal/versioning`.
   Native would need calver/semver token semantics it is not allowed to have.

## Decision

`config.ContentDriver.Output`/`TagPattern` keep their existing plain-string type and meaning —
`{TOKEN}` placeholders are optional, and a literal path with none is entirely unaffected. Token
substitution happens in a new unexported `rotatingGenerator`
(`internal/app/changelog_rotation.go`), a `port.Generator` decorator wrapping the constructed
`native.Generator`. Its `Generate(tag, lc)` — called by `internal/pipeline` **after** the tag is
resolved, receiving the one value guaranteed correct at every call site regardless of override —
computes the concrete `Output` and (when the user hasn't already set an explicit `tag_pattern`) a
matching tag-scoping pattern from `tag`, using `internal/versioning/calver` (`ParseFormat`,
`ParseVersion`, `TokenKindFromName`, `RenderToken`, `BucketPattern`) or
`internal/versioning/semver` (`MajorMinor`, `RotationPattern`) depending on the active strategy,
then builds a **fresh** `native.Generator` with those concrete values and delegates to it.
`internal/app` is the only new code — it already imports both `internal/versioning/*` and
`internal/generators/native` per the layer rules — and it's additive: `buildChangelogPipelineConfig`
and `buildReleasePipelineConfig`'s existing construction of `native.Generator` is unchanged, just
wrapped.

`internal/pipeline.ChangelogConfig.ChangelogFile` / `pipeline.Config.ChangelogFile` are *also*
resolved once at config-build time, for a separate reason (the "Commit changelog" step needs a
path to `git add`) — this has the identical problem, one layer up. Fixed with a small structural
`outputPathReporter` interface (`internal/pipeline/changelog_output.go`, `LastOutputPath() string`)
that `rotatingGenerator` implements; the commit/summary steps in both pipelines now call a shared
`resolvedChangelogFile(gen, configured)` helper that prefers the generator's own post-`Generate()`
report when present, falling back to the static field unchanged for every non-rotating config.
`internal/pipeline` never imports `internal/app` — the interface check is purely structural.

**A third, unplanned finding: `native` needed one small addition after all.** Tag-scoping a
rotation bucket (e.g. `tag_pattern: "^2026\."`) correctly excludes every prior-bucket tag from
`buildAllSections`'/`generateIncremental`'s historical walk — but it also leaves the *new* bucket's
first release with no in-scope previous tag to bound its own commit range by. Native's existing
`latest := tags[0]` (empty when the scoped list is empty) falls through to `commitRange("", "HEAD")`,
which means "since the beginning of history" — silently duplicating every prior bucket's entries
into the new file. Found by writing a real-git-repo test (T248) *before* any fix code existed, per
this project's TDD discipline, rather than being anticipated in the design.

Fixed with `ContentDriver.PreviousTagOverride` (`yaml:"-"`, alongside the existing
`HerautVersion`/`RegenerateChangelog`/`Force` app→native signaling fields — none user-configurable,
all set by `internal/app`) and a shared `Generator.newSectionBound(scopedTags)` helper that both the
bootstrap and splice paths call instead of inlining `tags[0]` directly. `rotatingGenerator` sets the
override to the true previous tag via one extra, independent `git tag -l <prefix>* --sort=
-version:refname` query — the same incantation `calver`/`semver`'s own resolvers already run
internally for their own bump/period-key computation, needed here because `port.Generator.Generate
(tag, lc)` never carries the resolver's own `Result.CurrentTag` through. Setting the override
unconditionally (whenever any previous tag exists, not only at a detected boundary) is safe: the
current bucket's own latest tag, when one exists, is by construction *also* the global latest tag —
there is never a case where the two disagree except exactly the boundary-crossing one being fixed.
Native still learns nothing about calver/semver/rotation semantics; it just prefers an explicit
bound over its own derived one when given one, which does not reopen the layer-import question this
ADR otherwise answers.

## Consequences

- `changelog.output` gains rotation-token syntax with zero migration cost: every existing literal
  path is a no-op path through `wrapWithRotation` (`len(tokens) == 0` returns the original
  generator unchanged).
- The decorator-around-`port.Generator` shape is now this project's answer to "a generator config
  value needs the resolved version" — a pattern other future version-dependent generator config
  could reuse instead of restructuring pipeline call order.
- `internal/config/changelog_rotation.go`'s static validation (token vocabulary, prefix-order,
  per-env rejection — T246) is a **deliberately duplicated**, lighter-weight scanner, not a reuse of
  `calver.ValidateRotationTokens` — `internal/config` sits at the bottom of the layer graph and must
  import nothing else from this repo. One consequence: it does not re-check T244's "rotation
  boundary must be followed by a literal separator" refinement, since that needs literal-segment
  tracking the lighter scanner doesn't do. A config with an ambiguous boundary (e.g. `YYYYMM.PATCH`
  rotating by `{YYYY}` alone) passes `heraut check config` but fails for real once
  `rotatingGenerator` calls `calver.ValidateRotationTokens` at generate time — an accepted v1 gap
  for a rare format shape.
- `port.Generator.Generate(tag, lc)`'s signature is unchanged — `rotatingGenerator` derives
  everything it needs (the true previous tag included) independently rather than the interface
  growing a parameter every future generator concern would otherwise need.

## Alternatives considered

- **Resolve rotation eagerly, before `Resolve()`, by independently recomputing what the version
  will be.** Rejected: duplicates resolver logic that could disagree with the real answer, and
  breaks entirely for a manual `--version` override, which the resolver never even runs its own
  computation for.
- **Teach `native` calver/semver token semantics directly.** Rejected: violates the layer rule
  barring `internal/generators/native` from importing `internal/versioning`, and would couple a
  generator meant to stay versioning-agnostic to two specific strategies.
- **Detect the boundary crossing explicitly and only set `PreviousTagOverride` then.** Rejected in
  favor of setting it unconditionally whenever a previous tag exists — simpler, and provably
  equivalent to the scoped derivation in every non-boundary case (see Decision above), so there is
  no behavior difference to protect with the extra detection logic.

## References

- `internal/app/changelog_rotation.go` — the decorator, `latestMatchingTag`, `resolveDriver`.
- `internal/pipeline/changelog_output.go` — `outputPathReporter`, `resolvedChangelogFile`.
- `internal/generators/native/generator.go` — `newSectionBound`.
- `internal/config/config.go` — `ContentDriver.PreviousTagOverride`.
- `internal/config/changelog_rotation.go` — static validation.
- `internal/versioning/calver/rotation.go`, `internal/versioning/semver/rotation.go` — the
  per-strategy token/bucket-pattern helpers.
