# Héraut — SemVer v2 Roadmap

> Status: Done — epic complete (Phases 1, 1.5, 2 and 3 closed)
> Phase 3 design: [`docs/superpowers/specs/2026-10-05-maintenance-branches-design.md`](../superpowers/specs/2026-10-05-maintenance-branches-design.md); plan: `.claude/plans/semver-v2-phase-3-maintenance-branches.md`
> Design: [`docs/superpowers/specs/2026-09-28-semver-v2-compliance-design.md`](../superpowers/specs/2026-09-28-semver-v2-compliance-design.md)
> ADRs: ADR-0064 ("SemVer v2 compliance and pre-release lifecycle" — written in T324), ADR-0065 ("Branch-aware SemVer resolution" — T355)
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
| T335 | Per-env tags containing "/" break GitLab package-registry uploads | Done |
| T336 | `--set-version` validated as SemVer v2 under `semver`/`semver-per-env` | Done |
| T337 | GitHub `--prerelease` derived from the version; remove `release.targets[].prerelease` | Done |
| T338 | Pre-release series resolution in `semver.Resolver` | Done |
| T339 | `--pre-release <label>` on `release` and `version next` | Done |
| T340 | A pre-release run never writes `CHANGELOG.md` | Done |
| T341 | Release-notes range for pre-releases; absent-tag fallback fix | Done |
| T342 | End-to-end pre-release scenarios on a real repo | Done |
| T343 | ADR-0064 Phase 2 status update and Phase 2 close | Done |
| T344 | Maintenance-branch support (last final + notes from branch history) | Done — delivered as T349–T355 |
| T349 | `versioning.branches` config, range parsing, validation | Done |
| T350 | Current-branch detection, rule matching, unlisted-branch publish guard | Done |
| T351 | Maintenance resolution in `semver.Resolver` | Done |
| T352 | History-aware changelog and notes bounds | Done |
| T353 | `version current` and pre-releases on maintenance branches | Done |
| T354 | End-to-end maintenance scenarios on a real repo | Done |
| T355 | ADR-0065, guide, Phase 3 close | Done |

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

### [x] T335 — Per-env tags containing "/" break GitLab package-registry uploads

Pre-existing, unrelated to `+`: `glab release upload --use-package-registry` uses the tag as the
generic-package version, which GitLab rejects when it contains `/` (every `{env}/{version}`
per-env tag), so per-env GitLab asset uploads fail with `400 package_version is invalid`. Also
unverified: GitLab's own release link encodes the slash (`/-/releases/uat%2F0.0.0+smoke`) while
heraut renders `/-/releases/uat/0.0.0%2Bsmoke` — check on a public project whether heraut's
per-env GitLab release URLs resolve, and whether `/` should be escaped there. Needs its own
design (e.g. derive a package version without `/`, or drop `--use-package-registry` for per-env).

