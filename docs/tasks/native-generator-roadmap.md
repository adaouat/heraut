# Héraut — Native Content Generator Roadmap

> Status: Planned
> ADR: [ADR-0032](../adr/0032-native-content-generator.md)
> Main roadmap: tracked as Phase 23 in [`roadmap.md`](roadmap.md)

This roadmap breaks down the **native (built-in) content generator** epic
([ADR-0032](../adr/0032-native-content-generator.md)) into incrementally shippable tasks.
It lives in its own file because the work is heavy and multi-phase; the main roadmap keeps
only a Phase 23 pointer here.

`native` is an **opt-in, zero-external-dependency** generator (`generator: native`) that
renders changelogs / release notes in pure Go, without the external `git-cliff` binary.
**`git-cliff` stays the default** and the power-user escape hatch — `native` does not aim
for parity with its full TOML / Tera surface (see ADR-0032).

## Conventions

- Task IDs **continue the global sequence** (`T122+`) so they never collide with the main
  roadmap's IDs.
- This file is the **single source of truth** for task status. Same checkbox markers as the
  main roadmap: `[ ]` not started, `[x]` done. Follow the same two-step flow
  ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD: failing test first),
  then flip `[ ]` → `[x]` and add a one-paragraph completion note.
- The main `roadmap.md` Phase 23 block is a navigable index only; it carries no checkboxes
  to keep status in one place.

## Progress at a glance

| Phase                                            | Tasks       | Status              |
|--------------------------------------------------|-------------|---------------------|
| Phase 1 — native renderer (no remote enrichment) | T122 – T126 | Not started         |
| Phase 2 — remote enrichment via platform CLIs    | T127 – T129 | Not started         |
| Phase 3 — raw-HTTP clients (drop `gh` / `glab`)   | —           | Deferred (new ADR)  |

---

## Phase 1 — Native renderer (no remote enrichment)

Goal: a selectable `generator: native` whose output reaches **parity with `git-cliff
--offline`** — full conventional-commit taxonomy, grouping, commit / compare links from the
resolved `port.LinkContext` — but **without** PR author / number or the contributors block
(those are Phase 2). This is the low-risk bulk of the epic and is independently useful:
single-binary, offline changelog and release-notes generation.

New package: `internal/generators/native/` (implements `port.Generator`, peer to
`internal/generators/gitcliff/` and `internal/generators/communique/`).

#### `[x]` T122: commit collection — walk a tag range into structured records

Walk git history for the target tag range and parse each commit into a structured record
(hash, author name / email, date, subject, body). Resolve the previous tag for compare
links, honouring `tag_pattern` (per-env glob filtering) — reuse `internal/versioning/tagfmt`
glob logic rather than re-deriving it.

