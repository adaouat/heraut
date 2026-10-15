# Héraut — Hardening and Refactor Roadmap

> Status: Active (T400 open, then Phase 73)
> Source: a read-only security, rules and structure review of the code base (2026-10-14), approved
> as a four-phase in-place refactor. Main roadmap: tracked as Phases 70–73 in [`roadmap.md`](roadmap.md)

The review found no Critical or High security finding, a clean `govulncheck`, and a well-layered
code base. What remains is a handful of Low and Low-Medium security findings, six business rules
whose current behavior the maintainer decided to change or to document, and structural debt in a few
large files. This epic fixes them in that order: security first, then correctness of versions and the
release flow, then structure.

## Conventions

- Task IDs **continue the global sequence** (`T380+`).
- This file is the **single source of truth** for task status. Same checkbox markers as the other
  roadmaps: `[ ]` not started, `[x]` done. Follow the two-step flow
  ([`workflow.md`](../../.claude/rules/workflow.md)): failing test first, then flip `[ ]` → `[x]` and
  add a one-paragraph completion note.
- One task per session. A phase starts only when its entry conditions below hold.
- Phases 71–73 are outlined, not yet broken into final test lists: refine a phase when it starts.
  The review that produced this epic is expected to be revised by what Phase 70 shows.
- No test row is deleted to make a change easier. A test whose behavior is deliberately changed
  needs an ADR first.
- Wire names in `internal/config` do not change (a rename needs an ADR).

## Baseline (before Phase 70)

Recorded at `71acdc5`, Go 1.27.2: `go test -count=1 ./...` passes in every package; total statement
coverage **88.1%** (the CI gate is 85%). Phase exit criteria include "the same command still passes
and coverage does not drop below the gate".

## Decisions taken

| Topic | Decision |
|---|---|
| CalVer dates | Use UTC |
| CalVer latest tag | The highest parsed value, not git's tag order |
| `--set-version` | Always probe for an existing tag |
| Changelog write | Atomic (temp file plus rename) |
| Hook variables | Stay unquoted; the hook author quotes. Document it, and warn that CI must not run hooks from an untrusted pull request's config |
| Failure recovery | No rollback and no `heraut recover` command. Write a recovery guide |

## Progress at a glance

| Phase | Theme | Tasks | Status |
|---|---|---|---|
| 70 | Forge and publish hardening (pilot) | T380–T382, T398 | Done |
| 71 | Versioning correctness | T383–T386, T399 | T399 open |
| 72 | Pipeline, changelog write and recovery docs | T387–T390, T400 | T400 open |
| 73 | Structural debt | T391–T397, T401 | Not started |

---

### Phase 70 — Forge and publish hardening

Goal: close the three forge-side security findings. Entry: baseline above recorded. Pilot unit:
T380; what it shows may reorder the rest.

#### `[x]` T380: do not forward GitLab token headers on a cross-host redirect

`internal/forge/gitlab` builds an `http.Client` with no `CheckRedirect`, and Go forwards custom
headers (`PRIVATE-TOKEN`, `JOB-TOKEN`) when a redirect changes host. Add a `CheckRedirect` that drops
the token headers when the host changes. Failing test first: an `httptest.Server` that redirects to a
second host, asserting the second host never sees the header. Also check the same client shape in the
GitHub and Azure enrichment clients and the client-go transport in `internal/platforms/gitlab`
(whether client-go follows redirects with the token is not yet verified): add a test for each client
that is affected, or record that it is not.

**Completion note:** both GitLab clients were affected and are fixed. `internal/forge/gitlab` (the
enrichment client) and `internal/platforms/gitlab` (client-go, which sets `Job-Token`/`Private-Token`
per request on a default pooled client) each now carry a `CheckRedirect` that deletes the token
headers when a redirect leaves the original host, compares `URL.Host` against the first request, and
keeps net/http's default limit of 10 (a custom `CheckRedirect` replaces it, so a test pins it). The
enrichment client applies the policy to a copy of any client passed to `New`, unless the caller already
set its own `CheckRedirect`; the publish client sets it on the client-go HTTP client after
construction. The helper is duplicated rather than shared because `internal/platforms/*` may not
import `internal/forge/*`. GitHub and Azure enrichment authenticate with the `Authorization` header
only, which net/http already strips on a cross-domain redirect, so they are not affected and got no
test. A redirect from `https` to `http` on the same host still forwarded the token; T381 closed it.

