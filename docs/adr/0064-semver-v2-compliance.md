# ADR-0064: SemVer v2 compliance and pre-release lifecycle

- **Status**: Accepted
- **Date**: 2026-09-28
- **Deciders**: bchatard
- **Design doc**: [`docs/superpowers/specs/2026-09-28-semver-v2-compliance-design.md`](../superpowers/specs/2026-09-28-semver-v2-compliance-design.md)

---

## Context

heraut says "semver," but it only implements the bare `MAJOR.MINOR.PATCH` subset of SemVer v2.
The design doc's § Problem names four concrete gaps:

`semver.IsBareVersion` is the only version recognizer the resolvers call
(`semver/resolver.go`, `perenv/auto.go`, `perenv/promote.go`): a tag carrying a pre-release
identifier (`-rc.1`) or build metadata (`+5`) is silently skipped during resolution rather than
being ordered against the rest of the tag list. There is no §11 precedence comparator anywhere
in the codebase — tag ordering is delegated to git's `--sort=-version:refname`, which puts
pre-release tags out of order relative to their finals unless the caller separately configures
`versionsort.suffix`, which is the very reason the skip policy above exists (Spec 04 §
Pre-release tags). The documented `{build}` usage is itself non-compliant:
`tag_format: "{env}/{version}-{build}"` renders `uat/7.4.1-158404`, which SemVer reads as a
**pre-release** of `7.4.1` — lower precedence than `7.4.1` itself — not as build metadata, the
only thing `{build}` is meant to carry. And heraut cannot mint a pre-release tag at all: GitHub's
pre-release flag is a static per-target config bool (`release.targets[].prerelease`) that can
contradict the version actually being published, and there is no mechanism to ship a real
`1.4.0-rc.1`.

Two forces motivate closing these gaps now rather than living with them: a real need to ship
pre-releases, both a human-driven RC phase and a CI-driven continuous channel, and a correctness
bar — a tool that names itself after SemVer should be strictly compliant with it.

## Decision

**Make `semver` and `semver-per-env` strictly SemVer v2 compliant in Phase 1, then add a
pre-release lifecycle for plain `semver` in Phase 2.**

**Strict parsing and §11 ordering live in Go, for SemVer strategies only.** A new
`internal/versioning/semver.Version` type and `Parse` implement the full SemVer v2 grammar —
numeric identifiers without leading zeros, no empty identifiers, `[0-9A-Za-z-]` only — replacing
`IsBareVersion`'s bare-only recognition. A new `Compare` implements §11 precedence: build
metadata is ignored, numeric identifiers compare numerically and always sort below alphanumeric
ones, alphanumeric identifiers compare in ASCII order, a longer identifier list wins when the
shared prefix is equal, and a pre-release always sorts below its final. Tag listing keeps git's
`--sort` flag in the invocation (it is harmless and remains a reasonable pre-filter), but the
order heraut actually acts on is decided by `Compare` in Go, not by git's `version:refname`
comparator, for version resolution, the E002 check, `version current`, changelog section bounds,
release-notes previous-tag resolution, changelog rotation, and compare links (T334) — the
`versionsort.suffix` caveat in Spec 04 § Pre-release tags goes away entirely for those surfaces.
Since `internal/generators/native` may not import `internal/versioning` (layering), the app layer
passes the ordering decision down as a plain function (`native.WithTagOrder`, built by
`app.tagOrderFor`) rather than native computing it itself. Under a SemVer strategy, a pre-release
tag is therefore never a range boundary in a *regenerated* `CHANGELOG.md` (`--regenerate`
walks tags via that same §11 order, which excludes pre-releases, so a rebuilt file folds a
pre-release's commits into the next release's section and gives the pre-release tag no section of
its own) — this is the steady-state Phase 1 guarantee. It is not yet true at cut time: until
Phase 2's changelog skip for pre-release cuts lands, `heraut changelog --set-version X-pre --tag`
still writes an anchored `[X-pre]` section when the pre-release is cut (incremental generation
has no "this tag is a pre-release, skip its own section" check), the next final's incremental run
re-lists those same commits in its own section, and a subsequent `--regenerate` is what actually
collapses the pre-release section away, per the rule above. A pre-existing `[X-pre]` section in an
upgraded repo's `CHANGELOG.md` survives incremental runs the same way until a `--regenerate`.
`calver` and `calver-per-env` keep their own lenient dotted-integer path
(`compareVersionStrings`) unchanged: a CalVer version like `2026.05.0` carries leading zeros,
which strict SemVer parsing rejects outright, so CalVer is deliberately never routed through the
new parser or comparator.

