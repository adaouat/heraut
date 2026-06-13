# Spec 04 — Versioning Strategies

Heraut supports four versioning strategies. The strategy is selected in
`.heraut.yml` (`versioning.strategy:`) and determines how the next version is computed
and what tag format is used.

| Strategy          | Picks bump from         | Tag namespace                | Use case                                  |
|-------------------|-------------------------|------------------------------|-------------------------------------------|
| `semver`          | Conventional commits    | Single (e.g. `v1.2.3`)       | Standard SemVer projects                  |
| `calver`          | Calendar + PATCH counter| Single (e.g. `2026.05.0`)    | Date-driven release cadence               |
| `semver-per-env`  | Conventional commits    | Per-env (e.g. `dev/1.2.3`)   | Multi-env promotion with SemVer arithmetic |
| `calver-per-env`  | Calendar + PATCH counter| Per-env (e.g. `dev/2026.05.0`)| Multi-env promotion with date-driven      |

The strategy selector is implemented in `internal/app/resolver.go` (`app.NewResolver`).

---

## SemVer

```yaml
versioning:
  strategy: semver
  tag_prefix: "v"                   # tag prefix, default "v"
  initial_version: "0.1.0"
  bump: auto                    # auto | manual
```

Version is inferred from [Conventional Commits](https://www.conventionalcommits.org/)
since the last tag.

### Bump determination

| Commit pattern                                    | Bump level |
|---------------------------------------------------|------------|
| `type!:` / `type(scope)!:` prefix (e.g. `feat!:`, `fix(api)!:`) or a `BREAKING CHANGE:` / `BREAKING-CHANGE:` footer | major |
| Any `feat:` commit                                | minor      |
| Any `fix:` commit                                 | patch      |
| Only chore/docs/refactor/style/test/ci commits    | patch (fallback) |
| No commits since last tag                         | error      |

The highest applicable bump wins (e.g. a single `feat!:` outranks ten `fix:` commits).
The `!` must sit immediately before the colon in the subject's type/scope prefix — a
bare `!:` inside the description does not trigger a major bump. `BREAKING-CHANGE:` is
treated as a synonym of `BREAKING CHANGE:`, per Conventional Commits 1.0.0.

### Prefix handling

`tag_prefix` (default `"v"`) is stripped before SemVer comparison and re-applied on output.
Tags are sorted by SemVer order, not lexicographically — `v1.10.0` is newer than
`v1.9.0`, and bumping `v1.9.0` produces `v1.10.0` (never `v1.100.0`).

### Pre-release tags

Without `versionsort.suffix` configured in the user's git config, git's default
`version:refname` tag sort orders a pre-release tag *above* its corresponding release —
e.g. `v1.3.0-rc.1` sorts above `v1.2.3`. The resolver skips any tag whose bare form (after
stripping `tag_prefix`) is not a plain `MAJOR.MINOR.PATCH` — so `v1.3.0-rc.1` is skipped
and `v1.2.3` becomes the current tag for bump resolution. If every tag matching the prefix
is non-conforming, heraut behaves as if no tags exist and returns `initial_version`.

Pre-release tags are therefore invisible to `semver` auto-resolution: they neither become
the current tag nor block resolution. heraut does not produce pre-release tags itself;
this only matters for repositories where pre-release tags were created by other tooling.

### Initial version

When no tags matching the prefix exist, the resolver returns `initial_version` (default
`0.1.0`) — the first release does not bump.

### Manual mode

`bump: manual` requires `--version X.Y.Z` to be passed to `heraut release` (or
`heraut version next`). If omitted, the command fails with a config error before any
git operations.

---

## CalVer

```yaml
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"       # CalVer format string
  tag_prefix: ""
```

The version is derived from the current date plus a `PATCH` counter that resets when
the calendar period changes.

### CalVer format tokens

| Token    | Description                        | Example value |
|----------|------------------------------------|---------------|
| `YYYY`   | 4-digit calendar year              | `2026`        |
| `MM`     | 2-digit month (zero-padded)        | `05`          |
| `DD`     | 2-digit day of month (zero-padded) | `07`          |
| `WW`     | 2-digit ISO week number (01–53)    | `19`          |
| `QQ`     | Quarter (1–4)                      | `2`           |
| `SS`     | Semester (1–2)                     | `1`           |
| `SPRINT` | Manually-managed sprint counter    | `5`           |
| `PATCH`  | Auto-incrementing patch (required) | `0`, `3`      |

`PATCH` is mandatory and always the last component. It resets to `0` whenever the
period defined by the other tokens changes (e.g. new month for `YYYY.MM.PATCH`), and
increments by one for each release within the same period.

`SPRINT` is the only token not derived from the clock. Set it in
`versioning.sprint` and advance it manually with `heraut version sprint bump` at the
start of each sprint. When sprint changes, `PATCH` resets to `0` on the next release.

`heraut init` prompts for the initial `sprint` value when the selected CalVer format
contains `SPRINT`; it writes the entered value to `versioning.sprint` in the generated
`.heraut.yml`.

### Common format examples

| Format              | Example tags                          | Period    |
|---------------------|---------------------------------------|-----------|
| `YYYY.MM.PATCH`     | `2026.05.0`, `2026.05.1`              | Monthly   |
| `YYYY.MM.DD.PATCH`  | `2026.05.07.0`                        | Daily     |
| `YYYY.WW.PATCH`     | `2026.19.0`                           | Weekly    |
| `YYYY.QQ.PATCH`     | `2026.2.0`, `2026.2.1`                | Quarterly |
| `YYYY.SS.PATCH`     | `2026.1.0`, `2026.2.0`                | Bi-annual |
| `YYYY.SPRINT.PATCH` | `2026.5.0`, `2026.5.1`                | Sprint    |
| `YYYY.PATCH`        | `2026.0`, `2026.1`                    | Yearly    |

### Sprint example

```yaml
versioning:
  strategy: calver
  format: "YYYY.SPRINT.PATCH"
  sprint: 5   # bumped by the PO at the start of each sprint
```

`heraut version sprint bump` increments `sprint` from `5` to `6` and writes back. The
next release after the bump produces `2026.6.0`.

### Period-change resolution

The resolver compares the period of the current date against the period of the latest
tag's date. If they differ, `PATCH` resets to `0`. If they match, `PATCH` becomes the
latest tag's `PATCH` + 1.

The "period" depends on which tokens appear in `format`:

| Format contains | Period bucket                |
|-----------------|------------------------------|
| `YYYY.MM`       | Calendar month               |
| `YYYY.WW`       | ISO week                     |
| `YYYY.MM.DD`    | Calendar day                 |
| `YYYY.QQ`       | Calendar quarter             |
| `YYYY.SS`       | Calendar semester            |
| `YYYY.SPRINT`   | The `sprint` config value    |
| `YYYY` only     | Calendar year                |

---

## SemVer per environment

```yaml
versioning:
  strategy: semver-per-env

environments:
  dev:
    tag_format: "dev/{version}"   # tags: dev/1.0.1, dev/1.0.2
    branch: develop
    bump: auto
  prod:
    tag_format: "prod/{version}"  # tags: prod/1.0.0, prod/1.0.1
    branch: main
    bump: promote                 # promote = take version from latest dev tag
```

Each environment has its own tag namespace. Typically one environment uses `bump: auto`
to drive version computation; others use `bump: promote` to receive copies.

### Tag format

Each environment specifies its tag structure via `tag_format`, a string containing the
mandatory `{version}` token. The token can appear anywhere, enabling any ordering. The
`{env}` token is also available (substituted with the environment name).

| `tag_format`          | Example tags    | Pattern        |
|-----------------------|-----------------|----------------|
| `"dev/{version}"`     | `dev/1.0.2`     | ENV/SEMVER     |
| `"{version}/dev"`     | `1.0.2/dev`     | SEMVER/ENV     |
| `"{version}_dev"`     | `1.0.2_dev`     | SEMVER_ENV     |
| `"dev_{version}"`     | `dev_1.0.2`     | ENV_SEMVER     |
| `"release/{version}"` | `release/1.0.2` | custom prefix  |
| `"{env}/{version}"`   | `dev/1.0.2`, `prod/1.0.2` | shared format using `{env}` |

The `{version}` / `{env}` substitution is implemented in `internal/versioning/tagfmt/`,
shared between both per-env strategies ([ADR-0009](../adr/0009-generic-perenv-resolver.md)).

### Bump modes

**`bump: auto`** — resolves the latest source-env tag by SemVer (not lexicographically,
so `dev/1.10.0` beats `dev/1.9.0`), reads conventional commits since that tag, and
increments the patch/minor/major component accordingly. The same pre-release-tag skip
policy as plain SemVer applies (§ Pre-release tags): a tag like `dev/1.3.0-rc.1` is
skipped in favor of `dev/1.2.3` when selecting the current tag.

**`bump: promote`** — resolves the latest tag of the source environment, strips the
source format to extract the bare version, and renders it under the destination format.
Example: `dev/1.0.2` → `prod/1.0.2`.

The source environment is determined by the optional `source:` field (see
[ADR-0008](../adr/0008-promote-source-env.md)):

- **`source` omitted** — backward-compatible default: the single `bump: auto`
  environment is used. Validation error if zero or more than one `auto` environment
  exists.
- **`source: <env>`** — promotes from the named environment regardless of its `bump`
  mode. Enables disambiguation when multiple `auto` environments exist, and chaining
  (e.g. `prod` promotes from `preprod`, which itself promotes from `dev`).

### Promotion guards (E001/E002/E003)

Three hard-fail conditions are checked before any tag is created
([ADR-0007](../adr/0007-version-promotion-error-handling.md)):

| Code | Condition                                                                | Bypassed by `--force`?   |
|------|--------------------------------------------------------------------------|--------------------------|
| E001 | The candidate target tag already exists                                  | Yes                      |
| E002 | The destination environment is already at a version >= the candidate    | Yes                      |
| E003 | The source environment has no tags yet (nothing to promote)              | No — `--force` has no effect |

Each error message includes the source env, destination env, candidate version, and
the latest tag(s) involved.

### Version resolution logic

1. List all tags matching the active environment's glob (derived from `tag_format` via
   `tagfmt.GlobPattern`)
