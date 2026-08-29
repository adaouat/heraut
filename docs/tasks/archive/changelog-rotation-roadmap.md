# Archived task detail — changelog-rotation-roadmap.md

> Moved from [`docs/tasks/changelog-rotation-roadmap.md`](../changelog-rotation-roadmap.md) on 2026-08-29 to keep the live file lean.
> Every task here is `Done`. See the live file's "Progress at a glance" table for the
> current status index; this file is the full implementation history.


#### `[x]` T244: `calver` — caller-scoped prefix-key + literal-prefix-regex helper

`internal/versioning/calver/resolver.go`'s existing `periodKey(tokens, values)` builds a key from
*every* non-`PATCH`, non-literal token in the format, in format order — used today only for
`PATCH`-reset comparison. Rotation needs the same idea generalized to a caller-chosen **subset** of
leading tokens (e.g. just `YYYY` even when the format is `YYYY.MM.PATCH`), plus a way to render that
prefix as a **literal regex** matching any tag in the same bucket (see design doc §4: render the
literal prefix — tokens up to and including the last requested token, with their known rendered
values, plus the literal separators between them — anchored, `regexp.QuoteMeta`'d, with a
digit-boundary guard against partial-digit false matches).

Two new exported functions (naming TBD at implementation time, e.g. `BucketKey`/`BucketPattern`),
built alongside the existing `ParseFormat`/`ParseVersion`/`RenderVersion`/`periodKey` in
`internal/versioning/calver`. Must also validate/report when a requested token set is **not** a
contiguous prefix of the format's own token order (design doc §1) — the hard constraint that keeps
tag-scoping derivable from the version string alone, no git-date lookups ever.

**Files (expected):** `internal/versioning/calver/resolver.go` (or a new file in the same package)
+ tests. **Scope:** S. **Dependencies:** none.

