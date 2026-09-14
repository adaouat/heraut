# ADR-0060: Move `rendering.trailers` to `rendering.commit.trailers`

- **Status**: Accepted
- **Date**: 2026-09-14
- **Deciders**: bchatard
- **Supersedes**: the config path (not the mechanism) introduced in
  [ADR-0057](0057-rendering-trailers.md) — `rendering.trailers` → `rendering.commit.trailers`.
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

## Decision

**Move `Rendering.Trailers []FooterRule` to `rendering.commit.trailers`** — a new `commit:`
object, sibling to `rendering.templates`, not nested inside it:

```yaml
rendering:
  commit:
    trailers:
      - token: Co-authored-by
        renderer: "**Co-authored by:** {{ .Value }}"
      - token: Refs
        hide: true
```

**Not `rendering.templates.commit.trailers`.** `rendering.templates.commit` (ADR-0059) already
holds `message`/`ticket`/`contributor` — three plain Go-template-snippet strings, stored as
`config.TemplateOverrides` (`map[string]string`) with a custom `UnmarshalYAML` that assumes
every leaf is a snippet string. `trailers` is a *list* of `{token, renderer, hide}` rules — a
structurally different value shape. Squeezing it into the same map would force
`TemplateOverrides` to become heterogeneously typed (`string | []FooterRule`), breaking its
ADR-0059 flattening logic and every downstream consumer (`mergeRendering`,
`effectiveTemplates`, `buildTemplateSet`) that assumes a flat `map[string]string`. A sibling
`rendering.commit` object keeps `commit.*` as the shared vocabulary root for "things that vary
per commit," while letting each mechanism keep its own shape (string map vs. rule list)
underneath its own key.

```go
// internal/config/commits.go

// Rendering configures content output (ADR-0033).
type Rendering struct {
	Excludes  []Exclude         `yaml:"excludes,omitempty"`
	Templates TemplateOverrides `yaml:"templates,omitempty"`
	// Commit groups rendering config that varies per commit — currently just Trailers — mirroring
	// rendering.templates.commit's namespace (ADR-0059) for config that isn't itself a template
	// snippet. Named RenderingCommit, not Commit, to avoid colliding with the unrelated top-level
	// Commits struct (conventional-commit type/scope/ticket taxonomy) in this same file.
	Commit *RenderingCommit `yaml:"commit,omitempty"`
}

// RenderingCommit is rendering.commit's value type (ADR-0060).
type RenderingCommit struct {
	// Trailers customizes how individual commit-message footer trailers render, matched by
	// token (ADR-0057; relocated here by ADR-0060).
	Trailers []FooterRule `yaml:"trailers,omitempty"`
}
```

**Everything else about the feature is unchanged.** `FooterRule`'s fields, case-insensitive
exact-token matching, the exactly-one-of `renderer`/`hide` validation, the deep-merge-by-token
semantics (global → per-driver → per-env, ADR-0057), and the built-in `Co-Authored-By` default
layered underneath (`config.DefaultTrailers()`, ADR-0058) all carry over verbatim — only the
config path moves. `effectiveTrailers` (`internal/app/pipeline.go`) reads
`cfg.Rendering.Commit.Trailers` / `driver.Rendering.Commit.Trailers` instead of
`cfg.Rendering.Trailers` / `driver.Rendering.Trailers`, nil-checked one level deeper;
`MergeFooterRules`/`config.DefaultTrailers()` are untouched. `validateTrailers`
(`internal/config/validator.go`) is called against `cfg.Rendering.Commit.Trailers` (nil-safe),
and its error paths become `rendering.commit.trailers[i].*` instead of `rendering.trailers[i].*`.

**A struct field removal needs a `removedKeys` migration hint, unlike ADR-0059's block
renames.** ADR-0059 renamed keys *inside* `rendering.templates`, a `map[string]string` — an old
flat key like `release_header` still decodes fine (maps accept any key), and only fails later at
`validateTemplateSnippets`'s semantic "unknown template block" check, which already prints a
helpful hint. `Rendering.Trailers` is a plain **struct field**, not a map entry: removing it
means a project's `rendering.trailers:` YAML hits the strict-decode `KnownFields` check
(`internal/config/loader.go`'s `forgeconfig.Decode`) and fails with a generic "unknown field"
error, not an actionable one. This needs a `removedKeys` entry (`internal/config/loader.go`,
alongside `changelog.remote` and friends) so `checkRemovedKeys` catches it first and reports
`` `rendering.trailers` — rename to `rendering.commit.trailers` `` instead.

## Consequences

- **Breaking rename, no alias** — pre-v1.0, same precedent as ADR-0048/ADR-0049/ADR-0059: any
  project with `rendering.trailers` set fails to load, with the `removedKeys` migration hint
  above rather than a raw strict-decode error.
- **Config-schema growth, tracked as the roadmap tasks below**: `internal/config/commits.go`
  (`RenderingCommit`, `Rendering.Commit`, drop `Rendering.Trailers`), `internal/config/loader.go`
  (`removedKeys` entry + `checkRemovedKeys` probe field), `internal/config/validator.go`
  (`validateTrailers` call site + error paths), `internal/config/merge.go` (`mergeRendering`
  merges `Commit.Trailers` one level deeper — currently it merges `Trailers` directly by
  reassigning the slice via `MergeFooterRules`; moving it under a pointer struct means a nil
  `Commit` on either side needs the same nil-else-merge handling `mergeRendering` already gives
  `Rendering` itself), `internal/app/pipeline.go` (`effectiveTrailers`'s two field reads),
  `schema.json` (`trailers` moves from a `Rendering` property to a new `commit` object's
  property), `docs/heraut.sample.yml`, `docs/specs/02-configuration.md`,
  `docs/guides/template-customization.md`.
- **Doc heading anchors stay put, same lesson as ADR-0059.** Spec 02's `### rendering.trailers
  (ADR-0057, ADR-0058)` heading and the guide's matching `## Customizing footer trailers
  (rendering.trailers, ADR-0057)` heading are linked from multiple other doc files by their
  current Markdown anchor slugs; per the precedent ADR-0059 established, the heading text (and
  therefore its anchor) stays fixed to the founding ADRs, with ADR-0060 and the path change
  cited in prose instead, not appended to the heading.
- **`FooterRule`'s own shape is unchanged.** No change needed to `internal/generators/native`
  beyond how the effective rule map reaches it — `buildCommit`, `tplFooter.Line`, and
  `release_notes.tmpl`'s `.Footers` loop are untouched.

## Alternatives considered

- **`rendering.templates.commit.trailers`** (nested inside the existing `templates` object).
  Rejected — see Decision above: forces `TemplateOverrides` to hold a non-string leaf, breaking
  its ADR-0059 flattening contract.
- **`rendering.commit.footer`/`rendering.commit.footers`.** Considered, since it was the name
  proposed before this ADR was written up. Rejected: `footer` (document-level credit-line block,
  ADR-0049) and `release.footer` (per-release trailing block, ADR-0059) already exist —
  introducing a third, differently-shaped `footer` at `commit.footer` reopens exactly the
  same-word-different-meaning ambiguity ADR-0048/ADR-0057 each deliberately avoided. `trailers`
  — ADR-0057's own established term, and git's formal name for these lines — has no such
  collision.
- **Leave `rendering.trailers` where it is.** Rejected: ADR-0057 itself flagged the orphaning
  this ADR fixes, and now that ADR-0059 has established `commit.*` as live vocabulary elsewhere
  in the same `rendering:` tree, leaving `trailers` un-namespaced is the more surprising state,
  not the safer one.
