# How the next version is found

`heraut version next` prints the version a release would create, without changing anything. This page
explains where it comes from, so a surprising version has an explanation.

## SemVer: the last release, plus your commits

heraut starts from the **last release tag** and looks at every commit since:

1. **The last release** is the highest version among your tags (`v1.2.0`), compared as versions, so
   `v1.10.0` comes after `v1.9.0`. Pre-releases (`v1.3.0-rc.1`) are skipped: the next final is
   computed from the last final. Tags that aren't versions are ignored.
2. **Each commit** since then gives a level:

   | Commit | Level |
   |---|---|
   | `feat!: …`, `fix(api)!: …`, or a `BREAKING CHANGE:` footer | major |
   | `feat: …` | minor |
   | any other type: `fix`, `docs`, `chore`, `refactor`, `ci`, … | patch |
   | a message that isn't a conventional commit (`Update readme`) | none |

3. **The release takes the highest level**: one `feat:` among ten `fix:` commits makes it a minor.

With `v1.2.0` as the last release:

| Commits since | Next |
|---|---|
| `fix: a` | `v1.2.1` |
| `fix: a`, `feat: b` | `v1.3.0` |
| `feat!: a`, or a `BREAKING CHANGE:` footer | `v2.0.0` |
| `docs: a`, or `chore: a` | `v1.2.1` |
| only `Update readme` | refused: no releasable commits (exit 5) |
| nothing | refused: no commits since `v1.2.0` (exit 5) |

The first release, with no tag yet, is `0.1.0`, or `versioning.initial_version`.

## Changing the levels

`versioning.bump.overrides` gives some commits another level. The first rule that matches wins:

```yaml
versioning:
  bump:
    overrides:
      - type: chore
        bump: none             # chores alone never release
      - regex: '^docs\(readme\)'
        bump: none
```

A release where every commit is `none` is refused, with the list of commits:

```text
No releasable commits since v1.2.0: 1 commit(s) since then are excluded from the version bump
  - chore: tidy
```

## Majors you decide on

Two settings stop a major from happening by accident:

- **`stay_at_v0: true`**: while the version is `0.x`, a breaking change gives a minor
  (`v0.4.0` → `v0.5.0`), with a warning. Once you reach `1.0.0` the setting does nothing.
- **`confirm_major: true`**: at any version, a major is refused until you run the command again with
  `--allow-major`. `heraut version next` still prints it, with a warning.

`--allow-major` lifts both for one run.

## Choosing the version yourself

`--set-version` skips the computation:

```bash
heraut release --set-version 2.0.0
```

The value must be a valid version for the strategy, and its tag mustn't exist yet. To always choose the
version yourself, set `versioning.bump.mode: manual`: every release then needs `--set-version`.

## CalVer: the date, then a counter

CalVer doesn't read commit messages. The next version is today's date in your `format`, and `PATCH`
counts the releases in that period:

| Last tag | Today | `YYYY.MM.PATCH` gives |
|---|---|---|
| none | 2026-11-06 | `2026.11.0` |
| `2026.11.0` | 2026-11-20 | `2026.11.1` |
| `2026.11.1` | 2026-12-02 | `2026.12.0`: a new month starts again at `0` |

A CalVer release needs a commit since the last tag, as SemVer does: run twice with nothing new and the
second run stops with `no commits since 2026.11.1` (exit 5). Dates are read in UTC. With `SPRINT` in the format, the
period is the sprint number in `versioning.sprint`, which you advance with
`heraut version sprint bump`.

## Environments and branches

With a `-per-env` strategy, an environment with `bump: auto` follows the rules above within its own
tags, and one with `bump: promote` copies another environment's version
([Several environments](../guides/environments.md)). On a maintenance branch, the last release is the
latest tag of that line, not of the whole repository
([Maintenance branches](../guides/maintenance-branches.md)).

## See why

```console
$ heraut version next --json
{"ok":true,"data":{"tag":"v1.3.0","version":"1.3.0","current_tag":"v1.2.0","bump":"minor"},"warnings":[]}
```

`current_tag` is the starting point and `bump` the level the commits gave. `heraut commit check
--from-latest-tag` lists the commits since that tag that aren't conventional.
