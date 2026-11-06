# Keep the release commit from running CI

`heraut release` pushes two things: the commit that updates `CHANGELOG.md`, and the tag. Each push can
start a pipeline. The commit's usually has nothing to do, since only the changelog changed, while the
tag's is often the one that builds and deploys the release. This guide skips the first and keeps the
second.

## GitLab: a push option

GitLab skips a push's pipeline when the push carries the `ci.skip` option. heraut sends options on
each of its two pushes separately:

```yaml
# .heraut.yml
git:
  push_options:
    commit: ["ci.skip"]   # the release commit: no pipeline
    tag: []               # the tag: its pipeline runs
```

The release commit's pipeline appears in GitLab as `skipped`; the tag's runs as usual. `--dry-run`
shows the option on the push:

```console
$ heraut release --dry-run
…
✓ [3/7] Commit changelog — [dry-run] would commit and push (--push-option=ci.skip)
…
```

Other GitLab push options work the same way, e.g. `ci.variable="DEPLOY=true"` on the tag push to
pass a variable to the tag's pipeline.

## Why not `[skip ci]` in the commit message

Putting `[skip ci]` in `versioning.commit_message` skips the commit's pipeline, and the tag's too:
the tag points at that same commit, and both GitLab and GitHub read the message. Use it only if no
pipeline should run for the release at all.

## GitHub: filter in the workflow

GitHub accepts push options but ignores them, and has none that skips a workflow. Skip the release
commit's run in the workflow that would start, not in heraut. Two ways:

**A path filter.** A push that changes nothing but the changelog doesn't start the workflow. GitHub
doesn't apply path filters to tags, so the tag's run still starts:

```yaml
on:
  push:
    branches: [main]
    paths-ignore: [CHANGELOG.md]   # your changelog.output
    tags: ["*"]
```

This works when the feature commits are already on GitHub before heraut runs, as they are when
heraut runs in CI: otherwise the push carries them too, and changes more than the changelog.

**A job condition.** The run starts, and its job is skipped for the release commit:

```yaml
jobs:
  build:
    if: "github.ref_type == 'tag' || !startsWith(github.event.head_commit.message, 'chore(release):')"
```

Match the start of your `versioning.commit_message` (`chore(release):` by default). Keep the
`github.ref_type == 'tag'` part: a tag push carries the release commit as its head commit, so without
it the tag's run is skipped too.

If heraut runs in GitHub Actions with the workflow's own token, you may need neither: pushes made
with that token start no workflow at all, the tag's included. See
[Release from GitHub Actions](github-actions.md).

## Per environment

With several environments, each can set its own lists. A list left out keeps the top-level one, and
`[]` clears it:

```yaml
git:
  push_options:
    commit: ["ci.skip"]
environments:
  prod:
    bump: promote
    source: staging
    git:
      push_options:
        commit: []        # prod's release commit runs its pipeline
```

## When it fails

| Message | Cause | Fix |
|---|---|---|
| `the receiving end does not support push options` | the remote doesn't accept push options | remove `git.push_options`, or use a remote that does |
| both pipelines skipped | `[skip ci]` in `versioning.commit_message` | remove it and use `push_options.commit` (GitLab) or a workflow filter (GitHub) |
| the tag's GitHub run is skipped too | a job condition without the `github.ref_type == 'tag'` part | add it |
