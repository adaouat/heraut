# Héraut — Hardening 2 Roadmap

> Status: Complete (both phases done)
> Source: the rule review of 2026-10-16 (`analysis/heraut/RULE_REVIEWS.md`): eight business rules the
> maintainer ruled wrong or not wanted. Main roadmap: tracked as Phases 74–75 in
> [`roadmap.md`](roadmap.md). Continues [`hardening-roadmap.md`](hardening-roadmap.md) (Phases 70–73).

The first hardening epic implemented the rule verdicts of 2026-10-14. The review of the remaining 63 rules
confirmed 54 and found eight whose code should change. Four are small corrections with no design question.
Four change what a user sees or what a config means, and each of those starts with an ADR or an addendum.

## Conventions

- Task IDs **continue the global sequence** (`T402+`).
- This file is the **single source of truth** for task status. Same checkbox markers as the other
  roadmaps: `[ ]` not started, `[x]` done. Follow the two-step flow
  ([`workflow.md`](../../.claude/rules/workflow.md)): failing test first, then flip `[ ]` → `[x]` and
  add a one-paragraph completion note.
- One task per session. Phase 75 starts only after Phase 74 is done, so the cheap corrections land first.
- Where a task says "ADR first", the ADR (or the addendum) is committed and approved before the test is
  written, because a tested behavior changes. No test row is deleted to make a change easier.
- Wire names in `internal/config` do not change. A new key (T409) is an addition: update `schema.json` and
  `docs/heraut.sample.yml` in the same commit.
- A rule card's source lines predate the first hardening epic; use the symbol, not the line number.

## Baseline (before Phase 74)

Recorded at `db67dfd`, Go 1.27.2: `go test -count=1 ./...` passes in every package (including e2e Lane A);
total statement coverage **88.0%** (the CI gate is 85%); `golangci-lint` reports 0 issues. Phase exit
criteria include "the same command still passes and coverage does not drop below the gate".

## Decisions taken

| Rule | Topic | Decision |
|---|---|---|
| 002 | SemVer parts | Reject leading zeros and signs in `BumpVersion`'s input |
| 016 | Config error order | Deterministic: sort environment names before validating |
| 013 | Security section | Drop the body heuristic; `security` becomes a built-in commit type, right after Bug Fixes ([ADR-0075](../adr/0075-security-is-a-built-in-commit-type.md)) |
| 015 | Sprint bump | Edit only inside the top-level `versioning:` block; tolerate a trailing comment on its line |
| 032 | `--set-version` | SemVer strategies were already strict (ADR-0064); the CalVer strategies now require the canonical form of the configured format. ADR-0074 addendum |
| 065 | Hook `stage` | `stage` with the changelog disabled (globally or for the env) is a config validation error. ADR-0061 addendum |
| 052 | Default excludes | Excludes stay render-only. Warn when every releasable commit is excluded from the changelog. No bump change |
| 039 | Asset globs | A glob matching nothing at publish time fails, unless the entry is marked optional. New ADR-0076 |

Already settled and not reopened: RULE-003 (build metadata is ignored in ordering) and the SEC-009 repo
confinement of asset globs (done in the first epic).

## Progress at a glance

| Phase | Theme | Tasks | Status |
|---|---|---|---|
| 74 | Small corrections | T402–T405 | Done |
| 75 | Behavior changes with a decision record | T406–T409 | Done |

---

### Phase 74 — Small corrections

Goal: four small fixes. Entry: baseline above recorded. T402, T403 and T405 need no ADR; T404 turned out to
need one (ADR-0075), written before its code.

#### `[x]` T402: reject leading zeros and signs in SemVer parts (RULE-002)

`semver.BumpVersion` splits on `.` and calls `strconv.Atoi` per part, so `01.2.3` and `+1.2.3` are
accepted. Tags come from git, so the exposure is low, but `--set-version` and a hand-made tag both reach
this code. Tests first: a table of rejected inputs (`01.2.3`, `1.02.3`, `+1.2.3`, `-1.2.3`, `1.2.3.4`)
beside the existing rows (`1.9.0` → `1.10.0` stays). Then a strict part parser shared with
`semver.ParseVersion` if it does not already enforce this. Check that an existing tag such as `v01.2.3`
does not become a hard failure for `version current`: decide and record whether it is skipped or reported.

