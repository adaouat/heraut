# Héraut — Native Content Generator Roadmap

> Status: Active
> ADRs: [ADR-0032](../adr/0032-native-content-generator.md) (generator) · [ADR-0033](../adr/0033-native-config-model.md) (config model) · [ADR-0034](../adr/0034-native-remote-enrichment.md) (enrichment)
> Main roadmap: tracked as Phase 23 in [`roadmap.md`](roadmap.md)

This roadmap breaks down the **native content generator** epic into incrementally shippable
tasks. It lives in its own file because the work is heavy and multi-phase; the main roadmap
keeps only a Phase 23 pointer here.

`native` (`generator: native`) renders changelogs / release notes in pure Go, with no external
`git-cliff` binary. Per [ADR-0033](../adr/0033-native-config-model.md) it is heraut's
**canonical** renderer — git-cliff is dropped as the design anchor (the git-cliff *package*
removal is sequenced after native enrichment, in a follow-up ADR), and rendering is **driven
by config**: a unified `commits:` block (the single source of truth for commit types, also
consumed by `heraut commit verify`/`create`) plus a `rendering:` block. git-cliff's output is
no longer a parity target — heraut's rendering is its own spec, validated by golden snapshots.

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

| Phase                                                | Tasks                  | Status      |
|------------------------------------------------------|------------------------|-------------|
| Phase 1 — config model + native canonical renderer   | T122–T126, T130–T136   | Complete    |
| Phase 2 — remote enrichment (GitHub/GitLab CLI, Azure HTTP) | T127, T128, T137, T129 | Complete    |
| Phase 2.6 — native ↔ git-cliff parity (prereq for 2.5) | T138 – T141            | Complete    |
| Phase 2.7 — unified enrichment model                 | T142 – T148            | Complete    |
| Phase 2.8 — user-customizable templates (ADR-0037)   | TT1 – TT11             | Complete    |
| Phase 2.9 — incremental changelog (ADR-0038)          | —                      | Complete    |
| Phase 2.10 — commit-author attribution (ADR-0039)    | T151 (follow-up)       | Complete — GitHub, GitLab, Azure |
| Phase 2.5 — remove the git-cliff package (own ADR)   | T177–T194              | Done        |
| Phase C — wizard simplification (supersedes T164)    | T195–T202              | Done        |
| Phase D — infra housekeeping (Dockerfile / mise / ADR-0016) | T205–T208       | Active      |
| Phase 3 — raw-HTTP clients (drop `gh` / `glab`)       | —                      | Deferred    |

---

## Phase 1 — Config model + native canonical renderer (no remote enrichment)

Goal: `generator: native` as heraut's canonical renderer, **driven by config** (ADR-0033) —
full conventional-commit taxonomy + grouping from `commits.types`, `rendering.excludes`
filtering, commit / compare links from the resolved `port.LinkContext` — but **without** PR
author / number or contributors (Phase 2). No git-cliff parity target; heraut's output is its
own spec (golden snapshots).

Reshaped per [ADR-0033](../adr/0033-native-config-model.md) (2026-06-28): T122 stands;
T123/T124 landed but are reworked to read config (T132/T133); two foundation tasks
(T130/T131) land the `commits` / `rendering` config model first. New package
`internal/generators/native/` (implements `port.Generator`).

### Foundation — config model (ADR-0033)

#### `[x]` T130: `commits` + `rendering` config model

Replace `commit_lint` and the top-level `tickets:` / `remote_metadata:` keys with two
heraut-native blocks. `commits:` — `types` (list: `name`/`order`/`render`/`remove`,
deep-merged over a built-in default type set), `scopes`, `scopes_restricted`, `tickets`,
`remote_metadata`, `types_heading_level` — is the single source of truth for commit types.
`rendering:` — `excludes` (`{type}` / `{regex}` filters dropping commits from output).
Built-in defaults merge **under** minimal user config (no user `includes:`). Hard cutover
(pre-v1.0): the old keys become parse errors.

**Implementation:** `internal/config/config.go` (structs), the loader (strict — old keys
error), an internal-defaults deep-merge, `internal/config/validator.go`, `schema.json`,
`docs/heraut.sample.yml`. Migrate every consumer of `CommitLint` / `Tickets` /
`RemoteMetadata` (app propagation to `ContentDriver`, gitcliff, commit tooling) to the new
locations so the build stays green; git-cliff keeps working, reading `tickets` /
`remote_metadata` from `commits.`.
**Tests:** parsing (new shape + old-key rejection), the types deep-merge over defaults
(add / override / `remove`), `scopes_restricted`, `rendering.excludes`, validator rules,
schema fixtures.
**Scope:** L. **Dependencies:** ADR-0033.

**Completion note (2026-06-28):** Landed across two commits — `EffectiveTypes` merge core
(`a0ab7db`), then one atomic breaking commit over ~30 files (config structs + ~8 consumers +
validator + `schema.json` + 4 testdata fixtures + sample + ~10 test files). `Config` gains
`Commits` / `Rendering`; the old `CommitLint`, top-level `Tickets` and `RemoteMetadata` are
removed, with nil-safe `(*Config).Tickets()` / `.RemoteMetadata()` accessors minimizing
read-site churn. The strict loader now rejects the old keys (hard cutover). **Semantics
change:** `commits.types` *merges* over the built-in defaults by name (was: `commit_lint.types`
*replaced*) — to narrow the verify allow-list you `remove:` defaults; the affected
verify/validator tests were rewritten and ADR-0033 records the change. Default types = the
ADR-0027 verify 10, render-labeled; `revert` / security body-rule / catch-all "Other" remain
rendering-only (T132/T134). New validation: `rendering.excludes` (exactly one of type/regex,
regex compiles) and `scopes_restricted` requires a non-empty `scopes`. Full suite 1285 green.
Deferred to a follow-up `docs` commit: specs 02/03/05 prose. git-cliff stays functional,
reading `tickets` / `remote_metadata` from their new `commits.` home.

#### `[x]` T131: migrate `heraut commit verify` / `create` to `commits.types`

Per ADR-0033 (supersedes [ADR-0027](../adr/0027-builtin-conventional-commit-checker.md)'s
`commit_lint` surface). The verify allow-list becomes the effective `commits.types` (built-in
defaults plus user overrides, minus `remove: true`). `scopes_restricted: true` newly makes
`verify` reject scopes outside `commits.scopes`. The `create` wizard reads `commits.scopes` /
`commits.types`.

**Implementation:** `internal/app` (`AllowedCommitTypes`, `VerifyCommit`),
`internal/commitwizard`, plus cmd wiring. **Tests:** allow-list derivation incl. `remove`;
`scopes_restricted` accept / reject; wizard type / scope sourcing.
**Scope:** M. **Dependencies:** T130.

**Completion note (2026-06-28):** Most of this landed inside T130 — the allow-list already
derives from `commits.types` (`AllowedCommitTypes` → `EffectiveTypes`), and the `create`
wizard already reads `commits.scopes` / `commits.types` (`configuredScopes` +
`AllowedCommitTypes`). T131 adds the one remaining new behavior: `scopes_restricted: true`
enforcement in `VerifyCommit` (`verifyScope` — a commit with no scope is always allowed; a
present scope must be in `commits.scopes`). The validator already rejects `scopes_restricted`
without a non-empty `scopes` list (T130). Full suite 1286 green.

#### `[x]` T135: object `commits.scopes` + type / scope descriptions

`commits.scopes` becomes a list of `{ name, remove?, description? }` objects (was `[]string`),
mirroring the `commits.types` shape but simpler — no built-in defaults (scopes are
project-specific), `remove` reserved for future config composition. `commits.types` gains a
`description`. Both `description`s feed the `heraut commit create` wizard pickers; the
built-in type descriptions move from a hardcoded wizard map into the default `commits.types`
so they become user-overridable.

**Completion note (2026-06-28):** Added `config.ScopeRule` + `EffectiveScopes` / `ScopeNames`
and `TypeRule.Description` (with descriptions on the default types). The wizard now reads
`config.EffectiveTypes` / `EffectiveScopes` and renders `optionLabel(name, description)` (the
old `commitTypeDescriptions` map and `typeOptionLabel` are gone); `verifyScope` uses effective
scope names; the validator validates scope names and uses effective scopes for the
`scopes_restricted` check. `schema.json`, sample, spec 02, and ADR-0033 updated. Full suite
green. **Scope:** M. **Dependencies:** T130, T131.

---

#### `[x]` T136: built-in default scopes (deps, deps-dev, release)

`commits.scopes` becomes merge-over-defaults (like `commits.types`): a small built-in set —
`deps`, `deps-dev`, `release` (dependabot/renovate + release-tooling conventions, aligned with
the default `rendering.excludes`) — is merged under user scopes, so `remove` now drops a
default (no longer a reserved no-op). `scopes_restricted` stays `false` by default, so the
defaults are wizard suggestions, not a gate.

**Completion note (2026-06-28):** `config.defaultScopes` + `EffectiveScopes` switched to the
same by-name merge as `EffectiveTypes`. `scopes_restricted: true` is now satisfied by the
defaults (error only when all defaults are removed and none added). Tests, sample, spec 02,
ADR-0033 updated. Full suite 1295 green. **Scope:** S. **Dependencies:** T135.

---

### Native rendering (config-driven)

New package: `internal/generators/native/` (implements `port.Generator`).

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

#### `[x]` T132: rework native taxonomy ← `commits.types` + `rendering.excludes`

Re-point T123's grouping so the taxonomy (group names, display order, which types skip) comes
from the effective `commits.types` + `rendering.excludes` instead of the hardcoded table in
`group.go`. The grouping / scope-sort algorithm survives; only the *source* changes.

**Tests:** grouping driven by a config fixture (custom `render` / `order`, a `remove`d type,
an `exclude` regex), plus the default-config path still matching today's taxonomy.
**Scope:** M. **Dependencies:** T130, T123.

**Completion note (2026-06-28):** `groupCommits` is now config-driven —
`groupCommits(commits, userTypes, userExcludes)`: type groups from
`config.EffectiveTypes(userTypes)`, drops from `config.EffectiveExcludes(userExcludes)` (new
`config.EffectiveExcludes`; built-in default excludes = `chore(release|deps|pr|pull)`).
Classification shifted from subject-regex to **exact parsed type**, so `commits.types` names
drive the groups; the built-in security body-rule, `revert`, and catch-all **Other** are
preserved as renderer-owned fallbacks (unmatched / non-conventional commits are never
dropped). Deliberate default-output changes (parity dropped): `build` → its own "Build" group
(was Other), a non-type subject like `doc:` → Other, and the security body-rule fires only for
unmatched-type commits. `group_internal_test.go` reworked with config-customization cases.
Full suite 1289 green. **Note:** because the catch-all / revert / security built-ins are
preserved here, T134's catch-all is already in place — T134 narrows to confirming + testing
the `revert` / security body-rule rendering rules.

---

#### `[x]` T133: rework native render ← config

Re-point T124's renderer so type-section labels, order, and heading level come from `commits`
(`render`, `order`, `types_heading_level`) and excluded commits are filtered per
`rendering.excludes`. Templates stay internal (no public template engine yet — deferred).
T124's three git-cliff "deviations" become heraut's canonical choices.

**Tests:** golden snapshots over config fixtures (custom labels / heading level, excludes).
**Scope:** M. **Dependencies:** T132, T124.

**Completion note (2026-06-28):** Most of T133 already landed in T132 — group section labels
and order come from `commits.types` (the groups carry config-derived names / order) and
`rendering.excludes` filtering happens at grouping. The remaining piece, **`types_heading_level`**,
is wired here: a `headingPrefix(level)` helper (default 3 → `###`) feeds
`changelogView` / `notesView.HeadingPrefix`, and the still-dumb templates use
`{{ $.HeadingPrefix }}` for group headings and the sibling "Commit Statistics".
`renderChangelogSection` / `renderReleaseNotes` gained a `typesHeadingLevel int` param (T125
supplies `cfg.Commits.TypesHeadingLevel`). Default golden output unchanged; a level-2 test
locks the override. Full suite 1296 green.

---

#### `[x]` T134: catch-all "Other" group for unmatched / non-conventional commits

When a project did not use conventional commits from the start — or a commit's type is not in
the effective `commits.types` — those commits must still appear in the changelog under a
catch-all **"Other"** section rather than being silently dropped (mirroring git-cliff's
catch-all). The `revert` group and the `security` body-rule are related rendering-only rules
in the same category — preserve them here too. These are **rendering rules independent of the
`commits.types` allow-list**: adding them as allow-list types would (incorrectly) make them
valid for `heraut commit verify`.

**Tests:** a non-conventional commit and an unmatched-type commit both render under "Other";
revert / security body-rule rows. **Scope:** S. **Dependencies:** T132, T133.

**Completion note (2026-06-28):** Subsumed by T132 and confirmed by T126. The catch-all
"Other" group, the `revert` group, and the security body-rule are built-in renderer fallbacks
in `group.go` (T132) — unmatched / non-conventional commits land in Other, `revert:` in Revert,
a body matching `.*security` in Security — none are `commits.types` allow-list entries (which
would change `commit verify`). Covered by `group_internal_test.go` (security-body, unknown-type
/ non-conventional → Other) and asserted on real commits by the T126 integration test
(Other + Revert). No separate implementation was needed.