**Completion note:** the upload half turned out not to be reachable from heraut, so it shipped no
code change. `app.buildTargetPlatforms` sets `LenientAssets = len(Assets) > 0` for every target,
so every asset rides as a positional file on `glab release create` (glab project uploads, which
accept any tag) and `UploadAssets` — the only `--use-package-registry` call site, which T332
exercised by hand — returns before reaching glab. The failure only reproduces if a user sets
glab's own `GITLAB_RELEASE_ASSETS_USE_PACKAGE_REGISTRY` themselves. Spec 02's GitLab
"Implementation" line, which still claimed `glab release upload --use-package-registry`, was
corrected. The URL half was a real bug: `platforms/gitlab` gained `releaseTag(tag)`
(`port.URLTag` plus `/` → `%2F`), used by `ReleaseURL` and both `ReleaseURLFromContext` branches,
matching the link GitLab itself renders; GitHub release URLs and every compare URL are unchanged
(GitHub and GitLab's compare route accept `/` in refs). `port.URLTag` was left `+`-only on
purpose, since it also feeds compare URLs. `TestReleaseURL_EscapesPlusInTag`'s ambient-context
row asserted the unescaped `uat/7.4.1%2B158404` and was edited to `uat%2F7.4.1%2B158404` (a
deliberate behaviour change, not a deleted row); `TestReleaseURL_EscapesSlashInTag` covers all
three builders, including a nested group and a multi-`/` tag. Spec 05's GitLab "Release URL" line
documents the escaping. Not verified against a live public project; T345's per-env scenario is
the place to assert it. Filed during the design discussion: T346 (remove the unreachable
`UploadAssets` registry path) and T347 (design spike: GitLab publishing over native `net/http`
instead of `glab`), both in `roadmap.md` Phase 61.

## Phase 1.5 — Remaining compliance

**Phase 1.5 closed** (T336–T337). Two items the design doc's § Delivery filed under Phase 2 that
need none of its series/escalation machinery, pulled forward once Phase 1 closed rather than
waiting on the pre-release lifecycle.

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

### [x] T337 — GitHub `--prerelease` derived from the version; remove `release.targets[].prerelease`

Changed `port.Platform.CreateRelease` to `CreateRelease(tag, notes string, prerelease bool)
error`; `internal/pipeline/release.go` computes `prerelease := isPreRelease(result.Version)`
once per run (`semver.Parse` + `IsPreRelease`, false on parse failure — every CalVer version) and
passes it to every target's `CreateRelease` call. `platforms/github` now reads the parameter
instead of `cfg.Prerelease`; `platforms/gitlab` accepts and ignores it (GitLab has no pre-release
concept) — both updated in the same commit as the port change, per the layering rule. Removed
`Target.Prerelease` and `Platform.Prerelease` from `internal/config/config.go`,
`app.platformConfigFromTarget`'s copy, and every scaffold passthrough field (`wizard.go`'s
`PlatformAnswer.Prerelease`, `generate.go`'s `answersToConfig`, `dropped.go`'s
`DroppedPlatformFields` branch) — GitHub's pre-release flag is no longer a wizard-editable or
passthrough setting at all. `release.targets[].prerelease` is a removed config key: added
`targetPrereleaseProbe`/`firstPrereleaseIndex` to `internal/config/loader.go` since it is the
first removed key living inside a YAML *list* rather than a flat path — the error names the exact
list index (`release.targets[1].prerelease`, or `environments.<env>.release.targets[N].prerelease`)
rather than just the key name, since a config with several targets needs to know which one to
edit. `schema.json`'s `Target` definition drops `prerelease`, so `additionalProperties: false`
now rejects it structurally; `testdata/config/invalid/targets_prerelease_removed.yml` covers both
the schema rejection and the loader's `ErrRemovedConfigKey`. Test rows edited (not deleted, per
this file's TDD rule) because they asserted the now-removed field: `platforms/github`'s
`TestCreateRelease_Prerelease`/`TestCreateRelease_DraftAndPrerelease` (config no longer sets
`Prerelease`; the call now passes the bool as a parameter); `scaffold`'s
`TestDroppedFields_PlatformDraftAndPrerelease_NotDropped` (renamed to
`...PlatformDraft_NotDropped`), `TestConfigToAnswers_PreservesPlatformPassthroughFields`,
`TestGenerateYAML_PlatformPassthroughFieldsRoundTrip`, and `wizard_internal_test.go`'s
`TestMatchPlatformSnapshot_SingleMatch` (all drop the `Prerelease`/`prerelease` assertion, keeping
every other field's coverage intact). New coverage: `platforms/gitlab`'s
`TestCreateRelease_PrereleaseParamIgnored` (both bool values produce identical args);
`pipeline`'s `TestRun_Prerelease_DerivedFromVersion` (table: `2.0.0-rc.1` → true, `2.0.0` →
false, `2026.05.0` → false) against `MockPlatform.CreateReleaseCalls`, which gained a
`Prerelease` field. No deferred items.

