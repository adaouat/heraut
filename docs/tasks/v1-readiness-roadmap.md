# Héraut — v1.0 Readiness Roadmap

> Status: Active (drafted 2026-11-07 from the pre-v1.0 review)
> Main roadmap: tracked as Phases 94–101 in [`roadmap.md`](roadmap.md).
> Source: a five-part review on 2026-11-07 (core logic, integrations and security, tests, user docs,
> contributor docs). Every item below was reproduced against the built binary or quoted from the source;
> items marked ✔︎ were re-run a second time before this file was written.

The user decided that **every finding is fixed before v1.0**, including the ones the review graded
post-v1 (Phase 100). The v1.0.0 cut (Phase 101) comes last.

## Conventions

- Task IDs **continue the global sequence** (`T492+`).
- This file is the **single source of truth** for these tasks. Markers: `[ ]` not started, `[x]` done.
  Two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)): failing test first, implement, flip the
  marker and add a completion note. "start Phase N" approves that phase's tasks.
- Every behaviour change gets a mutation check (revert the production change, confirm the new test
  fails; on macOS use `go test -timeout`, vet first, keep a surviving control).
- A behaviour change updates the spec and the user pages that show it (`workflow.md` § User docs), and
  `mise run docs:gen` when help text or `schema.json` changes.
- Commits go straight to `main`; never push without asking. Lane B (`e2e_forge`) runs only when the user
  says so, against `bchatard/heraut-testing` (GitHub and GitLab).
- **The GitLab driver (`internal/platforms/gitlab`) is off-limits without the user's explicit go.** Tasks
  that would touch it say so (T501, T516, T525, T548, T549) and stop for that go first.
- **CI/CD changes need the user's go first** (Dockerfile, `.github/workflows/*`, `.goreleaser.yml`):
  T520, T550.
- Tasks marked **Decision** start with a short options sheet for the user; the work follows the choice.
- Synthetic names only in tests and docs (`acme/app`, `example.com`).

## Progress at a glance

| Phase | Theme | Tasks | Status |
|---|---|---|---|
| 94 | Release-logic bugs that block v1 | T492–T501 | Done |
| 95 | Decisions to make before the format freezes (ADRs) | T502–T506 | Done |
| 96 | Behaviour fixes | T507–T522 | Done |
| 97 | Tests that give false confidence | T523–T529 | Not started |
| 98 | User docs | T530–T536 | Not started |
| 99 | Contributor docs | T537–T541 | Not started |
| 100 | Remaining review items (graded post-v1, done before v1 by decision) | T542–T554, T556 | Not started |
| 101 | Cut v1.0.0 | T555 | Not started |

---

### Phase 94 — Release-logic bugs that block v1

#### `[x]` T492: commit a rotating changelog under its real file name ✔︎

With `changelog.output: "CHANGELOG_{YYYY}.md"`, `heraut changelog --tag` and `heraut release` fail at
the commit step: `git add: … pathspec 'CHANGELOG_{YYYY}.md' did not match any files` (exit 3), leaving
`CHANGELOG_2026.md` untracked. The path is read when the step list is built
(`internal/pipeline/steps.go:99`, `changelog.go:140`), before `Generate` runs, so `LastOutputPath()` is
still empty and the raw pattern is passed. Regression from the "describe each release step once"
refactor; `release_test.go:648` hides it because its `MockGenerator` reports a path before `Generate`.
Fix: resolve the path inside the commit step's `exec`. Tests: make the mock report a path only after
`Generate`, a pipeline test for both flows, and a Lane A scenario with a rotating output.

**Done (2026-11-07).** Both flows (`steps.go`, `changelog.go`) now hold the path as a closure read when
each step runs, so the commit step sees the file `Generate` resolved; the summary lines were already
read after the run. `MockGenerator.LastOutputPath` now reports a path only after `Generate`, like the
real wrapper, which turned the two existing rotation tests red against the old code (that red run is
the mutation check). New Lane A scenario `TestChangelogFlow_RotatingOutputIsCommitted` (testclock
binary, `CHANGELOG_{YYYY}.md`) fails on the old code with the reviewer's `pathspec` error and passes
now. The dry-run line still shows the raw pattern, since nothing has been generated then; no spec or
user doc described the broken behaviour, so none changed.

#### `[x]` T493: re-rendering a tagged version keeps its section

Re-rendering the changelog of a version that is already tagged replaces its section with the commits
*after* the tag: the bound is `scopedTags[0]`, the tag itself, so the range is `<tag>..HEAD`
(`internal/generators/native/generator.go:447`, `:329`, `newSectionBound`). Hit by the documented
maintenance path (Spec 04 § Maintenance branches, row 12b), by `heraut changelog --set-version 0.1.0`
after v0.1.0, and by `--force` re-runs (T503); with `--regenerate` the section appears twice. Fix: when
the tag is already in the scoped list, bound by its predecessor with `tip=tag`, and skip it in the
historical loop. Tests: native unit tests for the three entry points; a Lane A scenario on a release
branch.

**Done (2026-11-07).** The previous-release computation of the historical loop moved into
`previousRelease`, and a new `newestSection` uses it when the target tag is already in scope: the
section then runs from the previous release to the tag. The incremental and full builds both go through
it, and the full build's loop skips that tag, so `--regenerate` renders it once.
`TestGenerateChangelog_RegenerateOldestScopedTagExcludesOutOfScopeHistory` regenerated an existing
`prod/v1.1.0` and queued a `prod/v1.1.0..HEAD` call that no longer happens; I removed that mock response
and moved the call index, and its assertions are unchanged. New tests: a contract test asserting the
`v1.0.0..v1.1.0` range and the in-place replacement; Lane A `TestChangelogFlow_RerenderingATaggedVersionKeepsItsSection`
(`--set-version` incremental and `--regenerate`) and `TestVersionBranch_ChangelogRerendersTheTaggedVersion`
(Spec 04 row 12b), both failing on the old code. Mutations: disabling the existing-tag branch and
removing the loop skip were both killed; an equivalent rewrite (`i > -1`) survived as the control.
Spec 05 § Changelog structure now states the bound. The `--force` re-run case belongs to T503.

#### `[x]` T494: the per-env branch guard and `--env auto` read the CI branch ✔︎

`internal/versioning/branch/guard.go:34` and `internal/app/env.go:35-42` use
`git rev-parse --abbrev-ref HEAD` only. On a detached CI checkout with `CI_COMMIT_BRANCH=main`,
`release --env prod` (`prod.branch: main`) exits 2 with `…must be operated from branch "main", but the
current branch is "HEAD"`, and `--env auto` says `HEAD is detached`. Every GitLab CI or Azure per-env
project with `branch:` is refused; the only escape is `--force`, which also drops E001/E002. Fix: use
`branch.CurrentBranch` (which already falls back to the CI variables) in both places. Tests: contract
rows per CI system, a Lane A detached-HEAD per-env scenario.

**Done (2026-11-07).** `CheckBranch` and `ResolveEnv("auto")` now call `branch.CurrentBranch`, the
helper the push and the branch rules already use; without a CI branch the guard still reports
`"HEAD"` and `--env auto` still says HEAD is detached, so the messages outside CI are unchanged. Tests:
a guard table over GitLab, GitHub (branch refs only, a tag build is refused), Azure, a different CI
branch and no CI, and an `--env auto` case. `TestResolveEnv_Auto_DetachedHEAD` now clears the CI
variables, since it would otherwise pass or fail depending on the runner. Lane A
`TestDetachedHEAD_PerEnvBranchGuardReadsTheCIBranch` (`--env prod`, `--env auto`, and a refused
`develop`) fails on the old code. Spec 02's `branch` row and the environments guide say where the
branch comes from on a CI checkout. `--force` is unchanged here (T503).

#### `[x]` T495: calver-per-env reads every CalVer format's tags

`internal/versioning/perenv/order.go:32` filters tags with `semver.IsBareVersion` (`semver/bump.go:170`),
so any CalVer format that isn't three dotted integers (`YYYY.MM.DD.PATCH`, `YYYY.PATCH`, a literal
suffix; all in Spec 04's examples) sees no tags: `version next --env dev` re-prints an existing
`dev/2026.11.07.0`, promotion says E003, and a second run fails at `git tag … already exists` (exit 3)
after the changelog commit was pushed. Plain `calver` is correct. Fix: validate with
`calver.ParseVersion(tokens, bare)`. Tests: a table over Spec 04's format examples under per-env.

