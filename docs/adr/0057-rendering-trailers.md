# ADR-0057: `rendering.trailers` — per-token commit-footer rendering customization

- **Status**: Accepted
- **Date**: 2026-09-14
- **Deciders**: bchatard
- **Builds on**: [ADR-0037](0037-native-template-api.md) (the `rendering.templates`
  customization mechanism this extends), [ADR-0049](0049-changelog-release-notes-footer-block.md)
  (the current `footer`/`release_footer` block names, whose proximity to this feature's naming
  is discussed below)

---

## Context

Every commit-message footer trailer (`Co-authored-by:`, `Refs:`, `Signed-off-by:`,
`Reviewed-by:`, …) is already parsed generically as a `{Token, Value}` pair
(`conventionalcommit.Footer`, `internal/conventionalcommit/conventionalcommit.go:31-34`) and
carried into the native generator's template model as `tplFooter`
(`internal/generators/native/templatemodel.go:108-111`), populated per commit by `buildCommit`.
`release_notes.tmpl` already loops over `.Footers` and renders every entry the same way —
`{{ .Token }}: {{ .Value }}` — with no way to render `Co-authored-by` differently from `Refs`,
or to suppress a trailer from output entirely. `blocks.tmpl`'s shared `commit` block (used by
`changelog.tmpl`) does not loop over `.Footers` at all, so footers are absent from changelog
output today regardless of token.

`rendering.templates` (ADR-0037) already lets users override *blocks* — `commit`, `group`,
`ticket`, `title`, `footer`, etc. — with Go template snippets, but nothing lets a snippet branch
on *which trailer token* it is currently rendering without hand-rolling an `if`/`else if` chain
over every token the project cares about, repeated in whichever block(s) render footers.

## Decision

**Add `rendering.trailers`, a list of `FooterRule` objects**, matched by token and applied
wherever a template renders a commit's `.Footers`.

```yaml
rendering:
  trailers:
    - token: Co-authored-by
      renderer: "**Co-authored by:** {{ .Value }}"
    - token: Refs
      hide: true
```

```go
// internal/config/commits.go

// Rendering configures content output (ADR-0033).
type Rendering struct {
	Excludes  []Exclude         `yaml:"excludes,omitempty"`
	Templates map[string]string `yaml:"templates,omitempty"`
	// Trailers customizes how individual commit-message footer trailers render in native's
	// changelog/release-notes output, matched by token (ADR-0057).
	Trailers []FooterRule `yaml:"trailers,omitempty"`
}

// FooterRule customizes the rendering of one commit-message footer trailer (a "trailer" in git
// terms — conventionalcommit.Footer). Token is matched case-insensitively, exact-match, against
// the trailer's literal token as parsed (conventionalcommit never normalizes casing). Exactly
// one of Renderer or Hide must be set.
type FooterRule struct {
	Token string `yaml:"token"`
	// Renderer is a Go text/template snippet executed with {Token, Value} as its data context
	// (the same shape as tplFooter) — same idiom as rendering.templates' block snippets.
	Renderer string `yaml:"renderer,omitempty"`
	// Hide drops matching footers from rendered output entirely.
	Hide bool `yaml:"hide,omitempty"`
}
```

**Matching is case-insensitive exact string match, not regex.** Footer tokens are a small,
tool-emitted vocabulary with fixed (if inconsistently cased) spelling — GitHub always writes
`Co-authored-by:`, `git commit -s` always writes `Signed-off-by:`, kernel-convention tooling
writes `Reviewed-by:`/`Acked-by:`. `conventionalcommit.Parse` captures the token verbatim with
no normalization (`internal/conventionalcommit/conventionalcommit.go:197-209`), so an
exact-but-case-sensitive match would silently miss a hand-typed `co-authored-by`. Regex
matching was considered and rejected as unneeded complexity for what is, in every real case,
a label lookup — see Alternatives.