---

#### `[x]` T125: wire `generator: native` as the canonical generator + config / schema / docs

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

**Completion note (2026-06-28):** `generator: native` is selectable end-to-end.
`internal/generators/native/generator.go` implements `port.Generator`: `Check`/`Validate`
trivial (no external binary; user templates deferred), `Degraded()` false (Phase 1), and
`Generate` orchestrating `collectCommits → groupCommits → render`. Release-notes mode returns
the latest release's notes; changelog mode regenerates the **full** `CHANGELOG.md` (a section
for the unreleased tag + one per existing tag, newest-first) and writes it to `cfg.Output`.
The app layer wires it: `buildGenerator` `case "native"` (+ `nativeMode` mapping), and
`withEnvDerivations` propagates `commits.types` / `rendering.excludes` / `types_heading_level`
onto the `ContentDriver` (beside the existing `tickets` / `remote_metadata`). The validator
accepts `generator: native` and now allows `tickets` with it; `schema.json` enum, sample
(native is the recommended default), and spec 05 updated. New `listTags` / `commitRange` /
`releaseDate` helpers; contract tests for both modes + a schema fixture. **Deferred (noted):**
per-env tag-glob scoping (native lists all tags — fine for the common semver case) and the
release-notes "days between releases" stat (needs the previous tag's date). Full suite 1303
green.

---

#### `[x]` T126: canonical golden snapshots (heraut's own output spec)

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

**Completion note (2026-06-28):** Re-scoped per ADR-0033 (parity dropped): instead of diffing
against real `git-cliff --offline`, this is a **real-git integration test**
(`internal/generators/native/integration_test.go` — `gitFixture` + the real `execadapter`
runner) that builds a deterministic two-release repo and runs the actual native generator.
It locks native's output **structurally** (group headings, descriptions, the catch-all Other
+ Revert built-ins, and the default `chore(deps)` exclusion) for both changelog and
release-notes modes — robust to real hashes/dates, which T124's synthetic byte-exact goldens
already cover. The fixture is hermetic (`GIT_CONFIG_GLOBAL`/`SYSTEM=/dev/null`) and skips when
git is absent. Also smoke-tested manually against this repo (91 releases). Full suite 1305 green.

---

## Phase 2 — Remote enrichment via platform CLIs

Goal: add PR-number / author / first-time-contributor / linked-issue enrichment by calling
`gh api` / `glab api` through the existing `port.Runner` (the same path already used for
auth checks in `internal/platforms/`). Gated by the existing `remote_metadata` policy
([ADR-0023](../adr/0023-remote-metadata-policy.md)) and surfaced through the existing
`Degraded()` signal. Contract-tested with `MockRunner` — **no real network**.

#### `[x]` T127: GitHub enrichment via `gh api` (batched)

Enrich every rendered commit with its PR (number, author handle, link) and first-time-
contributor status via a **batched** `gh api graphql` fetch (`associatedPullRequests`,
paginated — bounded, not per-commit; ADR-0034 §4), correlate to the rendered commits, fold
into the T124 view model, and populate the `### New Contributors` block. Enrich **all**
releases in both modes (ADR-0034 §5). Honour `remote_metadata`: `required` (fatal on failure),
`optional` (fall back to bare output + set `Degraded()`), `disabled` (never call). Auth rides
`LinkContext.Token` / `BaseURL` — **do not import `internal/platforms`** (layer rule); lift a
helper onto `port.LinkContext` if sharing is needed.

**Tests:** contract tests (`MockRunner`) queueing GraphQL/API JSON, asserting the exact
`gh api` argv + env, the commit↔PR correlation, and each `remote_metadata` branch incl. the
degraded fallback and malformed-JSON.

**Files:** `internal/generators/native/enrich_github.go` (+ test); view-model + render
updates for author / contributors; possibly `port.LinkContext.APIEnv()`.
**Scope:** L. **Dependencies:** T125. **Design:** [ADR-0034](../adr/0034-native-remote-enrichment.md).

**Completion note (2026-06-30):** Delivered in two slices. **T127a** — `enrich_github.go`:
`enrichGitHub` runs batched `gh api graphql` (aliased `associatedPullRequests`, ≤50 SHAs/chunk)
→ `map[string]prInfo{Number,URL,AuthorLogin,FirstTimer}`; `port.LinkContext.APIEnv()` builds the
`gh` auth env (`GH_TOKEN` + `GH_HOST` for GHES). Auth rides the `LinkContext` — native never
imports `internal/platforms`. **T127b** — `enrich.go` adds the platform-dispatch seam (`enrich`)
+ `enrichForRelease` applying the `remote_metadata` policy (`disabled` skips / `required` fatal /
`optional` degrades with a real `Degraded()` + warn-once); `render.go` threads `enrichment` like
`tickets` — `by @login in [#N](url)` after the hash link, and a `### New Contributors ❤️` block
from first-timers; golden output unchanged (nil enrichment). **Enrichment scope (refined in the final review):**
release-notes fully enriched; **changelog enriches only the new/unreleased section** (historical
sections render from git alone), keeping a full regeneration O(1) API calls — the original
per-release approach would cost O(releases) in CI (the ambient `LinkContext` carries a platform
even without a token; see ADR-0034 §5). The cross-release batched fetch (§4) stays deferred. Execution: T127a Sonnet
impl + Sonnet review + Opus nit-fix; T127b Opus **inline** (after two subagent infra failures —
a stall and a connection-close, each leaving a clean tree) + Sonnet review + Opus fix-pass
(warn-once changelog test, ticket-ordering test, comment/doc nits). Suite 1328 green; the
deferred "days between releases" stat remains a small follow-up.

---

#### `[x]` T128: GitLab enrichment via `glab api` (batched)

The GitLab equivalent of T127 — a **batched** `glab api` fetch of merged MRs
(`projects/{id}/merge_requests?state=merged`, paginated), correlated to commits by
`merge_commit_sha` / `squash_commit_sha` (ADR-0034 §4), with the same `remote_metadata`
gating, `Degraded()` behaviour, and `LinkContext`-based auth as T127. MR link shape (`!`,
`/-/merge_requests/`) already lives in Go (ADR-0022).

**Tests:** contract tests (`MockRunner`) for the GitLab endpoints + correlation + each policy branch.

**Files:** `internal/generators/native/enrich_gitlab.go` (+ test).
**Scope:** M. **Dependencies:** T127. **Design:** [ADR-0034](../adr/0034-native-remote-enrichment.md).

**Completion note (2026-06-30):** `enrich_gitlab.go` — `enrichGitLab` resolves each commit's MR
via per-commit `glab api projects/{id}/repository/commits/{sha}/merge_requests`. **Deviation from
ADR-0034 §4 (batched):** GitLab has no GitHub-style batched `associatedPullRequests` primitive,
and the merged-MR correlation approach is fragile (misses merge-workflow feature commits, needs
pagination bounds); the per-commit endpoint is the clean, correct primitive (bounded by the
release's commit count). `prInfo` gains `RefPrefix` so MRs render `!N` (GitHub PRs stay `#N`;
empty defaults to `#`); `port.LinkContext.APIEnv()` gains the GitLab branch (`GITLAB_TOKEN` +
`GITLAB_HOST` for self-managed; host helper generalised to `nonDefaultHost`); `enrich`'s `gitlab`
case dispatches to `enrichGitLab`. **First-timer detection deferred** (GitLab's API has no
`authorAssociation`), so GitLab gets the `by @author in !N` suffix but no New Contributors block —
a follow-up. Auth still rides `LinkContext` (no `internal/platforms` import). Contract tests cover
the glab argv/env, parsing, no-MR, error, and the end-to-end `!N` render. Done inline (Opus) per
the user's call; Sonnet review to follow. Suite green.

---

#### `[x]` T137: GitLab "New Contributors" block (first-time contributors)

Closes the first-timer gap deferred in T128. The renderer already emits the
`### New Contributors ❤️` block from `prInfo.FirstTimer` (shared with GitHub) — GitLab just
never sets it, because GitLab's MR API has no `authorAssociation`. Per
[ADR-0034](../adr/0034-native-remote-enrichment.md) §4, a GitLab author is a first-timer when
they do **not** appear in any earlier release's MRs. Realise that with one bounded `glab api`
call per **distinct** release author:
`projects/{id}/merge_requests?author_username={login}&state=merged&order_by=created_at&sort=asc&per_page=1`
returns the author's earliest merged MR; the author is a first-timer iff that MR's `iid` is not
less than the smallest MR `iid` they have in this release (i.e. their first-ever merged MR *is*
in this release). Calls are bounded by distinct authors (small), not commit count.

Gated by the same `remote_metadata` policy as the MR fetch (ADR-0034 §6): a failed first-timer
query propagates and is handled by `enrichForRelease` (`required` → fatal, `optional` →
degrade all enrichment). `RefPrefix: "!"` is preserved so the block renders `[!N]`.

**Tests:** contract tests (`MockRunner`) for the earliest-MR argv/env, first-timer vs
returning-contributor, multi-author dedup, and the end-to-end `### New Contributors` block with
`[!N]`.

**Files:** `internal/generators/native/enrich_gitlab.go` (+ test).
**Scope:** S. **Dependencies:** T128. **Design:** [ADR-0034](../adr/0034-native-remote-enrichment.md).

**Completion note (2026-07-02):** `enrich_gitlab.go` gains `markGitLabFirstTimers` (called
after the per-commit MR map is built) and `gitLabEarliestMergedMR`. For each **distinct** MR
author it issues one `glab api merge_requests?author_username={login}&state=merged&order_by=created_at&sort=asc&per_page=1`,
and sets `FirstTimer` on that author's `result` entries when their earliest merged MR `iid` is
`>= min(their release MR iids)` — i.e. their first-ever merged MR is in this release. **Realises
ADR-0034 §4's "author appears in any earlier release's MRs" as a per-author earliest-MR lookup**
(the concrete query shape the ADR left to implementation; T128 had already deviated from the
batched merged-MR list to a per-commit fetch, so the full corpus isn't otherwise available).
Calls are bounded by distinct authors, not commits, and an author on multiple commits is queried
once. `earliest == 0` (no merged MR returned) is treated conservatively as not-new. Failures
propagate through `enrich` → `enrichForRelease`, so the `remote_metadata` policy is applied
uniformly (`required` fatal, `optional` degrades all enrichment) — no partial-degrade path was
added. The renderer (`buildContributors`, template) was already generic, so no render change was
needed; `RefPrefix: "!"` flows through and the block renders `[!N]`. Contract tests cover the
earliest-MR argv/env, first-timer vs returning, single-query dedup, multi-author sorted order,
query-error propagation, and the end-to-end `### New Contributors ❤️` block. GitHub was
unaffected. Suite 1350 green. **Deferred:** best-effort first-timer degrade (keep `by @author`
when only the first-timer lookup fails) — not added to preserve ADR-0034 §6's single error model.

---

#### `[x]` T129: Azure DevOps enrichment via a native `net/http` client (optional)

Bring the native path to ADR-0026 parity — Azure DevOps PR / author enrichment. Per
[ADR-0035](../adr/0035-azure-enrichment-native-http.md) (which **supersedes ADR-0034 §3's `az`-CLI
choice**): a thin native `net/http` client, **not** the `az` CLI (bundling `az` = bundling Python
in the Docker image, and heraut does not already require `az` — net-new weight for the lowest-value
platform). One batched `POST {base}/{org}/{project}/_apis/git/repositories/{repo}/pullrequestquery?api-version=7.1`
with `{"queries":[{"type":"lastMergeCommit","items":[<release SHAs>]}]}`, correlating
`results[0][sha] → PR`. Auth: `Authorization: Basic base64(":"+LinkContext.Token)` (PAT from
`AZURE_DEVOPS_TOKEN` via ADR-0026 — no new env var). Reuse `azureRepoRoot` for the PR web URL;
`RefPrefix "!"`. Routed through the `enrich` / `enrichForRelease` seam so `remote_metadata` +
`Degraded()` behave like GitHub/GitLab. **Optional** — do only when Azure attribution is wanted.
**First-timer detection deferred** (Azure PRs have no `authorAssociation`; parity follow-up, like
GitLab pre-T137). Sequence last.

**Tests:** `httptest.Server` contract tests (assert method, path, api-version, `Authorization`
header, request body; canned JSON) + correlation + each `remote_metadata` policy branch. No network.

**Files:** `internal/generators/native/enrich_azure.go` (+ test); `generator.go` gains an
`*http.Client`; `enrich.go` gains the `azure_devops` dispatch case.
**Scope:** M. **Dependencies:** T127. **Design:** [ADR-0035](../adr/0035-azure-enrichment-native-http.md),
[ADR-0034](../adr/0034-native-remote-enrichment.md).

**Completion note (2026-07-03):** Implemented via ADR-0035's native `net/http` path (the `az`-CLI
reversal is recorded there). `enrich_azure.go` — `enrichAzure(client, lc, shas)` issues one batched
`POST …/pullrequestquery?api-version=7.1` with `type: lastMergeCommit`, correlates `results[0][sha]
→ prInfo` (first PR wins), composing the PR web URL via `azureRepoRoot` + `/pullrequest/{id}` and
`RefPrefix "!"`. Auth is `Authorization: Basic base64(":"+lc.Token)` (PAT from `AZURE_DEVOPS_TOKEN`);
`org`/`project` split from `lc.Owner`. Non-2xx / transport / decode errors wrap `pullrequestquery`
so `enrichForRelease` applies the policy unchanged. `generator.go` gained an `*http.Client`
(30s timeout, used only by this path); `enrich.go` dispatches `azure_devops`. **Zero new deps
(stdlib only).** Tested with `httptest.Server` (the HTTP analog of MockRunner): request contract
(method/path/api-version/`Authorization`/body), PR mapping, no-PR-absent, `uniqueName` local-part
vs `displayName` fallback, non-2xx error, malformed JSON, empty-shas-no-call, and the end-to-end
`by @jane in [!42](…)` render. **Author handle:** `uniqueName` local-part before `@`, else
`displayName`. **First-timer deferred** (Azure PRs have no `authorAssociation`) — no New Contributors
block for Azure yet. `port.LinkContext.APIEnv()` stays `nil` for azure_devops (unused). Suite 1357
green. **This completes Phase 2.**

