# Héraut — SemVer v2 Roadmap

> Status: Active
> Design: [`docs/superpowers/specs/2026-09-28-semver-v2-compliance-design.md`](../superpowers/specs/2026-09-28-semver-v2-compliance-design.md)
> ADRs: new ADR-0064 ("SemVer v2 compliance and pre-release lifecycle" — written in T324)
> Main roadmap: tracked as Phase 59 in [`roadmap.md`](roadmap.md)

heraut implements the bare `MAJOR.MINOR.PATCH` subset of SemVer v2 only. This epic makes it
strictly compliant (full grammar, §11 precedence, `+`-only build metadata) and then adds a
pre-release lifecycle (`--pre-release <label>`) for the plain `semver` strategy.

## Conventions

- Task IDs **continue the global sequence** (`T324+`).
- This file is the **single source of truth** for task status: `[ ]` not started, `[x]` done.
  Two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD), then flip
  `[ ]` → `[x]` and add a one-paragraph completion note.
- **No real data** anywhere: synthetic placeholders only.
- Two deliberate clean breaks (ADR-0064): `{build}` must directly follow `+` in `tag_format`;
  `release.targets[].prerelease` is removed (Phase 1.5, T337).
- The main `roadmap.md` Phase 59 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Task | Description | Status |
|------|-------------|--------|
| T324 | Roadmap, Phase 59 pointer, ADR-0064 | Done |
| T325 | `semver.Version`: strict Parse, §11 Compare, SortTags/Latest | Done |
| T326 | Plain `semver` resolver orders tags by §11 in Go | Done |
| T327 | `semver-per-env` ordering + E002 via §11; calver-per-env unchanged (zero-padded CalVer is not SemVer) | Done |
| T328 | `{build}` must directly follow `+` (validator, wizard, docs) | Done |
| T329 | `--set-build-id` on plain `semver` (`v1.4.0+<id>`) | Done |
| T330 | `version current`: latest final by default, `--include-pre-release` (+ `${version}` test-typo fix, own commit) | Done |
| T331 | Escape `+` in tag names inside generated URLs | Done |
| T332 | Manual smoke test: gh/glab with a `+` tag | Done |
| T333 | Validate `--set-build-id` against SemVer build-identifier grammar | Done |
| T334 | Changelog, rotation and compare links bound by SemVer precedence | Done |
| T335 | Per-env tags containing "/" break GitLab package-registry uploads | Not started |
| T336 | `--set-version` validated as SemVer v2 under `semver`/`semver-per-env` | Done |
| T337 | GitHub `--prerelease` derived from the version; remove `release.targets[].prerelease` | Not started |
| —    | Phase 2 (pre-release lifecycle) — planned after Phase 1.5 lands | Not planned |

## Phase 1 — Compliance

