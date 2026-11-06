# Pre-releases

A pre-release is a version you publish before the real one, for testers: `v1.4.0-rc.1`, then
`v1.4.0-rc.2`, then `v1.4.0`. heraut numbers the series for you and keeps it out of `CHANGELOG.md`
until the final release.

Pre-releases work with `strategy: semver`.

## Cut one

```bash
heraut release --pre-release rc
```

heraut computes the next version as for a normal release (`v1.4.0` after a `feat:` since `v1.3.0`)
and adds the label and a counter: `v1.4.0-rc.1`. The next run with the same label gives
`v1.4.0-rc.2`. To see the tag first:

```console
$ heraut version next --pre-release rc
v1.4.0-rc.1
```

The label is yours: `alpha`, `beta`, `rc`, `preview`… heraut always adds `.1`, `.2`, … after it.

## What a pre-release writes

```console
$ heraut release --pre-release rc --dry-run
✓ [1/5] Resolve version — v1.4.0-rc.1
✓ [2/5] Create tag v1.4.0-rc.1 — [dry-run] would tag
✓ [3/5] Push tag — [dry-run] would push v1.4.0-rc.1
✓ [4/5] Generate release notes — [dry-run] would generate
✓ [5/5] Publish to github — [dry-run] would create release
```

- **No changelog entry and no commit.** `CHANGELOG.md` is left alone; the final release lists every
  commit of the series in its own section.
- **A tag and a release.** The release notes of `rc.2` cover the commits since `rc.1`.
- **On GitHub, the release is marked as a pre-release.** GitLab has no such marker; the release is
  published like any other.

## Ship the final version

When the last candidate is good, release normally:

```console
$ heraut version next
v1.4.0
```

No new commit is needed: `v1.4.0` is tagged on the same commit as the last candidate, and its
changelog section and release notes cover everything since `v1.3.0`.

`heraut version current` ignores pre-releases (`v1.3.0` until `v1.4.0` ships);
`heraut version current --include-pre-release` shows the latest candidate.

## Moving through labels

Labels sort alphabetically, as SemVer orders them: `alpha` < `beta` < `rc`. You can move up the
list on the same commit (`beta.2` → `rc.1`), but not back down:

```text
v1.4.0-beta.1 would sort below existing v1.4.0-rc.1 — ship 1.4.0 or use a label that sorts higher.
```

Cutting the same label again needs a new commit:

```text
No commits since v1.4.0-rc.1 — create at least one commit before running heraut release.
```

## When the series grows

If a `feat:` lands during a `v1.3.1-rc` series, the next candidate becomes `v1.4.0-rc.1`, with a
warning that names the commits that raised it. A breaking change is stopped instead, since it would
start a new major series:

```text
pre-release series v1.4.0-rc.2 would escalate to a new major 2.0.0: breaking change(s) since v1.3.0
— pass --allow-major to open the 2.0.0 series, or ship 1.4.0 first
  - feat!: new api
```

Pass `--allow-major` to start `v2.0.0-rc.1`, or ship `v1.4.0` first.

## Choosing the version yourself

`--set-version` accepts a pre-release too, for a series heraut didn't start:

```bash
heraut release --set-version 2.0.0-beta.1
```

It is treated the same way: no changelog entry, and a GitHub pre-release. It can't be combined
with `--pre-release`.
