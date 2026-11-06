# Release from GitHub Actions

This guide sets up a workflow that releases your project when you click **Run workflow**. It assumes
your config already works locally ([Your first release](../getting-started.md)).

## The workflow

`.github/workflows/release.yml`:

```yaml
name: Release

on:
  workflow_dispatch:

concurrency: release

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write        # push the changelog commit and the tag, create the release
      pull-requests: read    # PR links and authors in the changelog
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0     # all commits and tags: heraut needs them to find the version

      - name: Install heraut
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          gh release download v1.2.3 --repo adaouat/heraut \
            --pattern 'heraut_*_linux_amd64' --output /usr/local/bin/heraut
          chmod +x /usr/local/bin/heraut

      - name: Git identity for the changelog commit
        run: |
          git config --global user.name "github-actions[bot]"
          git config --global user.email "41898283+github-actions[bot]@users.noreply.github.com"

      - name: Release
        env:
          GITHUB_TOKEN: ${{ github.token }}
        run: |
          heraut check
          heraut release
```

Replace `v1.2.3` with the heraut version you want. Pinning it means a new heraut release never
changes your release job by surprise; upgrade on purpose.

## What each part is for

**`workflow_dispatch`** runs the workflow from the **Actions** tab, when you choose to release.
You can trigger it on every push to `main` instead (`on: push: branches: [main]`), but then every
merge publishes a release.

**`concurrency: release`** stops two runs from releasing at the same time and racing for the same
tag.

**`permissions`**: `contents: write` lets the job push to the repository and create the release.
`pull-requests: read` lets heraut look up the pull request of each commit. Without it the changelog
still works, with a warning and without PR links.

**`fetch-depth: 0`** is the one setting you can't skip. By default `actions/checkout` fetches one
commit and no tags, and without them heraut can't tell which version comes next, so it stops with
`the repository is a shallow clone` before changing anything. With `fetch-depth: 0` it sees every
commit and tag.

**Install heraut**: `gh` is already on GitHub's runners, so this downloads the Linux binary of the
pinned release. Using mise instead? `jdx/mise-action` with
`"packslip:github.com/adaouat/heraut" = "1.2.3"` in your `mise.toml` installs it and checks its
signature.

**Git identity**: the runner has no git user, and the changelog commit needs one. The values above
make the commit show as GitHub Actions'; use your own bot's if you have one.

**`GITHUB_TOKEN`**: heraut publishes the release and reads pull request data with it. The git push
uses the token `actions/checkout` stored, the same one by default.

**`heraut check`** validates the config and the token before anything is written, so a mistake fails
the job at the start.

## The release commit doesn't start another workflow

A push made with the workflow's own token never starts another workflow run. The changelog commit
and the tag that `heraut release` pushes don't run your CI, and don't run a workflow listening for
tags either.

If you use a different token (see below), they do. To keep the release commit from running your CI
while the tag still runs it, filter on the path in the CI workflow (not the release one). GitHub
doesn't apply path filters to tag pushes, so the tag's run still starts:

```yaml
on:
  push:
    branches: [main]
    paths-ignore: [CHANGELOG.md]
    tags: ["*"]
```

Another way, and the details: [Keep the release commit from running CI](skip-ci-on-release-commit.md).

## A protected `main`

If a branch rule or ruleset requires pull requests on `main`, the push of the changelog commit is
refused (`GH006: Protected branch update failed`, or `push declined due to repository rule
violations`). Two ways out:

- **Let the release bypass the rule.** In the ruleset, add a bypass for a GitHub App you own, then
  generate its token in the job (`actions/create-github-app-token`) and pass it to `actions/checkout`
  as `token:` and to heraut as `GITHUB_TOKEN`.
- **Don't commit the changelog.** Without a `changelog:` block heraut commits nothing: it tags and
  publishes the release with its notes. The changelog then lives in the releases page only.

## When it fails

| Message | Cause | Fix |
|---|---|---|
| `the repository is a shallow clone` | the default checkout, one commit and no tags | `fetch-depth: 0` |
| `fatal: empty ident name` or `Please tell me who you are` | no git identity | the identity step |
| `403` or `Resource not accessible by integration` when creating the release | `contents: write` missing | the `permissions:` block |
| `GH006: Protected branch update failed` | protected `main` | see above |
| a warning that PR data could not be fetched | `pull-requests: read` missing, or the token can't see the PRs | add the permission |

If a release stops halfway, [Recover from a failed release](failure-recovery.md) says what is done and
how to finish.
