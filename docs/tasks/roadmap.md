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
| 36 | GPG-signed commits/tags hang: subprocess stdin was never wired to the terminal | Done |
| 37 | Skip version bump for no-op-only releases | Done — see ADR-0052 |
| 38 | Track every PR for a first-time contributor, not just the first | Done |
| 39 | Rename `--version`/`--build` override flags to disambiguate from root's `--version` | Done |
| 40 | Give `heraut init` its own `--overwrite` flag instead of overloading root's `--force` | Done |
| 41 | `heraut version sprint bump` respects `--dry-run` | Done |
| 42 | Scope `--dry-run`/`--env`/`--force`/`--offline` to the commands that use them, off root | Done |
| 43 | Release lifecycle hooks | Done — see `release-hooks-roadmap.md` |
| 44 | Windows hook execution | Done |

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

### Phase 36 — GPG-signed commits/tags hang: subprocess stdin was never wired to the terminal

#### ✦ `[x]` T260: run `git commit`/a signed tag through an interactive runner

User-reported: `heraut changelog`'s commit step failed with `gpg: signing failed: Timeout` in a
fresh shell (gpg-agent had nothing cached yet). Traced to `github.com/adaouat/forge`'s
`exec.CmdRunner.RunDir` (the concrete implementation behind `port.Runner`): it always built the
child command with `cmd.Stdout`/`cmd.Stderr` pointed at capture buffers and never set `cmd.Stdin`
at all — Go's `os/exec` reads that as "connect to `/dev/null`." Every subprocess heraut spawns,
`git commit` included, had no path back to the terminal. `commit.gpgsign=true` makes `git commit`
invoke GPG, which launches `pinentry-curses` to prompt for the passphrase — the log showed it
correctly finding the terminal (`PINENTRY_LAUNCHED ... /dev/ttys008`) but then timing out, since
nothing could reach it. Drafted feedback for the forge maintainer proposing an opt-in interactive
mode; **v0.19.0 shipped it** as `CmdRunner.Interactive bool` — when true, connects stdin/stdout/
stderr directly to the terminal instead of capturing (return values become empty strings in that
mode). Upgraded `github.com/adaouat/forge` v0.18.0 → v0.19.0 (`go get` + `go mod tidy`).

Wiring it into heraut needed care: `Interactive` is a whole-`CmdRunner` setting, and heraut's
existing write runner is reused for *every* git call in a pipeline run (`git add`, `git diff
--cached`, `git push`, tag creation, …) — most of which depend on captured stdout to work at all.
Flipping `Interactive` on the runner used for those would break them, not fix commit signing. So:
`gitHelper` (`internal/pipeline/git.go`) gained a second, optional `interactiveRunner` field and a
`runInteractive` helper that falls back to the regular runner when it's unset — meaning every
pre-existing `gitHelper{runner: mr}` test construction (and any caller that hasn't opted in)
behaves exactly as before, zero test churn. Only `commitChangelog`'s `git commit` call and `tag`'s
`git tag -s` branch (the two calls that can actually invoke GPG) route through it; `git add`,
`git diff --cached`, `git push`, and unsigned/annotated tags stay on the regular runner. Threaded
via a `WithInteractiveRunner` chaining method on both `Pipeline` and `ChangelogPipeline` — the same
pattern as the existing `WithReporter`/`WithLogger`, so `New`/`NewChangelog`'s constructor
signatures (and every test calling them) didn't need to change either. `app.PipelineOpts` gained an
`InteractiveRunner port.Runner` field threaded through `BuildPipeline`/`BuildChangelogPipeline`;
`internal/cmd/release.go` and `internal/cmd/changelog.go` each construct a second
`execadapter.New(dryRun, verbose)` with `.Interactive = true` set, alongside the existing runner
and readRunner.

TDD: `TestCommitChangelog_UsesInteractiveRunnerForCommit`, `TestCommitChangelog_NoInteractiveRunner-
FallsBackToRegular`, `TestTag_Signed_UsesInteractiveRunner`, `TestTag_Unsigned_UsesRegularRunner`
(`internal/pipeline/git_test.go`) — using two separate `MockRunner`s (regular + interactive) and
asserting each call landed on the right one. Verified end-to-end against a real scratch git repo
(`heraut changelog --commit --no-push`, no signing configured): committed correctly, and git's own
commit summary now appears live in heraut's output — visible confirmation the stream is connected
directly rather than captured. No ADR — this fixes a genuine defect (a hang with no workaround
inside heraut) using a capability the dependency now provides, it doesn't introduce a new design
decision.

---

### Phase 37 — Skip version bump for no-op-only releases

#### ✦ `[x]` T261: exclude commit types from bump determination

`versioning.bump` changed from a plain string to an object (`config.BumpConfig{Mode, Overrides}`,
ADR-0052 — a deliberate pre-v1.0 breaking config-shape change, no back-compat shim): `mode`
(`auto`/`manual`, defaulting to `auto`) plus `overrides`, a list of `config.BumpRule{Type, Regex,
Breaking *bool, Bump}` rules reusing the same `type`/`regex` matcher shape as `rendering.excludes`
rather than a standalone mechanism. `DetermineBump(commits []string, overrides []config.BumpRule)`
resolves each commit's level by checking, in order: the first matching user rule (all of its set
conditions — type, regex against the subject, breaking — must hold; unset conditions are
wildcards), then the built-in defaults (breaking → major, `feat` → minor, else patch), then "no
match, not conventional" → contributes nothing. A rule can override the previously-hardcoded
"breaking is always major" too (`{breaking: true, bump: minor}`), fully answering the "can this be
made configurable" follow-up. The release's bump is the highest level any commit contributes;
`versioning.BumpNone` now correctly means "nothing to release."