**Final review fixes**: `isPreRelease`'s "CalVer fails semver.Parse" reasoning was wrong — a
CalVer `format: YYYY.MM.SS-PATCH` can resolve something like `2026.10.2-0`, which itself parses
as a SemVer pre-release — so the pre-release flag is now gated by a new
`pipeline.Config.SemVerStrategy` bool, set by the app layer from `versioning.strategy`
(`prerelease := cfg.SemVerStrategy && isPreRelease(result.Version)`), not inferred from the
version's shape alone; `checkRemovedKeys`'s `release.targets[]` probe was made tolerant of a
non-mapping entry (`targets: [gh]`) and of an explicit `prerelease:` null, both of which
previously made the whole removed-key probe abort silently; and Spec 03's `--set-version` text
was corrected to drop the stale `rel-` example T336 now rejects, plus the resolver now names the
configured `tag_prefix` in the parse-failure error when it isn't the `"v"` default.

## Phase 2 — Pre-release lifecycle

Scope per the design doc § Delivery → Phase 2: `--pre-release <label>`, series rules,
`--allow-major` second trigger, changelog skip and notes ranges. (`--set-version` SemVer
validation and the GitHub-derived `--prerelease` / `release.targets[].prerelease` removal moved to
Phase 1.5 — T336/T337 above.) Plan:
[`.claude/plans/semver-v2-phase-2-pre-release-lifecycle.md`](../../.claude/plans/semver-v2-phase-2-pre-release-lifecycle.md).

Decisions taken while planning (2026-10-04), resolving notes (a)–(c) below:

- **Previous-tag rule for a pre-release** ((b), (c)): the highest-precedence tag of any kind
  strictly below the version being cut **and reachable from HEAD** (`git tag --merged HEAD`). One
  rule drives both the commit requirement and the notes range. Finals keep T334's rule (last
  final, precedence only). Core and last-final computation stay global — branch-aware resolution
  is T344.
- **A pre-release run is keyed on the version**, not the flag: `--pre-release <label>`, or a
  SemVer-strategy `--set-version` value carrying pre-release identifiers (same gate as T337's
  GitHub flag) — so `heraut changelog --set-version X-pre --tag` tags without writing a section.
- **(a) is not handled**: no user has pre-release sections on disk; no code, no migration note.

### [x] T338 — Pre-release series resolution in `semver.Resolver`

`--pre-release` mode for the plain `semver` resolver: floating core from the last final, series
escalation (warning for minor/patch, `ErrMajorEscalation` for major unless `--allow-major`),
`<core>-<label>.<N>` counter, global per-core monotonicity (`ErrPreReleaseRegression`), commit
requirement since the previous tag reachable from HEAD. Spec 04 § Pre-release lifecycle.

**Completed.** `semver.Resolver.SetPreRelease` and `resolvePreRelease` (new `prerelease.go`)
implement the algorithm as briefed; `ValidatePreReleaseLabel`, `ErrMajorEscalation` and
`ErrPreReleaseRegression` are exported for T339. `holdMajorAtZero`'s helper became
`commitsAtLevel(commits, overrides, level)` plus a shared `writeSubjects`, with its output
unchanged. Decisions: the escalation warning lists commits at the new bump level and appends `""`
to `wouldBeVersions`; with no final yet the escalation check still runs but lists no commits. The
`ErrMajorEscalation` error text is prefixed by the sentinel (`%w: …`). `--set-version` and manual
mode still bypass pre-release mode; rejecting those flag combinations is T339.

### [x] T339 — `--pre-release <label>` on `release` and `version next`

`app.WithPreRelease` + usage errors (`--set-version`, non-`semver` strategy, manual bump mode,
invalid label); flag on both commands; `--allow-major` help names its second trigger. Spec 03.

