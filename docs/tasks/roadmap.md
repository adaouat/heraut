# Héraut Build Roadmap

> Status: Active  
> Repo: `github.com/adaouat/heraut`

This roadmap is the executable plan for bringing Héraut to v1.0 with the feature set
described in `docs/specs/`. Each task carries an inline `[ ] / [x]` checkbox — read the
headings for what to do next, read the surrounding prose for *why* and *how*.

The behavioural authority is `docs/specs/` (six numbered specs); the architectural
authority is `docs/adr/` (51 ADRs). Where this roadmap mentions "behaviour", the specs
win; where it mentions a "decision", the ADR wins. If you find a disagreement between
roadmap and spec/ADR, fix the roadmap.

---

## Overview

Héraut is a Go CLI that resolves versions, generates changelogs, and publishes releases to
GitHub / GitLab — wrapping `git`, `gh`, and `glab` for git operations and publishing, with
changelog/release-notes generation built in (`native`, no external generator dependency —
ADR-0045) and PR/MR commit enrichment via direct HTTP against the GitHub/GitLab/Azure DevOps
APIs (ADR-0043). This roadmap captures the work to take it from an empty repo to a v1.0
release.

The goals of v1.0:

1. Implement the full feature set described in `docs/specs/` (four versioning
   strategies, one built-in generator, two publish platforms plus a third forge type for
   enrichment-only, init/check/commit/whatsnew tooling).
2. Establish a clean public home with proper distribution: GitHub Releases (raw
   binaries) and a GHCR container image.
3. Design internal packages with clear boundaries so the foundational ones (exec runner,
   config loading, exit codes, UI theming, update-check) could be extracted into a shared
   Go library later when other CLIs need them. Done: `github.com/adaouat/forge` now
   provides these (see [ADR-0014](../adr/0014-self-update-architecture.md), superseded,
   for the self-update → forge/updatecheck migration).

The `docs/specs/` (six numbered specs) and the 51 ADRs in `docs/adr/` are authoritative.

---

## Working process

Each task follows the two-step roadmap flow defined in
[`.claude/rules/workflow.md`](../../.claude/rules/workflow.md):

1. **Implement** — confirm the task is `[ ]`, then do the work (TDD: failing test first,
   then implementation).
2. **Done** — flip `[ ]` → `[x]`, add a one-paragraph note under the task describing
   actual decisions, deferred items, or deviations. Commit implementation + roadmap
   update together.

Task status markers:

| Marker | Meaning     |
|--------|-------------|
| `[ ]`  | Not started |
| `[x]`  | Done        |

One task at a time. The roadmap always reflects the current state of the branch.
TDD is required — write the failing test before writing implementation code.

---

## Architecture

The current package layout and tech stack live in [`CLAUDE.md`](../../CLAUDE.md) — its
`## Tech stack` and `## Project layout` sections — kept up to date as the codebase
evolves, rather than duplicated here where an earlier, pre-implementation version of this
same tree drifted badly from reality (found and replaced with this pointer, 2026-08-28,
following the docs-audit epic in `docs-audit-roadmap.md`).

For the *rationale* behind specific design choices, see the relevant ADR: strategy
selection and per-env resolution ([ADR-0009](../adr/0009-generic-perenv-resolver.md)),
CLI framework ([ADR-0003](../adr/0003-cli-framework-cobra-fang.md)), binary distribution
([ADR-0013](../adr/0013-raw-binary-goreleaser-format.md)), forge abstraction
([ADR-0043](../adr/0043-forge-abstraction.md)), native as sole generator
([ADR-0045](../adr/0045-native-sole-generator.md)).

---

## Dependency graph

Implementation proceeds bottom-up; vertical slices deliver working functionality at the
end of each phase.

```
Layer D: Documentation foundation
  CLAUDE.md, .claude/rules/, docs/specs/, docs/adr/, docs/tasks/

Layer 0: Repo skeleton
  go.mod, cmd/heraut/main.go, internal/cmd skeleton, GitHub Actions CI, GoReleaser

Layer 1: Core contracts
  internal/port/                  Runner, Generator, Platform interfaces

Layer 2: Infrastructure
  internal/adapter/exec/          shell runner (implements port.Runner)
  internal/testutil/              MockRunner, FakeBin

Layer 3: Config
  internal/config/                structs, loader, path, validator, errors

Layer 4: Versioning foundation
  internal/versioning/tagfmt/     shared tag format (no deps on config)
  internal/versioning/semver/     SemVer resolver + bump
  internal/versioning/calver/     CalVer resolver + parser + format

Layer 5: Per-env resolver
  internal/versioning/perenv/     generic wrapper over semver/calver

Layer 6: App wiring (resolver factory)
  internal/app/resolver.go        NewResolver(cfg, env, force, runner)

Layer 7: Generators
  internal/generators/gitcliff/
  internal/generators/communique/
  internal/generators/cocogitto/

Layer 8: Platforms
  internal/platforms/gitlab/
  internal/platforms/github/

Layer 9: Pipeline
  internal/pipeline/release.go
  internal/pipeline/changelog.go

Layer 10: App wiring (pipeline factory)
  internal/app/pipeline.go        BuildPipeline(), BuildChangelogPipeline()

Layer 11: CLI commands (thin layer)
  internal/cmd/                   cobra commands (package cmd); call app.*

Layer 12: Supporting features
  internal/scaffold/              wizard + YAML generation
  internal/selfupdate/            GitHub Releases API, atomic binary replace

Layer 13: Docs reconciliation, README
```

