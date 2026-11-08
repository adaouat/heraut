# Your first release

This page takes a project from no heraut config to a published release, from your own machine. Once
it works, move it to CI: [GitHub Actions](guides/github-actions.md) or
[GitLab CI/CD](guides/gitlab-ci.md). If the project already has release tags, a changelog or another
release tool, read [Adopt heraut on an existing project](guides/adopt-on-an-existing-project.md) too.

You need:

- heraut installed ([Install](install.md));
- a git repository whose `origin` is on GitHub or GitLab, and push access to it;
- commit messages that follow [Conventional Commits](https://www.conventionalcommits.org/)
  (`feat: …`, `fix: …`). heraut reads them to pick the next version and fill the changelog;
- a token that can create releases: `GITHUB_TOKEN` (or `GH_TOKEN`) for GitHub, `GITLAB_TOKEN` for
  GitLab.

The examples use a GitHub project, `acme/app`, with two commits: `feat: first version of the app`
and `fix: typo in the readme`.

## 1. Write the config

Create `.heraut.yml` at the root of the repository:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json
version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"

changelog:
  output: CHANGELOG.md

release:
  notes: {}
```

That's all heraut needs: versions like `v1.2.3`, a `CHANGELOG.md`, and a release with notes on the
forge your `origin` points to. heraut reads `git@github.com:acme/app.git` and knows it publishes to
GitHub, project `acme/app`.

Prefer questions to a blank file? `heraut init` asks them (strategy, prefix, changelog, where to
publish) and writes the file for you. `heraut init --defaults` writes this same file without asking
(with `release: {}`, which means the same as `notes: {}`) and says which forge it found.

Commit the file:

```bash
git add .heraut.yml
git commit -m "chore: add heraut config"
```

## 2. Check the config

```console
$ heraut check config
  .heraut.yml  (from .heraut.yml)
✓ config: ok
```

This runs offline. A typo, an unknown key or a missing field is reported here with its line and a
hint, instead of in the middle of a release:

```text
Config: line 4: unknown key "tag_prefx" in versioning — did you mean "tag_prefix"?
```

## 3. Check your machine

```console
$ heraut check runtime

Git

✓ git — git version 2.56.0
✓ git user.name — Ada Lovelace
✓ git user.email — ada@example.com

Platforms

✗ github — no GitHub token: none of GH_TOKEN, GITHUB_TOKEN is set

✗ 1 check(s) failed
```

Set the token and run it again. heraut calls the API with it, so an invalid token or one that can't
read the repository fails here too, not at the end of a release:

```bash
export GITHUB_TOKEN=…   # a token with write access to the repository's contents
heraut check runtime
```

## 4. See the next version

```console
$ heraut version next
v0.1.0
```

There is no tag yet, so heraut starts at `0.1.0`. After that, each release looks at the commits since
the last tag: a `fix:` bumps the patch (`v0.1.1`), a `feat:` the minor (`v0.2.0`), a breaking change
(`feat!:` or a `BREAKING CHANGE:` footer) the major, so the first breaking change takes `v0.x` to
`v1.0.0`. To stay below `1.0.0` until you decide otherwise, set `versioning.bump.stay_at_v0: true`.

## 5. Rehearse

`--dry-run` runs every step and changes nothing:

```console
$ heraut release --dry-run
✓ [1/7] Resolve version — v0.1.0
✓ [2/7] Generate changelog — [dry-run] would write CHANGELOG.md
✓ [3/7] Commit changelog — [dry-run] would commit and push
✓ [4/7] Create tag v0.1.0 — [dry-run] would tag
✓ [5/7] Push tag — [dry-run] would push v0.1.0
✓ [6/7] Generate release notes — [dry-run] would generate
✓ [7/7] Publish to github — [dry-run] would create release
```

To read the changelog before releasing, write it without committing anything:

```console
$ heraut changelog
✓ [1/2] Resolve version — v0.1.0
✓ [2/2] Generate changelog

Changelog updated for v0.1.0
```

```markdown
# Changelog

<!-- heraut-release: v0.1.0 -->
## [0.1.0] - 2026-11-06

### 🚀 Features

- First version of the app - 4e1c2a7


### 🐛 Bug Fixes

- Typo in the readme - b64e4d0


### ⚙️ Miscellaneous Tasks

- Add heraut config - 3023620
```

The HTML comments are markers heraut uses to add the next release in the right place; leave them in.
In CI and in the release notes, each hash links to its commit, and with a token each line gets its
pull request and author. Delete the file when you're done reading (`rm CHANGELOG.md`): the release
writes it again.

## 6. Release

```bash
heraut release
```

The same seven steps, for real:

1. picks `v0.1.0`;
2. writes `CHANGELOG.md`;
3. commits it as `chore(release): 0.1.0` and pushes the branch to `origin`;
4. creates the tag `v0.1.0`;
5. pushes the tag;
6. writes the release notes for this version;
7. creates the GitHub release `v0.1.0` with those notes.

Run it from the branch you release from (usually `main`), up to date with `origin`. If a step fails,
heraut stops there and says what is done and what isn't;
[Recover from a failed release](guides/failure-recovery.md) covers each case.

## Next

- Release from CI: [GitHub Actions](guides/github-actions.md) or [GitLab CI/CD](guides/gitlab-ci.md).
- Every guide, by situation: [the docs home](README.md).
- Every configuration key, with a comment: [`heraut.sample.yml`](heraut.sample.yml).
