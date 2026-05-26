# Spec 07 — Per-environment changelog and release notes

This spec defines how changelog generation and release notes generation can be
restricted to specific environments. It extends
[Spec 02 — Configuration](02-configuration.md) and is only meaningful for the
`semver-per-env` and `calver-per-env` strategies.

## Problem

With per-env strategies, a single `changelog` and `release.notes` configuration applies
to every environment. The two common needs are:

1. **Changelog only for selected envs** — committing `CHANGELOG.md` on every `dev` cut
   is noisy and creates merge friction. Only `prod` (or `prod` + `staging`) should
   generate a changelog entry.
2. **Release notes only for selected envs** — a `dev` release may go to the platform
   (for traceability) but with no notes text attached.

## Two mechanisms — choose based on intent

### `disable_changelog` / `disable_notes` — per-env blacklist

Set on individual environments inside `versioning.environments`. Suitable when most envs
generate a changelog and you want to opt specific ones out.

```yaml
versioning:
  environments:
    dev:
      bump: auto
      disable_changelog: true   # dev never generates a changelog
    prod:
      bump: promote             # prod runs normally
```

### `changelog.env` / `release.notes.env` — centralized whitelist

Set directly on the `changelog` or `release.notes` block. Suitable when only one (or
few) environments should ever generate content, and the rest should be silent by default.

```yaml
changelog:
  generator: git-cliff
  output: CHANGELOG.md
  env: prod                # only runs when --env prod

release:
  notes:
    generator: git-cliff
    env: prod              # same guard
```

The two mechanisms are equivalent in what they produce; pick the one that reads more
clearly for your config.

## `changelog.env`

### Field

Added to the `changelog` block (a `ContentDriver`). See
[Spec 02 § Content generators](02-configuration.md#content-generators).

| Field | Type   | Required | Default | Description |
|-------|--------|----------|---------|-------------|
| `env` | string | No       | `""`    | Environment name that the active `--env` must match for changelog generation to run. When empty (default), changelog runs for all environments. |

### Behavior

- `heraut release --env prod` with `changelog.env: prod` → changelog runs normally.
- `heraut release --env dev` with `changelog.env: prod` → changelog step is skipped; no
  file is written, no commit is made. The pipeline continues with tagging and platform
  publication.
- `heraut changelog --env dev` with `changelog.env: prod` → exits 0 with an info
  message, same as `disable_changelog: true`.

### Non-per-env strategies

`changelog.env` is silently ignored for `semver` and `calver` strategies because `--env`
is never active. Validation emits an error if `changelog.env` is set on a non-per-env
strategy — this is almost certainly a misconfiguration.

### Precedence

`disable_changelog: true` (in `versioning.environments.<env>`) takes full precedence. If
both are set, `disable_changelog` wins regardless of whether the env matches.

## `release.notes.env`

### Field

Added to the `release.notes` block (also a `ContentDriver`).

| Field | Type   | Required | Default | Description |
|-------|--------|----------|---------|-------------|
| `env` | string | No       | `""`    | Environment name the active `--env` must match for release notes generation to run. When empty, notes run for all environments. |

### Critical distinction — notes filter, not release filter

`release.notes.env` controls **notes generation only**. It does not affect whether the
platform release is created.

When the env does not match:

| Step | Result |
|------|--------|
| Generate release notes | **Skipped** — no notes text produced |
| Create platform release | **Still runs** — release created with no notes attached |
| Upload assets | **Still runs** — assets are attached regardless |

This mirrors the existing behaviour of `disable_notes: true`: the platform release entry
is created (so the tag is visible and assets are downloadable) but without a description
body.

Example: `release.notes.env: prod` with `heraut release --env dev`:

```
✓ version resolved: dev/1.4.0
✓ changelog generated (if configured)
✓ tag dev/1.4.0 created and pushed
  release notes: skipped (env: prod)
✓ gitlab release created (no notes)
```

If you want to suppress platform publication entirely for certain environments, use
`environments.<env>.release` overrides — that feature is currently deferred; see
[roadmap](../tasks/roadmap.md) for the tracking task.

### Non-per-env strategies

Same as `changelog.env`: silently ignored, and validation errors if set on a non-per-env
strategy.

### Precedence

`disable_notes: true` (in `versioning.environments.<env>`) takes full precedence over
`release.notes.env`.

## Precedence summary

| Condition | Changelog | Notes |
|-----------|-----------|-------|
| `disable_changelog: true` for active env | skipped | — |
| `disable_notes: true` for active env | — | skipped |
| `changelog.env` set, active env matches | runs | — |
| `changelog.env` set, active env does not match | skipped | — |
| `release.notes.env` set, active env matches | — | runs |
| `release.notes.env` set, active env does not match | — | skipped, platform release still created |
| Neither set | runs (if `changelog` block present) | runs (if `release.notes` block present) |

## Validation rules

| Condition | Result |
|-----------|--------|
| `changelog.env` set on `semver` or `calver` strategy | Config error |
| `release.notes.env` set on `semver` or `calver` strategy | Config error |
| `changelog.env` names an env not in `versioning.environments` | Config error |
| `release.notes.env` names an env not in `versioning.environments` | Config error |

## Examples

### Changelog and notes for prod only

```yaml
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
  environments:
    dev:
      bump: auto
    prod:
      bump: promote
      source: dev

changelog:
  generator: git-cliff
  output: CHANGELOG.md
  tag_pattern: "prod/*"
  env: prod

release:
  notes:
    generator: git-cliff
    tag_pattern: "prod/*"
    env: prod
  platforms:
    - platform: github
      repository: acme/widget
```

`heraut release --env dev` → resolves version, creates tag, creates GitHub release (no
notes, no changelog commit).

`heraut release --env prod` → resolves version, generates `CHANGELOG.md`, commits it,
creates tag, generates notes, creates GitHub release with notes.

### Notes for all envs, changelog only for prod

Dev and staging releases appear on the platform with notes (so the release is
self-describing), but only prod writes to `CHANGELOG.md`.

```yaml
versioning:
  strategy: calver-per-env
  format: "YYYY.MM.PATCH"
  environments:
    dev:
      bump: auto
      disable_changelog: true
    staging:
      bump: promote
      source: dev
      disable_changelog: true
    prod:
      bump: promote
      source: staging

changelog:
  generator: git-cliff
  output: CHANGELOG.md
  tag_pattern: "prod/*"

release:
  notes:
    generator: git-cliff
  platforms:
    - platform: gitlab
```

### Notes only for prod, dev/staging released silently

```yaml
versioning:
  strategy: semver-per-env
  tag_format: "{env}/{version}"
  environments:
    dev:
      bump: auto
      disable_changelog: true
    prod:
      bump: promote
      source: dev

release:
  notes:
    generator: git-cliff
    tag_pattern: "prod/*"
    env: prod
  platforms:
    - platform: github

# No changelog block at all — neither env generates one.
```

## Deferred: top-level `environments` overrides

The config schema already defines a top-level `environments` map
(`environments.<env>.changelog`, `environments.<env>.release`) that would allow
per-environment changelog output paths, generators, and platform lists. This machinery
is parsed and validated today but **not applied by the pipeline**. A future task will
decide whether to implement it or remove it. See the roadmap for the tracking entry.