**Completed.** `NewResolver` now applies its options first and runs `validatePreReleaseUsage` before
any other branch, so `--set-version` can never silently win over `--pre-release`; the option calls
`SetPreRelease` in the `semver` case only. `release` and `version next` declare the flag;
`PipelineOpts.PreReleaseLabel` is populated by `release` and consumed by T340. The two new
sentinels keep `wrapRunErr`'s Runtime code, pinned by a `version next` cmd test; usage errors are
Config (2). `changelog` keeps its original `--allow-major` help (it has no pre-release series).

### [x] T340 — A pre-release run never writes `CHANGELOG.md`

`app.isPreReleaseRun` decided at build time; reuses the `disable_changelog` skip path (step totals
stay correct); `heraut changelog --set-version X-pre` prints a pre-release message instead of
"changelog disabled". Specs 04/05 (drops the interim pre-release-changelog wording).

**Done.** `app.isPreReleaseRun` (label set, or a semver/semver-per-env `--set-version` that parses
as a pre-release, stripped like `NewResolver`) is computed in `BuildPipeline` /
`BuildChangelogPipeline` and passed to the two config builders as a `preRelease` bool, which set
`DisableChangelog`; `ChangelogConfig.PreRelease` selects the pre-release skip message, which wins
over "changelog disabled" when a per-env `disable_changelog` also applies. `heraut changelog` did
not forward `--set-version` into `PipelineOpts`; it now does, which the override detection needs.
Note (a) below is resolved as not handled: no user has pre-release sections on disk (decided
2026-10-04), so there is no migration note or code, and Specs 04/05 drop the interim wording.

### [x] T341 — Release-notes range for pre-releases; absent-tag fallback fix

native inserts the tag being released before ordering (fixes (b)); `native.WithReachableFromHead`
+ `app.notesTagOrderFor` (keeps pre-releases) for pre-release runs only. Spec 05.

`scopedTags` is split into `rawScopedTags` (glob/pattern only) plus the ordering; with a tag
order set, `scopedPreviousTag` adds the tag being released to the raw list when git does not have
it yet, then orders, so `v1.3.1-rc.1` with `v2.0.0` present resolves `v1.3.0`. `listTags` gained a
`mergedInto` parameter (`--merged HEAD` for `WithReachableFromHead`); `listMergedTags` is
untouched. `buildGenerator` takes trailing `extra ...native.Option`. Finals keep `tagOrderFor`
and no reachability filter, so their git calls are unchanged; calver gets no option because
`notesTagOrderFor` is nil. Notes (b) and (c) are resolved. Real-git tests cover rc.1 to rc.2,
rc to final, and a side-branch-only tag.

### [x] T342 — End-to-end pre-release scenarios on a real repo

`RealGitRepo` scenarios from the design doc § 7: beta → rc → final, escalation, blocked major +
`--allow-major`, `next` after `rc`, re-cut with no commit, side-branch tag, `version current`.

`internal/app/prerelease_realrepo_internal_test.go` runs the seven scenarios through
`app.NewResolver` with a real runner and annotated tags (reusing `notesRepo`). It exposed one bug:
a label promotion (`beta.2` then `--pre-release rc` with no new commit) failed the commit
requirement, contradicting the spec worked examples. `resolvePreRelease` now skips that check when
the previous reachable tag is a pre-release of the same core with a different label; re-cutting the
same label and opening a new series still require commits. The `switch to rc` unit row lost its
since-previous-tag `git log` call accordingly (edited with an ADR-0064 comment, not deleted).

### [x] T343 — ADR-0064 Phase 2 status update and Phase 2 close

ADR-0064 `## Status update (Phase 2)`, Phase 2 closing paragraph, flag lists in README/guides.

**Completed.** ADR-0064 gains `## Status update (Phase 2)` recording the delivered rules and
deviations: the pre-release previous-tag rule, the version-keyed "is a pre-release run", the
`disable_changelog` reuse, subjects instead of hashes in the escalation warning, the same-core
label-switch exemption from the commit requirement (explicitly amending § Decision, which is left
as the historical record), the absent-tag fix also covering finals' notes, and note (a) as
deliberately not handled. Two stale spec lines were corrected (Spec 04 "does not produce
pre-release tags yet"; Spec 03's "before any git call" wording, now "before any version-resolution
git call"). README and `docs/guides` list no release flags (`--allow-major` and
`--include-pre-release` appear nowhere), so no flag list needed `--pre-release`.

