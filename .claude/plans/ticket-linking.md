# Design — Ticket linking in changelog + release notes

**Status:** design approved (brainstorming) — pending implementation
**Prospective roadmap task:** T79 · **Prospective ADR:** ADR-0024

## Context

heraut's embedded git-cliff configs ship `commit_preprocessors = []` and
`link_parsers = []` — both unused. Teams that reference issue-tracker tickets in commits
(Jira `PROJ-123`, Linear `ENG-45`, GitHub issues `#67`) get plain text in the changelog,
not links. A user *can* hand-write git-cliff `link_parsers` in an override config today, but
that requires raw git-cliff TOML knowledge. This feature makes ticket linking **first-class
in `.heraut.yml`**, consistent with heraut's "own the config, thin generators" direction
([ADR-0022](../../docs/adr/0022-fat-injection-thin-templates.md)).

## Decisions (from brainstorming)

- **Mechanism: git-cliff `link_parsers` (appended references)**, not `commit_preprocessors`
  (inline). Inline replacement only links text the changelog actually renders — the
  **subject** — so a ticket that lives in the body or a `Refs:` footer is linked *in place*
  but never displayed (demo-verified: the link was created inside the footer, which the
  changelog template discards). `link_parsers` extracts every ticket into `commit.links`
  regardless of source location, so subject/body/footer all surface. The user requires
  body/footer matching, so this is the only viable mechanism.
- **Scope: git-cliff only.** cocogitto/communique have no link mechanism. `tickets` set with
  a non-git-cliff generator is a config error.
- **Config: a top-level `tickets:` list**, governing **both** changelog and release-notes
  generation — mirroring the top-level `remote_metadata` decision in
  [T78](../../docs/tasks/roadmap.md) for the same "applies to both generators" reason.
- **`{ticket}` URL placeholder**, mapped to git-cliff's `$1` capture group.
- **Append-only** for v1 — a subject ticket shows in prose *and* as the appended link.
  Subject stripping is out of scope (possible later refinement).

## Config surface

```yaml
tickets:
  - pattern: '[A-Z]+-[0-9]+'                          # Jira/Linear: whole token is the URL value
    url: 'https://acme.atlassian.net/browse/{ticket}' # → label "PROJ-123", …/browse/PROJ-123
  - pattern: 'GH-([0-9]+)'                            # GitHub issues: only the number goes in the URL
    url: 'https://github.com/acme/app/issues/{ticket}' # → label "GH-123", …/issues/123
```

- A **list** of `{ pattern, url }`. Absent/empty → feature off (today's behavior, no regression).
- `pattern`: a regex.
  - The **link label** is always the **full match** (git-cliff `text` defaults to `$0`).
  - `{ticket}` (the URL value) is the **first capture group** if the pattern has one,
    otherwise the **full match**. So a system whose URL needs only part of the token (GitHub
    `GH-123` → `…/issues/123`) wraps just that part in `()`; a system where the whole token
    is the URL value (Jira `PROJ-123` → `…/browse/PROJ-123`) needs no group.
- `url`: an http(s) URL that must contain the literal `{ticket}`.

## Mechanism / data flow

1. `internal/config/config.go`: add `type Ticket struct { Pattern, URL string }` and
   `Config.Tickets []Ticket` (yaml `tickets`). Add a programmatic `Tickets []Ticket` carrier
   on `ContentDriver` (`yaml:"-"`), exactly like `RemoteMetadata` / `HeadingVersionPattern`.
2. `internal/app/pipeline.go`: propagate `cfg.Tickets` onto each driver in
   `withEnvDerivations` (the existing clone point), alongside `RemoteMetadata`.
3. `internal/generators/gitcliff/generator.go`: in `effectiveConfig` (after `MergeTOML` +
   `injectHeadingPostprocessor`), add `injectLinkParsers`: for each ticket, build a git-cliff
   entry `{ pattern = <P>, href = <url with {ticket}→$1> }` where `<P>` is the user pattern
   wrapped in a capture group **only when it has none** (`regexp.Compile(pattern).NumSubexp()
   == 0` → wrap; otherwise use as-is so `$1` is the user's first group). `text` is omitted, so
   git-cliff uses the full match as the label. **Append** to the `[git].link_parsers` array
   (never clobber a user-supplied override — append, like the postprocessor injection prepends).
4. git-cliff scans the full commit message (subject/body/footer) — demo-verified — so footer
   `Refs: PROJ-123` references are linked.

## Output / templates

The shared `print_commit` macro in **both** `cliff.changelog.toml` and
`cliff.release-notes.toml` appends each extracted link after the existing
`by @author in #PR` segment:

```
- *(auth)* Add SSO login - (abc1234) by @bob in #42 ([PROJ-123](…/browse/PROJ-123))
```

Exact format (`([TICKET](url))` vs `— [TICKET](url)`) and whitespace control are settled at
implementation with a **real-CLI render check** (the same method used for the contributors
section), since automated tests cover config-acceptance only.

## Validation (`internal/config/validator.go`)

- each `tickets[].pattern`: non-empty and compiles via `regexp.Compile`.
- each `tickets[].url`: non-empty, valid http(s), contains the literal `{ticket}`.
- `tickets` non-empty **and** the effective generator (changelog or release.notes) is not
  `git-cliff` → error with a hint that ticket linking is git-cliff-only.

## Testing

- **config**: a valid fixture with `tickets`; invalid fixtures (bad regex, url missing
  `{ticket}`, `tickets` + cocogitto) + validator unit tests.
- **gitcliff unit**: `effectiveConfig`/`injectLinkParsers` produces the expected
  `[git].link_parsers` — the no-group pattern is wrapped (`[A-Z]+-[0-9]+` → `([A-Z]+-[0-9]+)`),
  a pattern that already has a group is left as-is (`GH-([0-9]+)`), `{ticket}`→`$1` in `href`,
  and entries are appended to an existing user `link_parsers` rather than clobbering it.
- **real-CLI smoke**: a `t.TempDir` repo commit carrying a ticket renders the link (offline,
  skippable — like `TestEmbeddedConfig_RealGitCliff`).
- **schema**: `schema.json` + valid/invalid fixtures; `docs/heraut.sample.yml`.

## Files touched

- `internal/config/{config.go,validator.go}` — `Ticket`, `Config.Tickets`, driver carrier, validation
- `internal/app/pipeline.go` — propagate `cfg.Tickets` onto the driver
- `internal/generators/gitcliff/generator.go` — `injectLinkParsers`
- `internal/generators/gitcliff/{cliff.changelog.toml,cliff.release-notes.toml}` — render `commit.links`
- `schema.json`, `docs/heraut.sample.yml`, `docs/specs/02-configuration.md`
- `docs/adr/0024-ticket-linking.md`, `docs/tasks/roadmap.md` (T79)

## Non-goals (v1)

Inline replacement; subject stripping; cocogitto/communique support; per-system labels;
deduplication of overlapping patterns.