#### `[x]` T381: require https for forge base URLs and compare scheme in `sameGitLabHost`

`base_url` and `CI_SERVER_URL` currently accept `http://` and any host, and `sameGitLabHost` ignores
the scheme, so a changed `base_url` can aim the token at another host. Require `https` except for
loopback, compare scheme and host, and say the policy in the validator hint. Decide before coding
whether an explicit opt-out for internal HTTP servers is needed; if so, it is an ADR and a new config
field with `schema.json` and `docs/heraut.sample.yml` updated together.

**Completion note:** decided with the maintainer: https is mandatory and there is no opt-out, so no
ADR and no new config field. The one exception is a loopback host (`localhost`, `127.0.0.0/8`, `::1`
over http), kept for local servers and because the test suite runs against `httptest`. A new
`config.IsSecureURL` is the single rule, used at three points: the validator refuses an `http://`
`forges[].base_url` or `api_url` with a hint (the ticket URL check is unchanged, it carries no
token); `forge.Resolve` refuses an identity whose host or API URL is plain http after config, CI
variables (`CI_SERVER_URL`, `CI_API_V4_URL`) and defaults are merged; and the GitLab publish
`NewClient` refuses a plain-http base URL before building the client-go client. `sameGitLabHost` now
compares scheme and host, so the job token is not trusted for an http URL when the runner is https.
Spec 05 and the `schema.json` descriptions say so. Both redirect policies (T380) now also drop the token on a same-host `https` to `http` redirect,
pinned by tests that call `CheckRedirect` directly because two test servers never share a host.
Not covered: the GitHub driver, which hands the host to `gh` through `GH_HOST`; where `gh` picks the
scheme is not verified.

#### `[x]` T382: confine release asset globs to the repository

`release.assets` globs are not confined, so an absolute path or `..` can publish files from outside
the repository. After expansion, resolve symlinks and refuse any path outside the repository root
(warn first if that is judged too abrupt, then enforce). Prefix `./` before passing an asset path to
`gh`, so a name starting with `-` cannot read as an option. Applies to `internal/platforms/globs.go`
and both drivers.

**Completion note:** refused outright, as decided with the maintainer, with no warn-first stage.
`platforms.ResolveGlobsLenient`, shared by both drivers, resolves the working directory (the
repository root, since asset globs are relative to it) and, for every matched file, makes the path
absolute and then resolves symlinks before comparing it to the root, so an absolute path, `../` and a
symlink that leaves the tree are all refused with an error naming the file. Making the path absolute
before resolving matters: a working directory reached through a symlink (`/var` on macOS) otherwise
compares unequal to itself; a first version had that bug and a test caught it. A pattern that matches
nothing still only warns. A relative match whose name starts with `-` is returned as `./name`, so it
cannot read as an option to `gh`. Existing asset tests now `t.Chdir` into their temp dir (a setup
change; no assertion was touched). Spec 03, the sample config and the `schema.json` descriptions say
so. Known limit, to decide separately: the refusal happens when the release step runs, which is
after the tag has been pushed (the pipeline order, RULE-061), so a bad asset path leaves a tag with
no release. A preflight check of the asset globs, before anything is pushed, would close that.

#### `[x]` T398: check the release assets before anything is pushed

Follow-up to T382, raised by its known limit. Asset refusal happened in the publish step, after the
changelog commit and the tag push (RULE-061), so a bad asset path left a public tag without a release.
Add `CheckAssets()` to `port.Platform`, implemented by both drivers with the T382 rules, and call it
for every platform right after the `post_bump` hooks. Failing tests first: a pipeline run with a
refusing platform makes no runner call, writes no changelog and creates no release, and both drivers
refuse an asset outside the repository.