**A tag carrying only build metadata is a release, not a skip.** `v1.4.0+5` has no pre-release
identifiers, so per §10 it is precedence-equal to `1.4.0` and semantically a release like any
other — it must be resolvable and eligible as a bump base exactly like a bare tag. Pre-release
tags are the only ones excluded as a bump base, never build-metadata-only tags.

**`{build}` must directly follow `+` in `tag_format`, with no deprecation window.** Any other
placement — including today's documented `{version}-{build}` — is a semantic validation error
with a hint. This is a clean break: no known real config depends on the old placement, and
leaving it valid would keep shipping pre-release-shaped tags for what config authors intend as
build metadata. `ParseVersion` and the tag-listing glob both follow the `+`-only form.

**`--set-build-id` gains parity on the plain `semver` strategy.** It renders
`<tag_prefix><version>+<id>` (`v1.4.0+158404`), matching the per-env behavior exactly, and it
keeps requiring `--set-version` alongside it, exactly as per-env does today.

**`version current` prints the latest final by default; `--include-pre-release` prints the
highest §11-precedence tag of any kind.** This keeps today's meaning for every existing script
that calls `version current` unqualified. `commit check --from-latest-tag` is unaffected and
keeps its existing "latest tag of any kind" semantics — it answers a different question from
`version current`.

**`--set-version` is validated as a SemVer v2 version under `semver` and `semver-per-env`,
delivered as "Phase 1.5" rather than bundled into Phase 2 (T336).** The design doc originally
scoped this alongside the pre-release lifecycle, since an accepted pre-release value is also the
manual escape hatch for retargeting a series by hand — but the validation itself needs none of
Phase 2's series/escalation machinery, so there is no reason to wait for it: it is strict
`semver.Parse` on the prefix-stripped value, run the same way for both strategies' override
paths, before either tag is rendered. A bare core (`1.4.0`), a prefixed value (`v1.4.0`), and a
pre-release (`1.4.0-rc.1`) are all accepted — the pre-release case ahead of the lifecycle that
will eventually mint one, matching the design doc's reasoning that it is always a valid manual
override. Anything that fails strict parsing (`1.4`, `01.4.0`) is rejected naming `--set-version`;
a value carrying build metadata (`1.4.0+5`) is rejected with a hint toward `--set-build-id`
instead, since build metadata has exactly one entry point into a tag. `calver` and
`calver-per-env` are unaffected — they keep today's lenient non-empty/no-whitespace check, since
a CalVer value like `2026.05.0` carries a leading zero and is not valid SemVer to begin with.