**Phase 2 closed** (T338-T343). Deviations from the plan are in the task notes above; the
epic stayed Active only for T335 and T344 at that point.

### Notes carried from Phase 1 / 1.5

(a) The changelog-skip item above must also resolve the interim pre-release-changelog state (see ADR-0064 §Decision,
Spec 04 § Pre-release tags, Spec 05 § Changelog structure): until it lands, a pre-release cut with
`--set-version X-pre --tag` still writes its own anchored `[X-pre]` section, which the next final's
incremental run re-lists and only `--regenerate` collapses away — and a pre-existing `[X-pre]`
section already in an upgraded repo's `CHANGELOG.md` survives incremental runs the same way.
Consider a one-time migration note (or an automatic `--regenerate` hint) for repos upgrading onto
Phase 2 with such sections already on disk. (Resolved: not handled — see Phase 2 decisions.)

(b) The notes-range work in scope above must also bound `previousInList`'s fallback for a
pre-release whose version is below the newest release — e.g. cutting `v1.3.1-rc.1` while `v2.0.0`
already exists currently falls through to `previousInList`'s "tag absent from the list → newest
existing tag is the predecessor" branch, producing a `v2.0.0..v1.3.1-rc.1` notes range (backwards
through history) instead of bounding by the previous tag actually below `v1.3.1-rc.1`.

(c) Pre-release notes need their own previous-tag rule. The design doc (§ 4) says a pre-release's
notes cover commits since the **previous tag, pre-release or final** (`rc.2`'s notes show only
what changed since `rc.1`). But T334's `tagOrderFor` — the order native uses for
`scopedPreviousTag` under SemVer strategies — is **releases-only** by design (pre-releases are
never a range boundary for a *final*). Left as is, `rc.2`'s notes would span back to the last
final. Phase 2 must keep the final's rule (last final, T334) and add a pre-release rule: the
highest-precedence tag of any kind below the pre-release being cut (`semver.SortTags` +
`semver.Latest(…, true)` already provide the ordering; the app layer knows the resolved version is
a pre-release). This also resolves (b).

(d) Building blocks already shipped and reusable: `semver.Parse`/`Compare`/`SortTags`/`Latest`
(T325); `version current --include-pre-release` (T330); `--set-version` accepts pre-release values
(T336); GitHub `--prerelease` derived from the version under SemVer strategies, carried to
`port.Platform.CreateRelease` via `pipeline.Config.SemVerStrategy` (T337); `native.WithTagOrder` +
`app.tagOrderFor` (T334). The resolver's "core" computation lives in
`internal/versioning/semver/resolver.go` (`resolveAuto`, `bumpAfterHold`, `stay_at_v0` hold in
`hold.go`); per-env is out of scope for minting (design doc § Non-goals).

## Phase 3 — Maintenance branches

Breaks T344 down (design: `docs/superpowers/specs/2026-10-05-maintenance-branches-design.md`;
plan: `.claude/plans/semver-v2-phase-3-maintenance-branches.md`, which carries each task's full
scope and tests). The real-data fixture cleanup recorded on T344 lands first as its own `test:`
commit.

### [x] T349 — `versioning.branches` config, range parsing, validation

Plan Task 1: `BranchRule`/`BranchRange`, `ParseBranchRange`/`DeriveBranchRange`, `validateBranches`,
`schema.json`, sample, fixtures, Spec 02.