**Completion note:** `platforms.CheckAssetGlobs` runs `ResolveGlobsLenient` with a silent warn
callback, and each driver's `CheckAssets` calls it. `Pipeline.Run` checks every platform after the
`post_bump` hooks (so a hook that builds the files has run) and before the dry-run return, so
`--dry-run` reports the refusal too; the error is wrapped as `platform <name>: ...`. A pattern that
matches nothing passes the preflight on purpose, since a `pre_tag` or `post_tag` hook may still
produce the files; publishing still warns and re-checks every match it finds. Not covered: files a
later hook creates are only checked at publish time, so an asset a `pre_tag` hook symlinks outside
the tree still fails after the tag exists. `port.Platform` gained a method, so `MockPlatform` got a
`CheckAssetsErr` field. Spec 03 and Spec 05 say so.

---

### Phase 71 — Versioning correctness

Goal: fix the versioning behaviors the maintainer decided are wrong. Entry: Phase 70 done. Pin
today's output with characterization tests before changing it; existing edge-case rows stay.

#### `[x]` T383: per-env `latestTag` returns only a tag that parsed

`latestTag` returns the first raw tag even when it does not parse. In promote runs that tag can reach
the hook variable `PreviousTag`, so a pushed tag containing shell syntax would run when a hook uses
it. Return a tag only when it parsed (`internal/versioning/perenv/order.go`, `promote.go`). Test with
a tag containing `$(…)` under both per-env strategies.

**Completion note:** `latestTag` (`internal/versioning/perenv/order.go`) now returns a tag only when
it holds a real version, and an empty tag otherwise. Before, it returned git's first line even when
nothing parsed, and for calver-per-env "parsed" was not enough either: `tagfmt.ParseVersion` accepts
any text matched by the `{version}` slot, so `prod/$(touch pwned)` counted as a version. The calver
branch now also requires `semver.IsBareVersion` (the same test `releaseTags` already applied) and
scans git's order for the first tag that passes, instead of giving up when the first line fails.
That is a small behavior change beyond the title: a stray ref sorting first no longer switches the
E002 check off, the check compares against the highest real tag. The auto path was not affected (it
reads `releaseTags`). A table test pins both strategies, with the hostile tag first and alone; it
failed before the change on three of the four rows.

#### `[x]` T384: CalVer picks the highest parsed tag as the base

The base is the first parsable tag in git's `--sort=-version:refname` order. Choose the highest
parsed value instead, so formats with a leading SPRINT or zero-padded fields cannot re-issue an
existing tag. Characterization tests first for `YYYY.MM.PATCH`, `YYYY.WW.PATCH` and one SPRINT-first
format.

**Completion note:** `calver.Resolver.Resolve` and `BumpFromDate` now scan every tag that parses and keep
the highest, compared by a new `compareValues` in chronological order (year, semester, quarter, month,
ISO week, day, sprint, patch). A dimension the format lacks is zero on both sides and never decides.
On equal values git's first line is kept. Characterization rows for `YYYY.MM.PATCH`, `YYYY.WW.PATCH`
and `SPRINT.YYYY.MM.PATCH` passed before the change and still do; the new rows feed the resolver tags
in an order git would not be trusted for (the mock stands in for it, since a real misordering needs a
specific tag set) and failed on the first-line pick. `BumpFromDate`, the entry point calver-per-env
uses, got the same rule. Not changed: calver-per-env still reports `Result.CurrentTag` as the first
release tag in git's order (`releaseTags`), so under an ordering git gets wrong that field and the
next version can name different tags. It only feeds `previous_tag` and the display, and the next
version is the safe one; worth a follow-up if it shows up in practice.

#### `[x]` T385: CalVer dates in UTC

Convert the injected clock to UTC before extracting year, month, day, ISO week, quarter and
semester. The test clock (`heraut_testclock`, `HERAUT_TEST_NOW`) keeps working. Test around a day,
ISO-week and quarter boundary with a non-UTC input.

