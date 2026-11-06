# heraut documentation

heraut computes your next version, writes the changelog and release notes, tags, and publishes the
release to GitHub or GitLab. Start where your question is.

## Get started

- [Install](install.md): Homebrew, mise, a prebuilt binary, Docker or `go install`, and how to upgrade.
- [Your first release](getting-started.md): from no config to a published release, step by step.

## Release from CI

- [GitHub Actions](guides/github-actions.md): a workflow you run from the Actions tab.
- [GitLab CI/CD](guides/gitlab-ci.md): a manual job on your default branch.

## Guides

Each guide answers one situation.

| I want to… | Guide |
|---|---|
| Publish to GitHub from a GitLab pipeline, or to both | [Publish to GitHub from GitLab CI/CD](guides/github-from-gitlab-ci.md) |
| Release on my company's own GitLab server | [Self-hosted GitLab](guides/self-hosted-gitlab.md) |
| Stop the release commit from running my CI, but keep the tag's pipeline | [Keep the release commit from running CI](guides/skip-ci-on-release-commit.md) |
| Promote one version through dev, staging and prod | [Several environments](guides/environments.md) |
| Tag every CI build of a mobile app with its build number | [Mobile apps: build numbers in tags](guides/mobile-ci-tagging.md) |
| Release a fix on an older version line | [Maintenance branches](guides/maintenance-branches.md) |
| Run my own scripts during a release | [Release pipeline and hooks](guides/release-pipeline-and-hooks.md) |
| Change what the changelog and release notes look like | [Template customization](guides/template-customization.md) |
| Get one release's notes as plain text for an app store | [Release notes for an app store](guides/store-release-notes.md) |
| Finish or undo a release that failed halfway | [Recover from a failed release](guides/failure-recovery.md) |

## Reference

- [Configuration](reference/configuration.md): where the file lives and every key, with its type and
  default.
- [`heraut.sample.yml`](heraut.sample.yml): every configuration key in context, with a comment.
- [Commands](reference/commands/README.md): every command, its flags and examples (the same text as
  `heraut <command> --help`).
- [Environment variables](reference/environment-variables.md): the tokens, the `HERAUT_` settings
  and the CI variables heraut reads.
- [Exit codes](reference/exit-codes.md): what each code means and what to do about it.
- [Changelog](../CHANGELOG.md): what changed in each version.

---

Working on heraut itself? See [CONTRIBUTING.md](../CONTRIBUTING.md).