Done. `internal/config/branches.go` adds `BranchRule`, `BranchRange` (`Minor == nil` means `N.x`),
`ParseBranchRange`, `DeriveBranchRange`, `IsGlob` and `String`, all pure; `Versioning.Branches` and
`validateBranches` (semver only, name required, glob compiles via `path.Match`, range parses,
explicit ranges unique) are wired into `Validate`. `schema.json` gains `BranchRule` (name
`minLength: 1`, range pattern); the sample and Spec 02 (new `versioning.branches` subsection) are
updated. Fixtures: `valid/semver_branches.yml` plus five `invalid/branches_*.yml`; the schema
rejects `bad_range` and `empty_name`, while `non_semver`, `duplicate_range` and `bad_glob` are
semantic-only and covered by the validator table. Deviation: the sample's pointer to the
maintenance-branch guide is generic because the guide itself lands in T355.

### [x] T350 — Current-branch detection, rule matching, unlisted-branch publish guard

`internal/app/branchrules.go` adds `CurrentBranch` (git, then `CI_COMMIT_BRANCH`,
`GITHUB_REF_NAME` only when `GITHUB_REF_TYPE` is `branch`, then `BUILD_SOURCEBRANCHNAME`),
`MatchBranchRule` (exact or `path.Match` glob; `/` is not crossed, so `release/*` does not match
`release/1.3/hotfix`; ambiguous matches and underivable globs are wrapped sentinel errors) and
`CheckReleaseBranch`, wired into `release` and `changelog --tag` right after `CheckBranch`, inside
the `!dryRun` block, exiting Config. Decisions: `--dry-run` is not refused, mirroring
`CheckBranch`; the guard makes no git call when `versioning.branches` is absent or `--force` is
set; an unknown branch (detached, no CI variable) is refused with a message naming the detached
HEAD. Spec 03 documents the extra `--force` meaning on `release` and `changelog`. Nothing
deferred; `NewResolver` wiring of `MatchBranchRule` is T351.

### [x] T351 — Maintenance resolution in `semver.Resolver`

Plan Task 3: `semver.Range`, `SetMaintenanceRange`, reachable in-range base, range cap
(`ErrOutOfRange`), collision guard (`ErrTagExists`), `NewResolver` wiring, Spec 04.

Done. `internal/versioning/semver/maintenance.go` adds `Range` (`[Lo, Hi)`, `Label`, `Branch`),
`RangeFrom`, `Contains` (on the core, so pre-release and build metadata are ignored),
`SetMaintenanceRange` and the sentinels `ErrOutOfRange`, `ErrTagExists`, `ErrNoInRangeRelease`.
With a range set, `resolveAuto` hands off to `resolveMaintenance`: `git tag -l <prefix>* --merged
HEAD --sort=-version:refname`, base = highest in-range final, `git log <base>..HEAD`, the bump
with the `stay_at_v0` hold applied first (its warnings still flow through `Warnings()`), the
range check (the error names the first commit subject at the applied bump level, falling back to
the first commit when a hold leaves none at that level), then the `git tag -l <next>` collision
probe. A nil range leaves `resolveAuto` untouched (guard test). `app.NewResolver` wires it for
`semver` only, after the `--set-version` early return, and skips branch detection entirely when
`versioning.branches` is absent or `bump.mode` is `manual`, so every existing `MockRunner` FIFO
test passes unmodified. Clarification: the spec's "collision guard still applies" under
`--set-version` relies on the existing `git tag` failure when the tag exists, since the override
path returns a `StaticResolver` and makes no git call; no new probe was added there. Spec 04 gains
§ Maintenance branches with the rules, the worked examples and the error table. Deviations: the
Spec 04 worked-examples table omits rows #9 (`version current`) and #10 (`--pre-release`), and
row #1's notes bound, which belong to T353 and T352. Deferred: `resolvePreRelease` and
`version current` ignore the range until T353.

### [x] T352 — History-aware changelog and notes bounds

Plan Task 4: reachable-only semver generators, scope-preserving per-section ancestry bound,
reachable rotation lookup, Spec 05.