---

## Testing principles

All code is written test-first. The cycle is **red → green → refactor**.

| Layer       | Scope                                                                                | Tooling                              |
|-------------|--------------------------------------------------------------------------------------|--------------------------------------|
| Unit        | Pure functions (version resolution, config parsing, tag format)                      | `go test`                            |
| Contract    | External CLI interactions (`glab`, `gh`, `git-cliff`, `cog`, `communique`)           | `testutil.MockRunner`                |
| Integration | Full pipeline against real git repo + fake binaries in PATH                          | `go test` + `testutil.FakeBin`       |
| Schema      | `.heraut.yml` validation against `schema.json`                                       | JSON Schema + test fixtures          |

Platform drivers (`gitlab/`, `github/`) must have contract tests that verify the exact
CLI arguments passed to `glab` and `gh`. No platform driver ships without its contract
tests.

See [`.claude/rules/testing.md`](../../.claude/rules/testing.md) for the testing
discipline that applies to every task.

---

## Tasks

### Progress at a glance

| Phase | Title | Status |
|---|---|---|
| D | Documentation Foundation | Done |
| 0 | Repo Bootstrap | Done |
| 1 | Core Contracts and Config | Done |
| 2 | First complete vertical: SemVer + gitcliff + GitHub | Done |
| 3 | Strategy Expansion | Done |
| 4 | Remaining Generators and GitLab Platform | Done |
| 5 | Complete Pipeline Surface | Done |
| 6 | Supporting Features | Done |
| 7 | Doc Reconciliation + Public README | Done |
| 8 | Stable Release Preparation | Done |
| 9 | TUI Polish | Done |
| 10 | Beta Polish | Done — one item open, see "Open items" below |
| 11 | Post-Beta Improvements | Done |
| 12 | Build-ID flow hardening | Done |
| 13 | Per-environment correctness | Done |
| 14 | Multi-platform release correctness | Done |
| 15 | Ticket linking | Done |
| 16 | Multi-instance same-platform releases | Done |
| 17 | Full-project review remediation | Done |
| 18 | `heraut init` config round-trip | Done |
| 19 | Branch-based environment auto-detection | Done |
| 20 | Pipeline UX and error messages | Done |
| 21 | Configurable changelog remote (Azure DevOps and beyond) | Done |
| 22 | Conventional-commit tooling | Done |
| 23 | Native (built-in) content generator | Done — see `native-generator-roadmap.md` |
| 24 | Forge abstraction + unified `forges:` config | Done — see `forge-abstraction-roadmap.md` |
| 25 | Release config simplification | Done — see `release-config-roadmap.md` |
| 26 | Publish-target driver-support awareness | Done |
| 27 | Documentation vs. code audit reconciliation | Done — see `docs-audit-roadmap.md` |
| 28 | Commit tooling enhancements | Done |
| 29 | Rotating changelog file naming | Done — see `changelog-rotation-roadmap.md` |
| 30 | Changelog/release-notes title & subtitle blocks | Done |
| 31 | Uniform empty/whitespace block-override handling | Done |
| 32 | Default footer credits heraut (version + timestamp) | Done |
| 33 | `tagfmt` token API cleanup | Done |
| 34 | Scoped-changelog `--regenerate` leaks out-of-scope history into the oldest section | Done |
| 35 | `heraut init` can emit an invalid rotation/per-env combination | Done |

### Open items

The single unchecked item across the entire roadmap — Phase 10's closing checkpoint:

#### ✦ `[x]` CHECKPOINT K — Beta polish complete, ready for v1.0.0

- [x] Release workflow attestation steps pass (T43 — `attestations: write` added)
- [x] `heraut self-update` hint does not fire immediately after a successful update (T44)
- [x] `heraut init` with empty changelog output defaults to `CHANGELOG.md` (T45)
- [x] `heraut check runtime` shows errors only for tools the active config requires (T46)
- [x] `heraut check runtime` fails fast on invalid/expired platform credentials (T47)
- [x] `versioning.tag_prefix` replaces `versioning.prefix` throughout (T48)
- [x] `go test ./...` passes; 675 tests across 23 packages
- [x] Docker build splits into parallel native-runner matrix; wall-clock ≤ 20 min (T49)
- [ ] v1.0.0 cut by running `heraut release` on the heraut repo itself

