# Several environments

Some projects ship the same version through several environments: built and tagged for `dev`,
then promoted to `staging`, then to `prod`. heraut gives each environment its own tags
(`dev/1.4.0`, `staging/1.4.0`, `prod/1.4.0`) and checks that promotions go forward.

## The configuration

```yaml
version: "1"

versioning:
  strategy: semver-per-env        # or calver-per-env, see below
  tag_format: "{env}/{version}"   # dev/1.4.0, staging/1.4.0, prod/1.4.0

changelog:
  output: CHANGELOG.md

environments:
  dev:
    branch: develop
    bump: auto                    # computes new versions from the commits
    disable_changelog: true       # tags only, no changelog commit
  staging:
    branch: main
    bump: promote                 # copies dev's latest version
    source: dev
  prod:
    branch: main
    bump: promote
    source: staging               # prod ← staging ← dev
```

Every command that resolves a version takes `--env`:

```bash
heraut version next --env dev       # dev/0.1.0
heraut release --env dev
heraut release --env staging        # staging/0.1.0, the same version
heraut release --env prod
```

This configuration has no `release:` block, so each `heraut release` tags only: changelog, commit,
tag and push, with a warning that publishing is off. Add `release: {}` to also publish each
environment's release to the forge.

## Two ways to get a version

**`bump: auto`** works like a single-environment release: it reads the commits since the
environment's last tag and bumps the version (`feat:` the minor, `fix:` the patch). Usually one
environment does this, the first in the chain.

**`bump: promote`** takes the latest version of its `source` environment and tags it under its own
name: `dev/1.4.0` becomes `staging/1.4.0`. Nothing is recomputed. `source` can itself promote from
another environment, which makes a chain. With a single `auto` environment, `source` can be left out.

## What each environment writes

Each environment's release can differ:

| Key under `environments.<name>` | Effect |
|---|---|
| `disable_changelog: true` | No changelog and no changelog commit: only the tag, and the release if there is one. |
| `disable_release: true` | No release notes and no publishing for this environment. |
| `changelog:` | Changes some changelog settings for this environment, e.g. its `output` file. |
| `release: targets:` | Publishes this environment's releases elsewhere, e.g. `dev` to GitLab only, `prod` to GitLab and GitHub. |
| `git: push_options:` | Its own push options; see [Keep the release commit from running CI](skip-ci-on-release-commit.md). |

## Branches

`branch` ties an environment to a branch. heraut then refuses to release it from anywhere else:

```text
Environment "dev" must be operated from branch "develop", but the current branch is "main" (use --force to override).
```

In CI, where the job checks out a commit rather than a branch, the current branch is the one the job
runs for (`CI_COMMIT_BRANCH`, `GITHUB_REF_NAME`, `BUILD_SOURCEBRANCH`), so a pipeline on `main` can
release `prod` as it is.

With branches set, `--env auto` picks the environment from the current branch. When two
environments share a branch, as `staging` and `prod` do above, name it with `--env`.

## The promotion guards

A promotion can be refused for three reasons. Each message names the tags involved and what to do.
The exit code is 4.

| Code | What happened | `--force` |
|---|---|---|
| `E001` | The tag already exists: this version was already promoted to this environment. | Bypasses it |
| `E002` | The destination is already at a higher version, e.g. `prod/0.2.0` exists and you promote `0.1.0`. Often a fix that went straight to `prod`. | Bypasses it |
| `E003` | The source environment has no release yet: there is nothing to promote. | No effect |

For example, promoting to `prod` before anything reached `staging`:

```text
error[E003]: no source tags found

  Cannot promote to prod: no staging/* tags exist in this repository.
```

`--force` also lets you release an environment from another branch than its own.

## CalVer per environment

`calver-per-env` works the same way with dates instead of commit types. The `auto` environment
takes this month's next number; `promote` copies it:

```yaml
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"
  tag_format: "{env}/{version}"

environments:
  dev:
    bump: auto       # dev/2026.11.0, dev/2026.11.1, …
  prod:
    bump: promote    # prod/2026.11.1
```

## In CI

Pass the environment to the release job, for instance from a variable per job or per branch:

```bash
heraut release --env "$DEPLOY_ENV"
```

The setup itself is the one in [Release from GitHub Actions](github-actions.md) or
[Release from GitLab CI/CD](gitlab-ci.md). For build numbers in tags (`uat/7.4.1+158404`), see
[Mobile apps: build numbers in tags](mobile-ci-tagging.md).
