# Maintenance branches

Cut hotfixes on a long-lived `release/x.y` branch while `main` moves on, and have heraut release
the next version *on that line* (`v1.3.2`), not on top of the newest tag in the repository
(`v2.0.1`).

Reference: [Spec 02 § `versioning.branches`](../specs/02-configuration.md#versioningbranches),
[Spec 04 § Maintenance branches](../specs/04-versioning.md#maintenance-branches), and the decision
record, [ADR-0065](../adr/0065-branch-aware-semver-resolution.md).

## When to use it

- You keep old lines alive (`release/1.3`) and tag fixes on them while `main` ships `v1.4`, `v2.0`.
- You use the `semver` strategy. `semver-per-env` and the CalVer strategies reject the block.

Without a `versioning.branches` block nothing changes: every branch resolves from the repository's
whole tag list, as before.

## Configuration

```yaml
versioning:
  strategy: semver
  branches:
    - name: main                  # release branch: global tags, as today
    - name: release/1.3           # maintenance branch, explicit range
      range: 1.3.x                # >=1.3.0 <1.4.0 (patch only)
    - name: "release/*"           # maintenance branch, range derived from the name
```

- `1.x` allows fixes and features up to the next major; `1.3.x` allows fixes only.
- A glob entry derives the range from the branch name's last segment: `release/1.3` and
  `release/1.3.x` give `1.3.x`, `release/1.x` gives `1.x`. A name with no version in it
  (`release/legacy`) is an error for automatic resolution.
- An exact name without `range` is a release branch (today's behaviour).
- On a branch matched by no entry, `heraut release` and `heraut changelog --tag` refuse to
  publish unless you pass `--force`. Previews (`version next`, `--dry-run`) keep working.
  `--force` is one flag with several meanings: it also bypasses the per-env promotion guards and
  downgrades `commits.enrichment_policy: required`. A CI job that passes `--force` for one of
  those reasons loses the unlisted-branch protection too, so run such jobs only on branches you
  intend to release from.

## What happens on a maintenance branch

Say `main` has `v1.3.0 → v1.3.1 → v1.4.0 → v2.0.0`, `release/1.3` was cut from `v1.3.1` and
`release/1.x` from `v1.4.0`, with `branches: [{name: main}, {name: "release/*"}]`.

| Branch / commit | Without `branches` | With `branches` |
|---|---|---|
| `release/1.3`, `fix: x` | `v2.0.1` | `v1.3.2`, notes `v1.3.1..v1.3.2` |
| `release/1.3`, `feat: y` | `v2.1.0` | error: 1.4.0 outside `1.3.x` |
| `release/1.x`, `feat: z` | `v2.1.0` | `v1.5.0` |
| `release/1.x`, `feat!: ...` | `v3.0.0` | error: 2.0.0 outside `1.x` |
| `main`, `fix: w` | `v2.0.1` | `v2.0.1` (unchanged) |
| `release/1.3`, `v1.3.2` already tagged elsewhere | collides | tag-exists error |
| `release/1.3`, `heraut version current` | `v2.0.0` | `v1.3.1` |
| `release/1.3`, `--pre-release rc` | `v2.0.1-rc.1` | `v1.3.2-rc.1` |
| `feature/foo`, `heraut release` | `v2.0.1` | unlisted-branch error (`--force` bypasses it) |

In CI a detached `HEAD` is fine: heraut falls back to `CI_COMMIT_BRANCH` (GitLab),
`GITHUB_REF_NAME` on branch refs (GitHub Actions) and `BUILD_SOURCEBRANCH` on `refs/heads/` refs
(Azure Pipelines — not `BUILD_SOURCEBRANCHNAME`, which holds only the ref's last segment).
On a GitHub `pull_request` run, `GITHUB_REF_NAME` is `<number>/merge`, which matches no entry, so
a PR preview (`version next`, `--dry-run`) shows the global version, not the target line's. That
is preview-only and safe: publishing from that ref is refused as unlisted.
The history of the line's base tag must be fetched: a shallow clone that lost it reports "no
release in range".

When the commit does not fit, the error names the range and the way out:
`feat: y would release 1.4.0, outside release/1.3 (>=1.3.0 <1.4.0) -- land it on a branch whose
range allows it, or on main`. If the branch has no tag in range yet (a fresh `release/1.3`), tag
its starting point or pass `--set-version`.

## Forward-merge and backport workflow

heraut neither merges nor cherry-picks; that stays a git workflow.

```bash
# fix on the maintenance line, release it there
git switch release/1.3
git commit -m "fix(widget): handle empty input"
heraut release                       # -> v1.3.2

# carry the fix to main (merge the line forward, or cherry-pick)
git switch main
git merge release/1.3
heraut changelog --regenerate        # v1.3.2 gets its own section: v1.3.1..v1.3.2
```

Changelog and release-notes bounds only use tags in a section's own history. Until `release/1.3`
is merged into `main`, `v1.3.2` has no section in `main`'s `CHANGELOG.md` and `v1.4.0` still
bounds at `v1.3.1`. After the merge, `v1.3.2` has its own section, and a later `main` section also
lists the merged-in fix, because it is in that section's history. This bounds-fix applies to every
`semver` repository, with or without a `branches` block; linear histories render identically.

## When the branch is the version

A client fixes the version up front: `release/7.8.0` is cut, `feat`/`fix` commits land there for
stabilisation, and the version never changes. That is not a maintenance line. Release it with the
manual escape hatch:

```bash
heraut release --set-version 7.8.0
heraut release --set-version 7.8.0 --set-build-id 158404    # repeat store builds: v7.8.0+158404
```

`--set-version` is not range-checked, and it works on a branch that matches a `release/*` glob
even though no range can be derived from `release/7.8.0`. A version that is already released —
`v7.8.0`, or a build-metadata tag such as `v7.8.0+158404` — still fails before anything is
written; use `--pre-release rc` or `--set-build-id` for re-releases (with `--set-build-id` only
that exact tag is checked). Automating this as
its own branch type is tracked as T348 in [`docs/tasks/roadmap.md`](../tasks/roadmap.md).
