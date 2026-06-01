# Spec 02 — Configuration (`.heraut.yml`)

This spec covers every field in `.heraut.yml`, the configuration file that controls how
`heraut` versions, generates changelogs, and publishes releases.

For design rationale see the ADRs at [`../adr/`](../adr/).

## IDE autocomplete

Add the schema comment at the top of your file to get inline validation and
autocompletion in VS Code, IntelliJ, and any editor with YAML Language Server support:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json
```

Run `heraut init` to generate a pre-filled file interactively, or
`heraut init --defaults` for a non-interactive opinionated default.

## File discovery

Heraut looks for the config file in this order
([ADR-0005](../adr/0005-config-file-discovery.md)):

1. `--config <path>` if passed explicitly
2. `HERAUT_FILE` environment variable if set
3. `.config/heraut.yml` if the file exists
4. `.heraut.yml` (default fallback)

The first match wins. `HERAUT_FILE` is useful in CI/CD environments where injecting an
env var is easier than passing a CLI flag. `heraut init` writes to `.config/heraut.yml`
if `.config/` already exists, otherwise to `.heraut.yml`.

## Top-level structure

```yaml
version: "1"          # required — schema version, currently only "1"

versioning: ...       # required — how the next version is determined
changelog: ...        # optional — how to generate and commit CHANGELOG.md
release: ...          # optional — where to publish releases
environments: ...     # optional — per-environment overrides for changelog/release
```

| Field          | Required | Description                                                                                                                |
|----------------|----------|----------------------------------------------------------------------------------------------------------------------------|
| `version`      | Yes      | Schema version. Only `"1"` is valid.                                                                                       |
| `versioning`   | Yes      | Version resolution strategy and options.                                                                                   |
| `changelog`    | No       | Changelog generator. When present, heraut generates and commits `CHANGELOG.md` during release.                             |
| `release`      | No       | Release notes generator and target platforms.                                                                              |
| `environments` | No       | Per-environment settings (versioning and content). Only valid with `semver-per-env` or `calver-per-env`. |

### Design principles

- **Root config is the default**; environment blocks are shallow-merged overrides.
- **`changelog` and `release.notes` are independent** — a project can have one, both, or
  neither.
- **Each generator/platform section is opaque to heraut** — the core tool passes the
  section as-is to the relevant driver, so adding generator/platform-specific fields
  does not require changes to the core.
- **Unknown keys fail validation** — the schema is strict; typos surface immediately.
  See [ADR-0006](../adr/0006-config-naming-generator-platform.md) for the naming
  convention (`generator: …`, `platform: …`).

## `versioning`

```yaml
versioning:
  strategy: semver    # required
```

| Field             | Required    | Default                                  | Description                                                                                                                                                                                                                |
|-------------------|-------------|------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `strategy`        | Yes         | —                                        | One of: `semver`, `calver`, `semver-per-env`, `calver-per-env`.                                                                                                                                                            |
| `tag_prefix`          | No          | `"v"` (semver), `""` (calver)            | Tag prefix prepended to the version string. Set to `""` to produce bare version tags.                                                                                                                                      |
| `initial_version` | No          | `"0.1.0"`                                | Version used when no tags exist yet. SemVer strategies only.                                                                                                                                                               |
| `bump`            | No          | `"auto"`                                 | `auto` — infer bump from conventional commits. `manual` — requires `--version` flag at runtime. SemVer strategies only.                                                                                                    |
| `format`          | CalVer      | —                                        | CalVer format string (see [Spec 04 — Versioning § CalVer format tokens](04-versioning.md#calver-format-tokens)). Required for `calver` and `calver-per-env`.                                                               |
| `sprint`          | Conditional | —                                        | Current sprint number. Required when `format` contains the `SPRINT` token. Advance with `heraut version sprint bump`.                                                                                                      |
| `tag_format`      | No          | —                                        | Common tag format for all environments (per-env strategies). `{env}` is replaced with the environment name; `{version}` with the resolved version. Per-environment `tag_format` overrides this.                            |
| `tag_type`        | No          | `annotated`                              | Git tag type: `annotated` (default) creates tags with `-a -m <commit_message>` so they carry a tagger, timestamp, and message. `lightweight` creates bare ref tags (`git tag <tag>`).                                     |

See [Spec 04 — Versioning](04-versioning.md) for strategy-specific behaviour.

### Strategy: `semver`

```yaml
versioning:
  strategy: semver
  tag_prefix: "v"             # produces tags like v1.2.3
  initial_version: "0.1.0"
  bump: auto
