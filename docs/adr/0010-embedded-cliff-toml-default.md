# ADR-0010: Embedded `cliff.toml` Default with Optional User Override

- **Status**: Superseded by [ADR-0045](0045-native-sole-generator.md)
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

> **Superseded (2026-08-14).** heraut no longer ships `git-cliff` as a generator — `native` is
> its sole content generator ([ADR-0045](0045-native-sole-generator.md)). This ADR's entire
> subject — the embedded `cliff.toml` defaults and their merge semantics — is gone along with
> `internal/generators/gitcliff/`. Kept as a historical record of the decision at the time.

## Context

When a project selects `git-cliff` as a generator in `.heraut.yml`, the underlying
binary needs a `cliff.toml`. A naive approach would require every project to commit a
full `cliff.toml`, leading to:

1. **Boilerplate**: a 150-line cliff.toml in every project, even when the opinionated
   defaults are fine.
2. **Hard partial customisation**: changing a single section (e.g. `[remote.gitlab]`)
   means copying the entire file and editing one block — with no indication of what
   differs from the default.

## Decision

heraut embeds two opinionated `cliff.toml` defaults in the binary (via `//go:embed`):

- **Changelog variant** (`cliff.changelog.toml`) — includes a stats block, version
  header, and the full Conventional Commits taxonomy. Used when the generator is
  configured under `changelog:`.
- **Release-notes variant** (`cliff.release-notes.toml`) — no header, no stats; just
  the body suitable for the release page. Used when the generator is configured under
  `release.notes:`.

When `config:` is set in `.heraut.yml`, heraut deep-merges the user file with the
embedded default at runtime, using a simple two-layer model:

```
Layer 1 (base)     — embedded opinionated default (ships in the binary)
Layer 2 (override) — optional user file, path from cfg.config
                     absent file → layer 1 is used as-is (no error)
```

### Merge semantics

- **Maps / TOML tables**: merged recursively — only keys present in the override are
  updated; absent keys keep the base value.
- **All other types** (scalars, arrays): the override value replaces the base value
  outright. Arrays are never concatenated or partially replaced.

Examples:

| User writes                                  | Effect                                                              |
|----------------------------------------------|---------------------------------------------------------------------|
| `[changelog]`<br>`header = "# My App"`       | Only the header changes; body, trim, etc. come from the default     |
| `[git]`<br>`commit_parsers = [...]`          | Entire parser list replaced; `conventional_commits` etc. unchanged  |
| `[remote.gitlab]`<br>`owner = "…"`<br>`repo = "…"` | Remote section added; all other sections from default          |
| _(no file)_                                  | Embedded default used verbatim                                      |

### Implementation

1. `internal/generators/gitcliff/embed.go` exposes the embedded TOML bytes via
   `//go:embed cliff.changelog.toml cliff.release-notes.toml`.
2. `internal/generators/gitcliff/merge.go` performs a pure TOML deep-merge using
   `github.com/pelletier/go-toml/v2`.
3. `Generator.prepareConfig()` loads both layers, merges, writes to a temp file, and
   returns the path + a cleanup func. Called at the start of every `Generate()` and
   `Validate()`.
4. `--config <tempfile>` is always passed to git-cliff; the temp file is removed after
   the command completes (including in dry-run).
5. `heraut init` does **not** write a `cliff.toml`; the wizard prints a tip explaining
   the layered model and points users at `heraut cliff changelog` /
   `heraut cliff release-notes` for inspecting the effective config.

## Consequences

- **Zero boilerplate for the common case**: projects that accept the defaults need no
  `cliff.toml` at all.
- **Surgical overrides**: users write only what differs. The rest is inherited
  transparently.
- **Predictable**: arrays replace, maps merge. No partial-array operations, no implicit
  ordering — easy to reason about.
- **No PATH dependency at init time**: the embedded default is always available; the
  external `git-cliff` binary is only needed at changelog-generation time.
- **Temp file overhead**: one small temp file created and deleted per
  `heraut release` / `heraut changelog` / `heraut check cliff` invocation — negligible.
- **`--config` is always a temp path in tests**: contract tests filter out `--config`
  from arg assertions and check its presence separately; integration tests do the same.

## Changing the embedded defaults

The embedded TOML files define the opinionated default look of every changelog and
release-notes output heraut produces. Changes to either file are user-visible: existing
users with a custom override will see different effective config after upgrading. Treat
any change to the embedded defaults as a behaviour change requiring its own ADR.
