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
  `release.targets[].prerelease` is removed (Phase 2).
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
| T332 | Manual smoke test: gh/glab with a `+` tag | Not started |
| T333 | Validate `--set-build-id` against SemVer build-identifier grammar | Not started |
| —    | Phase 2 (pre-release lifecycle) — planned after Phase 1 lands | Not planned |

## Phase 1 — Compliance

**Phase 1 closed** (T324–T330). Deviations from the design doc: `git tag`'s
`--sort=-version:refname` flag is kept in every call as a harmless pre-sort, not the
source of truth — §11 ordering is decided in Go by `semver.SortTags`/`Latest`;
`IsBareVersion` is kept for `calver-per-env`'s zero-padded dotted-integer comparisons,
since strict SemVer parsing rejects CalVer's leading zeros; and a tag carrying only
build metadata (`v1.4.0+158404`, no pre-release identifiers) counts as a release, per
§11 (build metadata does not affect precedence). T331 (escaping `+` in tag names inside
generated URLs) was added mid-phase, outside the original T324–T330 sequence, once the
build-metadata work in T328/T329 made `+`-bearing tags routine. T332 (manual `gh`/`glab`
smoke test with a `+` tag) and T333 (validating `--set-build-id` against the SemVer
build-identifier grammar) remain open follow-ups, tracked below.

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

### [ ] T332 — Manual smoke test: `gh` / `glab` with a `+` tag

heraut passes tag names to `gh release create/upload` and `glab release create/upload` as argv;
how those CLIs encode `+` when they call their APIs cannot be checked offline (testing.md: no
network in tests). Before the first release that ships T328/T329, create a throwaway release with
a `v0.0.0+smoke` tag on a scratch GitHub repo and a scratch GitLab project (create + upload an
asset + open the printed release URL), then delete both. Record the outcome here.

### [ ] T333 — Validate --set-build-id against SemVer build-identifier grammar

Since `{build}` always follows `+` (ADR-0064), the ID is SemVer build metadata and should match
dot-separated `[0-9A-Za-z-]+` identifiers; `tagfmt.ValidateBuildID` only rejects "/" and
whitespace today; tightening it would reject IDs currently accepted (e.g. with "_"), so it needs
its own decision.

## Phase 2 — Pre-release lifecycle

Not yet broken down. Scope per the design doc § Delivery → Phase 2: `--pre-release <label>`,
series rules, `--allow-major` second trigger, `--set-version` SemVer validation, changelog skip and
notes ranges, GitHub-derived `--prerelease`, removal of `release.targets[].prerelease`.