2. Sort by SemVer (not lexicographically)
3. Determine next version based on `bump` mode
4. Check E001/E002/E003
5. Render the candidate version under the destination's `tag_format`
6. Pass resolved tag to downstream generators / platforms

> **Note**: `git-cliff` and similar tools default to `v*` tag patterns. When using
> prefixed tags, the generator config must override `tag_pattern`. heraut injects the
> resolved tag into driver calls; it does not rely on drivers to resolve it.

---

## CalVer per environment

```yaml
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"       # CalVer format for the version component

environments:
  dev:
    tag_format: "dev/{version}"   # dev/2026.05.0
    bump: auto
  prod:
    tag_format: "prod/{version}"  # prod/2026.05.0
    bump: promote
```

Combines CalVer versioning (§ CalVer format tokens) with the multi-environment tag
model (§ SemVer per environment: tag format, bump modes, source field, E001/E002/E003).

**`bump: auto`** — finds the latest tag for the environment (by CalVer order, not
lexicographic), then computes the next version from the clock:

- If the calendar period has changed: `PATCH` resets to `0`.
- Within the same period: `PATCH` increments by one.

Unlike `semver-per-env`, **no conventional commit parsing** is performed — the CalVer
version advances purely from the date.

**`bump: promote`** — same source resolution, `source:` field semantics, and
E001/E002/E003 checks as `semver-per-env`, but the regression check (E002) uses CalVer
ordering instead of SemVer ordering.

---

## Generic per-env resolver internals

The two per-env strategies share a single implementation
([ADR-0009](../adr/0009-generic-perenv-resolver.md)):

```
internal/versioning/perenv/
   resolver.go    — VersionCalculator interface + New(runner, cfg, env, force, calc)
   auto.go        — auto mode (list tags → sort → bump from commits or date)
   promote.go     — promote mode (resolve source → strip → re-render → E001/E002/E003)
```

The `VersionCalculator` interface has two methods:

- `BumpAuto(tags []string, commits []string) (string, error)` — implemented by
  `internal/versioning/semver/`
- `BumpFromDate(tags []string) (string, error)` — implemented by
  `internal/versioning/calver/`

`app.NewResolver` selects which `VersionCalculator` to wire when constructing a
`perenv.Resolver`. SemVer and CalVer per-env modes share the same package; only the
calculator differs.
