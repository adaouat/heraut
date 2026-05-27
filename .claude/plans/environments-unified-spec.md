# Unified `environments` Block — Design Spec

## Problem

Per-environment configuration is currently split across two blocks:

```yaml
versioning:
  strategy: semver-per-env
  environments:           # ← versioning fields: bump, tag_format, source, …
    dev:
      bump: auto
      disable_changelog: true   # ← content flag inside a versioning struct

environments:             # ← content overrides (parsed, validated, never applied)
  dev:
    changelog:
      generator: communique
```

A user has to look in two places to understand what `dev` does. The split is driven by
implementation structure, not user mental model. `disable_changelog` / `disable_notes`
are content flags sitting inside a versioning struct. The ghost root `environments` block
is parsed but never applied.

## Solution

Collapse into a single root `environments` map. Each entry owns all per-env settings —
versioning and content — in one place.

```yaml
versioning:
  strategy: semver-per-env

environments:
  dev:
    bump: auto
    tag_format: "dev/{version}"
    disable_changelog: true
    changelog:
      generator: communique
  prod:
    bump: promote
    source: staging
    tag_format: "prod/{version}"
    release:
      platforms:
        - platform: github
```

---

## Field reference — `environments.<name>`

All fields are optional unless noted. Only valid when `strategy` is `semver-per-env` or
`calver-per-env`; the validator rejects `environments` on flat strategies (see §
Validation rules).

### Versioning fields (moved from `versioning.environments`)

| Field        | Required           | Default | Description |
|--------------|--------------------|---------|-------------|
| `bump`       | Yes (per-env)      | —       | `auto` or `promote`. |
| `tag_format` | Conditional        | —       | Tag format for this env; must contain `{version}`. Overrides `versioning.tag_format`. Required unless a common `versioning.tag_format` using `{env}` is set. |
| `branch`     | No                 | —       | Branch this env is released from. Informational — used in error messages. |
| `source`     | No                 | —       | Source env for `bump: promote`. See bump-mode rules in Spec 04. |

No behavioural change to bump-mode rules (auto, promote, E001/E002/E003, source
resolution, cycle detection). These move unchanged from their current location.

### Content fields (moved from `EnvVersioning`; `changelog`/`release` formerly `EnvOverride`)

| Field               | Default | Description |
|---------------------|---------|-------------|
| `disable_changelog` | `false` | When `true`, skips changelog generation and the git commit for this env. Takes precedence over `changelog:` if both are set (see § Warnings). |
| `disable_notes`     | `false` | When `true`, skips release notes generation. The platform release is still created but without notes. Takes precedence over `release.notes:` if both are set. |
| `changelog`         | —       | Override the root `changelog` block for this env (full replacement). Absent means use the root default. |
| `release`           | —       | Override `release.platforms` and/or `release.notes` for this env. Fields inherit independently (see § Override semantics). |

---

## Override semantics

### `changelog` field

- **Absent** → use root `changelog`
- **Present** → replace root `changelog` entirely for this env (full replacement, not merge)
- `disable_changelog: true` takes precedence when both are set

### `release` field

Field-level inheritance within `release`:

| Sub-field           | Absent in env                 | Present in env               |
|---------------------|-------------------------------|------------------------------|
| `release.platforms` | Use root `release.platforms`  | Replace entirely for this env |
| `release.notes`     | Use root `release.notes`      | Replace entirely for this env |

Rationale: users most often want to swap just the platform list (e.g. staging → GitLab
only, prod → GitHub + GitLab) without repeating the notes configuration on every env.
Field-level inheritance makes that natural.

`disable_notes: true` takes precedence over `release.notes:` when both are set.

### Effective config resolution (pipeline builder logic)

```
effective.changelog  = env.Changelog  ?? root.Changelog
effective.notes      = env.Release.Notes     ?? root.Release.Notes
effective.platforms  = env.Release.Platforms ?? root.Release.Platforms

if env.DisableChangelog → skip changelog step entirely
if env.DisableNotes     → skip notes step entirely
```

---

## Validation rules

All existing per-env validation rules are preserved, now reading from root
`environments` instead of `versioning.environments`:

- Per-env strategy requires `environments` to be present and non-empty.
- Each entry must have `bump` set.
- `tag_format` required unless `versioning.tag_format` contains `{env}`.
- `source` on a `bump: auto` env → error.
- `source` must reference an existing env name → error.
- Self-referencing `source` → error.
- Multiple `auto` envs without explicit `source` on every `promote` env → error.
- Cycle detection in `source` chains → error.

### New: flat strategy guard (hard error)

```
environments is only valid with semver-per-env or calver-per-env.
  current strategy: semver

Hint: remove the environments block, or change the strategy to
      semver-per-env or calver-per-env.
```

### New: warnings for contradictory per-env content flags

When `disable_changelog: true` AND `changelog:` are both set on the same env:

```
environments.dev: disable_changelog: true makes the changelog override unreachable.
  disable_changelog skips the changelog step entirely; the changelog: block is ignored.

Hint: remove either disable_changelog: true (to apply the override) or the
      changelog: block (to keep the step disabled).
```

Same pattern for `disable_notes: true` + `release.notes:`.

These are `ValidationError` entries with `Hint` populated, attached via
`ValidationErrors` — same type as existing semantic errors, just non-fatal (warnings
are printed but do not cause a non-zero exit from `heraut check config`).

Warnings cause a **non-zero exit** from `heraut check config` — same as errors.
The distinction between warning and error is in the message tone, not the exit code.

---

## Go struct changes

### Removed

