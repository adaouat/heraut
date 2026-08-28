# Changelog/Release-Notes Title & Subtitle Blocks — Design

- **Date**: 2026-08-28
- **Status**: Approved (brainstorm) — pending implementation plan
- **Scope**: rename the native generator's `header`/`release-notes` template block keys for
  clarity, and add new `title`/`subtitle` document-level blocks — closing the one customization
  gap left by [ADR-0037](../adr/0037-native-template-api.md). New ADR-0048 (supersedes the block
  set table in ADR-0037).

---

## Problem

[ADR-0037](../adr/0037-native-template-api.md) exposed the native generator's template blocks as
public config (`rendering.templates.<block>`), but one piece of output was never routed through
that API: the changelog's document-level title. `internal/generators/native/render.go` hardcodes

```go
const changelogHeader = "# Changelog\n\n"
```

which `buildAllSections` (`generator.go`) prepends once per file, on bootstrap or `--regenerate`
only. It is not reachable from any template block — not `rendering.templates.header` (that block
is the **per-release** version heading, e.g. `## [1.2.3] - 2026-08-28`, rendered once per section)
and not a `<driver>.template` file either, since the file is parsed on top of the same block set.

This surfaced from a user question while writing
[`docs/guides/template-customization.md`](../guides/template-customization.md): the existing
`header` block name reads as "the document header," but it is actually scoped to one release —
confusing enough that it hid the real gap (no document-level title override at all) until asked
about directly.

## Decision

1. **Rename** the block that fires once per rendered release from `header` to `release_header` —
   the name now says what varies (fires again for every release/section) instead of implying
   document scope.
2. **Add** `title` and `subtitle` — new blocks that fire exactly **once per document**, not once
   per release. Available on **both** drivers (changelog and release notes), for API symmetry,
   even though release notes has no equivalent hardcoded constant to replace — it simply defaults
   both to empty, same as `header` does there today.
3. **Rename** the `release-notes` root block key to `release_notes`, so every block key in
   `rendering.templates` uses a consistent `snake_case` separator — the only survivor of the old
   hyphenated form. `changelog` (the other root) was already a single word and is unaffected.

No other block name changes. `group`, `commit`, `ticket`, `contributor`, `contributors`, `stats`,
`footer` are untouched.

### Full renamed/new block set

| Key | Was | Context (`.`) | Fires |
|---|---|---|---|
| `title` | *(new)* | `.Heraut` only | once per document |
| `subtitle` | *(new)* | `.Heraut` only | once per document |
| `release_header` | `header` | `Release` | once per rendered release |
| `group` | unchanged | `Group` | once per commit-type group |
| `commit` | unchanged | `Commit` | once per commit |
| `ticket` | unchanged | `Link` | once per matched ticket in a commit |
| `contributor` | unchanged | `Contributor` | once per first-time contributor |
| `contributors` | unchanged | root `Release` | once (release notes only) |
| `stats` | unchanged | root `Release` | once (release notes only) |
| `footer` | unchanged | root `Release` | once per document |
| `changelog` | unchanged | root `Release` | once per rendered release (the per-section root) |
| `release_notes` | `release-notes` | root `Release` | once |

### Defaults

| Block | Changelog built-in | Release-notes built-in |
|---|---|---|
| `title` | `"# Changelog\n"` — **byte-identical to today's output** when unset | `""` (matches today's empty `header`) |
| `subtitle` | `""` — nothing renders, no stray blank line | `""` |

`subtitle`'s empty built-in follows the same pattern `footer` already uses
(`{{define "footer"}}{{end}}` in `blocks.tmpl`) — an unset optional block contributes zero bytes,
not an empty line.

---

## Rendering scope: the one real wrinkle

Every existing block executes as part of rendering **one release** — `native.Generate(tag, ...)`
builds a `Release` and executes the driver's root template (`changelog` or `release_notes`)
against it exactly once per call. `title`/`subtitle` cannot follow that path for the **changelog**
driver: `buildAllSections` calls `renderRelease` once **per section** (newest release, then every
historical tag), concatenates the results, and only *then* prepends the document title — today
that's the hardcoded `changelogHeader` constant; going forward it must be the `title`/`subtitle`
blocks rendered exactly once, not once per section.

**Design:** `title`/`subtitle` execute against a minimal context — `.Heraut` only (`tplHeraut`:
`.Version` `.URL` `.GeneratedAt`), the same struct already reachable from `header`/`footer` today
— not a full `Release`, since they are not tied to any single release. `buildAllSections` renders
them once, via the same `text/template` engine and `EffectiveTemplates` override map used
everywhere else, and prepends the result in place of the current `changelogHeader + "\n\n"`
concatenation.

This only changes *when* `buildAllSections` runs — unchanged from today:

- **Bootstrap** (missing/empty file) — renders `title`+`subtitle` once, plus every section.
- **`--regenerate` / `--regenerate-changelog`** — same, full rebuild.
- **Ordinary incremental append** (`generateIncremental`'s splice path) — untouched. It only
  splices a new section into the existing file's anchor list and never re-renders the preamble,
  exactly as today. A hand-edited or previously-generated title/subtitle survives incremental runs
  unchanged, same as the current hardcoded `# Changelog` line does.

For **release notes**, there is no wrinkle: `native.Generate` renders exactly one release per
call, so `title`/`subtitle` simply join `header`/`footer` in the normal per-release
`release_notes` root template, executed with the same `Release` context already available to
every other release-notes block (though their own built-in bodies still only reach `.Heraut`, for
symmetry with the changelog side — see below).

**Context consistency across drivers.** Even though release notes' root template *could* pass
`title`/`subtitle` the full `Release`, this design keeps their context to `.Heraut` on **both**
drivers, not just changelog. Rationale: a title/subtitle is conceptually about the document
("Foo Corp Changelog", "All notable changes to this project"), not about any one release's
data — allowing `.Version`/`.Groups`/etc. there would create an asymmetry where the same block key
means something different depending on which driver renders it. `.Heraut.Version` (heraut's own
CLI version) remains reachable for a credit-line style subtitle if wanted; a project's own version
number is not meaningful here since `title` is document-scoped, not release-scoped, on the
changelog side.

---

## Config surface (unchanged mechanics)

No new config concept — `title`/`subtitle` are ordinary entries in the existing
`rendering.templates` map, subject to the same four-layer precedence chain as every other block
(unchanged from ADR-0037):

```
built-in → rendering.templates (global) → <driver>.rendering.templates (per-driver)
  → environments.<env>.<driver>.rendering.templates (per-env) → <driver>.template file
```

```yaml
rendering:
  templates:
    release_header: "## [{{ .Version }}] - {{ date \"2006-01-02\" .Date }}"

changelog:
  rendering:
    templates:
      title: "# MyApp Changelog"
      subtitle: "All notable changes to this project, by version."

release:
  notes:
    rendering:
      templates:
        title: ""   # explicit no-op — release notes already defaults to empty
```

A `<driver>.template` file may `{{define "title"}}...{{end}}` / `{{define "subtitle"}}...{{end}}`
exactly like any other block, parsed last in the precedence chain as today.

---

## Errors & validation

Unchanged mechanism, updated set. `internal/config/validator.go`'s `validTemplateBlocks` map and
`validTemplateBlocksHint` string gain `title`, `subtitle`, `release_header` and drop `header`;
`release-notes` becomes `release_notes`. `schema.json`'s `templates` object properties are updated
to match (renamed `header` → `release_header` and `release-notes` → `release_notes`, two new
`title`/`subtitle` properties added). Both inline snippets and a `template` file continue to be
parsed at config-load time, per block key — unchanged validation flow, just a different key list.

An old config still setting `rendering.templates.header` after this ships gets today's existing
"unknown template block" config error (naming `release_header` in the hint) — not a silent
no-op. This is a **breaking rename**, consistent with the project's pre-v1.0 stance (SemVer 0.x —
see `.claude/rules/workflow.md` § Branching: "during the build phase... the roadmap is the
protection, not branches"). No deprecated-alias shim.

---

## Migration impact

- **Changelog output is byte-identical by default.** `title`'s changelog built-in
  (`"# Changelog\n"`) reproduces today's `changelogHeader` constant exactly — nobody's rendered
  `CHANGELOG.md` changes unless they explicitly set `title`/`subtitle`, or already had
  `rendering.templates.header` set (which now needs renaming to `release_header`, or the run
  fails config validation with an actionable error).
- **Any config using `rendering.templates.header` or `.release-notes` must rename** to
  `release_header` / `release_notes` respectively. No automatic migration — config validation
  catches it immediately with the block-key hint.
- `docs/guides/template-customization.md` (published this session) needs its block table, examples,
  and precedence-chain prose updated for the new names plus the two new rows.

---

## Testing

- Golden snapshots: unaffected for default output (title's built-in reproduces
  `changelogHeader` byte-for-byte); a new snapshot covering `title`+`subtitle` set together.
- `internal/config/validator_test.go`-equivalent: `release_header`/`release_notes` accepted,
  `header`/`release-notes` rejected with the updated hint; `title`/`subtitle` accepted and
  parse-validated like any other block.
- `internal/generators/native`: a test that `title`/`subtitle` render exactly once across a
  multi-section `buildAllSections` bootstrap (not once per section); a test that an ordinary
  incremental splice run leaves an existing title/subtitle untouched; a test that
  `--regenerate` re-renders them; a release-notes test confirming `title`/`subtitle` execute with
  `.Heraut`-only context even though the driver's root template has a full `Release` available.
- `app/pipeline.go`'s `effectiveTemplates`/`withEnvDerivations` merge logic needs no test changes
  — it already treats `EffectiveTemplates` as an opaque `map[string]string`, agnostic to which
  keys are valid.

## Out of scope

- Per-forge title/subtitle scoping — same "no forge axis" limitation the rest of the template API
  has (route forge-specific wording through an environment instead).
- A backward-compatible alias for the renamed keys — rejected per Errors & validation above.
- Any change to the anchor (`<!-- heraut-release: vX.Y.Z -->`) comment or its non-overridability —
  unrelated to this change, still emitted by the assembly layer, still independent of `title`.
