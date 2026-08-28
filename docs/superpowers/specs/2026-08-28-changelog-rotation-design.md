# Rotating Changelog File Naming — period/version-scoped `changelog.output`

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-08-28
- **Author**: bchatard (with Claude)
- **Related ADRs**: 0010 (embedded native defaults), 0032 (native generator), 0037 (template
  overridable blocks — precedent for "a single string field grows optional token syntax, literal
  values keep working unchanged"), 0045 (native sole generator)
- **New ADR required**: yes — ADR-0047, working title "Changelog output resolves after version,
  not at config-build time"
- **Roadmap**: own dedicated file — `docs/tasks/changelog-rotation-roadmap.md`, T244+, with a
  pointer from the main `docs/tasks/roadmap.md` (new Phase), matching the native-generator,
  forge-abstraction, and release-config epics' pattern.
- **Origin task**: `docs/tasks/roadmap.md` → T243 (design spike, this doc resolves it).

---

## Problem

`changelog.output` is a single static path (default `CHANGELOG.md`). A project that wants one
changelog file per calendar period (CalVer) or per release line (SemVer) — e.g. `CHANGELOG_2026.md`
rotating yearly, or `CHANGELOG_1.md` per major version — has no way to express that today; the
field is a literal string handed straight to `os.ReadFile`/`os.WriteFile`.

This design resolves T243's five open questions (scope, tag-scoping, rollover, cross-command
impact, interaction with per-env/`--regenerate`) and settles on a mechanism, informed by reading
the actual resolver/generator/pipeline code rather than assumption:

- `internal/versioning/calver/resolver.go` already has a **period** concept — `periodKey(tokens,
  values)` — used to decide when `PATCH` resets. It builds a key from whichever non-`PATCH`,
  non-literal tokens appear in the format, in format order. This is the exact mechanism rotation
  needs, generalized to a caller-chosen *subset* of leading tokens instead of all of them.
- `internal/generators/native/generator.go`'s tag walk already supports two independent knobs —
  `TagGlob` (a `git tag -l` glob, coarse) and `TagPattern` (a regex, applied via
  `filterByTagPattern` after fetching) — and the anchor/bootstrap/splice algorithm operates
  entirely on whatever tag set those two knobs leave it. Scoping tags correctly requires **zero
  changes to that algorithm**.
- `internal/pipeline/changelog.go`'s `Run()` calls `p.resolver.Resolve()` **before**
  `p.cfg.Changelog.Generate(result.Tag, changelogCtx)`. The generator is invoked with the final,
  resolved tag — after a manual `--version` override has already been applied, if one was given.
  This matters: rotation tokens must come from the *actual resolved version*, not an independently
  recomputed "now", because manual overrides bypass the resolver's own date/bump computation
  entirely. Computing "what the version will be" ahead of resolution to fill in `{YYYY}`/`{MAJOR}`
  would mean duplicating the resolver's own logic and could disagree with it. The generator already
  receives the one value that's guaranteed correct at every call site: `tag`.

## Goals

- `changelog.output` (root and per-env) accepts optional tokens; a literal string with no `{...}`
  behaves exactly as today. No new config field.
