# `heraut notes` — release notes for a tag, for any consumer

- **Status**: Draft (awaiting review)
- **Date**: 2026-10-13
- **Author**: bchatard (with Claude)
- **Roadmap**: not yet on `docs/tasks/roadmap.md`; the plan adds it
- **Related ADRs**: 0032/0033 (native generator), 0037/0048 (template API), 0038 (changelog
  structure), 0043 (forge enrichment), 0064/0065 (tag order and history-aware bounds)
- **Origin**: a mobile-project workflow. Each release is cut with `heraut release` (or
  `heraut changelog`), and the store submission (App Store, Play Store) needs the release text
  separately, in a shape that differs from the forge release body.

---

## Problem

heraut renders release notes in exactly two places: the `CHANGELOG.md` section written by
`heraut changelog` / `heraut release`, and the body sent to the forge release. Neither can be
printed on its own. To feed a store, the user would have to parse `CHANGELOG.md`, or pass the forge
release through its API, and either way the text has the forge-oriented layout (commit hashes,
authors, compare links).

## Decisions

Settled with the user during brainstorming:

1. **A read-only command, `heraut notes <tag>`.** No commit, tag, push, publish or hook.
2. **The tag's previous release is the lower bound, automatically.** It is the bound the release
   itself uses: SemVer finals skip pre-releases, maintenance lines and version branches stay on
   their own line, per-env uses `--env`. `--from <tag>` overrides the bound only, for example to span
   every `rc` build of a version into one text.
3. **Own template, configured once, with a flag override.** A new top-level `notes:` block carries
   a `template:` file and/or inline `rendering.templates`; `--template <file>` overrides it for one
   run. Same engine, data model and function set as the changelog and release templates.
4. **Default rendering is the changelog's groups and commit lines, without the release header and
   without the hash, author and PR reference.** Without any `notes` configuration the command prints
   the groups of the range as `CHANGELOG.md` would, one line per commit, but a store has no use for
   `## [v7.8.0](compare) - date` or for `([a1b2c3d](…)) by @alice in [#41](…)`:

   ```markdown
   ### Features

   - *(auth)* Add biometric login
   - [**breaking**] Drop the v1 sync API ([PROJ-12](https://…/PROJ-12))

   ### Bug Fixes

   - *(ui)* Fix crash when rotating the settings screen
   ```

   Scope, the breaking marker and ticket links stay. Commit bodies, footers, contributors and
   statistics (the `release_notes` root) are not part of it. Group heading level follows
   `commits.types_heading_level`, as everywhere.
5. **`--format markdown|text`, default `markdown`.** `markdown` is the rendered template as it is.
   `text` renders the same Markdown and then converts it to plain text with a CommonMark parser
   (goldmark, already in the module graph), so there is one set of templates and a user's own
   `notes` templates are converted too. The converter writes a heading as a plain line, a list item
   as `- text`, drops emphasis, strong and code-span markers, and keeps a link's text and drops its
   URL. Because it parses Markdown instead of stripping characters, `snake_case` is untouched
   (CommonMark does not treat an intraword `_` as emphasis) and `` `--force` `` becomes `--force`. Text
   that CommonMark itself reads as formatting (a description holding `*stars*`) loses those markers,
   exactly as a Markdown viewer would show it. The format is a flag only: no config key.
6. **stdout by default**, `--output <file>` to write a file. Warnings and degraded-run notices go to
   stderr, so `heraut notes v7.8.0 > store.txt` captures only the text.

## Behaviour

```
heraut notes <tag> [--from <tag>] [--format markdown|text] [--template <file>] [--output <file>]
                   [--env <name>] [--offline]
```

| Input | Result |
|---|---|
| `heraut notes v7.8.0` | the groups and commit lines for `v7.7.0..v7.8.0` (the previous release of `v7.8.0`), as above |
| `heraut notes v7.8.0 --from v7.6.0` | the same, spanning `v7.6.0..v7.8.0` |
| `--format text` | the same notes converted to plain text (`Features`, `- (auth) Add biometric login`) |
| `--format` with another value | usage error listing `markdown` and `text` |
| tag does not exist | runtime error naming the tag; nothing is written |
| `--from` does not exist | runtime error naming it |
| range holds no commit that survives `commits.types` / `rendering.excludes` | empty stdout, exit 0, a stderr notice |
| per-env strategy without `--env` | the usual `--env is required` error |
| `--offline` | forces `commits.enrichment_policy: disabled`, like every other command |
| `--output <file>` | writes the file instead of stdout, creating it or replacing its contents |