**Done (2026-11-07).** `releaseTags` keeps a calver-per-env tag when its version parses under
`versioning.format` (`calverVersionCheck`; an unusable format, which the validator rejects first,
falls back to the old check). The same gap was in E002: `compareVersions` read versions as dotted
integers, so with a trailing literal `2026.05.1-lts` compared equal to `2026.05.0-lts` and a
destination ahead went unseen. It now compares by calendar value under the format (it takes the
format as a new argument). Tests: an auto-mode table over Spec 04's examples plus `-lts` (three rows
red before), an E002 table over the four-component, yearly and suffixed formats, and Lane A
`TestCalVerPerEnv_DailyFormatReadsItsTags` (testclock), which on the old code re-minted
`dev/2026.11.07.0` and refused prod with E003. Mutations: reverting the comparison was killed by the
`-lts` row; an equivalent rewrite survived as the control. Spec 04 § Pre-release tags now describes
the format-based reading. The changelog's ordering of CalVer tags (`calverOrder`, Spec 05) is T496 and
T552.

#### `[x]` T496: calver-per-env changelogs stay in their environment

With `tag_format: dev/{version}` / `prod/{version}` (no `{env}`, Spec 04's own calver-per-env example),
`heraut changelog --env prod` writes the dev tags' sections and no prod section.
`internal/app/current.go:188-210` (`calverOrder`) keeps tags it can't read, and
`internal/app/pipeline.go:531-541` derives tag scoping only for `{env}` formats. semver-per-env is
correct (`SortTags` drops unparsable tags). Fix: drop unreadable tags in `calverOrder`, and derive the
tag glob from any per-env `tag_format`. Tests: both shapes, both strategies.

**Done (2026-11-07).** `withEnvDerivations` now derives the env's tag glob on every per-env run, not
only when `tag_format` names `{env}`: `prod/{version}` lists `prod/*`. I did not make `calverOrder`
drop unreadable tags, the other fix the review offered: under plain calver a legacy tag (`v1.2.3`)
bounds the oldest CalVer section today, and dropping it would make that section walk the whole history.
The glob alone fixes the reported case. Tests: `TestWithEnvDerivations_SetsTagGlobForALiteralEnvPrefix`
(killed by restoring the `{env}` condition), and Lane A `TestCalVerPerEnv_ChangelogStaysInItsEnvironment`
for both shapes; the literal one wrote only dev's sections before. Spec 02's `tag_pattern` row says the
derivation covers literal prefixes. Found along the way: a literal-prefix env's heading keeps the whole
tag (`## [prod/2026.11.0]`) while a `{env}` format strips it; recorded as T556.

#### `[x]` T497: say so when tags exist but none match the prefix ✔︎

With no `tag_prefix` and existing bare tags (`1.0.0`), semver uses the `v` default
(`internal/app/resolver.go:470`) and `version next` prints `v0.1.0` with no warning, so adopting heraut
on a project with bare tags silently restarts at 0.1.0. `docs/heraut.sample.yml:33` says omitting it
gives bare tags, and `docs/reference/configuration.md:69` shows no default. Fix: warn (version next,
release, changelog, check config) when the repository has version-like tags but none match the active
pattern, naming the prefix to set; correct the sample and the reference. Tests: resolver + Lane A.

**Done (2026-11-07).** A `foreignTagsResolver` wraps the plain `semver` and `calver` resolvers (not
`--set-version`, not a top-level `tag_format`). On a first release (no current tag) it lists every tag
once and, when some read as versions under another prefix (the text before the first digit), adds a
warning naming the most common prefix and the setting:
`! no tag matches versioning.tag_prefix "v", but 2 tags look like versions with the prefix "" (e.g.
1.0.0): set versioning.tag_prefix: "" to continue from them`. It rides `Result.Warnings`, so `version
next` (plain and `--json`), `release` and `changelog` show it; the run goes on. Deviation: `check
config` doesn't show it, since it reads no git; the adoption guide (T530) covers that path. A first
release now makes one more git call (`git tag -l`); three tests stubbing git had to answer it, and the
calver one now also asserts its arguments. Tests: a real-git table (bare tags, `v` tags with a bare
prefix, calver, matching tags, a non-version tag), an internal table for the prefix choice and the
plural, Lane A `TestSemver_WarnsWhenTagsUseAnotherPrefix`. Mutations: skipping the check and making
ties pick the last prefix were killed; an equivalent comparison survived. Docs: Spec 04 § Prefix
handling, the sample (default per strategy), `schema.json`'s description (so the generated reference),
and the versioning-strategies concept page.

#### `[x]` T498: re-running `heraut init` keeps what it doesn't ask about

Re-running `init` ("Update it?") rewrites the config and loses keys:
- **`tag_prefix`**: a semver config without the key gets `tag_prefix: ""`
  (`internal/scaffold/answers.go:119-121`, `generate.go:54-55`; `wizard_test.go:94-101` locks it in), so
  `version next` goes from `v1.4.3` to `0.1.0`.
- **Silently dropped**: `hooks`, `git.push_options`, `versioning.bump` (`stay_at_v0`, `confirm_major`),
  `versioning.branches`, `commit_message`, `release.allow_missing_assets`, `targets[].assets`.
  `DroppedFields` (`dropped.go:50-52`) is a stub returning nil. Losing `stay_at_v0` lets the next release
  reach 1.0.0.

Fix: pre-fill the prefix with the strategy default when unset (or omit the key), carry every key the
wizard doesn't ask about through verbatim (the existing pattern for `commits.tickets`), and make
`DroppedFields` report whatever still can't be kept, shown before writing. Spec 03:76-87 lists the wrong
set (it says tickets and assets are dropped; they're kept): rewrite it from the code. Tests: a
round-trip of a config holding every key.

**Done (2026-11-07).** Deviation from the planned fix: rather than adding each kept key to `Answers`,
`GenerateUpdatedYAML` lays the answers over the existing config (`overlay.go`). The wizard's own keys
take the answers; every other key, including ones added later, stays as it was. That covers `init`'s
update flow and `--defaults` on an existing file. `ConfigToAnswers` offers an unset prefix as the
strategy default, and `TestConfigToAnswers_NilPrefix`, which locked in `""`, now asserts `"v"`. Writing
a loaded config back exposed two encoders the config lacked: `Rendering` (nested `templates.commit.*`
and the trailers, flattened on load) and `PushOptions` (an environment's `tag: []`, meaning "none",
was dropped by `omitempty` and turned into "inherit"); both now have `MarshalYAML`. Forges no release
target uses (PR data only) are kept, since the wizard never shows them. The `DroppedFields` stub is
gone; its nine tests asserted the stub's nil, and they now check that their config survives an update.
New tests: every valid fixture plus the sample encode back to the same config (`internal/config`);
every valid fixture survives an unchanged update (18 of 21 lost something before); changed answers
apply and keep hooks, push options, bump, `allow_missing_assets` and target assets; turning the
release off drops it. Mutations: dropping the untargeted forges and writing `tag` only when non-empty
were both killed; an equivalent condition survived. Comments and key order are still not kept (the
file is rewritten); Spec 03 says so. Spec 03 § init lists what is kept, and `init`'s help says it.
No Lane A scenario: the wizard needs a terminal, and `--defaults` goes through the same function.

#### `[x]` T499: **Decision** — a config without `release:` ✔︎

With no `release:` block, `heraut release` still publishes to the detected forge and runs no
release-notes step (`internal/app/pipeline.go`: notes stay nil while targets come from detection);
`release --dry-run` plans `[6/6] Publish to github`. The docs say the opposite:
`docs/reference/configuration.md:111`, `docs/heraut.sample.yml:488-490` ("tag-only", "no config shape
that gets one without the other") and `docs/guides/environments.md:9-32`, whose example config hits it
for every environment. Options: (a) publish with default notes, (b) refuse with a config error naming
the block to add, (c) tag only. Then align the code, the spec, the reference, the sample and the guide.

**Done (2026-11-07). The user chose (c), tag only.** Without a `release:` block (root or environment),
`buildReleasePipelineConfig` takes the `disable_release` path, with no notes and no publish target even
when a forge auto-detects, and adds a pipeline warning: `publishing is off: the config has no
release: block, so no forge release is created (add release: {} to publish)`. A new
`pipeline.Config.Warnings` carries it, printed after the resolver's warnings and listed in the
`--json` plan. `heraut release` checks for a destination only when there is a `release:` block; its
message now names both ways out. `heraut check runtime` shows `publish — off (no release: block)` and
resolves no forge. Five tests assumed the old rule and were updated: the zero-config targets row and
`TestRelease_NoPlatforms_Error` now set `release: {}` (the CI shape Spec 02 documents), and two check
rows and one Lane A check config needed `release: {}` to keep testing what they test. The ambiguous-token
row now asserts publishing is off, with no forge row. New tests: a tag-only row,
`TestRuntimeCheck_WithoutAReleaseBlockPublishingIsOff`, and Lane A
`TestReleaseFlow_WithoutAReleaseBlockTagsOnly` (a real run pushes the tag; a GitHub Actions env plans
no notes and no publish). Docs: Spec 02 § release, Spec 03 § release and § check, CLAUDE.md's
constraint, the sample, the environments guide (its config tags only, now said), and the self-hosted
guide's error row.

#### `[x]` T500: CI values only fill a forge entry on the same instance

`internal/forge/resolve.go:84,93,98` fills a `forges:` entry's host, API URL, project and **token** from
CI whenever the CI platform type matches, even when its `base_url` names another instance:
- GitLab CI on gitlab.com + a forge `base_url: https://gitlab.example.com`, `token_env: SELF_PAT` →
  enrichment calls `https://gitlab.com/api/v4` with the self-hosted PAT.
- GitHub Actions on github.com + `base_url: https://ghe.example.com` → the github.com `GITHUB_TOKEN` is
  sent to GHES; `GH_ENTERPRISE_TOKEN` is ignored.
- Spec 05's own two-GitLab example, run on a gitlab.example.com runner, resolves `gitlab-com` to the
  wrong host.

The same type-only test in `internal/pipeline/linkctx.go:136` (`platformLinkContext`) gives that target
wrong-host links and prints the CI repository's release URL in the step and summary. Fix: take CI values
only when `base_url` is unset or has the CI server's origin; match host and repository in
`platformLinkContext`, and print `plat.ReleaseURL(tag)` in the summary. Tests: resolve tables for each
pairing; a linkctx test.

**Done (2026-11-07).** `resolveExplicit` now takes CI and git-origin values only when the entry's
`base_url` is unset or has the detected host's origin (`sameInstance`, on `config.SameOrigin`). A
self-hosted GitLab entry in a gitlab.com job keeps its `token_env` and its project; a GHES entry on a
github.com runner reads `GH_ENTERPRISE_TOKEN`. `platformLinkContext` uses the CI context only when it
describes the target itself: same platform, same instance, and the same repository unless the target
names none. The summary and publish-step URL already go through it, so they now print the target's
own URL; no separate `ReleaseURL` call was needed. Three pipeline tests modelled a self-hosted job with
only `CI_PROJECT_URL` set and a platform reporting `gitlab.com`. Real wiring gives the platform the CI
host (`platformConfigFromTarget` copies the identity's host), so their fixtures now use a GitLab job's
real variables and that host; they still assert the CI context reaches the notes and the release URL.
New tests: a resolve table (self-hosted with and without `token_env`, GHES, same instance with other
case and a trailing slash), an origin case, a `platformLinkContext` table, and the reviewer's GHES-mirror
case at pipeline level. Mutations: the type-only CI match and the old link-context rule were both
killed. Spec 02 § Identity resolution states the instance rule; Spec 05 states the link rule, and its
two-GitLab example now gives `gitlab-com` a `base_url` (it resolved to the runner's host without one).

#### `[x]` T501: the GitLab job token only goes to its own instance (needs the go for the GitLab driver)

`internal/platforms/gitlab/platform.go:157-165` (`inCIAutologin`) returns true for any GitLab CI job
when `base_url` is gitlab.com (`!p.selfHosted()` skips the host comparison), so a self-hosted job sends
its `CI_JOB_TOKEN` to gitlab.com and ignores `GITLAB_TOKEN`. Spec 05: the job token is used only "on the
instance named by `CI_SERVER_URL`". Fix: always compare `sameGitLabHost(resolveBaseURL(), CI_SERVER_URL)`,
an empty `CI_SERVER_URL` meaning gitlab.com. Tests: `credentials()` table over both directions.

**Done (2026-11-07), with the user's go for this one GitLab driver change.** `inCIAutologin` now always
compares the target's host (`resolveBaseURL`) with `CI_SERVER_URL`, gitlab.com when unset, so a
gitlab.com target in a self-hosted job gets the configured token. `selfHosted`, its only user, is gone.
No other driver change. Test: a `credentials()` table over gitlab.com and self-hosted targets and jobs,
with and without `base_url` or `CI_SERVER_URL`; the gitlab.com target in a self-hosted job returned the
job token before. Mutation: a wrong default for an unset `CI_SERVER_URL` was killed. Spec 05 § GitLab
names the gitlab.com case.

---

### Phase 95 — Decisions to make before the format freezes

Each task: an options sheet, the user's choice, an ADR, then the code, spec and docs.

#### `[x]` T502: **Decision** — compatibility after 1.0

Every config break so far was justified as "pre-v1, no shim" (ADR-0040:29, ADR-0046:96,
`roadmap.md:1418`); nothing says what counts as breaking once 1.0 ships. Write the ADR: the stable
surface (config keys, flags, exit codes, `--json` envelope, template data, embedded template output),
the deprecation window and how a deprecated key warns. Point to it from `coding.md` § Config and
CONTRIBUTING.md.

**Decision (user, 2026-11-08):** SemVer plus deprecation. The stable surface is config keys and values, flags, exit codes, the `--json` envelope and error codes, and template data fields; any of them breaks only in a major. A deprecated key or flag keeps working and warns, naming its replacement, for at least one minor release, and is removed in the next major. Message wording and the built-in templates' output may change in a minor.

**Done (2026-11-08).** [ADR-0080](../adr/0080-compatibility-after-1-0.md) lists the stable surface
(config keys, values and defaults; commands, flags and the environment variables heraut reads; exit
codes; the `--json` envelope, `error.code` values and `data` fields; template data; the tag shapes and
changelog anchors heraut reads back), what is not part of it (message wording, built-in template
output, `internal/`), the deprecation rule, and a security exception. The loader's deprecation warning
is built with the first deprecation, not now. Pointers: `coding.md` § Config, CONTRIBUTING.md §
Releases, the ADR index.

#### `[x]` T503: **Decision** — split `--force`, and make re-tagging honest

On `release`, `--force` bypasses six unrelated things: E001, E002, `enrichment_policy: required`, the
unlisted-branch refusal, the per-env branch guard and the per-env `--set-version` probe. And `--force`
past E001 can never succeed: resolution goes through, the changelog is regenerated, committed and pushed,
then `git tag` fails with `already exists` (exit 3; `perenv/promote.go:170`, `app/resolver.go:246`).
Options: narrow `--force` to the promotion guards and add explicit flags for the rest; make E001
non-bypassable (ADR-0007 matrix) or make it an explicit re-tag. Either way, fail before the commit.

**Decision (user, 2026-11-08):** `--force` means "release from here": it lifts E002 and the branch guards (unlisted branch, per-env branch) and nothing else. E001 is never bypassable and is refused before any commit; the per-env `--set-version` probe goes with it. `--force` no longer downgrades `enrichment_policy: required`: `--offline` is the way to release without PR data. No new flag. (Splitting into named flags was rejected as too heavy.)

**Done (2026-11-08).** [ADR-0081](../adr/0081-force-means-release-from-here.md) amends ADR-0007 (E001)
and ADR-0041 (the `required` downgrade). E001 is checked without `force`, so a forced promotion whose
tag exists stops with exit 4 before anything is written; its message no longer suggests `--force` and
ends "Then run the release again." `guardSetVersion` lost its `force` skip and parameter. The
enrichment downgrade is gone end to end: `ContentDriver.Force`, the `force` arguments of
`buildGenerator`, `wrapWithRotation`, `buildReleasePipelineConfig` and `resolveEnrichForgeIfNeeded`,
`PipelineOptions.Force`, `NotesOptions.Force`, and `heraut notes --force`. Tests that encoded the old
rule now expect the new one: E001 with force, the two per-env "taken, --force" probe rows, the E001
message suffix, and the e2e "E001 is bypassed by --force" row. Three tests of the removed downgrade
were deleted (two native, one resolution); ADR-0081 records the removal. New: a resolver test for
the per-env probe under `--force`, and Lane A `TestPerEnv_E001StopsBeforeWritingEvenWithForce` (the
reviewer's reproduction: exit 4, nothing committed or pushed, no changelog written). Docs: `--force`
help text on `release`, `changelog`, `version next` and `version current` (reference regenerated);
Spec 02/03/04/05/06; the environments, maintenance-branches and PR-data guides; the exit-codes
reference.

#### `[x]` T504: **Decision** — the `--json` envelope on failure

Inconsistent today: failed `version next/current --json` emit `"data":{"tag":"","version":""}`
(`internal/cmd/version.go:52,173`); `commit verify --json "feat:x"` emits `"data":{"skipped":true}` for
an invalid message (`commit.go:360-365`) though Spec 03:518 reserves `skipped` for merge and fixup
commits; `release --json` emits no `data`. Recommendation: omit `data` on failure except for `check`, as
documented. Update Spec 03, the skill and the reference.

**Decision (user, 2026-11-08):** no `data` on failure, for every command except `heraut check`; `commit verify` uses `skipped` only for merge and fixup commits.

**Done (2026-11-08).** `writeJSON` drops `data` whenever there is an error; `heraut check` uses the new
`writeJSONReport`, which keeps its findings. That fixes all three shapes at once: `version
next/current` no longer emit an empty `{tag, version}`, an invalid `commit verify` no longer emits
`{"skipped": true}`, and `release` was already right. Tests: a unit test of both writers, and Lane A
`TestJSON_FailuresCarryNoData` over the four commands (three red before) plus `check` keeping its report.
Docs: Spec 03 § Machine-readable output, the skill, the AI-agents guide.

#### `[x]` T505: **Decision** — reject keys heraut would ignore

- `rendering.excludes[]` and `rendering.templates.commit.trailers[]` are decoded with `node.Decode`
  (`internal/config/templates.go:76,118`), bypassing `KnownFields`: `excludes: [{type: chore, typo_key: x}]`
  passes, though `schema.json` rejects it.
- Keys a platform doesn't use pass `check config` and are ignored: `project:` on a github forge (it then
  publishes to the origin's repository), `repository:` on a gitlab forge, `draft:` on a gitlab target
  (publishes immediately).
- `release.notes.output` passes and is ignored (`configuration.md:95` says it isn't accepted).

Tightening after v1 breaks configs that load today. Recommendation: strict decode and a validation error
for each combination now.

**Decision (user, 2026-11-08):** a config error now for all of them, each naming the key and the right one.

**Done (2026-11-08).** `decodeStrict` (the re-marshal plus `KnownFields` that `HookStep` used) now also
decodes `rendering.excludes` and `rendering.templates.commit.trailers`, at every level, since the
`Rendering` type is shared. Validation reports `forges[i].project` on GitHub and
`forges[i].repository` on GitLab (Azure DevOps uses both), `draft` on a target whose forge is GitLab
(root and environment targets), and `release.notes.output` (root and environment); each message names
the key to use. Tests: a table of eight configs (all red before), plus a config using every key where it
applies, which stays valid. Deviation: `schema.json` already refuses the unknown keys but can't express
the per-platform rules (forges share one definition), so the schema still accepts those; the validator
is the authority. Spec 02's key tables say what is refused.

#### `[x]` T506: **Decision** — "nothing to release"

"No commits since v1.4.0", "No releasable commits", `version current` with no tags and a pre-release
that would sort below an existing one all exit 3 (`runtime`), like a real failure, and none is listed in
`docs/reference/exit-codes.md` or SKILL.md:81. A CI job that releases on every push goes red. Options: a
distinct exit code, a distinct `error.code`, or both. Document the result and give a CI recipe.

**Decision (user, 2026-11-08):** exit code 5 and `--json` `error.code: "nothing_to_release"`, for no commits since the last tag, no releasable commits, and `version current` with no tag yet. Documented with a CI recipe (`|| [ $? -eq 5 ]`).

**Done (2026-11-08).** [ADR-0082](../adr/0082-nothing-to-release-exit-code.md). `versioning.ErrNothingToRelease`
is matched by every "no commits since" error (semver, fixed, maintenance and pre-release paths), by
`noReleasableCommitsError`, and by `app`'s `errNoTagsFound`. That is a `nothingToRelease` type that
keeps each message and identity, so `errors.Is(err, errNoTagsFound)` still works. `wrapRunErr` maps
it to the new `exitcode.NothingToRelease` (5), `version current` now classifies through
`wrapRunErr`, and `--json` reports `nothing_to_release`. Seven tests asserted exit 3 for these cases
and now expect 5, with their scenarios unchanged. New: Lane A `TestNothingToRelease_ExitsFive` (five
cases, all red before). CalVer with no new commit is T510 and was not touched; a pre-release sorting
below an existing one stays a runtime error. Docs: Spec 01's exit table, Spec 03's `error.code` list,
the exit-codes reference (a new section with the CI recipe), the skill, the AI-agents guide, and the
failure-recovery and next-version pages.

---

### Phase 96 — Behaviour fixes

#### `[x]` T507: `release --dry-run --json` runs the same gates

`Plan()` skips gate steps (`internal/pipeline/plan.go:46`): with `release.assets: ["../*.txt"]`,
`release --dry-run` exits 3 (`asset … resolves outside the repository`) while `--dry-run --json` returns
`ok` and exit 0. Spec 03: "The process exit code is the same with and without `--json`." Fix: run each
step's gate in `Plan()`.

**Done (2026-11-08).** `Plan()` runs each gate step as a dry run does. A gate's warning (an unmatched
asset pattern) goes through a new `Pipeline.warn`, which prints during a run and collects into
`plan.Warnings` while `Plan()` runs, since `--json` discards the pipeline's output. Tests:
`TestPlan_RunsTheGates` (a refused asset fails the plan; an unmatched one lands in its warnings) and
Lane A `TestReleaseDryRunJSON_RefusesWhatTheDryRunRefuses` (an asset beside the repository: exit 3 in
both modes; the old `Plan` exited 0 with `--json`). No doc change: Spec 03 already promised the same
exit code.

#### `[x]` T508: `heraut notes` matches the strategy's tags exactly

- `tagfmt.ParseVersion` turns `{env}` into `[^/]+` (`internal/versioning/tagfmt/tagfmt.go:60`), so
  `notes dev/0.1.0 --env prod` prints notes (exit 0); Spec 03 § notes says exit 3. Substitute the active
  env as a literal (`internal/app/notes.go:143`).
- `notes` refuses CalVer tags heraut itself creates with a trailing literal: format `YYYY.MM.PATCH-lts`
  makes `2026.11.0-lts`, then `notes 2026.11.0-lts` says `Not a release tag` (`notes.go:136,151`).
  Check with `calver.ParseVersion` against the configured format.

**Done (2026-11-08).** `isReleaseTag` reads a per-env tag through the effective format with the active
environment rendered in (`envVersion`), and checks a CalVer version with `calver.ParseVersion` against
`versioning.format` instead of the digits-and-dots regex. The shared extractors are untouched: they also
order tags, where the env glob already scopes them. Tests: an `isReleaseTag` table (four rows red
before) and Lane A `TestNotes_ReleaseTagShapes` (both red on the old code). Spec 03 already said both;
no doc change.

#### `[x]` T509: `version current` under CalVer picks the highest CalVer tag

`internal/app/current.go:86` returns git's first listed tag even when it isn't CalVer: a legacy `v1.2.3`
plus `2026.11.0` gives `v1.2.3` (also `--bare` and `--json`) while `version next` gives `2026.11.1`.
Spec 04: "Tags that do not parse are ignored." Fix: pick as `calver.Resolver.Resolve` does.

**Done (2026-11-08).** `globalCurrentTag` under CalVer now goes through `latestCalverTag`: tags whose
version parses under `versioning.format`, highest by calendar value (`CompareByFormat`); with none,
it is `no tags found`, exit 5 since T506. Three `current_test.go` fixtures had no `format`, which the
validator requires for CalVer; they now set one. Tests: two unit tests (a legacy tag and `latest`
ignored; only other tags gives nothing to release), and Lane A `TestCalVer_VersionCurrentIgnoresALegacyTag`.
No doc change: Spec 04 already said it.

#### `[x]` T510: **Decision** — CalVer with no new commit

**Decision (user, 2026-11-08):** require a commit, like SemVer: no commit since the current tag stops with `no commits since <tag>`, exit 5.

**Done (2026-11-08).** `calver.Resolver.Resolve` counts commits since the current tag (`git rev-list
--count <tag>..HEAD`), and the calver-per-env auto path reuses `fetchCommitsSince`; with none, both
return `versioning.NothingToReleasef("no commits since …")`. Fallout, all fixtures with a tag and
nothing after it: 18 calver and 5 perenv mock tests now answer the new git call; three e2e tests
(period boundaries, sprint, the option-shaped tag) and the foreign-prefix app test gained a commit
after their tag. What each checks is unchanged. New: a resolver test (zero commits refused with the
call's arguments, three allowed), a per-env one, and Lane A
`TestCalVer_ReRunWithoutACommitHasNothingToRelease` (no second tag, no second commit). Docs: Spec 04 §
CalVer, the next-version page (it said the opposite). The failure-recovery guide's promise now holds.

A second `changelog --tag` with no new commit makes `2026.11.1` on the same commit as `2026.11.0`
(`internal/versioning/calver/resolver.go:27-63`, and the per-env calver auto path). The failure-recovery
guide (states B and E, `:62`) promises a re-run "stops with `no commits since`". Recommendation: require
at least one commit since the current tag (consistent with T506); otherwise document the difference.

#### `[x]` T511: an identical re-run doesn't commit

**Decision (user, 2026-11-08):** the footer doesn't count: when only the footer would change, the file is left untouched (no write, no commit); the footer still updates whenever a section changes.

**Done (2026-11-08).** `writeChangelog` compares the file on disk with the new content up to the
footer anchor (`withoutFooter`); when they match it returns the file as it is, without writing. That
covers the incremental and full builds. The flaky test and the T360 retry test now run the test-clock
binary at `10:25:59Z` and then `10:26:00Z`, so each is its own cross-minute case; the retry test also
asserts there is one release commit. Both fail on the old code every time. New: a unit test (footer
alone: untouched; a new line: written with the new footer). Spec 05 § Changelog structure says it.

The changelog footer reads `_Generated by … at HH:MM on DATE._`, so a re-run a minute later rewrites the
file and makes a second `chore(release)` commit; the "nothing to commit" path only works within the same
minute. This also makes `e2e/changelog_flow_test.go:107-118` flaky (failed once in a full run, reproduced
with `HERAUT_TEST_NOW` at `10:25:59Z` then `10:26:00Z`), and lets the T360 retry test (`:215-232`) pass
for the wrong reason. Fix: ignore the footer when deciding the file is unchanged (or keep the old footer
when the body is identical); pin `HERAUT_TEST_NOW` in those tests and add a cross-minute case.

#### `[x]` T512: a rotating changelog reports degraded enrichment

`rotatingGenerator` (`internal/app/changelog_rotation.go:77-90`) doesn't implement
`Degraded()`/`DegradedReason()`: with enrichment failing, `CHANGELOG.md` shows `Generate changelog —
without enrichment` and two warnings, `CHANGELOG_{YYYY}.md` shows a plain `✓`. Fix: keep the inner
generator of the last `Generate` and forward both methods.

**Done (2026-11-08).** `rotatingGenerator` keeps the generator its last `Generate` built and forwards
`Degraded()` and `DegradedReason()` to it; before any `Generate` both report nothing. Test:
`TestRotatingGenerator_ReportsADegradedEnrichment`, with a forge whose `Enrich` fails (red before: the
wrapper didn't implement the interface). No doc change: the step output already documents the
degraded line.

#### `[x]` T513: hook templates are checked by `check config`

`validateHooks` (`internal/config/validate_hooks.go:12-50`) never parses the templates: with
`post_tag: [{run: "echo {{ .Tga }}"}]`, `check config` says ok, then `changelog --tag` creates and pushes
the tag and fails at the hook (`can't evaluate field Tga`). Fix: parse and execute each `run`/`stage`
template against the hook data struct during validation.

**Done (2026-11-08).** The hook data struct moved to `config.HookData`, and `pipeline`'s `hookVars` is
an alias for it, so validation and rendering use one definition. `validateHooks` parses and executes
every `run` and `stage` template against sample data; an error is reported at its path
(`hooks.post_tag[0].run`, `…stage[0]`) with the list of fields. Tests: a validation table (unknown field
in `run`, an unparsable `run`, unknown field in `stage`) plus every documented field staying valid, and
Lane A `TestHooks_ATemplateTypoStopsBeforeTagging` (`check config` and `changelog --tag` exit 2,
nothing tagged or pushed). Spec 02 § hooks says it. A stale comment claiming `pipeline` never imports
`config` was corrected.

#### `[x]` T514: `--verbose` doesn't print credentials from the origin URL

`internal/forge/origin.go:11` runs `git remote get-url origin` through the echoing runner (forge
`exec/runner.go:86-88`), so `--verbose` prints `https://oauth2:glpat-…@gitlab.example.com/…` in full.
Fix: run that call without echo, or redact userinfo (`url.URL.Redacted`) in `GitOriginURL`; check every
other place the origin URL is printed.

**Done (2026-11-08).** `resolveForge` reads the origin through `quietRunner`, a copy of forge's
`CmdRunner` with `Verbose` off; for the memoizing runner, a copy that shares its cache and mutex (now a
pointer), so the call is still made once. Test doubles pass through unchanged. Trade-off: forge logs a
command line under the same switch, so `git remote get-url origin` itself no longer appears in the
verbose log. Test: a real repo whose origin carries a token, resolved through a verbose runner and a
memoizing one, with the token absent from the log (red before). The reviewer's reproduction prints it
0 times. The other place the URL could appear is `git push`'s `To <url>` line on stderr, which
`--verbose` echoes; whether git strips the token there was not checked offline, so it is added to
T552.

#### `[x]` T515: GitHub "Latest" goes to the highest release only

**Decision (user, 2026-11-08):** heraut decides: `make_latest` is true only when the tag is the highest final release of its line (for per-env, of its environment's tags), false otherwise. No config key.

**Done (2026-11-08).** The GitLab driver is untouched. A new optional `port.LatestMarker` (`SetLatest`)
is implemented by the GitHub platform, which passes it to the client as `MakeLatest`. That is sent with
the request that publishes: the create call, or the publish update after the uploads (never on a draft
create). `pipeline.Config.IsLatest` is asked before publishing to a platform that implements the marker.
`app` builds it from `globalCurrentTag` (the line's highest final tag, per strategy and environment,
pre-releases excluded), run after the tag push. When it fails, nothing is sent and the step says so; a
final that isn't the highest gets a sub-line. Tests: client contract tests (create and publish
bodies, absent when unset), a platform test, a pipeline test (asked once, only for the platform that
has a Latest), an `app` real-git table (`v2.0.0` yes; `v1.0.1` and `v2.1.0-rc.1` no), and Lane A
`TestRelease_GitHubLatestOnlyForTheHighestRelease` (`true` for a first release, `false` for
`--set-version 1.0.0` below `v2.0.0`). Docs: Spec 05 § GitHub, the maintenance-branches guide.

`make_latest` is never set (`internal/platforms/github/client.go:86,139`; no `MakeLatest` in
`internal/`), and GitHub's default is `true`, so a maintenance patch published after a newer major, or a
dev/uat per-env release, becomes "Latest". Fix: send `make_latest: "false"` when the version isn't the
highest final release of the strategy (or `"legacy"`), with an httptest contract assertion; document it.

#### `[x]` T516: `forges[].api_url` reaches publishing, or is documented as enrichment-only (needs the go for the GitLab driver)

**Decision (user, 2026-11-08):** publish through it too, on GitHub and GitLab; the user gave the go for the GitLab driver change this needs.

**Done (2026-11-08).** `config.Platform` gains `APIURL`, which `platformConfigFromTarget` copies from the
forge's explicit `api_url`. GitHub: `newClient` takes it as the API root (https or loopback only)
instead of deriving one. GitLab, the one driver change approved: a new `apiBase()` (api_url, else the
base URL) feeds both client constructions, while `resolveBaseURL` still serves links and the job-token
host check. Tests: one per driver (requests reach the api_url host, release links keep `base_url`;
red before) and a wiring test. The three GitHub client tests calling `newClient` pass `""`. Docs:
Spec 02's `api_url` row, the schema description (so the generated reference).

`platformConfigFromTarget` (`internal/app/platforms.go:49-69`) never copies the identity's API URL, so
`api_url` changes enrichment but not publishing on either platform. Spec 02 calls it an "API host
override" and says enrichment and publishing "always query the same instance". Pick one and make the
code, spec and reference agree.

#### `[x]` T517: Azure PR links escape project and repository names

`internal/forge/azure/azure.go:62`, `prquery.go:96` build the PR URL unescaped; a project or repo name
with spaces (common on Azure) gives `…/My Project/_git/My Repo/pullrequest/12`, a broken Markdown link.
Fix: `PathEscape` each segment, as the generator's `azureRepoRoot` does.

**Done (2026-11-08).** `webBase` escapes each segment of the project path and the repository name, so
the PR link (`prquery.go` builds on `webBase`), `ChangeURL` and `CommitURL` are all escaped. Test: names
with spaces give `%20` in both URLs (red before). Existing URL tests (plain names) are unchanged.

#### `[x]` T518: per-driver `rendering.excludes` are validated

`validateContentDriver` (`internal/config/validate_rendering.go:151-168`) validates no excludes:
`changelog.rendering.excludes: [{regex: "("}]` or `[{type: chore, regex: x}]` pass `check config`, and
`NewExcludeMatcher` skips the bad regex silently. The top-level validation (`:19-31`) has no test at
all, and `excludes.go:14-15` claims "the validator rejects it upstream". Fix: validate excludes for
changelog, notes, targets and environments; table tests for the three messages.

**Done (2026-11-08).** The top-level checks moved into `validateExcludes(excludes, path)`, which
`validateContentDriver` now also runs. So every driver it validates (`changelog`, `release.notes`, an
environment's changelog and release notes) reports `<path>.rendering.excludes[i]`. Targets have no
rendering block. Test: a table of three blocks × three faults (no type or regex, both, a regex that
doesn't compile); the six per-driver rows were red before, and the top-level rows give that block its
first tests. The comment in `excludes.go` ("the validator rejects it upstream") is now true.

#### `[x]` T519: the update hint stays quiet where heraut promises no network

**Decision (user, 2026-11-08):** no update check on `--dry-run`, `--offline`, `--json`, or `check config`.

**Done (2026-11-08).** `withUpdateHint` wraps forge's `Hinter.PostRun`. It skips the hint when the command
defines `dry-run`, `offline` or `json` and the flag is set, and for `heraut check config`. It takes the
hint as an argument, so the test counts calls instead of touching the network: six commands, four
quiet (`check runtime` still gets it). Docs: Spec 03 § Background update hint, install.md § Upgrade.

`internal/cmd/root.go:50` runs forge's update hint after every command, including `--dry-run`,
`--offline`, `--json` and `check config`, and it calls the GitHub API when its cache is stale. Spec
01:88-90 says a dry run makes no network calls; Spec 03:671 calls `check config` offline only. Fix: skip
the hint for those, or document the exception in both specs.

#### `[x]` T520: the Docker image works on Linux bind mounts (CI/CD: needs the go)

**Decision (user, 2026-11-08):** go for the one `safe.directory` line in the Dockerfile, verified with a local build.

**Implemented, not yet verified (2026-11-08).** The final stage runs `git config --system --add
safe.directory '*'`, with a comment saying why; hadolint passes. Still open: the Docker daemon wasn't
running here, so the planned check (build the image, run `heraut check runtime` on a bind mount owned
by another uid on Linux) hasn't run. Tick this task once it has.

**Verified (2026-11-08).** Built both images locally. Docker Desktop maps bind mounts to root and
hides the problem, so the check creates a repository inside the container, `chown`s it to uid 1000 and
runs `heraut version next` as root. The image without the line fails as the reviewer reported
(`detected dubious ownership in repository at '/repo'`, exit 3); the fixed image prints `v0.1.0`, exit
0. The documented `docker run -v "$PWD":/repo -w /repo` usage also works. Test images removed
afterwards.

The image runs as root with no `safe.directory` (`Dockerfile:32-41`); a bind-mounted repo keeps the host
uid on Linux, and git refuses it (`detected dubious ownership`), so the `docker run -v "$PWD":/repo`
usage in `docs/install.md:78` fails. Docker Desktop on macOS hides it. Fix:
`git config --system --add safe.directory '*'` in the final stage; verify with a Linux run.

#### `[x]` T521: **Decision** — `revert` commits

**Decision (user, 2026-11-08):** add `revert` as a built-in type (patch bump, its existing section).

**Done (2026-11-08).** `revert` joins `defaultTypes` (render `◀️ Revert`, order 9, after Miscellaneous
Tasks), so `commit verify` accepts it and it bumps a patch. The renderer's own Revert fallback (order
102) stays for a config that removes the type, now with a test. Two tests pinned the old built-in
set and the section's position (102 → 9) and were updated. New: a config test, Lane A `TestRevertType`
(verify accepts it; `revert:` after `v1.0.0` gives `v1.0.1`). Docs: Spec 02 § commits.types, the
commit-conventions guide, and an addendum to ADR-0075. The changelog-content page already showed the
section.

`docs/concepts/changelog-content.md:45` shows a `◀️ Revert` section, but `revert` isn't an allowed type:
`commit verify "revert: undo x"` exits 1 (`commit-conventions.md:104` agrees with the binary). Options:
add `revert` to the default types (it already has a section) or drop the row.

#### `[x]` T522: config errors name the key and suggest the fix

An unknown key gives `Config: line 4: field tag_prefx not found in type config.Versioning.`: a Go type
name and no hint, while `getting-started.md:61-62` promises "its line and a hint". Fix: name the YAML
path, suggest the closest known key, drop the Go type.

**Done (2026-11-08).** The loader decodes with its own strict decoder (`decodeConfig`, the same rules
as forge's `config.Decode`, which flattens yaml's `TypeError` into a string). Each unknown-field entry
becomes `line 4: unknown key "tag_prefx" in versioning — did you mean "tag_prefix"?`. The path comes
from the YAML node tree (`forges[0]`, or "at the top level"); the suggestion is the closest key of that
config type within two edits, found by reflection over the `Config` structs. Other errors pass through
unchanged. Two tests quoted the old Go-type wording (`TestLoad_NotesRejectsOutputAndTagPattern`, e2e
`TestCheckConfig`); they now expect the new text and still check what they did. New: a table (a typo in
`versioning`, at the top level, in a list entry, and one with no close key). The getting-started page
quotes the real output.

---

### Phase 97 — Tests that give false confidence

#### `[x]` T523: `internal/app` wiring tests assert what they wire

These tests assert only `NotNil`, and mutations of what they wire survived every layer:
- `pipeline_test.go:108` `TestBuildPipeline_LightweightTagType`: `AnnotatedTags = true` survived
  (`pipeline.go:334`).
- `:117` `TestBuildPipeline_PerEnvDisableFlags`: `DisableChangelog = false` survived (`pipeline.go:260`).
- `:150` `…ReleaseAssets_PropagatesToPlatforms`: `if len(releaseAssets) > 0` survived (`pipeline.go:398`),
  i.e. `release.assets` silently replacing a target's own assets.
- `:138` `TestBuildChangelogPipeline_PerEnvDisable` and `:98` `AnnotatedTagsDefault` (caught by e2e only).

Assert the built config, as `targets_internal_test.go` does for `disable_release`.

**Done (2026-11-08).** New internal tests assert the built config: `tag_type` → `AnnotatedTags` (default,
annotated, lightweight), per-env `disable_changelog` and `disable_release` in the release pipeline, both
in the changelog pipeline, and the target assets rule. For the last, the per-target step of
`buildTargetPlatforms` moved into `targetPlatformConfig`, since a built platform doesn't expose its
assets. The four mutants that survived the review (`AnnotatedTags = true`, release `DisableChangelog
= false`, `release.assets` replacing a target's own, changelog `DisableChangelog = false`) are now
killed; an equivalent condition survived as the control. The `NotNil` tests stay; they still prove
the build succeeds.

#### `[x]` T524: cover the guards no test executes

- `internal/generators/native/generator.go:453`: "nothing new, leave the file untouched"; the mutant
  `if false && newBody == ""` survived (it writes an empty anchor and a new footer).
- `internal/versioning/semver/maintenance.go:80-82` and `prerelease.go:117-119`: "no releasable commits";
  mutants survived, and for the pre-release path the user then gets "would sort below existing v1.2.0"
  instead of "No releasable commits".

**Done (2026-11-08).** Three guards are now tested, each mutant (`if false && …`) killed:
- the changelog's "nothing new": a native test where the only commit since `v1.0.0` is a dependency
  bump leaves the file byte-identical;
- the maintenance and pre-release "no releasable commits": Lane A scenarios (`Update readme` alone on
  `release/1.3`, and before `--pre-release rc`), each exiting 5 with `no releasable commits since …`.

No production change.

#### `[x]` T525: tests don't depend on the developer's environment (one file is a GitLab driver test: needs the go)

- `HERAUT_FILE` exported → 25 tests fail in `internal/cmd` (`notes_test.go:68` and others); it isn't in
  `config.CIEnvVars` / `testutil.ClearCIEnv` (`internal/config/forgeenv.go:49-53`,
  `internal/testutil/env.go:13`).
- GitLab CI variables set → `internal/pipeline/release_test.go:202-207` (a hand-rolled
  `clearAmbientCIEnv`) and `internal/platforms/gitlab/platform_test.go:25-37,113` fail. Use
  `testutil.ClearCIEnv(t)`.
- `testutil.RealGitRepo` (`realgit.go:20-31`) doesn't isolate git config: a global `commit.gpgsign` or
  `core.hooksPath` fails 10 tests. Set `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1`.

**Done (2026-11-08), with the user's go for the GitLab driver test file only.** Reproduced first:
`HERAUT_FILE` exported failed 25 tests, GitLab CI variables 7, a signing global git config 13.
`testutil.IsolateProcessEnv` (`GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `HERAUT_FILE`
unset) runs from a new `TestMain` in `app`, `cmd` and `commitwizard`, the packages that run real git;
`RealGitRepo` applies the same per test. The pipeline's `clearAmbientCIEnv` and the GitLab tests'
`notInCI` and eight `t.Setenv("GITLAB_CI", "")` lines now call `testutil.ClearCIEnv`. After: 0
failures in each of the three environments. No production change.

#### `[ ]` T526: the schema and the Go loader can't drift unnoticed

- `testdata/config/valid/` has no `azure_devops` forge fixture, which `testing.md` requires.
- `schema_test.go:64-97` runs invalid fixtures against `schema.json` only; `loader_test.go:296-313` loads
  4 of 21 valid fixtures. Go accepts `confirm_major_not_boolean.yml` (`confirm_major: "yes"`, and `"off"`
  reads as false): fix the loader to refuse non-boolean strings.
- Run every fixture through `Load` + `Validate` too, with an explicit list of schema-only cases.

#### `[ ]` T527: Lane A releases under every strategy

Every Lane A `release` uses `strategy: semver`. Add scenarios against the fake forge: `release` under
calver (testclock binary), `release --env` under semver-per-env (auto and promote) and calver-per-env,
and `release` with `disable_release` / `disable_changelog`.

#### `[ ]` T528: failures after the tag push are tested

Uncovered: `internal/pipeline/steps.go:293-296` (notes generation fails after the tag push, maybe after
an earlier platform published) and `changelog.go:198-200` (`git tag` fails after the changelog commit
was pushed). Pipeline tests assert the error text, the steps already done and the exit code, and match
the failure-recovery guide's states.

#### `[ ]` T529: small contract and assertion gaps

- `internal/app/notes.go:73` (`merge-base --is-ancestor`) and `:126` (`show-ref --verify --quiet`) have
  no argument-asserting test; the non-exit-1 path at `:78` is uncovered.
- `generator.go:513-531`: the atomic write's cleanup on write, sync, close or rename failure.
- `internal/cmd/version_test.go:426,443,462-463`: `assert.Contains(out, "6")` on sprint output; assert
  the whole line.

---

### Phase 98 — User docs

#### `[ ]` T530: guide — adopt heraut on an existing project

No page covers it. Existing tags and their prefix (T497), a `CHANGELOG.md` from another tool
(`heraut changelog` fails with exit 3, `no heraut-release anchors … run heraut changelog --regenerate`,
and `--regenerate` replaces the whole file, including hand-written text), and the mapping from
semantic-release, release-please and git-cliff. Linked from getting-started and the guides index.

#### `[ ]` T531: the configuration reference shows defaults and what's required

The Default column is empty for all keys but three, and nothing is marked required, while
`docs/README.md:51` and the sample promise "every key, with its type and default". Add `default` values
to `schema.json`, a Required column to `tools/docgen`, regenerate.

#### `[ ]` T532: quoted output matches the binary

- `getting-started.md:66-80`: real `check runtime` output also prints the working-tree and history rows.
  Re-run and check: the reviewer's run failed, so read why before quoting it.
- `getting-started.md:127-151`: the quoted changelog lacks `<!-- heraut-footer -->` and the footer.
- `guides/github-actions.md:122`, `gitlab-ci.md:108`: quote heraut's preflight message
  (`git user.name is not configured; run: …`), not git's.

#### `[ ]` T533: when commit links and PR data appear

- `concepts/changelog-content.md:99-100`, `guides/mobile-ci-tagging.md:134-135`, `getting-started.md:149`
  and the `--offline` help say hashes are plain outside CI. Real rule: linked whenever a forge is known
  (git origin or CI) unless `--offline` / `enrichment_policy: disabled` removes them; in CI the links stay
  even with `--offline`. Say it once, the same way everywhere.
- `guides/pr-data-and-tokens.md:104` says "no warning, and no links" with no token; a GitHub origin with
  no token prints `! remote enrichment unavailable … 403` and still links.

#### `[ ]` T534: fact fixes

- `reference/exit-codes.md:19`: `--set-build-id` without `--set-version` exits 2, not 1.
- `SKILL.md:22`, `guides/ai-agents.md:41`: `--offline` exists only on `release`, `changelog`, `notes`
  and `check`; `version next --offline` is an unknown flag.
- `heraut.sample.yml:60-62`: an unrecognized type is a patch, not "doesn't contribute".
- `heraut.sample.yml:498`: `tag_pattern: "v[0-9]*"` is a regex, use `"^v[0-9]"`.
- `guides/failure-recovery.md:45-46`: a `chore(release)` commit *is* releasable; give the real reason.
- `guides/environments.md:36-41`: add the `git switch` steps the branch guard needs.
- `getting-started.md:13`: `GH_TOKEN` is read before `GITHUB_TOKEN`; say which wins.
- `guides/github-actions.md:25`, `commit-conventions.md:71`: confirm `actions/checkout@v7` exists, or
  use the version heraut's own workflows pin.
- `concepts/how-heraut-works.md:19-28`: the "Check" row isn't a numbered step.
- `concepts/versioning-strategies.md:73`: not every command takes `--env`.
- `heraut.sample.yml:11-14`: one schema-URL convention across the docs, and a real URL in the sample.

#### `[ ]` T535: FAQ and troubleshooting

A page indexed by message and exit code, linked from `exit-codes.md`: nothing to release (with T506's
outcome and a CI recipe), monorepos (commits aren't filtered by path; say so plainly), Windows (a
PowerShell install and PATH block in `install.md`; hooks run under `cmd`).

#### `[ ]` T536: plain-language pass

- `guides/template-customization.md:85`: a garbled sentence; contributor jargon there and in the sample
  (`<driver>.template`, "both drivers", "data contract", "root `Release`").
- `guides/maintenance-branches.md:15-16,24,34,97-98`: changelog voice ("as before", "today's
  behaviour", "This bounds-fix…").
- Help text (`internal/cmd/*.go`): `--pre-release` ("mint … <core>-<label>.<N>"), `--allow-major`,
  `version current --force`, `--regenerate` / `--regenerate-changelog` (say they replace the file), and
  `commit tickets` ("Validate", but it never fails).
- `reference/configuration.md:115`: `release.notes` reuses the changelog's description.

---

### Phase 99 — Contributor docs

#### `[ ]` T537: specs say what the code does

- Spec 03:297-298 recommends `release --set-version <tag>` after `changelog --tag`; it fails with
  `tag already exists` (ADR-0073). Point to failure-recovery state C.
- `--offline` called global: CLAUDE.md:202, Spec 02:860, `internal/cmd/offline.go:9`.
- Spec 02:1027, Spec 05:538: `project: $CI_PROJECT_PATH` is never expanded (the literal reaches the API
  URL); show `acme/app`.
- Spec 03:138, :418 call config errors "usage errors"; Spec 01:101 says Usage = 1, Config = 2.
- Spec 06:63-66 says the README links into `docs/specs/` (forbidden now); Spec 06:171-177 describes the CI
  `build` job wrongly and omits `e2e.yml` (CLAUDE.md:131-133 too).
- Spec 01:88 and 06:25: not every side-effecting command has `--dry-run` (init, skill install,
  notes --output).
- Spec 02:41-66: the top-level key table lacks `notes`.
- Spec 01:42,57 describe `internal/cmd/` as `cmd/heraut/`.
- Leftovers: `internal/pipeline/linkctx.go:16` ("git-cliff's linkEnv"), Spec 03:282 ("mirrors
  `cog bump`"), Spec 05:25 ("matching git-cliff").

#### `[ ]` T538: ADR statuses and amendments

- 0022, 0024, 0040: status line says Accepted, the index says superseded (by 0045, 0033, 0043).
- 0034:14, 0043:26-28, 0044:71,101 and index rows `README.md:41,50,51` still say `gh`/`glab` publish:
  add "Amended by 0069/0077".
- 0018: `--version`, cliff config, `gh release create`, Docker on `push: tags` are all stale, and four
  docs cite it as the release-pipeline reference: add an amendment describing the current workflow.
- 0015 is Rejected, but its decision shipped through forge (`forgelog.New`, `release.go:123`).
- 0006 lacks the supersede note the index shows; index line 13 credits `release.targets` to 0043
  (it's 0044).

#### `[ ]` T539: CLAUDE.md, rules and indexes

- CLAUDE.md:17: release order is "… → tag → notes + publish per target".
- CLAUDE.md layout: `heraut commit tickets` (lines 26, 85); `.goreleaser.yml` "no archives" (it has
  two); missing `internal/platforms/{assets,globs}.go`, `docs/audit/`, `tools/docgen/`, `e2e/`;
  `cli.Run(ctx, root, Version, ui.Accent())`; goldmark in the stack.
- `workflow.md:22-35`: add `security` (built-in since ADR-0075).
- `coding.md` layer table: `tools/docgen` (imports `internal/cmd`).
- `docs/tasks/README.md:5` ("Phases 1–31") and its table (missing hardening-2…6, hook-file-staging,
  release-hooks, semver-v2, this file); CLAUDE.md:45-47 likewise.
- Epic headers still "Status: Active" though done: changelog-rotation, docs-audit, forge-abstraction,
  hook-file-staging, native-generator, release-config.

#### `[ ]` T540: roadmap housekeeping

- `roadmap.md:11,39` ("63 ADRs"), `:19-21` (wraps `gh`/`glab`), `:86-153` (dependency graph and testing
  principles naming `internal/adapter/exec`, gitcliff, cocogitto, selfupdate, `testutil.MockRunner`):
  delete or mark historical; stop repeating counts.
- `:264` "outside Phase 53", `:283` "three-section TUI", `:227` Phase 56 "not yet exercised", the Risks
  table's self-update row.
- `forge-abstraction-roadmap.md:51-66`: T164 has no marker; close it as superseded.
- The unmerged `worktree-monorepo-support` branch reuses T261: add a pointer in `docs/ideas.md`, note the
  ID clash.
- `.claude/notes/mid-flight-failure.md`: answered since; delete or mark resolved.

#### `[ ]` T541: package doc comments

`go list -f '{{.Doc}}'` is empty for app, cmd, config, pipeline, platforms/*, port, scaffold, testutil,
ui and versioning/*. One sentence each, saying what the package owns.

---

### Phase 100 — Remaining review items

#### `[ ]` T542: push the tag by its full ref

`internal/pipeline/git.go:131-136` pushes the bare name: on a branch named `v7.8.0` (allowed by
`branches: [{name: "v*"}]`), `Push tag` fails with `src refspec v7.8.0 matches more than one`. Push
`refs/tags/<tag>`.

#### `[ ]` T543: step messages say what happened

- No `changelog:` block: `heraut changelog` and `--commit` print "Changelog updated for v0.1.0" (exit 0)
  with no file written (`internal/pipeline/changelog.go:298`).
- "CHANGELOG.md unchanged, HEAD pushed" when no push ran (`:305`).
- The `changelog --commit` warning says "continuing to tag and release" (`:314`, `release.go:170`).

#### `[ ]` T544: `version sprint bump` refuses configs without `SPRINT`

On plain `semver` it writes `sprint: 1` and reports success (`internal/cmd/version_sprint.go:41-56`);
Spec 03 limits it to CalVer formats with `SPRINT`. Config error, dry run included.

#### `[ ]` T545: remove code no binary path reaches

A reporter is always installed (`internal/app/pipeline.go:101,173`), so every `reporter == nil` branch
is dead: each step's `plain` closure, `release.go:131` "[dry-run] would release", the plain summaries.
Also test-only: `pipeline.shouldRunHooks` (`hooks.go:87`), `semver.Resolver.SetVersionOverride` and the
override branch of `resolveManual`; `Generator.Validate()` has no production caller (Spec 05:421).

#### `[ ]` T546: a clear error on a repository with no commits

On an unborn branch, `version next` prints `v0.1.0` and `changelog` fails with
`listing git tags: … fatal: malformed object name HEAD` (exit 3). Say "no commits yet".

#### `[ ]` T547: the changelog is written inside the repository

`changelog.output: ../../escaped.md` writes two levels up, and a committed symlink `CHANGELOG.md ->
../victim.txt` (empty target) gets the changelog written through it. Open through `os.Root` on the repo
root and refuse symlinks, as `OpenAssets` does.

#### `[ ]` T548: publishing survives a transient error (needs the go for the GitLab driver)

Publishing runs after the tag push with no retry or idempotency check
(`internal/platforms/github/client.go:94`, `gitlab/client.go:107`): a 5xx or timeout leaves state C, and
a retry after a create that actually succeeded hits `already_exists`. On error, look up the release by
tag before failing; retry 5xx with backoff.

#### `[ ]` T549: the preflight check proves write access (needs the go for the GitLab driver)

`VerifyReleasesAccess` (`github/client.go:146`) and `VerifyUser` (`gitlab/client.go:121`) prove read
access only, so a read-only token passes and fails after the tag push; the GitLab hint says "api scope"
but `/user` accepts `read_user`. Check push permission (GitHub `permissions.push`) and token scopes
(GitLab `personal_access_tokens/self`).

#### `[ ]` T550: CI hardening (CI/CD: needs the go)

- `.github/workflows/ci.yml` has no `permissions:`: add `contents: read` at the top.
- `.goreleaser.yml` builds without `-trimpath` (the Dockerfile has it): add `flags: [-trimpath]`. The
  ldflags themselves are identical, as CLAUDE.md requires.

#### `[ ]` T551: detect more origin URL shapes

`parseGitOrigin` (`internal/forge/detect.go`) misses `ssh://git@github.com/acme/app.git`, a host with a
port, and mixed-case hosts, so zero-config detection silently goes offline. Parse with `net/url`,
normalise the host. (Found by reading; reproduce first.)

#### `[ ]` T552: settle the review's unverified suspicions

Reproduce each; fix it as its own commit or record why it holds:
- GitLab Generic Packages may reject some asset file names (spaces, `#`, non-ASCII) at upload, after the
  tag push; the GitLab preflight applies no rule.
- `associatedPullRequests(first:1)` (`forge/github/graphql.go:21`) may pick an open or unmerged PR, or
  one on another branch.
- GitLab GraphQL `commits{nodes{sha}}` per MR uses the default page size (MRs over ~100 commits).
- One `errors[]` entry fails a whole 50-SHA GitHub GraphQL chunk.
- The `docker-build` job checks out the dispatched commit, not the release commit (only the embedded
  `CHANGELOG.md` differs).
- A CRLF commit message fails `conventionalcommit.Parse`.
- `calverOrder` sorts with `CompareVersions`, not `CompareByFormat` (formats not leading with `YYYY`).
- `NewResolver` errors are all mapped to Config (`internal/cmd/release.go:158`), git failures included.
- `flagReader.Err` maps an undeclared-flag programming error to Runtime (3) instead of Internal (70).
- `git push` prints `To <url>` on stderr, which `--verbose` echoes: check whether git keeps a token from an
  origin URL there (found during T514).

#### `[ ]` T553: interactive packages get real coverage

`internal/commitwizard` 51%, `internal/scaffold` 67%, `internal/cmd` 79%; `whatsnew` is tested only for
registration (`root_test.go:38`) and its offline fallback has no e2e. Cover the decision logic behind the
forms (not the forms themselves) and the `whatsnew` fallback.

#### `[ ]` T554: the review's own environment notes

Two review agents made unintended network calls (tokenless read-only calls to api.github.com, and a DNS
miss on `gitlab.example.com`): find which commands still reach the network without `--offline` in a
fresh repo, and make sure the docs and `check` say so. Close with a note if T519 and T533 covered it.

---

#### `[ ]` T556: per-env changelog headings strip a literal env prefix too

Found during T496. With `tag_format: "{env}/{version}"` a per-env changelog heading reads
`## [2026.11.0]`; with a literal `prod/{version}` it reads `## [prod/2026.11.0]`, because
`tagfmt.DeriveHeadingVersionPattern` only strips around `{env}` / `{build}`. Derive the heading pattern
from the env's effective format whatever its shape, so both read the same; check the release notes and
`heraut notes` headings too.

---

### Phase 101 — Cut v1.0.0

#### `[ ]` T555: write the cut procedure, then cut

The last open item, `roadmap.md` checkpoint K ("v1.0.0 cut by running `heraut release` on the heraut repo
itself"), has no steps. heraut's own `.config/heraut.yml:10` sets `stay_at_v0: true`, so a normal run never
reaches 1.0.0. Write the steps under the checkbox: run the Release workflow with `version: v1.0.0`; then
drop `stay_at_v0` (it does nothing after 1.0) and decide on `confirm_major: true`; switch to the post-v1
branching rule in `workflow.md`. Cut only on the user's go, then tick checkpoint K.
