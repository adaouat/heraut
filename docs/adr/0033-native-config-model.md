# ADR-0033: Heraut-Native Config Model — Unified `commits` + `rendering`

- **Status**: Accepted
- **Date**: 2026-06-27
- **Deciders**: bchatard

---

## Context

[ADR-0032](0032-native-content-generator.md) introduced the built-in `native` generator and
sketched its config as "a fixed default taxonomy plus an optional `template:` override."
Implementing Phase 1 (T123 taxonomy, T124 templates) made a latent problem concrete: the
commit-type taxonomy is defined in **three disconnected places**:

1. `commit_lint.types` ([ADR-0027](0027-builtin-conventional-commit-checker.md)) — the
   `heraut commit verify` allow-list (a `[]string`).
2. git-cliff's embedded `commit_parsers` TOML — the changelog section taxonomy.
3. T123's hardcoded `group.go` — the native generator's section taxonomy.

Three sources drift. Separately, content config has accreted top-level `tickets:` and
`remote_metadata:` as loose peers of the generator blocks.

A native generator is the moment to make heraut's config **heraut-shaped, not
git-cliff-shaped**. The decision here goes further: **git-cliff is dropped as the design
anchor** — native rendering becomes heraut's *canonical* output (its own spec, validated by
golden snapshots, not a git-cliff parity copy), and git-cliff is slated for removal once
native reaches feature parity (its own follow-up ADR — see the boundary section). heraut does
more than git-cliff — it *lints* commits — so it can unify the commit-type definition that
git-cliff structurally cannot.

## Decision

Replace the `commit_lint` block and the loose top-level `tickets:` / `remote_metadata:` keys
with two heraut-native blocks: **`commits:`** (commit semantics + enrichment) and
**`rendering:`** (content output filtering). Names chosen over the design sample's
`conventional_commit:` / `generator:` (the latter collided with the per-driver `generator:`
tool selector).

### `commits:` — single source of truth for commit semantics + enrichment

Replaces `commit_lint`. Drives **both** `heraut commit verify` / `create` **and** the native
changelog's type→section taxonomy.

- `types:` — a list of objects, **deep-merged over heraut's built-in default type set**
  (it augments the defaults, it does not replace them):
  - `name` (required) — the conventional-commit type word.
  - `order` (optional) — display sort position for changelog sections; types without
    `order` sort after ordered ones (stable/natural).
  - `render` (optional) — the section heading label (e.g. `🚀 Features`); when absent, the
    type name is capitalized (`Chore`).
  - `remove: true` — drop this default type from the **allow-list** (so `heraut commit
    verify` rejects it) without re-listing every other type.
- `scopes:` — allowed scope list.
- `scopes_restricted:` (bool, default `false`) — when true, `heraut commit verify` rejects
  scopes outside `scopes:` (today `scopes` only feeds the `create` wizard; this newly lets
  it gate `verify`).
- `tickets:` — issue-link patterns (moved here from top level).
- `remote_metadata:` — enrichment policy `optional|required|disabled` (moved here from top
  level).
- `types_heading_level:` — the heading depth (`#` count) for type sections in rendered
  output.

### `rendering:` — content output filtering

Replaces the design sample's top-level `generator:` block (renamed to avoid colliding with
the per-driver `generator:` tool selector).

- `excludes:` — a list of filters dropping matched commits from the **rendered**
  changelog / release-notes, each either `{ type: <name> }` or `{ regex: <pattern> }`
  (matched on the commit subject; regex covers cases like `^chore\(deps.*\)`).

The named-partial **template engine** that the design sample places under this block is
**not exposed in Phase 1** (see "Templates are internal in Phase 1").

### `remove` and `excludes` are distinct (clarified)

They operate on different axes and do not overlap:

- `commits.types[].remove: true` → removes a **default type from the allow-list**. It is
  about which types are *valid* for `heraut commit verify`.
- `rendering.excludes` → filters matched commits out of the **rendered output**. It is about
  what *appears in the changelog/notes*.

A type can be valid yet excluded from the changelog, or removed from the allow-list while a
commit using it still renders (if not also excluded). The design sample applied both to
`build` only to demonstrate the keys — that coincidence is not a coupling.

### Internal defaults ("simulated include"), not a user `includes:`