**An unmatched token keeps today's default rendering**, `{{ .Token }}: {{ .Value }}` — no config
required, no behavior change for projects that don't set `rendering.trailers`. (Amended by
[ADR-0058](0058-default-coauthored-by-trailer.md): `Co-Authored-By` is no longer "unmatched"
by default — see that ADR.)

**Applied once per commit, in `buildCommit`.** Rather than exposing a new template func or
duplicating the match/execute logic in every block that might loop over `.Footers`,
`buildCommit` (`internal/generators/native/templatemodel.go`) resolves each parsed footer
against the effective `FooterRule` set once, producing the final display line, and either
appends it (as `tplFooter.Line`, a new field alongside the existing `Token`/`Value`) or omits
the footer from `tplCommit.Footers` entirely (`Hide`). `release_notes.tmpl`'s existing loop
changes from printing `{{ .Token }}: {{ .Value }}` to printing `{{ .Line }}` — behavior-preserving
when no rule matches. Because this happens in the shared model builder, any future changelog
`commit` block override that adds its own `{{ range .Footers }}` loop (see below) gets the same
customized rendering automatically, with no separate wiring.

**Does not change *whether* footers render in the changelog.** `blocks.tmpl`'s `commit` block
stays footer-less by default, exactly as today. A project that wants footers in the changelog
opts in the same way it customizes any other block: override `commit` in `rendering.templates`
to add its own footers loop (existing ADR-0037 lever). `rendering.trailers` governs *how* a
footer renders, never *whether* a given block shows it — adding a second, competing on/off
mechanism was rejected as redundant with the existing block-override lever (see Alternatives).

**Merge semantics mirror `Templates`, not `Types`/`Scopes`.** `Trailers` deep-merges global →
per-driver → per-env by `Token` (case-insensitive) — the app layer computes an
`EffectiveTrailerRules map[string]FooterRule` propagated onto `ContentDriver`, alongside the
existing `EffectiveTemplates` (`internal/config/config.go:162-165`). This is `Templates`'s
override-by-key merge, not `EffectiveTypes`/`EffectiveScopes`'s merge-over-built-in-defaults
merge (`internal/config/commits.go:90-115, 159-184`): there is no built-in trailer-rule set to
merge user rules over, since the default *is* "no rule, fall through to `Token: Value`."

> **Partially superseded by [ADR-0058](0058-default-coauthored-by-trailer.md):** the
> `Co-Authored-By` token now *does* have a built-in default (`config.DefaultTrailers()`,
> merged as the base layer under global `rendering.trailers`), so "there is no built-in
> trailer-rule set" is no longer true for that one token. The deep-merge-by-token mechanism
> described here is otherwise unchanged.

**Validation** (`internal/config/validator.go`), mirroring `CommitRule`'s exactly-one-of pattern
(ADR-0056):