```

### Strategy: `calver`

```yaml
versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"   # e.g. 2026.05.0, 2026.05.1
  tag_prefix: ""
```

### Strategy: `semver-per-env`

Each environment has its own tag namespace and versioning policy. Typically one
environment drives version computation (`bump: auto`) and others receive promoted
copies (`bump: promote`).

```yaml
versioning:
  strategy: semver-per-env

environments:
  dev:
    tag_format: "dev/{version}"    # tags: dev/1.0.0, dev/1.0.1
    branch: develop
    bump: auto
  staging:
    tag_format: "staging/{version}"
    branch: main
    bump: promote
    source: dev                    # promotes from dev (explicit, see § Bump modes)
  prod:
    tag_format: "prod/{version}"
    branch: main
    bump: promote
    source: staging                # chains: prod ← staging ← dev
```

### Strategy: `calver-per-env`

Same multi-environment model as `semver-per-env` but with CalVer versioning. The
`bump: auto` environment increments from the clock; the `bump: promote` environment
copies the version from its source. No conventional commit parsing.

```yaml
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"

environments:
  dev:
    tag_format: "dev/{version}"    # dev/2026.05.0
    bump: auto
  prod:
    tag_format: "prod/{version}"   # prod/2026.05.0
    bump: promote
```

## Per-environment fields (`environments.<name>`)

Used inside `environments.<name>` for `semver-per-env` and `calver-per-env`. All fields
are optional unless noted.

### Versioning fields

| Field        | Required      | Default | Description                                                                                                                                           |
|--------------|---------------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------|
| `bump`       | Yes (per-env) | —       | `auto` or `promote` (see § Bump modes).                                                                                                               |
| `tag_format` | Conditional   | —       | Tag format for this environment. Must contain `{version}`. Overrides `versioning.tag_format` when set. Required if no common `tag_format` is defined. |
| `branch`     | No            | —       | Branch this environment is released from. Informational — used in error messages.                                                                     |
| `source`     | No            | —       | Source environment for `bump: promote`. See § Bump modes → promote.                                                                                   |

### Content fields

| Field               | Default | Description                                                                                                                                                               |
|---------------------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `disable_changelog` | `false` | When `true`, skips changelog generation and the git commit for this env. If `--tag` is also requested, the tag is still created. Takes precedence over `changelog:` when both are set. |
| `disable_notes`     | `false` | When `true`, skips release notes generation. The platform release is still created, but without attached notes. Takes precedence over `release.notes:` when both are set. |
| `changelog`         | —       | Override the root `changelog` block for this env (full replacement). Absent means use the root default.                                                                   |
| `release`           | —       | Override `release.platforms` and/or `release.notes` for this env. Fields inherit independently (see § Content override semantics below).                                  |

### Content override semantics

**`changelog`** — absent: use root `changelog`. Present: replaces root `changelog`
entirely for this env (full replacement, not merge). `disable_changelog: true` takes
precedence when both are set.

**`release`** — field-level inheritance within `release`:

| Sub-field           | Absent in env                 | Present in env                |
|---------------------|-------------------------------|-------------------------------|
| `release.platforms` | Use root `release.platforms`  | Replace entirely for this env |
| `release.notes`     | Use root `release.notes`      | Replace entirely for this env |

`disable_notes: true` takes precedence over `release.notes:` when both are set.

Setting contradictory flags (e.g. `disable_changelog: true` and `changelog:` on the same
env) produces a non-zero exit from `heraut check config` with an actionable hint
explaining which field to remove.

### Bump modes

#### `auto`

Resolves the latest tag for this environment (by SemVer or CalVer order, not
lexicographically), then computes the next version:

- **SemVer**: reads conventional commits since the latest tag and applies the
  appropriate bump level (patch / minor / major).
- **CalVer**: advances from the current date. `PATCH` resets to `0` when the calendar
  period changes; otherwise it increments by one. Commits are not read.

#### `promote`

Resolves the latest tag of the source environment, extracts the bare version, and
re-renders it under this environment's `tag_format`.

Example: source `dev/1.2.3` → destination `prod/1.2.3`.

**Source resolution** — the `source:` field controls which environment is promoted from
(see [ADR-0008](../adr/0008-promote-source-env.md)):

| `source` value                  | Behavior                                                                |
|---------------------------------|-------------------------------------------------------------------------|
| Omitted, exactly one `auto` env | Uses that env automatically (backward-compatible)                       |
| Omitted, multiple `auto` envs   | **Config error** — add `source:` to disambiguate                        |
| Omitted, zero `auto` envs       | **Config error** — add an `auto` env or set `source:` explicitly        |
| Set to an env name              | Promotes from that env regardless of its `bump` mode (enables chaining) |

Setting `source` to a `promote` env enables **chaining**: `prod` promotes from `staging`,
which itself promotes from `dev`. Each hop is an independent promotion decision guarded
by the same E001/E002/E003 checks.

**Promotion guards** ([ADR-0007](../adr/0007-version-promotion-error-handling.md)) —
three hard-fail conditions are checked before any tag is created:

| Code | Condition                                                      | Bypassed by `--force`?    |
|------|----------------------------------------------------------------|---------------------------|
| E001 | The target tag already exists                                  | Yes                       |
| E002 | The destination is already ahead of the candidate (regression) | Yes                       |
| E003 | The source environment has no tags yet                         | No — nothing to promote   |

### Common `tag_format`

Instead of repeating `tag_format` on every environment, set it once at the `versioning`
level using the `{env}` token:

```yaml
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"    # expands to dev/1.0.0, prod/1.0.0, etc.