**Phase 2 (deferred, sketched here for completeness): a pre-release lifecycle for plain
`semver` only.** The core of a pre-release is always computed the same way a final's core would
be — the bump over all commits since the last final, with `stay_at_v0` applied — so a fix or
feature landing inside an open series keeps the core fixed (`beta.1` → `beta.2`) until the
batch's overall bump level actually rises. A minor or patch escalation of that core is allowed
with a warning naming the responsible commit(s); a major escalation errors unless
`--allow-major` is passed, which becomes that flag's second trigger. Every new pre-release tag
must have strictly higher §11 precedence than every existing tag sharing its core — including
that core's final, if one has already shipped — so cutting a pre-release of an already-released
core is impossible and a channel can never accidentally regress. The label is supplied per
invocation via `--pre-release <label>` only, with no branch-to-label mapping in config; heraut
always appends a dotted numeric counter (`rc.2`, never a bare `rc`) so §11 orders the series
correctly. Cutting a new pre-release requires at least one commit since the previous tag,
pre-release or final — the same "nothing to release" guard `semver`'s resolver already applies
today, now also naming the previous pre-release tag when there is one; promoting an existing
pre-release straight to its final is exempt from that rule and needs no new commit at all,
because the commits since the last final already justify the final's bump. A pre-release run
never touches `CHANGELOG.md` and makes no changelog commit — it reuses the existing
`disable_changelog` skip path, and the tag lands on the current `HEAD`. A final released after
one or more pre-releases has its `CHANGELOG.md` section and release notes span back to the
**last final**, not the most recent pre-release tag — pre-release tags are never range
boundaries for a final. GitHub's `--prerelease` flag is derived from whether the published
version carries pre-release identifiers, and `release.targets[].prerelease` is removed as a
config key — a static bool can only ever contradict the version being published, which the
derived flag makes structurally impossible.

## Status update (Phase 1.5)

Phase 1.5 closed (T336, T337): `--set-version` validation under `semver`/`semver-per-env`, and
the GitHub-derived `--prerelease` flag with `release.targets[].prerelease` removed, both landed
ahead of Phase 2 — see `docs/tasks/semver-v2-roadmap.md` § Phase 1.5. The `port.Platform`
contract changed to `CreateRelease(tag, notes string, prerelease bool) error`: the pipeline
derives `prerelease` once from `versioning.Result.Version` via `semver.Parse`/`IsPreRelease`,
gated by whether the active strategy is SemVer-based (`pipeline.Config.SemVerStrategy`, set by
the app layer from `versioning.strategy`) — a CalVer version can itself parse as a SemVer
pre-release (e.g. `2026.10.2-0` under a `format: YYYY.MM.SS-PATCH`), so "fails `semver.Parse`" is
not a safe proxy for "is CalVer"; only the strategy gate is. Under `calver`/`calver-per-env` the
flag is always false; under `semver`/`semver-per-env` it is `IsPreRelease`'s result. GitLab's
driver accepts and ignores it, having no pre-release concept of its own.

## Status update (Phase 2)

Phase 2 closed (T338-T342): `--pre-release <label>` on `heraut release` and `heraut version next`
for plain `semver`, with the series, escalation and regression rules sketched in § Decision above.
See `docs/tasks/semver-v2-roadmap.md` § Phase 2 for the task notes. The § Decision paragraph
headed "Phase 2 (deferred, sketched here for completeness)" and the interim DOC-1 wording
elsewhere in this ADR are historical; this update is authoritative where they differ.

- **Previous-tag rule.** A pre-release's previous tag is the highest-precedence tag of any kind
  (pre-release or final) strictly below the version being cut and reachable from `HEAD`
  (`git tag --merged HEAD`). One rule drives both the commit requirement and the release-notes
  range. Finals are unchanged: last final, by precedence only. Core and last-final computation
  stay global; branch-aware resolution is filed as T344.
- **Commit-requirement exemption amended.** § Decision exempts only promotion to the final from
  the "at least one commit since the previous tag" rule. T342 widens that: a pre-release of the
  same core switching to a different, higher label (`beta.2` to `rc.1`) needs no new commit,
  matching the worked-examples row in the design doc. Re-cutting the same label, or a previous
  tag of a different core, still requires a commit. Where this update and § Decision differ,
  this update governs.