Done. `app.buildGenerator` appends `native.WithReachableFromHead()` whenever a tag order is set,
so changelog, release notes, the changelog-only pipeline and the rotating generator all list tags
with `--merged HEAD`; the pre-release notes branch no longer appends it itself (it keeps
`notesTagOrderFor`). `latestMatchingTag` adds `--merged HEAD` under an order; its calver argv is
unchanged. `buildAllSections`, with an order, lists each section's ancestors once (`git tag -l
--merged <t> --no-contains <t>`) and bounds the section by the first later entry of the ordered,
scoped list found in that set. Clarification recorded here: the rule is scope-preserving, as the
spec's "first entry of `tagOrder(listMergedTags(t))`" would let `uat/1.3.0` bound `prod/1.3.0`
under `semver-per-env`; only when no in-scope ancestor exists does the T257/T334 oldest-in-scope
fallback apply, reusing the same listing (scoped: first of the ordered pool; unscoped: start of
history). Without an order the loop is byte-identical. Edited in place with an ADR-0065 comment:
`TestGenerator_GenerateChangelog_TagOrder_FiltersAndReorders` (two ancestor-listing responses
queued, log-call indexes 2/3 → 3/5). New: three `TestGenerator_BuildAllSections_*` unit tests,
`TestLatestMatchingTag_Argv`, `TestBuildGenerator_TagOrder_ListsReachableTagsOnly`, and
`historybounds_realrepo_internal_test.go` (unmerged maintenance tag, merged forward, per-env
scope). Deviation: the per-env real-git scenario passes on the pre-change code too (the old walk
already bounded by the scoped list); it was shown RED against the unscoped ancestry rule instead.
Spec 05 gains § History-aware bounds. ADR-0065 itself is T355's.

### [x] T353 — `version current` and pre-releases on maintenance branches

Plan Task 5.

Done. With a range set, `resolvePreRelease` keeps step 1's global listing (counter and per-core
monotonicity stay global, ADR-0064), then issues `git tag -l <prefix>* --merged HEAD
--sort=-version:refname` once, before `git log <L>..HEAD`; `L` is the highest in-range final
(none → `ErrNoInRangeRelease`), the bumped core must lie in the range (`ErrOutOfRange`), and the
parsed listing is reused for step 6's previous tag instead of a second identical call. Task 3's
base search and out-of-range message moved into `inRangeBase` / `outOfRangeError`, shared by both
paths. Without a range the call sequence is unchanged (the existing Phase 2 table pins it).
`app.CurrentTag`, for `semver` with a `branches` block, detects and matches the branch
(needRange) and on a maintenance branch returns the highest reachable tag whose core is in range
(final, or any kind with `--include-pre-release`); a miss wraps `errNoTagsFound` naming the range,
so `commit check --from-latest-tag` still falls back to full history there. Release/unlisted
branches and repos without the block take today's path (no `rev-parse` without the block).
Ruling recorded: `--pre-release` on a glob-matched branch with no derivable range
(`release/legacy`) fails with `ErrUnderivableRange` at `NewResolver`, since pre-release is auto
resolution; only `--set-version` is exempt (app-level row pins it); `version current` errors the
same way. Deviation: escalation now compares only the line's own open series (pre-releases whose
core is in range), because a higher series on `main` (`v2.1.0-rc.1`) would otherwise mask a real
escalation on the line (`v1.4.1-rc.1` → `v1.5.0`); without a range it is unchanged. Spec 04 gains
the `version current` and pre-release paragraphs plus worked-example rows #9/#10 (replayed on real
git in `maintenance_realrepo_internal_test.go`); Spec 03 notes `version current`'s branch
awareness. No existing test row was changed. Deferred: the full #1–#11 real-git replay is T354.
Review ruling (fix round 1): `commit check --from-latest-tag` must not fail on branches the rules
cannot place, so `ResolveFromLatestTag` falls back to the global latest tag (`globalCurrentTag`,
the no-`branches` path, shared with `CurrentTag`) when `CurrentTag` returns
`ErrUnderivableRange` or `ErrAmbiguousBranch`; `version current` keeps erroring there, and every
other error still propagates. Spec 03 notes it.

### [x] T354 — End-to-end maintenance scenarios on a real repo

Plan Task 6: the spec's worked examples #1–#11 on real git.

Done in `internal/app/maintenance_realrepo_internal_test.go`: rows #1–#8 and #11/#11b plus the
`--regenerate` history-bounds check run through `NewResolver`, `CheckReleaseBranch`, `buildGenerator`
and `tagOrderFor` with the real runner; #9/#10 were already covered there by T353. Release notes for
#1 assert the one-commit range (the notes body carries no compare link). Regression proof against
the tree at `5073768` (before T351), in a throwaway worktree with the new error sentinels stubbed:
#1 gave `v2.0.1` instead of `v1.3.2`, #9 gave `v2.0.0` instead of `v1.3.1`, and the regenerated
changelog on `release/1.3` listed `v2.0.0` and `v1.4.0` sections; #5 and #8 (unchanged behaviour)
passed there. No production code changed.

### [x] T355 — ADR-0065, guide, Phase 3 close

Plan Task 7.

Done. ADR-0065 ("Branch-aware SemVer resolution", `docs/adr/0065-branch-aware-semver-resolution.md`)
records the model and entry-type table, prior art (semantic-release, GitVersion), the branch-type
resolution table, why hard errors, the history-aware bounds and their one reach into configs
without `branches`, the `--set-version` rule, the rejected alternatives, and the implementation
clarifications: the `--set-version` collision relying on the `git tag` failure, `--dry-run` not
refused on unlisted branches, scope-preserving per-section bounds, the Config/Runtime exit split,
`ErrUnderivableRange` under `--pre-release`, escalation limited to the line's own series, the
`commit check --from-latest-tag` fallback, and forward-merged fixes appearing in a later `main`
section. ADR-0064's Phase 2 status update and Spec 04 link to it; `docs/guides/maintenance-branches.md`
(indexed in `docs/guides/README.md`) is the task-oriented how-to, including the fixed-version
`--set-version` case (T348); `CLAUDE.md`'s ADR count is 64. The CalVer equivalent of the history-aware
bounds is filed as T356 (Phase 63 in `roadmap.md`).

**Phase 3 closed** (T349-T355). Deviations are in the task notes above. T344 is flipped.

## Later

### [x] T344 — Maintenance-branch support: last final and notes from branch history

Done. Delivered as T349–T355 (Phase 3 above); the decision is recorded in
[ADR-0065](../adr/0065-branch-aware-semver-resolution.md). The questions below were answered there:
core, finals' notes and changelog bounds all become history-aware (the core only on declared
maintenance branches), and per-core monotonicity stays global.

Needs its own design. heraut resolves against the whole repo's tag list, not the current branch's
history: the semver resolver's `git tag -l <prefix>*` is global, so cutting `v1.3.2` on a
maintenance branch while `v1.4.0` exists on `main` computes its core from `v1.4.0`; a final's
release-notes predecessor (T334) is chosen by precedence only, so it can be a tag that never was
on the release's branch; the changelog walk has the same shape. git can't say which branch a tag
was cut on, but `git tag --merged <ref>` restricts to tags in a ref's history — Phase 2's
pre-release previous-tag rule (T338/T341) already uses it and is the model. Open questions for the
design: which surfaces become history-aware (core, finals' notes, changelog bounds), and how
global per-core monotonicity (ADR-0064) interacts with parallel maintenance lines.

**Also in scope (user decision, 2026-10-04):** replace the real host/project fixtures
(`git.adaouat.dev/bchatard/ecom-poc-release`) in `internal/pipeline/release_test.go` and
`internal/platforms/gitlab/platform_test.go` with synthetic placeholders (e.g.
`git.example.com/acme/widget`), per the no-real-data rule. Assertions stay identical apart from
the substituted strings; commit it separately as `test:`.
