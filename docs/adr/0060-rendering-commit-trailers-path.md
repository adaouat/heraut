# ADR-0060: Move `rendering.trailers` to `rendering.templates.commit.trailers`

- **Status**: Accepted
- **Date**: 2026-09-14
- **Deciders**: bchatard
- **Supersedes**: the config path (not the mechanism) introduced in
  [ADR-0057](0057-rendering-trailers.md) — `rendering.trailers` → `rendering.templates.commit.trailers`.
  Everything else ADR-0057 decided (the `FooterRule` shape, case-insensitive exact-token
  matching, exactly-one-of `renderer`/`hide`, deep-merge-by-token semantics) is unchanged, and
  the built-in `Co-Authored-By` default layered underneath it
  ([ADR-0058](0058-default-coauthored-by-trailer.md)) is unaffected.
- **Builds on**: [ADR-0059](0059-namespaced-template-blocks.md) (the `release.*`/`commit.*`
  block-namespacing vocabulary this adopts).

---

## Context

[ADR-0057](0057-rendering-trailers.md) shipped `rendering.trailers` as a flat, top-level sibling
of `rendering.templates`, and flagged its own eventual relocation in its Consequences section:

> The nested-restructuring idea above is unscoped and untracked beyond this note — picking it up
> later means deciding its own merge semantics and block-rename surface from scratch, and
> `rendering.trailers`'s location would likely move again if/when it lands (e.g. under
> `release.commit.footers`). Accepted as a known, deferred cost of shipping the narrower feature
> now.

[ADR-0059](0059-namespaced-template-blocks.md) then shipped exactly that nested restructuring for
`rendering.templates`, namespacing the release- and commit-cadence *block overrides* under
`release:`/`commit:` (`release.section`, `commit.message`, `commit.ticket`,
`commit.contributor`, …). Reviewing that change surfaced the follow-up ADR-0057 anticipated:
`rendering.trailers` is a per-commit-footer-token customization — conceptually commit-cadence,
the same category as `commit.message`/`commit.ticket`/`commit.contributor` — but it was left
behind at the old flat `rendering.trailers` path, orphaned from the vocabulary ADR-0059 just
established.

**This ADR's own first cut was wrong and is amended in place, not superseded.** The first draft
of this decision moved `rendering.trailers` to `rendering.commit.trailers` — a new `commit:`
object at the `rendering:` level, sibling to `rendering.templates` rather than nested inside it,
chosen because it avoided a real implementation wrinkle (see Decision below). Implemented,
documented, and committed the same session, then reviewed again before it was ever relied on by
any tagged release: the result was two different, same-named `commit` namespaces at two
different depths under `rendering:` — `rendering.templates.commit.*` for block overrides and
`rendering.commit.trailers` for the rule list — which defeats the entire point of ADR-0059's
namespacing (one discoverable place per concept). Because nothing outside this repository has
ever depended on the interim `rendering.commit.trailers` path, this ADR is corrected in place
rather than superseded by a new one — there is no real-world config to migrate away from, unlike
every other breaking rename in this project's history.

## Decision

**Move `rendering.trailers` to `rendering.templates.commit.trailers`** — inside the *existing*
`rendering.templates.commit` object, alongside `message`/`ticket`/`contributor`:

```yaml
rendering:
  templates:
    commit:
      message: "- {{ .Description }} ({{ .ShortHash }})"
      trailers:
        - token: Co-authored-by
          renderer: "**Co-authored by:** {{ .Value }}"
        - token: Refs
          hide: true
```