---

## Phase 2.6 — Native ↔ git-cliff parity (prerequisite for Phase 2.5)

Gaps that keep native from being a full git-cliff replacement — so they gate the git-cliff
package removal (Phase 2.5). Prioritised order below. **User-customizable templates** (the last
git-cliff-only feature) is intentionally excluded here: it gets its own brainstorm/ADR after
these land and are validated on a real repo.

#### `[x]` T138: native per-env tag scoping

The one thing blocking "native for all strategies": the validator (`validateNativePerEnv`)
rejects native under `*-per-env` strategies because native's `listTags` / `previousTag` walk all
tags regardless of environment. The scoping plumbing already exists — both helpers accept a git
glob, and `tagfmt.GlobPattern(tag_format, env)` derives the per-env glob (e.g. `*/prod`) — native
just passes `""`. native can't import `tagfmt` (layer rule), so the app layer computes the glob.

**Approach:** add app-set `ContentDriver.TagGlob` (`yaml:"-"`, computed in `withEnvDerivations` via
`tagfmt.GlobPattern` when the tag format has `{env}`, like the existing `TagPattern`/`Types`
propagation); native's `generateReleaseNotes` / `generateChangelog` pass `g.cfg.TagGlob` to
`previousTag` and `listTags`; **remove `validateNativePerEnv`**. Non-per-env keeps `TagGlob == ""`
(all tags — unchanged behaviour).

**Tests:** contract tests that native passes the glob to `git tag -l <glob>` / `git describe
--match <glob>`; app-layer test that `withEnvDerivations` sets `TagGlob` for a per-env strategy and
leaves it empty otherwise; validator test that native + `*-per-env` is now accepted; spec 05 update.
**Scope:** M. **Dependencies:** none.

**Completion note (2026-07-03):** Landed exactly as planned — it was a wiring task, the scoping
primitives already existed. Added `ContentDriver.TagGlob` (`yaml:"-"`, app-computed);
`withEnvDerivations` derives it via `tagfmt.GlobPattern(tf, env)` **only** when the format carries
`{env}` and env is non-empty and the user set no explicit `TagPattern` (so non-per-env keeps
`TagGlob == ""` = all tags, unchanged). `generator.go` now passes `g.cfg.TagGlob` to `previousTag`
(→ `git describe … --match <glob>`) and `listTags` (→ `git tag -l <glob>`). Removed
`validateNativePerEnv` entirely and flipped its test to `TestValidate_NativePerEnvAccepted`. Spec 05
updated (also corrected a stale "no remote enrichment" line now that Phase 2 shipped). Native is now
valid for **all four strategies**. Suite 1361 green.

#### `[x]` T139: native explicit `tag_pattern` support

