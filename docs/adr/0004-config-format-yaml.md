# ADR-0004: Config File Format — YAML

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

The per-project configuration file (`.heraut.yml`) is the primary interface between
teams and the tool. Two formats were considered: YAML and TOML.

The implementation language ([ADR-0001](0001-language-go.md)) supports both well, so the
decision is decoupled from the language.

## Decision

Use **YAML** as the configuration file format. The canonical filename is `.heraut.yml`
(or `.config/heraut.yml`, see [ADR-0005](0005-config-file-discovery.md)).

## Consequences

**Positive**
- Consistent with the surrounding CI context: `.github/workflows/*.yml` and
  `.gitlab-ci.yml` sit next to `.heraut.yml`; no mental model switch for the person
  writing both files.
- JSON Schema + `yaml-language-server` is the most mature IDE validation story. The
  `# yaml-language-server: $schema=…` pragma provides inline autocomplete and error
  highlighting in VS Code, JetBrains, and Helix with no extra plugins.
- YAML is already familiar to users coming from GitHub Actions, GitLab CI, Symfony,
  Kubernetes, Helm, and similar ecosystems.
- Strict parsing with line numbers (unknown keys fail loudly) addresses YAML's main
  failure mode — silent typos.

**Negative / trade-offs**
- git-cliff uses `cliff.toml` — teams using git-cliff already have a TOML file in the
  repo; having `.heraut.yml` alongside introduces a second format. Mitigated by the
  fact that `.heraut.yml` references `cliff.toml` by path rather than inlining its
  content. cocogitto's `cog.toml` and communique's `communique.toml` add the same minor
  multi-format friction.
- YAML has well-known footguns (implicit type coercion, indentation sensitivity).
  Mitigated by strict schema validation + actionable error messages (see
  [Spec 06 — DX](../specs/06-dx-and-testing.md)).

## Alternatives considered

**TOML** — idiomatic for Rust tooling (Cargo, git-cliff). Rejected because:
- JSON Schema IDE tooling for TOML is significantly less mature.
- Users' primary context (GitHub Actions / GitLab CI) is YAML.
- TOML's strengths (no indentation rules, explicit table syntax) matter less for a
  hierarchical config of this depth.

**JSON** — rejected: no comments, more punctuation, and no IDE story better than YAML's.
