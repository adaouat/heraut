# ADR-0006: Config Naming — generator / platform / release

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

An early `.heraut.yml` schema used `driver:` as the discriminator field for both content
generators (git-cliff, communique, cocogitto) and release platforms (gitlab, github). It
also placed `platforms:` and `release_notes:` at the root level of the config, alongside
`changelog:`.

Three problems arose:

1. **Naming**: `driver` is an overloaded term — it does not distinguish between something
   that *generates content* and something that *publishes a release*.
2. **Structural ambiguity**: top-level `platforms:` led users to believe it also governed
   changelog behaviour, when in fact it only controls release publishing.
3. **Conceptual grouping**: `release_notes:` and `platforms:` are tightly related (both
   concern the release) but were peers of `changelog:`, obscuring their relationship.

## Decision

### Discriminator fields

| Context                     | Field name             | Example                  |
|-----------------------------|------------------------|--------------------------|
| Content generators          | `generator: <name>`    | `generator: git-cliff`   |
| Platform drivers            | `platform: <name>`     | `platform: github`       |

`generator` accurately names something that produces content. `platform` names the
hosting platform the release is published to.

In Go, the `Platform` config struct uses the field name `Type string` (yaml: `platform`)
to avoid a `Platform.Platform` self-reference, following the conventional Go pattern for
discriminator fields:

```go
type Platform struct {
    Type string `yaml:"platform"`
    // platform-specific fields
}
```

### `release:` section

`release.notes:` and `release.platforms:` are nested under a top-level `release:` key:

```yaml
release:
  notes:          # optional: content generator for release notes
    generator: git-cliff
  platforms:      # optional: where to publish
    - platform: gitlab
    - platform: github
```

`changelog:` stays at the root because it is about the repository artefact
(`CHANGELOG.md` + a git commit), not about the release publishing step.

### Go struct

```go
type Release struct {
    Notes     *ContentDriver `yaml:"notes,omitempty"`
    Platforms []Platform     `yaml:"platforms,omitempty"`
}

type Config struct {
    Version       string         `yaml:"version"`
    Versioning    Versioning     `yaml:"versioning"`
    Changelog     *ContentDriver `yaml:"changelog,omitempty"`
    Release       *Release       `yaml:"release,omitempty"`
    Environments  map[string]EnvOverride `yaml:"environments,omitempty"`
}
```

## Consequences

- The schema (`schema.json`) uses `additionalProperties: false` so validation catches any
  use of the old `driver:` name or top-level `platforms:`.
- `heraut check config` reports clear errors (`changelog.generator is missing`,
  `release.platforms[0].platform is missing`) for stale configs.
- Internal Go code (`config.ContentDriver.Generator`, `config.Platform.Type`) is
  consistent with the YAML surface.