**Phase 1 closed** (T324–T330). Deviations from the design doc: `git tag`'s
`--sort=-version:refname` flag is kept in every call as a harmless pre-sort, not the
source of truth — §11 ordering is decided in Go by `semver.SortTags`/`Latest`;
`IsBareVersion` is kept for `calver-per-env`'s zero-padded dotted-integer comparisons,
since strict SemVer parsing rejects CalVer's leading zeros; and a tag carrying only
build metadata (`v1.4.0+158404`, no pre-release identifiers) counts as a release, per
§11 (build metadata does not affect precedence). T331 (escaping `+` in tag names inside
generated URLs) was added mid-phase, outside the original T324–T330 sequence, once the
build-metadata work in T328/T329 made `+`-bearing tags routine. T334 (binding changelog section
bounds, rotation, and compare links to SemVer precedence instead of git's tag order) has since
closed too, tracked in its own entry below. T332 (manual `gh`/`glab` smoke test with a `+` tag)
and T333 (validating `--set-build-id` against the SemVer build-identifier grammar) have since
closed too, both tracked in their own entries below. T332 surfaced a follow-up of its own
(T335, GitLab package-registry uploads under per-env tags containing `/`), tracked below.

### [x] T324 — Roadmap, Phase 59 pointer, ADR-0064

Filed this dedicated roadmap, the Phase 59 pointer block in `docs/tasks/roadmap.md` (status
table row + navigable index block, no checkboxes there), and
[ADR-0064](../adr/0064-semver-v2-compliance.md) recording the compliance decisions and the two
clean breaks. Phase 2 (pre-release lifecycle) is intentionally left unbroken-down in this file —
its scope is sketched under "Phase 2" below and will be decomposed into its own tasks once Phase
1 lands, per the design doc's delivery plan.

### [x] T325 — `semver.Version`: strict Parse, §11 Compare, SortTags/Latest

Added `internal/versioning/semver/version.go` with `Version`, `Parse` (strict SemVer 2.0.0
grammar per §9/§10, rejecting leading zeros in both the core and numeric pre-release
identifiers), `String`/`Core`/`IsPreRelease`, `Compare` (§11 precedence, build metadata
ignored), and `SortTags`/`Latest` for scheme-agnostic tag selection. Numeric pre-release
identifiers compare by digit-string length then lexically, so arbitrarily long identifiers
(e.g. `rc.99999999999999999999`) never overflow `uint64`, while a `MAJOR`/`MINOR`/`PATCH`
segment that large is a parse error per the fixed-width core grammar. `hk fix -S
golangci_lint` auto-applied a De Morgan's-law rewrite (QF1001/staticcheck) to the character
class check in `splitIdentifiers`; no other lint findings. Nothing is wired into the
resolvers yet — that starts at T326.

### [x] T326 — Plain `semver` resolver orders tags by §11 in Go

`resolveAuto` now parses `git tag -l`'s output through `SortTags`/`Latest` (Task 2) instead of
scanning for the first `IsBareVersion` match — the current tag is the highest SemVer §11 release,
decided in Go, with the git `--sort=-version:refname` flag kept as a harmless pre-sort (contract
tests still assert the exact `git tag` args). A build-metadata-only tag (`v1.4.0+158404`) is now
correctly treated as the release of its core rather than skipped, and tags that are not valid
SemVer (`v1.02.0`, `v2.0.0.1`) are ignored like pre-releases instead of being accepted by the old
lenient `IsBareVersion` check. `IsBareVersion`'s doc comment was updated to note it's now only
used by the CalVer-facing paths (`internal/versioning/perenv`'s zero-padded CalVer comparisons),
not by this resolver. No existing test row's expectation needed to change — the full suite
(`mise run test`) was green on the first run after implementation, meaning nothing outside this
package relied on git's `version:refname` order diverging from §11 order.

### [x] T327 — `semver-per-env` ordering + E002 via §11; calver-per-env unchanged

- CalVer versions like `2026.05.0` have leading zeros, which strict SemVer parsing rejects:
  `calver-per-env` shares `internal/versioning/perenv` and must keep its lenient
  dotted-integer path (`IsBareVersion`, `compareVersionStrings`) — guarded by
  `TestResolve_Auto_Calver_ZeroPaddedUnaffected` plus the existing calver rows, unmodified.

New `internal/versioning/perenv/order.go` centralizes the strategy split: `releaseTags` (source/auto
selection), `latestTag` (destination selection for E002), and `compareVersions` (the E002
inequality itself) each branch on `cfg.Versioning.Strategy` — `semver-per-env` delegates to
`semver.SortTags`/`semver.Latest`/`semver.Compare` (§11 precedence, in Go), `calver-per-env` falls
through to the pre-existing `tagfmt.ParseVersion` + `semver.IsBareVersion` + `compareVersionStrings`
path, byte-for-byte. `auto.go` and `promote.go` now call these three functions instead of inlining
the tag-scan loops; git call arguments (`git tag -l <glob> --sort=-version:refname`) are untouched,
so the `--sort` flag is still a harmless pre-sort, not the source of truth. Fixed a real bug this
uncovered: the old `compareVersionStrings("1.2.4+7", …)` parsed `"4+7"` as the int `0` via
`strconv.Atoi`'s error-swallowed zero-value, so a destination tag carrying build metadata could
sail past the E002 guard undetected (new test
`TestResolve_Promote_Semver_E002_BuildMetadataDestination`). All 43 package tests pass, including
every pre-existing row unmodified (`TestResolve_Auto_Semver_SkipsPrereleaseTag`,
`TestResolve_Promote_SkipsPrereleaseSourceTag`, both `promoteBackends` calver rows,
`TestResolve_Promote_E002_NoForce`'s calver subtest). Spec 04 § Pre-release tags rewritten to
describe the SemVer §11 behaviour and its calver-per-env carve-out; no deferred items.

### [x] T328 — `{build}` must directly follow `+`

Added `buildTokenMisplaced` next to `tagFormatMissingVersion` in `internal/config/validator.go`,
scanning every `{build}` occurrence in a `tag_format` string and rejecting any not immediately
preceded by `+` (a leading `{build}` or a second, misplaced occurrence both fail the same way).
Wired into `Validate` once for `versioning.tag_format` (applies to every strategy, since
`--set-build-id` renders it regardless of strategy) and once per environment inside
`validatePerEnv`, plus into `ValidateTagFormatForWizard` so `heraut init`'s live field validation
catches it before the config is ever written. Converted every pre-existing `-{build}` test row
across `internal/cmd`, `internal/app`, `internal/config/tagformat_test.go`,
`internal/versioning/perenv/resolver_test.go` and `internal/versioning/tagfmt/tagfmt_test.go` to
`+{build}` (format strings, tag literals, and FakeBin glob cases alike), preserving genuine
SemVer pre-release hyphens (`7.4.1-rc.1`) untouched. `docs/specs/02-04`, `docs/guides/
mobile-ci-tagging.md`, `docs/guides/README.md` and both `schema.json` `tag_format` descriptions
now show `+{build}` exclusively; `docs/adr/0064-*` and the archived roadmap history keep the old
`-{build}` form on purpose, as the record of what used to be documented. No deferred items — the
new fixture `testdata/config/invalid/build_token_hyphen.yml` and the schema
semantic-only-fixtures list cover the rule at the schema boundary (schema itself can't express
the constraint; `config.Validate` owns it).

### [x] T329 — `--set-build-id` on plain `semver`

Plain semver has no `tag_format` to carry `{build}`, so `--set-build-id` used to fail with
"`--set-build-id requires versioning.tag_format to contain a {build} token`". `NewResolver`
now special-cases `cfg.Versioning.Strategy == "semver"` with an empty `EffectiveTagFormat`:
it renders `<prefix><version>+<id>` (e.g. `v1.4.0+158404`) directly, still requiring
`--set-version` like the per-env strategies. Extracted `configuredTagPrefix(cfg
*config.Config) string` (versioning.tag_prefix when set, else the strategy default) out of the
pre-existing `else` branch so both paths share the same prefix logic; Task 7 reuses this helper
verbatim. `TestNewResolver_BuildID_NoTagFormat` moved from `semverCfg()` to `calverCfg()` (edited,
not deleted) since CalVer still has no build-metadata fallback and the "tag_format" error path
still needs coverage. No deferred items.

**Follow-up (whole-branch review, FIX-1):** this task's original implementation composed
`<prefix><version>+<id>` without validating that the result actually parses as SemVer, so
`--set-version 1.4.0 --set-build-id build_1` and `--set-version 1.4 --set-build-id 5` both
produced tags heraut's own resolver could never read back. Fixed by running `semver.Parse` on
`<version>+<id>` before constructing the tag and rejecting with a wrapped error on failure.

### [x] T330 — `version current`: latest final by default, `--include-pre-release`

Fixed the pre-existing `${version}` typo in `TestCurrentTag_SemverPerEnv`'s `TagFormat`
fixture (`prod/{version}`) in its own `test(app):` commit first — the row could never
have parsed a tag, and only passed because `CurrentTag` returned git's first output line
unparsed. `CurrentTag`/`CurrentVersion` both gained an `includePreRelease bool`
parameter: for `semver`/`semver-per-env` the tag list is now run through
`semver.SortTags`/`semver.Latest` (Task 2) using a `semverExtractor` that strips
`tag_prefix` for plain semver or parses through the effective `tag_format` for
semver-per-env; `calver`/`calver-per-env` fall through to the pre-existing
first-line-of-git's-sort behaviour and ignore the flag entirely. When every candidate
tag is a pre-release, the "no tags found" error now names `--include-pre-release` as the
way to see them. `commit check --from-latest-tag` (`ResolveFromLatestTag` in
`internal/app/commit_check.go`) passes `true` unconditionally, keeping its
"latest tag of any kind" semantics now that "latest" is correctly §11-ordered instead of
git's `version:refname` order. `heraut version current` gained
`--include-pre-release`, wired straight through to whichever of `CurrentTag`/
`CurrentVersion` `--bare` selects. No deferred items.

### [x] T331 — Escape `+` in tag names inside generated URLs

Added `port.URLTag` (`strings.ReplaceAll(tag, "+", "%2B")`) in `internal/port/generator.go`,
next to `LinkContext`, since `port` is the one package every URL builder (platforms, forge,
generators) may import without violating layering. Wired it into the five sites the brief named
— `platforms/{github,gitlab}.ReleaseURL`/`ReleaseURLFromContext`, `forge/{github,gitlab}.
CompareURL`, and native's `buildCompareURL` (all three platform branches) — plus one the brief's
file list missed: `forge/azure.CompareURL`, which builds the same
`branchCompare?baseVersion=GT<tag>&targetVersion=GT<tag>` query string as native's azure_devops
branch but had no escaping and, on inspection, no pre-existing test at all (`port.Forge`'s doc
comment confirms `CompareURL` is a "reserved" link builder not yet called from production
rendering — T168). Added a standalone `TestCompareURL_EscapesPlusInTag` there since there was no
baseline assertion to extend. Every pre-existing URL assertion in the touched test files passed
unchanged. Deferred: T332, a manual `gh`/`glab` smoke test with a live `+` tag, since CLI-to-API
encoding can't be verified offline.

### [x] T332 — Manual smoke test: `gh` / `glab` with a `+` tag

Run by the controller on 2026-09-30 against two private sandboxes,
`github.com/bchatard/heraut-testing` and `gitlab.com/bchatard/heraut-testing`. Pushing `+` tags
works on both forges. `gh release create`/`upload` (heraut's exact argv) succeeded for both
`v0.0.0+smoke` and `uat/0.0.0+smoke`; GitHub's own `html_url` came back as
`…/releases/tag/v0.0.0%2Bsmoke`, identical to `port.URLTag`'s output. Both GitHub's and GitLab's
tag-lookup APIs accept the tag as either raw `+` or `%2B`. `glab release create` worked for every
tag shape tried. `glab release upload --use-package-registry` worked for `v0.0.0+smoke` and
`v0.0.2-rc.1+smoke`, but failed with `400 package_version is invalid` for `uat/0.0.0+smoke` — and
for the control `uat/0.0.1` (no `+` at all) — so the cause is the `/` in the per-env tag shape, not
`+`; filed as T335 below. Web release pages could not be probed since both sandboxes are private.
All releases, tags and packages created during the test were deleted afterwards. No code change
resulted from this task — it only confirms T328/T329's `+`-tag encoding is safe to ship.

### [x] T333 — Validate --set-build-id against SemVer build-identifier grammar

Two checks, both scoped to `semver-per-env` only (plain `semver` was already covered by FIX-1
under T329): `validateSemVerBuildID` parses `"0.0.0+" + buildID` to catch a malformed build ID on
its own (e.g. `build_1` — `_` is not `[0-9A-Za-z-]`), and `validateSemVerComposition` parses
`"<version>+" + buildID` — refactored out of T329/FIX-1's plain-semver branch verbatim, same error
text, same tests — to also catch a malformed `--set-version` (e.g. `7.4`) that the build-ID-only
check can't see on its own. Both run inside `NewResolver`'s existing `tf != ""` tag-rendering
branch, gated on `cfg.Versioning.Strategy == "semver-per-env"`, so `calver-per-env` keeps
`tagfmt.ValidateBuildID`'s lenient "/"-and-whitespace-only check unchanged (a build ID like
`build_1` still round-trips there). Breaking change for `semver-per-env`: a build ID that used to
render (e.g. containing `_`) is now a config error naming `--set-build-id`. No deferred items.

**Final review fixes:** FIX-1 widened the `semver-per-env`-only gate above to also cover plain
`semver` with a top-level `tag_format` carrying `{build}` — that combination skipped
`validateSemVerBuildID`/`validateSemVerComposition` entirely, so `--set-version 1.4
--set-build-id build_1` rendered `v1.4+build_1` at exit 0, a tag the resolver can never read back.

### [x] T334 — Changelog, rotation and compare links bound by SemVer precedence

Added `native.WithTagOrder(order func(tags []string) []string) Option`: `order` receives the
already TagGlob/TagPattern-scoped tag list and returns the tags to walk, newest-first, and may
drop tags — nil (the default) keeps today's behaviour unchanged. `scopedTags` applies it last
(after glob/pattern filtering, driving the historical walk, `newSectionBound`, and compare links);
`scopedPreviousTag` resolves via `previousInList` against that same ordered list instead of `git
describe` whenever an order is set; and `buildAllSections`'s oldest-in-scope fallback (T257's
"regardless of scope" resolution) swaps its `git describe` call for the highest-precedence entry
of `order(listMergedTags(runner, t+"^"))` — ancestor tags only (`git tag -l --merged <ref>`, a new
helper next to `listTags`), not an unscoped listing (see review-round-1 fix below).
`internal/app`'s new `tagOrderFor(cfg, env)` (next to
`semverExtractor` in `current.go`) builds the order for `semver`/`semver-per-env` —
`semver.SortTags` plus a pre-release drop — and returns `nil` for `calver`/`calver-per-env`, so
their output stays byte-for-byte unchanged (no order ever injected). `buildGenerator` gained a
`tagOrder` parameter threaded through all three `internal/app/pipeline.go` call sites (changelog,
release notes, and the changelog-only pipeline) plus `wrapWithRotation`/`rotatingGenerator`
(`changelog_rotation.go`); `latestMatchingTag` gained the same parameter and returns
`tagOrder(list)`'s first entry when set (semver) instead of trusting git's sort (calver
unaffected). Compare links needed no separate change — they already render from whatever
`prev`/`version` the caller resolves, so fixing `scopedTags`/`scopedPreviousTag` fixes them too.
A pre-release tag therefore gets no `CHANGELOG.md` section of its own and is never a range
boundary under a SemVer strategy (ADR-0064) — its commits fold into the next release's section.
Deviated from the roadmap's original fix sketch (`PreviousTagOverride = result.CurrentTag`):
that shape only bounds the *newest* section, not the historical walk or the oldest-in-scope
fallback, so `WithTagOrder` (an injected list-transform, matching the brief) covers all three
call sites uniformly instead. TDD: a real-git regression test
(`internal/app/tagorder_realrepo_internal_test.go`) reproduces the roadmap's own "Review Focus"
1 and 2 scenarios verbatim (a repo with `v1.3.0`, `v1.4.0-rc.1`, `v1.4.0+158404` and a new commit;
then a final `v1.5.0` cut after `v1.5.0-rc.1`), plus unit/contract coverage in
`internal/generators/native/generator_internal_test.go` (`WithTagOrder` filtering/reordering, the
no-`git-describe` release-notes path, the oldest-in-scope fallback, and a guard that no order set
reproduces today's exact call sequence) and `internal/app/current_internal_test.go`
(`tagOrderFor` for semver/semver-per-env/calver/calver-per-env) and
`internal/app/changelog_rotation_internal_test.go` (`latestMatchingTag`). Every pre-existing
calver and native test row passed unmodified — no existing row relied on git's tag order
diverging from §11 order. No deferred items.

**Review-round-1 fix:** the oldest-in-scope fallback above originally resolved `prev` via
`previousInList(t, order(listTags(runner, "")))` — an unscoped, §11-ordered pool with no ancestry
guarantee. Under `semver-per-env`, `tagfmt.ParseVersion`'s `{env}` token is a wildcard, so every
env's tags parse identically through `tagOrderFor`'s extractor; the fallback fires on every env's
*first* release (`withEnvDerivations` sets `TagGlob` per env), so a diverged env's higher- or
lower-precedence tag could sort adjacent to `t` and get picked as "previous" despite living on an
unrelated branch — `git log <that tag>..t` would then span a non-ancestor range, silently
excluding commits `t` actually needs (the same missing-entries bug class T334 exists to fix, on a
different topology). Fixed by adding `listMergedTags(runner, ref)` (`git tag -l --merged <ref>`,
next to `listTags` in `commits.go`) and a `noParentCommit(stderr)` probe (the `--merged`
counterpart to `noEarlierTag`'s `git describe` probe — a root commit's `<tag>^` fails to resolve
at all, with a different message, "malformed object name", rather than "no tag describes it").
The fallback now calls `listMergedTags(g.runner, t+"^")` — ancestor tags only — applies `tagOrder`,
and takes its first entry directly (not `previousInList`: `t` itself is never in the `--merged
<t^>` pool, so "first entry" already *is* the true previous tag). Edited
`TestGenerator_GenerateChangelog_TagOrder_OldestInScopeFallbackUsesOrderedUnscopedList` →
renamed `...IsAncestryBounded` (asserts the `--merged` call instead of the old unscoped one) since
its assertions described the now-fixed behaviour; added
`TestListMergedTags_ReturnsAncestorTags`/`_RootCommitReturnsEmpty`/`_OtherErrorPropagates`/
`_EmptyOutput` and a second real-git regression,
`TestTagOrderFor_RealRepo_FallbackNeverBoundsByNonAncestorTag` (two envs on diverging branches;
confirmed it fails against the pre-fix code before re-verifying green). No other existing row
changed.

**Final review fixes:** FIX-2 replaced `listMergedTags`'s `<t>^` + `noParentCommit(stderr)`
English-only probe ("malformed object name") with `git tag -l --merged <t> --no-contains <t>
--sort=-version:refname`, called with `t` directly — a localised git renders that stderr message
differently, which would have hard-errored the fallback on a root-commit tag instead of treating
it as "no ancestor tags." The new flag combination needs no stderr probe at all: a root-commit `t`
simply yields an empty list at exit 0, confirmed against a real git binary.

### [ ] T335 — Per-env tags containing "/" break GitLab package-registry uploads

Pre-existing, unrelated to `+`: `glab release upload --use-package-registry` uses the tag as the
generic-package version, which GitLab rejects when it contains `/` (every `{env}/{version}`
per-env tag), so per-env GitLab asset uploads fail with `400 package_version is invalid`. Also
unverified: GitLab's own release link encodes the slash (`/-/releases/uat%2F0.0.0+smoke`) while
heraut renders `/-/releases/uat/0.0.0%2Bsmoke` — check on a public project whether heraut's
per-env GitLab release URLs resolve, and whether `/` should be escaped there. Needs its own
design (e.g. derive a package version without `/`, or drop `--use-package-registry` for per-env).

## Phase 1.5 — Remaining compliance

Two items the design doc's § Delivery filed under Phase 2 that need none of its series/escalation
machinery, pulled forward once Phase 1 closed rather than waiting on the pre-release lifecycle.

### [x] T336 — `--set-version` validated as SemVer v2 under `semver`/`semver-per-env`

Added `validateSemVerStrategyOverride(strategy, version string) error` in
`internal/app/resolver.go`, called once in `NewResolver`'s override branch right after `tag`/
`version` are computed by either the `tag_format` path or the plain-prefix path, gated on
`buildID == ""` — the `buildID != ""` paths already run `validateSemVerComposition` (and, for
`semver-per-env`, `validateSemVerBuildID`) from T329/T333, which parse `version` as part of the
full `"<version>+<buildID>"` string, so re-running the new check there would be redundant, not
wrong, and the brief asked for one check covering the two paths those don't reach. The check is a
no-op for `calver`/`calver-per-env` (returns `nil` immediately), and otherwise rejects a value
containing `+` with a hint toward `--set-build-id` before ever calling `semver.Parse`, since build
metadata has exactly one entry point into a tag. Breaking change: `internal/cmd/release_test.go`'s
`TestRelease_VersionFlag_ValidFormats` asserted that a bare word, a truncated core (`v1`, `v1.2`,
bare `v`), and CalVer-shaped values (`2024.03`, `2024.03.15.2`) all passed through unexamined under
`semver` — those seven rows moved to a new `TestRelease_VersionFlag_RejectedUnderSemVer`, re-asserting
them as config errors naming `--set-version` (ADR-0064), per this file's TDD rule against deleting
assertions. `internal/cmd/version_override_test.go`'s "set-version already carrying build metadata,
no --set-build-id, still works" row (`v1.4.0+abc`) is the same deliberate change: replaced by
`TestVersionNext_SetVersion_BuildMetadataWithoutSetBuildID_IsRejected`, asserting the config error
instead. No deferred items.

### [ ] T337 — GitHub `--prerelease` derived from the version; remove `release.targets[].prerelease`

Not started. Scope per the design doc § 3–4: change the `port.Platform` contract to
`CreateRelease(tag, notes string, prerelease bool)` so the pipeline — which may import
`internal/versioning` — decides the flag from the resolved version, keeping platforms free of
versioning imports; update every `port.Platform` implementor (`platforms/github`,
`platforms/gitlab`) in the same commit; remove `release.targets[].prerelease` from
`internal/config`, `schema.json`, `docs/heraut.sample.yml` and `testdata/config/`, with a
migration hint via the loader's `removedKeys` mechanism.

## Phase 2 — Pre-release lifecycle

Not yet broken down. Scope per the design doc § Delivery → Phase 2: `--pre-release <label>`,
series rules, `--allow-major` second trigger, changelog skip and notes ranges. (`--set-version`
SemVer validation and the GitHub-derived `--prerelease` / `release.targets[].prerelease` removal
moved to Phase 1.5 — T336/T337 above.)

(a) The changelog-skip item above must also resolve DOC-1's interim state (see ADR-0064 §Decision,
Spec 04 § Pre-release tags, Spec 05 § Changelog structure): until it lands, a pre-release cut with
`--set-version X-pre --tag` still writes its own anchored `[X-pre]` section, which the next final's
incremental run re-lists and only `--regenerate` collapses away — and a pre-existing `[X-pre]`
section already in an upgraded repo's `CHANGELOG.md` survives incremental runs the same way.
Consider a one-time migration note (or an automatic `--regenerate` hint) for repos upgrading onto
Phase 2 with such sections already on disk.

(b) The notes-range work in scope above must also bound `previousInList`'s fallback for a
pre-release whose version is below the newest release — e.g. cutting `v1.3.1-rc.1` while `v2.0.0`
already exists currently falls through to `previousInList`'s "tag absent from the list → newest
existing tag is the predecessor" branch, producing a `v2.0.0..v1.3.1-rc.1` notes range (backwards
through history) instead of bounding by the previous tag actually below `v1.3.1-rc.1`.