**Completion note:** `calver.Resolver.computeNext` converts the injected clock with `.UTC()` before any
field is read, so year, month, day, ISO week, quarter and semester all come from the UTC instant. This is
the only place the calver clock is read, so one line covers every format. The test clock needed no change:
`HERAUT_TEST_NOW` already parses an RFC 3339 instant and the conversion happens in the resolver, not in
the clock. A table test feeds instants with a non-UTC zone across a month, day, ISO-week, quarter,
semester and year boundary, in both directions (zone behind and ahead of UTC); every row failed on the
local date before the change. Effect for users: a release cut late in the evening in a zone behind UTC,
or early in the morning in one ahead of it, can now get a different calendar date than before; the date
is the UTC one, as decided.

#### `[x]` T386: always probe for an existing tag on `--set-version`

Today only the maintenance, branches and per-env cases probe. Run the probe for every `--set-version`
run, plain semver without `versioning.branches` included, so a duplicate fails before the changelog
commit is pushed. `--force` behavior and E003 are unchanged unless a test shows otherwise.

**Completion note:** [ADR-0073](../adr/0073-set-version-always-probes-for-an-existing-tag.md) records the
change, as the rules ask before a tested behavior moves. `guardSetVersion` now wraps every `--set-version`
resolver: SemVer strategies probe `git tag -l <tag> <tag>+*` (`semver.ExistingRelease`, unchanged), CalVer
strategies the exact tag. The branch/version mismatch check still needs `semver` plus a `branches` block.
Two rulings: `--force` skips the probe under the per-env strategies only, because there it already means
"proceed although the candidate exists" (E001), and under plain `semver`/`calver` it never skipped the
guard, so nothing changes; and `heraut version next --set-version` keeps its old scope (probe only with
`semver` and `branches`) through a new `WithBranchesOnlyCollisionGuard` option, since it prints a tag,
changes nothing and is sometimes run with no repository (several of its tests prove it with a `git` that
fails). The "no branches block: no git call" row of `TestNewResolver_SetVersion_CollisionGuard` was not
deleted: it now expects the probe, and a second row covers a taken tag. Seventeen override-parsing tests in
`internal/app` and the `release --dry-run --set-version` table got the guard switched off or a `git` that
answers the probe, a setup change with no assertion touched. Effect for users: `release --set-version X`
on a taken tag now fails up front instead of after the changelog commit is pushed; `release --dry-run
--set-version` in a directory that is not a git repository now fails, as the rest of `release` does.
Phase 71 is done.

#### `[ ]` T399: report the highest parsed tag as `CurrentTag` under `calver-per-env`