- `token` is required and non-empty.
- Exactly one of `renderer` / `hide` must be set per entry.
- `renderer`, when set, must parse as a valid Go template at config-validate time — the same
  stub-func-map path `rendering.templates` snippets already go through (config cannot import
  `native`; ADR-0037's Consequences note this precedent).
- A duplicate `token` (case-insensitive) within the same `Rendering.Trailers` list is a config
  error — ambiguous which entry would apply.

**Naming: `trailers`, not `footers`.** `footer` (singular) is already the document-level
credit-line block key (ADR-0049), and `tplCommit.Footers` is already the model field name for a
commit's parsed trailers. A `rendering.footers` config key — considered first — would sit
textually next to `rendering.templates.footer` while meaning something unrelated, reopening
exactly the kind of same-word-different-meaning ambiguity ADR-0048 deliberately resolved for
`header`/`release_header`. `trailers` — git's own formal term for these lines — avoids the
collision at the config-surface level while the Go types underneath keep the codebase's
existing "footer" vocabulary (`FooterRule`, matching `conventionalcommit.Footer`), since only
the YAML key needed disambiguating.

**Deliberately not bundled: a broader `rendering.templates` restructuring.** A nested shape —
grouping `release.header`/`release.footer`/`release.commit.{title,tickets,body,footers}` under
`rendering.templates` instead of today's flat `map[string]string` — was raised alongside this
feature and has real appeal (groups related blocks, gives `trailers` a more discoverable home).
It was deliberately kept out of this ADR: it renames every existing block key (`title` →
`header` reopens ADR-0048's disambiguation in the other direction, `footer`/`release_footer`
move again after being renamed three weeks prior by ADR-0049), turns `Templates` from a flat,
trivially-deep-mergeable map into a nested struct with its own merge-semantics question, and
makes `body` newly overridable when it isn't today (ADR-0037: *"release-notes body/footers stay
built-in"*). That is an independently-justified, independently-breaking decision and belongs in
its own future ADR and roadmap phase, not folded silently into this one.

## Consequences

- **Real config-schema growth**, tracked as the roadmap tasks below: `internal/config/commits.go`
  (`FooterRule`, `Rendering.Trailers`), `internal/config/validator.go`, `internal/config/config.go`
  (`EffectiveTrailerRules` propagation, mirroring `EffectiveTemplates`),
  `internal/generators/native/templatemodel.go` (`buildCommit` applies rules; `tplFooter` gains
  `Line`), `internal/generators/native/release_notes.tmpl`, `schema.json`,
  `docs/heraut.sample.yml`, `docs/specs/05-generators-and-platforms.md`.
- **Byte-identical default output.** No project's rendered changelog/release notes change unless
  `rendering.trailers` is set — unmatched tokens keep rendering exactly as today, and the
  changelog still shows no footers by default. (Amended by
  [ADR-0058](0058-default-coauthored-by-trailer.md) for the `Co-Authored-By` token: it is no
  longer byte-identical by default.)
- **Error contract mirrors `rendering.templates`.** A `renderer` snippet that fails to
  parse/execute fails the run with an error naming the offending token, never silent broken
  output (same shape as ADR-0037 § Validation & errors).
- **The nested-restructuring idea above is unscoped and untracked** beyond this note — picking
  it up later means deciding its own merge semantics and block-rename surface from scratch, and
  `rendering.trailers`'s location would likely move again if/when it lands (e.g. under
  `release.commit.footers`). Accepted as a known, deferred cost of shipping the narrower feature
  now.

## Alternatives considered

- **Structured `label`/`icon` fields instead of a free-form `renderer` template.** Rejected:
  less flexible (can't reorder value before label, add Markdown links, combine with other
  footer fields) and inconsistent with `rendering.templates`'s established idiom of "the
  customization surface is a Go template snippet," used everywhere else in this config.
- **One big `rendering.templates.footer_line`-style block with an in-template `if`/`eq .Token`
  chain**, instead of independent per-token list entries. Rejected: forces every
  footer-rendering customization through one growing conditional block rather than small,
  independently-addable entries, and the natural key name for that block (`footer`) is already
  taken by the document-level credit-line block.
- **`rendering.footers` as the config key.** Rejected in favor of `trailers` — see Naming above.
- **Nesting under a restructured `rendering.templates` now**, bundling the broader schema
  reorganization into this ADR. Rejected: independently-justified, independently-breaking
  change with its own open design questions (merge semantics, `header` naming, newly-overridable
  `body`); deferred to its own future ADR — see the dedicated note above.
- **Regex `token` matching.** Rejected as YAGNI: footer tokens are a small, fixed-spelling,
  tool-emitted vocabulary; case-insensitive exact match covers real usage without the added
  failure surface of user-authored regexes for what amounts to a label lookup.
- **A toggle to turn footers on/off per block** (e.g. `rendering.trailers.enabled_in:
  [changelog, release_notes]`), instead of relying on the existing `commit` block override to
  opt the changelog in. Rejected: redundant with the block-override lever ADR-0037 already
  provides — adding a second on/off mechanism for the same decision invites the two to
  disagree.