All Phase 10 tasks (T43–T49) shipped. `heraut check runtime` now displays a clean
three-section TUI (Git / Platforms / Generators) with config-aware required vs optional
tool checks and full API auth verification for configured platforms. The release workflow
uses native-runner parallel Docker builds and proper attestation. The one remaining item
is cutting v1.0.0 itself — all quality gates are green.

---

### Phase 32 — Default footer credits heraut (version + timestamp)

#### ✦ `[x]` T252: make the heraut-credit footer text the built-in default

The native generator's `footer` block shipped empty by default (`internal/generators/native/blocks.tmpl`)
— crediting heraut in the changelog/release notes required an explicit `rendering.templates.footer`
override, as this project's own `.config/heraut.yml` briefly did. Per user decision, the credit line
becomes the **built-in default** for every project instead: `_Generated by [heraut](url) version at
HH:MM on YYYY-MM-DD._`, reachable via `.Heraut.URL` / `.Heraut.Version` / `.Heraut.GeneratedAt` (root is
`Release`, same as every other non-title/subtitle block). This is a deliberate breaking change to
default output, surfaced and confirmed with the user before implementation: `.Heraut.GeneratedAt` is
wall-clock, so the footer's timestamp — and therefore the rendered file — changes on every
`heraut changelog`/`heraut release` run even absent new commits, trading default-output determinism for
out-of-the-box heraut credit. TDD: added `TestGenerate_DefaultFooterCreditsHeraut` (failing on empty
`blocks.tmpl` footer, passing once the define was filled in) exercising the full `Generator.Generate`
path with an injected clock. Fallout: 9 golden-equality tests in `render_internal_test.go` broke because
they fed a zero-value `tplHeraut{}` into the root template, which the now-non-empty footer rendered
literally (`_Generated by [heraut]()  at 00:00 on 0001-01-01._`) — fixed by introducing a shared
`fixtureHeraut` fixture (realistic version/URL/date) for those 9 call sites only, regenerating goldens
via `UPDATE_GOLDEN=1`, and reviewing every diff (each added exactly the expected footer line, nothing
else changed). Non-golden tests at those same call sites (`TicketBlockOverride`, `TypesHeadingLevel`)
were left on `tplHeraut{}` since they assert via `Contains`/`NotContains` and don't compare full output.
Docs updated: `docs/guides/template-customization.md` (block table's footer default, the
`.Heraut.GeneratedAt` determinism note — now describes what the default *does* render rather than
claiming built-ins never render it, plus a pointer to `footer: ""` for anyone who wants deterministic
output back — and the worked example, repurposed from "add heraut credit" to "drop the timestamp for
determinism" since credit is no longer opt-in). This project's own `.config/heraut.yml` override
(added transiently while iterating on the exact wording) was reverted — redundant now that it matches
the default. **Superseded same-session by T253 below** — placing `footer` on the per-release root was
itself the wrong scope; see T253.

#### ✦ `[x]` T253: split `footer` into a document-level block + `release_footer` (ADR-0049)

T252's placement was wrong: `footer` lived on the per-release root (`changelog`/`release_notes`),
so it rendered once per rendered *release*, not once per *document*. Invisible for release notes
(always exactly one release per render) but not for `changelog` — `buildAllSections` renders one
section per tag and the incremental splice path preserves every historical section's previously
rendered text verbatim, so a `CHANGELOG.md` accumulated over N releases would carry the credit line
repeated after **every** section instead of once at the bottom. Caught by inspection before any
tagged release shipped it — structurally the same problem [ADR-0048](../adr/0048-changelog-title-subtitle-blocks.md)
solved for `title`/`subtitle`, which cannot live on the per-release root either. Fix, full design
and rationale in [ADR-0049](../adr/0049-changelog-release-notes-footer-block.md): renamed the old
per-release block `footer` → `release_footer` (unchanged otherwise — `Release`-rooted, empty
default); added a new document-level `footer` (bare `tplHeraut` context, same as `title`/`subtitle`,
credit-line default) that fires exactly once, via a new `renderPostamble` helper
(`internal/generators/native/render.go`) mirroring `renderPreamble` — `buildAllSections` and
`renderReleaseNotes` each call it once and append the result. No change needed in `spliceSection`:
content trailing the last anchor is already preserved verbatim by the existing splice algorithm, so
a document footer appended once at bootstrap/`--regenerate` survives every subsequent incremental
splice untouched, exactly like the preamble — confirmed with a real end-to-end run against this
repo's full git history (dozens of releases via `--regenerate`): the footer appears exactly once, at
the true end of the file. TDD: five new/renamed tests in `generator_internal_test.go`
(`TestGenerate_ReleaseFooterOverride` renamed from the old `HerautMetaInFooter`,
`TestGenerate_DefaultDocumentFooterCreditsHeraut`, `TestGenerate_DocumentFooterContextIsHerautOnly`,
and the two regression tests that directly reproduce the bug —
`TestGenerateChangelog_DocumentFooterOnceAcrossMultipleSections` and
`TestGenerator_GenerateChangelog_IncrementalPreservesDocumentFooter`, both red against T252's code,
green after the fix) plus four new `renderPostamble` unit tests in `render_internal_test.go` mirroring
the existing `renderPreamble` suite. `config/validator.go`'s `validTemplateBlocks` and `schema.json`
gained `release_footer`; the 5 changelog golden files lost their (now-incorrect) per-section footer
line — the 4 release-notes goldens were byte-identical, confirming release notes' footer placement
was never actually wrong (release notes only ever renders one section anyway). Docs updated:
`template-customization.md` (block table, the `footer`-vs-`release_footer` distinction, the "fire
once" paragraph, the `.Heraut` reachability note, the Gotchas section, the "All four layers"
worked example — which had been using `.CompareURL` under `footer`, now moved to `release_footer`
where that field is actually reachable), `docs/specs/05-generators-and-platforms.md`'s overridable-
blocks paragraph, `docs/heraut.sample.yml`'s templates comment block and worked example. ADR count
references (`CLAUDE.md`, this roadmap) bumped 47 → 49.

#### ✦ `[x]` T254: preamble/postamble always render fresh, no `--regenerate` needed (ADR-0050)

T253 documented the (then-correct) behavior that `title`/`subtitle`/`footer` froze at whatever was
on disk after bootstrap, refreshed only by `--regenerate-changelog`/`--regenerate` — inherited from
ADR-0038's "preamble preserved verbatim" line. User pointed out the asymmetry directly: release
notes already refresh all three every single run (no incremental concept there to freeze anything),
so the changelog should behave the same way — no `--regenerate` requirement, ever. Full design and
rationale in [ADR-0050](../adr/0050-changelog-preamble-postamble-always-fresh.md), which supersedes
that ADR-0038 line and ADR-0049's frozen-until-regenerate framing. Mechanism: `generateIncremental`
now calls `renderPreamble`/`renderPostamble` unconditionally (previously only the bootstrap/
regenerate path, `buildAllSections`, did) and passes the fresh values into `spliceSection`, which no
longer reuses whatever `parseChangelog` found on disk for either. The preamble needed no new
bookkeeping — `parseChangelog` already knows its boundary for free (everything before the first
section anchor). The postamble did: nothing bounded where the true last section's body ended and a
trailing footer began, so a document footer would either get silently re-glued onto whatever tag is
currently oldest (if left alone) or duplicated (if simply appended fresh every time). Added a second
structural, non-overridable marker, `<!-- heraut-footer -->` (`internal/generators/native/
changelogfile.go`), placed immediately before the rendered postamble — same pattern the section
anchors already use — so `parseChangelog` strips the whole footer region before finding section
boundaries. `--regenerate`/`--regenerate-changelog` keeps its existing meaning exactly: it still
controls whether historical *sections* get re-enriched (the actually expensive part, up to one API
call per commit on GitLab REST); that flag no longer has anything to do with preamble/postamble
freshness. Considered (and rejected, per explicit user decision — this asymmetry was the actual
complaint) mutualizing further by giving every section an explicit start+end marker pair instead of
inferring boundaries from adjacent anchors: rejected as a breaking change to the already-shipped
`<!-- heraut-release: vX -->` format for zero practical benefit — every boundary except the trailing
one already has an anchor to lean on for free. TDD: `TestParseChangelog_StripsFooterRegion`,
`TestSpliceSection_RefreshesPreambleAndPostamble`, `TestSpliceSection_NoPostambleOmitsAnchor` (new,
`changelogfile_internal_test.go`); `TestGenerator_GenerateChangelog_IncrementalPreservesExisting-
Title`/`..._IncrementalPreservesDocumentFooter` renamed and inverted to `..._IncrementalRefreshes-
Title`/`..._IncrementalRefreshesDocumentFooter`, now asserting the opposite of what T252/T253
asserted — a deliberate behavior change, not a bug fix to those tests. Verified with a live
incremental run against this repo's own scratch config. Docs: `template-customization.md` (the
`.Heraut.GeneratedAt` paragraph, a Gotchas bullet, the intro citation list), `docs/specs/
05-generators-and-platforms.md`'s Incremental/Full-regeneration paragraphs (previously claimed
regeneration was "the one case where free-form preamble doesn't mean yours to keep" — now neither
mode preserves it). ADR count bumped 49 → 50.

