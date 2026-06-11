# ADR-0024: Ticket linking via git-cliff link_parsers

- **Status**: Accepted
- **Date**: 2026-06-11
- **Deciders**: bchatard

---

## Context

heraut's embedded git-cliff configs ship `commit_preprocessors = []` and `link_parsers = []`
— both unused. Teams that reference issue-tracker tickets in commits (Jira `PROJ-123`, Linear
`ENG-45`, GitHub issues) get plain text in the changelog and release notes, not links. A user
*could* hand-write git-cliff `link_parsers` in an override config today, but that requires raw
git-cliff TOML knowledge. heraut's direction ([ADR-0022](0022-fat-injection-thin-templates.md))
is to own the config and keep generator config thin, so ticket linking should be first-class
in `.heraut.yml`.

git-cliff offers two relevant mechanisms:

- **`commit_preprocessors`** — a find-and-replace on the raw commit text. It links a ticket
  *in place*, wherever it physically sits.
- **`link_parsers`** — extracts every match into a structured `commit.links` list, independent
  of where the ticket appeared.

cocogitto and communique have no link mechanism.

## Decision

**Add a top-level `tickets:` list** in `.heraut.yml`, validated in `internal/config`,
propagated onto each content driver by the app layer (like `remote_metadata`), and **injected
as git-cliff `link_parsers`** into the effective merged TOML by the gitcliff generator. Both
embedded templates render `commit.links`. **git-cliff only** — `tickets` with a non-git-cliff
generator is a config error.

```yaml
tickets:
  - pattern: '[A-Z]+-[0-9]+'
    url: 'https://acme.atlassian.net/browse/{ticket}'
```

### Why `link_parsers`, not `commit_preprocessors`

The changelog renders only the commit **subject** (`commit.message`). `commit_preprocessors`
links a ticket in place, so a ticket in the **body or a `Refs:` footer** is linked — but inside
text the changelog discards, so it never appears (demo-verified: the preprocessor produced
`Refs: [PROJ-202](…)` in the footer, which the template never prints). The user requires
body/footer matching, and `link_parsers` is the only mechanism that surfaces tickets regardless
of location, because it collects them into `commit.links` which the template renders.

### `{ticket}` = capture group; label = full match

`{ticket}` maps to git-cliff's `$1` — the pattern's **first capture group**, or the **full
match** if the pattern has none (heraut wraps a group-less pattern in `()`). The link **label**
is always the full match (git-cliff's `text` default). This one knob handles both shapes: Jira
`[A-Z]+-[0-9]+` (no group → label & URL value both `PROJ-123`) and GitHub `GH-([0-9]+)` (group
→ label `GH-123`, URL value `123` → `/issues/123`).

### Append-only

The ticket is appended to the commit line as `([TICKET](url))`. When the ticket is in the
subject it appears twice (in prose and as the appended link); stripping the subject reference
is deferred (YAGNI). heraut's entries are **appended after** any user-supplied `link_parsers`.

## Consequences

- Ticket linking is configured once in `.heraut.yml` and works in both the changelog and release
  notes, for tickets in the subject, body, or footer.
- The feature is git-cliff-only; cocogitto/communique users who set `tickets` get a clear config
  error rather than a silent no-op.
- Multiple ticket systems are a list of entries at near-zero extra cost (git-cliff `link_parsers`
  is already an array).
- Rendered-output formatting is verified by real-CLI render checks, not byte-assertion tests,
  consistent with how the embedded templates are otherwise tested.