**Implementation:** `internal/generators/native/commits.go` — `collectCommits(runner
port.Runner, tagRange string) ([]RawCommit, error)`, enumerating via `git log [range]
--format=…%x00`, extending the NUL / `\x01`-delimited parsing pattern already used in
`internal/versioning/semver/resolver.go` (and T119's `CheckCommitRange`). Changelog mode
walks full history; release-notes mode walks the latest tag range.

**Tests:** contract tests (`MockRunner`) asserting the exact `git log` args per mode and
multi-line body / footer parsing; table tests for tag-range / previous-tag resolution under
`tag_pattern`.

**Files:** `internal/generators/native/commits.go` (+ test).
**Scope:** M. **Dependencies:** ADR-0032.

**Completion note (2026-06-26):** Implemented as unexported `collectCommits` / `previousTag`
/ `rawCommit` with a white-box `commits_internal_test.go` (the repo's `_internal_test`
convention), keeping the package surface minimal until T123/T125 consume them. `logFormat`
is `%H\x01%an\x01%ae\x01%cI\x01%s\x01%b\x00` — committer date (`%cI`), RFC3339-parsed;
`strings.SplitSeq` over the NUL separator. **Deviation from the task text:** `previousTag`
takes the env glob as a *string* rather than computing it from `tagfmt` here —
`internal/generators/*` may import only `port`/`config` (coding.md layering), so
`tagfmt.GlobPattern` stays in the app/caller layer (T125). Previous-tag resolution delegates
to `git describe --tags --abbrev=0 [--match <glob>] <tag>^` (git's topological ordering,
matching git-cliff's tag walk) instead of list-and-sort; a first release returns `("", nil)`
via the same stderr probes T121 uses. 9 contract tests; full suite 1206 green; `hk check`
clean.

---

#### `[x]` T123: classify / group / skip engine

Map each raw commit through `internal/conventionalcommit.Parse`, apply the embedded default
taxonomy (the same groups and ordering as the git-cliff defaults — feat → Features, fix →
Bug Fixes, …, `chore(release)` / `chore(deps)` skipped, the `.*security` body rule, catch-all
Other), and produce ordered groups (group ordering, scope-sorted, oldest-first). Exclude
merge / fixup commits via the existing `IsMergeCommit` / `IsFixupCommit` helpers.

**Implementation:** `internal/generators/native/group.go` — a pure
`groupCommits([]ParsedCommit) []Group`. Taxonomy + skip rules as Go data, not regex TOML.

**Tests:** table-driven over the full taxonomy, ordering, skip rules, and merge / fixup
exclusion (mirror the rows the git-cliff default encodes).

**Files:** `internal/generators/native/group.go` (+ test).
**Scope:** M. **Dependencies:** T122.

**Completion note (2026-06-27):** Implemented three unexported types in `group.go`: `parsedCommit{raw rawCommit; parsed *conventionalcommit.Commit}` (nil parsed for non-conventional subjects), `group{name string; order int; commits []parsedCommit}`, and `matchRule` (the taxonomy row struct). The entry point is `func groupCommits(commits []rawCommit) []group`. The taxonomy is a package-level `[]matchRule` slice (`commitRules`) in exact TOML array order — array position = match priority (first-match-wins), `order` field = display sort position (the `<!-- N -->` prefix in git-cliff). The security rule uses `isBody: true` so its regex is matched against `rawCommit.Body` rather than Subject; it fires only when no earlier subject rule matched. Skip rules carry `skip: true` (no name/order needed). Within each group, `sort.SliceStable` puts scoped commits first (scope ascending), then unscoped, preserving input (oldest-first) order as the tiebreak. 38 new tests across 10 sub-tests in `TestGroupCommits_Taxonomy`, plus `SkipRules`, `MergeAndFixupExcluded`, `FirstMatchWins`, `SecurityBodyAfterSubjectRules`, `GroupDisplayOrder`, `DocRefactorDisplayOrder`, `WithinGroupOrdering`, `EmptyInput`, and `ParsedCommitFields`; full suite 1244 green; `hk check` clean.

---

#### `[x]` T124: Go template renderer + view model (changelog + release-notes)

Assemble the view model and render both variants with `text/template`: version heading +
compare link (composed from `port.LinkContext`, reusing the ADR-0022 link-shape knowledge
already in Go), per-group sections, per-commit lines with commit links and ticket links
(`tickets:`), and — for the release-notes variant — the commit-statistics block. No PR
author / contributors yet (Phase 2). Heading post-processing (env / build-id stripping,
today's `injectHeadingPostprocessor`) applies here.

**Implementation:** `internal/generators/native/render.go` + embedded Go templates
(`//go:embed`). Keep templates thin (ADR-0022): assemble in Go, render dumb. Provide the
small func set (`upper_first`, short-hash truncate, etc.).

**Tests:** golden-file snapshots for both variants over deterministic fixture input; table
tests for the template funcs and link composition (GitHub / GitLab / Azure DevOps shapes).

**Files:** `internal/generators/native/render.go`, embedded `*.tmpl` (+ tests, golden
fixtures).
**Scope:** L. **Dependencies:** T123.

**Completion note (2026-06-27):** Implemented `render.go` (view-model builders + render
entry points + helpers), `links.go` (URL composition mirroring gitcliff's linkEnv/
azureDevOpsLinkEnv without importing that package), `changelog.tmpl`, and
`release_notes.tmpl`. Entry-point signatures:

```go
func renderChangelogSection(
    version, previousVersion string,
    releaseDate time.Time,
    groups []group,
    lc *port.LinkContext,
    tickets []config.Ticket,
    headingVersionPattern string,
) (string, error)

func renderReleaseNotes(
    version, previousVersion string,
    releaseDate time.Time,
    groups []group,
    lc *port.LinkContext,
    tickets []config.Ticket,
    prevReleaseDate time.Time, // zero ⇒ omit "days passed since last release" stat
) (string, error)

const changelogHeader = "# Changelog\n\n"
```

Links live in `links.go` (`buildCommitURL`, `buildCompareURL`); ticket matching in
`resolveTickets` (config order, full match text / capture-group-1 URL sub). The heading
is pre-computed in Go (`buildChangelogView`); templates are dumb iterators over
pre-populated view structs (`changelogView`, `notesView`). The release-notes commit block
(`commitNoteView.Block`) is also pre-formatted in Go (`buildCommitBlock`), keeping the
template branch-free. Intentional deviations from git-cliff --offline: (1) lc==nil
renders bare 7-char hashes (no Markdown link) and omits the compare URL, whereas git-cliff
still wraps in `([hash](hash))` with an empty base; (2) footer separator rendered as `: `
(space after colon) rather than git-cliff's `:` (no space); (3) release-notes body starts
directly with `### Group` (no leading `\n\n` as git-cliff produces). T126 will quantify
these. 33 new tests (golden + link-composition table + ticket table + upperFirst); full
suite 1277 green; `hk check` clean.

---

#### `[ ]` T125: native generator wiring + config + schema / docs (make it selectable)

Make `generator: native` an end-to-end, documented option. Implement `port.Generator`
(`Check` is trivial — no external binary; `Validate` checks the optional `template:` path;
`Generate(tag, lc)` runs T122–T124; `Degraded()` returns false in Phase 1). Wire the
`case "native"` into `buildGenerator` (`internal/app/pipeline.go`). Accept the value in
`internal/config/validator.go`. Update `schema.json`, `docs/heraut.sample.yml`, and
`docs/specs/05-generators-and-platforms.md` with the `native` enum value and its
(smaller) override surface.

**Implementation:** `internal/generators/native/generator.go`; `buildGenerator` case;
validator enum; schema + sample + spec.

**Tests:** contract test for `Generate` end-to-end against a fixture repo; validator test
for the accepted enum; a schema fixture in `testdata/config/`.

**Files:** `internal/generators/native/generator.go` (+ test), `internal/app/pipeline.go`
(+ test), `internal/config/validator.go` (+ test), `schema.json`,
`docs/heraut.sample.yml`, `docs/specs/05-generators-and-platforms.md`,
`testdata/config/native-*.yml`.
**Scope:** M. **Dependencies:** T124.

---

#### `[ ]` T126: golden-file parity harness vs real `git-cliff --offline`

The CHANGELOG-churn guard from ADR-0032. A **skippable** test (same precedent as the
existing git-cliff embedded-config smoke test, `testing.md`) runs the *real* `git-cliff
--offline` against a fixture repo and asserts `native`'s output matches, modulo documented,
intentional differences. `t.Skip`s when the binary is absent; runs in CI where `mise`
installs the pinned tool. Heraut's own deterministic golden snapshots (T124) always run;
this cross-check is the extra safety net.

**Implementation:** `internal/generators/native/parity_test.go` using `testutil.RealGitRepo`;
document any intentional deltas inline.

**Tests:** the harness itself (this task *is* the test).

**Files:** `internal/generators/native/parity_test.go`, fixture / golden data.
**Scope:** M. **Dependencies:** T125.

---

## Phase 2 — Remote enrichment via platform CLIs

Goal: add PR-number / author / first-time-contributor / linked-issue enrichment by calling
`gh api` / `glab api` through the existing `port.Runner` (the same path already used for
auth checks in `internal/platforms/`). Gated by the existing `remote_metadata` policy
([ADR-0023](../adr/0023-remote-metadata-policy.md)) and surfaced through the existing
`Degraded()` signal. Contract-tested with `MockRunner` — **no real network**.

#### `[ ]` T127: GitHub enrichment via `gh api`

Per commit SHA, resolve the associated PR (number, author handle) and first-time-contributor
status via `gh api` (e.g. `repos/{repo}/commits/{sha}/pulls`), fold the result into the
T124 view model, and populate the contributors block. Honour `remote_metadata`:
`required` (fatal on failure), `optional` (fall back to bare output + set `Degraded()`),
`disabled` (never call). Reuse the token / host env plumbing from the GitHub platform driver.

**Tests:** contract tests (`MockRunner`) queueing API JSON, asserting endpoints + args, and
covering each `remote_metadata` branch incl. the degraded fallback.

**Files:** `internal/generators/native/enrich_github.go` (+ test); view-model + render
updates for author / contributors.
**Scope:** L. **Dependencies:** T125.

---

#### `[ ]` T128: GitLab enrichment via `glab api`

The GitLab equivalent of T127 — resolve the associated MR (number, author) and contributors
via `glab api`, with the same `remote_metadata` gating and `Degraded()` behaviour. MR link
shape (`!`, `/-/merge_requests/`) already lives in Go (ADR-0022).

**Tests:** contract tests (`MockRunner`) for the GitLab endpoints + each policy branch.

**Files:** `internal/generators/native/enrich_gitlab.go` (+ test).
**Scope:** M. **Dependencies:** T127.

---

#### `[ ]` T129: Azure DevOps enrichment

Bring the native path to ADR-0026 parity — Azure DevOps PR / author enrichment via its API
(through the runner), reusing the Azure URL composition already in Go. Lower priority than
GitHub / GitLab; sequence last in Phase 2.

**Tests:** contract tests for the Azure DevOps endpoints + policy branches.

**Files:** `internal/generators/native/enrich_azure.go` (+ test).
**Scope:** M. **Dependencies:** T127.

---

## Phase 3 — Raw-HTTP platform clients (deferred)

**Not scheduled.** Replacing `gh` / `glab` with direct `net/http` platform clients — which
would let heraut drop those binaries entirely and reach a fully self-contained generator +
publisher — is **explicitly deferred behind its own ADR** (see ADR-0032, "Phase 3"). It
reimplements asset upload, pagination, and rate-limit handling the CLIs currently absorb, and
shifts ongoing API-churn maintenance onto heraut. Listed here only so the epic's full arc is
visible; do **not** start it without a follow-up ADR.
