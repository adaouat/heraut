# User-Customizable Templates for the Native Generator — Design

- **Date**: 2026-07-04
- **Status**: Approved (brainstorm) — pending implementation plan
- **Scope**: a public template API for the `native` generator — the last major git-cliff-parity
  feature. Builds directly on the exposed-model bridge from
  [ADR-0036](../adr/0036-unified-enrichment-model.md).

---

## Problem

`native` renders changelogs / release-notes with internal Go `text/template`s using the
**fat-injection / thin-template** pattern ([ADR-0022](../adr/0022-fat-injection-thin-templates.md)):
Go builds each line as a finished string, the template just prints it. Users cannot change the
*format* of anything — only the config knobs (`commits.types` labels/order,
`commits.types_heading_level`, `rendering.excludes`). Full output control means falling back to
`generator: git-cliff` (TOML/Tera). [ADR-0033](../adr/0033-native-config-model.md) deferred a
public template engine, calling its **data contract "the single hardest thing to get right"** —
once exposed, it is a compatibility commitment.

## Decision

Expose a public template API with **two tiers of friendliness**:

1. **Inline block overrides** (the common case) — short Go-template snippets as config keys under
   `rendering.templates.<block>`, overridable per-driver and per-env. Reformat the commit line in
   one YAML line, no external file, schema-documented (IDE autocomplete).
2. **Whole-document replacement** (the power case) — a `<driver>.template` file path that supplies
   a full Go template with total layout control.

Both use the **same Go `text/template` engine and the same public data contract**. The built-in
templates are **rewritten from fat-injection onto that contract** (dogfooded — our own output is
the reference implementation, guaranteeing the contract works).

---

## Data contract (the public template model)

The root passed to the template is a `Release` (release-notes) or `{ Releases []Release }`
(changelog). Field names are the public API.

```
Release       .Version "1.2.3"   .Tag "v1.2.3"   .Date (time)
              .PreviousTag "v1.2.2"   .CompareURL
              .Groups []Group   .Contributors []Contributor   .Stats   .Heraut.Version

Group         .Name "🚀 Features"   .Commits []Commit

Commit        .Type "feat"   .Scope "api"   .Breaking (bool)
              .Description "Add OAuth login"   .Subject (raw)   .Body
              .Hash   .ShortHash   .CommitURL   .Date (time)
              .Author (Author)
              .PR    → nil | (PullRequest)
              .Tickets []Link
              .Footers []Footer

Author        .Name   .Email   .Username

PullRequest   .Number   .URL   .Title   .Labels []string   .Ref ("#42"/"!42")
              .Author (Author)
              .CreatedAt (time)   .MergedAt (time)   .MergedBy (Author)   .Approvers []Author

Contributor   .Author   .PR

Stats         .CommitCount   .ConventionalCount   .TimespanDays
              .DaysSincePrev   .HasDaysSincePrev   .Tickets []StatTicket(.Text .Href .Count)

Link          .Text   .Href
Footer        .Token   .Value
```

