# ADR-0070: `heraut notes` — release notes of a tag for any consumer

- **Status**: Accepted
- **Date**: 2026-10-13
- **Deciders**: bchatard

---

## Context

heraut renders release notes in two places only: the `CHANGELOG.md` section written by
`heraut changelog` / `heraut release`, and the body sent to the forge release. Neither can be printed
on its own, and both carry forge-oriented decoration (a release heading with a compare link, a commit
hash, an author handle, a PR reference). A mobile project needs the release text separately for the
App Store or Play Store submission, without that decoration and sometimes without Markdown.

## Decision

1. **A read-only command, `heraut notes <tag>`.** It prints the groups and commit lines of the range
   from the tag's previous release (or `--from`) to the tag, and does nothing else.
2. **The bound is the previous release reachable from the requested tag.** A release walks the tags
   reachable from `HEAD`; a notes run for a tag cut on another line would bound wrongly if it did the
   same. A SemVer final skips pre-releases (ADR-0064); a pre-release is bounded by the previous tag of
   any kind.
3. **A `notes:` block with `template` and `rendering`, no `output` or `tag_pattern`**, and a
   `--template` flag for one run. A dedicated config type makes strict parsing reject every other key.
4. **Own built-in blocks, below every user override.** An empty `release.section` and a
   `commit.message` without hash, author or PR reference are seeded as the lowest inline snippets of
   a third generator mode (`ModeNotes`), so the existing precedence (global `rendering.templates`,
   then the driver's, then its template file) applies unchanged. A project-wide `commit.message`
   override therefore reaches the notes too.
5. **`--format markdown|text`.** `text` renders the same Markdown, then converts it with a
   CommonMark parser (goldmark). There is one set of templates and a user's own `notes` templates
   are converted too. Command-line only, no config key.

## Consequences

- `github.com/yuin/goldmark`, already in the module graph through the terminal UI libraries, becomes a
  direct requirement.
- Text mode drops what CommonMark reads as formatting: a description containing `*stars*` loses its
  asterisks, as a Markdown viewer would show it. `snake_case` and a backticked flag survive or lose
  only their backticks.
- The default output keeps the changelog's own blank-line spacing between commits; text mode
  normalises it.
- The first consumer is an app store, but nothing in the command is store-specific.

## Alternatives considered

- **Strip Markdown characters from the rendered text.** Lossy: `snake_case` or a literal `*` in a
  commit description cannot be told from formatting.
- **A second, plain-text set of built-in templates.** Two sets to keep in sync, and a user's own
  Markdown templates would not be converted.
- **Reuse the `release.notes` driver.** Couples the store text to the forge release body, which
  differs in purpose.
- **Per-store character limits, locales, upload.** Left out of the first version.
