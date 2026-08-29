# Uniform Empty/Whitespace Block-Override Handling — Design

- **Date**: 2026-08-29
- **Status**: Approved (brainstorm) — pending implementation
- **Scope**: fix the two known gaps in Phase 30's `execPreambleBlock` workaround (T250, Phase 31 in
  `docs/tasks/roadmap.md`), generalized to **every** `rendering.templates` block, not just
  `title`/`subtitle`.

---

## Problem

`internal/generators/native/templateset.go`'s `buildTemplateSet` layers three sources onto one
`*template.Template` (`ts`): built-in blocks, then each `rendering.templates` snippet (each
registered via `ts.Parse(fmt.Sprintf("{{ define %q }}%s{{ end }}", k, snippets[k]))`), then an
optional `<driver>.template` file — later `Parse` calls normally win. But Go's `text/template`
documents a special case on `Parse`:

> A template definition with a body containing only white space and comments is considered empty
> and will not replace an existing template's body.

So `rendering.templates.title: ""` (or `"   "`) can never reach the engine as a real override —
the built-in stays. Phase 30 (ADR-0048) needed `title`/`subtitle` to be nullable and added
`execPreambleBlock`, a Go-level short-circuit that intercepts `snippets[name] == ""` *before*
calling into the template engine at all:

```go
func execPreambleBlock(name, rootTmpl string, snippets map[string]string, templateFile string, heraut tplHeraut) (string, error) {
	if v, ok := snippets[name]; ok && v == "" {
		return "", nil
	}
	return execBlocks(name, rootTmpl, snippets, templateFile, heraut)
}
```

This has two known gaps (found in Phase 30's final review, deliberately left unfixed):

1. **Precedence inversion.** The short-circuit fires before `templateFile` is ever consulted, so
   `title: ""` wins even over a `<driver>.template` file that redefines `title` with real content —
   inverting the documented "a template file always wins outright" rule.
2. **Whitespace blindness.** The short-circuit matches only the exact string `""`. A whitespace-only
   override (`"   "`) hits the underlying Go quirk with no guard at all — silently ignored, no
   error.

And a third problem, not previously a "gap" so much as a scope limit: `execPreambleBlock` only
exists for `title`/`subtitle`. `internal/config/validator.go`'s `validTemplateBlocks` already
accepts overrides for all twelve block names (`title`, `subtitle`, `release_header`, `footer`,
`group`, `commit`, `ticket`, `contributor`, `contributors`, `stats`, `changelog`,
`release_notes`) — but only `title`/`subtitle` can actually be nulled today. Setting
`rendering.templates.footer: ""` passes validation and silently does nothing.

## Investigation

The original assumption going into this design spike (recorded in the roadmap task) was that
`title`/`subtitle` are structurally special — they're the only blocks Go code calls directly
(`renderPreamble` → `execPreambleBlock`), which is what lets a Go-level short-circuit intercept
them. Every other block is invoked via a nested `{{ template "x" . }}` call *inside* another
block's body (e.g. `changelog.tmpl`'s root calls `{{ template "footer" . }}`), with no Go call site
to intercept — implying a uniform fix would need to either guard every `{{ template }}` call site
or restructure block execution entirely.