- CalVer/CalVer-per-env strategies: tokens are the format's own vocabulary — `YYYY, MM, DD, WW, QQ,
  SS, SPRINT` (confirmed: include `SPRINT` — symmetric with the rest, free once the mechanism
  exists, a sprint-cadence project rotating per sprint is a legitimate case).
- SemVer/SemVer-per-env strategies: tokens are `MAJOR`, `MINOR` only (never `PATCH` — that would
  create a new file per release, defeating the point of a changelog).
- Tag-scoping is **auto-derived** from the same tokens present in `output` — no second field to
  keep in sync, no way for the filename and the tag set backing it to silently drift apart.
- Native's anchor/bootstrap/splice algorithm changes not at all. Native stays ignorant of
  calver/semver token semantics (see layer-rule note under Design §3).

## Non-goals (v1)

- **Per-env strategies are explicitly out of scope for v1** (confirmed). `withEnvDerivations`
  already auto-derives `TagPattern`/`TagGlob` from the per-env `tag_format` when the user hasn't
  set an explicit `tag_pattern`. Composing that derivation with a second, independently-sourced
  rotation-derived constraint is real additional complexity — safely intersecting two
  auto-derived regexes needs to be proven once, in isolation, on the flat case first. Using
  rotation tokens in `output` under `semver-per-env`/`calver-per-env` is a **config error** at
  `heraut check config` time in v1, not a silent no-op — consistent with how this project treats
  unreachable config states elsewhere (e.g. the zero-resolvable-publish-destination rule).
- **`heraut init` does not gain a rotation prompt** (confirmed). The wizard already has documented
  coverage gaps; this isn't the moment to grow its scope. Manual `.heraut.yml` editing only.
- **`release.notes` is unaffected.** It shares the `ContentDriver` struct with `changelog`, but
  `Output` is never written to disk for release notes — the generated string is handed to the
  platform driver, not persisted. Rotation is a `changelog`-only concept.
- **Rotation tokens computed from a tag's git commit date are out of scope.** See Design §1 — this
  is what makes SemVer rotation tractable in the first place, so it's a hard constraint, not a
  deferred nice-to-have.
- **No cross-file `--regenerate`.** `--regenerate` regenerates the *current* bucket's file only
  (whatever the current run's resolved tag maps to). Reaching back into a prior period's file to
  regenerate it isn't a supported "time travel" operation.
- **`heraut whatsnew` needs no changes.** It reads heraut's own release history via the GitHub
  Releases API (with an embedded offline fallback) — never the user's project changelog. Flagged
  as a risk in T243's filing; retracted after re-checking what `whatsnew` actually reads.

## Design

### 1. The hard constraint: rotation tokens must be a prefix of the format, in format order

For CalVer, a rotation token is only valid if it appears in `versioning.format`, and the full set
of requested rotation tokens must form a contiguous *prefix* of the format's non-`PATCH` tokens (in
the order they appear in the format). E.g. `format: "YYYY.MM.PATCH"` permits `{YYYY}` or
`{YYYY}_{MM}`, but not `{MM}` alone (doesn't uniquely bucket across years) and not `{QQ}` (not in
the format at all).

This is a real constraint, not a simplification of convenience: the alternative — deriving a token
like `{QQ}` from the tag's *git commit date* when the format doesn't carry quarter information —
decouples the changelog bucket from what the version string itself claims, and invites exactly the
version-vs-date divergence class of bug this codebase has a documented history of guarding against
(leap years, period-boundary edge cases in the CalVer test suite; see `.claude/rules/testing.md`'s
"preserve hard-won edge cases"). Constraining rotation to tokens the format already tracks means
tag-scoping is always derivable by parsing the *version string*, never by shelling out for a tag's
commit date.

For SemVer, `MAJOR`/`MINOR` are always structurally present in any `X.Y.Z` version, so this
constraint is automatically satisfied — no format-dependency check needed there.

### 2. Token vocabulary is strategy-family-gated, validated statically

At `heraut check config` (and at load time generally, alongside the other enum/strategy-specific
checks in `internal/config/validator.go`), a new check parses `{TOKEN}` placeholders out of
`changelog.output` (and any per-env override) and rejects:

- A token outside the active strategy family's vocabulary (e.g. `{YYYY}` under `semver`).
- For CalVer: a token not present in `versioning.format`, or a requested token set that isn't a
  prefix of the format's token order (§1).
- Any use under a `*-per-env` strategy (non-goal, §above) — a clear, actionable error, not a silent
  ignore.

This means a broken rotation pattern is caught before a release ever runs, matching how strategy
and enum validation already work — never surfaced for the first time mid-`heraut release`.

> **Update (T246, 2026-08-28).** `internal/config` sits at the bottom of the layer graph and must
> import nothing else from this repo, so this check cannot call §1's `calver.ValidateRotationTokens`
> or reference `calver.TokenKind` — it reimplements a small, intentionally-duplicated token scanner
> instead (`internal/config/changelog_rotation.go`), consistent with how the existing SPRINT check
> in `validateStrategySpecific` already uses a crude `strings.Contains` rather than
> `calver.ParseFormat`. One consequence: T244's "rotation boundary must be followed by a literal
> separator" refinement needs literal-segment tracking that this lighter scanner doesn't do, so it
> is **not** re-checked here — a config with an ambiguous boundary (e.g. `YYYYMM.PATCH` rotating by
> `{YYYY}` alone) passes `heraut check config` but fails for real once §3's decorator calls
> `calver.ValidateRotationTokens` at generate time. Accepted as a v1 gap for a rare format shape
> rather than duplicating calver's full tokenizer across the layer wall.

### 3. Where substitution happens: a thin `port.Generator` decorator in `internal/app`, resolved at `Generate()` time

Per the Problem section, the concrete output path can only be computed once the tag is resolved —
which happens *after* `buildChangelogPipelineConfig` runs, inside `ChangelogPipeline.Run()`. Layer
rules (`.claude/rules/coding.md`) also settle *where* the token math can live:
`internal/generators/native` may only import `internal/{port,config,conventionalcommit}` — **not**
`internal/versioning/*` — so native itself must never learn calver/semver token semantics.

The shape: `internal/app` wraps the constructed `native.Generator` in a small unexported decorator
(new file, e.g. `internal/app/changelog_rotation.go`) that also implements `port.Generator`. Its
`Generate(tag string, ctx GenerateContext)`:

1. Detects whether `{TOKEN}` placeholders are present in the raw `Output`/derived tag-scoping (none
   → delegate straight through, zero overhead, zero behavior change for every existing config).
2. Parses `tag` into its bare version (existing `tagfmt`/strategy-specific parsing).
3. For CalVer: `calver.ParseFormat` + `calver.ParseVersion` → `Values`; render each requested token
   into the output pattern (`%04d` for `YYYY`, etc. — reusing the same formatting `RenderVersion`
   already uses per token kind).
4. For SemVer: split the bare version on `.` → `MAJOR`/`MINOR` ints; substitute directly.
5. Derives the tag-scoping pattern (§4) from the same parsed values.
6. Constructs a **fresh** `native.Generator` with the now-concrete `Output`/`TagPattern` (via the
   same constructor `buildGenerator` already uses — cheap, called once per pipeline run, not
   performance-sensitive) and delegates `Generate(tag, ctx)` to it.

Net effect: `native.Config.Output`/`TagPattern` keep their existing string type and meaning. No
change to `internal/generators/native` at all. `internal/app` — which already imports both
`internal/versioning/*` and `internal/generators/native` per the layer table — is the only new
code, and it's additive (a decorator, not a modification to `buildChangelogPipelineConfig`'s
existing construction path).

### 4. Tag-scoping derivation

Given the parsed `Values` (or `MAJOR`/`MINOR`) and the requested rotation tokens, build a regex
matching only tags in the same bucket: render the *literal* prefix of the format (tokens up to and
including the last requested rotation token, with their known values, plus the literal separators
between them — e.g. format `YYYY.MM.PATCH` rotating by `{YYYY}` renders literal prefix `2026.`),
anchored at the start (respecting `tag_prefix`, `regexp.QuoteMeta`'d), followed by a boundary that
rules out partial-digit false matches (e.g. `2026` must not match a hypothetical `20260`-prefixed
tag). This reuses the same token-to-regex approach `calver`'s own `buildParseRegex` already uses
internally — a natural, mechanical extension, not a new parsing engine. SemVer's version is
simpler: `MAJOR` alone → prefix `1.`; `MAJOR.MINOR` → prefix `1.4.`.

This derived pattern is only applied when the user hasn't already set an explicit `tag_pattern` —
same precedence rule `withEnvDerivations` already uses for per-env scoping (`if driver.TagPattern
== ""`), applied consistently rather than inventing a second precedence rule.

### 5. Rollover needs no special handling

At a period/version boundary, the target file (e.g. `CHANGELOG_2027.md`) doesn't exist yet. Native's
existing "missing file → bootstrap every matching tag into it" path already produces the correct
result once tags are correctly scoped: if this is genuinely the first tag in the new bucket, exactly
one entry gets written. If rotation is turned on for an existing project mid-period (tags already
exist in the new bucket with no rotated file for it yet), bootstrap correctly backfills all of
them — which is the *right* behavior, not an edge case to special-case around.

> **Correction (T248, 2026-08-28).** The claim above is only half right, and the missed half is the
> *normal* case, not an edge case: at the exact boundary — the new bucket's genuinely first release,
> immediately following the old bucket's last one — the scoped tag list is empty, so native's own
> `tags[0]`-derived bound for the new section is also empty, which `commitRange` treats as "since
> the beginning of history." Confirmed with a real-git-repo test before touching any code: a 2026
> bucket's first release absorbed the 2025 bucket's commits too. Fixed with one small,
> **deliberate** exception to Design §3's "native needs no changes" framing: a new
> `ContentDriver.PreviousTagOverride` field (internal, `yaml:"-"`, alongside the existing
> `HerautVersion`/`RegenerateChangelog`/`Force` app→native signaling fields) that native's
> `buildAllSections`/`generateIncremental` prefer over their own scoped-tag derivation when set.
> `internal/app`'s decorator sets it to the true previous tag (an independent, unscoped `git tag -l`
> query — the same incantation `calver`/`semver`'s own resolvers already use) whenever one exists;
> this is always safe, never just for the boundary case, since the current bucket's own latest tag
> (when one exists) is by construction *also* the global latest tag — there's never a case where the
> two disagree except exactly the one being fixed. Native remains ignorant of *why* the override is
> set — it never learns about calver/semver/rotation semantics — so this doesn't reopen the
> layer-import question Design §3 exists to answer, but it does mean "zero changes to native" was
> not quite true; "no changes to native's *tag-scoping* mechanism, one small addition to its
> *range-bounding* logic" is the accurate version.

## New ADR-0047 outline

**Title**: Changelog output resolves after version, not at config-build time.

**Decision**: `native.Config.Output`/`TagPattern` may contain versioning-strategy tokens
(`{YYYY}`, `{MAJOR}`, …). Substituting them requires the pipeline's resolved tag, which is only
available inside `ChangelogPipeline.Run()`, after `Resolve()` — not at
`buildChangelogPipelineConfig` time, where the rest of native's config is assembled today.
Resolution is deferred via a `port.Generator` decorator in `internal/app` rather than by
restructuring `buildChangelogPipelineConfig`'s call order.

**Why this needs its own ADR** (vs. just a roadmap note): it fixes a non-obvious constraint a
future contributor could easily get wrong — "just compute the rotated filename where the rest of
`Output` gets set" looks like the natural place to a reader unfamiliar with why manual `--version`
overrides make that wrong. It also establishes the decorator-around-`port.Generator` shape as the
project's answer to "config value needs the resolved version," which is a pattern other future
version-dependent generator config could reuse.

**Also document** (found during T248, see the §5 correction above): `ContentDriver.PreviousTagOverride`
— native's one small addition — and why it's safe to set unconditionally (the current bucket's
latest tag, when one exists, is always also the global latest tag) rather than only at a detected
boundary crossing.

## Roadmap placement

New `docs/tasks/changelog-rotation-roadmap.md`, T244+, following the release-config epic's
structure (own file, pointer from `docs/tasks/roadmap.md`'s Phase list). Expected breakdown (sizing
TBD when filed):

1. `calver`: generalize `periodKey` into a caller-scoped prefix-key + literal-prefix-regex helper
   (pure unit tests, no config/app changes).
2. `semver`: add `MAJOR`/`MINOR` extraction helper (pure unit tests).
3. `internal/config/validator.go`: static token-vocabulary + prefix-order + per-env-rejection
   checks for `changelog.output` (root + per-env).
4. `internal/app`: the `port.Generator` rotation decorator + wiring into
   `buildChangelogPipelineConfig` (contract tests: decorator computes the expected concrete
   `Output`/`TagPattern` for a given tag, delegates correctly, no-op passthrough when no tokens
   present).
5. Integration test: a real-git-repo run producing `CHANGELOG_2026.md`-style rotation across a
   simulated period boundary.
6. Docs: `schema.json`, `docs/heraut.sample.yml`, `docs/specs/02-configuration.md` (or wherever
   `changelog.output` is documented today), ADR-0047.

## Testing plan

- Unit: calver prefix-key generation for every token subset shape (single token, multi-token
  contiguous prefix, rejecting a non-prefix subset); semver Major/Minor extraction including
  pre-release/build-metadata suffixes if `IsBareVersion`-style versions can carry them.
- Unit: literal-prefix-regex construction, including `tag_prefix` values containing regex
  metacharacters (must be `QuoteMeta`'d) and the digit-boundary false-match case (§4).
- Contract: the `internal/app` decorator — given a fixed resolved tag and a tokenized raw
  `Output`, asserts the exact concrete `Output`/`TagPattern` strings passed to the wrapped
  generator; asserts a non-tokenized `Output` passes through unchanged (proves zero behavior change
  for every existing config).
- Schema: one fixture per strategy family in `testdata/config/` with a valid tokenized `output`,
  and one invalid fixture per rejected case (wrong-family token, non-prefix token, per-env use).
- Integration: reuse the existing full-pipeline real-git-repo harness for one happy-path rotation
  run spanning a simulated period boundary (two tags in different buckets → two separate files,
  each containing only its own bucket's entries).

## Resolved questions

- **Scope (T243's open question)**: both CalVer and SemVer, via two independent but structurally
  parallel mechanisms — not "CalVer-only" as originally floated in T243's filing. Resolved by the
  user's SemVer proposal (`{MAJOR}`/`{MAJOR}_{MINOR}`, extracted from the version string, no git-date
  lookup), which removed the original blocker (SemVer has no calendar info) by sidestepping the
  calendar question entirely.
- **SPRINT token**: included in the CalVer vocabulary (user-confirmed) — symmetric with the rest,
  no additional mechanism needed beyond what `{YYYY}` etc. already require.
- **Per-env**: out of scope for v1 (user-confirmed) — config error, not silent ignore, when
  combined with a `*-per-env` strategy.
- **`heraut init` wizard**: no rotation prompt in v1 (user-confirmed).
- **Config field**: reuses `changelog.output` — no new field. Matches the `tag_format` precedent
  (tokens embedded in a plain string field) rather than adding a parallel enum/flag.
- **Trigger mechanism**: implicit — `{TOKEN}` presence in `output` signals rotation, no separate
  "rotation enabled" flag. This is a design call (not user-asked), made for consistency with
  `tag_format`'s existing convention and to avoid a field that can disagree with `output`'s actual
  content. Open to override if the implementation pass finds it awkward in practice.
- **Where substitution happens**: `internal/app` decorator around `port.Generator`, resolved from
  the pipeline's already-resolved tag — not native, not config-build time. See Design §3 and
  ADR-0047 for why the two more-obvious-looking placements (native itself; config-build time) are
  both wrong.
- **Tag-scoping**: auto-derived from the same tokens as `output`, intersected with (never
  overriding) an explicit user-set `tag_pattern` when present — no second field to hand-maintain.
- **Rollover**: no special-cased step. Existing bootstrap-on-missing-file behavior is already
  correct once tags are scoped right.
- **`whatsnew`**: unaffected — reads heraut's own release history, not the user's project
  changelog. Earlier concern in T243's filing retracted after checking what it actually reads.
