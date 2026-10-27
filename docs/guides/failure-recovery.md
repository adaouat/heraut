# Recovering from a failed release

`heraut release` does not roll back. A step that fails stops the run, and everything that already
happened stays (Spec 02 § Failure semantics, [ADR-0053](../adr/0053-release-lifecycle-hooks.md)).
This guide lists the states a failed run can leave and how to get out of each one. It never asks you
to rewrite history, and every command below was run against a GitHub and a GitLab test repository.

heraut always pushes to the remote named `origin` ([Spec 03](../specs/03-commands.md#the-remote-is-always-origin)),
so the commands below use that name.

The examples use the tag `v1.2.3`, the GitHub repository `acme/widget` and the GitLab project
`acme/widget`; substitute your own. The forge commands use the `gh` and `glab` CLIs, which heraut
itself does not need: install them for the recovery, or do the same in the forge's web UI.

## 1. Find out what exists

A release is made of five things, in this order: the changelog commit on your branch, the local tag,
the tag on the remote, the forge release, and the hooks around them. Check each:

```bash
git log origin/main..HEAD --oneline        # commits not pushed yet (use your branch)
git tag -l v1.2.3                          # local tag
git ls-remote --tags origin v1.2.3         # tag on the remote
gh release view v1.2.3 --repo acme/widget  # GitHub release (exits non-zero when missing)
glab release view v1.2.3 -R acme/widget    # GitLab release
```

Then pick the state that matches:

| Commit pushed | Tag on remote | Release | State | Go to |
|:---:|:---:|:---:|---|---|
| yes | no (and no local tag) | no | the run stopped before the tag | [A](#a-commit-pushed-no-tag) |
| yes | no, but a local tag exists | no | the tag push failed | [B](#b-local-tag-only) |
| yes | yes | no | publishing failed | [C](#c-tag-pushed-no-release) |
| yes | yes | on some forges | one target failed | [D](#d-release-on-one-forge-only) |
| yes | yes | yes | a hook failed | [E](#e-a-hook-failed-after-the-release-exists) |

## A. Commit pushed, no tag

Typical cause: a `pre_tag` hook failed, creating the tag failed, or a release asset pattern matched no file
(that check runs before the changelog commit, so the run stops with nothing committed and a clear message).

Fix the cause, then run `heraut release` again. The version is recomputed and comes out the same,
because the `chore(release)` commit is not a releasable commit. If the regenerated changelog is
byte-identical, heraut skips the commit with a warning and goes on to tag and publish. The changelog
footer carries a timestamp to the minute, so a retry in a later minute commits the file again: you
get a second `chore(release): 1.2.3` commit, which is harmless.

If the hook that failed is not worth fixing right now, skip it for the retry:

```bash
heraut release --skip-hook pre_tag
```

## B. Local tag only

Typical cause: the tag push failed (network, permissions, a protected-tag rule).

`heraut release` will not help here: the tag exists and no commit is newer, so it stops with
`no commits since v1.2.3`. Push the tag yourself, then continue with [C](#c-tag-pushed-no-release):

```bash
git push origin v1.2.3
```

## C. Tag pushed, no release

Typical cause: the forge call failed (token, scope, outage). A release asset pattern that matches nothing is not a cause: it is refused before anything is pushed (state A). No heraut command publishes a release
for an existing tag, so create it with the forge's CLI, using the notes heraut generates:

```bash
heraut notes v1.2.3 --output notes.md
```

GitHub:

```bash
gh release create v1.2.3 --verify-tag --notes-file notes.md --repo acme/widget
```

GitLab:

```bash
glab release create v1.2.3 -F notes.md -R acme/widget
```

Add `--prerelease` or `--draft` on GitHub if your `release.targets` entry asked for them. Two things
to know:

- `--verify-tag` makes `gh` refuse when the tag is not on the remote. Without it `gh` would create the
  tag from the default branch. `glab` has no such flag and **does** create a missing tag, on the
  default branch's latest commit, which is rarely the release commit. Check the tag is on the remote
  (step 1) before running `glab release create`.
- `heraut notes` regenerates the notes, with PR/MR attribution when the token allows it; add `--offline`
  to skip the attribution.

Assets are not republished by these commands. Add them afterwards:

```bash
gh release upload v1.2.3 dist/app.bin --repo acme/widget
glab release create v1.2.3 dist/app.bin -F notes.md -R acme/widget    # GitLab: pass the files here
```

On GitLab the files heraut publishes live in the project's Generic Packages registry and are linked
from the release (ADR-0069). `glab release create <files>` attaches them as plain release links
instead (link type `other`, not a package link). If that matters, delete the release (see
[Undo a release](#undo-a-release)) and let `heraut release` publish it again after fixing the cause.

## D. Release on one forge only

With several `release.targets`, heraut publishes them in order and stops at the first publish
failure; the output shows which targets were released. The tag is shared, so for each missing target
follow [C](#c-tag-pushed-no-release) on that forge. A failing `pre_release` hook is gentler: it skips
only that target and the loop goes on, so the same recovery applies to the skipped target.

## E. A hook failed after the release exists

A failing `post_release` hook is reported as a failure of that target, but the release is already
published and nothing needs undoing. Do not run `heraut release` again (it would stop with
`no commits since v1.2.3`). Run the hook's command by hand, with the template variables filled in yourself
(`{{ .Tag }}` becomes `v1.2.3`; the failed run printed the rendered command):

```bash
echo "notifying v1.2.3"        # the hook's own command
```

If a `pre_release` hook failed and the target was skipped, publish it with [C](#c-tag-pushed-no-release)
and then run the `post_release` command by hand.

## Undo a release

Only do this when the release must be created again from scratch, for example to republish assets
with the right layout, or because the notes were wrong. Delete the release and the tag together:

```bash
gh release delete v1.2.3 --yes --cleanup-tag --repo acme/widget     # GitHub: release and remote tag
glab release delete v1.2.3 -y --with-tag -R acme/widget             # GitLab: release and remote tag
git tag -d v1.2.3                                                   # your local tag
```

`gh --cleanup-tag` removes the tag only when the release exists. If you removed the release earlier
(or it never existed), delete the tag with `git push origin --delete v1.2.3`. The changelog commit
stays on your branch; leave it, the next run regenerates the same version on top of it (state
[A](#a-commit-pushed-no-tag)).

## What cannot be undone

- **A tag a pipeline has already consumed.** If CI builds, publishes a package or deploys on a tag
  push, deleting the tag afterwards does not take the artifacts back. Treat that version as spent and
  release the next one.
- **A tag someone has fetched.** Deleting and recreating a tag on another commit leaves their clones
  on the old one. Prefer a new version over moving a tag.
- **A published package or container image** pushed by a hook or by CI, which usually refuses to
  overwrite a version.
- **The changelog commit** once others have pulled it. Do not rewrite it; the next release carries
  on from it.