#### ✦ `[x]` T255: automatic visual separator before `footer` (ADR-0051)

User asked for a clean visual break: an empty line, then a horizontal rule, then the footer
content. Previously `footer` rendered immediately under the last commit bullet with no spacing.
Design in [ADR-0051](../adr/0051-footer-visual-separator.md): made the separator (`"\n\n---\n"`,
`footerSeparator` in `changelogfile.go`) structural — applied by a new shared `appendFooter` helper
whenever `footer` is non-empty, whether default or a user override, same category as the anchors
rather than something baked into the block's own template text (which would have been silently
swallowed by `renderPostamble`'s `TrimSpace`, the same trim `renderPreamble` applies to
`title`/`subtitle` for consistent, override-formatting-independent presentation).
`renderPostamble`'s return contract simplified to just the trimmed block text (previously wrapped
in its own leading/trailing newline) — every caller now owns its own presentation:
`buildAllSections`/`spliceSection` via `appendFooter` (anchor + separator + text), and
`renderReleaseNotes` inline (separator + text, no anchor needed since release notes has no
persisted file to re-parse). TDD: updated the three `TestRenderPostamble_*` tests pinning the old
wrapped-string contract to the new trimmed one; regenerated the 4 release-notes goldens (each
gained exactly the blank line + `---` before its footer line — the changelog goldens were already
footer-free since ADR-0049 moved footer out of the per-section render they test, so unaffected).
Verified with a live scratch run showing the exact intended shape (last commit line → blank line →
`---` → footer text). Docs: `template-customization.md` (new paragraph on the automatic separator,
the intro ADR citation list tightened while adding 0051). ADR count bumped 50 → 51.

