# Héraut — User Documentation Roadmap

> Status: In progress (drafted 2026-11-06; 89a and 89b done)
> Source: the user's review of 2026-11-06. heraut's docs are written for contributors and agents
> (specs, ADRs, roadmap); a user who lands on the repo has no path that avoids them. This epic adds
> user docs next to the contributor docs, without moving `docs/specs/`, `docs/adr/` or `docs/tasks/`.
> Page map and decisions: [`.claude/plans/user-docs-page-map.md`](../../.claude/plans/user-docs-page-map.md).
> Main roadmap: tracked as Phase 89 in [`roadmap.md`](roadmap.md).

## Conventions

- Task IDs **continue the global sequence** (`T453+`).
- This file is the **single source of truth** for task status. Markers: `[ ]` not started, `[x]` done.
  Two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)): write the page, flip the marker and
  add a completion note.
- One task per session, unless the user approves more (a "start Phase 89" message approves the phase).
- **Two front doors.** Users enter through `README.md` → `docs/README.md`; contributors and agents through
  `CONTRIBUTING.md` (and `CLAUDE.md`). User pages never send the reader to a spec or an ADR to finish a
  task; they link to the user reference instead.
- **The specs stay the authority.** A user page explains and shows; it never states a rule the specs
  don't. A page that finds a spec gap or a spec/behaviour mismatch stops and raises it.
- **Every example is run.** Commands, configs and outputs shown on a page are checked against the built
  binary in a scratch repo before the commit. Placeholders are synthetic (`acme/app`,
  `gitlab.example.com`), never real projects, users or hosts.
- **Style:** second person, task first, example first, one question per page. No ADR numbers, task IDs or
  internal package names in user pages.
- **Tests where there is code.** The generated reference tasks (T460, T462) are TDD like any code task: a
  failing drift test first.
- Commit types: `docs(readme)`, `docs(guides)`, `docs(concepts)`, `docs(reference)`, `docs(contributing)`;
  `feat(cmd)` / `docs(schema)` / `test(...)` where the task touches code or `schema.json`.

## Progress at a glance

| Phase | Theme | Tasks | Status |
|---|---|---|---|
| 89a | Front doors and getting started | T453–T458 | Done |
| 89b | Generated reference | T459–T464 | Done |
| 89c | Guides | T465–T475, T486 | Planned |
| 89d | Concepts | T476–T479 | Planned |
| 89e | Keeping user docs current | T480 | Planned |

---

### Phase 89a — Front doors and getting started

#### `[x]` T453: `CONTRIBUTING.md`, the contributor front door

New `CONTRIBUTING.md`: setup (`mise install`, hk hooks), TDD and the test layers, conventional commits, the
roadmap flow, then links to `docs/specs/`, `docs/adr/`, `docs/tasks/` and the e2e guide.
`docs/guides/e2e-tests.md` moves to `docs/contributing/e2e-tests.md` (update every link to it, `CLAUDE.md`
and the rules files included). Points to `.claude/rules/` rather than repeating them.

**Done.** `CONTRIBUTING.md` covers setup, the hooks, how work is done (specs and ADRs first, roadmap,
TDD, conventional commits, architecture, config sync), e2e and releases, and links to `.claude/rules/`
for the full rules instead of copying them. `e2e-tests.md` moved to `docs/contributing/` (its relative
links keep working at the same depth); live references updated in Spec 06, `docs/guides/README.md`,
`CLAUDE.md`'s layout and the comment at the top of `.github/workflows/e2e.yml` (comment only). Past
roadmap notes and `docs/superpowers/` plans keep the old path: they record what was true then.

#### `[x]` T454: README as the user front door, and `docs/install.md`

`README.md` keeps: what heraut is and why, one short example, install with Homebrew and
`mise use packslip:…` only, a "Read the docs" link to `docs/README.md`, license. The Quickstart, Commands,
Configuration and Updates sections move out (to Getting started, the reference and `docs/install.md`).
The Documentation section stops calling the specs "the authority for users" and links the user docs, with
one line for contributors pointing to `CONTRIBUTING.md`.

New `docs/install.md`: the plain `github:` mise backend, the prebuilt binary (checksums, packslip
verification), Docker, `go install`, prerequisites (`git`), updates and the update hint.