**The implementation wrinkle this requires.** `rendering.templates` is parsed by
`config.TemplateOverrides` (`map[string]string`, ADR-0059) — every leaf under `release:`/
`commit:` is expected to be a template-snippet string. `trailers` is a *list* of `{token,
renderer, hide}` rules, not a string, so it cannot flow through `TemplateOverrides`'s flat map
unmodified. Rather than accept that mismatch (this ADR's first cut) or change
`TemplateOverrides` to a heterogeneous `map[string]any` (loses type safety for every other
block), `Rendering` gets its own `UnmarshalYAML`:

```go
// internal/config/commits.go

// Rendering configures content output (ADR-0033).
type Rendering struct {
	Excludes []Exclude `yaml:"excludes,omitempty"`
	// Templates is populated by Rendering.UnmarshalYAML below, not decoded directly from its own
	// struct tag — the custom decode needs to see the raw templates node before handing it to
	// TemplateOverrides, so it can pull commit.trailers out first.
	Templates TemplateOverrides `yaml:"-"`
	// Commit holds rendering.templates.commit's one non-snippet field — Trailers — extracted by
	// UnmarshalYAML from the same templates.commit node TemplateOverrides decodes (ADR-0060).
	Commit *RenderingCommit `yaml:"-"`
}

// RenderingCommit holds rendering.templates.commit's non-snippet fields (ADR-0060).
type RenderingCommit struct {
	Trailers []FooterRule
}
```

`Rendering.UnmarshalYAML` decodes `excludes` and `templates` normally (the latter still through
`TemplateOverrides.UnmarshalYAML`, invoked explicitly), then makes a second, narrow pass over the
same raw `templates` node looking only for `commit.trailers`, decoding it into `Commit.Trailers`
when present. `TemplateOverrides.UnmarshalYAML` gains one line: when walking a `commit:` mapping,
it skips a `trailers` sub-key instead of trying (and failing) to decode it as a string — that key
is `Rendering.UnmarshalYAML`'s to handle, not its own.

**Everything else about the feature is unchanged**, and everything ADR-0060's first cut already
built on top of `Rendering.Commit` carries over untouched: `FooterRule`'s fields, case-insensitive
exact-token matching, the exactly-one-of `renderer`/`hide` validation, the deep-merge-by-token
semantics (global → per-driver → per-env, ADR-0057), the built-in `Co-Authored-By` default
(ADR-0058), `mergeRenderingCommit`, and `effectiveTrailers` all work identically whether `Commit`
was populated by a plain struct-tag decode (the first cut) or by this extraction (only *how*
`Rendering.Commit` gets populated changed, not what anything downstream does with it).
`validateTrailers`'s error paths become `rendering.templates.commit.trailers[i].*`.

## Consequences

- **Breaking rename, no alias** — pre-v1.0, same precedent as ADR-0048/ADR-0049/ADR-0059: any
  project with the original `rendering.trailers` set fails to load, with the `removedKeys`
  migration hint (`internal/config/loader.go`) pointing at the new path.
- **A small, contained custom-decode surface**, not a wholesale nested-struct rewrite of
  `rendering.templates`: `TemplateOverrides` keeps its flat-map, dotted-key design for every
  block (ADR-0059) unchanged; only `Rendering` itself gains a custom `UnmarshalYAML` to carve out
  one non-string leaf.
- **`schema.json`**: `trailers` is a property of the `commit` object nested inside
  `rendering.templates`, alongside `message`/`ticket`/`contributor` (JSON Schema has no trouble
  with a `string`-typed sibling next to an `array`-typed one in the same object — the type
  constraint that forced `Rendering`'s custom decode is a Go implementation detail, not a schema
  one).
- **One discoverable `commit` namespace, not two.** This was the entire motivation for correcting
  the first cut: everything commit-cadence — block overrides and the trailers rule list alike —
  now lives under the one path a user would already look at,
  `rendering.templates.commit`.

## Alternatives considered

- **`rendering.commit.trailers`** (this ADR's own first cut — a new `commit:` object sibling to
  `rendering.templates`, not nested inside it). Rejected on review: avoided the custom-decode
  wrinkle above, but produced two different `commit` namespaces at two depths under `rendering:`,
  reopening the same discoverability problem ADR-0059 existed to solve. Implementation
  convenience lost to config-authoring clarity.
- **`rendering.commit.footer`/`rendering.commit.footers`.** Considered even earlier, before this
  ADR was first written. Rejected: `footer` (document-level credit-line block, ADR-0049) and
  `release.footer` (per-release trailing block, ADR-0059) already exist — introducing a third,
  differently-shaped `footer` at `commit.footer` reopens exactly the same-word-different-meaning
  ambiguity ADR-0048/ADR-0057 each deliberately avoided. `trailers` — ADR-0057's own established
  term, and git's formal name for these lines — has no such collision.
- **A heterogeneous `TemplateOverrides` (`map[string]any`).** Rejected: loses compile-time type
  safety for every other block key to accommodate exactly one exception, and complicates
  `mergeRendering`'s per-key merge (which currently just copies string values) for no benefit
  beyond avoiding one small custom-decode method.
- **Leave `rendering.trailers` where it is.** Rejected: ADR-0057 itself flagged the orphaning
  this ADR fixes, and now that ADR-0059 has established `commit.*` as live vocabulary elsewhere
  in the same `rendering:` tree, leaving `trailers` un-namespaced is the more surprising state,
  not the safer one.