User config stays **minimal**. heraut layers a built-in default set **under** the user's
config by deep-merge (the ADR-0010 pattern, generalized): default `types` (the current
git-cliff/native taxonomy — `feat`→`🚀 Features`, … with their orders), default `excludes`
(`chore(deps)` / `chore(pr)` / `chore(pull)` skips), and the default internal templates. The
user overrides only what they need (add `render`/`order` to a default type, `remove:` one,
append an `exclude`). There is **no user-facing `includes:`** mechanism in this phase;
local/remote config composition is deferred to its own future ADR.

### Templates are internal/private in Phase 1

The named-partial template engine in the design sample (`header` / `footer` /
`commit_message` / `content` with a data model — `.Tags`, `.Types`, `.Commit.sha_link`,
`.Links | render`, `.Heraut.version`, …) is **not exposed to users in Phase 1**. heraut
ships built-in templates, **driven by** the `commits` / `rendering` config (type labels,
order, heading level, excludes). A **public** template engine — with a documented, stable
data contract — is deferred to a later phase: a public template API is the single hardest
thing to walk back, so we earn it only after the config model has settled. The per-driver
`template:` override is therefore also deferred.

### Breaking renames (pre-v1.0 hard cutover)

heraut is pre-v1.0 (trunk-based, no installed base — see
[ADR-0028](0028-drop-cocogitto-generator.md)). These are hard cutovers with no deprecation
window, but deliberate and documented:

- `commit_lint` → `commits` (and `commit_lint.types: []string` → `commits.types: []object`).
- top-level `tickets:` → `commits.tickets:`.
- top-level `remote_metadata:` → `commits.remote_metadata:`.

### git-cliff is dropped as the anchor; removal sequenced after native parity

`commits.types` is the **sole** taxonomy source — it drives native rendering and `heraut
commit verify` / `create`. We do **not** invest in unifying git-cliff's embedded taxonomy
with `commits.types`: git-cliff is being removed, so wiring `commits.types` into a doomed
component is wasted effort. During the transition git-cliff stays **functional but
unreconciled** — it keeps its embedded `commit_parsers` TOML and reads `tickets` /
`remote_metadata` from their new `commits.` home — so nothing breaks while native is built.

The **actual deletion** of the git-cliff package is a separate follow-up ADR, deliberately
**sequenced after native reaches remote-enrichment parity** (native Phase 2 — PR authors /
numbers / contributors). git-cliff's one capability native lacks today is exactly that
enrichment; removing git-cliff before native has it would regress the tool. So: drop git-cliff
as the *design anchor* now (this ADR), delete the *package* later (its own ADR), with no
capability gap in between.

## Consequences

- **T123** (taxonomy) and **T124** (templates) are reworked to read `commits` / `rendering`
  rather than hardcoded data; the grouping algorithm, link composition, and statistics logic
  survive — only the *source* of taxonomy/labels and the template layer change.
- `heraut commit verify` / `create` (T116–T121) migrate from `commit_lint` to `commits`: the
  allow-list becomes `commits.types` minus `remove:`d defaults, and `scopes_restricted`
  newly gates `verify`.
- `internal/config/config.go`, `schema.json`, `docs/heraut.sample.yml`, and the relevant
  specs change; the breaking renames touch existing `testdata/` fixtures.
- ADR-0032 is **superseded** beyond its config-model section: native is no longer "opt-in
  alongside the default git-cliff" — it is heraut's canonical renderer, and git-cliff is
  slated for removal. ADR-0027's `commit_lint` surface is superseded by `commits`. The
  superseded ADRs' other decisions stand as historical record.
- **Parity goal dropped.** T126 changes from "diff native against real `git-cliff --offline`"
  to "golden snapshots of heraut's own canonical output." T124's three noted git-cliff
  deviations become heraut's deliberate choices, not deviations.
- **git-cliff removal** is a sequenced follow-up (its own ADR), after native Phase 2
  enrichment, to avoid regressing PR/author/contributor enrichment. Until then git-cliff
  stays functional, reading `tickets` / `remote_metadata` from their new `commits.` home.
- The native-generator roadmap Phase 1 is restructured: **config model → `commit
  verify/create` migration → rework T123 (taxonomy ← config) → rework T124 (render ←
  config) → wire `generator: native` → canonical golden snapshots**.
- A user-facing **template engine** and a **`includes:`** mechanism are explicitly deferred,
  each to its own future ADR.