**Done.** The README keeps the pitch, then Install (Homebrew and `mise use packslip:…` only, one line
each), "A first look" (the four commands and a minimal `.heraut.yml` with no `forges:` block, checked
with the built binary: `check config` passes and the dry run resolves `v0.1.0` and would publish to the
GitHub origin), "What it handles" in user terms, and Documentation (the user docs, plus one line to
`CONTRIBUTING.md`). The Prerequisites, Commands, Configuration and Updates sections left the README:
`git` and upgrades went to `docs/install.md`; tokens, commands and configuration come back with Getting
started and the reference. `docs/install.md` fixes a bug in the old curl example (the tag has a `v`,
the file name doesn't) and was checked against the real 0.84.0 assets: the checksum line reads `OK`
and `gh attestation verify` succeeds (the packslip action attests each binary). The upgrade table
uses the commands forge's update hint prints (`brew upgrade heraut`, `mise upgrade heraut`).

#### `[x]` T455: `docs/README.md`, the user docs home

Rewrite `docs/README.md` as the user docs home, sorted by goal: get started, set up CI, guides by
situation, concepts, reference, changelog. It lists only pages that exist; each later task adds its page.
One closing line points contributors to `CONTRIBUTING.md`.

**Done.** `docs/README.md` opens with one sentence on what heraut does, then Get started (Install),
Guides as an "I want to…" table over the six user guides, and Reference. Until T460/T462 the reference
is the commented `heraut.sample.yml`, `--help` and the changelog; no spec is linked. The specs, ADRs,
roadmap and `ideas.md` rows the old index had now live in `CONTRIBUTING.md` (`ideas.md` added there).
`docs/guides/README.md` stays as the folder index; its titles are aligned in T475.

#### `[x]` T456: Getting started — your first release

New `docs/getting-started.md`: from an empty repo to a published release, locally. `heraut init` →
`heraut check` → `heraut release --dry-run` → `heraut release`, with what each step prints (from a real run)
and what it changed in the repo. Ends with links to the two CI guides.

**Done.** `docs/getting-started.md`: write the minimal config (the README's, forge detected from
`origin`) with `heraut init` as the guided alternative, then `check config`, `check runtime` (with
the token), `version next` and the bump rules, `release --dry-run`, a local `heraut changelog` to read
the result, and what `heraut release` does step by step. Every output is from a run of the built
binary in a scratch repo with a `git@github.com:acme/app.git` origin; the git identity shown is
synthetic. The publish step itself was not run against a real forge. Deviation: the page starts from a
written config, not `heraut init --defaults`, because `--defaults` always writes a GitLab forge (Spec
03), even in a repo whose origin is on GitHub; raised with the user as a possible change. Found while
checking: the changelog has commit links only with a CI or target context, so a local `heraut
changelog` shows bare hashes; the page says so. Links forward to the two CI guides (T457, T458).

#### `[x]` T457: Guide — release from GitHub Actions

New `docs/guides/github-actions.md`: a complete workflow file (manual trigger, `permissions:`,
`fetch-depth: 0`, token, the heraut install step), what each part is for, and the common failures
(shallow clone, missing `contents: write`, protected branch).

**Done.** `docs/guides/github-actions.md`: a `workflow_dispatch` workflow (concurrency, `contents:
write` and `pull-requests: read`, `fetch-depth: 0`, heraut installed with `gh release download` at a
pinned version as forge's release-setup does, a bot git identity, `heraut check` then `heraut
release`), what each part is for, why the release push starts no workflow with the workflow token,
the GitHub path filter for other tokens, a protected `main`, and a failure table. The workflow was
not run on GitHub (no sandbox run requested); its parts are those of heraut's own release workflow
and forge's release actions. Checked with the binary: a depth-1 clone without tags resolves the first
version again, silently (raised with the user as a possible check); without a `changelog:` block the
release has 5 steps and commits nothing. A user page doesn't link the spec, so the path filter from
Spec 02 is inlined; T467 will own it.

#### `[x]` T458: Guide — release from GitLab CI/CD

New `docs/guides/gitlab-ci.md`: a complete `.gitlab-ci.yml` job (rules, `GIT_DEPTH`, the image), job token
vs project access token (what each can push and publish), protected tags and branches, and the common
failures.

**Done.** `docs/guides/gitlab-ci.md`: the job-token push setting first, then a manual job on the
default branch in heraut's Docker image (`entrypoint: [""]`, pinned tag), `resource_group`,
`GIT_DEPTH: "0"`, `git checkout -B "$CI_COMMIT_BRANCH"`, a git identity, `heraut check` then
`heraut release`; what each part is for, protected branches and tags (a project access token for the
push only, since in GitLab CI the API always uses the job token), `git.push_options.commit: [ci.skip]`
for the release commit's pipeline (observed on gitlab.com in T449), and a failure table. Found and
checked with the binary: on a detached HEAD, GitLab CI's default checkout, the changelog commit's
`git push origin HEAD` fails ("not a full refname") and nothing reaches the remote; after
`git checkout -B main` the commit and tag are pushed. The job was not run on GitLab (no sandbox run
requested). Raised with the user: heraut could detect the detached HEAD (and the shallow clone of
T457) itself.

---

### Phase 89b — Generated reference

#### `[x]` T459: a `Long:` description for every command

Only 2 commands have a `Long:` today. Write one for each command and subcommand: what it is for, when to use
it, one example. It improves `--help` as much as the docs. Tests: a table test that every command has a
non-empty `Long:` and `Example:`.

Done 2026-11-06.
- **Text.** Every command has a `Long:` (what it is for, when to use it, in two to four lines) and an
  `Example:` block (cobra's two-space indent, `#` comments where examples need a word).
- **`whatsnew`.** It comes from forge's `updatecheck.WhatsNewCommand`, so `root.go` sets its text
  on the returned command instead of changing forge.
- **The root command.** Its `Long:` stays the gold banner, and only an `Example:` was added. T460's
  generator will replace the banner with plain text for the page.
- **The test.** `TestEveryCommandHasLongAndExample` (`internal/cmd/help_text_test.go`) walks the
  tree, skipping cobra's `help`/`completion`. Each command needs both fields, and every non-comment
  example line must start with `heraut`. A mutant breaking that rule fails it.
- **Not touched.** `check runtime`'s `Short:` still says "binaries on PATH, token env vars". It's
  stale since native has no binary, but out of scope; its new `Long:` describes what it checks.

#### `[x]` T460: generated commands reference

`docs/reference/commands/*.md`, one page per command, generated with `cobra/doc` (part of the cobra module
already required). A `mise run docs:gen` task writes them; a test regenerates into `t.TempDir()` and fails
on any diff with the committed files. Exit codes and env vars link to T463/T464's pages.

Done 2026-11-06.
- **Not `cobra/doc`, a deviation.** It imports `go-md2man` and `blackfriday`, absent from `go.sum`,
  so it would have added two dependencies, against the plan's "no new dependency". The generator is
  ~150 lines in `internal/cmd/reference.go` (`WriteCommandReference`), walking the cobra tree. Its
  output follows the docs' own style: title, Short, Long, Usage, Examples, a Flags and a Global
  flags table (type, default, description), Subcommands, and See also (parent, index, exit codes,
  environment variables).
- **The root page.** It skips the root's `Long:`, which is the terminal banner.
- **Escaping.** `<placeholder>` text outside code is escaped so Markdown doesn't eat it as HTML, and
  `|` is escaped in table cells.
- **Entry point.** `tools/docgen/main.go` is a thin program run by the new `mise run docs:gen` task.
  It writes `docs/reference/commands/`: 22 pages plus a `README.md` index linked from the docs home.
- **Tests.** A unit test on a synthetic tree covers hidden commands and flags, shorthand, defaults,
  slices, escaping and the root banner. `TestCommandReference_IsCurrent` regenerates into
  `t.TempDir()` and fails on any difference; editing a `Long:` without regenerating fails it.
- **`go.mod`.** `go mod tidy` moved `spf13/pflag` from indirect to direct (now imported for the
  flag tables); no new module.
- **Also updated.** CONTRIBUTING lists the task and says help text is user documentation, and the
  CLAUDE.md layout names `docs/reference/`.

#### `[x]` T461: `schema.json` descriptions written for users

128 keys: fill the 15 without a description, drop the ADR references (25), shorten the long ones. Each
description says what the key does, its default and one example value. IDEs show these on hover. The
`docs/heraut.sample.yml` comments are aligned where they disagree. Schema tests stay green; a test asserts
every property has a description and none cites an ADR.

Done 2026-11-06.
- **The survey.** 124 described keys in today's schema (the 128 in the plan counted differently):
  11 with no description, 15 citing ADRs, 5 over 300 characters.
- **`$ref` keys.** The schema is draft-07, which ignores a `$ref`'s sibling keys. So a `$ref` key is
  described by its definition, and the four definitions without one got one: `Versioning`,
  `ContentDriver`, `Release`, `BranchRule`. That fixes the empty hover on `versioning:`,
  `changelog:`, `release:`, `commits:`, `rendering:`, `hooks:` and `git:` at once.
- **Rewritten for users.** 41 descriptions, the ADR-citing and long ones, plus weak or jargon ones
  (`strategy`, `platform`, `draft`, `bump`, `source`, `api_mode`, `types_heading_level`, "on both
  drivers"). Each says what the key does, then its default or an example where one helps. Defaults
  were checked in code; `types_heading_level` is 3 (`headingPrefix`).
- **Formatting.** Edited through a JSON round trip, which reproduces the file byte for byte.
- **The test.** `TestSchema_EveryKeyIsDescribedForUsers` (`internal/config`) walks every property
  (root, definitions, inline items and maps). Each needs a description, on itself or on its `$ref`
  definition, with no ADR number, task id, spec path or `§`, and at most 300 characters. It failed
  37 subtests before the rewrite.
- **The sample.** The keys rewritten here agree with `docs/heraut.sample.yml`.
- **Deferred.** The sample's comments still cite ADRs (20 times). They are contributor references
  in a file users read; removing them belongs to a separate pass on the sample.

#### `[x]` T462: generated configuration reference

`docs/reference/configuration.md`: a hand-written intro (where the file lives, `HERAUT_FILE`, how the keys
are organised, a minimal example, a link to `docs/heraut.sample.yml`) above a table generated from
`schema.json` (key path, type, default, description). Same generator task and drift test as T460.

Done 2026-11-06.
- **The page.** `docs/reference/configuration.md` is hand-written up to a marker comment: where the
  file lives, the minimal file with its schema line, how the keys are organised, how to read the
  paths. Below the marker, `mise run docs:gen` writes one section per top-level key, each a table of
  path, type (enum values listed), schema default and description. The intro stays editable in
  place.
- **The generator.** It is in `tools/docgen/config.go` and keeps the schema's own key order (a token
  walk, since Go maps lose it). Lists show as `forges[].name` and maps as
  `environments.<name>.bump`.
- **Repeated definitions.** A definition is expanded once, at its own top-level key when it has
  one, and every other place says "Same keys as `rendering`". So `environments.<name>.changelog`
  doesn't repeat 30 rows.
- **Result.** 126 rows.
- **Tests.** Unit tests on a synthetic schema cover order, types, enums, defaults, escaping, list
  and map paths, and first expansion at the top level. `TestConfigReference_IsCurrent` fails while
  the page is behind `schema.json`. Both drift tests (commands and configuration) now share the one
  `docs:gen` task.
- **Two more schema fixes.** The per-environment `changelog` and `release.notes` said "Override …
  generator"; they now say they merge over the top-level block. Linked from the docs home.

#### `[x]` T463: environment variables reference

New `docs/reference/environment-variables.md`: every variable heraut reads — `HERAUT_FILE`,
`HERAUT_SKIP_HOOKS`, `HERAUT_CHECK_UPDATE`, the token variables, the CI variables used for detection — with
what each does. Hand-written, checked against `internal/config` and `internal/forge`.

Done 2026-11-06.
- **The page.** `docs/reference/environment-variables.md` has four sections: heraut's own settings;
  the tokens (who reads which, in what order); the CI variables, per CI system, with what each is
  read for; and `NO_COLOR` and `PAGER`/`NO_PAGER` (forge's `whatsnew` pager). It is linked from the
  docs home.
- **Checked against the code.** The token rows come from `internal/platforms/{github,gitlab}`
  (publishing) and `internal/forge/{detect,resolve}.go` (enrichment). `HERAUT_TEST_NOW` is left out
  because only the `heraut_testclock` test build reads it.
- **Drift test.** `TestEnvironmentVariablesReference_NamesEveryVariable` (`internal/config`) fails
  when a name in `config.CIEnvVars`, or one of the three `HERAUT_` settings, is missing from the page.
- **Finding, not fixed.** For GitHub, publishing reads `GH_TOKEN` then `GITHUB_TOKEN`, but
  pull-request enrichment reads only `GITHUB_TOKEN` (`defaultTokenEnvFor`). A user who sets only
  `GH_TOKEN` publishes fine but gets no PR data outside Actions. The page states it as it is.

#### `[x]` T464: exit codes reference

New `docs/reference/exit-codes.md`: each exit code, what causes it, and what to do (config error, runtime,
promotion E001–E003, …). Checked against `internal/exitcode`.

Done 2026-11-06.
- **The page.** `docs/reference/exit-codes.md` has a summary table (code, `--json` `error.code`,
  meaning), one section per code listing its causes and what to do, and a table for the three
  promotion guards with `--force`'s effect on each. It is linked from the docs home.
- **Sources.** The causes come from Spec 01's table, Spec 03 and Spec 04 § Promotion guards. The
  shallow-clone and detached-HEAD refusals from Phase 90 are included under 3.
- **Drift test.** `TestExitCodesReference_ListsEveryCode` (`internal/exitcode`) fails when a code
  constant or a guard has no table row; deleting the 70 row fails it.

---

### Phase 89c — Guides

#### `[x]` T465: Guide — publish to GitHub from GitLab CI/CD

New `docs/guides/github-from-gitlab-ci.md`: a GitLab project that publishes to GitHub, or to both. The two
`forges:` entries, tokens (`token_env`, why the job token does not work for GitHub), `release.targets`,
`commits.enrichment_forge`, and which links point where in each target's notes.

Done 2026-11-06.
- **The guide.** `docs/guides/github-from-gitlab-ci.md` covers the config (two forges,
  `enrichment_forge: gitlab`, a GitHub target, or both), the token, the job, which links point
  where, and a failure table. It is linked from the docs home and the GitLab CI guide.
- **A step the plan missed: the tag must be on GitHub first.** The GitHub driver creates the
  release by tag name only (no `target_commitish`). When the tag isn't on GitHub yet, GitHub creates
  it on the head of its own default branch, which may be an older commit. heraut pushes only to
  `origin`, so the guide pushes the commit and the tag to a `github` remote from a `post_tag` hook,
  which runs after the tag push and before publishing. A GitLab push mirror isn't enough: it is
  asynchronous.
- **Verified with the built binary,** simulating a GitLab CI job:
  - `check config` passes, and the dry run shows the hook between "Push tag" and "Publish to github";
  - from a detached HEAD, `changelog --tag` runs the hook for real into a bare repository standing in
    for GitHub (branch and tag arrive), and the release commit lands on `origin`'s `main` (T482);
  - the committed `CHANGELOG.md` links to the job's GitLab host;
  - the quoted messages come from runs.
- **Not run on real forges.** No sandbox run was requested.
- **Finding, not fixed.** Passing the tagged commit's SHA as `target_commitish` would make GitHub
  refuse a release whose commit it doesn't have, instead of tagging another commit silently.

#### `[x]` T466: Guide — self-hosted GitLab

New `docs/guides/self-hosted-gitlab.md`: `base_url`, `token_env`, what CI fills in for you, and running
locally on a work machine (the config route, and the env-only route with its limits).

Done 2026-11-06.
- **The guide.** `docs/guides/self-hosted-gitlab.md` covers CI on the instance (the minimal config
  is enough), a `forges:` entry for your own machine (with a table of `base_url`, `project`,
  `api_url` and `token_env`), the env-only trap, a private CA, and a failure table. It is linked
  from the docs home and the GitLab CI guide.
- **The env-only trap, found with the built binary.** Outside CI, with a self-hosted origin, no
  `forges:` and `GITLAB_TOKEN` set, zero-config detection falls back to the token and assumes
  gitlab.com. The dry run shows "Publish to gitlab", and a real run would push the tag to the
  server and then fail creating the release. The guide says to add the entry; detection works as
  Spec 02 describes, so there's no code change.
- **Checked in the code.**
  - Plain `http` is refused (the message is quoted from a run).
  - The job token is used for a `base_url` naming the job's own instance (`inCIAutologin`).
  - `api_url` only affects enrichment, since publishing goes through `base_url` with client-go.
  - The GitLab "not set" message comes from `credentials()`.
- **Not run against a real self-hosted GitLab.**

#### `[x]` T467: Guide — keep the release commit from triggering CI

New `docs/guides/skip-ci-on-release-commit.md`: `git.push_options` on GitLab, `[skip ci]` and why it also
skips the tag pipeline, the GitHub `paths-ignore` and job-condition patterns.

Done 2026-11-06.
- **The guide.** `docs/guides/skip-ci-on-release-commit.md` covers GitLab's `ci.skip` on the commit
  push only, why `[skip ci]` in the message skips the tag's pipeline too, GitHub's path filter and
  job condition (with the `ref_type == 'tag'` guard), the workflow-token note, per-environment
  lists, and a failure table.
- **Sources.** The forge behaviour comes from Spec 02 § `git`, checked on gitlab.com and github.com
  on 2026-11-04 (T446, T449). The dry-run line is quoted from a run of the built binary.
- **Links.** The two CI guides keep their short version and point here; the docs home lists the
  guide.

#### `[x]` T468: Guide — several environments

New `docs/guides/environments.md`: `semver-per-env` / `calver-per-env`, `--env`, promotion between
environments, the E001–E003 errors and `--force`, `disable_changelog`.

Done 2026-11-06.
- **The guide.** `docs/guides/environments.md` covers:
  - a dev → staging → prod chain;
  - `auto` against `promote`, and `source`;
  - the per-environment keys (`disable_changelog`, `disable_release`, `changelog`,
    `release.targets`, `git.push_options`);
  - `branch` and `--env auto`;
  - the three guards, with `--force` and exit 4;
  - `calver-per-env`;
  - a CI pointer.
- **Run with the built binary in a scratch repo.**
  - E003 before anything reached staging.
  - `dev/0.1.0` tagged with no changelog commit (`disable_changelog`).
  - `staging/0.1.0` promoted with its changelog commit.
  - E001 on a second promotion, and E002 with a `prod/0.2.0` tag in the way.
  - `--force` bypassing E002.
  - The branch-guard message, and `--env auto` picking `staging` from `main`.
  - `calver-per-env` giving `dev/2026.11.1` and `prod/2026.11.0`.
  - The quoted messages are from those runs.
- **Cosmetic finding, not fixed.** The error panel ends E003's text with ".." and E001's with
  "--force.": the renderer adds a period to messages that already end with one or with a command.

#### `[x]` T469: Guide — pre-releases

New `docs/guides/pre-releases.md`: `--pre-release`, the `-rc.N` series, moving to the final version, what is
and is not written (no `CHANGELOG.md`), the GitHub pre-release flag.

Done 2026-11-06.
- **The guide.** `docs/guides/pre-releases.md` covers:
  - cutting a candidate, and what it writes (no changelog or commit; the notes span the series step);
  - the GitHub flag, and no GitLab equivalent;
  - shipping the final on the same commit, and `version current` with and without
    `--include-pre-release`;
  - label ordering;
  - escalation, and the major block with `--allow-major`;
  - `--set-version` with a pre-release value.
- **Run with the built binary in a scratch repo.**
  - `rc.1`, then `rc.2` after a fix.
  - The no-new-commit and label-regression errors.
  - The final `v1.4.0`.
  - The `1.3.1` → `1.4.0` escalation warning, and the major-escalation error.
  - `--set-version 2.0.0-beta.1` (5 steps, no changelog) and its conflict with `--pre-release`.
  - The quoted outputs are from those runs.
- **Spec only.** GitLab's lack of a pre-release marker comes from Spec 05 and
  `gitlab.Platform.CreateRelease`.

#### `[x]` T470: Guide — attach files to a release

New `docs/guides/release-assets.md`: `release.targets[].assets` patterns, what GitHub renames, GitLab generic
packages, failures on a missing file.

Done 2026-11-06.
- **The guide.** `docs/guides/release-assets.md` covers:
  - `release.assets`, and a target's own `assets` replacing it;
  - the early check after `post_bump`, with building in a `post_bump` hook and
    `allow_missing_assets`;
  - the refusals: outside the repository, duplicate names, GitHub name collisions, non-regular
    files;
  - how GitHub (draft while uploading, renames) and GitLab (generic packages plus release links)
    store them;
  - `draft`.
- **Run with the built binary.** Three cases ran as dry runs; the quoted messages are from those
  runs:
  - the no-match warning, silent once `allow_missing_assets` is set;
  - the outside-repository refusal through a symlink to `/etc/hosts`;
  - the GitHub collision of `meta#1.tar.gz` with `meta.1.tar.gz`.
- **What couldn't run locally.**
  - The `App`/`app` case: macOS's filesystem is case-insensitive.
  - A real release: the runtime preflight calls GitHub with the token first.
- **From the code and spec.** Directories are skipped, not refused (`matchAssets`); the
  GitHub/GitLab storage details come from Spec 05.

#### `[ ]` T471: Guide — PR/MR data in the changelog

New `docs/guides/pr-data-and-tokens.md`: what enrichment adds (PR/MR links, `@handles`, New Contributors),
the tokens it needs per forge, `enrichment_policy`, `--offline`, one enrichment forge for every target.

#### `[ ]` T472: Guide — link tickets

New `docs/guides/tickets.md`: ticket patterns and URLs, `require_ticket`, `heraut commit tickets`.

#### `[ ]` T473: Guide — check commit messages

New `docs/guides/commit-conventions.md`: `commit verify` in a commit-msg hook (hk, lefthook, plain git
hooks), `commit check` in CI, `commit create`, types, scopes and `commits.rules`.

#### `[ ]` T474: Guide — let an AI agent drive heraut

New `docs/guides/ai-agents.md`: `heraut skill install`, the `--json` commands, a safe agent loop
(`check`, `version next`, `release --dry-run --json`).

#### `[ ]` T475: the six existing guides, reviewed for users

`mobile-ci-tagging`, `maintenance-branches`, `release-pipeline-and-hooks`, `template-customization`,
`store-release-notes`, `failure-recovery`: titles from the page map, ADR/task references removed, links
moved from specs to the user reference, examples re-run. May split per guide if the diff gets large.

#### `[ ]` T486: `heraut.sample.yml` for users

Found while writing T461: the sample's comments cite ADRs 20 times, and its header sends readers to
`docs/specs/02-configuration.md` and `04-versioning.md`. It is linked from the docs home and the
configuration reference as the commented full example, so it should follow the user-page rule:
- drop the ADR and task references;
- point to the user reference (`docs/reference/configuration.md`) and the guides instead of the
  specs;
- keep each comment in step with the schema descriptions rewritten in T461.

`TestShippedExamples_LoadAndValidate` must stay green; consider a test that the sample cites no ADR,
like the schema's.

---

### Phase 89d — Concepts

#### `[ ]` T476: How a release runs

New `docs/concepts/how-heraut-works.md`: the steps of `heraut release` and `heraut changelog`, what each
writes or pushes, what heraut never does (force-push, rewrite history), dry runs.

#### `[ ]` T477: Choose a versioning strategy

New `docs/concepts/versioning-strategies.md`: a decision table for the four strategies, one example each,
`tag_format`.

#### `[ ]` T478: How the next version is found

New `docs/concepts/next-version.md`: commit types to bump, tags as the starting point, `stay_at_v0`,
`confirm_major`, `--set-version`, CalVer tokens and `PATCH` reset.

#### `[ ]` T479: What goes into the changelog

New `docs/concepts/changelog-content.md`: which commits appear, sections, breaking changes, the changelog vs
release notes, where links come from.

---

### Phase 89e — Keeping user docs current

#### `[ ]` T480: user docs in the workflow

`.claude/rules/workflow.md` gains a rule: a change to behaviour, a flag or a config key updates the spec
and the user page that mentions it in the same commit series. `CLAUDE.md` § Docs and `docs/tasks/README.md`
describe the two front doors. A link check over `docs/` (lychee or an equivalent hk step) runs in hk and CI.
