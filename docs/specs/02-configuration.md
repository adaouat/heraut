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
- **Platform sections are opaque to heraut** — the core tool passes `forges:` entries and
  `release.targets[]` as-is to the relevant driver, so adding driver-specific fields
  does not require changes to the core.
- **Unknown keys fail validation** — the schema is strict; typos surface immediately.
  See [ADR-0006](../adr/0006-config-naming-generator-platform.md) for the platform naming
  convention (`platform: …`).

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
| `branch`     | No            | —       | Branch this environment is operated from. **Enforced**: heraut refuses any `--env <env>` command (release, changelog, version next/current) unless the current git branch matches. Bypass with `--force`. **Omit (or leave empty) to impose no branch restriction** — the default. |
| `source`     | No            | —       | Source environment for `bump: promote`. See § Bump modes → promote.                                                                                   |

### Content fields

| Field               | Default | Description                                                                                                                                                               |
|---------------------|---------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `disable_changelog` | `false` | When `true`, skips changelog generation and the git commit for this env. If `--tag` is also requested, the tag is still created. Takes precedence over `changelog:` when both are set. |
| `disable_notes`     | `false` | When `true`, skips release notes generation. The platform release is still created, but without attached notes. Takes precedence over `release.notes:` when both are set. |
| `changelog`         | —       | Override the root `changelog` for this env. Deep-merges field-by-field (see § Content override semantics). Absent means use the root default.                              |
| `release`           | —       | Override `release.notes` (deep-merge) and/or `release.targets` (replace) for this env (see § Content override semantics below).                                            |

### Content override semantics

Per-environment `changelog` and `release.notes` blocks **deep-merge** field-by-field over
the root driver ([ADR-0019](../adr/0019-perenv-content-driver-merge.md)): a field you set
per-env wins; a field you omit inherits from the root. So you can override just one field:

```yaml
changelog:
  tag_pattern: "^v[0-9]+"  # include only v* tags
  output: CHANGELOG.md

environments:
  prod:
    changelog:
      tag_pattern: "^v[0-9]+_prod$"  # prod-only tags
      # output inherits from root
```

**Limitation:** because an empty field inherits, a per-env block cannot blank out a value
the root sets (there is no explicit "unset").

`disable_changelog` / `disable_notes: true` take precedence over `changelog:` / `release.notes:`
when both are set on the same env.

**Lists stay replace** — `release.targets` is replaced wholesale per env (merging lists
is ambiguous); absent means use the root list:

| Sub-field         | Absent in env               | Present in env                |
|-------------------|------------------------------|--------------------------------|
| `release.targets` | Use root `release.targets`  | Replace entirely for this env |
| `release.notes`   | Use root `release.notes`    | **Deep-merge** over root       |

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

**Changelog headings are cleaned automatically.** When `tag_format` carries an `{env}`
(prefix or suffix) or `{build}` token, native strips those tokens from the version heading,
leaving just the version: `prod/1.0.0` → `1.0.0`, `2026.3.0_prod` → `2026.3.0`,
`uat/7.4.1-158404` → `7.4.1` (SemVer pre-release preserved: `7.4.1-rc.1`). Compare links
still use the full tags.

### `{build}` token — CI build IDs

`tag_format` supports a third token, `{build}`, for pipelines that append a CI build
number to the tag (common in mobile projects):

```yaml
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}-{build}"  # e.g. uat/7.4.1-158404
```

`{build}` is populated by the `--build <id>` flag on `heraut changelog` and `heraut release`:

```bash
heraut changelog --tag --env uat --version 7.4.1 --build $CI_PIPELINE_ID
heraut release         --env uat --version 7.4.1 --build $CI_PIPELINE_ID
```

**Constraints:**

- `--build` requires `--version` — build IDs come from CI, not from commit analysis.
- If `{build}` appears in `tag_format` but `--build` is not passed, heraut exits with
  an error.
- Build IDs must not contain `/` or whitespace (git tag constraint). `--build` rejects
  an invalid value up front with an actionable error.
- Internally, the changelog range comparison treats `{build}` as a non-capturing wildcard,
  so existing tags like `uat/7.4.0-155391` correctly yield version `7.4.0` when computing
  the commit range.

**Scope:** the `{build}` flow is supported by `heraut changelog --build` and
`heraut release --build`. With a `tag_format` that contains `{build}`, commands that infer
the tag from git history cannot render one (no build ID is available) and will error:

| Command | Status |
|---|---|
| `heraut changelog --tag --version … --build …` | ✅ supported |
| `heraut release --version … --build …` | ✅ supported |
| `heraut version next` | ❌ cannot render a build tag |
| `heraut version current --env <env>` | ✅ raw tag; add `--bare` for the stripped version (`7.4.1`) |

`heraut release --build` publishes **one platform release per build** — intentional for
build-per-release teams. Passing both `--version` and `--build` is the explicit opt-in;
heraut does not guard or warn (mirrors `changelog --build`).

**Changelog note:** Native generates one section per tag boundary. Multiple builds
of the same semantic version produce multiple sections with the same heading. For a clean
per-version changelog, set `disable_changelog: true` on UAT environments and only
generate the changelog on the production/main release. Use `tag_pattern` scoped to the
production env; heraut automatically strips the env prefix and build ID from version
headings (`[uat/7.4.1-158404]` → `[7.4.1]`).

## `changelog`

Controls how `CHANGELOG.md` is generated and committed.

```yaml
changelog:
  output: CHANGELOG.md    # optional, defaults to CHANGELOG.md
  tag_pattern: "dev/*"    # optional, for prefixed-tag strategies
  template: path/to/my-template.tmpl  # optional, custom Go template
```

When `changelog` is present, heraut generates the changelog and commits it to the
release branch as part of `heraut release`. Omit this block entirely to skip changelog
generation. The commit ownership is described in
[ADR-0012](../adr/0012-changelog-commit-ownership.md).

See § Content generation below for native generator fields.

## `release`

Controls release notes generation and where releases are published.

```yaml
release:
  notes: {}               # optional — release notes config (empty uses defaults)
  targets:                 # optional — publish destinations
    - forge: gitlab-saas
    - forge: github
```

Both `notes` and `targets` are optional independently:

- **`targets` only (no `notes`)** — the release is published to each target with no
  inline content. This is intentional and valid: the CHANGELOG.md in the repository (or
  the forge's own auto-generate feature) serves as the record. Use a comment to make
  the intent explicit:

  ```yaml
  release:
    # No inline release notes — CHANGELOG.md in the repo is the record.
    targets:
      - forge: github
  ```

- **`notes` only (no `targets`)** — notes are generated but no release is published.
  Useful for previewing or piping output to another tool.

- **Neither, with `targets` omitted entirely** — the release still publishes to the single
  resolved forge with default options (zero-config publishing; see § Identity resolution
  under § `forges` below). This is the common CI shape: no `forges:`, no `release.targets`,
  and heraut auto-detects the destination from the CI environment or git origin. On GitHub
  Actions this still requires `GH_TOKEN` to be exported: the identity's token is resolved for
  enrichment but is not passed to the `gh`-based driver, and `gh`'s own check has no CI
  exemption (unlike GitLab CI, where `glab`'s job-token autologin covers this case — see
  `inCIAutologin` in the GitLab driver).

- **`release:` omitted entirely** — valid when `heraut release` is not used. Note: `heraut
  release` itself requires at least one **resolvable** publish destination — an explicit
  `release.targets` entry, or a forge that auto-detects. Zero resolvable destinations (no
  targets, no forge, no CI/origin to detect one) is a configuration error for that command.

### `release.targets[]`

`release.targets` is the publishing surface ([ADR-0044](../adr/0044-publishing-config-unification.md)).
Each entry references a `forges[].name` (see § `forges` below) and carries only publish
behaviour:

```yaml
release:
  targets:
    - forge: gitlab-saas   # → forges[].name; optional when exactly one forge is configured
      draft: false          # GitHub only
      prerelease: false     # GitHub only
      assets:                # optional — overrides release.assets entirely for this target
        - "dist/myapp_*"
```

| Field        | Required                              | Default | Description                                                                                    |
|--------------|----------------------------------------|---------|--------------------------------------------------------------------------------------------------|
| `forge`      | Conditional                            | —       | References a `forges[].name`. Optional when exactly one forge is configured/resolved; required when more than one is configured. |
| `draft`      | No                                      | `false` | Create the release as a draft. GitHub only.                                                     |
| `prerelease` | No                                      | `false` | Mark as a pre-release. GitHub only.                                                              |
| `assets`     | No                                      | —       | Target-specific glob patterns. When set, overrides `release.assets` entirely for this target (no merging). |

Publishing constructs the existing GitHub/GitLab drivers from the resolved
`forges[].name` identity — host, project/repository, and token are inherited from the same
CI/git-origin auto-detection that drives enrichment (§ Identity resolution below), so a
target typically needs no more than `forge:` (or nothing at all, in the single-forge case).

Multi-instance publishing (same platform type, multiple hosts — [ADR-0025](../adr/0025-multi-instance-platforms.md))
is expressed as multiple `forges:` entries, each referenced by its own `release.targets`
entry:

```yaml
forges:
  - name: gitlab-saas
    platform: gitlab
    project: acme/widget
  - name: gitlab-internal
    platform: gitlab
    project: tools/widget-mirror
    base_url: https://gitlab.example.com
    token_env: GITLAB_INTERNAL_TOKEN

release:
  targets:
    - forge: gitlab-saas
    - forge: gitlab-internal
```

## `forges`

A top-level **`forges:`** list — each entry is one code-hosting platform heraut talks to:
**connection and identity only** ([ADR-0043](../adr/0043-forge-abstraction.md),
[ADR-0044](../adr/0044-publishing-config-unification.md)). A forge does not itself say what
to publish or draft — that lives on `release.targets[]` (see above) — and it is not the
enrichment source, either — that is `commits.enrichment_forge` (see below). `forges:` is
entirely optional in the common case, since heraut auto-detects a single forge from the CI
environment or the git remote when the block is omitted.

```yaml
forges:
  - name: gitlab-saas          # referenced by release.targets[].forge and commits.enrichment_forge
    platform: gitlab           # github | gitlab | azure_devops
    project: group/subgroup/project
    api_mode: rest              # rest (default) | graphql
```

### Fields

| Field         | Required | Default              | Description                                                                                          |
|---------------|----------|-----------------------|-------------------------------------------------------------------------------------------------------|
| `name`        | Yes      | —                     | Unique identifier for this forge entry within `forges:`, referenced by `release.targets[].forge` and `commits.enrichment_forge`. |
| `platform`    | Yes      | —                     | One of: `github`, `gitlab`, `azure_devops`.                                                            |
| `project`     | No       | Inferred (see below)  | GitLab project path (`namespace[/subgroup]/repo`). For `azure_devops`, `organization/project`.        |
| `repository`  | No       | Inferred (see below)  | GitHub repository (`owner/repo`). For `azure_devops`, the repository name.                            |
| `base_url`    | No       | Per-type default (see below) | Web base URL of the forge instance, e.g. `https://gitlab.example.com` for a self-hosted instance. |
| `api_url`     | No       | Derived from `base_url` | API host override, when it differs from `base_url`.                                                 |
| `api_mode`    | No       | `rest`                | `rest` or `graphql` — see § `api_mode` trade-off below.                                               |
| `token_env`   | No       | Per-type default (see below) | Env var holding the API token.                                                                 |

Only `name` and `platform` are required. `project`/`repository`, `base_url`, `api_url`, and
`token_env` are all filled in when omitted — see § Identity resolution.

### Identity resolution

Each field of a `forges:` entry (or, with no `forges:` block at all, the single
auto-detected forge) is resolved in this precedence order, stopping at the first source
that supplies a value:

1. **Explicit config** — the value set directly on the `forges:` entry.
2. **CI environment** — when the entry's `platform` matches the CI system detected from
   ambient environment variables:

   | CI system    | Marker           | Host             | API URL            | Project/repo         | Token            |
   |--------------|------------------|-------------------|---------------------|-----------------------|------------------|
   | GitLab CI    | `GITLAB_CI`      | `CI_SERVER_URL`   | `CI_API_V4_URL`     | `CI_PROJECT_PATH`     | `CI_JOB_TOKEN`   |
   | GitHub Actions | `GITHUB_ACTIONS` | `GITHUB_SERVER_URL` | `GITHUB_API_URL`  | `GITHUB_REPOSITORY`   | `GITHUB_TOKEN`   |
   | Azure Pipelines | `TF_BUILD`    | `SYSTEM_COLLECTIONURI` | —              | `SYSTEM_TEAMPROJECT`  | `SYSTEM_ACCESSTOKEN` |

   Detection checks GitLab, then GitHub, then Azure, and stops at the first marker present.
3. **`git remote get-url origin`** — when the origin host is a known public host
   (`github.com`, `gitlab.com`, `dev.azure.com`), its type, host, and `owner/repo` path are
   used (both the SSH `git@host:path` and HTTPS `https://host/path` forms are parsed).
4. **Offline fallback** — a per-type default host (`https://github.com`,
   `https://gitlab.com`, `https://dev.azure.com`) and default token env var
   (`GITHUB_TOKEN`, `GITLAB_TOKEN`, `AZURE_DEVOPS_TOKEN`).

With **no `forges:` block**, heraut auto-detects at most one forge the same way: CI markers
first, then git origin, then — if neither applies — it looks for exactly one of the three
default token env vars set in the environment. Auto-detection **never guesses**: if more
than one candidate is found (e.g. both `GITLAB_TOKEN` and `GITHUB_TOKEN` are set with no CI
markers and no recognized git origin) heraut fails with:

```
detected candidates [gitlab github] and no CI/origin to disambiguate: ambiguous forge
```

Declare an explicit `forges:` block to resolve the ambiguity. Publishing to multiple forges,
or choosing an enrichment source among several, always requires an explicit `forges:` block —
zero-config resolution is single-forge by definition.

### `commits.enrichment_forge` / `commits.enrichment_policy`

`enrichment_forge` names which `forges:` entry supplies PR/MR metadata; `enrichment_policy`
governs whether that fetch happens at all. Both are fields of the `commits:` block — see
§ `commits.enrichment_forge` / `commits.enrichment_policy` under § `commits` below for the
full reference.

### `api_mode` trade-off (GitLab)

`api_mode` chooses the transport heraut's GitLab forge uses for PR/MR enrichment:

- **`rest` (default)** — works out of the box with the CI job token
  (`CI_JOB_TOKEN`): merge-request association comes from
  `GET /projects/:id/repository/commits/:sha/merge_requests`, an endpoint GitLab allows a job
  token to call. REST carries no linked handle, so commit authors render as `by @<local-part>`
  using the git author email's local-part, falling back to the git author name when no email is
  present (matching the Azure DevOps forge's fallback).
- **`graphql` (opt-in)** — requires a `read_api` personal or project access token and renders
  the linked `@username` instead of the plain author name. GitLab's GraphQL API structurally
  rejects job tokens ("You cannot use job tokens to authenticate GraphQL requests"), so
  resolving a job token (e.g. the CI-provided `CI_JOB_TOKEN`) as the token for
  `api_mode: graphql` fails at enrichment time — set `api_mode: rest`, or supply a `read_api`
  token via `token_env`.

### Zero-config example (GitLab CI)

No `forges:` block at all — the common case in GitLab CI, where `CI_JOB_TOKEN` is always
present:

```yaml
version: "1"

versioning:
  strategy: semver

changelog: {}
```

heraut auto-detects the GitLab forge from `GITLAB_CI` / `CI_SERVER_URL` /
`CI_PROJECT_PATH` / `CI_JOB_TOKEN`, with `api_mode: rest` (the auto-detected default),
requiring no host, project, or token configuration.

### Minimal explicit example

```yaml
forges:
  - name: gitlab-saas
    platform: gitlab
    project: group/subgroup/project
```

## `commits`

A top-level block that is the **single source of truth for commit semantics and enrichment**
(ADR-0033): the conventional-commit **type set** — used by both `heraut commit verify` /
`create` and the native renderer's changelog section taxonomy — plus scope rules, ticket
links, and the enrichment forge/policy. Every field is optional; with no `commits` block the
built-in defaults apply.

```yaml
commits:
  types:                              # merged OVER the built-in defaults, by name
    - name: feat
      order: 1
      render: "🚀 Features"
    - name: docs                      # no render → capitalized type name ("Docs")
    - name: build
      remove: true                    # drop a default from the verify allow-list
  types_heading_level: 3              # heading depth for type sections in rendered output
  scopes:                             # objects: name (+ optional description / remove)
    - name: cmd
      description: CLI commands
    - name: config
  scopes_restricted: false            # when true, verify rejects scopes outside the list
  enrichment_forge: gitlab-saas        # references a top-level forges[].name
  enrichment_policy: optional          # required | optional (default) | disabled
  tickets:
    - pattern: '[A-Z]+-[0-9]+'
      url: 'https://acme.atlassian.net/browse/{ticket}'
```

### `commits.types`

A list of type rules **deep-merged over the built-in defaults by name**: a listed type
replaces that default's entry wholesale (an omitted `render`/`order` means "no label /
unordered", not "inherit the default"); `remove: true` drops a default; an unknown name is
appended. The effective set is **both** the `heraut commit verify` allow-list **and** the
changelog section taxonomy.

| Field    | Meaning |
|----------|---------|
| `name`   | The conventional-commit type word (required). |
| `order`  | Display sort position for the changelog section; omit for unordered (sorts after ordered types). |
| `render` | Section heading label (e.g. `🚀 Features`); omit to render the capitalized type name. |
| `remove` | Drop this default type from the effective set — removes it from the verify allow-list. |
| `description` | One-line hint shown beside the type in the `heraut commit create` wizard picker. |

Because `types` **merges** (it does not replace), listing types does **not** narrow the
verify allow-list to only those — to narrow it, `remove:` the unwanted defaults. The built-in
defaults are the 10 types in [`workflow.md`](../../.claude/rules/workflow.md)'s commit-type
table (`feat, fix, docs, chore, refactor, test, style, perf, ci, build`), render-labeled.
Merge and `fixup!`/`squash!` commits are always skipped by verify, unconditionally.

### `commits.scopes` / `commits.scopes_restricted`

Each scope is an object — `name` (required), plus an optional `description` (shown beside the
scope in the wizard picker) and `remove` (drops a built-in default). Scopes are **merged over a
small built-in default set** — `deps`, `deps-dev`, `release` (the dependabot/renovate and
release-tooling conventions, which also align with the default `rendering.excludes`). `scopes`
populates `heraut commit create`'s scope picker, and when `scopes_restricted: true`, `heraut
commit verify` also **rejects scopes outside the effective list**. With `scopes_restricted`
unset (the default), scopes are not enforced by verify — so the defaults are suggestions, not
a gate.

### `commits.tickets`

Links issue-tracker references found in commit messages — **subject, body, or footer** (e.g.
`Refs: PROJ-123`) — in the changelog and release notes.

| Field | Meaning |
|---|---|
| `pattern` | A regex matching ticket IDs. Each match is rendered as a link; the **label** is always the full match. |
| `url` | A URL template containing `{ticket}` — the pattern's **first capture group**, or the **full match** when there is no group. |

Heraut injects each entry as a link parser; the link is appended to the commit line as
`([TICKET](url))`.

### `commits.enrichment_forge` / `commits.enrichment_policy`

`enrichment_forge` names which entry of the top-level `forges:` list (connection/identity for a
code-hosting platform heraut talks to — see `docs/heraut.sample.yml` and
[ADR-0043](../adr/0043-forge-abstraction.md)) supplies PR/MR metadata (author handle, PR number)
for changelog and release-notes generation. With a single `forges` entry (or none, relying on
auto-detection from CI environment or `git remote get-url origin`), `enrichment_forge` can be
omitted; it becomes required when multiple forges are configured and heraut cannot pick one
unambiguously.

`enrichment_policy` governs **whether** that fetch happens at all: `required` (fetch, and **fail**
when the metadata cannot be fetched — the forge is unavailable/unreachable, *or* no forge is
configured or auto-detected to fetch from), `optional` (default — fetch when possible, else warn),
`disabled` (never fetch). `--force` downgrades `required` to `optional` for a single run (degrade
with a warning instead of failing). The global `--offline` flag forces `disabled` for a single run
(ADR-0023; renamed from `commits.remote_metadata` by ADR-0043 — semantics unchanged).

## `rendering`

A top-level block configuring content output (ADR-0033).

```yaml
rendering:
  excludes:
    - regex: '^chore\(deps.*\)'
    - type: chore
```

### `rendering.excludes`

A list of filters that drop matched commits from the rendered changelog / release-notes. Each
entry sets **exactly one** of `type` (match a conventional-commit type) or `regex` (match the
commit subject). This is independent of `commits.types` `remove`, which governs the verify
allow-list rather than the rendered output.

## Content generation

Configured under `changelog` and `release.notes`. `native`, heraut's built-in renderer, is the
only generator (ADR-0045) — there is no `generator:` key to set; an empty `changelog: {}` /
`release: {notes: {}}` block means "generate with native, using defaults."

| Field         | Required | Description                                                                                                                                                                                                                                                                       |
|---------------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `output`      | No       | Output file path (e.g. `CHANGELOG.md`).                                                                                                                                                                                                                                            |
| `tag_pattern` | No       | Tag pattern regex scoping which tags are considered. **For per-env strategies heraut auto-derives this from the effective `tag_format` so `--env <env>` only considers that environment's tags** (e.g. `{version}_{env}` + `--env prod` → `^.+_prod$`); set it explicitly to override the derivation. |
| `template`    | No       | Path to a full custom Go `text/template` file, parsed on top of native's built-ins (ADR-0037). See [Spec 05 § User-customizable templates](05-generators-and-platforms.md#user-customizable-templates-adr-0037). |

See [Spec 05 — Generators and Platforms](05-generators-and-platforms.md) for the full
behaviour of the native generator.

## Platform drivers

Publishing is driven by a `release.targets[]` entry (draft/prerelease/assets) plus the
`forges[].name` it references (host, project/repository, token) — see § `release.targets[]`
and § `forges` above. The drivers themselves (`gh`/`glab`) are unchanged by ADR-0044: only
how they are configured and constructed changed, from a standalone `release.platforms` entry
to a resolved `forges:` identity.

### GitLab

```yaml
forges:
  - name: gitlab
    platform: gitlab
    project: $CI_PROJECT_PATH   # optional, defaults to $CI_PROJECT_PATH
    token_env: GITLAB_TOKEN     # optional, defaults to GITLAB_TOKEN
    base_url: https://gitlab.com  # optional, defaults to https://gitlab.com

release:
  targets:
    - forge: gitlab
      assets:
        - dist/myapp_*            # glob patterns for files to attach
```

A project registered with the GitLab CI/CD Catalog publishes automatically — there is no
`catalog` field.

Implementation: shells out to `glab release create` + `glab release upload --use-package-registry`.

### GitHub

```yaml
forges:
  - name: github
    platform: github
    repository: org/repo        # optional, defaults to $GITHUB_REPOSITORY
    token_env: GH_TOKEN         # optional, defaults to GH_TOKEN
    base_url: https://github.com  # optional, defaults to https://github.com

release:
  targets:
    - forge: github
      draft: false
      prerelease: false
      assets:
        - dist/myapp_*
```

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
  output: CHANGELOG.md

forges:
  - name: github
    platform: github
    repository: acme/widget
    token_env: GH_TOKEN

release:
  notes: {}
  targets:
    - forge: github
```

### CalVer — monthly releases, GitLab

```yaml
version: "1"

versioning:
  strategy: calver
  format: "YYYY.MM.PATCH"
  tag_prefix: ""

changelog:
  output: CHANGELOG.md

forges:
  - name: gitlab
    platform: gitlab

release:
  notes: {}
  targets:
    - forge: gitlab
```

### SemVer per environment — dev → staging → prod chain

```yaml
version: "1"

versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"    # common format: dev/1.0.0, staging/1.0.0, prod/1.0.0

changelog:
  output: CHANGELOG.md
  tag_pattern: "dev/*"

forges:
  - name: gitlab
    platform: gitlab
  - name: github
    platform: github

commits:
  enrichment_forge: gitlab

release:
  notes:
    tag_pattern: "dev/*"
  targets:
    - forge: gitlab

environments:
  dev:
    branch: develop
    bump: auto
    disable_changelog: true        # no CHANGELOG commit on dev
    release:
      targets:
        - forge: gitlab            # dev releases only go to GitLab
  staging:
    branch: main
    bump: promote
    source: dev                    # staging ← dev
  prod:
    branch: main
    bump: promote
    source: staging                # prod ← staging ← dev
    release:
      targets:
        - forge: gitlab
        - forge: github            # prod releases go to both
```

### SemVer — multiple platforms, with binaries

```yaml
version: "1"

versioning:
  strategy: semver
  tag_prefix: "v"

forges:
  - name: gitlab
    platform: gitlab
    token_env: GITLAB_TOKEN
  - name: github
    platform: github
    repository: org/myapp
    token_env: GH_TOKEN

commits:
  enrichment_forge: gitlab

release:
  notes: {}
  targets:
    - forge: gitlab
      assets:
        - dist/myapp_linux_amd64.tar.gz
        - dist/myapp_darwin_arm64.tar.gz
        - dist/checksums.txt
    - forge: github
      assets:
        - dist/myapp_*
```
