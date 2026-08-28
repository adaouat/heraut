# ADR-0037: Native generator public template API

- **Status**: Accepted
- **Date**: 2026-07-06
- **Deciders**: bchatard
- **Builds on**: [ADR-0022](0022-fat-injection-thin-templates.md) (fat-injection pattern, now
  retired for the built-ins), [ADR-0033](0033-native-config-model.md) (which deferred the public
  template engine, calling the data contract "the single hardest thing to get right"),
  [ADR-0036](0036-unified-enrichment-model.md) (the normalized enrichment model this contract
  exposes).

---

## Context

The native generator rendered changelogs / release-notes with internal Go `text/template`s under
the **fat-injection / thin-template** pattern ([ADR-0022](0022-fat-injection-thin-templates.md)):
Go built each finished line as a string and the template only printed it. Users could not change
the *format* of anything — only config knobs (`commits.types`, `commits.types_heading_level`,
`rendering.excludes`). Full output control meant falling back to `generator: git-cliff` (Tera).

[ADR-0033](0033-native-config-model.md) deferred a public template engine deliberately: once the
data a template sees is exposed, it is a compatibility commitment. With the unified enrichment
model ([ADR-0036](0036-unified-enrichment-model.md)) giving native a normalized cross-platform
data shape, the contract was finally worth pinning down. Full design:
[`docs/superpowers/specs/2026-07-04-user-customizable-templates-design.md`](../superpowers/specs/2026-07-04-user-customizable-templates-design.md).

## Decision

Expose a public template API for the **native** generator with **two entry points**:

1. **Inline block overrides** (the common case) — short Go-template snippets as config keys under
   `rendering.templates.<block>`, overridable per-driver and per-env. Reformat one block in a
   single YAML line; everything else stays built-in. Schema-documented (IDE autocomplete).
2. **A full template file** (the power case) — `<driver>.template: <path>` points at a real
   `.tmpl` file authored with editor tooling (syntax highlighting, no YAML escaping). Parsed on
   top of the built-ins, it may redefine the document **root** and/or any set of blocks.

Both feed the **same block set + data contract**. The built-in templates were **rewritten from
fat-injection onto that contract** (dogfooded): our own output is the reference implementation, so
the contract cannot rot without breaking heraut's own golden tests.

### Block set

> **Superseded by [ADR-0048](0048-changelog-title-subtitle-blocks.md):** `header` → `release_header`,
> `release-notes` → `release_notes`, and new `title`/`subtitle` blocks. The table below reflects the
> block set as originally shipped; see ADR-0048 for the current set.

`internal/generators/native/blocks.tmpl` (shared) + the two roots (`changelog.tmpl`,
`release_notes.tmpl`). Overridable block keys:

| Key | Context | Renders |
|-----|---------|---------|
| `header` | root (`Release`) | document/section header (the changelog `## [ver]` heading; empty for notes) |
| `group` | `Group` | the group heading |
| `commit` | `Commit` | one commit's line (the most common override) |
| `contributor` | `Contributor` | one "New Contributors" line |
| `contributors` | `Release` | the whole New Contributors section |
| `stats` | `Release` | the Commit Statistics block |
| `footer` | root (`Release`) | document footer (empty by default) |
| `changelog` / `release-notes` | `Release` | the document roots (whole-document control) |

**Two commit variants, one overridable block.** The changelog renders a one-line commit; the
release-notes root renders `{{ template "commit" . }}` and then, when the commit has a body /
footers, appends them indented *around* the shared `commit` block (not inside it). So overriding
`commit` reformats the line in both modes, while the release-notes body/footers stay built-in.

### Data contract

The template root is a `Release` (see the `tpl*` types in
[`internal/generators/native/templatemodel.go`](../../internal/generators/native/templatemodel.go)).
`.Heraut` (version, URL, generated-at) is reachable from `header`/`footer`/root blocks. `HeadingPrefix`
(from `commits.types_heading_level`) rides on the model, not template literals. `.PR` is nil when
absent; `.PR.Ref` is precomputed (`#`/`!` per platform). All `.PR.*` fields are remote-only (empty
offline). New since ADR-0036: `PullRequest` gains `CreatedAt`, `MergedAt`, `MergedBy`, `Approvers`;
`Commit` gains `Date` and `Footers`. **Approvers is best-effort** — populated on GitHub + Azure,
left empty on GitLab (a separate `/approvals` call per MR is not paid for).

### Funcs

A small, safe set — no OS/file/network: `upperFirst`, `date`, `join`, `list`, `indent`, `trim`.

### Precedence

Per block key, lowest → highest: built-in → `rendering.templates` (global) →
`<driver>.rendering.templates` → `environments.<env>.<driver>.rendering.templates` →
`<driver>.template` file (parsed last, wins for whatever it defines). Per-env merge via
[ADR-0019](0019-perenv-content-driver-merge.md); the app layer collapses global/driver/env into
`ContentDriver.EffectiveTemplates`.

### Determinism

`Heraut.GeneratedAt` is wall-clock, so the generator takes an injected `now func() time.Time`
(default `time.Now`; tests fix it). The built-in templates never render `GeneratedAt`, so golden
snapshots stay deterministic — a user template that renders it opts into non-determinism knowingly.

### Validation & errors

`rendering.templates` / `template` require `generator: native` (config error otherwise, like
`tag_pattern`). Each inline snippet is parsed at config-validation time; the `template` file must
exist and parse. A snippet or file that fails to parse/execute fails the run with an error naming
the offending block key / path — never silent broken output.

## Consequences

- **Byte-identical built-ins.** The rewrite is behaviour-preserving: the golden snapshots
  ([T126]) pass unchanged, no re-baseline. This was the load-bearing success criterion.
- **Experimental in v1.** The data contract ships experimental: additive changes (new fields) are
  free; renames/removals are avoided and would be called out. Dogfooding keeps it honest.
- **No new dependencies; layer rule holds.** Everything is stdlib `text/template` plus existing
  internal packages. The config validator parses snippets with a small stub func-map mirroring the
  native func names (config cannot import `native`).
- **git-cliff keeps its own Tera engine.** This API is native-only; non-native generators are out
  of scope.

## Alternatives considered

- **Inline-only (no file).** Rejected: large or multi-block customization in YAML strings gets no
  editor tooling (highlighting, Go-template linting) and needs escaping. The file is the ergonomic
  home for whole-document control.
- **File-only (no inline).** Rejected: reformatting one line should not require a whole template
  file. The common case deserves a one-line config key with schema autocomplete.
- **A richer/sprig-style func set.** Rejected (YAGNI + safety): the six funcs cover the built-ins'
  needs; arbitrary/unsafe funcs (shell, file, network) are explicitly excluded.
