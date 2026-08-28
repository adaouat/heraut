# ADR-0048: Changelog/release-notes title & subtitle template blocks

- **Status**: Accepted
- **Date**: 2026-08-28
- **Deciders**: bchatard
- **Supersedes**: the block-set table in [ADR-0037](0037-native-template-api.md) (`header` →
  `release_header`, `release-notes` → `release_notes`; adds `title`/`subtitle`)

---

## Context

[ADR-0037](0037-native-template-api.md) exposed the native generator's template blocks as public
config, but the changelog's document-level title was never one of them: `internal/generators/
native/render.go` hardcoded `const changelogHeader = "# Changelog\n\n"`, prepended once per file
by `buildAllSections` on bootstrap or `--regenerate` only — unreachable from
`rendering.templates.header` (which is the **per-release** version heading, e.g.
`## [1.2.3] - 2026-08-28`, rendered once per section) or from a `<driver>.template` file. The
`header` block's name read as "the document header," which hid this gap until a user asked
directly whether the changelog's `# Changelog` title could be customized.

Full design: [`docs/superpowers/specs/2026-08-28-changelog-title-subtitle-design.md`](../superpowers/specs/2026-08-28-changelog-title-subtitle-design.md).

## Decision

1. **Renamed** the per-release heading block from `header` to `release_header` — the name now
   says what varies (fires again for every release/section).
2. **Added** `title`/`subtitle` — new blocks that fire exactly **once per document**, on both the
   changelog and release-notes drivers. They execute against a bare `tplHeraut` context
   (`.Version` `.URL` `.GeneratedAt` directly — not `.Heraut.Version`), not the full `Release`,
   since they are not tied to any single release.
3. **Renamed** `release-notes` (root block) to `release_notes`, so every block key uses a
   consistent `snake_case` separator.

### Block set (supersedes ADR-0037's table)

| Key | Was | Context (`.`) | Fires |
|---|---|---|---|
| `title` | *(new)* | bare `tplHeraut` | once per document |
| `subtitle` | *(new)* | bare `tplHeraut` | once per document |
| `release_header` | `header` | `Release` | once per rendered release |
| `group`, `commit`, `ticket`, `contributor`, `contributors`, `stats`, `footer` | unchanged | unchanged | unchanged |
| `changelog` | unchanged | `Release` | once per rendered release (the per-section root) |
| `release_notes` | `release-notes` | `Release` | once |

### Defaults

`changelog`'s `title` built-in is `"# Changelog"` — byte-identical to the former
`changelogHeader` constant when unset. `changelog`'s `subtitle`, and both `release_notes` blocks,
default to empty — contributing zero bytes, no stray blank line, same pattern `footer` already
used.

### The rendering-scope wrinkle

Every other block executes once per rendered release. `title`/`subtitle` cannot follow that path
for the **changelog**: `buildAllSections` calls the per-section render once per section (newest,
then every historical tag) — title/subtitle must render exactly once regardless of section count.
The new `renderPreamble` helper (`internal/generators/native/render.go`) executes `title` and
`subtitle` as independent named-block executions against `tplHeraut`, trims each, and joins them
with a blank line — contributing nothing when both are unset. `buildAllSections` calls it once and
prepends the result; the ordinary incremental splice path is untouched (it never re-renders the
preamble, exactly as before). Release notes has no such wrinkle — `renderReleaseNotes` calls the
same `renderPreamble` helper once per render, prepending its result to the per-release body.

## Consequences

- **Byte-identical default output.** Nobody's rendered `CHANGELOG.md`/release notes change unless
  they explicitly set `title`/`subtitle`, or already had `rendering.templates.header`/
  `release-notes` set — those now fail config validation with an actionable "unknown template
  block" error naming the new key.
- **Breaking rename, no alias.** Consistent with the project's pre-v1.0 stance (SemVer 0.x).
- **A documented context asymmetry.** `title`/`subtitle` reach heraut's own metadata via
  `.Version`/`.URL`/`.GeneratedAt` directly; every other block reaches the same data via
  `.Heraut.Version` etc. This is called out explicitly in the guide and schema descriptions to
  avoid the obvious footgun.

## Alternatives considered

- **A plain (non-templated) string config field** instead of a template block. Rejected: no
  `.Heraut`/funcs access, and inconsistent with every other piece of native's output being
  template-driven.
- **A backward-compatible alias** for the renamed keys. Rejected: no other block rename in this
  codebase has one; pre-v1.0 breaking changes are the established norm here.