**Completion note:** `BumpVersion` now parses each part with `plainNumber` (digits only, no leading zero, no
sign, via the same `isNumericIdentifier` rule `Parse` uses) and wraps `ErrInvalidVersion`; the old error
messages ("invalid semver", "invalid major/minor/patch") are kept because existing tests assert them. A split
on every `.` replaces `SplitN(…, 3)`, so `1.2.3.4` is now rejected too. The question about a hand-made tag
such as `v01.2.3` needed no decision: the SemVer strategies already skip tags that fail the strict `Parse`
before `BumpVersion` sees them, so `version current` is unchanged. In production the input is always a
canonical `Core()`, so this is defence in depth, not a reachable bug. `IsBareVersion` stays lenient on
purpose (calver-per-env's `2026.05.3`), and `rotation.go` keeps its own `Atoi` (not in scope).

#### `[x]` T403: make config validation error order deterministic (RULE-016)

`validateEnums` iterates the `environments` map in Go's random order, so the list printed by
`heraut check config` and `release` changes between runs. Test first: a config with three invalid
environments yields the same error slice on 50 consecutive calls (it must fail on the current code, so
make the environments enough to catch a random order). Fix by sorting the names once. Look for the same
pattern in the other `validate*` files and fix every map walk, not only this one.

**Completion note:** the only unsorted walk was the per-env loop in `validateEnums`
(`internal/config/validator.go`); it now goes through the existing `sortedEnvKeys`. The test builds six
environments that each carry an invalid `tag_pattern` and checks the error paths come out in name order on
30 consecutive calls (it failed on the old code with a shuffled order). The audit of the other walks found
nothing more to fix: the loader, the forge and per-env validators, the template-snippet check, the `--env
auto` match list and the scaffold's dropped-field list already sort.

#### `[x]` T404: make `security` a built-in commit type and drop the body heuristic (RULE-013)

Reframed after the discussion of 2026-10-20: the original plan (match "security" as a whole word) kept a
hidden rule that reads prose. [ADR-0075](../adr/0075-security-is-a-built-in-commit-type.md) (ADR first)
replaces it: `classify` stops reading the body, and `security` joins `defaultTypes()` as `🛡️ Security`,
ordered right after Bug Fixes (feat 0, fix 1, security 2, refactor 3, docs 4, perf 5, style 6, test 7,
chore and ci 8). Tests first: a `security:` and a `security(scope):` commit group under Security; a
commit of an unconfigured type with "security" in its body now lands in Other; a user `security` entry
replaces it and `remove: true` drops it; `commit verify` accepts `security:`; the wizard picker lists it.
The existing rows change on purpose (the body-rule row now expects Other; the numeric orders after Bug
Fixes move by one); none is deleted, and `TestGroupCommits_SecurityBodyOnlyForUnmatchedType` stays. Update
Spec 02 (default types), Spec 05 (classification looks at the type only), the sample's type comment and
`commit create`'s expectations. No `schema.json` change: `TypeRule` is unchanged.

**Completion note:** [ADR-0075](../adr/0075-security-is-a-built-in-commit-type.md) was committed and accepted
first (`04edac5`). `classify` no longer reads the body and lost its `raw` parameter; `securityGroup` and
`orderSecurity` are gone (Revert 102 and Other 103 keep their numbers). `defaultTypes()` gained `security`
(`🛡️ Security`, "A security fix or hardening") at order 2, and refactor through ci moved up by one. Tests
changed on purpose and none was deleted: the table rows for refactor, docs, perf, style, test, chore and
ci carry the new numbers, the "security body" row now expects Other, and
`TestGroupCommits_SecurityBodyOnlyForUnmatchedType` stays as is. New tests: `security` and `security(scope)`
group under Security, the default section order (Features, Bug Fixes, Security, Refactor), a user entry
replaces the type and `remove: true` drops it, the config defaults (order, label, description), and
`commit verify` accepts `security:` until `remove: true`. The `commit create` picker reads the same
`EffectiveTypes`, so it needed no code or separate test. Docs: Spec 02 and 03 (11 default types), a new
Spec 05 "Commit classification" section, the sample's comment. Deferred: none.

#### `[x]` T405: confine `version sprint bump` to the versioning block (RULE-015)

`config.IncrementSprint` replaces every indented `sprint:` line and inserts only after a bare
`versioning:` line. Tests first (`internal/config/sprint_test.go`): a per-env block that also has a
`sprint:` key stays untouched; a `versioning: # comment` line works for both replace and insert; a
missing key is inserted with the block's own indentation; a file with no `versioning:` block fails with
the existing error. Fix by locating the top-level block first and editing inside it. No YAML round-trip
(it loses comments).

**Completion note:** `IncrementSprint` now works line by line. It finds the top-level `versioning:` line (a
trailing comment is fine), reads the block until the next unindented key, and replaces `sprint:` only at the
block's own indentation, so a deeper `sprint:` or one inside a hook script is untouched. The value's trailing
comment is kept. A missing key is inserted right after the `versioning:` line with the block's indentation.
Inline (flow) style such as `versioning: {strategy: calver}` is refused with a message asking for block style,
and the file is left as it was. Five new tests in `sprint_test.go` (the first one, a `sprint:` line inside a
hook script, and the comment-on-the-versioning-line insert case failed on the old code); the three existing
tests stay. The `detectIndent` helper is gone. One note on the task text: a per-env block cannot carry a
`sprint:` key (the strict loader rejects it), so the realistic stray match is inside a multi-line string such
as a hook script, which is what the test uses.

**Phase 74 exit:** the baseline command passes, coverage ≥ 88.0%, lint clean.

---

### Phase 75 — Behavior changes with a decision record

Goal: four rules whose fix changes what a user sees. Entry: Phase 74 done. Each task commits the ADR (or
addendum) first, waits for approval, then goes test → code → spec.

#### `[x]` T406: validate a CalVer `--set-version` against the configured format (RULE-032)

Narrowed after reading the code (2026-10-20): the SemVer strategies already validate the value
(`validateSemVerStrategyOverride`, ADR-0064); only `calver` and `calver-per-env` kept "non-empty, no
whitespace", so `x$(id)` still reached the tag and the hooks. ADR-0074 addendum first (items 5-7): after the
prefix is stripped, the value must parse under `versioning.format` **and** equal its rendering
(`2026.5.3` is rejected, `2026.05.3` passes), because the ADR-0073 probe compares exact tag strings. No
"full tag" mode and no date-sanity check. Tests first (`internal/app`): a table for `calver` and
`calver-per-env` (canonical value passes; `2026.5.3`, `x$(id)`, `2026.05`, `2026.05.3;id` and an empty
part fail with a Config error naming the format), the prefix cases (`tag_prefix`, leading `v`), and an
end-to-end case through `heraut version next --set-version`. Existing rows stay; the lenient CalVer row, if
one asserts a free-form value, changes with the ADR. Update Spec 03 (the `--set-version` row, line 117, and
the `changelog` row) and the flag help text.

**Completion note:** the ADR-0074 addendum was committed first (`32bd927`). `validateCalverStrategyOverride`
(`internal/app/resolver.go`) runs after the prefix / `tag_format` wrapping is stripped, for both `calver` and
`calver-per-env`, with or without a build ID: the value must parse under `versioning.format` (a regex
anchored both ends) and equal `calver.RenderVersion` of its own values. The error names the format and
shows an example, or names the canonical spelling (`2026.5.3` -> "write 2026.05.3"). It sits after the
`{build}` / `tag_format` checks, so those older messages still win. Tests: a table over both strategies
(canonical, patch zero, leading `v`, unpadded month, padded patch, `x$(id)`, `;id`, missing patch, extra
segment, SemVer-looking), the `tag_prefix` cases, the build-ID path, and a `version next` test asserting the
Config exit code (this last one was written after the code and passed at once; the unit tests failed first).
No existing row was removed: the two "SemVer check does not apply to CalVer" tests keep their values, which
are canonical, and only their comments changed. Decided during the task: PATCH stays unpadded (RULE-009), so
`2026.05.03` is rejected; padding would be a separate opt-in with its own ADR and is not on the roadmap.
Spec 03 (the `release` and `changelog` rows) and the flag help text are updated. Nothing deferred.

#### `[x]` T407: reject hook `stage` when the changelog is disabled (RULE-065)

ADR-0061 addendum first. Today `postBumpStage` is used only inside the changelog block, so with
`disable_changelog` the staged files are dropped without a message. A `stage` entry on `post_bump` or
`pre_changelog` becomes a validation error when the changelog is disabled globally or for the env that
declares the hook (Path, Message and Hint like the other validator errors, pointing at ADR-0061). Tests
first in `internal/config`: global disable, per-env disable, and the valid case (changelog enabled).
Update Spec 02's hooks section and the hooks guide. Check `heraut init` and the sample do not generate the
combination.

**Completion note:** the ADR-0061 addendum was committed first (`90452b0`). Refined while reading the code: `hooks:`
is global but `disable_changelog` is per environment, and `--pre-release` and `heraut changelog` without
`--commit` decide at run time, so the plan "error when the changelog is disabled globally or for the env" would
have rejected legitimate configs. The decision (yours): a **config error** only when the changelog can never run
(`changelogNeverRuns`: no `changelog:` anywhere, or every environment disables it), and a **run-time warning**
otherwise. The warning comes from `warnIfStageUncommitted`, which wraps the `post_bump` (and, in
`ChangelogPipeline`, `pre_changelog`) hook step when its pipeline makes no changelog commit, and names only
the rendered patterns that step added, so two hook points never repeat each other. Dry runs are unaffected.
Existing rows: the two "stage ... is valid" validator rows gained a `changelog:` block (they exercise the
hook-point scope, not the missing changelog); no row was removed. New tests: seven validator cases (incl.
per-env), five pipeline cases (release and changelog, warning and no warning), and two e2e scenarios (the
config error through `check config`, and the warning on `changelog` without `--commit`, added to the existing
"stays uncommitted" scenario). Spec 02 § `stage` and the hooks guide are updated. Not done: `heraut init` never
generates `stage`, so nothing to adjust there.

#### `[x]` T408: warn when every releasable commit is excluded from the changelog (RULE-052)

No ADR: bumping does not change, and Spec 04 gets a note that excludes are render-only. When the commits that
drove the bump (`feat`, `fix`, breaking) are all matched by `rendering.excludes` or the built-in defaults
(`chore(deps…)`, `chore(release)`, `chore(pr)`, `chore(pull)`), the run prints a warning that the changelog
for this release will be empty. Settle in the task where the information lives (the resolver knows the
commits, the generator knows the excludes) without a new import across the layer rules; `Result.Warnings`
is the expected carrier. Tests first: a deps-only range warns and still bumps; a range with one real `feat`
does not warn; the warning appears in `--dry-run` and in the `--json` envelope.

**Completion note:** no ADR, as planned. The question was where the commit list (resolver) meets the excludes
(config, used by the renderer); answered without a new import: `semver` already imports `config`, so the
definition of "excluded" moved into a new `config.ExcludeMatcher` (`NewExcludeMatcher`, `Excluded(subject,
type)`) and the native renderer now uses it too, replacing its private `buildExcludes`/`isExcluded` (one
definition, so the two cannot drift; the existing grouping tests pass unchanged). `Resolver.bumpAfterHold`
calls the new `warnIfChangelogWouldBeEmpty`: with a `changelog:` configured and no pre-release, when every
commit with a bump level above none is excluded, it appends a warning (and a matching empty entry to
`WouldBeVersions`, kept parallel) and the bump is unchanged. Wording: "every releasable commit is hidden from
the changelog by rendering.excludes: this release will have an empty changelog section", followed by up to
five subjects. It rides `Result.Warnings`, so it shows in the run output, `release --dry-run --json` and
`version next --json`. Tests: matcher table (config), eight resolver cases plus the pre-release case, and an
e2e scenario with three rows (warns, real change beside the noise, no changelog configured). Scope notes:
the excludes considered are the global list plus the top-level `changelog:` driver's, not a per-environment
`changelog:` override, and `calver` has no bump to explain, so it never warns. Spec 02 (render-only paragraph)
and Spec 04 are updated.

#### `[x]` T409: fail when an asset glob matches nothing at publish time (RULE-039)

New ADR-0076 first. Today `ResolveGlobsLenient` warns and publishes a release without its assets, so a
typo ships silently. The pre-publish check (`CheckAssetGlobs`) keeps skipping zero matches because a hook
may still build the files; the check at publish time, after the hooks ran, fails instead. The ADR decides
how an entry is marked optional (a per-asset `optional: true` is the working proposal) and what the error
says. This adds a config key, so update `schema.json`, `docs/heraut.sample.yml`, the validator and the
`testdata/config/valid` fixtures with it. Tests first: a missing required glob fails both drivers before
the forge call, an optional one warns, and a matching glob is unchanged (contract tests keep their exact
argument assertions). Update Spec 05 and the failure-recovery guide (state C).

**Completion note:** the ADR changed shape before any code. Reading the specs showed the lenient behavior was
deliberate: assets resolve at publish time, after the tag is pushed, so failing there leaves a tag with no
release. That cost was not weighed when "fail at publish time" was chosen, so the decision (yours) became
[ADR-0076](../adr/0076-a-release-asset-pattern-must-match-a-file.md): the existing pre-commit asset gate
(`CheckAssets`, after the `post_bump` hooks, before the changelog commit, the tag and any push) now refuses a
pattern that matches no file, wrapping the new `port.ErrAssetUnmatched`; publish-time resolution stays
lenient. `release.allow_missing_assets: true` (root `release` block, default false, covers `release.assets`
and every target) restores warn-and-skip for assets a later hook builds. A dry run only warns, because hooks do
not run in it. Code: `platforms.CheckAssetGlobs(patterns, allowMissing)` reports every unmatched pattern at
once and counts a directory-only match as unmatched; `ResolveGlobsLenient` warns on the same rule;
`config.Release.AllowMissingAssets` and `config.Platform.AllowMissingAssets` are wired in
`buildTargetPlatforms` and `effectiveTargetPlatforms`. Tests: platform-level cases for both drivers, the
`app` wiring, two pipeline cases (refused before any mutation; dry run warns), a loader test with a new
fixture (`release-allow-missing-assets.yml`, also checked against `schema.json`), and the e2e scenario now
covers matching files, refusal with nothing committed or pushed, the dry-run warning and the flag. Existing
rows changed on purpose, none removed: the e2e "a pattern matching nothing publishes without assets" now runs
under `allow_missing_assets`, and the platform and glob tests that relied on a silent zero match pass the flag.
**Found on the way, fixed in this task:** a refusal from any pre-commit gate was never shown. The gate runs
outside a reported step and the command swaps the returned error for "Release failed", so the user saw no
reason (this already hid the outside-the-repository refusal). `runSteps` now prints a gate failure when a
reporter is in use; a pipeline test pins it. Exit code: Runtime (3), like the outside-the-repository refusal,
not Config as the draft said. Updated: `schema.json`, the sample, Specs 02, 03 and 05, the failure-recovery
guide (states A and C). Deferred: none; a per-pattern optional marker stays a future option (ADR-0076,
alternatives).

**Phase 75 exit:** the baseline command passes, coverage ≥ 88.0%, lint clean, and `heraut check config`
still accepts every file in `testdata/config/valid/`.

---

## Out of scope

- RULE-003 is confirmed as is. RULE-007, 008, 046 and 058 were done in the first epic (T383–T390).
- Repo confinement of asset globs is done (SEC-009, T381).
- The 60 rules confirmed in the review are not reopened; the brief's other follow-ups
  (`runPlatformWizard` size, the `git remote get-url origin` call in `scaffold/wizard.go`, duplicated token
  env names in `forge/detect.go` and `pipeline/linkctx.go`) stay in the main roadmap's open items.
