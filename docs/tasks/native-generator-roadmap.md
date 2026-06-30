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
| Phase 2 — remote enrichment via platform CLIs        | T127 – T129            | Not started |
| Phase 2.5 — remove the git-cliff package (own ADR)   | —                      | Deferred    |
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

#### `[ ]` T129: Azure DevOps enrichment via `az` (optional)

Bring the native path to ADR-0026 parity — Azure DevOps PR / author enrichment via the **`az`
CLI** (`az repos pr`, the `azure-devops` extension; token `AZURE_DEVOPS_EXT_PAT`) through the
runner, correlating PRs to commits (Azure has no direct "PRs for a commit" call), reusing the
Azure URL composition already in Go. **Optional** — a new dependency for a platform with no
heraut publish driver; do only when Azure attribution is wanted (ADR-0034 §3). Sequence last.

**Tests:** contract tests for the `az repos pr` invocation + correlation + policy branches.

**Files:** `internal/generators/native/enrich_azure.go` (+ test).
**Scope:** M. **Dependencies:** T127. **Design:** [ADR-0034](../adr/0034-native-remote-enrichment.md).

---

## Phase 3 — Raw-HTTP platform clients (deferred)

**Not scheduled.** Replacing `gh` / `glab` with direct `net/http` platform clients — which
would let heraut drop those binaries entirely and reach a fully self-contained generator +
publisher — is **explicitly deferred behind its own ADR** (see ADR-0032, "Phase 3"). It
reimplements asset upload, pagination, and rate-limit handling the CLIs currently absorb, and
shifts ongoing API-churn maintenance onto heraut. Listed here only so the epic's full arc is
visible; do **not** start it without a follow-up ADR.
