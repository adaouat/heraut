# Choose a versioning strategy

`versioning.strategy` decides what a version looks like and where it comes from. There are four,
from two questions: are versions about **changes** or about **dates**, and do you ship **one line**
or **one per environment**?

| | One version line | One line per environment (dev, staging, prod) |
|---|---|---|
| **From your commit messages** (`1.4.0`) | `semver` | `semver-per-env` |
| **From the date** (`2026.11.0`) | `calver` | `calver-per-env` |

## `semver`: versions that say what changed

For libraries, CLIs, APIs: anything whose users care whether an upgrade can break them.

```yaml
versioning:
  strategy: semver
  tag_prefix: "v"        # tags v1.4.0; "" for 1.4.0
```

The next version comes from the commits since the last release: `fix:` gives `1.4.1`, `feat:` gives
`1.5.0`, a breaking change gives `2.0.0`. The first release is `0.1.0`. See
[How the next version is found](next-version.md).

`semver` is the only strategy with [pre-releases](../guides/pre-releases.md) (`v1.4.0-rc.1`) and
[maintenance branches](../guides/maintenance-branches.md).

## `calver`: versions that say when

For applications and services released on a schedule, where the date matters more than the size of
the change.

```yaml
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"   # 2026.11.0, 2026.11.1, then 2026.12.0
```

`PATCH` counts releases within the period and starts again at `0` when it changes. Commit messages
don't affect the version, only the changelog.

| Format | Example | A new period every |
|---|---|---|
| `YYYY.MM.PATCH` | `2026.11.0` | month |
| `YYYY.MM.DD.PATCH` | `2026.11.06.0` | day |
| `YYYY.WW.PATCH` | `2026.45.0` | ISO week |
| `YYYY.QQ.PATCH` | `2026.4.0` | quarter |
| `YYYY.SS.PATCH` | `2026.2.0` | half-year |
| `YYYY.SPRINT.PATCH` | `2026.5.0` | sprint, advanced by `heraut version sprint bump` |
| `YYYY.PATCH` | `2026.0` | year |

`PATCH` is required and comes last; other text between tokens is kept as written. Dates are read in
UTC, so a laptop and a CI runner agree.

## `semver-per-env` and `calver-per-env`: one line per environment

When the same build goes through several environments and each needs its own tags. One environment
computes new versions (`bump: auto`), the others copy them (`bump: promote`):

```yaml
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"   # staging/1.4.0, prod/1.4.0

environments:
  staging:
    bump: auto
  prod:
    bump: promote                 # prod gets staging's latest version
```

Every command then takes `--env`. `calver-per-env` is the same with a `format:` and dates. The whole
setup, promotion guards included, is in [Several environments](../guides/environments.md).

## Tag formats

`tag_prefix` is enough for one line: `"v"` gives `v1.4.0`. The per-environment strategies use
`tag_format` instead, with these placeholders:

| Placeholder | Becomes |
|---|---|
| `{version}` | the version, required: `1.4.0` |
| `{env}` | the environment's name: `staging` |
| `{build}` | a CI build number passed with `--set-build-id`, after a `+`: `uat/7.4.1+158404` |

The order is free: `{env}/{version}`, `{version}-{env}`, `release/{version}`. Each environment can also
set its own `tag_format`. For build numbers, see
[Mobile apps: build numbers in tags](../guides/mobile-ci-tagging.md).

## Changing strategy later

heraut finds the last release by parsing your tags with the current strategy: tags that don't fit
are ignored. After a switch (from `calver` to `semver`, say), heraut starts from the first version of
the new scheme, or from the version you give it once with `--set-version`.
