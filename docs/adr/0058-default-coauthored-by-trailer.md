# ADR-0058: Built-in default `Co-Authored-By` trailer rendering

- **Status**: Accepted
- **Date**: 2026-09-14
- **Deciders**: bchatard
- **Supersedes**: the "no built-in trailer-rule set" merge-semantics decision and the
  "byte-identical default output" consequence in
  [ADR-0057](0057-rendering-trailers.md) (`rendering.trailers`), for the `Co-Authored-By`
  token only — everything else in ADR-0057 (the config shape, case-insensitive exact-match
  token lookup, `renderer`/`hide` validation, deep-merge-by-token semantics) is unchanged
  and this ADR builds on it directly.

---

## Context

[ADR-0057](0057-rendering-trailers.md) shipped `rendering.trailers`, deliberately with no
built-in trailer-rule set: an unmatched token falls through to `{{ .Token }}: {{ .Value }}`,
and that ADR's own Consequences section calls out "byte-identical default output" as a
design goal — no project's rendered changelog/release notes change unless it explicitly
sets `rendering.trailers`.

In practice, the very first real use of the feature (configuring heraut's own
`.config/heraut.yml` to de-emphasize its `Co-Authored-By:` trailers, added by nearly every
commit in this repo per [`CLAUDE.md`](../../CLAUDE.md)'s attribution convention) showed the
gap: `Co-authored-by`/`Co-Authored-By` is not a project-specific customization, it is the
single most common trailer heraut's own users are likely to want rendered as a de-emphasized
credit line rather than the raw `Co-authored-by: Name <email>` — the same way GitHub itself
renders it distinctly in its own UI. Requiring every project to hand-author the identical
`rendering.trailers` entry just to get a sensible default is exactly the kind of repeated
boilerplate `commits.types`/`commits.scopes` already avoid via a built-in default set merged
under user config (`config.EffectiveTypes`, `config.EffectiveScopes` — the shape ADR-0057
explicitly considered for `Trailers` and rejected, on the grounds that no default was worth
shipping yet).

## Decision

**Add `config.DefaultTrailers()`, a built-in `[]FooterRule{...}` of exactly one entry:**

```go
func DefaultTrailers() []FooterRule {
	return []FooterRule{
		{Token: "Co-Authored-By", Renderer: "_Co-Authored-By: {{ .Value }}_"},
	}
}
```

**Merged under user config as the base layer, once, at the same point `rendering.trailers`
already resolves global→driver→env** (`effectiveTrailers`, `internal/app/pipeline.go`):

```go
withDefaults := config.MergeFooterRules(config.DefaultTrailers(), global)
merged := config.MergeFooterRules(withDefaults, perDriver)
```

`MergeFooterRules` (ADR-0057, unchanged) already implements exactly the override-wins-by-token
semantics this needs: a user entry for `co-authored-by` (any casing) replaces the built-in
wholesale; a project that wants the raw `Token: Value` line back, or wants to suppress the
trailer entirely, uses the *existing* escape hatches — no new config surface:

```yaml
rendering:
  trailers:
    - token: Co-Authored-By
      renderer: "Co-authored-by: {{ .Value }}"   # revert to the old default line
    # — or —
    - token: Co-Authored-By
      hide: true                                  # drop it entirely
```

**Only `Co-Authored-By`, not a broader built-in set.** `Refs`, `Signed-off-by`,
`Reviewed-by`, `Acked-by`, etc. have no comparably universal rendering preference — adding
built-ins for them speculatively would be exactly the YAGNI ADR-0057 already rejected. This
ADR ships the one token with a demonstrated, immediate use case; a future default for another
token is a new ADR when a real need shows up, not a bundled addition here.

**heraut's own `.config/heraut.yml` needs no `rendering.trailers` entry as a result** — the
built-in now covers it. (A prior attempt to add that entry directly to config was reverted in
favor of this native-code default, precisely because the config-only version would have left
every other heraut project without it.)

## Consequences

- **Default output changes for the `Co-Authored-By` token.** ADR-0057's "byte-identical
  default output" guarantee no longer holds for this one token: any project whose commits
  carry a `Co-authored-by:`/`Co-Authored-By:` trailer (common — GitHub inserts it on
  multi-author commits/co-authored PRs, `git commit --trailer`, and any AI pairing tool that
  follows the same convention) now renders it as `_Co-Authored-By: {{ .Value }}_` in release
  notes by default, instead of the raw `Co-authored-by: Name <email>` line — unless the
  project overrides or hides it as shown above.
- **Still governs formatting, not visibility**, unchanged from ADR-0057: the changelog's
  `commit` block still does not loop over `.Footers` by default, so this default has no
  effect there unless a project has already opted the changelog in via
  `rendering.templates.commit`.
- **`internal/config` gains `DefaultTrailers()`, tested directly** (mirrors `defaultTypes()`
  /`defaultScopes()` — but exported, since the merge call site lives in `internal/app`, not
  `internal/config`, unlike `EffectiveTypes`/`EffectiveScopes` which are called from within
  the same package chain that owns their built-in lists).
- **No schema.json change.** `FooterRule`'s shape is unchanged; only the *effective* set
  resolved at runtime gains an implicit first entry. `docs/heraut.sample.yml`'s existing
  commented `Co-authored-by` example and `docs/specs/02-configuration.md`'s "unmatched token"
  language both need a one-line note that `Co-Authored-By` is no longer "unmatched" by
  default.

## Alternatives considered

- **Leave it as a per-project config concern** (the original ask, then reverted). Rejected:
  the whole point raised in review is that this is not heraut-repo-specific — every heraut
  user benefits from the same default, and requiring each one to copy the same
  `rendering.trailers` entry is the boilerplate `commits.types`/`commits.scopes` already
  solved for an analogous case.
- **A broader built-in set covering several common trailers.** Rejected as speculative
  (YAGNI) — see Decision above.
- **Opt-in via a config flag instead of a default-on built-in with an opt-out.** Rejected for
  consistency with the existing `commits.types`/`commits.scopes` precedent (default-on,
  `remove`/override to opt out) and because opt-in would silently under-deliver the exact
  value proposition (projects that don't know to ask for it are the ones who'd benefit most).