Also fixed along the way: `BumpVersion`'s switch had no `BumpNone` case and fell to the same
`patch++` branch as `BumpPatch` — a latent bug, since `BumpNone` was previously only reachable via
paths that never call `BumpVersion`. Confirmed out of scope: calver/perenv never call
`DetermineBump` (calver has no "bump amount" concept), so this is inherently SemVer-only;
`semver-per-env`'s "auto" environments get it for free via the shared `BumpAuto`.

`resolveAuto` and `BumpAuto` now return a dedicated error when resolution yields `BumpNone` with
real commits present — distinct from the pre-existing "no commits at all" error — listing each
excluded commit's subject line:
```
no releasable commits since v0.62.0: 3 commit(s) since then are excluded from the version bump
  - chore: bump deps
```
Commit hashes were deliberately left out of the list: including them would require changing the
`git log --format` in two independent places (`semver`'s own call, and `perenv`'s separate fetch
feeding the shared `VersionCalculator.BumpAuto` interface) for a display-only improvement.

TDD: `TestBumpVersion_NoneLeavesVersionUnchanged`; `TestDetermineBump`'s table extended in place
(existing rows kept, `nil` overrides) with 8 new override-behavior rows (type/regex/breaking
matching, first-match-wins ordering, breaking-override, `breaking: false` scoping);
`TestResolve_AllCommitsExcluded_Error` / `TestBumpAuto_AllCommitsExcluded_Error` for the new error;
config-layer: `TestValidate_bumpOverride_*` (missing matcher, type+regex both set, invalid regex,
invalid bump level, breaking-only-matcher valid) plus the two existing bump-mode tests adapted to
the nested shape. `schema.json` (`BumpConfig`/`BumpRule` definitions), `docs/heraut.sample.yml`,
`docs/specs/04-versioning.md` (new "Bump-level overrides" section), `docs/specs/02-configuration.md`,
`README.md`, this repo's own `.config/heraut.yml`, and every `testdata/config/valid/*.yml` fixture
with a top-level `versioning.bump` were updated to the new shape. Full `go test ./...` and
`hk check` (golangci-lint, gofmt, yamlfmt, typos) clean.

---

### Phase 38 — Track every PR for a first-time contributor, not just the first

#### ✦ `[x]` T262: credit every PR a first-time contributor opened, not just their first

