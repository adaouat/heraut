# How a release runs

`heraut release` runs a fixed sequence of steps, prints each one as it goes, and stops at the first
that fails. This page lists the steps, what each one writes, and what heraut never does.

## The steps of `heraut release`

```console
$ heraut release
✓ [1/7] Resolve version — v1.4.0
✓ [2/7] Generate changelog
✓ [3/7] Commit changelog
✓ [4/7] Create tag v1.4.0
✓ [5/7] Push tag
✓ [6/7] Generate release notes
✓ [7/7] Publish to github
```

| Step | What it does | What it changes |
|---|---|---|
| Check | Validates `.heraut.yml`, the branch rules, git and its identity, and that the history is complete. | Nothing. |
| Resolve version | Finds the last release tag and computes the next version from the commits since. | Nothing. |
| Generate changelog | Adds the new version's section to `CHANGELOG.md`. | The file, on disk. |
| Commit changelog | Commits the file (`chore(release): 1.4.0`) and pushes the branch to `origin`. | One commit, pushed. |
| Create tag | Tags that commit `v1.4.0`, annotated by default. | One tag, local. |
| Push tag | Pushes the tag to `origin`. | The tag, on the remote. |
| Generate release notes | Writes this version's notes. | Nothing on disk. |
| Publish | Creates the release on each target forge (GitHub, GitLab), with its files attached. | One release per target. |

Some steps drop out depending on the configuration: without a `changelog:` block there's no
changelog and no commit, and a pre-release never writes the changelog. Your own commands can run
between the steps; see [Run your own steps during a release](../guides/release-pipeline-and-hooks.md).

## `heraut changelog`

`heraut changelog` runs the first steps only, and stops where you tell it:

| Command | Stops after |
|---|---|
| `heraut changelog` | writing `CHANGELOG.md`: nothing is committed |
| `heraut changelog --commit` | committing and pushing the file |
| `heraut changelog --tag` | creating and pushing the tag |
| add `--no-push` | the same, without pushing |

It never publishes a release. Use it for projects that only tag, or to read the changelog before a
release.

## What heraut never does

heraut only adds to your repository. It runs `git add`, `git commit`, `git tag` and two plain
`git push origin`, and nothing else that writes:

- **no force push**: a push the remote refuses (new commits on the branch, a protected branch) stops
  the run;
- **no rewriting**: no amend, rebase or reset, so a commit heraut made stays as it is;
- **no deleting**: no tag, branch or release is ever removed, even after a failure;
- **no skipped git hooks**: your pre-commit and commit-msg hooks run on the release commit;
- **no rollback**: when a step fails, what already happened stays.
  [Recover from a failed release](../guides/failure-recovery.md) says how to finish or undo.

It pushes only to the remote named `origin`.

## Rehearse with `--dry-run`

```console
$ heraut release --dry-run
✓ [1/7] Resolve version — v1.4.0
✓ [2/7] Generate changelog — [dry-run] would write CHANGELOG.md
✓ [3/7] Commit changelog — [dry-run] would commit and push
✓ [4/7] Create tag v1.4.0 — [dry-run] would tag
✓ [5/7] Push tag — [dry-run] would push v1.4.0
✓ [6/7] Generate release notes — [dry-run] would generate
✓ [7/7] Publish to github — [dry-run] would create release
```

A dry run computes the real version and checks the configuration and the files to attach, then
prints each step instead of running it. It skips the branch rules, so you can preview from any
branch. It writes nothing, pushes nothing, calls no
forge API, and runs no hook. `heraut release --dry-run --json` gives the same plan as data.

## What runs where

| Where | What heraut does there |
|---|---|
| Your repository | reads the history and tags; writes `CHANGELOG.md`, one commit, one tag |
| `origin` | receives the commit and the tag |
| The forge's API | reads pull request data for the changelog, creates the release and uploads its files |
| GitHub, once a day | checks for a newer heraut, unless `HERAUT_CHECK_UPDATE=false` |