---

### Phase 33 — `tagfmt` token API cleanup

#### ✦ `[x]` T256: replace positional token args with a `Tokens` struct

`internal/versioning/tagfmt.Render`, `DeriveTagPattern`, and `GlobPattern` each took `env`,
`version`, and/or `build` as separate positional string params. Replaced with a single
`Tokens{Version, Env, Build string}` struct argument across all three functions and their 10 call
sites (`internal/app/{current,resolver,pipeline}.go`, `internal/versioning/perenv/{promote,auto}.go`).
`ParseVersion` and `DeriveHeadingVersionPattern` take no token values (only `template`, or
`template`+`tag`) and were left untouched. Pure signature refactor — every substitution/output
behavior is unchanged; only the four `TestRender*`/`TestDeriveTagPattern`/`TestGlobPattern*` call
shapes in `tagfmt_test.go` changed to match. TDD: updated the test file to the new signature first
(compile failure = red), then added `Tokens` and re-shaped the three functions (green), then fixed
up the 10 call sites one package at a time until `go build ./...` and `go test ./...` were clean
again. `hk check` (golangci-lint, gofmt, typos) passes with 0 issues. Surfaced during monorepo-
support design exploration (docs/superpowers/specs — no design doc committed yet, still in-chat
brainstorming) as standalone prep work: a clean seam for a future `{package}` token, landed on
`main` independently of (and before) the still-unfinished monorepo design, which will continue in
its own dedicated worktree.

---

### Phase 34 — Scoped-changelog `--regenerate` leaks out-of-scope history into the oldest section

#### ✦ `[x]` T257: bound the oldest scoped release's commit range against its true previous tag

Surfaced by a user question during the footer work (Phase 32): does a rotated `changelog.output`
(e.g. `CHANGELOG_{YYYY}.md`) fetch/enrich only the current bucket's commits, or everything? Answer
for the common path: correctly scoped — `buildAllSections`'s historical loop bounds every section
by `prev..tag` using the *next-older tag in the scoped list*. But that broke down for exactly one
case, confirmed by direct reproduction before writing any fix: the **oldest** tag within a scope,
re-rendered via `--regenerate` when that scope already has 2+ releases. `prev := ""; if i+1 <
len(tags) { prev = tags[i+1] }` leaves `prev = ""` for the last item in the *scoped* list — which
`commitRange` turns into "walk from the very beginning of history," silently absorbing (and
re-enriching) every prior-scope commit into the wrong section. Verified this is generic, not
rotation-specific: reproduced identically against `internal/generators/native` directly using
per-environment `TagGlob` scoping (`prod/v*`) with an unrelated `staging/v1.0.0` tag preceding it —
same root cause, same file, unrelated to Phase 29's rotation code entirely. This is the same class
of gap T247 already fixed for the *newest* section (`PreviousTagOverride`/`newSectionBound`); it
was never extended to the historical loop, because until this investigation nobody had exercised
`--regenerate` against a scope with more than one prior release in it.