Landed as planned, plus one refinement found during implementation: `ValidateRotationTokens` also
rejects a rotation boundary that isn't immediately followed by a literal separator in the format
(e.g. `YYYYMM.PATCH` rotating by `{YYYY}` alone — `YYYY` runs straight into `MM` with no separating
character). Without this, `BucketPattern` would need a generic non-digit-or-end fallback guard that
can incorrectly reject a valid tag when the token right after the boundary has no separator before
it either (traced through concretely: format `YYYY.MMPATCH` rotating by `{YYYY, MM}` would render
patch values with no separator, e.g. `2026.055` for MM=05/PATCH=5, and the fallback guard rejects
this even though it's in the right bucket). Rejecting the ambiguous case at validation time — a
clear config error — was judged better than shipping a bucket-scoping regex that's correct only for
some patch values. `BucketKey`/`BucketPattern` both call `ValidateRotationTokens` internally, so
this same rule applies uniformly regardless of which one a caller reaches for.

Refactored `RenderVersion` and the existing (unexported, previously untested-in-isolation)
`periodKey` to share a new `renderToken(kind, Values) string` helper instead of duplicating the
same per-token-kind switch three times (`RenderVersion`, `periodKey`, and the new
`BucketKey`/`BucketPattern`) — a pure refactor, confirmed behavior-preserving by every pre-existing
`TestRenderVersion`/`TestResolve_*`/`TestBumpFromDate_*` case passing unchanged. `go test ./...` and
`hk check` both clean.

#### `[x]` T245: `semver` — `MAJOR`/`MINOR` extraction helper

Given a bare SemVer version string, extract `MAJOR` and `MINOR` as separate values for substitution
into `{MAJOR}`/`{MINOR}` tokens. Simpler than T244 — no format-token parsing needed, SemVer's
`X.Y.Z` shape is fixed. Cover pre-release/build-metadata suffixes if `IsBareVersion`-style versions
in this codebase can carry them (check `internal/versioning/semver/bump.go`'s existing
`IsBareVersion` for what "bare" means here before assuming plain `X.Y.Z`).

**Files (expected):** `internal/versioning/semver/` (new file or extend `bump.go`) + tests.
**Scope:** XS. **Dependencies:** none.

Landed as a new `rotation.go` (mirroring `calver`'s file for this epic, rather than extending
`bump.go`) with a single `MajorMinor(version string) (major, minor int, err error)`. Confirmed via
`tagfmt.ValidateVersionOverride` that a manual `--version` is only checked for emptiness/whitespace
— heraut is strategy-agnostic about override shape — so a pre-release/build-metadata-suffixed
version (e.g. `1.4.2-rc.1`) can genuinely reach this function; `MajorMinor` handles it correctly
since the suffix always attaches to PATCH (`strings.SplitN(version, ".", 3)`, same splitting
convention `BumpVersion`/`IsBareVersion` already use), never to MAJOR or MINOR. Requires a full
`MAJOR.MINOR.PATCH` shape (rejects a bare `"1.4"` with PATCH missing entirely) rather than only
requiring 2 components — a resolved semver version missing PATCH is malformed for this strategy, not
a case to accept quietly. No semver-equivalent of `ValidateRotationTokens` (T244) is needed — unlike
CalVer, there's no format-dependent constraint; the only two valid rotation shapes are "MAJOR alone"
or "MAJOR+MINOR" (never PATCH, per the design's goals), which is a T246 (config validator) vocabulary
check rather than something this package needs to gate itself. `go test ./...` and `hk check` both
clean.

#### `[x]` T246: config validation — token vocabulary, prefix-order, per-env rejection

At `heraut check config` / load-time validation (`internal/config/validator.go`), parse `{TOKEN}`
placeholders out of `changelog.output` (root and any per-env override) and reject:

- A token outside the active strategy family's vocabulary (e.g. `{YYYY}` under `semver`).
- For CalVer: a token not present in `versioning.format`, or a requested set that isn't a prefix of
  the format's token order (uses T244's helper).
- Any use under a `*-per-env` strategy — explicit config error, not a silent ignore (design doc
  "Non-goals").

Add one valid fixture per strategy family with a tokenized `output` to `testdata/config/`, plus one
invalid fixture per rejected case.

**Files (expected):** `internal/config/validator.go` + `internal/config/validator_test.go` +
`testdata/config/` fixtures. **Scope:** S. **Dependencies:** T244, T245.

**Important correction found during implementation, worth flagging explicitly:** `internal/config`
sits at the bottom of the layer graph and must import nothing else from this repo
(`.claude/rules/coding.md`'s layer table) — it cannot call T244's `calver.ValidateRotationTokens`
or reference `calver.TokenKind` at all. Confirmed this is a real, already-enforced wall (not a
theoretical rule) by checking `validateStrategySpecific`'s existing SPRINT check, which uses a
crude `strings.Contains(format, "SPRINT")` rather than `calver.ParseFormat` — `internal/config` has
never had access to calver's real tokenizer. New `internal/config/changelog_rotation.go`
reimplements a small, intentionally-duplicated token scanner (`calverTokenOrder`,
`calverRotationTokens`, `semverRotationTokens`) rather than reusing T244/T245 directly, with a
comment pointing at calver's `knownTokens` as the source of truth to keep in sync if it changes.
One deliberate scope cut from this duplication: T244's "rotation boundary must be followed by a
literal separator" refinement is **not** re-implemented here (it needs literal-segment tracking,
not just keyword order) — that authoritative check still runs for real in T247, at the point the
`internal/app` decorator calls `calver.ValidateRotationTokens` directly with a resolved version.
This means a config with an ambiguous rotation boundary (e.g. `YYYYMM.PATCH` rotating by `{YYYY}`
alone) currently passes `heraut check config` but fails at actual `heraut changelog`/`release` time
— an acceptable v1 gap for a genuinely rare format shape, rather than deep-duplicating calver's full
tokenizer across the layer wall for it.

Wired into `validateEnums` (both the root `changelog.output` call site and the per-env loop, using
the same already-merged effective driver `validateContentDriver` uses) rather than a top-level
function that re-walks environments itself. Deliberately **not** wired into `release.notes`'
`validateContentDriver` call sites — rotation is changelog-only (design doc's non-goals:
`release.notes.output` is never written to disk). Five fixtures added: two valid
(`changelog-rotation-calver.yml`, `changelog-rotation-semver.yml`) and three invalid
(`changelog_rotation_wrong_family.yml`, `changelog_rotation_not_prefix.yml`,
`changelog_rotation_per_env.yml`), plus inline `mustLoad`-based tests for the finer-grained cases
(duplicate token, `{MINOR}` without `{MAJOR}`, multi-token valid combinations) matching the existing
test file's mix of fixture- and inline-driven coverage. `go test ./...` and `hk check` both clean.

#### `[x]` T247: `internal/app` rotation decorator + wiring

New unexported `port.Generator` decorator (design doc §3) wrapping the constructed
`native.Generator`. On `Generate(tag, ctx)`: no tokens present in the raw `Output` → delegate
straight through unchanged (zero behavior change for every existing config, must be covered by a
test asserting exact passthrough). Tokens present → parse `tag` into its bare version, compute
concrete `Output`/`TagPattern` via T244/T245's helpers, construct a fresh `native.Generator` with
those concrete values (reusing the existing `buildGenerator` constructor), delegate `Generate(tag,
ctx)` to it. Wire into `buildChangelogPipelineConfig` (`internal/app/pipeline.go`) — changelog only,
never the release-notes config-building path (design doc "Non-goals": `release.notes.output` is
never written to disk, rotation doesn't apply). Derived `TagPattern` only applies when the user
hasn't already set an explicit one, same precedence `withEnvDerivations` already uses.

**Files (expected):** new `internal/app/changelog_rotation.go` + test, `internal/app/pipeline.go`.
**Scope:** M. **Dependencies:** T244, T245, T246.

Landed as a `rotatingGenerator` (`internal/app/changelog_rotation.go`) implementing `port.Generator`,
wired into both `buildChangelogPipelineConfig` (`heraut changelog`) and `buildReleasePipelineConfig`
(`heraut release`) — the roadmap's "Files (expected)" only named the former, but `heraut release`
builds its own changelog generator independently and needed the identical wrap. `resolveDriver`
strips `versioning.tag_prefix` (reusing T222's existing `defaultTagPrefix` helper) then dispatches to
calver (`calver.ParseFormat`/`ParseVersion`/`BucketPattern`) or semver
(`semver.MajorMinor`/`RotationPattern`) to compute the concrete `Output` and — only when the user
hasn't already set an explicit `tag_pattern` — the derived `TagPattern`, before delegating to a
freshly-built `native.Generator` via the same `buildGenerator` constructor. Two small additions to
calver (`TokenKindFromName`, the inverse of T244's `TokenKind.String()`; `RenderToken`, an exported
single-token formatter) and one to semver (`RotationPattern`, mirroring `calver.BucketPattern`'s
contract for the MAJOR/MAJOR.MINOR case) were needed to let the decorator convert parsed `{TOKEN}`
names into concrete values and a tag-scoping regex — kept in their own versioning packages rather
than inlined in `internal/app`, for the same testability-at-the-right-layer reason T244/T245 exist as
separate packages at all.

**Real bug found and fixed along the way, not anticipated in the design doc:**
`pipeline.ChangelogConfig.ChangelogFile` / `pipeline.Config.ChangelogFile` are resolved once at
config-build time (before the tag is known) and consumed later by the "Commit changelog" step and
the dry-run/summary display in both `internal/pipeline/changelog.go` and `internal/pipeline/release.go`.
For a rotating `output`, this field would have stayed the literal, unsubstituted `"CHANGELOG_{YYYY}.md"`
string forever — `git add`-ing a file that was never written, while the real
`CHANGELOG_2026.md` sat uncommitted. Fixed with a new structural `outputPathReporter` interface
(`internal/pipeline/changelog_output.go`, `LastOutputPath() string`) that `rotatingGenerator`
implements; the commit/summary steps now call a shared `resolvedChangelogFile(gen, configured)`
helper that prefers the generator's own post-`Generate()` report when present, falling back to the
static field unchanged for every non-rotating config. `testutil.MockGenerator` gained a matching
`LastOutputPathV` field, mirroring the existing `DegradedVal`/`DegradedReasonV` optional-method
pattern exactly. One accepted, explicitly-scoped gap: dry-run's informational "`[dry-run] would
write …`" line still shows the raw pattern, since dry-run never calls `Generate()` and so has
nothing concrete to report — a cosmetic-only limitation (dry-run performs no writes either way),
not the correctness bug the commit-step fix addresses.

Tests: unit tests for `TokenKindFromName`/`RenderToken` (calver) and `RotationPattern` (semver);
`resolveDriver`-level tests in `internal/app` covering calver single/multi-token, semver
MAJOR/MAJOR+MINOR, tag-prefix stripping, explicit-`tag_pattern`-wins precedence, and both an invalid
manual-version-override error and an unsupported-strategy error; one full `Generate()` round-trip
test (`TestRotatingGenerator_Generate_WritesConcreteFile`) proving the concrete file is actually
written to disk and the literal `{YYYY}` pattern never is; two pipeline-level regression tests
(`TestChangelogRun_WithCommit_UsesRotatedOutputPath`, `TestRun_UsesRotatedChangelogOutputPath`)
reproducing the `ChangelogFile` bug before the fix and proving it after. `go test ./...` and
`hk check` both clean.

**Correction (T248).** A second real bug surfaced only once T248's real-git-repo test existed: the
new bucket's first release has no in-scope previous tag to bound its commit range by, so it silently
absorbed the *entire prior bucket's* history too. `internal/generators/native` gained one small
field (`ContentDriver.PreviousTagOverride`) to fix it — see T248's note and the design doc's §5
addendum for the full explanation. "Native needs zero changes" (this note's own framing above) was
not quite accurate; corrected there rather than rewritten here.

#### `[x]` T248: integration test — rotation across a simulated period boundary

Using the existing real-git-repo integration harness, drive a changelog run producing two tags in
different rotation buckets (e.g. two CalVer tags a year apart) and assert two separate output files
exist, each containing only its own bucket's entries — proving native's bootstrap-on-missing-file
path produces correct rotation with zero changes to its own algorithm (design doc §5).

**Files (expected):** an integration test file alongside the existing pipeline integration tests.
**Scope:** S. **Dependencies:** T247.

Landed as `internal/app/changelog_rotation_realrepo_internal_test.go` (package `app`, not `app_test`
— it calls the unexported `buildGenerator`/`wrapWithRotation` directly, following the same
`_internal_test.go` convention as T247's other tests rather than native's own external-package
`integration_test.go`, since there's no exported entry point at this layer for "build a rotation-
wrapped changelog generator"). Real git repo, real exec runner (`execadapter.New`), no MockRunner:
one commit + `Generate("2025.12.0", ...)` (before the tag exists, matching the real pipeline's
generate-then-tag sequencing) + `git tag 2025.12.0`, then a second commit +
`Generate("2026.01.0", ...)`.

**This is the test that found T247's `PreviousTagOverride` gap**, before any fix code was written:
the first real assertion attempt showed `CHANGELOG_2026.md` containing both "First feature" and
"Second feature" — the 2025 entry duplicated into the 2026 file. Traced to `buildAllSections`'s
`latest := tags[0]` (used identically by `generateIncremental`'s splice path): with the bucket-scoped
`tag_pattern` correctly excluding every 2025 tag, `scopedTags()` returns empty for the new bucket,
`latest` becomes `""`, and `commitRange("", "HEAD")` returns bare `"HEAD"` — full history, not
`"2025.12.0..HEAD"`. Fixed with `ContentDriver.PreviousTagOverride` (`internal/config/config.go`) and
a new `Generator.newSectionBound(scopedTags)` helper (`internal/generators/native/generator.go`)
that both `buildAllSections` and the splice path now call instead of inlining the same `tags[0]`
check; `internal/app`'s `resolveDriver` sets it via a new `latestMatchingTag` helper — one extra,
unscoped `git tag -l <prefix>*` query, the same incantation calver/semver's own resolvers already
run internally, needed because `rotatingGenerator` has no access to the version resolver's own
`Result.CurrentTag` (the `port.Generator.Generate(tag, lc)` interface never carries it through).

Verified non-vacuous the same way T242's recap test was: temporarily removed the
`PreviousTagOverride` computation, re-ran this test, watched it fail with the exact duplicated-entry
symptom described above, then restored the fix and confirmed green again. Added matching
narrower-scope regression coverage at the layer the bug actually lives in: two new MockRunner
contract tests in `internal/generators/native` (`TestGenerator_GenerateChangelog_
PreviousTagOverride_BoundsBootstrapRange` for the bootstrap path, `..._SpliceUsesOverride` for the
splice path) asserting the exact `git log` range argument, since MockRunner doesn't interpret git
ranges — a body-content assertion alone (my first draft of these) doesn't actually discriminate
between "HEAD" and "prevTag..HEAD" when there's only one commit in the fixture, which is why the
first version of this integration test needed the args-level assertion to genuinely fail before the
fix; also two new `internal/app` unit tests (`TestRotatingGenerator_ResolveDriver_
SetsPreviousTagOverride`, `..._NoPriorTags_EmptyOverride`) pinning `resolveDriver`'s own contribution
in isolation. Design doc (§5) and T247's own note above both carry a dated correction rather than
being silently rewritten. `go test ./...` and `hk check` both clean.

#### `[x]` T249: docs — schema, sample, spec, ADR-0047

Update `schema.json` (`changelog.output` description/pattern examples), `docs/heraut.sample.yml`
(a commented rotation example per strategy family), and wherever `changelog.output` is documented
in `docs/specs/` today. Write ADR-0047 ("Changelog output resolves after version, not at
config-build time") per the design doc's outline — the decorator pattern and why the two more
obvious-looking placements (native itself; config-build time) are both wrong.

**Files (expected):** `schema.json`, `docs/heraut.sample.yml`, `docs/specs/02-configuration.md` (or
wherever applicable), `docs/adr/0047-changelog-output-resolves-after-version.md`,
`docs/adr/README.md`. **Scope:** S. **Dependencies:** T247.

`schema.json`'s shared `ContentDriver.output` description now covers rotation-token syntax with a
note that it applies to `changelog.output` only (`release.notes.output` is never written to disk —
an existing, undocumented quirk this task didn't try to fully resolve, just didn't paper over).
`docs/heraut.sample.yml` gained four commented examples (calver yearly/monthly, semver
major/major+minor) right under the active `output: CHANGELOG.md` line, matching the file's existing
convention of showing the inactive strategy's fields commented out under whichever strategy is
active. `docs/specs/02-configuration.md` gained a new "Rotating changelog output" subsection
covering both token vocabularies, the CalVer prefix-order constraint, and the two explicit
non-goals (per-env strategies, the `init` wizard) — plus a table-row pointer from `output`'s own
description. `docs/specs/05-generators-and-platforms.md`'s existing bootstrap/splice algorithm
description gained a short cross-reference explaining that rotation reuses that path unchanged
except for the `PreviousTagOverride` bound (T248), rather than leaving a reader of that section to
wonder how the two interact.

Wrote ADR-0047 per the design doc's outline, folding in T248's `PreviousTagOverride` finding as its
own dedicated paragraph (per the design doc's own "Also document" note) rather than treating it as
a footnote — it's a real, if narrow, correction to the ADR's central "native needs no changes"
framing, and burying it would misrepresent what actually shipped. Bumped the "46 ADRs" count to 47
in the three places `CLAUDE.md`/`docs/tasks/roadmap.md` cited it. `go build ./...` and `hk check`
both clean (docs-only change set — no Go logic touched).

This closes the changelog-rotation epic (T244–T249). See Phase 29 in the main roadmap.