environments:
  dev:
    bump: auto
  prod:
    bump: promote
```

A per-environment `tag_format` always overrides the common one.

### `{build}` token — CI build IDs

`tag_format` supports a third token, `{build}`, for pipelines that append a CI build
number to the tag (common in mobile projects):

```yaml
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}-{build}"  # e.g. uat/7.4.1-158404
```

`{build}` is populated by the `--build <id>` flag on `heraut changelog`:

```bash
heraut changelog --tag --env uat --version 7.4.1 --build $CI_PIPELINE_ID
```

**Constraints:**

- `--build` requires `--version` — build IDs come from CI, not from commit analysis.
- If `{build}` appears in `tag_format` but `--build` is not passed, heraut exits with
  an error.
- Build IDs must not contain `/` or whitespace (git tag constraint). *(Enforcement is
  planned — see roadmap T55; today an invalid build ID surfaces as a `git tag` error.)*
- Internally, the changelog range comparison treats `{build}` as a non-capturing wildcard,
  so existing tags like `uat/7.4.0-155391` correctly yield version `7.4.0` when computing
  the commit range.

**Scope — changelog-only:** the `{build}` flow is supported by `heraut changelog --build`
only. With a `tag_format` that contains `{build}`, the following commands cannot render a
tag (no build ID is available) and will error until the noted work lands:

| Command | Status |
|---|---|
| `heraut changelog --tag --version … --build …` | ✅ supported |
| `heraut release` | ❌ no `--build` flag (planned — roadmap T57) |
| `heraut version next` | ❌ cannot render a build tag |
| `heraut version current --env <env>` | ⚠️ returns the raw tag (bare-version output planned — roadmap T58) |

**Changelog note:** git-cliff generates one section per tag boundary. Multiple builds
of the same semantic version produce multiple sections with the same heading. For a clean
per-version changelog, set `disable_changelog: true` on UAT environments and only
generate the changelog on the production/main release. Use `tag_pattern` scoped to the
production env; heraut automatically injects a postprocessor that strips the env prefix
and build ID from version headings (`[uat/7.4.1-158404]` → `[7.4.1]`).

## `changelog`

Controls how `CHANGELOG.md` is generated and committed.

```yaml
changelog:
  generator: git-cliff
  config: cliff.toml      # optional
  output: CHANGELOG.md    # optional, defaults to CHANGELOG.md
  tag_pattern: "dev/*"    # optional, for prefixed-tag strategies