`Contributor.PR *PullRequest` (`internal/generators/native/model.go`) became `PRs []PullRequest`.
`collectContributors` (`contributors.go`) no longer `break`s after the first PR-bearing commit for
an email — it scans every commit for that contributor in the release, dedupes by PR `Number` (a
rebase-merged PR can attach the same number to several commits), and appends each distinct PR in
first-seen order; the handle overlay still comes from the *first* PR-bearing commit only, unchanged.
`tplContributor.PR *tplPR` → `PRs []tplPR` (`templatemodel.go`'s `buildContributors` maps the full
slice via the existing `tplPRFrom`, unchanged itself). `blocks.tmpl`'s `contributor` block replaced
its single `{{ if .PR }}` guard with `{{ range $i, $pr := .PRs }}{{ if $i }}, {{ end }}[...]{{ end }}`
— comma-joins multiple refs, and is byte-identical to the old single-PR output when there's only
one (confirmed: `TestRenderReleaseNotes_Contributors_Golden`'s existing golden needed no changes).

TDD: `TestCollectContributors_CollectsEveryDistinctPR` (two distinct PRs on non-adjacent commits)
and `TestCollectContributors_DedupsSamePRAcrossCommits` (same PR number on two commits → one entry)
in `contributors_internal_test.go`, both red against the unfixed `break`-after-first logic; the
three pre-existing `collectContributors` tests updated from `.PR`/`*PullRequest` to `.PRs`/slice
assertions (behavior unchanged, only the field shape). New golden
`testdata/release_notes_multi_pr_contributor.golden` + `TestRenderReleaseNotes_MultiPRContributor_-
Golden` locks in the end-to-end two-PR render: `* @alice made their first contribution in
[#7](…), [#9](…)`. Full `go test ./...` and `hk check` (golangci-lint, gofmt, typos) clean. No ADR
— additive field-shape change to an internal type, no behavior change for the existing single-PR
path, no config/schema surface touched.

---

### Phase 39 — Rename `--version`/`--build` override flags to disambiguate from root's `--version`

#### ✦ `[x]` T263: rename `release`/`changelog`'s `--version`/`--build` flags to `--set-version`/`--set-build-id`

`heraut release`/`heraut changelog` accept a local `--version <value>` (override the resolved
version) and `--build <id>` (append a build ID, requires `--version`). Both share a name with
root's own `--version`/`-v` (print the binary version) — no technical collision (cobra scopes
local flags to their own command), but confusing enough that the README carried a dedicated
"Gotcha" callout. Renamed to `--set-version` and `--set-build-id`, which read unambiguously as
"set an explicit value" and don't shadow the root flag's name at all. `--build` became
`--set-build-id` rather than staying as-is, for symmetry with `--set-version` even though it had
no collision of its own to fix.

TDD: updated the flag-name assertions in `internal/cmd/release_test.go`/`changelog_test.go`
first (`TestRelease_Structural`/`TestNewChangelogCmd`'s flag-lookup loops, plus every
`executeRoot(...)` call and error-string assertion) to red against the old flag names, then
renamed the `StringVar` registrations in `internal/cmd/release.go`/`changelog.go`. Every
user-facing error string mentioning the old flag names was updated in lockstep — duplicated
`--build requires --version` guards in both `internal/cmd/release.go` and `changelog.go`
(pre-existing duplication, left as-is — out of scope for a rename), `internal/app/resolver.go`
(three error strings + doc comments), `internal/versioning/tagfmt/tagfmt.go`'s
`{build}`-without-a-value error, and `internal/versioning/semver/resolver.go`'s manual-bump-mode
error — plus matching test assertions in `resolver_test.go` and `tagfmt_test.go`. Doc-comment-only
references (no behavior, not test-covered) updated across `versioning/static.go`,
`app/changelog_rotation.go`, `semver/rotation.go`, and their test files. No renaming of internal Go
identifiers (`versionOverride`, `buildID`, `ValidateBuildID`) — only the CLI-facing flag strings
and user-visible error text changed.

Docs updated: `docs/specs/02-configuration.md`, `03-commands.md`, `04-versioning.md`,
`06-dx-and-testing.md`, `docs/guides/mobile-ci-tagging.md`, `docs/heraut.sample.yml`,
`schema.json` (`bump.mode` description), `.goreleaser.yml` comment, and this repo's own
`CLAUDE.md`. The README's `--version`/`-v` "Gotcha" callout was deleted outright — the flags no
longer share a name, so there's nothing left to disambiguate. `CHANGELOG.md` (historical) and
`.claude/plans/*.md` (point-in-time design docs) were deliberately left untouched — they're
records of what was true when written, not living documentation. No ADR — this is a pre-v1.0 CLI
flag rename with no config-shape change. The `{build}` tag_format token itself was **not**
renamed to `{build-id}`/`{build_id}` — raised mid-task as a follow-on question, it's a much
larger, separately-scoped change (breaks every existing `.heraut.yml` with `{build}` in
`tag_format`, touches `tagfmt`'s token constant, `schema.json`, the sample config, native's
template rendering, and every fixture under `testdata/config/`) and was not pursued here.