The default text carries no version, so `HeadingVersionPattern` plays no part. A user template that
prints `.Version` gets it as for `CHANGELOG.md`.

### Template resolution

Lowest to highest precedence, the existing order (Spec 05 § User-customizable templates), with the
`notes` block as the driver:

1. built-in templates; for this command the `release.section` block is empty and `commit.message`
   is the lean line of decision 4 (the other built-in blocks are the changelog's)
2. global `rendering.templates` (so a project-wide `commit.message` override reaches the notes too)
3. `notes.rendering.templates`
4. `notes.template` file, or `--template <file>` when given (the flag replaces the config file)

The root block is `changelog`, so every changelog override (`release.section`, `release.group`,
`commit.message`, ...) applies. Restoring the hash and author on a line, or adding a heading, is one
override:

```yaml
notes:
  rendering:
    templates:
      release:
        section: "## {{ .Version }}"        # add a heading
      commit:
        message: "- {{ .Description }} ({{ .ShortHash }})"
```

A `notes:` block with neither key is allowed and equals no block.

## Design

- **`internal/config`**: `Config.Notes *ContentDriver` (reusing the type that already carries
  `Template` and `Rendering`; `Output` and `TagPattern` are rejected by validation under `notes`,
  since the command neither writes a changelog nor filters by pattern). `schema.json`, the sample
  and Spec 02 get the block.
- **`internal/generators/native`**: a third `Mode`, `ModeNotes`. `Generate(tag, lc)` collects the
  range commits and renders them through `renderChangelogSection` with the notes built-in blocks
  (empty `release.section`, lean `commit.message`), with no file read, no splice and no write. The previous-tag bound comes from the existing `scopedPreviousTag`; `--from` is a
  new `Option` that replaces it.
- **`internal/app`**: `BuildNotesGenerator(cfg, env, ...)` builds the generator with the
  `notes` driver config, the strategy's tag order and `HeadingVersionPattern`, and the same
  enrichment and degraded-run handling as `BuildChangelogPipeline`. `app` stays the only place
  that constructs it.
- **`internal/cmd/notes.go`**: parses flags, loads and validates the config, resolves `--env`
  like `changelog` does, calls the app builder, prints. No pipeline run: the command needs the
  generator only.
- **Plain text**: `internal/generators/native/plaintext.go`, a goldmark AST walk, applied by
  the command layer's caller (`app`) only when `--format text`. `github.com/yuin/goldmark` moves from
  an indirect to a direct requirement in `go.mod`.
- **Layering**: unchanged (`cmd` to `app` to `generators/native`).
- **Docs**: Spec 03 (`heraut notes`), Spec 02 (`notes:`), Spec 05 (template precedence names the new
  driver), README command list, CLAUDE.md command list, a short ADR (the new command and block are
  public surface), and a how-to in `docs/guides/` for the store workflow.

## Testing

- **Unit**: flag handling and errors (unknown tag, unknown `--from`, per-env without `--env`);
  config validation (`notes.output` and `notes.tag_pattern` rejected, empty block accepted);
  `ModeNotes` renders the changelog section without touching any file (a `MockRunner` asserts the
  exact git calls and that nothing is written).
- **Real git**: bounds. A final skips its `rc` builds, `--from` spans several, a maintenance line and
  a version branch stay on their line, an unreleased tag cut elsewhere does not leak in.
- **Template**: the default prints the groups and lean lines of decision 4 and no header; the
  override above restores a heading and hash; `--template` replaces `notes.template`; precedence of
  the layers, including a global `commit.message` reaching the notes.
- **Plain text**: a table over the converter (headings, nested lists, emphasis, strong, code span,
  link, autolink, escaped characters, `snake_case`, a description holding `*stars*`), and the sample
  range of decision 4 converted end to end.
- **E2E (Lane A)**: on the built binary, stdout carries the text and stderr stays empty; `--format
  text` has no Markdown characters in it; `--output` writes the file; an unknown tag exits non-zero
  with nothing on stdout.

## Out of scope

- Per-store character limits, one text per locale, uploading to the stores.
- JSON or any other output format; a `notes.format` config key; a named-profile mechanism (several `notes` templates in one config).
- A range given as two arbitrary refs that are not release tags.
- Writing the text into `CHANGELOG.md` or any other managed file.