Fix: when the historical loop reaches the last item in an *active* scope (`TagGlob != "" ||
TagPattern != ""` — a plain unscoped changelog's list is already the full unfiltered history, so
`prev = ""` there is already correct and the branch is skipped, no extra git call), resolve the
true previous tag via `previousTag(runner, t, "")` — no `--match`, so it walks `git describe`
topology across *any* tag name — the exact same primitive `scopedPreviousTag` already uses for the
identical "regardless of scope" reason. Deliberately left `newSectionBound`/`PreviousTagOverride`/
`latestMatchingTag` (the already-correct rotation-specific mechanism) untouched — only the
historical loop needed the gap closed, and generalizing further wasn't necessary to fix the
confirmed bug. TDD:
`TestGenerateChangelog_RegenerateOldestScopedTagExcludesOutOfScopeHistory` (new, MockRunner-based,
`internal/generators/native/generator_internal_test.go`) reproduces the per-env case end-to-end and
pins the exact `git describe --tags --abbrev=0 <tag>^` call; red against the unfixed code (the
mocked response for that call gets consumed by the wrong git subcommand instead, producing a
parse error — confirming the call genuinely wasn't being made). `TestGenerator_GenerateChangelog_-
TagGlob` needed one extra queued response (a "no earlier tag" no-op) since it now always exercises
this path when scoped. Verified against both real scenarios with actual git repos (temporary,
deleted after): the original CalVer-rotation reproduction (`2025.12.0` → `2026.01.0` → `2026.02.0`,
regenerate the 2026 bucket) and the per-env reproduction — both confirmed fixed, no docs/ADR needed
(a correctness fix extending T247's already-established design intent, not a new decision).

---

### Phase 35 — `heraut init` can emit an invalid rotation/per-env combination

#### ✦ `[x]` T258: live-validate `changelog.output` against strategy/format in the wizard

User-reported: `heraut init` let them pick `calver-per-env` and then type
`CHANGELOG_{YYYY}.md` for the changelog output with no feedback, producing a `.heraut.yml` that
failed on the very next command — `changelog.output: rotation tokens ... are not supported with
calver-per-env yet`, exactly [T246](changelog-rotation-roadmap.md)'s deliberate, correct rejection
(rotation + per-env genuinely isn't implemented, this was never a validator bug). The bug was the
wizard: `internal/scaffold/wizard.go`'s "Changelog output file" field was a bare `huh.NewInput()`
with no `.Validate(...)` at all — unlike the neighboring "Custom CalVer format" field, which already
had one (`ValidateCalVerFormat`) — so any `{TOKEN}`/strategy mismatch (not just the per-env case)
sailed straight through to a file the user couldn't use. Fix: exported
`config.ValidateChangelogRotationForWizard(strategy, calverFormat, output string) error`
(`internal/config/changelog_rotation.go`), a thin wrapper around the exact same
`validateChangelogRotation` logic `config.Validate` already runs at load time — single source of
truth rather than a hand-rolled duplicate subset, so it catches every rule that function knows about
(invalid token names, tokens outside the format, wrong order, per-env) not just the one case
reported. Wired it into the wizard's field via `.Validate(...)`, reading `a.Strategy` (already set
by the earlier group in the same sequential form) and reconstructing the in-progress CalVer format
from `formatChoice`/`customFormat` (the form's own locals — `a.Format` itself isn't finalized until
after the whole form returns, so the closure can't read it directly). TDD: added
`TestValidateChangelogRotationForWizard` (`internal/config/validator_test.go`, 6 cases including the
exact reported repro) — failed to compile against the not-yet-existing function, green once added.
Wizard wiring itself isn't separately unit-tested — same as `ValidateCalVerFormat`'s existing
wiring, huh forms aren't practical to drive in a unit test — verified by code inspection plus
`go build`/`go vet`. No ADR: this closes a UX gap in `heraut init`, it doesn't change what's
actually supported (per-env + rotation still isn't, same as before).

#### ✦ `[x]` T259: live-validate the rest of the per-env wizard fields the same way

Follow-up audit after T258, prompted by the user asking directly whether *every* wizard field was
now correctly validated — it wasn't. Went through every `huh.New*()` field in `wizard.go` against
`internal/config/validator.go`'s actual rules and found three more instances of the same class of
bug (wizard accepts what `config.Validate` rejects later) plus one worse case (silent data loss, no
error at any stage):

1. **"Common tag format (per-env)"** (`a.TagFormat`) and **"Tag format override"** (per-env
   `env.TagFormat`) — neither validated `validatePerEnv`'s `{version}`-token requirement live.
   Fixed by extracting the shared boolean check (`tagFormatMissingVersion`) out of
   `validatePerEnv`'s two existing inline call sites and exposing it as
   `config.ValidateTagFormatForWizard(s string) error` — same pattern as T258's
   `ValidateChangelogRotationForWizard`, single source of truth, both wizard fields wired to it.
2. **"Source environment (promote mode)"** (`env.Source`) — free text, no validation, but
   `validatePerEnv` requires it to name an existing environment and not itself. New
   `validateEnvSource(existing []EnvAnswer, currentName, source string) error` in
   `internal/scaffold/wizard.go`, scoped to what the wizard's sequential one-at-a-time loop can
   actually know: checks only against environments defined *earlier* in this run (a source naming
   one added later isn't live-checkable without over-constraining valid definition order); empty
   stays valid — `validatePerEnv` accepts it as "auto-detect the sole auto environment," a
   cross-environment count this per-field check deliberately doesn't attempt to replicate (would
   need to see the *whole* environment set to avoid false positives, not just what's typed so far).
3. **"Environment name"** (`env.Name`) — a genuinely different, worse bug: `generate.go` assigns
   `cfg.Environments[e.Name] = ...`, a plain map write, so two environments sharing a name (or two
   left blank) silently overwrite each other with **no error anywhere** — not even
   `config.Validate`, since by validation time the duplicate is already gone. New
   `validateEnvName(existing []EnvAnswer, name string) error` rejects both empty and
   already-used names.

Deliberately **not** touched: `api_mode: graphql` + a manually-typed `CI_JOB_TOKEN` bypassing
`hideAPIMode`'s Select-level guard — confirmed this is a different class of gap entirely
(`config.Validate` doesn't check this combination at all, by design — `T157`, a documented
resolution-time-only concern — so it fails identically whether the config was hand-written or
wizard-generated; fixing it isn't a wizard-vs-validator mismatch to close). TDD: `TestValidateTag-
FormatForWizard` (`internal/config/validator_test.go`), `TestValidateEnvName`/`TestValidateEnv-
Source` (`internal/scaffold/wizard_internal_test.go`) — all red (undefined function) before the
implementations existed, green after. Existing `TestValidate_rotationOutput_*`/tag_format tests
confirm the `validatePerEnv` refactor changed no behavior. Wizard field wiring itself not
separately unit-tested, same reasoning as T258. No ADR — UX-gap closure, no behavior change to what
`config.Validate` actually accepts.

---

### Active epics tracked in their own file

Phases 23, 24, 25, 27, and 29 are heavy, multi-phase epics whose task breakdown and live
`[ ] / [x]` status live in a dedicated roadmap file instead of inline here — this file keeps only
a navigable summary for each.

### Phase 23 — Native (built-in) content generator

Per [ADR-0032](../adr/0032-native-content-generator.md) (generator) and
[ADR-0033](../adr/0033-native-config-model.md) (config model): a pure-Go `generator: native`
that becomes heraut's **canonical** changelog / release-notes renderer, **driven by config**
(unified `commits:` + `rendering:` blocks). git-cliff is dropped as the design anchor; its
package removal is deferred (after native enrichment, own ADR). Heavy and multi-phase, so the
task breakdown **and live `[ ] / [x]` status** live in a dedicated roadmap:

→ **[Native Content Generator Roadmap](native-generator-roadmap.md)** — T122+

Summary of the arc (full detail, tests, and files in the dedicated file):

- **Phase 1** — config model + native canonical renderer: T130 `commits`/`rendering` config,
  T131 migrate commit verify/create, T122 commit collection, T123/T124 (landed) reworked
  config-driven by T132/T133, T125 wire native canonical, T126 canonical golden snapshots.
- **Phase 2** — remote enrichment via platform CLIs: T127 GitHub (`gh api`), T128 GitLab
  (`glab api`), T129 Azure DevOps.
- **Phase 2.5** — remove the git-cliff package: deferred, own ADR (after native enrichment).
- **Phase 3** — raw-HTTP clients to drop `gh` / `glab`: deferred behind a future ADR.

---

### Phase 24 — Forge abstraction + unified `forges:` config

A single top-level `forges:` list (a forge = one code-hosting platform heraut talks to) replaces
`changelog.remote` + `release.platforms`, and a new `port.Forge` resolves its identity from **CI env
or git `origin`** (fail loud on ambiguity), builds links, and fetches enrichment metadata. GitLab
gains a native `net/http` enricher (REST default, `JOB-TOKEN`-aware) so `CI_JOB_TOKEN` enriches with
**zero config** — no manual PAT — plus an opt-in GraphQL path (`api_mode: graphql`) for linked
commit-author handles. Consumers reference a forge by name: `commits.enrichment_forge` and
`release.targets[].forge`; `commits.remote_metadata` → `commits.enrichment_policy`. Breaking config
change (pre-v1.0) under new ADR-0043. Heavy and multi-phase, so the task breakdown **and live
`[ ] / [x]` status** live in a dedicated roadmap:

→ **[Forge Abstraction Roadmap](forge-abstraction-roadmap.md)** — T154+

Design: [`docs/superpowers/specs/2026-07-24-forge-abstraction-design.md`](../superpowers/specs/2026-07-24-forge-abstraction-design.md).

Summary of the arc (full detail, tests, and files in the dedicated file):

- **P1** — GitLab-first: `port.Forge` + config (`forges:` / `release.targets:` /
  `commits.enrichment_*`) + resolution + native REST/GraphQL forge + links + migration (T154–T160).
- **P2** — migrate GitHub + Azure onto `port.Forge`, retire the enrich switch (T161, T162).
- **P3** — `release.targets` replaces `release.platforms` as the publishing surface; the
  transport (`gh`/`glab`) deliberately stays unchanged — see ADR-0044 (T163).
- **P4 (last)** — `heraut init` wizard generates the forge config, after the schema is
  battle-tested (T164).

---

### Phase 25 — Release config simplification

`release:` collapses from two independently-optional axes (`notes`, `targets`) into one atomic
intent: block presence (even `release: {}`) means "generate notes and publish them," root and
per-environment alike — no config shape splits the two anymore. `release.notes` stops being an
on/off toggle and becomes a rendering-customization sub-block, default-populated the same way
`changelog: {}` already defaults `Output` to `CHANGELOG.md`. Supersedes T214 (this session): its
`notesConfigured` synthesis gate protected a "notes only, no publish" state that traced to nothing —
`heraut release` generated the notes string and discarded it when there was no publish target,
confirmed by tracing `Run()` end to end; no command ever surfaced it. Per-environment `disable_notes`
is renamed `disable_release` (turns off the whole block for that environment, not half of it) via a
hard removed-key error — pre-v1.0, no deprecation window, matching ADR-0028's precedent. New
ADR-0046. Breaking config change, lands before the v1.0.0 cut. Small, single-pass epic, so the task
breakdown **and live `[ ] / [x]` status** live in a dedicated roadmap:

→ **[Release Config Roadmap](release-config-roadmap.md)** — T216+

Design: [`docs/superpowers/specs/2026-08-26-release-config-simplification-design.md`](../superpowers/specs/2026-08-26-release-config-simplification-design.md).

---

### Phase 27 — Documentation vs. code audit reconciliation

A full audit (four parallel Opus passes over specs, ADRs, `CLAUDE.md`/`.claude/rules/`, and
`schema.json`/`heraut.sample.yml` against current code) found 142 doc/code mismatches, seven of
which are real code bugs the docs happened to expose rather than pure documentation drift (e.g.
`--version` ignoring `tag_prefix`, per-environment `release:` never getting `Notes`
default-populated per ADR-0046, dead `rendering.excludes`). Too large for one task; broken out —
bug fixes kept separate from doc-only reconciliation so they can be prioritized and reviewed
independently:

→ **[Documentation Audit Roadmap](docs-audit-roadmap.md)** — T222+

---

### Phase 29 — Rotating changelog file naming

`changelog.output` (and its per-env override) gains optional CalVer/SemVer tokens (`{YYYY}`,
`{MAJOR}`, …) so a project's changelog can rotate into one file per calendar period or per release
line, with tag-scoping auto-derived from the same tokens — no second field to keep in sync, no
change to native's anchor/bootstrap/splice algorithm. Resolves T243. New ADR-0047. Six tasks
(pure-function token helpers → config validation → app-layer wiring → docs), so the task breakdown
and live `[ ] / [x]` status live in a dedicated roadmap:

→ **[Changelog Rotation Roadmap](changelog-rotation-roadmap.md)** — T244+

Design: [`docs/superpowers/specs/2026-08-28-changelog-rotation-design.md`](../superpowers/specs/2026-08-28-changelog-rotation-design.md).

---

### Archived task detail

Every Phase above marked `Done` and not pointing to a dedicated roadmap (D, 0–9, 11–22, 26, 28,
30, 31) has had its full task write-up — implementation notes, decisions, deviations — moved to
[`archive/roadmap-phases.md`](archive/roadmap-phases.md) to keep this file lean. Nothing is lost,
just relocated; the table above is the status index.

---

## Risks and mitigations

| Risk                                                                                | Impact            | Mitigation                                                                |
|-------------------------------------------------------------------------------------|-------------------|---------------------------------------------------------------------------|
| `perenv.VersionCalculator` interface doesn't cleanly fit both semver and calver     | High — blocks T12 | Prototype with semver first (T10); validate with calver before locking it |
| GitHub Actions release pipeline + GoReleaser GHCR push requires setup               | Med — blocks T02  | Test release pipeline on a scratch tag early; don't wait until T24        |
| Self-update on macOS blocked by quarantine / Gatekeeper on downloaded binaries      | Med — poor UX     | Implement `xattr -d com.apple.quarantine` step post-download in T21       |
| `heraut check runtime` needs `git user.name` which may not be set in CI containers  | Low — known issue | Detect the missing config, print an actionable hint, exit non-zero        |

---

## Resolved questions

| Question                                                                            | Resolution                                                                |
|-------------------------------------------------------------------------------------|---------------------------------------------------------------------------|
| Module path                                                                         | `github.com/adaouat/heraut`                                               |
| GoReleaser release ownership for the heraut repo's own releases                     | heraut owns GitHub Release creation (T51 / ADR-0018); goreleaser is build-only (`release: disable: true`) |
| Docker image registry                                                               | `ghcr.io/adaouat/heraut`                                                  |
| Dev tooling                                                                         | `mise` + `hk` (already configured in `.config/`)                          |
| Self-update version check                                                           | GitHub Releases API directly (no Pages hosting)                           |
| ADR numbering                                                                       | Sequential from 0001, no gaps                                             |
| Spec layout                                                                         | Six numbered files in `docs/specs/`                                       |