### Phase 40 — Give `heraut init` its own `--overwrite` flag instead of overloading root's `--force`

#### ✦ `[x]` T264: split init's overwrite semantics off root's shared `--force`

Root's persistent `--force` flag already carries two related meanings (bypass promotion
guards E001/E002; downgrade required PR/MR enrichment to optional for the run). `heraut
init` piggybacked on the same flag for a third, unrelated meaning: "overwrite an existing
config file." Same flag, three meanings depending on which command reads it — the same
shape of confusion `--version` had before T263, except here it's one flag silently doing
double duty instead of two same-named flags. Gave `init` its own local `--overwrite` flag
(`internal/cmd/init.go`); root's `--force` keeps its original two (now genuinely related)
meanings, unchanged for `release`/`changelog`/`version next`/`version current`.

Also reworded root's `--force` `--help` description (`internal/cmd/root.go`), which leaned
on internal error codes (`bypass E001/E002 promotion errors`) a first-time reader has no
reason to know, to `override safety checks blocking tag promotion or missing PR/MR
metadata`. The full technical detail (E001/E002, ADR-0007 link, the enrichment-downgrade
behavior) stays in `docs/specs/03-commands.md`'s global-flags table, which is the right
altitude for it; only the live `--help` one-liner needed to drop the jargon.

TDD: renamed `TestInitCmd_DefaultsForceOverwrites` →
`TestInitCmd_DefaultsOverwriteFlagOverwrites` and `TestInitCmd_DefaultsWithExistingNoForceErrors`
→ `TestInitCmd_DefaultsWithExistingNoOverwriteErrors` in `internal/cmd/init_test.go` to red
against `--overwrite` first (unknown flag), then added the `overwrite` bool flag and
threaded it through `init.go` in place of the borrowed `force` read. Root's `--force`
remains technically inherited-but-unread on `init` (same as `--dry-run`/`--env`/`--offline`
already were) — not hidden from `init --help`, since cobra has no clean per-command
suppression of an inherited persistent flag and that's a pre-existing pattern, not new
scope for this task.

Docs updated: `docs/specs/03-commands.md` (global-flags table row, `heraut init`'s own flag
table + example). No config-schema or `.heraut.yml` surface touched — this is a CLI-flag-only
change.

### Phase 41 — `heraut version sprint bump` respects `--dry-run`

#### ✦ `[x]` T265: make `--dry-run` prevent the write in `version sprint bump`

The command increments `versioning.sprint` in `.heraut.yml` and writes it back
immediately; the spec explicitly called out that `--dry-run` "has no effect" for it. That
directly contradicted root's own `--dry-run` contract ("no file writes outside `/tmp`") —
this was the one command in the whole CLI where the global flag was silently a no-op on a
command that writes.

`internal/cmd/version_sprint.go`'s `RunE` now checks `--dry-run` before calling
`config.IncrementSprint` (which both computes and writes in one step): on dry-run it loads
the config directly (`config.Load`), computes `cfg.Versioning.Sprint + 1` itself, and prints
`[dry-run] would bump sprint N -> M in <path>` — matching the `[dry-run] would <action>`
phrasing already used throughout `internal/pipeline/{release,changelog}.go` — without ever
calling the writing function. No changes to `internal/config/sprint.go`: keeping the
peek-without-writing logic in the cmd layer avoided adding a second entry point or a
bool parameter to `IncrementSprint` for a single caller.

TDD: `TestVersionSprintBump_DryRun_DoesNotWrite` added to `internal/cmd/version_test.go`,
red against the unmodified command (asserted `[dry-run]` in output and an unchanged file;
got the real "sprint bumped to 6" success message and a mutated file instead).

Docs updated: `docs/specs/03-commands.md`'s `heraut version sprint bump` section, replacing
the "`--dry-run` has no effect" caveat with a description of the new behavior.

### Phase 42 — Scope `--dry-run`/`--env`/`--force`/`--offline` to the commands that use them, off root

#### ✦ `[x]` T266: move four persistent root flags to local flags on the commands that actually read them

T264 gave `init` its own `--overwrite` so it stopped borrowing root's `--force` for an
unrelated meaning, but left `--force` (unused, inert) still showing on `init --help` —
cobra persistent flags are the *same* `*pflag.Flag` object shared across every
subcommand's merged flag set (`AddFlagSet` copies the pointer, not the value), so hiding
it from one subcommand without hiding it everywhere isn't possible while it stays a root
persistent flag. Root persistent flags fundamentally can't be scoped per-subcommand at
all — every command inherits every one, whether it reads it or not.

