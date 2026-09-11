# Release Hooks — exploration notes (cocogitto-style bump hooks)

- **Status**: Superseded by
  [`2026-09-11-release-hooks-design.md`](2026-09-11-release-hooks-design.md), which resolves every
  open question below.
- **Date**: 2026-08-28
- **Author**: bchatard (with Claude)
- **Source**: [cocogitto bump hooks](https://docs.cocogitto.io/guide/bump.html#bump-hooks)

---

## The idea

Let a user run arbitrary shell commands at specific points in the release pipeline — `cargo build
--release` before the version bump, `npm publish` / `git push` after the tag is created. This is
cocogitto's "bump hooks" feature; heraut has nothing like it today.

## What cocogitto does

- Two hook lists in `cog.toml`: `pre_bump_hooks` (run before the changelog and version commit are
  created) and `post_bump_hooks` (run after the tag is created).
- Commands are plain shell strings with template variables substituted in: `{{version}}`,
  `{{latest}}` (most recent known version), `{{version_tag}}`, `{{latest_tag}}`, `{{package}}`
  (monorepo package id). Variables support Tera-style defaults, e.g. `{{latest|0.0.0}}`.
- Sequence: resolve version → run pre-bump hooks → write changelog → version commit → tag → run
  post-bump hooks.
- **Failure handling is asymmetric and a known rough edge**: a pre-bump hook failure aborts the
  bump and stashes the working changes under `cog_bump_{{version}}` (recoverable). A post-bump
  hook failure aborts with **no rollback** — the tag already exists, the repo is left in whatever
  state the failed hook left it in.
- Advanced features layered on top: **bump profiles** (`--hook-profile`, alternate hook sets per
  branching strategy), a **version-arithmetic DSL** usable inside hook strings
  (`{{version+1minor-SNAPSHOT}}`), and **branch whitelisting**. All of these look like natural
  non-goals for a first pass at this in heraut.

## Mapping onto heraut's pipeline

heraut's release pipeline is richer than cocogitto's — cocogitto has no publish step at all.
Current sequence (`internal/pipeline/release.go`): resolve → generate changelog → commit → tag →
publish (create GitHub/GitLab release + assets). The changelog-only pipeline
(`internal/pipeline/changelog.go`) stops after an optional commit/tag.

cocogitto's two hook points map directly onto "before changelog generation" and "after tag," which
both of heraut's pipelines already have as steps. heraut's *publish* step has no cocogitto
equivalent, which is the first real open question below.

## Where this would touch the codebase

| Area | What's needed |
|---|---|
| Config schema (`internal/config/config.go` + validator) | A `Hooks`-shaped struct, probably alongside `versioning:` since it's about the bump lifecycle rather than content generation. Per-env override would need its own merge logic, mirroring `MergeContentDriver`. |
| Template substitution | `{{version}}`/`{{previous_tag}}` etc. in hook strings. Could reuse Go's `text/template` (already used for changelog rendering) instead of copying cocogitto's Tera syntax. |
| **Shell execution via `port.Runner`** | New territory. Every existing `Runner.Run` call today is a structured invocation of a known binary (`git`, `gh`, `glab`) with discrete args. A hook is a free-form user string that needs `sh -c "<string>"` — nothing in heraut does this today. Also the first place heraut would need an OS-conditional execution path: `sh -c` doesn't exist on Windows, and heraut ships Windows binaries (GoReleaser, ADR-0013). |
| Pipeline wiring | New steps in both `internal/pipeline/{changelog,release}.go`'s `Run()`, following the existing `runStep`/dry-run conventions. |
| App-layer plumbing | Resolve + merge effective hooks in `internal/app`, same shape as existing `CommitMessage`/`ContentDriver` propagation. |
| Docs + likely a new ADR | This would be heraut's first "execute arbitrary shell commands from config" capability — the trust-model note alone (config file = code execution, same model as a CI YAML `run:` step) seems worth its own ADR rather than a spec-only mention. |

None of these are individually hard — each follows a pattern already in the codebase somewhere.
But the surface area (schema + two pipelines + a genuinely new execution mode + docs) puts this in
the same ballpark as the changelog-rotation epic just finished: a handful of tasks, its own
dedicated roadmap file, not a single S-sized task.

## Open questions

**Hook points**
- Mirror cocogitto's 2 (before-changelog, after-tag), or add a 3rd for heraut's own post-*publish*
  step, which cocogitto has no equivalent of?
- Do hooks apply to `heraut changelog` too (which also commits/tags), or `heraut release` only?

**Template syntax**
- Go's own `{{ .Version }}` (consistent with heraut's changelog templates, and reuses existing
  machinery) vs. cocogitto's `{{version}}` (familiar to anyone migrating from cocogitto)?
- Template substitution vs. environment variables (`HERAUT_VERSION`, `HERAUT_PREVIOUS_TAG`, …) as
  the mechanism — or both? Env vars sidestep needing any templating engine at all, at the cost of
  being less discoverable than an explicit `{{version}}` in the config file.

**Failure semantics**
- Match cocogitto's "pre-hook failure aborts, post-hook failure aborts with no rollback," or add a
  "warn and continue" mode for post-hooks specifically, since by that point the tag/release already
  exists and a hard abort doesn't undo anything either way?

**Platform**
- POSIX-only for v1 (`sh -c`, documented limitation on Windows), or does it need `cmd`/PowerShell
  support from day one given heraut ships Windows binaries?

**Scope**
- Per-env hook overrides in scope for v1, or flat-only first (mirroring how changelog rotation
  started flat-only and left per-env for later)?
- A `--no-hooks` escape hatch for troubleshooting/CI edge cases, or is that scope creep for a v1?
- Does `--dry-run` show the *rendered* command (real version substituted in) or the raw template?
- Working directory: always repo root, or configurable per hook?

## Explicit non-goals (v1, if this gets built)

- Bump profiles / `--hook-profile`
- Version-arithmetic DSL in hook strings
- Branch whitelisting
- Per-package hooks (heraut has no monorepo/multi-package concept at all, unlike cocogitto)

## Next steps

Nothing filed yet — no roadmap task, no design doc committed to `Status: Approved`. Revisit this
file and turn it into a real design doc (`docs/superpowers/specs/<date>-release-hooks-design.md`,
following the changelog-rotation design's structure) once the open questions above have answers.
