# Héraut — User Documentation Roadmap

> Status: Planned (drafted 2026-11-06)
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
| 89a | Front doors and getting started | T453–T458 | In progress |
| 89b | Generated reference | T459–T464 | Planned |
| 89c | Guides | T465–T475 | Planned |
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

#### `[ ]` T457: Guide — release from GitHub Actions

New `docs/guides/github-actions.md`: a complete workflow file (manual trigger, `permissions:`,
`fetch-depth: 0`, token, the heraut install step), what each part is for, and the common failures
(shallow clone, missing `contents: write`, protected branch).

#### `[ ]` T458: Guide — release from GitLab CI/CD

New `docs/guides/gitlab-ci.md`: a complete `.gitlab-ci.yml` job (rules, `GIT_DEPTH`, the image), job token
vs project access token (what each can push and publish), protected tags and branches, and the common
failures.

---

### Phase 89b — Generated reference

#### `[ ]` T459: a `Long:` description for every command

Only 2 commands have a `Long:` today. Write one for each command and subcommand: what it is for, when to use
it, one example. It improves `--help` as much as the docs. Tests: a table test that every command has a
non-empty `Long:` and `Example:`.

#### `[ ]` T460: generated commands reference

`docs/reference/commands/*.md`, one page per command, generated with `cobra/doc` (part of the cobra module
already required). A `mise run docs:gen` task writes them; a test regenerates into `t.TempDir()` and fails
on any diff with the committed files. Exit codes and env vars link to T463/T464's pages.

#### `[ ]` T461: `schema.json` descriptions written for users

128 keys: fill the 15 without a description, drop the ADR references (25), shorten the long ones. Each
description says what the key does, its default and one example value. IDEs show these on hover. The
`docs/heraut.sample.yml` comments are aligned where they disagree. Schema tests stay green; a test asserts
every property has a description and none cites an ADR.

#### `[ ]` T462: generated configuration reference

`docs/reference/configuration.md`: a hand-written intro (where the file lives, `HERAUT_FILE`, how the keys
are organised, a minimal example, a link to `docs/heraut.sample.yml`) above a table generated from
`schema.json` (key path, type, default, description). Same generator task and drift test as T460.

#### `[ ]` T463: environment variables reference

New `docs/reference/environment-variables.md`: every variable heraut reads — `HERAUT_FILE`,
`HERAUT_SKIP_HOOKS`, `HERAUT_CHECK_UPDATE`, the token variables, the CI variables used for detection — with
what each does. Hand-written, checked against `internal/config` and `internal/forge`.

#### `[ ]` T464: exit codes reference

New `docs/reference/exit-codes.md`: each exit code, what causes it, and what to do (config error, runtime,
promotion E001–E003, …). Checked against `internal/exitcode`.

---

### Phase 89c — Guides

#### `[ ]` T465: Guide — publish to GitHub from GitLab CI/CD

New `docs/guides/github-from-gitlab-ci.md`: a GitLab project that publishes to GitHub, or to both. The two
`forges:` entries, tokens (`token_env`, why the job token does not work for GitHub), `release.targets`,
`commits.enrichment_forge`, and which links point where in each target's notes.

#### `[ ]` T466: Guide — self-hosted GitLab

New `docs/guides/self-hosted-gitlab.md`: `base_url`, `token_env`, what CI fills in for you, and running
locally on a work machine (the config route, and the env-only route with its limits).

#### `[ ]` T467: Guide — keep the release commit from triggering CI

New `docs/guides/skip-ci-on-release-commit.md`: `git.push_options` on GitLab, `[skip ci]` and why it also
skips the tag pipeline, the GitHub `paths-ignore` and job-condition patterns.

#### `[ ]` T468: Guide — several environments

New `docs/guides/environments.md`: `semver-per-env` / `calver-per-env`, `--env`, promotion between
environments, the E001–E003 errors and `--force`, `disable_changelog`.

#### `[ ]` T469: Guide — pre-releases

New `docs/guides/pre-releases.md`: `--pre-release`, the `-rc.N` series, moving to the final version, what is
and is not written (no `CHANGELOG.md`), the GitHub pre-release flag.

#### `[ ]` T470: Guide — attach files to a release

New `docs/guides/release-assets.md`: `release.targets[].assets` patterns, what GitHub renames, GitLab generic
packages, failures on a missing file.

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