Root now declares only `--config` (read by literally every command) and `--verbose`
(deferred — used by 9 of 13 commands, close enough to universal to leave alone for now,
revisit later if it becomes its own source of confusion). `--dry-run`, `--env`, `--force`,
and `--offline` became **local** flags, redeclared with identical name/type/default/help
text on exactly the commands that read them — duplicated declarations, deliberately, since
that's the only way cobra allows a flag to differ by command:

| Command | Local flags added |
|---|---|
| `release` | `--dry-run`, `--env`, `--force`, `--offline` |
| `changelog` | `--dry-run`, `--env`, `--force`, `--offline` |
| `check` (bare) | `--env`, `--offline` |
| `check runtime` | `--env` |
| `commit check` | `--env` |
| `commit tickets` | `--env` |
| `commit create` | `--dry-run` |
| `version next` | `--env`, `--force` |
| `version current` | `--env`, `--force` |
| `version sprint bump` | `--dry-run` (was already reading it via root's persistent flag since T265; now genuinely local) |

`check config`, `commit verify`, and `init` get none of the four — confirmed via
`--help` inspection (`go run ./cmd/heraut init --help` no longer lists `--force` at all,
since there's nothing left to inherit it from). This is a stronger fix than the
point-patch considered earlier (making `init` error on a stray `--force`): the ghost flag
is gone from `--help` entirely, not just handled better once typed.

Since every `RunE` already read these via `cmd.Flags().GetBool/GetString(...)` — which
resolves local and inherited flags identically — no `RunE` body needed to change at all;
this was purely additive `Flags()` registration calls plus removing the four lines from
`root.go`'s `PersistentFlags()`. The entire existing test suite passed unchanged after the
move, which is itself the regression proof: every command that legitimately used one of
these four flags kept behaving identically.

TDD (for the actual new behavior — commands newly *rejecting* these flags):
`TestInitCmd_DoesNotAcceptUnrelatedFlags`, `TestCommitVerify_DoesNotAcceptUnrelatedFlags`,
`TestCheckConfig_DoesNotAcceptUnrelatedFlags`, `TestVersionSprintBump_DoesNotAcceptUnrelatedFlags`
— one representative command per matrix shape (fully-stripped x3, plus sprint bump which
gains `--dry-run` while losing the other three) rather than exhaustive per-command
coverage, since the mechanism being proven (an unregistered flag now hard-errors with
"unknown flag") is identical each time. `TestNewRootCmd` (`root_test.go`) updated to assert
the *absence* of the four flags from `root.PersistentFlags()`, inverting its previous
presence assertion.

Docs: `docs/specs/03-commands.md`'s global-flags table now lists only `--config`,
`--verbose`, `--version`/`-v`, `--help`/`-h`; every per-command section that gained a
flag it didn't document before (`--offline` on `release`/`changelog`/bare `check`; `--env`
on `check runtime`/`commit check`/`commit tickets`) got a row added to its flag table.

Phases 23, 24, 25, 27, 29, and 43 are heavy, multi-phase epics whose task breakdown and live
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

### Phase 43 — Release lifecycle hooks

Six hook points (`post_bump`, `pre_changelog`, `pre_tag`, `post_tag`, `pre_release`,
`post_release`) let a `.heraut.yml` run arbitrary shell commands at points in the release
lifecycle — a version bump in another file, a build/test gate before tagging, `npm publish`
after the tag, a Slack notification after publish. Go `text/template` command strings, executed
via `sh -c` through the existing GPG-pinentry interactive runner mode (T260) so output streams
live. `pre_release`/`post_release` are isolated per publish target — a failing hook skips only
that platform, not the whole release. New ADR-0053. Seven tasks (config schema → execution helper
→ pipeline wiring → per-platform isolation → dry-run → integration test → docs), so the task
breakdown and live `[ ] / [x]` status live in a dedicated roadmap:

→ **[Release Hooks Roadmap](release-hooks-roadmap.md)** — T267+

Design: [`docs/superpowers/specs/2026-09-11-release-hooks-design.md`](../superpowers/specs/2026-09-11-release-hooks-design.md).

---

### Phase 44 — Windows hook execution

Phase 43's `hooks:` feature runs every command via `sh -c` — POSIX-only, a documented gap
in ADR-0053 since heraut ships Windows binaries (ADR-0013). This phase closes that gap by
selecting a Windows shell per `runtime.GOOS`, without adding a second per-OS command syntax
to the config schema: hook command strings stay a single, OS-specific shell string the user
owns, the same portability contract a CI YAML `run:` step already has. Small (three tasks),
so it stays inline here rather than in a dedicated roadmap file.

#### ✦ `[x]` T274: ADR-0054 — Windows hook shell + portability stance

[ADR-0054](../adr/0054-windows-hook-execution.md): Windows invokes `cmd /D /C "<rendered
command>"` — not PowerShell. The deciding factor was exit-code propagation, not
familiarity or feature richness: `cmd /C` re-exits with the invoked command's own exit
code, matching `sh -c`'s behavior exactly, while `powershell -Command` does not (`
$LASTEXITCODE` gets set, but `powershell.exe` itself still exits 0 unless the script
explicitly ends with `exit $LASTEXITCODE`) — a well-documented footgun that would have
silently broken hook failure detection, the one behavior ADR-0053 can't compromise on.
`/D` disables `cmd`'s `AutoRun` registry hook, `cmd.exe`'s analogue of `-NoProfile`/`sh
-c` never sourcing an rc file.

Confirmed as explicit non-goals: no per-OS `hooks:` config keys (hook commands stay one
OS-specific shell string, same portability contract as a CI YAML `run:` step — a user who
wants POSIX syntax on Windows can already get it by writing `bash -c "..."` as their own
hook command, no heraut-side detection needed), and no Windows CI runner in this phase
(`ci.yml`/`release.yml` remain `ubuntu-latest`-only — the Windows branch ships covered by
mocked `MockRunner` unit tests in T275, not real `cmd.exe` execution; a Windows CI leg is
a separate, later decision given its ongoing cost).

#### ✦ `[x]` T275: `internal/pipeline`: OS-parameterized shell selection

Added `hookShellInvocation(goos, cmd string) (name string, args []string)` in
`internal/pipeline/hooks.go` — a pure function implementing ADR-0054's mapping (`sh -c` on
everything but Windows, `cmd /D /C` on Windows). `runHook` now calls it with `runtime.GOOS`
instead of hardcoding `"sh", "-c"`.

New table-driven `TestHookShellInvocation` in `hooks_test.go` covers all three GOOS values
(`linux`, `darwin`, `windows`) directly against the pure function, so the Windows branch is
asserted deterministically without depending on the host OS running `go test` — same
approach as CalVer's injectable `now func()`. Written first as a failing test (`undefined:
hookShellInvocation`) before the implementation, per TDD.

The four other test files named in this task's original scope
(`release_hooks_test.go`, `changelog_hooks_test.go`, `dryrun_hooks_test.go`,
`cmd/changelog_hooks_realrepo_test.go`) needed **no changes** — they already assert
`runHook`/`runHookPoint`'s behavior on the host OS (always POSIX in this repo's CI), and
`hookShellInvocation`'s POSIX branch is byte-identical to the old hardcoded call, so they
kept passing unchanged. Full suite (`go test ./...`) and `hk check` (`golangci_lint`,
`typos`, `go_fmt`) green.

#### ✦ `[x]` T276: Docs — Spec 02 § hooks Execution + ADR-0053 cross-reference

[Spec 02 § `hooks` → Execution](../specs/02-configuration.md#execution) now describes the
per-OS shell selection (`sh -c` / `cmd /D /C`) instead of "POSIX shells only," and states
the portability contract explicitly: a hook command string is one OS-specific shell
string, never dual-authored, same as a CI YAML `run:` step. ADR-0053's "POSIX-only in v1"
Consequences bullet gained a "Resolved by ADR-0054" note (kept, not deleted, as a record
of the v1 scoping decision — mirrors how ADR-0034/ADR-0037 handle a partial supersession).
`docs/adr/README.md` gained a row for ADR-0054 and an updated status annotation on
ADR-0053. `docs/guides/release-pipeline-and-hooks.md`'s one `sh -c`-specific mention
(in the `--dry-run` callout) was reworded to stay accurate on both OSes. `CLAUDE.md`'s ADR
count bumped 53 → 54 in both places it's stated. This closes Phase 44 — Windows hook
execution: all of T274–T276 are done.

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
