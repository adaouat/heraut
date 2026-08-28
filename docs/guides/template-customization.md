# Guide: Customizing changelog & release-notes templates

heraut's `native` generator (its sole generator, ADR-0045) renders Markdown from built-in
Go `text/template`s. Those templates are **user-customizable** (ADR-0037): you can reformat
individual pieces (a commit line, a ticket link, a heading) without touching anything else,
or replace the whole document structure. This guide covers every knob, where each one can be
set, and how they combine.

This is a companion to
[Spec 05 § User-customizable templates](../specs/05-generators-and-platforms.md#user-customizable-templates-adr-0037-adr-0048)
(the terse behavioral contract), [ADR-0037](../adr/0037-native-template-api.md) (the original
design rationale), and [ADR-0048](../adr/0048-changelog-title-subtitle-blocks.md) (the
`release_header`/`release_notes` rename and the `title`/`subtitle` blocks). This guide is the
worked-example version.

**Applies to `native` only.** Since ADR-0045, `native` is heraut's sole generator, so there
is no `generator:` key to gate this feature on — every `.heraut.yml` gets it.

---

## Two ways to customize

| Entry point | Use for | Where it lives |
|---|---|---|
| **Inline block override** | Reformat one piece — a commit line, a ticket link, a heading | `rendering.templates.<block>` — a single YAML string |
| **Full template file** | Whole-document control — reorder sections, add custom prose, redefine the root | `<driver>.template: path/to/file.tmpl` |

Both feed the **same block set and data contract** — a file override and an inline override
of the same block key do the same job, just with different ergonomics (a `.tmpl` file gets
real editor syntax highlighting and no YAML string-escaping; an inline snippet needs no
separate file for a one-line change).

```yaml
rendering:
  templates:
    commit: "- {{ upperFirst .Description }} ({{ .ShortHash }})"
    contributor: "* @{{ .Author.Username }} — first contribution 🎉"

changelog:
  template: .config/heraut/changelog.tmpl   # optional full template file
  rendering:
    templates:
      title: "# Changelog"
      subtitle: "All notable changes."
```

---

## Overridable blocks

Any key under `rendering.templates` other than the ones below is a **config error** at load
time — `schema.json` enumerates the same set for editor autocompletion, so a typo is caught
before a run, not silently ignored.

| Key | Context (data passed in) | Renders | Built-in default |
|---|---|---|---|
| `title` | bare `.Heraut` (`.Version` `.URL` `.GeneratedAt` — **not** `.Heraut.Version`) | Document-level title, once per document | Changelog: `# Changelog`. Release-notes default: empty. |
| `subtitle` | bare `.Heraut` (same as `title`) | Document-level subtitle rendered under `title`, once per document | Empty on both drivers |
| `release_header` | root `Release` | One release's version heading, once per rendered release | Changelog: `## [version](compare-url) - date`. Release-notes default: empty. |
| `group` | `Group` | One commit-type group's heading | `{{ .HeadingPrefix }} {{ .Name }}` (e.g. `### 🚀 Features`) |
| `commit` | `Commit` | One commit's line — description, scope, breaking marker, hash link, `by @author`, PR/MR ref, ticket links | See [built-in `commit`](#worked-examples) below |
| `ticket` | `Link` (one `.Tickets` entry) | One matched ticket reference *within* a commit line | `([TICKET](url))` |
| `contributor` | `Contributor` | One "New Contributors" entry | `* @user made their first contribution in [#42](url)` |
| `contributors` | root `Release` | The whole "New Contributors ❤️" section (release notes only) | Heading + a loop over `contributor` |
| `stats` | root `Release` | The Commit Statistics block (release notes only) | Commit/conventional counts, timespan, linked tickets |
| `footer` | root `Release` | Trailing content after the body | Empty |
| `changelog` | root `Release` | The whole changelog-section document (whole-document control) | Composes `release_header` → `group` → `commit` |
| `release_notes` | root `Release` | The whole release-notes document (whole-document control) | Composes `release_header` → `group` → `commit` (+body/footers) → `contributors` → `stats` → `footer` |

**`title`/`subtitle` fire once per document, not once per release — the one exception to every
other block's scope.** Every other block above renders again for each release/section a run
touches; `title`/`subtitle` render exactly once regardless of how many sections a changelog
build walks. They also reach heraut's own metadata differently: bare `.Version` `.URL`
`.GeneratedAt` directly on `.` (their context *is* `.Heraut`), **not** `.Heraut.Version` the way
`footer`/`release_header`/every other block does, since their root isn't a `Release` at all.

**`ticket` is a nested block.** `commit` calls `{{ template "ticket" . }}` once per matched
`commits.tickets` pattern. Override just `ticket` to restyle ticket links (e.g. add an emoji)
without restating the entire commit-line template.

**Two commit renderings, one block.** The changelog renders a one-line `commit`. The
release-notes root renders that *same* `commit` block, then — only when the commit has a body
or footers — appends them indented *around* it (not inside it). So overriding `commit`
reformats the line in both outputs; the body/footer indentation stays built-in.

**`contributors` / `stats` are release-notes-only.** Both blocks are invoked only from the
`release_notes` root template. Setting `changelog.rendering.templates.contributors` (or
`stats`) has no effect — the changelog's own root never calls them.

---

## Where to set overrides — and how they combine

Overrides layer at four scopes. **Precedence, lowest → highest:**

```
built-in
  → rendering.templates                              (global)
    → <driver>.rendering.templates                    (per-driver: changelog vs release.notes)
      → environments.<env>.<driver>.rendering.templates (per-env)
        → <driver>.template file                       (parsed last — wins outright)
```

Merging is **per block key**: a level that doesn't set a given key falls through to the level
below it. `rendering.templates.commit` set globally still applies even if a driver or env
overrides only `release_header`.

`title`/`subtitle` follow this exact same four-layer chain — they're the only blocks that fire
once per document instead of once per rendered release, but where you set the override works
identically to every other block.

| Scope | Config path | Applies to |
|---|---|---|
| Global | `rendering.templates.<block>` | Both changelog and release notes |
| Per driver | `changelog.rendering.templates.<block>` / `release.notes.rendering.templates.<block>` | Just that output — reformat the changelog without touching the release page, or vice versa |
| Per env | `environments.<env>.changelog.rendering.templates.<block>` / `environments.<env>.release.notes.rendering.templates.<block>` | Just that environment's runs |
| Full file | `changelog.template: path.tmpl` / `release.notes.template: path.tmpl` (also settable per-env) | Whatever the file redefines — parsed on top of everything above |

**No per-forge axis.** Templates aren't scoped by publish target (`forges:` / `release.targets`).
A release published to multiple forges regenerates release notes once per platform so
commit/PR links resolve to the right host — but it's the *same* template each time, just fed
different link data. If you need forge-specific wording, route it through an environment
instead (pair one env with one forge in your config) — there is no `forges[].rendering` key.

### All four layers together

```yaml
# Global default — applies everywhere unless a more specific layer overrides it.
rendering:
  templates:
    commit: "- {{ .Description }} ({{ .ShortHash }})"

changelog:
  output: CHANGELOG.md
  rendering:
    templates:
      # Changelog-only: give the changelog a plainer release heading than release notes get.
      release_header: "## {{ .Version }} — {{ date \"2006-01-02\" .Date }}"

release:
  notes:
    rendering:
      templates:
        # Release-notes-only: a friendlier footer on the GitHub/GitLab release page.
        footer: "\n_Full diff: {{ .CompareURL }}_"

environments:
  prod:
    changelog:
      rendering:
        templates:
          # prod changelog entries get a rocket; other envs keep the global "- description" form.
          commit: "🚀 {{ .Description }} ({{ .ShortHash }})"
```

Running against `prod`, a commit line resolves `commit` from the env layer (`🚀 …`); running
against any other env (or none), it falls through to the global `- description (…)`. The
changelog `release_header` override applies regardless of env; the release-notes `footer`
override applies only to release notes, regardless of env.

---

## Full template file mode

`<driver>.template: path/to/file.tmpl` points at a real `.tmpl` file, parsed **on top of**
the built-ins (and any inline overrides below it in precedence). It can redefine:

- Any individual block (same names as above) — same effect as an inline override, but authored
  with real editor tooling and no YAML string-escaping.
- The document **root** (`changelog` or `release_notes`) — full control over section order,
  extra prose, or structure the built-in root doesn't support.

```yaml
changelog:
  template: .config/heraut/changelog.tmpl
```

```gotemplate
{{/* .config/heraut/changelog.tmpl */}}
{{define "release_header"}}## {{ .Version }} ({{ date "2006-01-02" .Date }}){{end}}

{{define "changelog"}}{{ template "release_header" . }}

{{range .Groups}}{{ template "group" . }}
{{range .Commits}}{{ template "commit" . }}
{{end}}
{{end}}
---
Generated by heraut {{ .Heraut.Version }}
{{end}}
```

A `template:` file is parsed **last** — it wins outright for whatever blocks it defines, even
over a per-env inline override. Blocks it doesn't touch keep whatever the layers below it
resolved to.

---

## Data contract

The template root is a `Release`. Everything below is reachable from any block via its
context argument (a block invoked with `.` receives the type in its "Context" column above).

**`title`/`subtitle` are the one exception to this contract.** Their root is `.Heraut` itself
(`tplHeraut`), not a `Release` — they're not tied to any single release, so there's no `Release`
to root them on. Write `{{ .Version }}`, not `{{ .Heraut.Version }}`, inside a `title` or
`subtitle` override.

**`Release`** (root)
`.Version` `.Tag` `.PreviousTag` `.CompareURL` `.Date` `.Groups []Group` `.Contributors
[]Contributor` `.Stats` `.Heraut` `.HeadingPrefix` (leading `#`s sized by
`commits.types_heading_level`, for the contributors/stats headings)

**`Group`**
`.Name` `.HeadingPrefix` `.Commits []Commit`

**`Commit`**
`.Type` `.Scope` `.Breaking` `.Description` (cleaned, upper-cased subject) `.Subject` (raw
commit subject line) `.Body` `.Hash` `.ShortHash` `.CommitURL` `.Date` `.Author` `.PR`
(nil when no associated PR/MR) `.Tickets []Link` `.Footers []Footer`

**`Author`**
`.Name` `.Email` `.Username` (platform handle; empty offline or when unresolvable)

**`PR`** — all fields remote-only (empty offline; gated by `commits.enrichment_policy`)
`.Number` `.URL` `.Title` `.Ref` (`"#42"` / `"!42"`, precomputed per platform) `.Labels
[]string` `.Author` `.CreatedAt` `.MergedAt` `.MergedBy` `.Approvers []Author`
(best-effort — populated on GitHub + Azure DevOps, empty on GitLab)

**`Contributor`**
`.Author` `.PR` (their first PR in this release; nil offline)

**`Link`** (a `.Tickets` entry, what the `ticket` block receives)
`.Text` (matched ticket text) `.Href` (resolved URL)

**`Footer`** (a git trailer parsed from the commit body)
`.Token` `.Value`

**`Stats`**
`.CommitCount` `.ConventionalCount` `.TimespanDays` `.DaysSincePrev` `.HasDaysSincePrev`
`.Tickets []StatTicket` (`.Text` `.Href` `.Count`)

**`Heraut`** (reachable as `.Heraut` from `release_header`/`footer`/root blocks; reachable
directly as `.` — bare `.Version` `.URL` `.GeneratedAt` — from `title`/`subtitle`, see above)
`.Version` `.URL` `.GeneratedAt`

Field names are the **experimental-in-v1** public API: additive changes (new fields) are
free; renames or removals would be called out explicitly. `.Heraut.GeneratedAt` is
wall-clock — the built-in templates never render it, keeping golden snapshots deterministic;
a custom template that renders it is opting into non-deterministic output knowingly.

---

## Template funcs

A small, safe set — no OS, filesystem, or network access:

| Func | Signature | Example |
|---|---|---|
| `upperFirst` | `(s string) string` | `{{ upperFirst .Description }}` |
| `date` | `(layout string, t time.Time) string` | `{{ date "2006-01-02" .Date }}` |
| `join` | `(sep string, s []string) string` | `{{ join ", " .PR.Labels }}` |
| `list` | `(items ...string) []string` | `{{ join "/" (list .Type .Scope) }}` |
| `indent` | `(n int, s string) string` | `{{ indent 4 .Body }}` |
| `trim` | `(s string) string` | `{{ trim .Body }}` |

---

## Validation & errors

Every inline snippet and every `.tmpl` file is parsed at **config-load time**, not at render
time — a bad snippet fails `heraut check config` (and any other command) before anything is
generated:

- **Unknown block key** — `unknown template block "bogus"`, with a hint listing the valid set.
- **Snippet that fails to parse** — `invalid template: <parse error>`.
- **`template:` file that's missing or fails to parse** — fails the same way, naming the path.

There is never a silent fallback to the built-in on a broken override — you always get an
actionable error naming the offending block key or file path.

---

## Worked examples

**Built-in `commit` block, for reference** (what you're overriding):

```gotemplate
- {{ if .Scope }}*({{ .Scope }})* {{ end }}{{ if .Breaking }}[**breaking**] {{ end }}{{ .Description }} - {{ if .CommitURL }}([{{ .ShortHash }}]({{ .CommitURL }})){{ else }}{{ .ShortHash }}{{ end }}{{ if .Author.Username }} by @{{ .Author.Username }}{{ end }}{{ if .PR }}{{ if .PR.Number }} in [{{ .PR.Ref }}]({{ .PR.URL }}){{ end }}{{ end }}{{ range .Tickets }} {{ template "ticket" . }}{{ end }}
```

**Drop the hash link, keep it minimal:**

```yaml
rendering:
  templates:
    commit: "- {{ .Description }}{{ if .Author.Username }} (@{{ .Author.Username }}){{ end }}"
```

**Emoji-prefixed ticket links:**

```yaml
rendering:
  templates:
    ticket: "🎫 [{{ .Text }}]({{ .Href }})"
```

**Custom footer crediting heraut, release notes only:**

```yaml
release:
  notes:
    rendering:
      templates:
        footer: "\n_Generated by [heraut]({{ .Heraut.URL }}) {{ .Heraut.Version }}._"
```

**Plainer release heading, no compare link:**

```yaml
changelog:
  rendering:
    templates:
      release_header: "## {{ .Version }} - {{ date \"2006-01-02\" .Date }}"
```

**Custom document title + subtitle:**

```yaml
changelog:
  rendering:
    templates:
      title: "# MyApp Changelog (generated by heraut {{ .Version }})"
      subtitle: "All notable changes, by version."
```

Both fire once, before the first `release_header`, regardless of how many release sections the
changelog contains. Note the context: `{{ .Version }}` here resolves to **heraut's own running
version** (the CLI that generated the file), not any particular release's — `title`/`subtitle`
execute against `.Heraut` directly, not a `Release`. Compare `release_header` above, where
`{{ .Version }}` means the release being rendered.

---

## Gotchas

- **`native` only.** There's no `generator:` key to check — every project gets this API.
- **`title`/`subtitle` address `.Heraut` differently from every other block.** They execute
  against `.Heraut` directly — write `{{ .Version }}`, not `{{ .Heraut.Version }}`. Every other
  block (`footer`, `release_header`, …) reaches the same data through `.Heraut.Version` because
  its own root is a `Release`. Copying a `{{ .Heraut.Version }}` snippet from `footer` into
  `title` parses fine at config-load time (snippets are only parse-checked, not executed) but
  fails at render time — there's no `.Heraut` field on `.Heraut` itself.
- **`contributors`/`stats` never fire from the `changelog` driver** — they're release-notes-only
  blocks; overriding them under `changelog.rendering.templates` is accepted but has no effect.
- **No per-forge scoping** — route forge-specific formatting through an environment instead.
- **A `template:` file wins outright**, even over a more specific per-env inline override, for
  whatever blocks it defines — it's parsed last in the precedence chain. **Exception:** nulling
  a built-in with an explicit `title: ""` / `subtitle: ""` snippet override (via
  `rendering.templates`/per-driver/per-env) currently wins even over a `<driver>.template` file
  that redefines the same block with non-empty content — the one case where the file does not
  win outright.
- **`title`/`subtitle` live outside the document root.** They're rendered by Go code (once per
  document, via `renderPreamble`) *before* the `changelog`/`release_notes` root template runs —
  so a custom `<driver>.template` file that fully redefines the root cannot suppress or
  reposition them. If that custom root also invokes `{{ template "title" . }}` itself, it
  executes there against a full `Release` (not the bare `.Heraut` context `renderPreamble`
  uses) — same block name, different context, potentially different output, rendered twice.
- **Rendering `.Heraut.GeneratedAt`** makes output non-deterministic run-to-run; the built-ins
  deliberately never do.
