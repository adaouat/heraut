# User docs — page map (draft)

Goal: a user who lands on the repo finds what they need without reading a spec. Contributor docs
(`docs/specs/`, `docs/adr/`, `docs/tasks/`) stay where they are and get their own front door.

Each line: **file** — title — *the user question it answers*. Status: `new`, `exists` (review for
tone), `move`, `rewrite`, `generated`.

## Front doors

| File | Title | Question | Status |
|---|---|---|---|
| `README.md` | heraut | *What is this, should I use it, how do I install it?* Pitch, one example, install (Homebrew and `mise use packslip:…` only), link to the docs. Quickstart, commands and config sections move out. | rewrite |
| `docs/README.md` | heraut documentation | *Where do I start, where is the page for my case?* Home of the user docs, sorted by goal. | rewrite |
| `CONTRIBUTING.md` | Contributing | *How do I work on heraut?* Setup (mise, hk), TDD, commits, then links to specs, ADRs, roadmap, e2e. | new |

## Get started

| File | Title | Question | Status |
|---|---|---|---|
| `docs/install.md` | Install | *How else can I install heraut, and how do I check what I installed?* The plain `github:` mise backend, prebuilt binary (checksums, packslip verification), Docker, `go install`, updates. The README keeps Homebrew and mise packslip. | move + expand |
| `docs/getting-started.md` | Your first release | *How do I go from nothing to a published release?* `init` → `check` → `release --dry-run` → `release`, locally, with what each step prints. | new |

## Guides (one situation each)

| File | Title | Question | Status |
|---|---|---|---|
| `docs/guides/github-actions.md` | Release from GitHub Actions | *What does the workflow file look like?* Permissions, token, fetch depth, manual trigger. | new |
| `docs/guides/gitlab-ci.md` | Release from GitLab CI/CD | *What does the `.gitlab-ci.yml` job look like?* Job token vs project token, the push rules, protected tags. | new |
| `docs/guides/github-from-gitlab-ci.md` | Publish to GitHub from GitLab CI/CD | *My CI is GitLab but the release goes to GitHub (or both), what do I configure?* Forges, tokens, `release.targets`, which forge enriches, which links point where. | new |
| `docs/guides/self-hosted-gitlab.md` | Self-hosted GitLab | *What do I set for my company's GitLab, in CI and on my machine?* `base_url`, `token_env`, the env-only option. | new |
| `docs/guides/skip-ci-on-release-commit.md` | Keep the release commit from triggering CI | *How do I stop a second pipeline from starting?* Push options on GitLab, `[skip ci]`, the GitHub workflow patterns. | new |
| `docs/guides/environments.md` | Several environments (dev, staging, prod) | *How do I version and promote per environment?* The per-env strategies, promotion, E001–E003 errors. | new |
| `docs/guides/mobile-ci-tagging.md` | Mobile apps: build numbers in tags | *How do I tag each CI build?* | exists |
| `docs/guides/maintenance-branches.md` | Hotfix branches | *How do I release a fix on an older version line?* | exists |
| `docs/guides/pre-releases.md` | Pre-releases (rc, beta) | *How do I cut an rc and then the final version?* | new |
| `docs/guides/release-assets.md` | Attach files to a release | *How do I upload my binaries or archives?* GitHub assets, GitLab generic packages. | new |
| `docs/guides/release-pipeline-and-hooks.md` | Run your own steps during a release | *Where do my scripts run, and how do I commit files they change?* | exists |
| `docs/guides/template-customization.md` | Customize the changelog and release notes | *How do I change what the notes look like?* | exists |
| `docs/guides/pr-data-and-tokens.md` | PR/MR data in the changelog | *How do I get PR links, @handles and New Contributors, and what token does it need?* `enrichment_policy`, `--offline`, one enrichment forge. | new |
| `docs/guides/tickets.md` | Link Jira (or other) tickets | *How do I turn ticket IDs in commits into links?* | new |
| `docs/guides/commit-conventions.md` | Check commit messages | *How do I enforce Conventional Commits in a hook and in CI, or write them interactively?* `commit verify/check/create`. | new |
| `docs/guides/store-release-notes.md` | Release notes for an app store | *How do I get one tag's notes as plain text?* | exists |
| `docs/guides/ai-agents.md` | Let an AI agent drive heraut | *How does my coding agent learn to use heraut?* `skill install`, `--json`. | new |
| `docs/guides/failure-recovery.md` | Recover from a failed release | *A release failed halfway, how do I finish or undo it?* | exists |
| `docs/guides/e2e-tests.md` | — | Contributor material, not a user guide. Linked from `CONTRIBUTING.md`. | move to `docs/contributing/` |

## Concepts (explanations, no rules)

| File | Title | Question | Status |
|---|---|---|---|
| `docs/concepts/how-heraut-works.md` | How a release runs | *What does `heraut release` do, step by step, and what can I trust it not to do?* | new |
| `docs/concepts/versioning-strategies.md` | Choose a versioning strategy | *SemVer or CalVer, with or without environments?* A decision table, one example each. | new |
| `docs/concepts/next-version.md` | How the next version is found | *Why did heraut pick this version?* Commit types, tags, `stay_at_v0`, `--set-version`. | new |
| `docs/concepts/changelog-content.md` | What goes into the changelog | *Which commits appear, in which section, and where do links come from?* | new |

## Reference (generated, never hand-edited)

| File | Title | Question | Status |
|---|---|---|---|
| `docs/reference/commands/*.md` | Commands | *What flags does this command take, what does it exit with?* From cobra, one page per command. | generated |
| `docs/reference/configuration.md` | Configuration | *What does this key do, what values does it accept?* From `schema.json` descriptions, with the sample. | generated |
| `docs/reference/environment-variables.md` | Environment variables | *Which env vars does heraut read?* `HERAUT_FILE`, tokens, CI variables, update check. | new (hand-written, short) |
| `docs/reference/exit-codes.md` | Exit codes | *What does exit 2 mean in my CI log?* | new |
| `CHANGELOG.md` | Changelog | — | exists |

## Decisions (review of 2026-11-06)

1. **Install:** the README keeps Homebrew and `mise use packslip:…`; every other method moves to
   `docs/install.md`, with more detail.
2. **CI pages:** under `docs/guides/`, listed first in the guides.
3. **Generated reference:** both, each with a prerequisite task (see below).
4. **`docs/heraut.sample.yml`** stays as the commented full example, linked from the configuration
   reference.
5. **Order of work:** front doors + Getting started + the two CI guides first, then the other
   guides one task each.

## Generated reference: what it takes

**Commands (from cobra).** `cobra/doc` ships in the cobra module already required, so no new
dependency. The flag help is already written for users (`heraut release --help`). Only 2 commands
have a `Long:` description, so the generated pages would be thin: a task writes `Long:` (purpose,
an example) for each command first. That text improves `--help` and the docs at once. A test
regenerates the pages and fails on a diff.

**Configuration (from `schema.json`).** 128 keys: 15 have no description, 25 descriptions cite
ADRs, some run past 400 characters. A task rewrites them for users first (what the key does, its
default, one example value), and drops ADR references. IDEs show these descriptions on hover, so
editor users benefit too. The generated page is a lookup table; a short hand-written
`docs/reference/configuration.md` intro (where the file lives, how it is organised, a minimal
example) sits above it and links the sample. A test fails on drift.
