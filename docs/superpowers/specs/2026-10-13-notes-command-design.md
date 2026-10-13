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
4. **Default rendering is the changelog's.** Without any `notes` configuration the command prints
   one changelog section for the range: the `## [<tag>](<compare>) - <date>` heading, the groups, and
   one line per commit, exactly what `CHANGELOG.md` holds for that release. Commit bodies, footers,
   contributors and statistics (the `release_notes` root) are not part of it.
5. **stdout by default**, `--output <file>` to write a file. Warnings and degraded-run notices go to
   stderr, so `heraut notes v7.8.0 > store.txt` captures only the text.

## Behaviour

```
heraut notes <tag> [--from <tag>] [--template <file>] [--output <file>] [--env <name>] [--offline]
```

| Input | Result |
|---|---|
| `heraut notes v7.8.0` | the changelog-style section for `v7.7.0..v7.8.0` (the previous release of `v7.8.0`) |
| `heraut notes v7.8.0 --from v7.6.0` | the same section spanning `v7.6.0..v7.8.0` |
| tag does not exist | runtime error naming the tag; nothing is written |
| `--from` does not exist | runtime error naming it |
| range holds no commit that survives `commits.types` / `rendering.excludes` | the heading with no groups, exit 0, a stderr notice; the text is still valid for a store |
| per-env strategy without `--env` | the usual `--env is required` error |
| `--offline` | forces `commits.enrichment_policy: disabled`, like every other command |
| `--output <file>` | writes the file instead of stdout, creating it or replacing its contents |

The section's heading version is the tag as rendered by the existing `HeadingVersionPattern`
logic, so a per-env or build-metadata tag shows its bare version like `CHANGELOG.md` does.

### Template resolution

Lowest to highest precedence, the existing order (Spec 05 § User-customizable templates), with the
`notes` block as the driver:

1. built-in templates
2. global `rendering.templates`
3. `notes.rendering.templates`
4. `notes.template` file, or `--template <file>` when given (the flag replaces the config file)

The root block is `changelog`, so every changelog override (`release.section`, `release.group`,
`commit.message`, ...) applies, and a plain-text store template is the usual case:

```yaml
notes:
  rendering:
    templates:
      release:
        section: ""                         # no heading
        group: "{{ .Name }}:"
      commit:
        message: "- {{ .Description }}"    # no hash, author or links
```

A `notes:` block with neither key is allowed and equals no block.

## Design

- **`internal/config`**: `Config.Notes *ContentDriver` (reusing the type that already carries
  `Template` and `Rendering`; `Output` and `TagPattern` are rejected by validation under `notes`,
  since the command neither writes a changelog nor filters by pattern). `schema.json`, the sample
  and Spec 02 get the block.
- **`internal/generators/native`**: a third `Mode`, `ModeNotes`. `Generate(tag, lc)` collects the
  range commits and renders them through `renderChangelogSection`, with no file read, no splice
  and no write. The previous-tag bound comes from the existing `scopedPreviousTag`; `--from` is a
  new `Option` that replaces it.
- **`internal/app`**: `BuildNotesGenerator(cfg, env, ...)` builds the generator with the
  `notes` driver config, the strategy's tag order and `HeadingVersionPattern`, and the same
  enrichment and degraded-run handling as `BuildChangelogPipeline`. `app` stays the only place
  that constructs it.
- **`internal/cmd/notes.go`**: parses flags, loads and validates the config, resolves `--env`
  like `changelog` does, calls the app builder, prints. No pipeline run: the command needs the
  generator only.
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
- **Template**: the default equals the section `heraut changelog` writes for the same tag; the
  plain-text override above; `--template` replaces `notes.template`; precedence of the layers.
- **E2E (Lane A)**: on the built binary, stdout carries the text and stderr stays empty; `--output`
  writes the file; an unknown tag exits non-zero with nothing on stdout.

## Out of scope

- Per-store character limits, one text per locale, uploading to the stores.
- JSON or any non-text output; a named-profile mechanism (several `notes` templates in one config).
- A range given as two arbitrary refs that are not release tags.
- Writing the text into `CHANGELOG.md` or any other managed file.
