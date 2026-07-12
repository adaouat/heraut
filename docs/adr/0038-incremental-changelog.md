# ADR-0038: Incremental changelog generation (native)

- **Status**: Accepted
- **Date**: 2026-07-10
- **Deciders**: bchatard
- **Builds on**: [ADR-0034](0034-native-remote-enrichment.md) §5 (O(1) enrichment of the newest
  section), [ADR-0037](0037-native-template-api.md) (the customizable `header` block this feature
  is deliberately decoupled from)

---

## Context

`native.generateChangelog` rewrote the entire `CHANGELOG.md` on every run, enriching only the
newest (unreleased) section — historical sections rendered from git alone, keeping enrichment
O(1) per [ADR-0034](0034-native-remote-enrichment.md) §5. git-cliff, by contrast, also
full-regenerates but enriches *every* section it renders, so a git-cliff-managed changelog carries
`by @user` attribution throughout its history.

Consequence: switching a project (heraut itself included) from `git-cliff` to `native` meant the
next release rewrote `CHANGELOG.md` and **stripped historical PR-author attribution** from every
past section — a visible, one-time regression in committed history.

There was also no `docs/specs/` definition of the changelog file structure; the format was
implicit in the generator.

Full design: [`docs/superpowers/specs/2026-07-10-incremental-changelog-design.md`](../superpowers/specs/2026-07-10-incremental-changelog-design.md).

## Decision

Give the `native` generator's changelog **two modes**, and formalize the changelog file
structure. **Native-only** — `git-cliff` is unchanged (it already full-regenerates with
enrichment).

### 1. Incremental (default)

Render only the unreleased section (commits `latest_tag..HEAD` → the version being released),
enrich just that section (O(1) API calls, regardless of history length), and **splice** it into
the existing file, leaving every other section verbatim. Historical attribution is preserved
because past sections are never rewritten.

The existing `Output` file is branched on its state:

- **Missing or empty** → **bootstrap**: build all sections from all tags (anchored), enrich the
  newest only (today's default enrichment). No warning — there is nothing to preserve.
- **Contains ≥1 anchor** → **splice**: locate the first anchor (everything before it is the
  preserved preamble). If its tag equals the tag being released, **replace** that section
  (idempotent re-runs); otherwise **insert** the new anchored section above it. All other sections
  are untouched.
- **Non-empty but anchorless** (foreign — e.g. a `git-cliff`-generated file, or legacy heraut
  output) → **stop with an error naming `--regenerate`**; the file is left byte-for-byte
  unchanged. During `heraut release` this aborts the run — the operator must consciously
  `--regenerate` once before switching to incremental.
- A malformed anchor (no captured tag) counts as no usable anchor at that position; if none exist
  anywhere, the file is treated as anchorless (the foreign-file error above), never silently
  spliced at the wrong position.

### 2. Full regeneration (`--regenerate` / `--regenerate-changelog`)

Ignore the existing file's contents. Build every section from all tags (each anchored), enrich
**all** sections, and overwrite `Output`. Enrichment happens **per section** — `buildAllSections`
calls `renderRelease` → `enrichForRelease` once per release tag — so each platform's batching
primitive is applied within a single release, not across the whole file:

| Platform | Batch primitive | Cost for full history |
|----------|-----------------|-----------------------|
| GitHub | GraphQL, 50 SHAs/query, issued once per release | O(releases) |
| Azure | one `pullrequestquery` POST per release | O(releases) |
| GitLab | per-commit `glab api` call | **O(commits)** |

GitHub and Azure only exceed one call per release when a single release's commit count outgrows
the batch (>50 SHAs on GitHub); GitLab pays one call per commit regardless of how those commits
are distributed across releases.

When full regeneration targets a **GitLab** remote, the changelog pipeline step (not the
generator — it alone knows both the flag and the resolved platform) emits a warning that
enrichment is one API call per commit and may be slow / rate-limited.

This mode is exposed as a CLI flag only — no `.heraut.yml` knob (YAGNI; incremental is always the
default):

- `heraut changelog --regenerate`
- `heraut release --regenerate-changelog`

The flag is plumbed to the native generator the same way the running heraut version is: a
`yaml:"-"` field on `config.ContentDriver` (`RegenerateChangelog bool`), set from `PipelineOpts` in
the app layer. Non-native generators ignore it.

### Changelog file structure (formalized)

A native-managed `CHANGELOG.md` is a preamble followed by anchored sections, newest first:

```
<preamble>            # everything before the first section anchor — e.g. "# Changelog\n\n"
<anchor> <section>     # newest release
<anchor> <section>     # …older releases, newest-first
```

- **Preamble** — free-form content before the first anchor. Incremental mode preserves it
  verbatim.
- **Anchor** — a stable HTML comment on its own line immediately before each section:
  ```
  <!-- heraut-release: v0.49.0 -->
  ```
  It carries the release **tag** (not the display version) and is invisible in every Markdown
  renderer (GitHub included), mirroring git-cliff's own use of HTML comments (`<!-- 0 -->` for
  group ordering). **It is emitted by the assembly layer, not by any template block** — it is
  structural and non-overridable. Customizing the `header` block ([ADR-0037](0037-native-template-api.md))
  can neither remove it nor change its shape; that decoupling is what lets the splice parser stay
  independent of the visible header.
- **Section** — the rendered release (the `header` block + groups + commits), unchanged by this
  feature — the anchor wraps around a section's render output, never inside it.

## Consequences

- **Historical attribution survives generator switches and every subsequent release.** Only the
  section being released is ever rewritten by default.
- **A one-time, explicit migration step is required per project switching onto `native`** (or onto
  anchored native output for the first time): the first release must pass `--regenerate` /
  `--regenerate-changelog`, or it aborts with an actionable error. This is intentional — silently
  stripping history on a generator switch is the exact failure this ADR removes.
- **`heraut release --regenerate-changelog` in CI is a one-time input, not a code change.**
  `.github/workflows/release.yml`'s `workflow_dispatch` gained a `regenerate_changelog` boolean
  input (default `false`); heraut's own migration to `native` dispatches once with it checked, then
  leaves it unchecked for every subsequent release.
- **GitLab full regeneration is expensive** (one API call per commit) and now surfaced as a pipeline
  warning rather than a silent slow run.
- **Anchors are a permanent structural commitment for `native`-managed changelogs.** Any tool or
  hand-edit that strips them (or a header/footer template rewrite that somehow removed the
  assembly-layer wrapping) turns the file anchorless, which reverts new releases to the
  full-regeneration error path until `--regenerate` is run again.

## Alternatives considered

- **Always full-regenerate with enrichment** (adopt git-cliff's model outright). Rejected: this is
  the status quo problem — it reintroduces unbounded API cost (O(commits) rather than O(1)) on
  every release, not just the one-time migration.
- **A `.heraut.yml` mode knob** (`changelog.incremental: false`). Rejected as YAGNI: a CLI flag
  used once per migration (or rarely, to repair drift) doesn't warrant a persistent config surface;
  flags suffice.
- **A machine-readable sidecar (YAML front matter / `changelog.json`) instead of an HTML-comment
  anchor.** Rejected: front matter renders visibly on GitHub, and a sidecar doesn't solve
  section-boundary location within `CHANGELOG.md` itself. Revisit only if a real machine consumer
  appears.
- **Backfilling missing historical releases in incremental mode.** Out of scope — that is what
  `--regenerate` is for.