Follow-up to T384. calver-per-env derives the next version from the highest parsed tag, but
`Result.CurrentTag` (and so the `previous_tag` hook variable and `heraut version current`) is still the
first release tag in git's order. When git's order is wrong (clock-skewed taggers, a tag cut by hand), the
two can name different tags. Report the tag the next version was derived from. Characterization test
first: with tags in a misleading order, `CurrentTag` equals the tag that fed the next version. SemVer
per-env is untouched (git's `-version:refname` order is the version order there).

---

### Phase 72 — Pipeline, changelog write and recovery docs

Goal: make the changelog write safe, document the hook and failure contracts, and tidy the release
flow. Entry: Phase 71 done.

#### `[x]` T387: write the changelog atomically

`internal/generators/native/generator.go` writes with `os.WriteFile` in place. Write a temp file in
the same directory and rename it, keeping mode `0644`. Test an interrupted write leaves the original
file intact.

**Completion note:** `writeChangelog` now goes through `writeFileAtomic`: `os.CreateTemp` next to the
target, write, chmod `0644`, close, rename; the temp file is removed on any failure. A symlinked changelog
path is resolved with `filepath.EvalSymlinks` first, so the link survives and its target is replaced (an
in-place `os.WriteFile` did the same, so this keeps behavior). The "interrupted write" test makes the
directory read-only: the old in-place write still truncated the writable file and passed, the new one fails
with the original byte for byte intact and no temp file left; it is skipped on Windows and for root. One
visible effect: a changelog that is writable inside a read-only directory is now refused, where it used to
be overwritten. Tests in `writechangelog_internal_test.go`.

#### `[x]` T388: document the hook variable contract and the untrusted-config warning

Document that hook template values are inserted into `sh -c` unquoted, that quoting is the hook
author's job, and that CI must not run hooks from an untrusted pull request's config. Spec and guide
update, with an ADR note if the existing hook ADRs (0053, 0054) need amending. Add a test that a tag
with shell syntax reaches a hook only through the documented path.

**Completion note:** Spec 02 gains a "Quoting and trust" subsection under the template variables, and the
hooks guide a short "Quoting and untrusted configs" section; ADR-0053 got a dated clarification line, not a
new ADR, since no decision moved. The doc says only what was checked: single quotes keep a value inert,
double quotes do not stop `$(...)`, and `Version`/`Tag`/`PreviousTag` come from the config's `tag_format`
plus a parsed version (T383 closed the one path that let a stray ref in). The test
(`internal/pipeline/hooks_shell_test.go`) runs a real `sh` with a `PreviousTag` of `v1.0.0$(touch pwned)`:
unquoted and double-quoted expand it, single-quoted does not. It is a characterization, so it passed on
first run, as the behavior is unchanged; it fails if heraut ever starts quoting values. Found while
checking the claims: under `calver-per-env`, `--set-build-id` is only checked for `/` and whitespace, so a
build ID such as `x$(id)` reaches `Tag` and the hooks. The doc says so instead of promising more; the
tightening is T400. Not verified: how `--env` is validated outside the per-env strategies.

#### `[x]` T389: write the failure-recovery guide

A task-oriented guide under `docs/guides/` listing the states a failed release can leave (commit
pushed without tag; local tag only; tag pushed without release; release on one forge only; hook
failed after the release exists) and how to recover each, including what cannot be undone (a tag
already consumed by CI). Verify every command on the sandbox repos before writing it, including how to
delete a GitHub and a GitLab release and whether any heraut command can publish for an existing tag.
No history rewriting is described.

**Completion note:** [`docs/guides/failure-recovery.md`](../guides/failure-recovery.md), linked from the
guides index and Spec 02 § Failure semantics, covers five states (commit pushed without tag, local tag only,
tag pushed without release, release on one forge only, hook failed after the release), how to undo a
release, and what cannot be undone. Every command was run against the GitHub and GitLab sandbox repositories
and the sandboxes were cleaned afterwards (tags, releases, branches). Findings that shaped the guide: no
heraut command publishes for an existing tag (`heraut release` stops with `no commits since <tag>`), so
recovery is `heraut notes <tag> --output` plus `gh release create --verify-tag` or `glab release create -F`;
`glab release create` tags the default branch's HEAD when the tag is not on the remote, `gh --verify-tag`
refuses; `gh release delete --cleanup-tag` removes the tag only if the release exists; `glab release create
<files>` attaches plain links, unlike heraut's Generic Packages layout. The hook-failure state was observed
for `post_release` only; the `pre_release` path is from Spec 02. Correction made on the way: Spec 03 said a
retry skips the changelog commit when the file is identical, but the footer timestamp is minute-resolution,
so only a retry in the same minute does; the spec now says so. Two process notes: I pushed test commits to
the GitHub sandbox's `main` (the harness works on `e2e/<id>` branches), which needs a force-push to undo,
left to the maintainer; and no real repository names are in the guide.

#### `[x]` T390: split `Pipeline.Run` and unify the dry-run path

`Pipeline.Run` is about 190 lines mixing resolve, hooks, changelog, tag, publish and notes, and the
dry-run path repeats it while discarding errors with `_ =`. Split into named steps and drive both
paths from the same step list. Test that the `--json` plan equals the executed steps. Move-only where
possible.

**Completion note:** `Pipeline.Run` is now 36 lines. The steps are described once, in
`internal/pipeline/steps.go`: a `releaseStep` carries what a real run executes, what a dry run shows
(reporter and plain), and what `Plan` lists, and `releaseFlow` builds the ordered list for a resolved
version. `Run` resolves, then executes or renders the prelude (post_bump hooks, the asset preflight gate)
and the body; `Plan` walks the same list. The dry-run path no longer repeats the sequence or discards step
errors with `_ =`. Safety net first: `plan_equivalence_test.go` (committed before the refactor, passing on
the old code, and shown to fail when `Plan` drops a step) asserts that the plan, a dry run and a real run
name the same steps in the same order across six configurations; the 257 existing pipeline tests passed
unchanged, so output is identical. One presentation difference, deliberate: a dry-run hook template that
does not render now aborts before the publish step is reported, as it already did for the other hook
points, instead of showing a failed "Publish to X" step. Not done: `ChangelogPipeline` keeps its own copies
of the hook helpers, which is T401. Phase 72's remaining work is T400.

#### `[ ]` T400: restrict the `calver-per-env` build ID to tag-safe characters

Found by T388. `--set-build-id` under `calver-per-env` is checked only for `/` and whitespace
(`tagfmt.ValidateBuildID`), so shell syntax such as `x$(id)` reaches `Tag` and the hook variables, while
the SemVer strategies already require dot-separated `[0-9A-Za-z-]` identifiers. Decide the allowed set
(likely `[0-9A-Za-z._-]`), ADR first since a tested behavior changes, then test and enforce it. Also check
that `--env` is rejected when it names no configured environment under every strategy.

---

### Phase 73 — Structural debt

Goal: reduce the size and coupling of the largest files, without behavior change. Entry: Phase 72
done and coverage still above the gate.

#### `[x]` T391: split `internal/config/validator.go` by concern

About 1,100 lines. Split by concern (forges and targets, commits, per-env, hooks); `ValidationErrors`
and messages unchanged.

**Completion note:** `validator.go` (1,151 lines) is now seven files in `internal/config/`: `validator.go`
(enum tables, `Validate`, required fields, enums, 119 lines), `validate_versioning.go` (bump, branches,
strategy rules, tag-format helpers), `validate_commits.go` (tickets, commit types and rules),
`validate_forges.go` (forges, targets, release blocks), `validate_rendering.go` (rendering, trailers,
template snippets, content drivers), `validate_perenv.go` (environments, contradictions, cycles) and
`validate_hooks.go`. Move-only: the sorted non-blank lines of the old file equal those of the new files
except `package`, `import` and `var (` lines, and no test changed. `validCommitRuleTargets` and
`commitTypePattern` moved to the commits file with the code that uses them.

#### `[ ]` T392: handle flag-read errors in cobra constructors

64 `cmd.Flags().GetX(...)` calls ignore their error. Handle them through one small helper, and shorten
the longest constructors (`NewReleaseCmd`, `NewInitCmd`) as part of it.

#### `[ ]` T393: define forge host URLs once

The GitHub and GitLab hosts are defined in four places (`internal/platforms/*`, `internal/config`,
`internal/forge/detect.go`). Define each once.

#### `[ ]` T394: decide the `origin` remote

`origin` is hardcoded in `internal/pipeline/git.go` and `internal/app/pipeline.go`. Either add a
configurable remote (config field, schema, sample, spec) or document the limit. Decide first.

#### `[ ]` T395: reconcile the layer rules with the real imports

`cmd/heraut` imports `internal/ui`, `internal/platforms/*` import a shared `internal/platforms`
package, and `cmd` imports `exitcode`, none of which the layer table lists. Amend the table in
`.claude/rules/coding.md` or change the code; decide first.

#### `[ ]` T396: move policy code out of `internal/app`

Branch rules, tickets, pre-release and changelog rotation live in the wiring layer. Move them to the
packages that own them, and move the git remote lookup out of `app/pipeline.go`.

#### `[ ]` T397: separate form code from decisions in `internal/scaffold/wizard.go`

About 870 lines with form code interleaved with decision logic. Extract the decisions into testable
functions, then slim the wizard.

#### `[ ]` T401: share the hook helpers between `Pipeline` and `ChangelogPipeline`

Found by T390. `ChangelogPipeline` (`internal/pipeline/changelog.go`) keeps its own copies of
`runHookPointStep`, `dryRunHookStep`, `printDryRunHookLinesPlain`, `dryRunHookLinesOrNil` and
`runOrRenderHookPoint`, and repeats the dry-run versus real split that T390 removed from `Pipeline`.
Drive `heraut changelog --tag` from the same `releaseStep` list, with the same equivalence test.
