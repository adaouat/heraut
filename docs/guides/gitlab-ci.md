# Release from GitLab CI/CD

This guide adds a release job you start by hand from a pipeline on your default branch. It assumes
your config already works locally ([Your first release](../getting-started.md)).

## One setting first

The job pushes the changelog commit and the tag with the pipeline's own token, `CI_JOB_TOKEN`. GitLab
refuses that until you allow it: **Settings › CI/CD › Job token permissions**, turn on **Allow Git
push requests to the repository**.

## The job

In `.gitlab-ci.yml`:

```yaml
stages:
  - release

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
    GIT_DEPTH: "0"    # all commits and tags: heraut needs them to find the version
  script:
    - git config user.name "Release Bot"
    - git config user.email "release-bot@example.com"
    - heraut check
    - heraut release
```

Replace `1.2.3` with the heraut version you want. Pinning it means a new heraut release never changes
your release job by surprise; upgrade on purpose.

## What each part is for

**The image** is heraut's own, with `git`. Its entry point runs `heraut`, so `entrypoint: [""]` gives
GitLab the shell it needs to run `script:`. Using your own image instead? Install the binary in it
([Install](../install.md)); heraut needs only `git` next to it.

**`rules:`** adds the job to pipelines on the default branch, as a manual job: it waits for you to
press play. Drop `when: manual` to release on every push to the default branch. Keep the job in
branch pipelines: GitLab checks out the pipeline's commit rather than its branch, and heraut pushes
the changelog commit to the branch the pipeline runs for (`CI_COMMIT_BRANCH`). A tag or merge request
pipeline has no such branch, so heraut stops there before writing anything.

**`resource_group: release`** runs one release job at a time, so two runs never race for the same tag.

**`GIT_DEPTH: "0"`** is the one setting you can't skip. GitLab clones with a limited depth by default;
heraut then may not see the last release tag, starts over at your first version, and the tag push
fails because that tag already exists. `0` clones the whole history.

**Git identity**: the runner has no git user, and the changelog commit needs one.

**Tokens**: none to set. In GitLab CI heraut finds the project from the pipeline and uses
`CI_JOB_TOKEN` to create the release and read merge request data; the push uses the same token, through
the clone's remote.

**`heraut check`** validates the config and the token before anything is written, so a mistake fails
the job at the start.

## Protected branches and tags

The job token acts as the user who runs the pipeline. If your default branch is protected, that user
must be allowed to push to it; if your tags are protected (`v*`, for example), allowed to create them.
Merge-only branches block the changelog commit for everyone. Two ways out:

- **Push with a project access token.** Create one with the `write_repository` scope and a role allowed
  to push to the branch, store it as a masked CI/CD variable (`RELEASE_TOKEN`), and point the remote
  at it before releasing:

  ```yaml
  - git remote set-url origin "https://release:${RELEASE_TOKEN}@${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git"
  ```

  heraut still creates the release with the job token; only the push changes.
- **Don't commit the changelog.** Without a `changelog:` block heraut commits nothing: it tags and
  publishes the release with its notes.

## The pipelines the release starts

The changelog commit and the tag are pushes, so they can start pipelines of their own. To skip the
commit's pipeline and keep the tag's (to build and deploy the release, say), ask GitLab with a push
option:

```yaml
# .heraut.yml
git:
  push_options:
    commit: ["ci.skip"]
```

## When it fails

| Message | Cause | Fix |
|---|---|---|
| the next version is your first version again, then the tag push fails | shallow clone | `GIT_DEPTH: "0"` |
| `HEAD is detached and no CI branch variable names the branch` | the job ran in a tag or merge request pipeline | run it in a branch pipeline (the `rules:` above) |
| `fatal: empty ident name` or `Please tell me who you are` | no git identity | the two `git config` lines |
| `You are not allowed to push code to this project` | job token push not allowed | the setting at the top |
| `You are not allowed to push code to protected branches` | protected branch | see above |
| `! [rejected] … (fetch first)` | new commits landed on the branch since the pipeline started | run the job again from the newest pipeline |

If a release stops halfway, [Recover from a failed release](failure-recovery.md) says what is done and
how to finish.