That assumption turned out to be irrelevant, once the actual mechanism was tested rather than
inferred from the doc comment. `execBlocks` builds `ts` fresh per call
(`buildTemplateSet(blocksTmpl+"\n"+rootTmpl, snippets, templateFile)`) and then calls
`ts.ExecuteTemplate(&sb, rootName, data)` for exactly one `rootName` — but every block registered
into that `ts`, whether it's the top-level `rootName` or one referenced by a nested `{{ template
"x" . }}` inside another block's body, resolves against the *same* merged template set at execution
time. There is no separate resolution path for "the name I called `ExecuteTemplate` with" versus
"a name some other block references" — Go looks both up in `ts` by name, using whatever definition
survived the Parse layering. So a fix that makes the *snippet-registration step itself* honor an
empty override works for every block, nested or top-level, with no restructuring required. The only
open question was how to make that one step actually work, given Go's empty-body special case.

Three ways to defeat that special case were tested empirically (`text/template` on the project's Go
toolchain), because the stdlib docs describe the special case only on `Parse` and are silent on
whether other entry points share it:

| Attempt | Result |
|---|---|
| `t.Parse(`{{define "foo"}}{{end}}`)` (empty body) over an existing non-empty `"foo"` | Old body wins — confirms the documented quirk |
| `t.Parse(`{{define "foo"}}   {{end}}`)` (whitespace-only body) | Old body still wins — quirk applies to whitespace too, not just fully-empty |
| Build a standalone empty/whitespace tree via `template.New("foo").Parse(...)` and call `t.AddParseTree("foo", tree)` instead of `t.Parse` | **Old body still wins.** `AddParseTree`'s doc reads as an unconditional replace ("the existing definition is replaced"), but empirically it is not — the empty/whitespace-tree special case applies here too. This rules out "swap `Parse` for `AddParseTree`" as a general escape hatch. |
| `t.Parse(`{{define "foo"}}{{if false}}{{end}}{{end}}`)` — a body containing a real (if inert) **action node**, not just whitespace/comments | **Override succeeds — renders as `""`.** An `{{if false}}{{end}}` node is neither whitespace nor a comment, so Go's "is this body empty" check does not treat it as empty, and the redefinition replaces the built-in normally. Its own execution produces no output. |
| Same, but with a comment (`{{/* null */}}`) instead of an action | Old body still wins — the doc's "comments count as empty too" is accurate; comments don't defeat the check. |

The fourth row is the mechanism: **an override that should render as nothing must be encoded as a
real, executable, output-free node — never as literal empty or whitespace text** — and this needs
to happen exactly once, at the point every snippet is already registered into `ts`
(`buildTemplateSet`), not per-block-name in Go code.

## Decision

**Move the null-handling from `execPreambleBlock` (Go-level, title/subtitle-only) into
`buildTemplateSet` (template-level, every block), by changing how an empty/whitespace snippet gets
encoded into its `{{define}}` body.**

In `buildTemplateSet`'s snippet loop (`internal/generators/native/templateset.go`), for each key
`k`:

```go
body := snippets[k]
if strings.TrimSpace(body) == "" {
	body = "{{if false}}{{end}}"
}
if _, err := ts.Parse(fmt.Sprintf("{{ define %q }}%s{{ end }}", k, body)); err != nil {
	return nil, fmt.Errorf("parsing rendering.templates.%s: %w", k, err)
}
```

Consequences, all falling out of this one change with no further plumbing:

- **Generalizes to every block for free.** Any of the twelve `validTemplateBlocks` names —
  `footer`, `release_header`, `group`, `commit`, `ticket`, `contributor`, `contributors`, `stats`,
  `changelog`, `release_notes`, in addition to `title`/`subtitle` — can now be nulled with
  `rendering.templates.<name>: ""`, because the fix lives in the one code path all of them already
  share (per the Investigation section: nested `{{ template "x" . }}` calls resolve against the
  same `ts`).
- **Fixes gap 2 (whitespace) directly.** `strings.TrimSpace(body) == ""` catches `"   "` the same
  as `""` — no separate whitespace-specific guard needed.
- **Fixes gap 1 (precedence) as a side effect, not a special case.** The bug was never really about
  detecting whether `templateFile` redefines the block — it was that `execPreambleBlock` skipped
  the layering order (snippets → file) entirely by returning before `buildTemplateSet` ran. Once
  null-handling moves *inside* that layering (as one more `ts.Parse` call, still happening before
  the file is parsed, in the existing snippets-then-file order), a `<driver>.template` file that
  redefines the same block parses *after* and naturally wins — restoring the same "file wins
  outright" rule every other override already follows, with no new precedence-detection logic
  required.
- **`execPreambleBlock` is retired.** `renderPreamble` calls `execBlocks("title", ...)` /
  `execBlocks("subtitle", ...)` directly, same as every other block; the wrapper and its Go-level
  short-circuit are deleted.

### Per-item blocks (`group`, `commit`, `ticket`, `contributor`)

These fire once per loop iteration (once per commit-type group, once per commit, once per matched
ticket, once per first-time contributor) rather than once per document or once per release. The
fix above applies to them identically — no special case, no restriction. Nulling `commit: ""`
blanks every commit line inside every group (the group headings and surrounding structure still
render); nulling `ticket: ""` suppresses ticket links inline without touching the commit line
itself. This is a deliberate consequence of "uniform for every block," not an oversight: the design
spike considered gating null-override to document/release-level blocks only, but that reintroduces
a **second** special case (which blocks are nullable) on top of the one just removed, for a
restriction nobody asked for. If an actual footgun report surfaces in practice, add config
validation to reject it then, against a real complaint rather than a hypothetical one.

### `changelog` / `release_notes` (root blocks)

Also nullable under the same mechanism, with the obvious consequence: nulling the document's own
root block produces an empty document body. No special-casing here either, for the same reason as
per-item blocks above.

## Non-goals

- No new config surface — `rendering.templates.<block>: ""` already validates today
  (`validTemplateBlocks` already lists all twelve names); this fixes what it does, not what it
  accepts.
- No ADR. This is a bugfix restoring already-documented behavior (ADR-0037's "a template file
  always wins outright" rule, ADR-0048's block set), not a new decision — `docs/guides/
  template-customization.md`'s Gotchas section entries for both gaps are removed/corrected as part
  of the fix, no new ADR needed.
- No change to `AddParseTree` usage anywhere else in the codebase — it was evaluated and rejected
  as the mechanism (see Investigation table), not adopted.

## Implementation sketch

Single task, not a multi-task epic — smaller than Phase 30 itself. Expected files:

- `internal/generators/native/templateset.go` — the `buildTemplateSet` change above.
- `internal/generators/native/render.go` — delete `execPreambleBlock`; `renderPreamble` calls
  `execBlocks` directly for `title`/`subtitle`.
- Tests (table-driven, `internal/generators/native`):
  - Null override wins for a **non-preamble** block (e.g. `footer: ""` now suppresses the built-in
    footer) — proves the generalization.
  - Whitespace-only override (`"   "`) nulls a block — proves gap 2 fixed.
  - A `<driver>.template` file redefining a block wins over a null snippet override for that same
    block — proves gap 1 fixed (precedence restored).
  - Existing `title`/`subtitle` null-override tests (from Phase 30) continue to pass unchanged
    through the new path — proves no regression from retiring `execPreambleBlock`.
- `docs/guides/template-customization.md` — remove the two Gotchas entries describing the gaps as
  known limitations (they're fixed); keep the `.Heraut`-addressing-difference and
  `contributors`/`stats`-changelog-driver-no-op Gotchas, which are unrelated and still true.

**Files (expected):** `internal/generators/native/{templateset,render}.go` + tests,
`docs/guides/template-customization.md`. **Scope:** S. **Dependencies:** none.
