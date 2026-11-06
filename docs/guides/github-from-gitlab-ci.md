# Publish to GitHub from GitLab CI/CD

Your code lives on GitLab and is mirrored to GitHub, and you want the release on GitHub, or on both.
This guide starts from a working GitLab release job ([Release from GitLab CI/CD](gitlab-ci.md)) and
adds GitHub to it.

Three things change compared with a GitLab-only release:

1. **A GitHub token.** The job's `CI_JOB_TOKEN` only works on GitLab.
2. **The tag must reach GitHub before the release is created.** heraut pushes to `origin`, your
   GitLab project. If the tag isn't on GitHub yet, GitHub creates it itself, on the head of its own
   default branch, which may be an older commit.
3. **Two forges in the configuration**, so heraut knows where to publish and where to read merge
   request data from.

## The configuration

```yaml
version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"

changelog:
  output: CHANGELOG.md

forges:
  - name: gitlab
    platform: gitlab          # host and project come from the CI job
  - name: github
    platform: github
    repository: acme/app      # the GitHub copy

commits:
  enrichment_forge: gitlab    # merge requests live on GitLab

release:
  targets:
    - forge: github

hooks:
  post_tag:
    - run: 'git push github "HEAD:refs/heads/$CI_COMMIT_BRANCH" "{{ .Tag }}"'
```

To publish on both, list both targets:

```yaml
release:
  targets:
    - forge: gitlab
    - forge: github
```

**`forges`** names the two platforms. The GitLab entry needs nothing else in a GitLab job: heraut
reads the host and the project from it. The GitHub entry needs the repository, since nothing in the
job names it.

**`commits.enrichment_forge`** says where to read merge request data (links, authors, new
contributors). With two forges it is required. Your merge requests are on GitLab, so it names the
GitLab forge.

**The `post_tag` hook** pushes the release commit and the tag to a remote called `github` (added in
the job below), right after heraut pushes them to GitLab and before it creates the GitHub release.
`$CI_COMMIT_BRANCH` is the branch the pipeline runs for.

Already mirroring with GitLab's push mirror? Keep the hook anyway. The mirror runs on its own
schedule and may not have pushed the tag when heraut publishes.

## The token

Create a fine-grained personal access token on GitHub (or use a GitHub App), limited to the
repository, with **Contents: read and write**. Store it in GitLab as a masked CI/CD variable named
`GH_TOKEN`. heraut reads `GH_TOKEN` to publish on GitHub, and the job uses it to push.

The token is only exposed to protected branches when the variable is protected; release from a
protected branch, or untick **Protect variable**.

## The job

The job from [Release from GitLab CI/CD](gitlab-ci.md), with one more line to add the `github`
remote:

```yaml
release:
  stage: release
  image:
    name: ghcr.io/adaouat/heraut:1.2.3
    entrypoint: [""]
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
      when: manual
  resource_group: release
  variables:
    GIT_DEPTH: "0"
  script:
    - git config user.name "Release Bot"
    - git config user.email "release-bot@example.com"
    - git remote add github "https://x-access-token:${GH_TOKEN}@github.com/acme/app.git"
    - heraut check
    - heraut release
```

`heraut check` fails early if `GH_TOKEN` is missing or can't read the GitHub repository.

## Which links point where

Each target's release notes are written for that target: the commit and compare links of the GitHub
notes point to github.com, and those of the GitLab notes to your GitLab. The merge request links
(`!12`), `@handles` and New Contributors come from `enrichment_forge`, so they point to GitLab in
both.

The `CHANGELOG.md` committed to the repository uses the job's own host, GitLab.

## When it fails

| Message | Cause | Fix |
|---|---|---|
| `no GitHub token: none of GH_TOKEN, GITHUB_TOKEN is set` | the variable isn't available to the job | check its name, and **Protect variable** against the branch |
| `fatal: 'github' does not appear to be a git repository` | the `post_tag` hook ran without the remote | the `git remote add github` line in the job |
| `remote: Permission to acme/app.git denied` or `403` | the token can't push | give it **Contents: read and write** on the repository |
| `commits.enrichment_forge: required when more than one forge is configured` | two forges and no enrichment forge | name the forge whose merge requests you want |
| the GitHub release points at an older commit | the tag wasn't on GitHub when heraut published | the `post_tag` hook |

If a release stops halfway, [Recover from a failed release](failure-recovery.md) says what is done and
how to finish.