```

When `changelog` is present, heraut generates the changelog and commits it to the
release branch as part of `heraut release`. Omit this block entirely to skip changelog
generation. The commit ownership is described in
[ADR-0012](../adr/0012-changelog-commit-ownership.md).

See § Content generators below for generator-specific fields.

## `release`

Controls release notes generation and where releases are published.

```yaml
release:
  notes:                  # optional — release notes generator
    generator: git-cliff
  platforms:              # optional — target platforms
    - platform: gitlab
    - platform: github
```

Both `notes` and `platforms` are optional independently:

- **`platforms` only (no `notes`)** — the release is published on each platform with no
  inline content. This is intentional and valid: the CHANGELOG.md in the repository (or
  the platform's own auto-generate feature) serves as the record. Use a comment to make
  the intent explicit:

  ```yaml
  release:
    # No inline release notes — CHANGELOG.md in the repo is the record.
    platforms:
      - platform: github
        repository: org/repo
  ```

- **`notes` only (no `platforms`)** — notes are generated but no platform release is
  created. Useful for previewing or piping output to another tool.

- **Neither** — `release:` may be omitted entirely when `heraut release` is not used.
  Note: `heraut release` requires at least one entry in `platforms`; omitting the whole
  `release` block (or leaving `platforms` empty) is a configuration error for that command.

## Content generators

Used under `changelog` and `release.notes`. A project can use different generators for
each.

| Field         | Required | Description                                                                                                                                                                                                                                                                       |
|---------------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `generator`   | Yes      | One of: `git-cliff`, `communique`, `cocogitto`.                                                                                                                                                                                                                                   |
| `config`      | No       | Path to the generator config file (relative to project root). For `git-cliff`: optional partial override, deep-merged with heraut's built-in default. For `communique`: required. For `cocogitto`: optional path to `cog.toml`.                                                  |
| `output`      | No       | Output file path (e.g. `CHANGELOG.md`). For `cocogitto`, heraut captures stdout and writes this file (cog itself has no `--output` flag).                                                                                                                                          |
| `tag_pattern` | No       | Tag pattern regex for `git-cliff`. Required when using prefixed-tag strategies (e.g. `"dev/.*"`). Not used by `communique` or `cocogitto`.                                                                                                                                         |
| `template`    | No       | Path to a custom Tera template for `cocogitto` (passed as `-t`). Not used by `git-cliff` or `communique`.                                                                                                                                                                          |

See [Spec 05 — Generators and Platforms](05-generators-and-platforms.md) for the full
behaviour of each generator.

## Platform drivers

Used inside `release.platforms` (and inside per-environment `release.platforms`).

### GitLab

```yaml
release:
  platforms:
    - platform: gitlab
      project: $CI_PROJECT_PATH   # optional, defaults to $CI_PROJECT_PATH
      token_env: GITLAB_TOKEN     # optional, defaults to GITLAB_TOKEN
      catalog: false              # optional, set true for a CI/CD Catalog release
      assets:
        - dist/myapp_*            # glob patterns for files to attach
