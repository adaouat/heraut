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

Expose a public template API with **two entry points**, right-sized to the job:

1. **Inline block overrides** (the common case) — short Go-template snippets as config keys under
   `rendering.templates.<block>`, overridable per-driver and per-env. Reformat the commit line in
   one YAML line; everything else stays built-in. Schema-documented (IDE autocomplete).
2. **A full template file** (the power case) — `<driver>.template: <path>` points at a real
   `.tmpl` file authored with proper tooling (syntax highlighting, version control, no YAML
   escaping). The file is parsed on top of the built-ins and may redefine the **root** block
   (whole-document replacement) and/or any set of blocks.

Both entry points feed the **same Go `text/template` block set + data contract** — a file that
`{{ define "commit" }}` and an inline `rendering.templates.commit` do the identical thing; the file
is just the ergonomic home for large or multi-block customization. The built-in templates are
**rewritten from fat-injection onto that contract** (dogfooded — our own output is the reference
implementation, guaranteeing the contract works).

---

## Data contract (the public template model)

The root passed to the template is a `Release` (release-notes) or `Changelog` (changelog). Both
carry `.Heraut` (document meta) so `header`/`footer`/root blocks can reach it. Field names are the
public API.

```
Changelog     .Releases []Release   .Heraut          (changelog root)
Release       .Version "1.2.3"   .Tag "v1.2.3"   .Date (time)
              .PreviousTag "v1.2.2"   .CompareURL
              .Groups []Group   .Contributors []Contributor   .Stats   .Heraut   (release-notes root)

Heraut        .Version "0.48.0"   .GeneratedAt (time)   .URL "https://github.com/adaouat/heraut"

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

Two entry points, same underlying blocks: **inline `rendering.templates.<block>` keys** for quick
overrides, and a **`<driver>.template` file** for full / multi-block templates.

```yaml
rendering:                     # global (shared by changelog + release-notes)
  templates:
    commit: "- {{ .Description }} ({{ .ShortHash }})"
    group: "### {{ .Name }}"
    contributor: "* @{{ .Author.Username }} — first contribution 🎉"
    footer: "\n_Generated by [heraut]({{ .Heraut.URL }}) {{ .Heraut.Version }}._"

changelog:
  generator: native
  template: .config/heraut/changelog.tmpl      # optional: a full template FILE (whole doc or blocks)
  rendering:                                   # optional: per-driver inline overrides
    templates:
      header: "# Changelog\n\nAll notable changes.\n"

release:
  notes:
    generator: native
    # inherits the global `commit`/`footer`; no per-driver override
```

The `<driver>.template` file is a Go template parsed on top of the built-ins. It authors full
customization with real tooling and may `{{ define }}` the root block (whole-document replacement)
and/or any set of blocks:

```gotmpl
{{/* .config/heraut/changelog.tmpl — override just two blocks, inherit the rest */}}
{{ define "commit" }}- {{ .Description }}{{ if .PR }} ([{{ .PR.Ref }}]({{ .PR.URL }})){{ end }}{{ end }}
{{ define "group" }}## {{ .Name }}{{ end }}
```

- **`<driver>.rendering`** is the *same* `rendering` block as the global one (fields: `templates`,
  `excludes`), **deep-merged over global** — so `excludes` is also per-driver overridable for free
  (a nice consequence, not special-cased).
- **Precedence (per block key, lowest → highest):** built-in → `rendering.*` inline (global) →
  `<driver>.rendering.*` inline → `environments.<env>.<driver>.rendering.*` inline →
  **`<driver>.template` file** (parsed last, wins for whatever it defines). Per-env merge via
  [ADR-0019](../adr/0019-perenv-content-driver-merge.md). Each level overrides only the keys/blocks
  it sets; unset ones fall through.

### Override points (blocks)

The common, friendly set:

| Key | Context (`.`) | Renders |
|-----|---------------|---------|
| `header` | root (`Changelog` / `Release`) | document header (e.g. the `# Changelog` title) |
| `group` | `Group` | the group **heading** (the commit loop stays built-in) |
| `commit` | `Commit` | one commit's line/block (the most common override) |
| `contributor` | `Contributor` | one "New Contributors" line |
| `footer` | root (`Changelog` / `Release`) | document footer (e.g. a "generated by" line) |

Advanced/structural blocks (settable inline for small ones, but typically authored in the
`<driver>.template` file since they're larger — this is what whole-document control means):
`contributors` (`[]Contributor`), `stats` (`Stats`), `release` (`Release`), `changelog`
(`Changelog`), `release-notes` (`Release`). Internally the built-in `release` renders `header` →
iterates groups (`group` heading + `commit` loop) → `contributors` → `stats` → `footer`. Inline
snippets are synthesized into `{{ define "<key>" }}<snippet>{{ end }}` and parsed together with the
`template` file on top of the built-ins.

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

- A template (inline snippet or `template` file) that **fails to parse or execute** fails the run
  with a clear error naming the offending block key / file path — never silent broken output. Parse
  errors are caught at config-validation time where feasible (snippets and the file are known
  statically); exec errors surface at generation.
- `rendering.templates` and `<driver>.template` **require `generator: native`** (config error
  otherwise, like `tag_pattern`); the `template` file must exist and be readable/parseable.

## Determinism note

`Heraut.GeneratedAt` is a wall-clock timestamp, so the generator takes an injected
`now func() time.Time` (like the CalVer resolver), defaulting to `time.Now`; tests fix the clock.
The built-in templates do **not** render `GeneratedAt`, so golden snapshots stay deterministic — a
user template that renders it opts into non-determinism knowingly.

## Stability

The data contract ships **experimental in v1** (docs say so): additive changes (new fields) are
free; renames / removals are avoided and would be called out. Dogfooding via the built-in
templates keeps the contract honest — if a field the built-ins use were removed, our own output
would break in CI.

## Testing

- Golden snapshots re-baselined (built-in output unchanged — the load-bearing check).
- New tests: each inline override (`header`/`group`/`commit`/`contributor`/`footer`); the merge
  precedence (global < driver < env < file); a `<driver>.template` **file** that both whole-replaces
  a root and overrides individual blocks; parse-error, exec-error, and missing-file paths; the new
  fetch fields per platform (contract tests, incl. GitLab approvers empty); a
  `templates`/`template` requires-native validator test.
- Determinism preserved (MockRunner / httptest; no network; injected clock for `GeneratedAt`).

## Out of scope

- Non-`native` generators (git-cliff keeps its own TOML/Tera).
- Arbitrary/unsafe template funcs (shell, file, network).
- Per-block file references for the *inline* keys (a block key's value is always a literal snippet;
  file-based authoring goes through `<driver>.template`).
- GitLab approvers via the extra `/approvals` call (deferred; field stays empty there).
- A stable/versioned contract guarantee (experimental in v1; revisit at v1.0).