Deliberate choices:
- **`.PR` is nil when absent** (templates use `{{ if .PR }}`), not empty-but-present.
- **`.Ref`** is precomputed (`#`/`!` per platform) so templates never hardcode a prefix.
- All `.PR.*` fields are **remote-only** (empty offline, same as today's `.Number`/`.URL`).

### Fields requiring a fetch extension (new since ADR-0036)

`PullRequest` gains `CreatedAt`, `MergedAt`, `MergedBy`, `Approvers`; `Commit` gains `Date` and
`Footers` (both free — from git / the parser). Fetch cost:

| Field | GitHub | GitLab | Azure |
|-------|--------|--------|-------|
| `CreatedAt` / `MergedAt` | GraphQL `createdAt`/`mergedAt` | MR `created_at`/`merged_at` | PR `creationDate`/`closedDate` |
| `MergedBy` | GraphQL `mergedBy` | MR `merged_by` | PR `closedBy` |
| `Approvers` | GraphQL `reviews(...)` state=APPROVED | **best-effort: empty** (needs a separate `/approvals` call per MR — not paid) | PR `reviewers` vote ≥ 10 |

`Approvers` is **best-effort** (user decision): populated on GitHub + Azure (data already in the
objects we fetch / one query extension), left **empty on GitLab** to avoid O(commits) extra API
calls. Documented as such.

---

## Config surface

```yaml
rendering:                     # global (shared by changelog + release-notes)
  templates:
    commit: "- {{ .Description }} ({{ .ShortHash }})"
    group: "### {{ .Name }}"
    contributor: "* @{{ .Author.Username }} — first contribution 🎉"

changelog:
  generator: native
  template: .config/heraut/changelog.tmpl     # optional: whole-document replacement (file)
  rendering:                                   # optional: per-driver override of the rendering block
    templates:
      commit: "- {{ .Description }} ({{ .ShortHash }}){{ if .PR }} (#{{ .PR.Number }}){{ end }}"

release:
  notes:
    generator: native
    # inherits the global `commit`; no per-driver override
```

- **`<driver>.rendering`** is the *same* `rendering` block as the global one (fields: `templates`,
  `excludes`), **deep-merged over global** — so `excludes` is also per-driver overridable for free
  (a nice consequence, not special-cased).
- **Precedence (per block key, lowest → highest):** built-in → `rendering.*` (global) →
  `<driver>.rendering.*` → `environments.<env>.<driver>.rendering.*` (via the existing per-env
  content-driver merge, [ADR-0019](../adr/0019-perenv-content-driver-merge.md)). Each level
  overrides only the keys it sets.
- **`<driver>.template`** (file) is whole-document replacement. It is parsed **last**, so any block
  it `{{ define }}`s takes highest precedence — above the inline keys and the built-ins. Full
  precedence for a given block: built-in → `rendering.*` inline → `<driver>.rendering.*` inline →
  `environments.<env>.<driver>.rendering.*` inline → `<driver>.template` file. Mixing a file with
  inline keys is allowed but unusual; the file simply wins for whatever it defines.

### Override points (blocks)

Inline `rendering.templates` keys — the common, friendly set:

| Key | Context (`.`) | Renders |
|-----|---------------|---------|
| `commit` | `Commit` | one commit's line/block (the most common override) |
| `group` | `Group` | the group **heading** (the commit loop stays built-in) |
| `contributor` | `Contributor` | one "New Contributors" line |

A whole-document **file** may additionally `{{ define }}` the structural blocks the inline keys
don't expose: `contributors` (`[]Contributor`), `stats` (`Stats`), `release` (`Release`),
`changelog` (`{ Releases }`), `release-notes` (`Release`). Internally the built-in `release`
iterates groups → for each, renders the `group` heading + loops `commit`; then `contributors`,
then `stats`. Inline snippets are synthesized into `{{ define "<key>" }}<snippet>{{ end }}` and
parsed on top of the built-ins.

### Template funcs

A small, documented, safe set — no arbitrary/OS access:
`upperFirst`, `date "<layout>" .Date`, `join <sep> <list>`, `indent <n>`, `trim`.

---

## Rendering refactor (dogfooding)

The built-in `changelog.tmpl` / `release_notes.tmpl` are rewritten from fat-injection onto the
public blocks + funcs: the `render.go` line-builders (`buildCommitLine`, `buildCommitBlock`, the
contributors/stats helpers) become template blocks and template funcs. The view model shifts from
pre-rendered `Line`/`Block` strings to the structured `Release`/`Group`/`Commit`/… contract above.

**Invariant:** built-in output is **byte-identical** to before (same result, different path). The
golden snapshots ([T126](../tasks/native-generator-roadmap.md)) are **re-baselined** — regenerated
and diff-reviewed to confirm the diff is empty.

## Errors & validation

- A template (inline or file) that **fails to parse or execute** fails the run with a clear error
  naming the offending block / file — never silent broken output.
- `rendering.templates` / `<driver>.template` **require `generator: native`** (config error
  otherwise, like `tag_pattern`); a `template:` file must exist and parse at config-validation time
  where feasible, else at generation.

## Stability

The data contract ships **experimental in v1** (docs say so): additive changes (new fields) are
free; renames / removals are avoided and would be called out. Dogfooding via the built-in
templates keeps the contract honest — if a field the built-ins use were removed, our own output
would break in CI.

## Testing

- Golden snapshots re-baselined (built-in output unchanged — the load-bearing check).
- New tests: each inline override (`commit`/`group`/`contributor`); the merge precedence
  (global < driver < env); a whole-document `template:` file replacement; parse-error and
  exec-error paths; the new fetch fields per platform (contract tests, incl. GitLab approvers
  empty); a `template` requires-native validator test.
- Determinism unchanged (MockRunner / httptest; no network).

## Out of scope

- Non-`native` generators (git-cliff keeps its own TOML/Tera).
- Arbitrary/unsafe template funcs (shell, file, network).
- GitLab approvers via the extra `/approvals` call (deferred; field stays empty there).
- A stable/versioned contract guarantee (experimental in v1; revisit at v1.0).
