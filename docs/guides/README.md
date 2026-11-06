# Guides

Each guide answers one situation. The [docs home](../README.md) lists them by question.

## Release from CI

| Guide | Covers |
|---|---|
| [Release from GitHub Actions](github-actions.md) | A workflow you run from the Actions tab |
| [Release from GitLab CI/CD](gitlab-ci.md) | A manual job on your default branch |
| [Publish to GitHub from GitLab CI/CD](github-from-gitlab-ci.md) | A GitLab project released on GitHub, or on both |
| [Self-hosted GitLab](self-hosted-gitlab.md) | Your own GitLab server, in its CI and on your machine |
| [Keep the release commit from running CI](skip-ci-on-release-commit.md) | Skip the release commit's pipeline, keep the tag's |

## Versions

| Guide | Covers |
|---|---|
| [Several environments](environments.md) | One version promoted through dev, staging and prod |
| [Pre-releases](pre-releases.md) | Release candidates (`v1.4.0-rc.1`) before the final version |
| [Maintenance branches](maintenance-branches.md) | Fixes on older version lines, and branches named after their version |
| [Mobile apps: build numbers in tags](mobile-ci-tagging.md) | One tag per CI build: `uat/7.4.1+158404` |

## The release itself

| Guide | Covers |
|---|---|
| [Attach files to a release](release-assets.md) | Binaries, archives and checksums on GitHub and GitLab |
| [Run your own steps during a release](release-pipeline-and-hooks.md) | Hooks: where they run, their values, committing the files they change |
| [Recover from a failed release](failure-recovery.md) | What a failed run leaves, and how to finish or undo it |

## Changelog and notes

| Guide | Covers |
|---|---|
| [Pull request data in the changelog](pr-data-and-tokens.md) | PR links, authors and New Contributors, and the token each forge needs |
| [Link tickets](tickets.md) | Jira or issue numbers in commit messages, turned into links |
| [Customize the changelog and release notes](template-customization.md) | Template blocks, where to set them, full template files |
| [Release notes for an app store](store-release-notes.md) | One tag's notes as Markdown or plain text |

## Commits and agents

| Guide | Covers |
|---|---|
| [Check commit messages](commit-conventions.md) | Conventional Commits in a hook, on pull requests, and interactively |
| [Let an AI agent drive heraut](ai-agents.md) | The bundled skill and the `--json` output |