```go
config.EnvVersioning                      // replaced by Environment
config.EnvOverride                        // replaced by Environment
config.Versioning.Environments field      // moved to Config.Environments
```

### Added

```go
// Environment is the unified per-env block under the root environments map.
type Environment struct {
    // Versioning (moved from EnvVersioning)
    Bump      string `yaml:"bump"`
    TagFormat string `yaml:"tag_format,omitempty"`
    Branch    string `yaml:"branch,omitempty"`
    Source    string `yaml:"source,omitempty"`

    // Content (moved from EnvVersioning + EnvOverride)
    DisableChangelog bool           `yaml:"disable_changelog,omitempty"`
    DisableNotes     bool           `yaml:"disable_notes,omitempty"`
    Changelog        *ContentDriver `yaml:"changelog,omitempty"`
    Release          *EnvRelease    `yaml:"release,omitempty"`
}

// EnvRelease holds per-env release overrides. Both fields are optional;
// absent means "inherit from root release".
type EnvRelease struct {
    Notes     *ContentDriver `yaml:"notes,omitempty"`
    Platforms []Platform     `yaml:"platforms,omitempty"`
}
```

### Changed

```go
// Config.Environments
map[string]EnvOverride  →  map[string]Environment
```

### Why `EnvRelease` and not reuse `Release`

The YAML is identical either way — this is purely a Go implementation detail.

The reason for a dedicated type is nil semantics. At root level, `Release.Notes == nil`
means "don't generate release notes". At env level, the same nil means "inherit from
root". A single type carrying two different nil contracts is a future bug waiting to
happen: the pipeline builder must always remember which context it is in, and nothing
in the type system enforces that.

A dedicated `EnvRelease` makes the contract unambiguous by name: fields are always
optional-meaning-inherit, never optional-meaning-disabled.

---

## Wizard changes (`internal/scaffold/`)

### `Answers` and `EnvAnswer`

`EnvAnswer` gains two boolean fields:

```go
type EnvAnswer struct {
    Name             string
    Bump             string
    TagFormat        string
    Source           string
    Branch           string
    DisableChangelog bool   // new
    DisableNotes     bool   // new
}
```

Per-env content overrides (`changelog:`, `release:`) are **not** added to the wizard —
they are advanced config that users edit in YAML directly. The wizard covers the common
case; the schema and sample config document the rest.

### `runEnvWizard`

Add two confirm prompts at the end of each env loop (after branch/source):

```
Disable changelog for this environment? (y/N)
Disable release notes for this environment? (y/N)
```

Both default to `false` (pre-filled from `EnvAnswer` when re-running the wizard on an
existing config).

### `answersToConfig`

```go
// Before
cfg.Versioning.Environments = make(map[string]config.EnvVersioning, …)
cfg.Versioning.Environments[e.Name] = config.EnvVersioning{…}

// After
cfg.Environments = make(map[string]config.Environment, …)
cfg.Environments[e.Name] = config.Environment{
    Bump:             e.Bump,
    TagFormat:        e.TagFormat,
    Source:           e.Source,
    Branch:           e.Branch,
    DisableChangelog: e.DisableChangelog,
    DisableNotes:     e.DisableNotes,
}
```

### `ConfigToAnswers`

```go
// Before
for name, env := range cfg.Versioning.Environments { … }

// After
for name, env := range cfg.Environments {
    a.Environments = append(a.Environments, EnvAnswer{
        Name:             name,
        Bump:             env.Bump,
        TagFormat:        env.TagFormat,
        Source:           env.Source,
        Branch:           env.Branch,
        DisableChangelog: env.DisableChangelog,
        DisableNotes:     env.DisableNotes,
    })
}
```

---

## Files affected

| File | Change |
|------|--------|
| `internal/config/config.go` | Remove `EnvVersioning`, `EnvOverride`, `Versioning.Environments`; add `Environment`, `EnvRelease`; update `Config.Environments` type |
| `internal/config/validator.go` | Read from `cfg.Environments`; add flat-strategy guard (error); add disable+override warnings |
| `internal/config/validator_test.go` | Update per-env fixtures/table tests; add flat-strategy and warning tests |
| `internal/app/resolver.go` | Read versioning fields from `cfg.Environments[env]` |
| `internal/app/pipeline.go` | Apply content overrides from `cfg.Environments[env]` in pipeline builders |
| `internal/app/current.go` | Read `cfg.Environments` |
| `internal/scaffold/wizard.go` | Add `DisableChangelog`/`DisableNotes` to `EnvAnswer`; add two confirm prompts in `runEnvWizard` |
| `internal/scaffold/generate.go` | `answersToConfig` writes to `cfg.Environments`; `ConfigToAnswers` reads from `cfg.Environments` |
| `internal/scaffold/generate_test.go` | Update roundtrip tests |
| `schema.json` | Remove `versioning.environments`; add unified `environments` with all fields |
| `docs/heraut.sample.yml` | Rewrite per-env examples |
| `docs/specs/02-configuration.md` | Replace split sections with unified `environments` block |
| `docs/specs/04-versioning.md` | Update all per-env YAML examples |
| `testdata/config/valid/` | Update per-env fixtures to use root `environments` |
| `testdata/config/invalid/` | Add fixture for flat-strategy + environments (new hard error) |

---

## Breaking change

Any `.heraut.yml` with `versioning.environments` fails after this change. Strict YAML
parsing produces: `line N: field environments not found in type config.Versioning`.

Migration is mechanical:

```yaml
# Before
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

# After
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
```

Heraut is pre-v1.0; this is the right moment for the break.