```

| Field       | Required | Default            | Description                                                                                |
|-------------|----------|--------------------|--------------------------------------------------------------------------------------------|
| `platform`  | Yes      | —                  | Must be `"gitlab"`.                                                                        |
| `project`   | No       | `$CI_PROJECT_PATH` | GitLab project path in `namespace/repo` format.                                            |
| `token_env` | No       | `GITLAB_TOKEN`     | Name of the env var holding the GitLab API token.                                          |
| `catalog`   | No       | `false`            | When true, passes `--publish-to-catalog` to `glab release create`.                         |
| `assets`    | No       | `[]`               | Glob patterns for files to upload as release assets.                                       |

Implementation: shells out to `glab release create` + `glab release upload-asset`.

### GitHub

```yaml
release:
  platforms:
    - platform: github
      repository: org/repo        # optional, defaults to $GITHUB_REPOSITORY
      token_env: GH_TOKEN         # optional, defaults to GH_TOKEN
      draft: false
      prerelease: false
      assets:
        - dist/myapp_*
```

| Field        | Required | Default              | Description                                                                                |
|--------------|----------|----------------------|--------------------------------------------------------------------------------------------|
| `platform`   | Yes      | —                    | Must be `"github"`.                                                                        |
| `repository` | No       | `$GITHUB_REPOSITORY` | GitHub repo in `owner/repo` format.                                                        |
| `token_env`  | No       | `GH_TOKEN`           | Name of the env var holding the GitHub token.                                              |
| `draft`      | No       | `false`              | Create the release as a draft.                                                             |
| `prerelease` | No       | `false`              | Mark the release as a pre-release.                                                         |
| `assets`     | No       | `[]`                 | Glob patterns for files to upload as release assets.                                       |

Implementation: shells out to `gh release create` + `gh release upload`.

## `environments`

The root `environments` map is the single place for all per-environment configuration —
versioning policy and content overrides together. It is only valid with `semver-per-env`
or `calver-per-env`; using it with a flat strategy (`semver`, `calver`) is a hard
validation error with an actionable hint.

See § Per-environment fields (`environments.<name>`) for the full field reference.

## Complete examples

### Standard SemVer — GitHub

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/adaouat/heraut/main/schema.json

version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"
  initial_version: "0.1.0"
  bump: auto

changelog:
  generator: git-cliff
  output: CHANGELOG.md

release:
  notes:
    generator: git-cliff
  platforms:
    - platform: github
      repository: acme/widget
      token_env: GH_TOKEN
```

### CalVer — monthly releases, GitLab

```yaml
version: "1"

versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
  tag_prefix: ""

changelog:
  generator: git-cliff
  output: CHANGELOG.md

release:
  notes:
    generator: git-cliff
  platforms:
    - platform: gitlab
```

### SemVer per environment — dev → staging → prod chain

```yaml
version: "1"

versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"    # common format: dev/1.0.0, staging/1.0.0, prod/1.0.0

changelog:
  generator: git-cliff
  output: CHANGELOG.md
  tag_pattern: "dev/*"

release:
  notes:
    generator: git-cliff
    tag_pattern: "dev/*"
  platforms:
    - platform: gitlab

environments:
  dev:
    branch: develop
    bump: auto
    disable_changelog: true        # no CHANGELOG commit on dev
    release:
      platforms:
        - platform: gitlab         # dev releases only go to GitLab
  staging:
    branch: main
    bump: promote
    source: dev                    # staging ← dev
  prod:
    branch: main
    bump: promote
    source: staging                # prod ← staging ← dev
    release:
      platforms:
        - platform: gitlab
        - platform: github         # prod releases go to both
```

### SemVer — multiple platforms, with binaries

```yaml
version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"

release:
  notes:
    generator: git-cliff
  platforms:
    - platform: gitlab
      token_env: GITLAB_TOKEN
      assets:
        - dist/myapp_linux_amd64.tar.gz
        - dist/myapp_darwin_arm64.tar.gz
        - dist/checksums.txt
    - platform: github
      repository: org/myapp
      token_env: GH_TOKEN
      assets:
        - dist/myapp_*
```