- **"Is a pre-release run" is keyed on the version, not the flag.** It is true for
  `--pre-release <label>` and for a `semver`/`semver-per-env` `--set-version` value carrying
  pre-release identifiers (the same gate as the GitHub `--prerelease` flag). So
  `heraut changelog --set-version X-pre --tag` tags without writing a section; `heraut changelog`
  now forwards `--set-version` to make this detectable.
- **Changelog skip reuses `disable_changelog`.** The app layer decides the pre-release run at
  build time and feeds the existing skip path, so step totals are fixed at build time and no new
  pipeline branch exists. A pre-release message wins over "changelog disabled" when both apply.
- **Escalation warning lists commit subjects, not hashes.** This deviates from the design doc's
  `(feat abc1234)` example and matches the shape of the `stay_at_v0` warning. A major escalation
  still errors with `ErrMajorEscalation` unless `--allow-major`; a regression errors with
  `ErrPreReleaseRegression`; both exit with the Runtime code, while usage errors are Config (2).
- **Absent-tag ordering fix (T341).** native inserts the tag being released into the ordered
  list before choosing the predecessor, so a tag absent from git no longer falls through to "the
  newest existing tag". This also applies to finals' release notes: a hotfix `v1.3.1` cut while
  `v2.0.0` exists resolves `v1.3.0`, not `v2.0.0`.
- **Not handled, deliberately.** Pre-release `[X-pre]` sections already on disk in an upgraded
  repo's `CHANGELOG.md` get no migration code or note: no user has any (decided 2026-10-04).

## Consequences

Test rows that asserted the old `-{build}` tag shape or git-sort-driven ordering are rewritten
in place under this ADR, not deleted — the hard-won edge cases they protect (`v1.9.0` →
`v1.10.0`, CalVer `PATCH` reset, per-env cycle detection) stay intact; only the assertions that
this ADR deliberately changes move. Tags that were accepted before under the loose bare-version
check but are not valid SemVer v2 (`v1.02.0`, with a leading zero) are now ignored during
resolution instead of silently treated as a version — a stricter parser necessarily rejects some
input the old one let through. Once Phase 2 ships, a continuous channel opened on a core that
already has a higher-labeled pre-release (e.g. a `next` tag cut after an `rc` of the same core)
is blocked until the final ships — an accepted, deliberate consequence of global per-core
monotonicity rather than a per-label one, because a per-label scheme would need heraut to track
which labels form a single ordered "line" and which are independent channels, a concept nothing
in the codebase or the design needs today.

## Alternatives considered

- **A locked pre-release core**, where the core is fixed at the first pre-release of a series and
  never re-derived from new commits. Rejected: it lets a final ship containing a feature it was
  never bumped for, if a `feat` lands inside what started as a patch-only series — the exact
  outcome SemVer's own bump rules exist to prevent.
- **Per-label monotonicity** instead of per-core. Rejected: it requires heraut to know which
  labels belong to the same conceptual "line" (is `next` a continuation of `rc`, or a separate
  channel?) — a concept with no existing counterpart in config or code, for a distinction no real
  request has asked for.
- **An auto-incrementing build counter**, minting a fresh `+N` on every CI run. Rejected: tags
  that differ only in build metadata are precedence-equal under §10, so a tag-per-build scheme
  produces entries that no SemVer-aware tool — including heraut itself — can tell apart or order.
  CI run numbers belong on build artifacts; `--set-build-id` already covers the one case that
  needs a tag-level identifier.
- **Keeping `{build}` free-form**, tolerating any placement rather than requiring it directly
  after `+`. Rejected: that is exactly the documented non-compliance this ADR exists to close —
  `{version}-{build}` would keep rendering a SemVer pre-release for what every user actually
  means as build metadata.
- **Pre-release minting under `semver-per-env`**, deferred rather than rejected outright.
  Environments are already per-env's staging model (`dev → uat → prod`); stacking RC labels on
  top would double the concepts on offer for a use case nobody has asked for. Revisit if a real
  user needs a per-env gate on promoting pre-releases.