Today `tag_pattern` requires git-cliff (`validator.go`). Allow it with native and have native honour
it. **Open decision:** native `tag_pattern` as a **glob** (simple, matches native's glob plumbing)
vs **regex** (drop-in git-cliff parity, filter tags in Go). An explicit `tag_pattern` overrides the
T138 auto-derived glob (`withEnvDerivations` already suppresses auto-derivation when the user sets
one). Update validator + `schema.json` + `docs/heraut.sample.yml` + spec.
**Scope:** S–M. **Dependencies:** T138.

**Completion note (2026-07-03):** Decision (user): **regex**, so the same `tag_pattern` value means
one thing across generators. Native scoping precedence is now `TagGlob` (T138 per-env auto glob) →
explicit `TagPattern` (Go regex, in-process filter) → all tags; the two never collide because
`withEnvDerivations` suppresses the auto glob when the user sets `tag_pattern`. New pure helpers
`filterByTagPattern` (regexp filter) and `previousInList` (predecessor from a scoped, newest-first
list, incl. the "new release not yet tagged → newest existing" case); `generator.go` routes through
new `scopedTags` / `scopedPreviousTag` (glob/regex/unscoped). Validator now accepts `tag_pattern`
with native and rejects a non-compiling regex. Corrected the long-standing docs bug that described
`tag_pattern` as a *glob* with glob-shaped examples (`dev/*`, `v[0-9]*`) — it is a regex for
git-cliff too; schema/sample/spec now say regex with anchored examples (`^v[0-9]`, `^prod/`).
Kept T138's glob path (it is an equivalent matcher to the auto regex, and git-side `git tag -l
<glob>` is cheaper) rather than reworking it. Suite 1366 green.

#### `[x]` T140: native "days between releases" stat

Deferred in T125/T127. `generateReleaseNotes` passes `time.Time{}` for `prevReleaseDate`, so the
`renderReleaseNotes` `DaysSincePrev` / `HasDaysSincePrev` path (already built) stays dormant.
**Approach:** resolve the previous tag's commit date (`git log -1 --format=%cI <prev>`) and pass it.
**Tests:** contract test for the date resolution + a render assertion that the stat line appears.
**Scope:** S. **Dependencies:** none.

**Completion note (2026-07-03):** Added `tagDate(runner, tag)` (`git log -1 --format=%cI <tag>`,
%cI matches the commit collector) and wired `generateReleaseNotes` to resolve the previous tag's
date and pass it as `prevReleaseDate` (only when a previous tag exists — first releases keep the
zero time, so the stat is omitted). The `renderReleaseNotes` `DaysSincePrev` / `HasDaysSincePrev`
path was already built, so no render change. Cost: one extra `git log -1` per release-notes run
with a predecessor, which rippled through the 9 release-notes contract tests (shared
`queueReleaseNotesGit` helper updated once; the rest inline) — mechanical, no assertions dropped.
Suite 1367 green.

#### `[x]` T141: Azure "New Contributors" block — **reconsider**

**git-cliff has no Azure first-timer logic** (verified against `git-cliff-core/src/remote/azure_devops.rs`:
it attributes authors via `created_by.display_name`/`unique_name` but computes no first-time status).
So this **cannot mirror git-cliff** — matching git-cliff means Azure has attribution (`by @a in !N`,
already shipped in T129) but *no* contributors block. Doing it anyway would be net-new, GitLab-T137
style: per distinct PR author, query their earliest PR (`pullrequests?searchCriteria.creatorId={guid}`)
and mark first-timer when it falls in this release — more API calls, lowest-value platform.
**Decision pending:** implement net-new, or drop/defer. Sequence last.
**Scope:** M. **Dependencies:** T129.

**Resolution note (2026-07-03):** Resolved by Phase 2.7 (below), not implemented as originally
scoped here. The unified enrichment model (ADR-0036) computes `first_time` from git author-email
history — a **local, platform-agnostic** signal independent of any platform's API — so Azure gets
the "New Contributors" block automatically, with **no per-author Azure query** and no net-new API
surface. This also retires the "implement net-new vs. drop" choice above: neither GitLab's T137
per-author approach nor a from-scratch Azure equivalent is needed going forward (T137 itself was
removed in T145).

---

## Phase 2.7 — Unified enrichment model

Goal: replace native's three divergent per-platform first-timer mechanisms (GitHub
`authorAssociation`, GitLab's T137 earliest-MR query, Azure's none/T141) and the ad hoc `prInfo`
struct with **one two-tier model** — a git-derived local tier (`authorsBefore` /
`collectContributors`, `first_time` from git author-email history) plus a normalized remote tier
(`Author`/`PullRequest`/`Contributor`, `Title`/`Labels` common fields, a `Platforms` escape hatch).
Design: [ADR-0036](../adr/0036-unified-enrichment-model.md), which supersedes the first-timer
portions of [ADR-0034](../adr/0034-native-remote-enrichment.md). Plan:
`docs/superpowers/plans/2026-07-03-unified-enrichment-model.md`.

#### `[x]` T142: rename `prInfo` → `PullRequest` in a new `model.go` (behavior-neutral)

**Completion note (2026-07-03):** Mechanical rename, additive fields. `internal/generators/native/model.go`
now holds `Author`, `PullRequest` (renamed from `prInfo`, gaining unused `Title`/`Labels`/`Platforms`
and a transitional `FirstTimer` kept for this commit only), and `Contributor`. Every `prInfo`
reference across `enrich_github.go` / `enrich_gitlab.go` / `enrich_azure.go` / `enrich.go` /
`render.go` (and their tests) became `PullRequest`. No behavior change. **Scope:** S.

#### `[x]` T143: local git tier — `authorsBefore` + `collectContributors`

**Completion note (2026-07-03):** New `internal/generators/native/contributors.go`.
`authorsBefore(runner, prev)` issues one `git log <prev> --format=%ae` and returns the set of
author emails reachable from `prev`; an empty `prev` (first release) short-circuits with no git
call. `collectContributors(commits, before, prs)` returns the release's distinct first-time
contributors (dedup by email, first-seen order), overlaying the PR (handle/number/URL) from the
first commit seen for that author's email — the email is marked seen immediately, so if that
first commit has no PR, none is attached even when a later commit by the same author does.
Purely additive — nothing called it yet. **Scope:** M.
**Dependencies:** T142.

#### `[x]` T144: render New Contributors from the git-based local tier

**Completion note (2026-07-03):** `render.go`'s old enrichment-scanning contributors helper was
replaced by `buildContributorViews([]Contributor)`, which renders a contributor line only when
`Author.Username != ""` (a remote handle was resolved) — so offline built-in output is byte-identical
to before. `generateReleaseNotes` (`generator.go`) now calls `authorsBefore` + `collectContributors`
after `enrichForRelease` and threads `[]Contributor` into `renderReleaseNotes`; the changelog path
(`renderChangelogSection`) has no contributors block and was untouched. This adds one `git log
<prev> --format=%ae` call per release-notes generation with a predecessor — rippled through the
`MockRunner`-based release-notes contract tests (one more queued response each), mirroring the
T140 `tagDate` ripple. No golden-snapshot fixture under `testdata/` renders the New Contributors
block, so no re-baselining was required in practice — the design's re-baseline-and-review guidance
was moot for this repo's existing goldens. **Scope:** M. **Dependencies:** T143.

#### `[x]` T145: remove per-platform first-timer paths

**Completion note (2026-07-03):** `PullRequest.FirstTimer` deleted from `model.go`. GitHub:
`prFragment` and `graphQLPR` drop `authorAssociation`; `parseGitHubResponse` no longer derives
`FirstTimer`. GitLab: `markGitLabFirstTimers` and `gitLabEarliestMergedMR` (T137) deleted outright,
along with their per-author `merge_requests?author_username=…` calls and five now-obsolete tests
(`TestEnrichGitLab_FirstTimer*`, `_ReturningContributor`, `_FirstTimerQueryError`); `enrichGitLab`
no longer calls them. One local git computation now fully replaces the three platform-specific
mechanisms. **Scope:** S. **Dependencies:** T144.

#### `[x]` T146: fetch PR `title` + `labels` — GitHub

**Completion note (2026-07-03):** `prFragment` (GraphQL) extended to request `title` and
`labels(first:20){nodes{name}}`; `graphQLPR` gained `Title`/`Labels` fields; `parseGitHubResponse`
populates `PullRequest.Title`/`.Labels`. **Scope:** S. **Dependencies:** T145.

#### `[x]` T147: fetch MR `title` + `labels` — GitLab

**Completion note (2026-07-03):** `gitLabMR` gained `Title string` / `Labels []string` (GitLab's
merge-request response already carries both); `enrichGitLab`'s `PullRequest{...}` literal now sets
`Title`/`Labels` from the fetched MR. **Scope:** S. **Dependencies:** T145.

#### `[x]` T148: fetch PR `title` (+ labels best-effort) — Azure

**Completion note (2026-07-03):** `azurePR` gained `Title string` and a `Labels []struct{Name
string}` field; `enrichAzure`'s `PullRequest{...}` literal sets `Title`/`Labels` from the
`pullrequestquery` response (labels stay empty if a given response omits them — best-effort, no
extra `expand` request added). This completes Phase 2.7 — see
[ADR-0036](../adr/0036-unified-enrichment-model.md) for the full model. (The initial "contributors
over all commits" behavior was refined to rendered-commits scope in T149.) **Scope:** S.
**Dependencies:** T145.

#### `[x]` T149: native enrichment follow-ups (from Phase 2.7 final review)

Non-blocking polish items surfaced by the Phase 2.7 final whole-branch review
([ADR-0036](../adr/0036-unified-enrichment-model.md)). Four of five landed; the fifth is folded
into the future user-templates task (its offline consumer).

- **[x] First-PR-bearing-commit overlay** (`4647c1d`). `collectContributors` now overlays a
  first-timer's PR from their *first PR-bearing* commit (was: only the first commit seen), so a
  first-timer whose earliest commit is unlinked but a later one has a PR still renders online.
- **[x] Contributors from rendered commits** (`560cc71`). `renderedCommits` filters to commits that
  survive `rendering.excludes` before `collectContributors`, so a first-time bot `chore(deps)`
  (excluded) no longer surfaces in "New Contributors" — the celebrated set is the authors whose
  work is shown.
- **[x] Dead test helper + empty-email test** (`4647c1d`). Dropped the inert `association` param
  and `authorAssociation` JSON from `ghGraphQLResponse` (last remnant of the retired GitHub
  first-timer path); added an `email == ""` guard test for `collectContributors`.
- **[ ] Deferred — offline `authorsBefore` guard.** `generateReleaseNotes` runs `git log <prev>
  --format=%ae` even under `remote_metadata: disabled`, where the result is unused *today*. Left
  "always on" deliberately: the local tier is the source the future offline user-customizable
  templates will consume, so the guard (skip when `enrichment == nil`) belongs with that task once
  its offline needs are pinned — tracked there, not here.

**Completion note (2026-07-04):** landed across `4647c1d` + `560cc71`; #3 (rendered-commits scope)
was a user decision — "new contributors" are the authors whose work is shown. Full suite 1372 green.
**Scope:** S. **Dependencies:** Phase 2.7.

---

## Phase 2.8 — User-customizable templates (ADR-0037)

The last major native ↔ git-cliff parity feature: a public template API for the native generator.
Design spec: [`docs/superpowers/specs/2026-07-04-user-customizable-templates-design.md`](../superpowers/specs/2026-07-04-user-customizable-templates-design.md);
plan: [`docs/superpowers/plans/2026-07-04-user-customizable-templates.md`](../superpowers/plans/2026-07-04-user-customizable-templates.md).

- `[x]` **TT1–TT4** — PR review fields (`CreatedAt`/`MergedAt`/`MergedBy`/`Approvers`) on the
  normalized `PullRequest`, fetched per platform (GitHub GraphQL, GitLab MR, Azure PR). Approvers
  best-effort: GitHub + Azure only, empty on GitLab.
- `[x]` **TT5–TT6** — the public `tpl*` template model (`templatemodel.go`) + `buildRelease`
  builder bridging the internal render data onto the contract (reuses the render.go helpers).
- `[x]` **TT7** — `templateFuncs()` (`upperFirst`/`date`/`join`/`list`/`indent`/`trim`).
- `[x]` **TT8** — **the load-bearing rewrite:** built-in changelog / release-notes templates
  rewritten as named blocks over `tplRelease` (dogfooded). Built-in output is **byte-identical** —
  golden snapshots pass unchanged, no re-baseline. Fat-injection view models / line-builders
  deleted; `indent` is per-line; the release-notes body/footer tail wraps the shared `commit` block.
- `[x]` **TT9** — config: `rendering.templates` + per-driver `rendering`, deep-merged
  global → driver → env; app-computed `ContentDriver.EffectiveTemplates`. Schema + sample synced.
- `[x]` **TT10** — `buildTemplateSet` (built-in → inline snippets → `template` file precedence)
  wired through the render path; validator requires `generator: native` and parse-checks snippets +
  the template file.
- `[x]` **TT11** — end-to-end override tests (inline + file), ADR-0037, spec 05, schema, sample,
  this roadmap.

**Completion note (2026-07-06):** executed inline (subagents session-limited) with full TDD and the
golden byte-identity gate as the objective check for TT8. New PR fields are additive; the contract
ships **experimental in v1**. No new dependencies; layer rule held (the config validator parses
snippets with a stub func-map mirroring the native names rather than importing `native`). **Scope:**
L. **Dependencies:** Phase 2.7 (unified enrichment model).

---

## Phase 2.9 — Incremental changelog (ADR-0038)

`[x]` Give the native generator's changelog two modes so a generator switch (or any regular
release) no longer strips historical PR-author attribution. Design spec:
[`docs/superpowers/specs/2026-07-10-incremental-changelog-design.md`](../superpowers/specs/2026-07-10-incremental-changelog-design.md).

**Completion note (2026-07-10):** incremental splicing is now the default — only the new
release's section is rendered, enriched (O(1) API calls), and spliced past a structural
`<!-- heraut-release: <tag> -->` anchor (assembly-layer only, never a template block, so it stays
decoupled from the ADR-0037 customizable `header`), leaving every historical section verbatim. A
missing/empty file bootstraps a full build; a non-empty anchorless file (foreign, e.g.
`git-cliff`-produced) stops the run with an error naming `--regenerate`, file untouched.
`heraut changelog --regenerate` / `heraut release --regenerate-changelog` force a full rebuild
that re-enriches every section (batched on GitHub/Azure; a pipeline warning fires for the
per-commit GitLab cost). heraut's own CI migration is a one-time `regenerate_changelog`
`workflow_dispatch` input on `.github/workflows/release.yml` rather than a code change — dispatch
once with it checked to adopt `native` with full attribution, then leave it unchecked. Documented
in [ADR-0038](../adr/0038-incremental-changelog.md), Spec 05 (changelog structure & incremental
generation), and Spec 03 (both flags). **Scope:** M. **Dependencies:** Phase 2.7 (unified
enrichment model), Phase 2.8 (ADR-0037, for the anchor/header decoupling).

---

## Phase 2.10 — Commit-author attribution (ADR-0039)

`[x]` Credit the **commit author** — `by @<handle>` — on every native commit line, independent
of any associated pull request; the PR now contributes only its `in [#N](url)` reference link.
Closes the gap dogfooding surfaced: heraut's own direct-commit trunk (no PRs) rendered zero
attribution under native's previous PR-author-only model, where git-cliff always showed
`by @bchatard`. Design spec:
[`docs/superpowers/specs/2026-07-17-commit-author-attribution-design.md`](../superpowers/specs/2026-07-17-commit-author-attribution-design.md).

**Completion note (2026-07-18):** GitHub only, this cut. `enrich_github.go`'s existing batched
`gh api graphql` query (one call per ≤50 commit SHAs) gained `author{user{login}}` on the
`Commit` fragment, resolving a `sha → authorHandle` map at zero extra API calls; the map rides
`enrichForRelease` → `renderRelease`/`renderChangelogSection`/`renderReleaseNotes` and is stamped
onto each grouped commit by `overlayAuthorHandles` before `buildCommit` reads it into
`Author.Username`. `blocks.tmpl`'s `commit` block now renders `by @{{ .Author.Username }}`
unconditionally (was: gated on `{{ if .PR }}`, sourced from the PR author), with the PR
contributing only `in [{{ .PR.Ref }}](...)`. Committer ≠ PR author → committer wins, matching
git-cliff. Unlinked author (`author.user == null`) or offline → no `by @`, unchanged. Byte-identity
held for **every** golden — including `release_notes_contributors.golden`: its fixture's PR-author
login already equalled the overlaid commit-author handle (`alice`), so the source switch produced
identical bytes and no golden changed at all (the switch was proven instead by a dedicated test
using differing PR-author vs commit-author logins). GitLab and Azure resolve no commit-author
handle, so their commit lines now render no `by @` — they lose the previous PR/MR-author credit but
keep the `in [!N]` reference link — until the follow-up tasks below land. **Scope:** M. **Dependencies:** Phase 2.7 (unified enrichment model).

**Regression fix (2026-07-19):** v0.51.0 shipped the attribution feature but heraut's own
changelog still rendered zero `by @` in CI. Root cause was pre-existing (latent since the v0.50.0
native switch, invisible until this feature depended on it): `pipeline.ambientLinkContext()` set
`BaseURL` to the full `host/owner/repo` and left `Owner`/`Repo` empty, so native's
`buildGitHubQuery` addressed `repository(owner:"",name:"")` and the enrichment `gh api graphql`
call 404'd — degrading silently to no attribution. Fix: `ambientLinkContext` now sets `BaseURL` to
the host only and splits `Owner`/`Repo` from `GITHUB_REPOSITORY` (and, for GitLab, from
`CI_SERVER_URL` + `CI_PROJECT_PATH` via `splitProjectPath`, which keeps subgroups in the owner).
git-cliff's `linkEnv` composes the identical `{remote}` from `BaseURL`+`Owner`+`Repo`, so its links
are unchanged; the `CI_PROJECT_URL`-only branch remains a links-only GitLab fallback. Covered by
`TestAmbientLinkContext` (github/gitlab-split cases now assert populated `Owner`/`Repo`).

#### `[x]` T150: GitLab commit-author handle + MR refs (batched GraphQL)

GitLab's REST API cannot resolve an arbitrary commit-author email to a user (privacy-restricted).
Whether GitLab's **GraphQL** API can — and whether it can batch the way the GitHub query does —
is unconfirmed against the live schema. Start with a **schema spike**: use `glab api graphql`
introspection (or `/-/graphql-explorer`) to check whether `mergeRequests(commitSha: ...)`, a
commit-by-SHA field, or `Commit.author { username }` exist on the current GitLab schema, and
whether any of them can be queried for multiple SHAs in one round trip (mirroring GitHub's
aliased batch) rather than one call per commit. If a batchable path exists, extend
`enrich_gitlab.go`'s `sha → authorHandle` map the same way `enrich_github.go` does; if only a
per-commit path exists, weigh the added API cost against ADR-0038's GitLab full-regeneration
warning (already O(commits)) before deciding whether to ship it. **Scope:** M (spike) + S–M
(implementation, pending spike result). **Dependencies:** Phase 2.10 (GitHub cut).

**Completion note (2026-07-23):** The spike found no commit→MR field on GitLab's GraphQL schema,
but two independently batchable connection queries cover both gaps (ADR-0042): `commits(ref:,
committedAfter:)`, paginated, resolves `sha → author.username` for the `by @<handle>` credit —
GitLab's per-commit-email-lookup restriction doesn't apply here, since the query walks commits by
ref rather than looking up an email. `mergeRequests(state: merged, mergedAfter:)`, also paginated,
is inverted into a `commitSha → MR` map keyed by `mergeCommitSha` and every `commits.nodes.sha`
(merge-commit, squash-with-merge-commit, and fast-forward merges all land on one of those SHAs) for
the `in [!N]` reference plus MR review-metadata (`mergeUser`, `mergedAt`, labels, title). GitLab
exposes no squashed-commit SHA (only a `squashOnMerge` bool), so a squash+fast-forward merge
matches no commit and that commit renders no ref — a graceful, documented gap, not a bug. The old
per-commit `glab api projects/{id}/repository/commits/{sha}/merge_requests` REST call (T128) is
dropped entirely: GitLab moves from O(commits) to O(pages), and the ADR-0038 `--regenerate`
GitLab rate-limit warning no longer applies and was removed (`gitlabRegenWarning` deleted from
`internal/pipeline/warn.go`). Files: `internal/generators/native/enrich_gitlab.go` (+ tests).

#### `[x]` T151: Azure DevOps commit-author handle

Azure needs a separate identity lookup to map a commit's author email to an Azure DevOps
identity — there is no equivalent to GitHub's `author.user.login` riding the existing PR-fetch
call. Investigate the identity API (e.g. `_apis/identities` or the `pullrequestquery` response's
committer fields) for a batchable resolution path before implementing; until then `enrich_azure.go`
continues to return no author-handle data and Azure commit lines render no `by @`. **Scope:** M.
**Dependencies:** Phase 2.10 (GitHub cut).

**Completion note (2026-07-24):** A live spike proved Azure exposes no identity resolvable from a
git commit email — the Commits API carries only git `name`/`email`, and both `_apis/identities`
and Graph `subjectquery` returned no match for the author email. So the Azure commit-author handle
is rendered from the **local git author email local-part** (via the existing `azureAuthorLogin`,
the same rendering Azure PR authors use) — no new API call. It rides `enrich()` → `enrichForRelease`,
so it is gated by `remote_metadata` like GitHub/GitLab (absent under `disabled`/offline or a
degraded `pullrequestquery`). It is a text attribution, not a clickable Azure @mention (inherent to
Azure). **Scope:** S.

A platform's `sha → authorHandle` map (shipped for GitLab in T150 and Azure in T151)
could also feed the "New Contributors" block: that block's handle today is still overlaid from a
contributor's first PR/MR (unchanged by [ADR-0039](../adr/0039-commit-author-attribution.md) — see
the design spec's "out of scope" section), so the map could, as a further extension, also drive
first-timer credit for direct-commit contributors — noted here, not scheduled.

#### `[ ]` T153: GitLab enrichment ref-anchor — use the topological range tip

`enrichGitLab` anchors its `commits(ref:)` query on `newestSHA(commits)` — the commit with the
newest committer date (`%cI`). `collectCommits` runs `git log --reverse` with git's default date
ordering (not `--topo-order`), so this is the true range tip in normal history. Under **rewritten
committer dates** (rebase / amend / cherry-pick), an ancestor commit can carry a newer date than
the real range head, so the anchor may not have every range commit as an ancestor and those
commits' authors won't resolve → no `by @` (graceful — missing attribution, never wrong data; see
ADR-0042). A precise fix threads the actual range head as the ref — the tag for a historical
section, the HEAD commit SHA for the unreleased section (the new tag isn't on the remote yet) —
through `enrichForRelease` → `enrich`; alternatively `--topo-order` in `collectCommits` would make
`commits[len-1]` the true tip but reorders output for every consumer (golden-snapshot impact), so
it needs its own review. **Scope:** S–M. **Dependencies:** T150.

#### `[x]` T152: changelog.remote for native + base_url host override (ADR-0040)

`heraut changelog --regenerate` locally on self-hosted GitLab produced no links/attribution
because the changelog-only pipeline's link-context chain is `changelog.remote → ambient`,
and locally the block was rejected for native and `remoteLinkContext()` hardcoded gitlab.com.
Lifted the git-cliff-only gate on `changelog.remote` and replaced `api_url` with a unified
`base_url` host override across github/gitlab/azure_devops. Because `LinkContext.BaseURL`
already drives both links and `APIEnv()` host routing, no generator/enrichment plumbing
changed. Breaking (pre-v1.0): `api_url` removed. GitLab commit-author `by @` stays out of
scope (T150); an offline attribution fallback was deferred. **Scope:** S. **Dependencies:**
Phase 2.7 (unified enrichment), ADR-0026.

---

## Phase 2.5 — Remove the git-cliff package (own ADR)

> See `docs/superpowers/specs/2026-08-08-native-only-generator-design.md` for the full design
> (drops `communique` too, in a later phase of this same epic) and ADR-0045 (written as part of
> this phase) for the decision record.

Config cutover: `generator:` / `config:` under `changelog:` and `release.notes:` become removed
keys (hard error, no deprecation window — matching ADR-0028's cocogitto-removal precedent).
Native becomes implicit. `git-cliff`/`communique` package deletion, the `heraut cliff` command,
and wizard simplification are separate, later phases in this same file.

#### `[x]` T177: reject `generator:`/`config:` keys at load time

Extended the existing `ErrRemovedConfigKey`/`checkRemovedKeys` mechanism (built by
T160/T163 for `changelog.remote`/`release.platforms`) with four new entries
(`changelog.generator`, `changelog.config`, `release.notes.generator`,
`release.notes.config`, plus their per-env variants) — commit 5a5a069. Note that this
task's own review found and required reverting an out-of-scope `validator.go` edit
(fixed in this commit) and required a properly-scoped fix to `.config/heraut.yml` (also
fixed in this commit) — both documented in more detail in the plan's Task 2a amendment
note.

A follow-up (Step 7, added mid-execution — see the plan doc) then found that the loader
change and `.config/heraut.yml`'s fix alone deadlocked every commit in this repository:
`heraut commit verify` (this project's own `commit-msg` hook) runs `config.Load` *and*
`config.Validate`, so a present `generator:` key was rejected by the new removed-key
check while an absent one still tripped `validateContentDriver`'s pre-existing
"required" check — no valid `.config/heraut.yml` existed in between. The loader and
validator halves of this change turned out not to be independently stageable for any
config on the full Load-then-Validate path, so the "required" check's removal was pulled
forward into this task instead of waiting for T180: `TestValidate_changelogMissingGenerator`
was replaced with `TestValidate_changelogAbsentGeneratorIsValid`, and
`validateContentDriver` no longer errors on an empty `Generator`. The *enum* check
(`validGenerators`) and the `tag_pattern` generator gate stay deferred to T180/T181, since
neither fires on an absent generator, only a present-but-invalid one.

#### `[x]` T178a: fix collateral test damage from T177 (`internal/config`)

#### `[x]` T178b: fix collateral test damage from T177 (`internal/cmd`)

#### `[x]` T178c: fix heraut init — it now generates configs it can't load (`internal/scaffold`)

#### `[x]` T179: empty `Generator` builds native end-to-end

#### `[x]` T180: validator — drop generator-required/enum + tag_pattern generator gate

#### `[x]` T181: validator — drop template/tickets/rendering generator gates

#### `[x]` T182: merge — drop the generator-switch full-replacement branch

#### `[x]` T183: schema.json + testdata fixtures go native-only

#### `[x]` T184: `docs/heraut.sample.yml` drops `generator:`/`config:`

**Completion note (2026-08-14):** Phase 2.5's config cutover is done — nine tasks landed
against a seven-task original estimate, every added task or same-scope fix found by an
implementer or reviewer stopping to verify empirically rather than trusting the plan's
research. T177 (the `ErrRemovedConfigKey` loader extension) had to absorb the validator's
"required"-check removal, originally slated for T180: `heraut commit verify` — this repo's
own commit-msg hook — runs the full Load-then-Validate path, so the loader-rejects and
validator-stops-requiring halves were not independently stageable for any config on that
path, including this repository's own `.config/heraut.yml`. T178 grew from one task into
three because the plan's research had only mapped the `internal/config` slice of the
blast radius: T178a (`internal/config`, ~30 tests) also found and fixed a second
validator.go bug — the `tag_pattern` regex-compile check had the same "doesn't treat an
empty `Generator` as native" flaw T177 had just fixed elsewhere — and dropped a
`TestValidate_invalidFixtures` row whose fixture stopped being invalid; T178b
(`internal/cmd`, 17 tests) surfaced a real cross-task ordering dependency — two of its own
tests couldn't pass until `buildGenerator` stopped unconditionally constructing a
generator in dry-run mode, pulling T179 forward out of its normal reading-order position
to unblock it; T178c (`internal/scaffold`) fixed a genuine product regression, not just
tests — `heraut init` was generating `.heraut.yml` files that failed to load on the very
next invocation — and swapped one scaffold test's "arbitrary passthrough field" example
from `Generator` to `TagPattern` after confirming the original code path is unreachable
from any real `heraut init` flow. T179–T182 landed as pure mechanical follow-through:
`buildGenerator`/`usesNative` (T179), then validator cleanup (T180–T181), then the
generator-switch merge branch deletion (T182). T183 (schema + fixtures) found a second
package-local copy of `forge-minimal.yml` the plan's file list had missed, and a review
round caught an initial fix that deleted whole `release.notes`/`changelog` blocks instead
of using `{}` — which would have silently flipped six fixtures from "generation
configured" to "disabled" per `internal/app/pipeline.go`'s nil-pointer gate, even though no
test happened to catch it; `internal/config/loader_forge_test.go`'s
`TestLoad_ForgesAndTargets` self-resolved off the same fixture list. T184 closes the phase:
`docs/heraut.sample.yml`'s four `generator:`/`config:` sites are gone, and — added mid-plan
by Task 2a, no task in the original scope had ever touched `README.md` — the two
`generator: git-cliff` lines inside the fenced yaml block `TestShippedExamples_LoadAndValidate`
loads and validates are gone too (the two further hits in README's prose comparison table
are untouched, deferred to Phase B). T184's own self-review hit the same "dangling empty
YAML key" class of bug T183 had already hit once: leaving `release.notes:` with nothing (or
only comments) under it collapses to a nil `*ContentDriver` in yaml.v3, and
`cfg.Release.Notes != nil` is the live gate both `internal/app/pipeline.go` and
`internal/pipeline/release.go` use to decide whether release notes generate at all —
silently contradicting the adjacent "Omit if you want ... no description body" comment in
both files. Fixed with `notes: {}` in both `docs/heraut.sample.yml` and `README.md`
(matching the `{}` precedent already used in T184's own per-env example edit), verified
with a throwaway test asserting `cfg.Release.Notes` is non-nil after load. The full suite
and `hk check` are clean.

**Final whole-branch review (2026-08-14):** a review across the full `f526dd9..37a8bd5` diff
(all 9 tasks at once) found 1 Critical + 2 Important issues no per-task review had caught,
since each was only visible looking at the phase as a whole. Critical: `heraut init`'s update
path (`ConfigToAnswers` in `internal/scaffold/wizard.go`) was still reading
`cfg.Changelog.Generator`/`cfg.Release.Notes.Generator` to pre-populate the wizard's generator
prompts — post-T177 those fields are structurally always `""`, which is exactly the wizard's
"None" option's bound value, so re-running `heraut init` against any existing config and
accepting the pre-populated defaults silently dropped `changelog:`/`release.notes:` from the
regenerated file. Important: `docs/heraut.sample.yml`'s commented `tag_pattern` example ended
up at the wrong YAML nesting level once `notes: {}` flattened onto one line (a flow-style `{}`
can't carry an indented child comment); and the "dangling empty key parses to YAML `null`"
bug class that had already been hit reactively three times in this phase (T177, T183, T184)
still had no permanent test guarding against a fourth occurrence. Fixed in commits `90d94ad`
(wizard: derive the generator sentinel from block presence, not the stale field — TDD, new
test goes through the real `config.LoadFromReader` path so it can't miss this class of bug
again), `78b91be` (sample.yml: rewrote the tag_pattern example to the file's existing
block-style-comment convention), and `40bbe31` (added `cfg.Changelog`/`cfg.Release.Notes`
non-nil assertions to `TestShippedExamples_LoadAndValidate` — a permanent net for the
bug class, not another reactive fix). Re-reviewed independently (red/green repro, hand-traced
YAML, throwaway load tests, `hk check`) — clean, zero findings. Phase A is done.

**Phase B scope** — deleting the `git-cliff`/`communique` packages, removing `heraut cliff`,
and rewriting `docs/specs/02`'s "Content generators" section and `docs/specs/05` — is a
separate, not-yet-started plan. Both reviews of this phase surfaced items that are stale or
misleading today but out of Phase A's scope by the plan's own Global Constraints; carry these
into Phase B's scope rather than losing them:
- `README.md`'s prerequisites table (`generator: git-cliff`/`generator: communique` lines) and
  its "Generator for…" prose in the config-reference section.
- `docs/specs/02-configuration.md` and `docs/specs/05-generators-and-platforms.md` still
  document `generator:`/`config:` as live and, in one place, `config` as "required" for
  communique.
- `internal/app/check.go`'s `configuredGenerators(nil)` still hard-requires both `git-cliff`
  and `communique` on `PATH` when no config file is found, even though the no-config case is
  now unambiguously native.
- `internal/cmd/check.go`'s `heraut check cliff` output renders `generator is , not
  git-cliff` (empty string) for every valid config now, not just an edge case — the skip
  message needs updating or the command needs to go straight to removal.
- `internal/scaffold/wizard.go`'s "Changelog generator"/"Release notes generator"
  `huh.Select` prompts still offer `git-cliff`/`communique`/`None` as live choices even though
  neither of the first two is ever written to the emitted config anymore (Phase A only fixed
  the presence-tracking regression, not the prompt's stale option list — see the wizard
  simplification note this phase already carries below).
- `internal/cmd/check_test.go` has two remaining test-only dangling `changelog:` keys
  (harmless today, same bug class, worth a pass when that file gets touched for Phase B
  anyway).

**Phase B execution** (started 2026-08-14, plan at
`docs/superpowers/plans/2026-08-14-native-only-generator-phase-b.md`, executed via
`superpowers:subagent-driven-development`, task IDs continue from T185):

#### `[x]` T185: collapse `buildGenerator` to native-only; delete `usesNative`

Collapsed `internal/app/pipeline.go`'s `buildGenerator` from a three-way switch to an
unconditional `native.New(...)` call, dropping its `error` return and retyping its mode
parameter from `gitcliff.Mode` to `native.Mode`; deleted `usesNative` and `nativeMode`
entirely, and the `usesNative` conjunct/guard at both of its two call sites. Eight tests
whose scenario depended on a non-empty `Generator` value reaching `buildGenerator` (now
structurally impossible pre-T188, since the field still exists but nothing routes a
non-empty value there via `config.Load`) were deleted; two — `PerEnvDerivesTagPattern` and
`ExplicitTagPatternWins` — covered a still-real behavior (env-scoped `TagPattern`
derivation) via a mechanism (asserting on a mocked `git-cliff` subprocess's CLI args) that
no longer applies to native, so they were rewritten as direct `withEnvDerivations` unit
tests in `tagglob_internal_test.go` instead of dropped. Review flagged one pre-existing
plan imprecision (the brief incorrectly asserted Go tooling never flags unused
parameters) as a named risk; the reviewer verified empirically that this repo's actual
`.golangci.yml` doesn't enable `unusedparams`, so `resolveEnrichForgeIfNeeded`'s now-dead
`drivers` parameter is non-blocking and deliberately deferred to T188's sweep. Commits
`208eba7..eec5e62`; review clean, zero Critical/Important findings.

#### `[x]` T186: delete the `heraut cliff` command

Deleted `internal/cmd/cliff.go`, `internal/app/cliff.go`, and their orphaned test files
(`internal/cmd/cliff_test.go`, `internal/app/cliff_test.go`); removed
`root.AddCommand(NewCliffCmd())` from `internal/cmd/root.go`; added
`TestRootCmd_NoCliffCommand` to `internal/cmd/root_test.go`. Pure deletion, no other files
touched. Commits `739405e..bd309d7`; review clean, zero Critical/Important findings.

#### `[x]` T187: delete `heraut check cliff` and the git-cliff/communique runtime probes

Deleted `newCheckCliffCmd`/`newCheckCliffChangelogCmd`/`newCheckCliffReleaseNotesCmd`/
`runCliffChecks`/`checkCliffDriver` and the default-output "Cliff" section from
`internal/cmd/check.go`; deleted `configuredGenerators`/`CheckCliff` and `RuntimeCheck`'s
"Generators" probe section from `internal/app/check.go`. Also deleted
`internal/app/checkcliff_test.go` and 3 `TestAppCheckCliff_*` tests from
`internal/app/check_test.go` (not in the plan's explicit file list, but required —
compile-necessity, they called the now-deleted `app.CheckCliff` directly) and stripped
git-cliff/communique FakeBin setup from a 4th test beyond the plan's named 3 (same
unused-setup pattern). After this task, zero non-test code in `internal/app/`/`internal/cmd/`
imports `internal/generators/gitcliff` or `communique` — verified, unblocking T188's package
deletion. Review found 2 Important findings, both about documentation rather than the code
itself: the implementer's report omitted 3 of the test deletions from its own accounting
(fixed by completing the report — no code change), and `CLAUDE.md` still documented the
now-deleted `heraut cliff` command and an inflated generator count in 4 places the plan
hadn't anticipated (2 flagged by review, 2 more found by the controller reading the file
directly: the "Project layout" file-tree's second mention of `check.go`'s subcommands, and
the `cliff.go` file-tree row itself). Fixed in one fix round; the fix also caught 2 further
stale CLAUDE.md references beyond the 4 named (a Tech-stack table row and the "Bundled
external CLIs" section, both directly falsified by this task's own probe removal, not
premature Task 188 scope). Commits `11eb39c..8b74989`; re-review clean, all findings
addressed, no new breakage.

#### `[x]` T188: delete `gitcliff`/`communique` packages + `ContentDriver.Generator`/`.Config` fields

Deleted `internal/generators/gitcliff/` and `internal/generators/communique/` wholesale (11
files) and `ContentDriver.Generator`/`.Config` from `internal/config/config.go` — the payoff
commit T177 (Phase A) anticipated when it made `generator:`/`config:` removed YAML keys. The
brief's file list undercounted the blast radius substantially (one native test file alone
needed 14 literal fixes, not 3; 9 more files across `internal/app/`/`internal/generators/native/`
were never listed) — all found and fixed, each checked individually for a companion field-read
before stripping, not just blindly stripped. Two controller rulings expanded scope mid-task,
both investigated directly before ruling rather than taken on the implementer's word: (1)
`internal/pipeline/release_integration_test.go` — a real functional `gitcliff.New` consumer the
brief's `_test.go`-excluding verification grep missed — deleted in full; its specific purpose
(proving `HERAUT_REMOTE_URL` propagates through a *real* subprocess) is structurally moot once
native, which never shells out for its own generation, is the sole generator, and the broader
per-platform-distinct-notes behavior it also covered is separately proven by
`internal/pipeline/release_test.go:243`'s `TestRun_MultiPlatform_NotesPerPlatform`. (2) a narrow
exception granted to fix 4 `internal/scaffold` **test** files
(`wizard_internal_test.go`/`wizard_test.go`/`generate_test.go`/`dropped_test.go`) whose compile
depended on the deleted fields — the 4 **production** files (`wizard.go`/`generate.go`/`cliff.go`/
`dropped.go`, Phase C's actual scope) stayed completely untouched, verified independently by the
reviewer via a direct `git diff` on each. This resolved a genuine tension in the plan's own
wording between "internal/scaffold is out of scope" (directory-wide framing, meant to protect
Phase C's wizard-redesign work) and "no phase lands with a broken build even temporarily" (a
higher-order, repo-wide invariant) — full reasoning in the SDD ledger. One controller error along
the way: an unrelated `git add`/`git commit` accidentally swept up the implementer's
already-staged package deletions into a commit (`a052da5`) whose message only describes a
plan-doc amendment — not rewritten per this repo's norms against amending, noted here so a future
`git log` reader isn't misled; the remaining work landed accurately in `74b3972`. Commits
`f6383c1..74b3972` (2 commits); review clean (reviewer independently re-verified all 3 rulings
against the diff, including a direct re-check of the scaffold production-file boundary), zero
Critical/Important findings, 2 Minor prose-staleness items deferred (a stale doc comment on
`ContentDriver` and a stale doc comment on one scaffold test, both non-behavioral).

#### `[x]` T189: remove the Real-CLI smoke-test exception from testing docs

Replaced `.claude/rules/testing.md`'s "Real-CLI smoke tests" section (its only remaining
example, git-cliff's, is gone; cocogitto's was already dropped by ADR-0028) with a one-line
note that the category has no live example today. Updated `docs/specs/06-dx-and-testing.md`'s
Unit-layer target list, Contract-layer tool list, and "Hard-won edge cases" list to match.
Also fixed two directly-adjacent, actively-broken references in the same
`.claude/rules/testing.md` file the review flagged as beyond the brief's literal steps: the
"Four test layers" table's Contract row still listed `git-cliff` as a live example CLI two
sections above this task's own rewrite saying it's gone, and the MockRunner code example
still invoked `gitcliff.New(mr, cfg)`, a package T188 already deleted — both would have
self-contradicted the section this task exists to fix, in a file loaded into every Claude
Code session via `CLAUDE.md`. Adjudicated as in-scope consistency, not scope creep. Commits
`549aee0..897076b`; review found no code/content defects, one process-labeled finding
adjudicated as not-a-defect (reasoning above and in the SDD ledger).

#### `[x]` T190: rewrite `README.md`

Rewrote the intro prose (drops `git-cliff`/`communique` from the tool list, "generator/platform
composition" → "platform composition"), the strategy/generator/platform summary line ("two
content generators" → "a built-in content generator (`native` — no external binary
required)"), the Prerequisites table (drops both rows, adds a note that generation ships in the
binary), the Commands table (drops `heraut cliff <mode>`, `heraut check`'s description drops
"cliff"), and the config field table (`changelog`/`release.notes` rows drop "Generator for..."
phrasing). Pure prose/table edit, single file. Commits `f322de7..1fd3cff`; review clean, zero
Critical/Important findings.

#### `[x]` T191: rewrite Spec 02's content-generators field table

Replaced the "Content generators" field table (`generator`/`config`/`output`/`tag_pattern`/
`template` rows) with a "Content generation" section: `generator`/`config` rows dropped
(fields no longer exist), `tag_pattern` reworded to drop "for git-cliff only" (native uses it
too), `template` reworded to describe native's real ADR-0037 usage instead of the stale
"vestigial, no current consumer" claim. The task's own final sweep found and fixed 12 further
issues in the same file — 8 worked-example YAML blocks still using the now-invalid
`generator:`/`config:` keys (independently verified schema-invalid against `schema.json`'s
`ContentDriver` definition before the fix), 2 prose passages attributing native's own behavior
(heading-pattern stripping, one-section-per-tag rendering) to git-cliff, and 2 cross-references
to the renamed heading — all confirmed accurate and in-category by review. One fix round: the
sweep itself missed a self-contradiction its own grep had surfaced — a "Design principles"
bullet (line 62) still citing `generator: …` in present tense as a live naming convention, 40
lines above the section now correctly saying no such key exists — dismissed as historical
context in the first pass, reworded in the fix round to drop the stale example (platform-only
naming convention, generator opacity principle rescoped to platforms). Commits
`ecac173..763b487` (2 commits); re-review clean, finding addressed, no new breakage.

#### `[x]` T192: rewrite Spec 05 for native as heraut's sole generator

The biggest single doc edit in Phase B. Replaced the 3-generator comparison table with a
single `## Generator` intro; promoted `### native` to `## native`, content unchanged;
deleted git-cliff-exclusive content (TOML embed description, `heraut cliff` invocation
blocks, `[remote.*]` injection paragraph) and the whole `### communique` section; correctly
identified that two subsections nested under the old `### git-cliff` heading —
`forges` (explicit metadata forge, ADR-0043) and `Auto-detection and self-hosted hosts` — were
**not** actually git-cliff-exclusive (general `forges:` config and a fallback chain shared
with native), kept both, promoted `#####`→`###`, re-homed after native's own content, reworded
their "both git-cliff and native" framing to native-only, and deleted only the
git-cliff-specific bullet within the auto-detection divergence prose (the native bullet
stands, describing real current behavior). `### No generator` renamed/reworded to
`### Omitting changelog or release notes`. `## Generator interface`'s `Validate()` line and
"Per-platform link resolution" paragraph updated to drop git-cliff/communique framing.
`## Platforms` and `## Extensibility` untouched, verified zero diff. One fix round: promoting
only the top `### native`→`## native` heading (per the plan's literal Step 2 text) left two
of native's own nested `#### ...` sub-headings under-promoted relative to the newly re-homed
`###` siblings — a plan gap (Step 2 didn't anticipate the cascading depth effect), not an
implementer error — fixed by promoting both to `###` (heading markers only, no prose
changed). Commits `553d6ae..b59f8f5` (2 commits); re-review clean, finding addressed, no new
breakage. The hardest judgment call in the whole task — correctly separating shared content
from git-cliff-exclusive content within one nested section — was executed correctly in both
directions on the first pass, per independent reviewer verification.

#### `[x]` T193: update `docs/tasks/roadmap.md`'s intro line

Deliberately narrow — dropped `git-cliff`/`communique` from the intro sentence's tool list
(kept `cog`, heraut's own commit-msg hook tool, unrelated to this epic), touching only that
one line. `docs/tasks/roadmap.md`'s other, much older staleness (cocogitto, a pre-forge-
extraction package layout, "14 ADRs") is out of scope — a separate, unrelated documentation-
debt concern predating even Phase A, left untouched per the plan's own explicit instruction.
(CLAUDE.md's stale command references, also originally slated for this task, were already
fully handled by T187's fix round.) Commit `e40bcbf..49e059a`; review clean.

#### `[x]` T194: author ADR-0045; update the ADR index; supersede ADR-0010

Wrote `docs/adr/0045-native-sole-generator.md`, modeled on ADR-0028's structure (the direct
precedent for a hard-cutover generator removal). The plan's own draft text was written
speculatively before T185-T193 executed; the implementer correctly reconciled it against
what actually shipped rather than transcribing it verbatim — moving schema.json/`docs/
heraut.sample.yml`/testdata changes back to Phase A (T183/T184) where they actually
happened, adding T190's README.md rewrite (missing from the draft's Decision list), and
adding T188's actual scope (the `release_integration_test.go` deletion and the narrow
scaffold-test-file exception, neither anticipated by the draft). Added ADR-0045's row to
`docs/adr/README.md` and, beyond the plan's literal instruction, synced ADR-0010's Status
*column* in that same index table (not just its own file header) to "Superseded by
ADR-0045" — matching the established convention already used for ADR-0014/ADR-0020's rows,
confirmed by review. `docs/adr/0010-embedded-cliff-toml-default.md` got the Status-line +
blockquote supersession treatment, rest of the file untouched. Commit `8005976..9ed5b15`;
review independently cross-checked every Decision/Consequences claim against the roadmap's
own T177/T185-T192 completion notes and current repo state — zero Critical/Important
findings, two Minor items deferred (a paraphrase presented in quotation marks, inherited
unchanged from the plan's own draft text; a cosmetic ADR-date observation, within this
repo's existing precedent range).

---

**Phase B is done.** All 10 tasks (T185-T194) landed on `main`, each with an independent task
review and, where findings surfaced, a fix round verified by a scoped re-review — 5 of 10 tasks
needed at least one fix round (T187, T191, T192 on controller-adjudicated or genuine findings;
T188 hit two real BLOCKED escalations, both resolved by controller rulings investigated
firsthand rather than taken on faith). `git-cliff` and `communique` are fully gone: no
package, no command, no config field, no runtime probe, no doc describing either as live.
`internal/scaffold/`'s production wizard code is untouched, exactly as scoped — its 4 test
files needed a narrow, ruled exception to keep compiling once the struct fields were deleted.
Phase C (wizard simplification) and Phase D (Docker/mise infra cleanup) remain separate,
not-yet-started plans, each still needing its own brainstorm→plan→SDD cycle per this epic's
established pattern. The final whole-branch review (Opus) found zero Critical findings and 5
Important + several Minor ones — all fixed in one wave, re-reviewed clean; the SDD ledger
that recorded every ruling made along the way has been deleted per this project's standing
process (`git log` on commits `79fbd73..dc0861f` is the permanent record now — 27 commits,
one rebase reword applied to `a052da5`/now `3636186` to fix a misleading commit message
before push, everything else landed as originally authored).

**Phase C scope note (added 2026-08-17):** not yet started, but two things are already
confirmed and worth carrying forward rather than rediscovering:
- `internal/scaffold/wizard.go`'s `huh.Select` prompts still offer `git-cliff`/`communique`/
  `None` as live-looking choices, but this is decorative-not-broken today —
  `internal/scaffold/generate.go`'s `answersToConfig` only reads
  `Answers.ChangelogGenerator`/`NotesGenerator`'s presence as a boolean gate and never emits a
  `generator:` key regardless of which option is picked, so `heraut init` always produces a
  loadable config. Real UX debt (the question has no live consequence), not a config-generation
  bug — state this plainly in Phase C's brief so it isn't mistaken for an urgent fix.
- Phase B's own final review found stale git-cliff/communique prose survives in plain comments/
  doc strings that a symbol-focused grep (`\.Generator\b|GitCliff|Communique`) doesn't catch —
  three separate times, reactively. Phase C touches `wizard.go`, the densest remaining cluster
  of this; build a raw `grep -rn "gitcliff\|git-cliff\|communique"` sweep into Phase C's own
  plan from the start instead of finding it the same way a fourth time.

## Phase C — Wizard simplification (supersedes T164)

> See `docs/superpowers/specs/2026-08-08-native-only-generator-design.md` §4 (original high-level
> scope) and `docs/superpowers/specs/2026-08-17-native-only-generator-phase-c-wizard-design.md`
> (detailed design: exact wizard flow, field renames, the `internal/forge` export decision) for
> the full design. Plan: `docs/superpowers/plans/2026-08-19-native-only-generator-phase-c.md`.

Drops `internal/scaffold/wizard.go`'s decorative generator-choice step (`git-cliff`/`communique`/
`None` — options with no live effect since Phase A removed the `generator:` config key entirely)
and builds the forge/target questions `docs/tasks/forge-abstraction-roadmap.md`'s T164 originally
scoped, plus what T164 didn't anticipate: an `api_mode` prompt and wizard-editable
`commits.enrichment_forge`/`enrichment_policy`. Supersedes T164, which stays `[ ]` there with a
pointer note rather than being implemented twice.

#### `[x]` T195: export a CI/git-origin type detector from `internal/forge`

Implemented `forge.DetectForWizard(getenv func(string) string, gitOrigin string) (typ, projectOrRepo string, ok bool)` as a new exported wrapper combining the existing unexported `detectCIForge` and `parseGitOrigin` helpers. CI environment detection (GitLab/GitHub/Azure) takes priority; falls back to parsing git origin URL for public hosts (github.com/gitlab.com/dev.azure.com). Unlike `Resolve`'s zero-config path, never falls back to inspecting ambient token env vars — the wizard asks users to pick/confirm type explicitly when detection is inconclusive. Tests added in `internal/forge/detect_test.go` (reused `env` helper from `resolve_test.go`). Full suite 56 tests green.

#### `[x]` T196: `internal/scaffold` layer-rule + wire platform-type pre-fill into `runPlatformWizard`

Replaced `detectRemoteProject` with `gitRemoteOriginURL` (raw `git remote get-url origin` output) and `detectPlatform(getenv, gitOrigin) (typ, projectOrRepo string)`, which calls `forge.DetectForWizard` (T195) and only accepts a detected type when it's one of the wizard's two Select options (`"github"`/`"gitlab"`) — a detected `azure_devops` type is discarded, falling back to `parseRemoteProject`'s any-host path parsing so at least the project path still pre-fills. `parseRemoteProject` itself is untouched; it remains the fallback for self-hosted GitLab/GitHub Enterprise remotes that `forge.DetectForWizard`'s host allowlist doesn't type. `runPlatformWizard` now pre-fills `p.Type` from detection before the platform Select renders (previously the Select always started blank), and the gitlab branch's local `detected` variable was renamed to `detectedProject` for clarity alongside the same rename in the github branch. Added `internal/scaffold/` to `.claude/rules/coding.md`'s layer-rules table (`internal/{config,ui,versioning,forge}/`) since this is scaffold's first dependency on `internal/forge`. Four new table-driven-style `TestDetectPlatform_*` tests cover CI detection, self-hosted fallback, the Azure-DevOps-discarded case, and no-detection; full `internal/scaffold` suite and `go build ./...` both green.

#### `[x]` T197: `Answers.ChangelogGenerator`/`NotesGenerator` → `EnableChangelog`/`EnableReleaseNotes` bools

Replaced the two decorative `huh.NewSelect` "generator" prompts (git-cliff/communique/None — options with zero live effect since `generator:` was removed from the config schema) with two honest `huh.NewConfirm()` toggles. `Answers.ChangelogGenerator`/`NotesGenerator` string fields became `EnableChangelog`/`EnableReleaseNotes` bools; `Defaults()`, `ConfigToAnswers`, `mainForm`'s changelog/notes groups, and `generate.go`'s `answersToConfig` gates were all updated in the same commit, since Go requires the whole `scaffold` package to compile together. One call site not explicitly listed in the plan's line ranges also needed updating to keep the package compiling: `RunWizard`'s `if a.NotesGenerator != ""` gate (which decides whether to run the platform wizard) became `if a.EnableReleaseNotes` — a direct 1:1 mechanical translation, not a new decision. All renamed/replaced tests from the plan landed verbatim (`TestDefaults_EnableChangelogAndNotes`, `TestConfigToAnswers_EnableChangelog`, `TestConfigToAnswers_NoChangelog`, `TestConfigToAnswers_EnableReleaseNotes`, `TestConfigToAnswers_ChangelogAndNotesPresenceSurviveLoadRoundTrip`, plus 9 struct-literal updates across `generate_test.go`). Confirmed RED (`unknown field EnableChangelog in struct literal of type scaffold.Answers`) before implementing; full `internal/scaffold` suite, `go build ./...`, `go vet ./...`, `go test ./...`, and `hk check` all green after.

#### `[x]` T198: `Answers.RemoteMetadata` → `EnrichmentPolicy` rename

Mechanical rename in `internal/scaffold`: the stale field name `RemoteMetadata` (a leftover from pre-rename config keys) becomes `EnrichmentPolicy` to reflect its actual content. Updated both test cases (`TestConfigToAnswers_PreservesAssetsTicketsEnrichmentPolicy` / `TestGenerateYAML_AssetsTicketsEnrichmentPolicy`), the `Answers` struct in `wizard.go`, the `ConfigToAnswers` function, and the usage in `generate.go`. All tests pass; no scope creep beyond the rename target. **Scope:** S. **Dependencies:** none.

#### `[x]` T199: independent "Publish releases?" confirm replaces the `NotesGenerator`-gated trigger

Added a new `Answers.PublishReleases bool` field and an independent `huh.NewConfirm()` prompt ("Publish releases to a platform (GitHub/GitLab)?") that gates `runPlatformWizard` execution. This decouples platform setup from release-notes generation, resolving the temporary coupling that T197 introduced when it mechanically renamed `NotesGenerator` to `EnableReleaseNotes` while keeping the post-form gate on the old field. `ConfigToAnswers` now derives `PublishReleases` from platform presence (`len(a.Platforms) > 0`). Three new tests added: `TestDefaults_PublishReleases` (true by default), `TestConfigToAnswers_PublishReleasesWhenPlatformsExist` (true when platforms exist), `TestConfigToAnswers_NoPublishReleasesWhenNoPlatforms` (false when none). The field is a wizard-flow-control device only — it is never consumed by `generate.go`, since `answersToConfig`'s existing `hasPlatforms` check on `len(a.Platforms)` already correctly reflects platform presence. **Fix (found in review):** Added `else { a.Platforms = nil }` in `RunWizard`'s post-form gate to handle the edit-existing-config path correctly — when editing a config with pre-populated platforms and declining to publish, platforms are now cleared before `GenerateYAML` sees them, ensuring the user's "No" answer is respected. Full suite 114 tests green.

#### `[x]` T200: `api_mode` prompt in the GitLab platform branch

Added `PlatformAnswer.APIMode string` and a new `hideAPIMode(platformType, tokenChoice string) bool` helper next to `resolveTokenChoice`: hidden for any non-GitLab platform, or when the chosen token is `CI_JOB_TOKEN`, since GitLab's GraphQL API structurally rejects job tokens. `runPlatformWizard`'s Step 3 gained a third `huh.NewGroup` (rest/graphql select) gated by `WithHideFunc(func() bool { return hideAPIMode(p.Type, tokenChoice) })`; `p.APIMode` is only persisted when the group is shown, otherwise reset to `""` so the implicit "rest" default never gets written into `.heraut.yml`. `ConfigToAnswers` and `generate.go`'s forge-building loop both carry `APIMode` straight through since `config.Forge.APIMode` already existed (added by an earlier task). Four new table-driven `TestHideAPIMode` cases plus `TestGenerateYAML_PlatformAPIMode` and `TestConfigToAnswers_PreservesAPIMode` landed verbatim per the brief. Confirmed RED (`undefined: hideAPIMode`) before implementing; full `internal/scaffold` suite and `go build ./...` green after.

#### `[x]` T201: enrichment policy/forge prompts; `EnrichmentForge` becomes wizard-editable

Added `runEnrichmentWizard(a *Answers) error`, run in `RunWizard`'s post-form flow right after the `PublishReleases`-gated `runPlatformWizard` call: it always shows a "PR/MR enrichment policy" select (optional/required/disabled) whenever changelog or release-notes generation is enabled, and additionally shows an "Enrichment forge" select — gated by the new `shouldPromptEnrichmentForge(platforms []PlatformAnswer) bool` (true only for 2+ platforms, since with 0 or 1 the choice is unambiguous) — offering the exact forge names `GenerateYAML` will assign. Extracted `platformDisplayNames(ps []PlatformAnswer) []string` in `generate.go` from the inline dedup loop previously embedded in `answersToConfig`'s forge-building block, so both the wizard's forge-choice list and the actual forges written to YAML compute names identically; this is a pure refactor; the pre-existing `TestGenerateYAML_PlatformNamesDefaultedAndDeduped`, `TestGenerateYAML_PreservesEnrichmentForgeOnSecondForge`, and `TestGenerateYAML_EnrichmentForgeDefaultsToFirstWhenUnset` tests pass unchanged, confirming behavior-preservation. The first-forge-fallback block in `answersToConfig` (`if len(cfg.Forges) > 1 { ... }`) is kept exactly as-is per the design doc's correction — it remains the required fallback for non-wizard callers of `answersToConfig` (hand-built `Answers`, or a future `--defaults` preset with 2+ platforms) — only its comment was updated to note that the wizard now asks explicitly. `Answers`' struct comment was trimmed to drop the stale "EnrichmentForge not wizard-editable" note now that both `EnrichmentPolicy` and `EnrichmentForge` are wizard-editable. Two new tests landed verbatim per the brief: `TestPlatformDisplayNames`, `TestShouldPromptEnrichmentForge`. Confirmed RED (`undefined: platformDisplayNames`, `undefined: shouldPromptEnrichmentForge`) before implementing; full `internal/scaffold` suite (85 tests) and `go build ./...` green after.

#### `[x]` T202: delete `internal/scaffold/cliff.go`; cleanup sweep; manual wizard verification

Deleted `internal/scaffold/cliff.go` and `internal/scaffold/cliff_test.go` — the single-function
`IsCliffGenerator` helper had no callers outside its own test, confirmed by a repo-wide grep before
deletion. `go build ./...` green immediately after. The cleanup sweep (`grep -rn
"gitcliff|git-cliff|communique" internal/scaffold/`) returned no output — T197's earlier work had
already removed every such reference from `wizard.go`/`generate.go` and their tests, so this task's
deletion is the clean final step, not a discovery of new stragglers. Full suite green (1402 tests,
26 packages) and `hk check` clean (0 files to lint, since the change is a pure deletion). Manual
verification of `heraut init --defaults --config <scratch>` produced a config that
`heraut check config` confirms is valid, exercising `Defaults()`'s new bool fields
(`EnableChangelog`/`EnableReleaseNotes`/`PublishReleases`) end-to-end through the non-interactive
path. No TTY was available in this execution environment, so the interactive wizard flow itself
(the `huh` form wiring — toggles, hide-funcs, group gating) was not manually stepped through; per
the brief, this is stated explicitly rather than claimed, and is consistent with this package's
existing pattern of leaving `RunWizard`'s interactive path to the pure-function unit tests
(`hideAPIMode`, `shouldPromptEnrichmentForge`, `detectPlatform`, `platformDisplayNames`) rather than
a driven end-to-end test.

---

**Phase C is done.** All 8 tasks (T195-T202) landed on `main`, each with an independent task
review. T199 was the only task needing a fix round: the reviewer found that declining the new
"Publish releases?" confirm on the edit-existing-config path left stale pre-populated platforms in
`a.Platforms` (only `runPlatformWizard` cleared it, and declining skipped that call) — fixed with a
one-line `else { a.Platforms = nil }`, verified by a clean scoped re-review. The other 7 tasks
passed their first review with zero Critical/Important findings — only Minor, non-actionable notes,
mostly inherited verbatim from the plan/brief's own prescribed test code or comments rather than
implementer defects. `internal/scaffold`'s wizard now has an honest, generator-free flow: two
independent confirms for changelog/notes generation (replacing decorative git-cliff/communique/None
selects that had zero live effect since Phase A removed the `generator:` config key entirely), an
independent "Publish releases?" toggle decoupling publishing from notes generation, CI/git-origin
platform-type pre-fill via a new `internal/forge.DetectForWizard` export, a GitLab-only `api_mode`
prompt hidden when the chosen token can't support it (`CI_JOB_TOKEN`), and enrichment
policy/forge prompts (the forge prompt shown only when genuinely ambiguous, i.e. 2+ platforms).
`cliff.go`/`cliff_test.go` are deleted; the cleanup sweep found nothing outstanding. This closes out
`docs/tasks/forge-abstraction-roadmap.md`'s T164, already marked there as superseded by this phase.

---

## Follow-ups

#### `[x]` T203: `PlatformAnswer.TokenEnv`/`APIMode` lost on wizard re-edit

`runPlatformWizard` rebuilds each platform from a fresh `PlatformAnswer{}` inside its per-platform
loop (`internal/scaffold/wizard.go`), so `TokenEnv` and `APIMode` are never re-seeded from the
pre-existing snapshot before the Step 3 (token/api_mode) prompts render. Only
`matchPlatformSnapshot` — applied *after* the loop completes — carries passthrough fields (`Name`,
`BaseURL`, `Draft`, `Prerelease`) forward via type-scoped positional matching; `TokenEnv`/`APIMode`
are not in that passthrough set. Practical effect: re-running `heraut init` on an existing config
with `api_mode: graphql` set silently reverts to `"rest"` (Step 3's own default when
`p.APIMode == ""`), and a pre-existing `TokenEnv` similarly reverts to `resolveTokenChoice`'s
platform default instead of staying selected — both because Step 3 sees a zero-value `p`, not the
snapshot's value. Found by the whole-branch Phase C final review (2026-08-20); explicitly deferred
from that fix wave because it shares this root cause with a bigger, pre-existing restructure than a
fix-wave patch warrants. Fixing both requires seeding `p` from the type-scoped positional snapshot
match (similar to what `matchPlatformSnapshot` already does for `Name`/`BaseURL`/`Draft`/
`Prerelease`) *before* the token/api_mode prompts render in Step 3, not after the platform loop
completes. **Scope:** S–M.

**Fixed 2026-08-21** (commit `32d6d39`). Added `snapshotTokenAndAPIMode(snapshot, rebuiltSoFar
[]PlatformAnswer, platformType string) (tokenEnv, apiMode string, ok bool)` — a pure, unit-tested
function using the same type-scoped positional algorithm as `matchPlatformSnapshot`, but called
per-iteration in `runPlatformWizard` right after Step 1 (once `p.Type` is known) instead of once
after the whole loop, so it seeds `p.TokenEnv`/`p.APIMode` before Step 3 builds its prompts.
`matchPlatformSnapshot` itself is unchanged — it still restores `Name`/`BaseURL`/`Draft`/
`Prerelease` in its existing post-loop pass, since those fields aren't read by any prompt and don't
need to be seeded early. `runPlatformWizard` remains untestable directly (interactive `huh` forms,
no harness anywhere in this package); TDD coverage lives entirely in the new pure function, with one
test per scenario matching the existing `TestMatchPlatformSnapshot_*` style (single match,
type-scoped, second entry of the same type, no match for a new entry, empty snapshot). Full suite +
`hk check` clean.

#### `[x]` T204: unify the two platform-snapshot-matching algorithms in `wizard.go`

T203's fix added `snapshotTokenAndAPIMode` (`internal/scaffold/wizard.go`), a second, independent
implementation of "type-scoped positional matching" alongside the pre-existing
`matchPlatformSnapshot` — one is a per-iteration two-linear-scans-with-a-counter function, the other
is a post-loop map+consumed-counter batch function. T203's own review (2026-08-21) hand-traced and
2000-trial-fuzzed the two against each other and confirmed they currently agree in every case, but
nothing structurally enforces that agreement: a future change to either algorithm (e.g. switching
one to name-based matching, or changing how ties/gaps are handled) could silently desync the other,
reintroducing the exact "wrong snapshot entry restored" bug class T203 just closed — a config
data-loss risk on wizard re-edit, not just a cosmetic split. Fix direction: have `runPlatformWizard`
compute the matched snapshot entry once per loop iteration (via a single lookup, run right after
Step 1 when `p.Type` is known) and reuse that same match both to seed `TokenEnv`/`APIMode` before
Step 3 renders, and to populate `Name`/`BaseURL`/`Draft`/`Prerelease` on `p` before it's appended to
`a.Platforms` — collapsing the current post-loop `matchPlatformSnapshot` pass entirely, since every
rebuilt platform would already carry its full matched snapshot data by the time it's appended.
`TestMatchPlatformSnapshot_*` and `TestSnapshotTokenAndAPIMode_*` would need to be reconciled into a
single test suite for whatever the unified lookup function ends up being — per this project's "never
delete a load-bearing test row" rule, every scenario either suite currently covers must survive in
the merged suite, not just the union's line count. **Scope:** S.

**Fixed 2026-08-21** (commit `2ba6942`). `matchPlatformSnapshot` is now the single lookup: same
type-scoped positional algorithm, but its signature changed from a post-loop batch function
(`(original, rebuilt []PlatformAnswer) []PlatformAnswer`) to a per-iteration single-match function
(`(snapshot, rebuiltSoFar []PlatformAnswer, platformType string) (PlatformAnswer, bool)`), called
once right after Step 1 sets `p.Type` in `runPlatformWizard`. It returns the whole matched entry;
the call site copies all six fields (`Name`, `BaseURL`, `Draft`, `Prerelease`, `TokenEnv`,
`APIMode`) from it in one place, so a platform's TokenEnv/APIMode seed and its passthrough fields
are now structurally guaranteed to come from the same snapshot entry — not just proven to agree by
a one-time fuzz check, as T203's review had to do. `snapshotTokenAndAPIMode` and the old
`matchPlatformSnapshot` are both deleted; the post-loop `a.Platforms = matchPlatformSnapshot(...)`
call is gone, since every rebuilt platform now already carries its matched snapshot data by the
time it's appended. Test suites merged into one covering every scenario both prior suites had
(single match, no match for a new entry, type-scoped, second entry of the same type, empty
snapshot), plus a new `TestMatchPlatformSnapshot_InterleavedTypes` case combining repeats of one
type with an interleaved different-type entry — the scenario T203's review flagged as untested by
either prior suite. Net diff: +87/-119 lines. Full suite + `hk check` clean.

---

## Phase D — Infra housekeeping (Dockerfile / mise / ADR-0016)

> See `docs/superpowers/specs/2026-08-08-native-only-generator-design.md` §5 ("Infra
> housekeeping (Phase D)") for the full scope — already a complete, unambiguous spec, so no
> separate design doc was written. Plan:
> `docs/superpowers/plans/2026-08-22-native-only-generator-phase-d.md`.

The last remaining traces of `git-cliff`/`communique` outside code and wizard flow (already
removed in Phases A–C): the Docker image still bundles both, the dev-toolchain `mise` config
still pins `git-cliff`, and ADR-0016's bundled-CLI inventory still lists both.

#### `[x]` T205: Dockerfile — drop git-cliff/communique

Drop the `GIT_CLIFF_VERSION`/`COMMUNIQUE_VERSION` ARGs, their `mise use -g` install entries,
and their `cp` steps from the tools stage. `gh`/`glab` stay (ADR-0044, unrelated to this
epic). Rewrite the stage-3 base-image comment, which currently attributes the
`debian:trixie-slim`-over-`alpine` choice to communique's dynamic linking — verify via a real
`docker build --target tools` + `file` on the extracted binaries whether git-cliff also drove
that choice, or whether gh/glab alone still require it (i.e. whether the base image could
simplify further, or not) — an implementation-time check per the design doc, not a design
blocker.

**Files:** `Dockerfile`.
**Scope:** S. **Dependencies:** none.

Dropped `GIT_CLIFF_VERSION`/`COMMUNIQUE_VERSION` from the top-of-file ARGs block, both from
their `mise use -g` invocation and their `cp` lines in the tools stage, and the now-unused
`ARG GIT_CLIFF_VERSION`/`ARG COMMUNIQUE_VERSION` re-declarations inside that stage. Only
`GLAB_VERSION`/`GH_VERSION` remain. The stage-3 base-image comment previously attributed the
`debian:trixie-slim`-over-`alpine` choice to communique needing glibc for dynamic linking; a
real `docker build --target tools` + `file` on the extracted binaries (run during planning,
reconfirmed here) showed `gh` and `glab` are both statically-linked Go binaries — communique
was the *only* dynamically-linked tool in the old stage, so with it gone the remaining reason
to stay on a glibc-based distro is Debian's `apt`-installed `git`/`ca-certificates` in stage 3,
not the bundled CLI tools. The comment was rewritten to say so; `golang:trixie` (the builder
stage) stays consistent with that base to avoid glibc version surprises. `hadolint Dockerfile`
passed clean before and after (no lint drift). Verified with a full local build: `docker build
--target tools` followed by `ls /tools` showed only `gh`/`glab` (no `git-cliff`, no
`communique`); a full `docker build` of the final image, `heraut --version` printed the version
banner, and `gh --version`/`glab --version`/`git --version` inside the container all reported
correctly, with `/usr/local/bin` containing only `gh`, `glab`, `heraut`. Diagnostic images
(`heraut-t205-tools-check`, `heraut-t205-check`) were removed after verification.

#### `[x]` T206: `.config/mise/config.toml` — drop the git-cliff tool pin

Drop the `git-cliff = "2.13"` line from `[tools]`. Regenerate/edit `.config/mise/mise.lock`
accordingly (the orphaned `[[tools.git-cliff]]` block).

**Files:** `.config/mise/config.toml`, `.config/mise/mise.lock`.
**Scope:** S. **Dependencies:** none.

Removed the `git-cliff = "2.13"` pin from `config.toml`'s `[tools]` block (alphabetical
order preserved for the remaining entries). `mise.lock`'s orphaned `[[tools.git-cliff]]`
block was deleted by hand rather than via a fresh `mise lock` run: a bare `mise lock`
regenerate was tried during planning on a mise version newer than whatever produced the
committed lockfile, and it rewrote unrelated `url_api` metadata across every other pinned
tool (~63 unrelated added/changed lines, none of them meaningful) — pure out-of-scope
churn for a task about removing one tool. The manual edit removes exactly the orphaned
block and preserves the single blank line before `[[tools.go]]`, matching the file's
existing spacing convention. Also fixed `CLAUDE.md`'s "Tooling (mise)" section, which
claimed git-cliff was installed via mise — now false the moment the pin is dropped, so
the line was updated to list only Go, golangci-lint, and goreleaser, per the same
directly-adjacent-stale-reference precedent as T189. Verified with
`grep -n "git-cliff" .config/mise/config.toml .config/mise/mise.lock CLAUDE.md` (no
output, exit 1) and `mise install --locked` (no error mentioning `git-cliff`; the only
errors were pre-existing `claude`/`rtk` resolution failures from the user's global mise
config outside this repo, explicitly out of scope per the task brief).

#### `[ ]` T207: `docs/adr/0016-bundled-docker-image.md` — update the bundled-CLI inventory

Update the bundled-CLI table and surrounding prose (tool-orchestration list, Decision bullet,
table, base-image-choice rationale, Consequences' version-isolation and image-size examples)
to drop `git-cliff`/`communique`, per ADR-0028's established precedent that this specific
table is a living inventory, not a frozen historical record.

**Files:** `docs/adr/0016-bundled-docker-image.md`.
**Scope:** S. **Dependencies:** T205 (cites its verification findings).

#### `[ ]` T208: final sweep, verification, and phase close-out

Repo-wide grep sweep for any remaining `git-cliff`/`communique` reference this phase should
have caught, full `go build`/`go test`/`hk check` regression pass, a final real `docker build`
+ smoke test, and closing out the Phase D roadmap section.

**Files:** `docs/tasks/native-generator-roadmap.md` (+ read-only checks across the repo).
**Scope:** S. **Dependencies:** T205, T206, T207.

---

## Phase 3 — Raw-HTTP platform clients (deferred)

**Not scheduled.** Replacing `gh` / `glab` with direct `net/http` platform clients — which
would let heraut drop those binaries entirely and reach a fully self-contained generator +
publisher — is **explicitly deferred behind its own ADR** (see ADR-0032, "Phase 3"). It
reimplements asset upload, pagination, and rate-limit handling the CLIs currently absorb, and
shifts ongoing API-churn maintenance onto heraut. Listed here only so the epic's full arc is
visible; do **not** start it without a follow-up ADR.
