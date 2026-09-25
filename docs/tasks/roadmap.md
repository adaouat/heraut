# Héraut Build Roadmap

> Status: Active  
> Repo: `github.com/adaouat/heraut`

This roadmap is the executable plan for bringing Héraut to v1.0 with the feature set
described in `docs/specs/`. Each task carries an inline `[ ] / [x]` checkbox — read the
headings for what to do next, read the surrounding prose for *why* and *how*.

The behavioural authority is `docs/specs/` (six numbered specs); the architectural
authority is `docs/adr/` (63 ADRs). Where this roadmap mentions "behaviour", the specs
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

The `docs/specs/` (six numbered specs) and the 63 ADRs in `docs/adr/` are authoritative.

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
| 45 | `{{ .Env }}` hook template variable | Done |
| 46 | Configurable commit-message rules (`commits.rules`) | Done |
| 47 | Per-token footer/trailer rendering customization (`rendering.trailers`) | Done |
| 48 | Built-in default `Co-Authored-By` trailer rendering | Done |
| 49 | Namespaced template blocks (`release.*` / `commit.*`) | Done |
| 50 | Move `rendering.trailers` to `rendering.templates.commit.trailers` | Done |
| 51 | Hook-declared file staging | Done — see `hook-file-staging-roadmap.md` |
| 52 | Selective hook skipping (`--skip-hook`, `HERAUT_SKIP_HOOKS`) | Done |
| 53 | Stay at v0 — hold major bumps at v0 (`stay_at_v0`, `--allow-major`) | Done |
| 54 | Phase 53 follow-ups — hygiene, test breadth, docs polish, `version next` in manual mode | Done |
| 55 | Phase 54 follow-ups — promote.go error handling, cmd naming, message polish | Done |
| 56 | Sign the raw binaries with a packslip manifest | Done — not yet exercised by a real release run, see T319/T320 |
| 57 | SBOM generation; shell completions investigated | Done — completions not shipped (ADR-0013 + notarization gap), see T321/T322 |

### Open items

The only unchecked item outside Phase 53 (T304 is a deliberately unscheduled future task) is the
last sub-checkbox of Phase 10's closing checkpoint, `v1.0.0 cut …`:

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

### Phase 45 — `{{ .Env }}` hook template variable

`hooks:` stayed a flat, top-level block after Phase 43 — per-env overrides were an
explicit non-goal of that pass. This phase closes the gap the same way Phase 44 closed
the Windows one: not with a new config axis, but with a new template variable a single
hook command can branch on. Small (three tasks), so it stays inline here rather than in a
dedicated roadmap file.

#### ✦ `[x]` T277: ADR-0055 — `{{ .Env }}` hook template variable, no per-env config key

[ADR-0055](../adr/0055-env-hook-template-variable.md): add `{{ .Env }}` as a new hook
template variable rather than an `environments.<env>.hooks:` config key. Mirrors
ADR-0054's own framing for the Windows-shell question: `{{ if eq .Platform ... }}` had
already proven the templating engine can express per-axis branching inside one command
string without a second config axis, so `{{ .Env }}` extends that same proof to the
per-environment case. Unlike `Platform` (scoped to `pre_release`/`post_release`, since a
publish target only exists at those two points), `Env` is set at all six hook points —
which environment is active is meaningful for the whole run, not just at publish time.
Confirmed as an explicit non-goal for this phase: `environments.<env>.hooks:` overriding
or merging with root `hooks:` — rejected for now given the unresolved replace-vs-merge
semantics question and no evidence yet that single-string branching is insufficient;
revisit only if that evidence shows up.

#### ✦ `[x]` T278: `internal/pipeline` + `internal/app`: thread `--env` into hook template context

Added `Env string` to `hookVars` (`internal/pipeline/hooks.go`), `pipeline.Config`
(`internal/pipeline/config.go`), and `pipeline.ChangelogConfig`
(`internal/pipeline/changelog.go`). Both pipelines' `hookVars(result)` methods now set
`Env: p.cfg.Env`. `internal/app/pipeline.go`'s `buildReleasePipelineConfig` and
`buildChangelogPipelineConfig` set `pCfg.Env`/`cCfg.Env` from the `env`/`opts.Env`
parameters already in scope there — no new plumbing needed above that layer, since
`--env` already reached both builders for the existing per-env changelog/release-notes
merge logic.

Written test-first (TDD): `TestRenderHookCmd_SubstitutesVars` in `hooks_test.go` extended
to assert `{{ .Env }}` renders alongside the existing vars; new
`TestRun_PostBumpHook_SubstitutesEnv` / `TestChangelogRun_PostBumpHook_SubstitutesEnv`
prove `cfg.Env` reaches a real hook command through each pipeline's `Run()`; new
`TestBuildReleasePipelineConfig_PropagatesEnv` /
`TestBuildChangelogPipelineConfig_PropagatesEnv` /
`TestBuildReleasePipelineConfig_EmptyEnvIsFlatDefault` in `internal/app/hooks_internal_test.go`
prove the app-layer wiring, including the empty-string default when no `--env` is passed.
All five were confirmed failing (`unknown field Env` / `undefined: pCfg.Env`) before the
struct fields and wiring were added. Full suite (`go test ./...`) and build green; no
schema, sample-config, or validator changes needed since this is a template-variable
addition, not a config-schema change (ADR-0055).

#### ✦ `[x]` T279: Docs — Spec 02 § hooks Template variables table + ADR-0055 cross-reference

[Spec 02 § `hooks` → Template variables](../specs/02-configuration.md#template-variables)
gained a `{{ .Env }}` row ("All six points") and the branching-example prose now mentions
`{{ if eq .Env "prod" }}...{{ end }}` alongside the existing `{{ if eq .Platform ... }}`
example, both citing ADR-0053/ADR-0055. `docs/adr/README.md` gained a row for ADR-0055.
`CLAUDE.md`'s ADR count bumped 54 → 55 in both places it's stated. This closes Phase 45 —
`{{ .Env }}` hook template variable: all of T277–T279 are done.

---

### Phase 46 — Configurable commit-message rules (`commits.rules`)

`heraut commit verify`/`check` enforce an allow-list of types (`commits.types`) and,
optionally, scopes (`commits.scopes` + `scopes_restricted`), but have no way to express
project-specific constraints like "no WIP markers" or "commits must reference a ticket."
This phase adds `commits.rules`, a generic list of deny/require/require-ticket matchers,
optionally scoped by type/scope and targetable at the message header, body, or footer.

#### ✦ `[x]` T280: ADR-0056 — `commits.rules`: generic commit-message pattern rules

[ADR-0056](../adr/0056-configurable-commit-message-rules.md): add `commits.rules`, a list
of `CommitRule` objects (`name`, one of `deny`/`require`/`require_ticket`, optional
`target` and `types`/`scopes` scoping, `message`), evaluated by `app.VerifyCommit`
alongside the existing type/scope checks. Mirrors the "matcher → outcome" shape
`versioning.bump[]`'s `BumpRule` already established. `require_ticket` reuses
`commits.tickets` as the source of truth (any configured pattern matches) instead of
asking rule authors to duplicate a ticket regex in `require`; `target` maps onto the
header/body/footer split `conventionalcommit.Parse` already produces, so a ticket
requirement can be scoped to a footer trailer (commitlint-style `Refs: JIRA-123`) rather
than anywhere in free text. Confirmed as explicit non-goals: surfacing rule violations
live in the `heraut commit create` wizard (a `verify`/`check`-time gate is a different
concern), and any change to how `commits.tickets` itself is declared.

#### ✦ `[x]` T281: `internal/config` + `internal/app`: implement `commits.rules`

Implemented in two TDD increments. **Config layer**: `CommitRule`
(`internal/config/commits.go`) and `Commits.Rules []CommitRule`, plus
`validateCommitRules` (`internal/config/validator.go`) — exactly-one-of `deny`/`require`/
`require_ticket` enforcement, regex compilation for `deny`/`require`, `require_ticket:
true` requiring non-empty `commits.tickets`, `message` required for `deny`/`require`,
`target` restricted to `header`/`body`/`footer`/`message`, and duplicate-name detection
(mirroring the existing `commits.types`/`commits.scopes` checks). **App layer**:
`app.VerifyCommit` (`internal/app/commit.go`) gained `verifyRules` / `ruleAppliesTo` /
`evaluateRule` / `ruleTarget` — every matching rule (types/scopes-filtered) is evaluated
and all violations are collected into one error rather than returning on the first;
`ruleTarget` extracts header (raw first line) / body / footer (each
`conventionalcommit.Footer` rendered as `Token: Value`, joined) / message text.

**Deviation from ADR-0056's "compile once, not per commit" note**: implemented as a fresh
`regexp.Compile` per `VerifyCommit` call instead of a precompiled cache. At the rule counts
and commit-range sizes this targets, compile cost is negligible (low-microsecond compiles
× realistic rule/commit counts, still well under a second) — a cache would need mutable
derived state on the shared `*config.Config` pointer, or a new parameter threaded through
every `internal/cmd/commit.go` call site, either of which is real complexity for a win that
doesn't show up at this scale. Deferred unless profiling on a large rev-range shows
otherwise.

TDD: `internal/config/validator_test.go` gained a `commits.rules` section (valid-configs
table, exactly-one-of table, name/duplicate-name, invalid-regex table, message-required
table, `require_ticket`-needs-tickets, invalid target) — confirmed failing (`field rules
not found in type config.Commits`) before the schema change.
`internal/app/commit_test.go` gained 13 `TestVerifyCommit_Rules_*` tests (deny, require,
`require_ticket` with configured and default messages, header/body/footer targeting —
including the footer-scoped ticket convention, type/scope scoping, multi-rule
aggregation, no-rules-configured regression) — confirmed failing before `verifyRules` was
wired into `VerifyCommit`. Full suite (`go test ./...`), build, and
`hk check -S golangci_lint` green. T282 (spec/schema/sample docs) is not started.

#### ✦ `[x]` T282: Docs — Spec 02 § `commits.rules` + schema.json + sample config + ADR-0056 cross-reference

`docs/specs/02-configuration.md` gained a `### \`commits.rules\`` section (alongside the
existing `commits.types`/`commits.scopes`/`commits.tickets` sections) with a field table
and a `require_ticket`/`target: footer` example, citing ADR-0056; the `commits:` overview
YAML block at the top of the page also gained a one-line `rules:` example for
discoverability. `docs/specs/03-commands.md`'s `heraut commit verify` section (not in the
original task scope, but directly describes the behavior T281 just changed) now mentions
that verify evaluates `commits.rules` and aggregates violations — left undocumented, the
spec would have been silently inaccurate about what the command actually validates.

`schema.json` gained the `CommitRule` definition and `Commits.rules` array property,
matching the existing `Exclude`/`BumpRule` precedent of documenting cross-field
constraints ("exactly one of deny/require/require_ticket") in prose rather than
JSON-Schema `oneOf`, since heraut's own semantic validator is the enforcement layer.
`docs/heraut.sample.yml` gained a commented `rules:` example under the existing
(fully-commented) `commits:` block. `testdata/config/valid/commit-rules.yml` is a new
schema fixture (mirroring `tickets.yml`'s pattern) covering `deny`, `require_ticket` +
`target: footer` + `types` scoping — `TestSchema_ValidFixtures` picks it up automatically
via its glob. `docs/adr/README.md` gained a row for ADR-0056. `CLAUDE.md`'s ADR count
bumped 55 → 56 in both places it's stated.

Full suite (`go test ./...`), build, and `hk check` (yamlfmt, typos — no Go files changed
in this docs-only slice) all green. This closes Phase 46 — `commits.rules`: all of
T280–T282 are done.

This file's own intro (lines 11 and 39) also said "51 ADRs" — stale since well before
this task (`CLAUDE.md` was kept in sync at each ADR-adding task; this intro prose was
not). Noticed during T282 but out of its original scope, so flagged to the user rather
than silently fixed; corrected here on request.

---

### Phase 47 — Per-token footer/trailer rendering customization (`rendering.trailers`)

Commit-message footer trailers (`Co-authored-by:`, `Refs:`, `Signed-off-by:`, …) already parse
generically into `{Token, Value}` pairs and reach `release_notes.tmpl` via `tplCommit.Footers`,
but every trailer renders identically (`Token: Value`) with no way to relabel, reformat, or
suppress one by token, and the changelog's `commit` block doesn't render footers at all. This
phase adds `rendering.trailers`, a list of per-token `FooterRule` matchers (case-insensitive
exact match on token; a Go-template `renderer` snippet or `hide`), applied wherever a template
renders `.Footers`.

#### ✦ `[x]` T283: ADR-0057 — `rendering.trailers`: per-token footer rendering customization

[ADR-0057](../adr/0057-rendering-trailers.md): add `rendering.trailers`, a list of `FooterRule`
objects (`token`, exactly one of `renderer`/`hide`), resolved once per commit in
`buildCommit` and exposed as a new `tplFooter.Line` field so every consumer of `.Footers`
(today: `release_notes.tmpl`; the changelog `commit` block if a project opts in via the
existing `rendering.templates.commit` override) gets customized rendering for free. Matching is
case-insensitive exact-match on token, not regex — footer tokens are a small, tool-emitted,
fixed-spelling vocabulary. Merge semantics mirror `rendering.templates` (deep-merge by key,
global → driver → env), not `commits.types`/`commits.scopes` (no built-in trailer rules to
merge over — the implicit default is the existing `Token: Value` fallback).

Named `trailers`, deliberately not `footers`: `footer` (singular) is already the document-level
credit-line block key (ADR-0049) and `tplCommit.Footers` is already the model field name, so
`rendering.footers` risked the exact same-word-different-meaning ambiguity ADR-0048 resolved for
`header`/`release_header`. Confirmed as an explicit non-goal: a broader restructuring of
`rendering.templates` from its current flat `map[string]string` into a nested shape (e.g.
`release.commit.{title,tickets,body,footers}`) was raised in the same discussion and
deliberately kept out of this ADR — it renames every existing block key (reopening ADR-0048's
`header` disambiguation, moving `footer`/`release_footer` again after ADR-0049 renamed them
three weeks prior) and makes `body` newly overridable when it isn't today. That idea is not yet
scoped or tracked as a task; picking it up later is a separate ADR + roadmap phase.

#### ✦ `[x]` T284: `internal/config` + `internal/generators/native`: implement `rendering.trailers`

Implemented in three TDD increments (config → app → native), matching T281's precedent.
**Config layer**: `FooterRule` (`internal/config/commits.go`) and `Rendering.Trailers
[]FooterRule`, plus `config.MergeFooterRules` — an exported override-wins-by-token merge
(case-insensitive), reused both by `mergeRendering` (driver+env, `internal/config/merge.go`)
and by the app layer (driver+global). `validateTrailers`
(`internal/config/validator.go`) mirrors `validateCommitRules`'s exactly-one-of pattern:
token required, exactly one of `renderer`/`hide`, `renderer` parsed via the existing
`parseTemplateSnippet` stub-func-map path, duplicate token (case-insensitive) rejected.
**App layer**: `ContentDriver.EffectiveTrailerRules map[string]FooterRule`
(`internal/config/config.go`) and `effectiveTrailers` (`internal/app/pipeline.go`), wired
into `withEnvDerivations` alongside the existing `effectiveTemplates`/`effectiveExcludes` —
same global→driver→env deep-merge-by-key shape as `EffectiveTemplates`, flattened to a
lowercased-token-keyed map for O(1) lookup. **Native layer**: `tplFooter` gained `Line`
(`internal/generators/native/templatemodel.go`); a new `resolveFooterLine(token, value,
rules)` resolves each footer once in `buildCommit` — matching `Hide` drops the footer from
`tplCommit.Footers` entirely, a matching `Renderer` executes as a `text/template` snippet
against `{Token, Value}`, no match keeps the built-in `"Token: Value"` format.
`release_notes.tmpl`'s footer loop now prints `.Line` instead of composing `Token: Value`
itself — the actual wiring point; a red end-to-end test
(`TestRenderReleaseNotes_TrailerRuleCustomizesFooterLine`) confirmed the feature was
inert without this template change even though the model-building tests already passed,
so it was added deliberately before the template edit, not skipped.

**Deviation from the ADR's implicit assumption that config validation catches renderer
errors:** it doesn't, fully. Go's `text/template` doesn't type-check field references
against a concrete data type at parse time, only at `Execute` — so a snippet like `{{
.Toke }}` (typo) passes `parseTemplateSnippet` at config-validate time and only fails at
render time. `buildCommit`/`buildRelease` therefore both gained an `error` return to
propagate that failure with the offending token named, threaded through
`renderChangelogSection`/`renderReleaseNotes` — the same error contract ADR-0037 already
established for block snippets, just newly proven to apply here too.

**Deviation from the ADR's silence on caching:** `resolveFooterLine` parses its template
string fresh per footer occurrence rather than pre-parsing once per configured rule.
Mirrors T281's "negligible cost at realistic scale, deferred unless profiling shows
otherwise" reasoning for regex compilation.

**Unchanged, as designed:** the changelog's `commit` block still does not loop over
`.Footers` by default — `rendering.trailers` governs formatting, not visibility, exactly
per ADR-0057. Opting the changelog in still requires overriding `commit` via the existing
`rendering.templates` lever.

TDD: `internal/config/commits_test.go` (`MergeFooterRules`), `internal/config/merge_test.go`
(driver+env trailer merge subtest), `internal/config/validator_test.go` (6 new trailer
validation tests), `internal/app/templates_internal_test.go`
(`TestWithEnvDerivations_MergesTrailers`), `internal/generators/native/
templatemodel_internal_test.go` (`TestResolveFooterLine` table, `TestBuildCommit_
ResolvesFooterLines`, `TestBuildCommit_DefaultFooterLineUnchangedWhenNoRulesConfigured`),
`internal/generators/native/render_internal_test.go`
(`TestRenderReleaseNotes_TrailerRuleCustomizesFooterLine`) — each confirmed failing for
the expected reason (missing symbol, or assertion against unchanged output) before its
implementation landed. All existing golden-file tests pass unchanged, confirming
byte-identical default output as ADR-0057 requires. Full suite (`go test ./...`), build,
`go vet`, and `hk check` (`go_fmt`, `golangci_lint`, `typos`) all green. schema.json,
`docs/heraut.sample.yml`, and the spec are deliberately deferred to T285, matching how
T281/T282 split this same config+docs work for `commits.rules`.

#### ✦ `[x]` T285: Docs — `rendering.trailers` spec, schema.json, sample config, guide

**Deviation from this task's own title:** the config-field reference landed in
[`docs/specs/02-configuration.md`](../specs/02-configuration.md) as a new `### rendering.trailers
(ADR-0057)` section, not Spec 05 — checking the actual precedent showed `rendering.excludes`/
`rendering.templates` already live in Spec 02 (field-level config reference), while Spec 05 only
carries the native generator's *behavioral* contract (ADR-0037's block set, data model). The
task title was written before that check; corrected here rather than silently filed under the
wrong spec. Spec 02 also gained a one-line `trailers:` example in both the page's top-level
`rendering:` overview block and the `## rendering` section's own intro example, matching how
T282 added a `rules:` one-liner to `commits:`'s overview for discoverability.

Spec 05 still needed a fix of its own, found while cross-checking: the `Commit` data-model
prose listed `.Footers` but never expanded its per-entry shape (unlike `.Tickets`, which does)
— now documents `.Token` `.Value` `.Line`, with `.Line` explained as the trailer's
`rendering.trailers`-resolved display line and a pointer back to Spec 02.

`schema.json` gained `FooterRule` (required `token`; `renderer`/`hide` documented as
exactly-one-of in prose, matching the `Exclude`/`CommitRule`/`BumpRule` precedent of not
encoding cross-field constraints as JSON-Schema `oneOf`) and `Rendering.trailers`, inserted
next to `Exclude` in the definitions list (both are `Rendering`'s array-item sub-types).
`docs/heraut.sample.yml` gained a commented `trailers:` example under the existing
(fully-commented) `rendering:` block. `testdata/config/valid/rendering-trailers.yml` is a new
schema fixture (mirroring `rendering-templates.yml`'s pattern, covering one `renderer` entry and
one `hide` entry) — `TestSchema_ValidFixtures` picks it up automatically via its glob.

**Also updated, found out of the original task's scope but directly affected (same reasoning
T282 applied to `03-commands.md`):**
[`docs/guides/template-customization.md`](../guides/template-customization.md) is a
dedicated worked-example guide for exactly this customization surface — leaving it silent
about `rendering.trailers` would mislead a reader into thinking block overrides are the only
way to customize footer display. It gained a new `## Customizing footer trailers
(rendering.trailers, ADR-0057)` section (config shape, field table, the
formatting-not-visibility distinction, precedence note), a pointer from the "Two ways to
customize" intro, a `.Line` mention in the `Footer` data-contract entry, and a Gotchas bullet.

Full suite (`go test ./...`), build, `go vet`, and `hk check` (`yamlfmt`, `typos` — no Go files
changed in this docs-only slice) all green, including `TestSchema_ValidFixtures` and
`TestShippedExamples_LoadAndValidate` (confirms `docs/heraut.sample.yml` and README's fenced
config blocks still parse/validate after the edits). This closes Phase 47 —
`rendering.trailers`: all of T283–T285 are done.

---

### Phase 48 — Built-in default `Co-Authored-By` trailer rendering

Phase 47 shipped `rendering.trailers` with a deliberate non-goal: no built-in trailer-rule
set, since every token's implicit default is the raw `Token: Value` line. The first real use
of the feature (configuring heraut's own `.config/heraut.yml` to de-emphasize its own
`Co-Authored-By:` trailers) showed that call was too conservative for this one token — see
[ADR-0058](../adr/0058-default-coauthored-by-trailer.md). This phase bakes a
`Co-Authored-By` → `_Co-Authored-By: {{ .Value }}_` default into the native generator
itself, following the same built-in-defaults-merged-under-user-config shape as
`commits.types`/`commits.scopes`.

#### ✦ `[x]` T286: ADR-0058 — built-in default `Co-Authored-By` trailer rendering

[ADR-0058](../adr/0058-default-coauthored-by-trailer.md): add `config.DefaultTrailers()`,
merged as the base layer under global `rendering.trailers` in `effectiveTrailers`
(`internal/app/pipeline.go`), so `Co-Authored-By` renders as `_Co-Authored-By: {{ .Value }}_`
by default — overridable/hideable via the existing `rendering.trailers` config, no new
config surface. Partially supersedes ADR-0057's "no built-in trailer-rule set" merge
semantics and "byte-identical default output" consequence, for this one token only. Added
inline supersede notes to ADR-0057 itself (three spots: the merge-semantics paragraph, the
"unmatched token" sentence, and the Consequences bullet) rather than changing its Status,
since only this one narrow point is superseded — mirrors the precedent set by
ADR-0048's note inside ADR-0037.

**Trigger for this ADR:** the initial ask was to add this exact rendering
(`_Co-Authored-By: {{ .Value }}_`) directly to heraut's own `.config/heraut.yml` via the
`rendering.trailers` config ADR-0057 already shipped — a config-only change requiring no ADR.
That was implemented, then explicitly reverted on follow-up: the actual intent was a
heraut-native default so every project benefits, not a per-project config entry, which
directly contradicts ADR-0057's "no built-in trailer-rule set" decision and needed its own
ADR rather than a silent reversal.

#### ✦ `[x]` T287: `internal/config` + `internal/app`: implement the built-in default

`config.DefaultTrailers()` (`internal/config/commits.go`), merged via the existing
`config.MergeFooterRules` in `effectiveTrailers` (`internal/app/pipeline.go`) as
`MergeFooterRules(DefaultTrailers(), global)` before the existing per-driver merge. No
changes needed in `internal/generators/native` (it already consumes whatever map
`EffectiveTrailerRules` resolves to, agnostic of where entries came from) or in
`internal/config/validator.go` (validation only walks user-authored config; the built-in
list is a trusted Go literal, not YAML).

**Deviation found during implementation:** `withEnvDerivations`'s early-return fast path
(`internal/app/pipeline.go`) returned the original `*ContentDriver` pointer unchanged when
every derived field — including trailers — was empty. Since `effectiveTrailers` can no
longer ever return an empty map (the built-in default always contributes at least one
entry), that fast path's guard condition became permanently unreachable dead code. Removed
the guard entirely rather than leave it in place: the function now always clones and always
sets `EffectiveTrailerRules` unconditionally (previously guarded by `len(trailers) > 0`),
with `hasCommits`/`hasRendering` also removed as they existed solely to feed that now-gone
condition. Purely a dead-code cleanup forced by this change, not a scope expansion — the
function's externally observable behavior for every other field is unchanged.

TDD: `internal/config/commits_test.go`
(`TestDefaultTrailers_CoAuthoredByRendersItalicCreditLine`),
`internal/app/templates_internal_test.go`
(`TestWithEnvDerivations_AppliesBuiltInCoAuthoredByDefault`,
`TestWithEnvDerivations_UserRuleOverridesBuiltInCoAuthoredByDefault`,
`TestWithEnvDerivations_UserRuleHidesBuiltInCoAuthoredByDefault`) — the first of each pair
confirmed failing (undefined symbol / empty renderer) before its implementation landed; the
override/hide tests exercise the pre-existing `MergeFooterRules` override-wins-by-token path
against the new default rather than new logic, so they weren't expected to start red, and
didn't. Full suite (`go test ./...`), `go build`, `go vet`, and `hk check` (`go_fmt`,
`golangci_lint`, `yamlfmt`, `typos`) all green — including every existing
`rendering.trailers` test from T284, unaffected because none of them route through
`effectiveTrailers` (they call `MergeFooterRules`/`buildCommit`/`resolveFooterLine`
directly with explicit rule maps).

#### ✦ `[x]` T288: Docs — ADR-0057 supersede notes, spec, sample config, guide

`docs/adr/README.md`'s index row for 0057 gains a supersede annotation and a new 0058 row.
`docs/specs/02-configuration.md`'s `rendering.trailers` section gains the ADR-0058
cross-reference in its heading and a note on the `Co-Authored-By` exception to "unmatched
token keeps the built-in format." `docs/heraut.sample.yml`'s existing commented
`Co-authored-by` example gains a note that it now only demonstrates *overriding* the
built-in, not introducing new behavior. `docs/guides/template-customization.md` gains the
same exception note in its `rendering.trailers` section plus a new Gotchas bullet, matching
how T285 extended the same three doc surfaces for the original feature.

heraut's own `.config/heraut.yml` needed no `rendering.trailers` entry as a result — the
built-in covers it directly, confirmed via `heraut check config` (passes unchanged, config
untouched from its pre-session state).

Full suite, build, vet, and `hk check` (all four linters: `go_fmt`, `golangci_lint`,
`yamlfmt`, `typos`) green after the doc edits. This closes Phase 48 —
`Co-Authored-By` default trailer rendering: all of T286–T288 are done.

---

### Phase 49 — Namespaced template blocks (`release.*` / `commit.*`)

A proposal to add a fires/renders reference table for the native template blocks surfaced that
`rendering.templates`' flat 13-key namespace (`release_header`, `group`, `commit`, `ticket`,
`contributor`, `contributors`, `stats`, `release_footer`, plus the document-level
`title`/`subtitle`/`footer` and the `changelog`/`release_notes` roots) only inconsistently hints
at cadence via a `release_`/no-prefix convention. See
[ADR-0059](../adr/0059-namespaced-template-blocks.md): regroup the release- and commit-cadence
blocks under nested `release:`/`commit:` YAML objects (`release.section` was `release_header`,
`release.group` was `group`, `release.contributors` was `contributors`, `release.stats` was
`stats`, `release.footer` was `release_footer`, `commit.message` was `commit`, `commit.ticket`
was `ticket`, `commit.contributor` was `contributor`); the document-level trio and the two
document-root overrides stay flat. Breaking rename, no alias — pre-v1.0, same precedent as
ADR-0048/ADR-0049.

#### ✦ `[x]` T289: ADR-0059 — namespace `rendering.templates` into `release.*`/`commit.*`

Wrote [ADR-0059](../adr/0059-namespaced-template-blocks.md) after reviewing a proposed
fires/renders reference table for spec 05's template-blocks section: the table made the
existing flat-namespace inconsistency (only `release_header`/`release_footer` carry a prefix;
`group`/`commit`/`ticket`/`contributor`/`contributors`/`stats` don't, despite the same
release/commit cadence) visible enough to fix at the config-surface level rather than just in
prose. Decided nested YAML objects (`release:`/`commit:`) over a flat dotted-string key
(`"release.section": "..."`), since the latter keeps the existing flat `map[string]string`
shape but doesn't let `schema.json` document the two sub-namespaces independently or read as a
group in `.heraut.yml` — the actual goal. `release_header`→`release.section` (not
`release.header`) to match ADR-0038's own "section" vocabulary for the anchored unit;
`commit`→`commit.message` (not bare `commit`) to avoid reading as the `Commit` data type once
nested. No code changed in this task — `docs/specs/05-generators-and-platforms.md`,
`schema.json`, `docs/heraut.sample.yml`, and `docs/guides/template-customization.md` still
describe the pre-ADR-0059 flat block set until T290 implements the rename and T291 updates
them, per the project's two-step flow (one roadmap task per session).

#### ✦ `[x]` T290: `internal/config` + `internal/generators/native`: implement nested `release`/`commit` template blocks

**Deviation from ADR-0059's anticipated approach.** The ADR's Consequences section expected new
nested Go structs (`Release{Section, Group, ...}`, `Commit{Message, Ticket, Contributor}`) plus a
one-level-deeper field-by-field deep-merge. Turned out unnecessary: `Rendering.Templates` was
already a plain `map[string]string`, and the existing deep-merge (`mergeRendering`,
`effectiveTemplates`) only ever copies map entries key-by-key, blind to what the keys mean. So
instead of restructuring the config type, `rendering.templates`' value type became a new named
type `config.TemplateOverrides` (`internal/config/templates.go`, still `map[string]string`
underneath) with a custom `UnmarshalYAML` that flattens `release: {section: ..., ...}` /
`commit: {message: ..., ...}` into dotted keys (`release.section`, `commit.message`, ...) at
parse time — any other top-level key decodes as a plain snippet string, unchanged. Every
downstream layer (`mergeRendering`'s deep-merge, `effectiveTemplates`, `buildTemplateSet`) keeps
working against a flat map exactly as before, completely unaware the YAML surface is nested — zero
changes needed in `merge.go` or `internal/app/pipeline.go`. A key named `release`/`commit` whose
value isn't itself a mapping falls through to a plain flat key instead of being special-cased,
letting the existing `validateTemplateSnippets`/`validTemplateBlocks` "unknown template block"
mechanism reject it — no separate error path needed for that case either.

Renamed the `{{ define }}` blocks in `internal/generators/native/blocks.tmpl` and the
`{{ template "..." }}` call sites in `changelog.tmpl`/`release_notes.tmpl` to match
(`release_header`→`release.section`, `group`→`release.group`, `contributors`→
`release.contributors`, `stats`→`release.stats`, `release_footer`→`release.footer`,
`commit`→`commit.message`, `ticket`→`commit.ticket`, `contributor`→`commit.contributor`);
updated `internal/config/validator.go`'s `validTemplateBlocks`/`validTemplateBlocksHint` to the
new dotted set. `title`/`subtitle`/`footer`/`changelog`/`release_notes` (document-level/root
blocks, no release/commit-scoped counterpart) are untouched, confirming the ADR's naming
decision.

TDD: `internal/config/templates_test.go` (new — flat keys unchanged, nested
release/commit flatten to dotted keys, mixed flat+nested, a non-mapping `release`/`commit` value
passes through as an ordinary flat key, an empty nested object contributes nothing, a
too-deeply-nested sub-value errors naming its dotted path) confirmed failing to compile
(`undefined: TemplateOverrides`) before `internal/config/templates.go` landed.
`internal/config/validator_test.go` gained `TestValidate_RenderingTemplatesNamespacedBlocksValid`,
`TestValidate_RenderingTemplatesOldFlatBlockKeysRejected` (table-driven, all 8 renamed flat names),
and `TestValidate_RenderingTemplatesUnknownNestedSubBlockRejected`, and updated the
pre-existing `NativeValid`/`TicketValid`/`BadSnippet`/`RenamedBlocksValid`/`OldHeaderKeyRejected`
tests to the new namespaced keys. `internal/generators/native`'s existing override-behavior tests
(`TestGenerate_InlineCommitOverride`, `TestGenerate_ReleaseFooterOverride`,
`TestGenerateChangelog_IncrementalWithCustomHeader`, `TestRenderChangelogSection_TicketBlockOverride`,
`TestRenderReleaseNotes_InlineCommitOverride`, `TestCommitBlock_*`) had their snippet-map keys
renamed and were confirmed failing (`template: no template "commit.message" associated with
template "native"` and equivalent override-not-applied assertions) against the pre-rename
`.tmpl` files before the block renames landed.

Full suite (`go test ./...`), `go build`, `go vet`, and `hk check` (`go_fmt`, `golangci_lint`,
`typos`) all green. Scope held to `internal/config` + `internal/generators/native` as planned —
`docs/heraut.sample.yml`'s `rendering.templates` example is entirely commented out (never
live-parsed) and `testdata/config/valid/rendering-templates.yml`/`schema.json` are exercised only
by `schema_test.go`'s JSON-Schema-only checks, so neither needed touching to keep the suite green;
both still carry the pre-ADR-0059 flat names and go stale until T291 updates them alongside the
spec/guide.

#### ✦ `[x]` T291: Docs — spec, schema, sample config, guide for the namespaced block set

`schema.json`'s `rendering.templates` object restructured: `title`/`subtitle`/`footer`/
`changelog`/`release_notes` stay flat properties; `release`/`commit` are new nested objects
(each `additionalProperties: false`) with the renamed sub-properties (`release.section`,
`release.group`, `release.contributors`, `release.stats`, `release.footer`; `commit.message`,
`commit.ticket`, `commit.contributor`). `testdata/config/valid/rendering-templates.yml` (the
`schema_test.go` fixture demonstrating this block) updated to the nested syntax to match —
otherwise it would have started failing `TestSchema_ValidFixtures` against the new schema.
`docs/specs/05-generators-and-platforms.md`'s "User-customizable templates" section (overridable
block list, YAML example, data-model prose, the anchor-comment cross-reference) and
`docs/specs/02-configuration.md`'s `rendering.templates` paragraph updated to the new names.
`docs/heraut.sample.yml`'s commented `rendering.templates` example (never live-parsed, so it
didn't need to change for T290's tests to pass, but was stale documentation) rewritten to the
nested form. `docs/guides/template-customization.md` — the "Overridable blocks" table, every
worked example, the "four layers together" example, the full-template-file `.tmpl` example, the
data-contract prose, and the Gotchas section — updated throughout; the intro's ADR list gained
[ADR-0059](../adr/0059-namespaced-template-blocks.md).

**Deliberately left untouched:** the `### User-customizable templates (ADR-0037, ADR-0048)`
spec-05 heading itself, and its guide backlink — established precedent (ADR-0049/ADR-0050/
ADR-0051 all amended this same section without ever being appended to the heading) is that the
heading anchor stays fixed to the founding ADRs; later amending ADRs are cited in prose instead.
Appending ADR-0059 to the heading would have changed its Markdown anchor slug and broken every
cross-reference to it (`.claude/rules/coding.md`, spec 02, the guide). `CHANGELOG.md`,
`docs/tasks/roadmap.md`'s own history, and the ADR files themselves (`0037`, `0048`, `0049`,
`0057`, `README.md`) also still name the pre-ADR-0059 flat block names where they describe what
those ADRs did *at the time* — correct as historical record, not doc drift.

Full suite (`go test ./...`, including `TestShippedExamples_LoadAndValidate` and
`TestSchema_ValidFixtures`/`TestSchema_InvalidFixtures`), `go build`, `go vet`, and `hk check`
(`yamlfmt`, `typos`) all green. This closes Phase 49 — namespaced template blocks: all of
T289–T291 are done.

---

### Phase 50 — Move `rendering.trailers` to `rendering.templates.commit.trailers`

ADR-0057 flagged its own eventual relocation ("`rendering.trailers`'s location would likely move
again if/when it lands, e.g. under `release.commit.footers`"). Reviewing Phase 49's namespacing
work surfaced exactly that follow-up: `rendering.trailers` is a per-commit-footer-token
customization, conceptually commit-cadence like `commit.message`/`commit.ticket`/
`commit.contributor` (ADR-0059), but was left behind at the old flat path. See
[ADR-0060](../adr/0060-rendering-commit-trailers-path.md): move it to
`rendering.templates.commit.trailers` — inside the *existing* `rendering.templates.commit`
object, alongside `message`/`ticket`/`contributor`, so every commit-cadence customization lives
under one discoverable path. Breaking rename, no alias — pre-v1.0, same precedent as
ADR-0048/ADR-0049/ADR-0059.

**Mid-phase correction.** T292's first cut of ADR-0060 placed the rule list at
`rendering.commit.trailers` instead — a new `commit:` object *sibling* to `rendering.templates`,
chosen to sidestep a real implementation wrinkle (see T293). Implemented and documented, then
reviewed again before ever being relied on by a tagged release: two different `commit`
namespaces at two different depths under `rendering:` defeated the whole point of ADR-0059's
namespacing. Corrected by amending ADR-0060 **in place** (not superseding it with a new ADR) —
the user's explicit call, since nothing outside this repository ever depended on the interim
path. T293/T294 below describe the corrected, final implementation directly; ADR-0060 itself
documents both the correction and why it was made in its own Context section.

#### ✦ `[x]` T292: ADR-0060 — move `rendering.trailers` to `rendering.templates.commit.trailers`

Wrote [ADR-0060](../adr/0060-rendering-commit-trailers-path.md) after the user asked whether
Phase 49 had covered relocating `rendering.trailers` too (it hadn't — a genuinely missed
follow-up, not part of ADR-0059's scope). Considered and rejected `commit.footer`/
`commit.footers` (the name first proposed) in favor of keeping ADR-0057's own `trailers` term:
`footer` (document-level credit-line block, ADR-0049) and `release.footer` (per-release trailing
block, ADR-0059) already exist, so a third, differently-shaped `footer` at `commit.footer` would
reopen the exact same-word-different-meaning ambiguity ADR-0048/ADR-0057 each deliberately
avoided. The first-cut placement (`rendering.commit.trailers`, a sibling of `rendering.templates`)
was corrected in place after implementation, once it produced two different `commit` namespaces
at two depths — see the mid-phase correction note above and ADR-0060's own Context section for
the full account.

#### ✦ `[x]` T293: `internal/config` + `internal/app`: implement `rendering.templates.commit.trailers`

**The implementation wrinkle driving the design.** `rendering.templates` is parsed by
`config.TemplateOverrides` (`map[string]string`, ADR-0059) — every leaf under `release:`/
`commit:` must be a template-snippet string. `trailers` is a *list* of `{token, renderer, hide}`
rules, so it can't flow through that flat map unmodified. Rather than accept a heterogeneously
typed map, `Rendering` (`internal/config/commits.go`) gained its own `UnmarshalYAML`
(`internal/config/templates.go`): it decodes `excludes`/`templates` itself (re-implementing
"unknown field" strictness for its own two keys, since a custom `Unmarshaler` bypasses the outer
`forgeconfig.Decode`'s `KnownFields` strictness for its own node — confirmed by
`TestRendering_UnmarshalYAML_UnknownFieldRejected`), then makes a second pass over the same raw
`templates` node via the new `extractCommitTrailers` helper, pulling `commit.trailers` out into
the (unchanged from T293's first cut) `Rendering.Commit *RenderingCommit{Trailers
[]FooterRule}`. `TemplateOverrides.UnmarshalYAML` gained one line: it skips a `trailers` sub-key
under `commit:` instead of trying (and failing) to decode it as a string.

Everything built on top of `Rendering.Commit` in the first cut carries over unchanged:
`mergeRenderingCommit`, `effectiveTrailers` (`internal/app/pipeline.go`), and
`validateTrailers`'s nil-safe call site (`internal/config/validator.go`) — only
`validateTrailers`'s error-path string prefix changed, to `rendering.templates.commit.trailers[i].*`.
The `removedKeys` migration hint (`internal/config/loader.go`) for the original top-level
`rendering.trailers` now points at `rendering.templates.commit.trailers`. No changes needed in
`internal/generators/native` — `FooterRule`'s shape, `buildCommit`, and
`release_notes.tmpl`'s `.Footers` loop are untouched throughout this whole phase, exactly as
ADR-0060 predicted.

**Scoping note on the `removedKeys` migration hint** (unchanged from the first cut): only the
top-level (global) `rendering.trailers` is probed for a friendly migration error. A project using
the old key at `changelog.rendering.trailers` / `release.notes.rendering.trailers` (or their
per-env variants) still gets a generic strict-decode error rather than this hint — full coverage
would need `rendering` sub-probes nested inside each of the `changelog`/`release.notes`/per-env
probe structs already in `checkRemovedKeys`, a larger change than this phase scoped for a path
move. Documented as a known limitation in `checkRemovedKeys`'s doc comment.

TDD: `internal/config/templates_test.go` gained `TestTemplateOverrides_CommitTrailersSkipped`,
confirmed failing (list-into-string decode error) before the skip logic landed.
`internal/config/commits_test.go` gained four `TestRendering_UnmarshalYAML_*` tests (extracts
trailers into `Commit`, leaves `Commit` nil when absent, preserves excludes, rejects an unknown
top-level key) — the first and last confirmed failing (decode error / no error where one was
expected) before `Rendering.UnmarshalYAML` landed. `internal/config/validator_test.go`'s five
`TestValidate_RenderingTrailers*` tests and `internal/config/migration_test.go`'s
`rendering.trailers` row were updated to the final nested YAML shape and error paths.
`internal/config/merge_test.go` and `internal/app/templates_internal_test.go`'s trailers tests
needed no changes in this task — they construct `config.Rendering{Commit: &config.RenderingCommit{...}}`
directly as Go literals, never through YAML decoding, so they were unaffected by moving *how*
`Commit` gets populated.

Full suite (`go test ./...`), `go build`, `go vet`, and `hk check` (`go_fmt`, `golangci_lint`,
`typos`) all green.

#### ✦ `[x]` T294: Docs — spec, schema, sample config, guide for `rendering.templates.commit.trailers`

`schema.json`'s `rendering.templates.commit` object gained a `trailers` array property alongside
`message`/`ticket`/`contributor` (JSON Schema has no trouble with a `string`-typed sibling next
to an `array`-typed one in the same object — the type constraint that drove `Rendering`'s custom
decode is a Go implementation detail, not a schema one); the standalone `commit` object the first
cut added at the `Rendering` level was removed. `testdata/config/valid/rendering-trailers.yml`
(the `schema_test.go` fixture) updated to match. `docs/specs/02-configuration.md`'s `###
rendering.trailers (ADR-0057, ADR-0058)` section and `docs/specs/05-generators-and-platforms.md`'s
`.Footers`/`.Line` data-model prose updated to the final path, citing ADR-0060.
`docs/heraut.sample.yml`'s commented trailers example folded into the existing `templates:`
block's `commit:` entry (previously a separate `commit:` block, now `trailers:` sits alongside
`message`/`ticket`/`contributor`). `docs/guides/template-customization.md` — the intro's ADR
list, the "skip ahead" pointer, the "Customizing footer trailers" section's example/table/prose,
the data-contract `Footer` entry, and both Gotchas bullets — updated throughout.

**Deliberately left untouched, same precedent T291 set:** both the spec-02 heading (`###
rendering.trailers (ADR-0057, ADR-0058)`) and the guide's matching heading (`## Customizing
footer trailers (`rendering.trailers`, ADR-0057)`) keep their exact pre-ADR-0060 text — changing
either would change its Markdown anchor slug and break the cross-references to it (spec 05, the
guide's own intro/data-contract/gotchas, `.claude/rules/coding.md` indirectly via spec 05).
ADR-0060 and the final path are cited in prose under the heading instead. ADR-0060's own filename
(`0060-rendering-commit-trailers-path.md`) was likewise left unrenamed to avoid a second cascade
of link updates across README.md/specs/guide — filenames don't need to literally encode an ADR's
final decision (ADR-0057's own filename, "rendering-trailers", already doesn't). A stray
pre-existing inaccuracy in spec 02 — "`rendering.templates.commit` override" (missing
`.message`, predating even ADR-0059) — was also corrected while this section was open, since it
was touched anyway.

Full suite (`go test ./...`, including `TestSchema_ValidFixtures`/`TestSchema_InvalidFixtures`),
`go build`, `go vet`, and `hk check` (`yamlfmt`, `typos`) all green. This closes Phase 50 — move
`rendering.trailers` to `rendering.templates.commit.trailers`: all of T292–T294 are done.

---

### Phase 51 — Hook-declared file staging

A hook entry (`post_bump`/`pre_changelog` only) can declare `stage: [...]` — file(s)/pattern(s)
it produces, staged into the same commit as `CHANGELOG.md`. Investigated cocogitto's equivalent
(`pre_bump_hooks` + a blanket `git add -A` via `add_all()`, confirmed from its actual source) and
deliberately rejected that blanket approach in favor of explicit, opt-in declarations — no
unrelated dirty file ever rides along into a release commit. Reuses `git add`'s own pathspec
matching (no new glob dependency); a zero-match pattern is already a `git add` failure, so no new
error-detection code is needed either. **Deliberate breaking change**: every hook point's list
entries become object-only (`{run: "...", stage: [...]}` — `run` required) — the bare-string
shorthand ADR-0053 shipped is no longer accepted. New ADR-0061. Six tasks (config schema →
validator → pipeline/app plumbing → commit staging → integration test → docs), so the task
breakdown and live `[ ] / [x]` status live in a dedicated roadmap:

→ **[Hook File Staging Roadmap](hook-file-staging-roadmap.md)** — T295+

Design: [`docs/superpowers/specs/2026-09-17-hook-file-staging-design.md`](../superpowers/specs/2026-09-17-hook-file-staging-design.md).

---

### Phase 52 — Selective hook skipping

`--no-hooks` is all-or-nothing. A repeatable, comma-separable `--skip-hook <point>` flag on
`heraut release` / `heraut changelog` (and a `HERAUT_SKIP_HOOKS` env var carrying the same
comma-separated list) skips individual hook *points* (`post_bump`, `pre_changelog`, `pre_tag`,
`post_tag`, `pre_release`, `post_release`) for one run. Granularity is the point, not the
individual step — steps have no identifier today and naming them would need a config-schema change.
No `config.go`/`schema.json`/sample change. `--skip-hook` together with `--no-hooks` is a config
error; `heraut changelog` rejects `pre_release`/`post_release` (it never publishes). The skip is
applied in `internal/app` by emptying the skipped points' step lists after config translation, so
`internal/pipeline` (dry-run lines, step totals, `stage` collection) is untouched. New ADR-0062.

#### ✦ `[x]` T301: `--skip-hook` flag + `HERAUT_SKIP_HOOKS` env var

Files: `internal/app` (validation + skip application + `PipelineOpts.SkipHooks`), `internal/cmd`
(`release.go`, `changelog.go`, flag/env resolution helper), tests at each layer, Spec 02 § hooks,
Spec 03 flag tables, `docs/guides/release-pipeline-and-hooks.md`, ADR-0062 + ADR index,
`CLAUDE.md` ADR counts.

**Completion note (2026-09-20).** Implemented as designed: `app.ValidateSkipHooks` normalizes
(trim, drop blanks, dedup) and checks against the invoking command's allowed points, and
`applySkipHooks`/`applyChangelogSkipHooks` empty the skipped points' step lists right after config
translation, so `internal/pipeline` needed no change at all — dry-run lines, `[N/total]` counters
and `stage` collection already treat an empty list as "not configured". `internal/cmd/skiphooks.go`
holds the flag declaration (with shell completion limited to the command's own points) and
`resolveSkipHooks`, called first in each `RunE` so a bad value fails before the config is read.
Decisions the design left open, settled with the user or by precedent: `--skip-hook` + `--no-hooks`
is a `Config` error; a release-only point on `changelog` is rejected; the env var is an ambient
default, so an explicit flag *replaces* it (like `--config` over `HERAUT_FILE`) and `--no-hooks`
beats it without erroring. The env var is validated per command exactly like the flag, so a job-wide
`HERAUT_SKIP_HOOKS=post_release` fails any `heraut changelog` step — relaxing that for the env var
alone is a small, backward-compatible change (ADR-0062 Consequences) and was deferred rather than
guessed at. Error text leads with a plain word ("Cannot combine…", "Invalid value in …") because
fang capitalizes the first letter of an error and mangled `--skip-hook` into `--Skip-Hook` (an
existing quirk — `--set-build-id`'s error does the same — left alone). Deferred, not built:
per-step selection (steps have no identifier; needs a `name:` config field). Tests: table-driven
`internal/app` unit tests, an end-to-end dry-run `BuildPipeline` test (mutation-checked: removing
the `applySkipHooks` call fails it), and real-git-repo `changelog` tests for flag, repeat/comma
forms, env var, flag-over-env precedence and `--no-hooks`-over-env. There is no real-repo
`release` test: it needs a resolvable publish target, and the shared wiring is already proven by
the changelog tests plus the `BuildPipeline` one. Full suite, `-race` on `internal/app` and
`internal/cmd`, and `hk check` green.

---

### Phase 53 — Stay at v0

`DetermineBump` resolves any breaking commit to a major bump, so a project that deliberately stays
pre-1.0 (heraut itself) is one `feat!` away from an unintended `v1.0.0`. ADR-0052's
`{breaking: true, bump: minor}` override can demote it, but silently, permanently and with no
per-run lift. New `versioning.bump.stay_at_v0: true`: while the current major is `0`, a
release-level major bump is held back to minor with a visible warning naming the commits, and a
dedicated `--allow-major` flag (on `release`, `changelog`, `version next`) lifts it for one run.
Self-retiring at 1.0. Deliberately **not** `--force` (already two unrelated meanings), **not** a
hard error (would fail every unattended release containing a breaking commit) and **not** a
permanent ceiling — SemVer §4 allows breaking changes in `0.y.z`, but §8 requires a major bump
above 1.0, so a demotion there would make the version lie. Two tasks; new ADR-0063.

Design: [`docs/superpowers/specs/2026-09-20-stay-at-v0-design.md`](../superpowers/specs/2026-09-20-stay-at-v0-design.md).
Plan: [`.claude/plans/phase-53-stay-at-v0.md`](../../.claude/plans/phase-53-stay-at-v0.md).

#### ✦ `[x]` T302: `stay_at_v0` config + resolver hold-back logic

`internal/config` (`BumpConfig.StayAtV0`, `Versioning.StayAtV0()`, `schema.json`, sample, valid
fixture), `internal/versioning/semver` (`holdMajorAtZero`, `majorCommits`, `SetAllowMajor`,
`Warnings()`). Unit level; no CLI surface yet.

**Completion note (2026-09-20).** Implemented as designed, with no behavioural deviation from the
design doc; the one structural refinement is that `resolveAuto` and `BumpAuto` both go through a
private `(*Resolver).determineBump` rather than each calling `holdMajorAtZero` directly (same
behaviour, one shared clamp — ADR-0063 must describe what shipped). `BumpConfig.StayAtV0` and a
nil-safe `Versioning.StayAtV0()` landed with the `schema.json` property, the
`docs/heraut.sample.yml` entry (`stay_at_v0: false`) and a valid fixture
`testdata/config/valid/semver-stay-at-v0.yml` (74d7617). `holdMajorAtZero` and `majorCommits` live
in the new `internal/versioning/semver/hold.go`; `determineBump` wraps `DetermineBump` with the
hold-back, so `semver-per-env` "auto" environments get it without a separate path. `SetAllowMajor`
lifts it and `Warnings()` exposes what it recorded (c560df7). There is no CLI surface yet: the
setting is reachable only through config until T303 adds `--allow-major`, so four forward
references dangle until T303 lands — the `ADR-0063` reference in the `config.go` comment, the
`--allow-major` mention in the sample, the `stay_at_v0` description in `schema.json` ("Lift it for
one run with --allow-major") and the runtime warning text built in
`internal/versioning/semver/hold.go` ("pass --allow-major to release …"). A user who sets
`stay_at_v0: true` before T303 lands is therefore told to pass a flag that does not exist yet
(heraut's own `.config/heraut.yml` does not enable the key, so nothing dogfooded is affected, but no
release should be cut from `main` between T302 and T303). Full suite green (1944 tests at c560df7)
and `hk check` clean; a mutation check (removing the hold-back call) made the new resolver tests
fail. The final whole-branch review (Opus) returned "ready to merge with fixes"; its two test-guard
fixes (a guard for the `Resolve()` warning reset and an exactly-five-major-commits boundary row,
the latter listing all five with no "… and N more" line) landed as a follow-up commit (e91a0cc). Deferred
minors, not covered: `--set-version` under auto mode with `stay_at_v0` (folded into T303's `app`
resolver-warnings tests) and the no-tags-yet case (guarded by early returns this change did not
touch).

#### ✦ `[x]` T303: `--allow-major`, warning output, docs, ADR-0063, dogfood

`internal/ui` (`WarnLines`), `internal/versioning` (`Result.Warnings`), `internal/app`
(`WithAllowMajor`, warning-copying resolver wrapper), `internal/pipeline` (print warnings after the
resolve step), `internal/cmd` (`--allow-major` on `release`/`changelog`/`version next`;
`version next` warns on stderr), real-repo tests, Spec 03/04, ADR-0063 + index, `CLAUDE.md` ADR counts, and `stay_at_v0: true` in
`.config/heraut.yml`. Depends on T302.

**Completion note (2026-09-20).** Built in four code commits plus one docs commit. `ui.WarnLines`
(`internal/ui/status.go`, fec15d9) prints a multi-line warning; `versioning.Result.Warnings`, the
variadic `NewResolver(..., opts ...ResolverOption)` with `app.WithAllowMajor`, and the
`warningResolver` wrapper in `internal/app/resolver.go` (f2969d6) carry the semver resolver's
recorded warnings into the result — the wrapper copies them after `Resolve`, which is why
`perenv.VersionCalculator` and its interface stayed untouched; `printResolveWarnings`
(`internal/pipeline/warn.go`), called right after Step 1 in `release.go` and `changelog.go`
(1d3a947); and `--allow-major` on `release`, `changelog` and `version next` but not `version
current` (2b88e22), where `version next` prints warnings to stderr so stdout stays exactly the tag.
ADR-0063, Specs 03/04, the ADR index and the `CLAUDE.md` counts (62 → 63), the
`docs/heraut.sample.yml` `bump:` header fix, and `stay_at_v0: true` in heraut's own
`.config/heraut.yml` landed together (3f9f764). Three design-doc corrections were made while
planning and are already committed: warnings are printed by the pipeline through `ui.WarnLines`
rather than as spinner sub-lines, because the spinner renders sub-lines with a green check mark;
the warning text uses bare versions, because per-env resolvers never see the tag format; and Spec
02 has no `versioning.bump` section, so only Specs 03/04 changed. `--allow-major` is a silent no-op
when nothing is held back — without `stay_at_v0`, with `--set-version`, once the major is >= 1,
under CalVer, and on promote environments. This resolves all four T302 forward references (the
ADR-0063 comment in `config.go`, and the `--allow-major` mentions in `schema.json`, the sample and
the runtime warning text). Verification: full suite green (1965 tests), `-race` on
`internal/cmd`, `internal/app` and `internal/pipeline`, `hk check` clean, and `go run
./cmd/heraut version next` on heraut's own history prints `v0.69.0` with no warning and `check
config` passes with the dogfood key enabled. Deferred minors, not covered: a few test-breadth gaps
in `internal/pipeline` and `internal/app` (failed-resolve and CalVer guards, ordering of several
warnings). T304 remains unscheduled.

#### ✦ `[ ]` T304: (future, not scheduled) major-bump gate for versions ≥ 1

A separate, deliberately deferred decision: a setting under which — for **every** major version,
not only v0 — an automatic major bump *fails* with a clear message until the run is repeated with
`--allow-major`, so a major release is always a conscious act. It must be an error, not a
demotion: releasing a breaking change as a minor above 1.0 violates SemVer §8 and Conventional
Commits' `BREAKING CHANGE` ↔ MAJOR mapping. Reuses T303's `--allow-major`. Needs its own design
pass (setting name and placement, interaction with `stay_at_v0`, per-env behaviour) before any
work; do not start it without one.

---

### Phase 54 — Phase 53 follow-ups

Small items the Phase 53 reviews found and deliberately did not fix inside that phase — none
blocked anything; T309 (new flags on `version next`) and T312 (an exit-code fix) changed shipped
behaviour and the rest are tests, docs and hygiene (T304 stays its own deferred design decision).
Each task below was independent and could be picked up alone, in any order.

#### ✦ `[x]` T305: stop claiming `heraut version next` accepts `--set-version`

`docs/heraut.sample.yml`'s `bump.mode` comment and Spec 04 § Manual mode both said `--set-version`
can be passed to `heraut version next`; that command has no such flag (found by the Phase 53 final
review, which ran the binary). Both now name `heraut release` / `heraut changelog` — the two
commands that do take it — and state plainly that `version next` cannot resolve a version under
`bump.mode: manual` (it fails with "Manual bump mode requires --set-version flag", exit 3,
verified against the binary). Docs-only; `TestShippedExamples_LoadAndValidate` still passes. Whether
`version next` *should* work in manual mode is a separate question, filed as T309.

#### ✦ `[x]` T306: Phase 53 code hygiene (three latent nits)

None changes behaviour today; each is a one-line hardening or rename the Phase 53 reviews flagged.
(a) `ui.WarnLines` (`internal/ui/status.go:25`) prints a stray blank line when `msg` ends in `\n`
(`strings.Cut` leaves a trailing empty segment and `Fprintln` adds another newline); no producer
emits one, but the helper's signature advertises general use — `strings.TrimRight(msg, "\n")` plus
two `TestWarnLines` rows (trailing newline, empty message). (b) `determineBump`
(`internal/versioning/semver/resolver.go:83`) assigns `r.warnings = []string{warning}` rather than
appending; correct while there is one hold per resolution and both entry points reset on entry, but
it would silently drop an earlier warning if a future path (the monorepo epic resolves per module)
called `determineBump` twice in one resolution — use `append`, mirroring the fix already made in
`warningResolver`. (c) Naming: `maxHeldBackCommits` (`hold.go:11`) caps how many commit subjects the
warning *lists*, not how many are held back (`maxListedCommits`); `determineBump`
(`resolver.go:75`) differs from the exported `DetermineBump` by one capital letter at both call
sites (`bumpAfterHold` reads unambiguously).

Completed 2026-09-21. (a) `ui.WarnLines` now trims trailing newlines from `msg` before splitting, so
`"held back\n"` prints exactly one line and a message ending after detail lines no longer emits a
stray blank line; three rows were added to `TestWarnLines` (trailing newline, trailing newline after
detail lines, and empty message, which pins the existing `"! \n"` output), the first two failing
before the fix. (b) `(*Resolver).bumpAfterHold` now appends to `r.warnings` instead of
overwriting it; a new internal test, `TestBumpAfterHold_AppendsAcrossCalls` in
`internal/versioning/semver/hold_internal_test.go` (the package's other tests are external), calls it
twice in one resolution and failed with only the second warning surviving. The resets at the top of
`Resolve` and `BumpAuto` still keep warnings scoped to a single resolution, and the existing
`WarningsResetBetweenCalls` and `ReturnsACopy` tests pass unchanged. (c) `determineBump` was renamed
`bumpAfterHold` (definition, doc comment and both call sites) and `maxHeldBackCommits` was renamed
`maxListedCommits`; ADR-0063 was updated to the new helper name, while the completed T302 and T303
notes, the design doc and the plan file deliberately keep the old names as historical record.
Verification: full suite green with no failures, and `hk check` clean.

#### ✦ `[x]` T307: Phase 53 test-breadth gaps

Guards, not bug fixes — every behaviour below was verified by hand during the reviews.
`internal/app/resolver_warnings_test.go`: `calver` and `calver-per-env` come back from `NewResolver`
unwrapped and warning-free even with `stay_at_v0: true` set (checked manually, untested).
`internal/pipeline/resolve_warnings_test.go`: several warnings print in order; the non-dry-run
path; the `ChangelogPipeline` `DisableChangelog && !Tag` path (the warning must still print before
that early return); a direct table test for `printResolveWarnings` (nil / one / two entries); and
`TestRun_NoResolveWarnings_PrintsNoWarningLine` only asserts `NotContains "held back"`, which is
weaker than its name — tighten it to the warning-line shape without colliding with the
`! changelog disabled` line. `internal/cmd/allowmajor_test.go`: `version next` whose `Resolve()`
fails prints no warning (guaranteed by structure today: the error return precedes the loop); the
two real-git `changelog` rows use the merged-stream `executeRoot`, so they cannot prove which
stream the pipeline's warning goes to — add one run through `executeRootSeparateStreams`.
`internal/versioning/semver`: no test for "no tags yet" under `stay_at_v0` (returns the initial
version, no warning, via the untouched early returns).

Completed 2026-09-21. Tests only; no production file changed, and every new test passed against the
existing code on first run. `internal/app`: `calver` and `calver-per-env` with `stay_at_v0: true`
resolve warning-free after a single tag-list call (no commit walk). `internal/pipeline`: several
warnings print in order as separate `! ` headlines from both pipelines, the release warning still
prints on a non-dry-run that really publishes, the `ChangelogPipeline` `DisableChangelog && !Tag`
early return prints the warning before the `changelog disabled` line, and a direct table test
(nil, empty, one, two, detail lines, detail then headline) pins `printResolveWarnings`;
`TestRun_NoResolveWarnings_PrintsNoWarningLine` now also asserts no output line begins with `! `
(the release dry-run prints none, so no exception was needed). `internal/cmd`: `version next` with
a failing `git log` returns a `reading git log` error with empty stdout and no warning on stderr,
and one real-git `changelog --tag --no-push` run through `executeRootSeparateStreams` proves the
hold-back headline lands on stdout, not stderr. `internal/versioning/semver`: with no tags,
`Resolve` and `BumpAuto` return `0.1.0` with `BumpNone` and no warning. Mutations verified, each
tripping the intended test and restored (production diff empty): deleting or reordering the
`printResolveWarnings` call in `changelog.go` (deleted, moved past the early return, and printed
after the disabled line); in `release.go` printing only in dry-run, only the first warning, or not
at all; reversing the loop or printing only headlines in `warn.go`; emitting a spurious warning
when there are none; wrapping `calver` / `calver-per-env` in a `warningResolver` that yields a
warning; printing the tag or a warning on the `version next` failure path; routing the changelog
pipeline to stderr; and recording a warning, reporting a bump, or walking the log on the semver
no-tags branches. Limits: the `version next` guard is only partly mutation-covered, because a
failed resolve cannot carry warnings today, so moving the warning loop above the `err` check alone
is undetectable and trips the test only when paired with a resolver that leaks warnings on error
or with the tag print moved too; and because `warningResolver` is unexported, the calver tests can
assert only empty warnings, not that the resolver is unwrapped.

#### ✦ `[x]` T308: Phase 53 docs polish

Cosmetic. `docs/specs/03-commands.md`: the `changelog` `--allow-major` row lacks the "no effect"
list (without `stay_at_v0`, with `--set-version`, under `bump.mode: manual`, once major ≥ 1) that the
`release` row has. `docs/adr/0063-hold-major-at-v0.md`: the illustrative warning shows the raw
`Result.Warnings` entry (no `!` glyph) while Spec 04 shows the rendered line — add a clause saying
which is which. `docs/heraut.sample.yml` (`stay_at_v0` paragraph, ~lines 74-76): a review edit
replaced "applies to semver-per-env auto environments too" with "still applies", which no longer
says *auto* (promote environments are unaffected) — restore the precise wording. Lines past the
~100-column wrap in ADR-0063 (~31-32, ~72-73) and the design doc (~144). This file's "Open items"
sentence (~line 228) is true but clunky ("Phase 10's closing checkpoint" is itself `[x]`; the
open item is its sub-checkbox). The plan file's embedded copies of the sample/Spec 04 paragraphs
keep the pre-review "ignored under `bump.mode: manual`" wording — historical, leave.

Completed 2026-09-21. Docs only, no Go or schema change. (1) Spec 03's `changelog` `--allow-major`
row now carries the same "no effect" list as the `release` row: without `stay_at_v0`, with
`--set-version`, once the major is ≥ 1, or, for the `semver` strategy, under `bump.mode: manual`
(plus "Deliberately not `--force`"). The `release` row turned out to omit the `bump.mode: manual`
case the task text assumed it had, although Spec 04 and the resolver both make the flag a no-op
there, so it gained the same clause and the two rows now read identically. (2) ADR-0063's warning
block is now introduced as the raw `Result.Warnings` entry text, with a clause saying the pipelines
and `version next` render its first line with a `! ` prefix through `ui.WarnLines`, as in Spec 04's
example. (3) The sample's `stay_at_v0` comment says again that the setting applies to the
`bump: auto` environments of `semver-per-env` and does not apply to `bump: promote` environments or
to CalVer, matching the `bump:` header comment above it. (4) Rewrapped only the prose the review
edits had made long, words unchanged: ADR-0063's "The rule." and "silent no-op" bullets and the
design doc's `--allow-major` paragraph; the 101-108-column lines that predate Phase 53, code blocks,
headings and the ADR header were left alone. (5) The "Open items" sentence now reads "outside
Phases 53 and 54" and names the last sub-checkbox of Phase 10's closing checkpoint; a search for
every `[ ]` marker in this file confirms the only open items are T304, T309, T310 and that
sub-checkbox. Left alone on purpose: the plan file's embedded copies of the pre-review wording, T304
and T309/T310, and the Phase 54 table row, which stays "In progress". Verification:
`go test ./internal/config/` and `go test ./...` green, `hk check` clean.

#### ✦ `[x]` T309: `heraut version next` in manual mode is a dead end

Under `bump.mode: manual`, `heraut version next` always fails with "Manual bump mode requires
--set-version flag" (exit 3) — but `version next` has no `--set-version` flag, so the message names
something the user cannot pass (T305 corrected the docs that claimed otherwise). Related existing
limitation (Spec 03 note under `version next`): it also cannot render a tag that needs a `{build}`
ID because it has no `--set-build-id`. Needs a small decision before code: (1) give `version next`
`--set-version` / `--set-build-id` so it echoes the tag it would produce for a given version —
useful for CI, and fixes both limitations — or (2) keep it compute-only and make the manual-mode
error say that `version next` has nothing to compute there and point at `release --set-version`.
Either way update Spec 03 and Spec 04 § Manual mode.

**Completion note (2026-09-21).** Option (1) was chosen, with both flags: `heraut version next` now
takes `--set-version` and `--set-build-id`, so it echoes the tag `release` / `changelog` would
create for a given version, which makes it usable under `bump.mode: manual` and able to render
`{build}` tag formats. The code change is plumbing: `newVersionNextCmd` declares the two local flags
(not on `version current`, not persistent) and passes them to `app.NewResolver`, whose existing
static path already does the rendering, so `--set-version 1.2.3` and `v1.2.3` both give `v1.2.3` and
`--allow-major` stays a silent no-op with an explicit version. Validation runs first in `RunE`,
before the config is read, with the Config exit code, and is identical to the other two commands
because the `--set-version` / `--set-build-id` block that `release.go` and `changelog.go` each
carried verbatim moved into `internal/cmd/versionoverride.go` (`validateVersionOverrideFlags` and
`addVersionOverrideFlags`) rather than gaining a third copy; that refactor was made first, with all
186 existing `internal/cmd` tests passing unchanged. Without `--set-version` under manual mode the
existing "manual bump mode requires --set-version flag" error (exit 3) is unchanged, and is now
satisfiable. The `{build}` error in `tagfmt.Render` no longer claims `version next` cannot supply a
build ID; it now says to pass `--set-version <version> --set-build-id <id>` to `heraut changelog`,
`heraut release` or `heraut version next`, and `TestRender_BuildRequiredButEmpty` gained assertions
for `--set-version` and `heraut version next` (no existing assertion needed loosening). Docs: Spec 03
§ `version next` (usage line, a render-mode paragraph, the rewritten `{build}` note), Spec 02
§ `{build}` token (flag list, example, scope text, and the table row that read "cannot render a
build tag"), Spec 04 § Manual mode and the tag-format paragraph, and the sample's `bump.mode`
comment; the "Open items" sentence near the top of this file no longer lists T309. New tests in
`internal/cmd/version_override_test.go` (registered on `version next` only; a git that fails every
call proves no version resolution happens for semver, custom prefix, manual mode and per-env
`{build}`; manual mode without the flag still exits 3; the `{build}` error text; validation failing
before a missing config is read with exit 2; `--allow-major` a no-op) were written first and failed
with "unknown flag". Mutations verified and restored: passing `""` for the override to
`NewResolver` failed the render tests, dropping the build ID failed the per-env `{build}` row, and
removing the validation call failed the four fail-before-config rows. Deviation: `--set-version`
still runs the `--env` and branch-guard checks, so a per-env config with `branch:` set can still
call git for the branch name; only version resolution is skipped, and Spec 03 says so. Verification:
`go test ./...` and `hk check` clean.

#### ✦ `[x]` T310: (decided) surface the hold-back warning in heraut's own release run

heraut dogfoods `stay_at_v0: true` (`.config/heraut.yml`), so a `feat!` landing on `main` now
becomes a minor release with the hold-back warning visible only in the `workflow_dispatch` job
log of `.github/workflows/release.yml`. Decide whether that is loud enough or the workflow should
promote it (a GitHub Actions `::warning::` annotation or a step-summary line, or a `version next`
preview step before the release). Touches CI, so per `.claude/rules/claude.md` it needs explicit
approval before any edit; a pure documentation answer ("the log is enough") is also a valid
outcome.

**Completion note (2026-09-21).** Decision: no workflow change — the "documentation answer" from the
options above; `.github/workflows/release.yml` was not touched. Where the warning appears today:
the bootstrap `heraut version next` inside the forge `release-setup` action's "Resolve version"
sub-step (only once the bootstrap binary — the latest published release — knows `stay_at_v0`), and
the fresh binary's "Version sanity check" step, whose `$(…)` capture takes stdout only so the
warning stays in that step's log. The final `Release` step passes `--set-version "$VERSION"`, which
takes the static path and never resolves a version, so it prints no warning by design. Considered
and not taken because the log was judged enough and each edits CI (which needs explicit approval per
`.claude/rules/claude.md`): a `::warning::` annotation or a `$GITHUB_STEP_SUMMARY` line emitted from
the sanity-check step (about four lines), and an extra `version next` preview step (redundant with
the sanity check). Revisit if a held-back major ever surprises a release. Operational note: the
bootstrap binary is the latest release (v0.68.0 at the time of writing) and rejects the
`stay_at_v0` key in heraut's own `.config/heraut.yml` (`Config: line N: field stay_at_v0 not found in
type config.BumpConfig`, exit 2, verified against a v0.68.0 build), so until the first release
containing the key ships, the workflow must be dispatched with the `version` input set; the
maintainer chose that over a workflow or config change. Verified against the pinned `release-setup`
action (forge v0.7.2): its "Resolve version" step strips only `del(.release)` and runs the bootstrap
`version next` only when the `version` input is empty. Consequence for the "the log is enough"
reasoning above: `release.yml` gates the "Version sanity check" step on
`env.VERSION_OVERRIDDEN == 'false'`, so a dispatch with the `version` input skips BOTH `version next`
runs — the first release that carries `stay_at_v0` shows no hold-back warning in any workflow step.
The mitigation is the maintainer's own: run `heraut version next` locally (it prints the warning on
stderr) to get the value to pass. From the following release on (the bootstrap knows the key and the
input is left empty) both steps run and the warning appears as described above.

#### ✦ `[x]` T311: T309 review polish (two test rows, two wording fixes)

Found by the T309 final review; none is a defect. Tests: `internal/cmd/version_override_test.go`
— `TestVersionNext_BuildTagFormat_WithoutBuildID_ExplainsHowToSupplyOne` asserts the message but not
the exit code (add `exitcode.Config`, like its sibling tests), and Spec 03 now promises the
`--env` / `branch:` guard still runs under `version next --set-version` (verified by hand, exit 3)
but no `cmd`-level test pins it (add one `version next --env <env> --set-version …` row on the wrong
branch; today `CheckBranch` is only unit-tested in `internal/app/branch_test.go`). Docs: Spec 04
§ Manual mode still says `--set-version` "bypasses git calls" (~line 168) one paragraph below the
sentence T309 deliberately hedged to "without resolving a version from git history" — align it;
Spec 03's `version next` paragraph says a leading `v` is accepted, which is true for the default
prefix and `tag_format` strategies but yields `rel-v1.2.3` with `tag_prefix: "rel-"` (pre-existing
`NewResolver` behaviour, identical on all three commands) — add a cross-reference to the
`release` flag table's precise rule. Optional: the `{build}` error text in
`internal/versioning/tagfmt/tagfmt.go` (~line 34) asks a user who already passed `--set-version` to
"pass `--set-version <version> --set-build-id <id>`" — `pass --set-build-id <id> (alongside
--set-version <version>)` reads better; and `internal/cmd/versionoverride.go` /
`version_override_test.go` differ in spelling (the package uses both styles — pick one pair).

**Completion note (2026-09-21).** The four mandatory items were done; the two optional ones were
deliberately left (the maintainer's call): the `{build}` error text in `tagfmt.go` is unchanged, and
`versionoverride.go` / `version_override_test.go` keep their differing spellings. Tests
(`internal/cmd/version_override_test.go`): the `{build}`-without-build-ID row now also asserts
`exitcode.Config` (via `exitcode.Resolve`, like its siblings in that file — no other assertion
changed), and a new table test, `TestVersionNext_SetVersion_StillEnforcesBranchGuard`, pins the
branch guard under `version next --env prod --set-version 1.2.3` with a per-env config declaring
`branch: main` and a FakeBin `git` that answers only `rev-parse --abbrev-ref HEAD` (every other call
fails): a wrong branch exits `exitcode.Runtime` (3) with the "must be operated from branch" message
and empty stdout, the matching branch prints exactly `prod/1.2.3`, and `--force` lets the wrong
branch through. Both new guards passed on first run, as they pin behaviour that was already correct.
Mutation check, restored afterwards: passing `true` for `force` to the `app.CheckBranch` call in
`internal/cmd/version.go` made the wrong-branch row fail ("An error is expected but got nil"), while
the other two rows kept passing. Docs: Spec 04 § Manual mode now says `--set-version` bypasses "the
git calls that version resolution would make" instead of "git calls", matching the sentence above
it; Spec 03's `version next` paragraph now says a leading `v` is accepted with the default prefix or
a `tag_format` but a custom `tag_prefix` strips only itself, cross-referencing the `--set-version`
row of the `heraut release` flag table (checked against `NewResolver`'s static path: `rel-` with
`v1.2.3` yields `rel-v1.2.3`). No production code changed.

#### ✦ `[x]` T312: (decided) one `{build}` render failure, two exit codes

The same "tag format template contains {build} but no build ID was provided" failure exits **2**
when it surfaces from `NewResolver` (an explicit `--set-version` was given, error wrapped as
`exitcode.Config`) and **3** when it surfaces from `Resolve()` (no override, wrapped by `wrapRunErr`
as a runtime error) — on `version next`, `changelog` and `release` alike. Pre-existing on
`release`/`changelog`; T309 made it reachable both ways on `version next`, a command whose output is
meant to be scripted. Decide the intended class (a missing build ID is a usage/config problem, so
`Config` (2) is the natural answer; the alternative is documenting the split), then make the two
paths agree and update Spec 01 § Exit codes / Spec 03 accordingly. Changing an exit code changes
documented behaviour for scripts, so it needs an explicit choice before any code.

**Completion note (2026-09-21).** The maintainer chose option A: a missing build ID is a
configuration error (exit 2) on every path, expressed as a typed sentinel rather than by matching
the message, per the project's error rules (wrap with `%w`, never string-match, sentinels at package
boundaries checked with `errors.Is`) and mirroring `app.IsPromotionGuard`. `tagfmt.Render` now
returns the exported sentinel `tagfmt.ErrBuildIDRequired` (built with `errors.New` from the existing
`buildToken` constant); `app.IsBuildIDRequired` is `errors.Is` against it, so `internal/cmd` need
not import `tagfmt`; and `wrapRunErr` maps it to `exitcode.Config` after the promotion-guard check
(guards keep exit 4, everything else stays Runtime). `version next`, `changelog` and `release` (the
three `wrapRunErr` callers) inherit the mapping, as does the `perenv/promote.go` render through its
`%w` chain (`version current` never renders a tag, so it cannot raise this error); the explicit `--set-version` path already wrapped `exitcode.Config` and is unchanged. The
error text is byte-identical (the existing `tagfmt` assertions pass unchanged). This changes the
exit code from 3 to 2 for a missing build ID on the auto path of all three commands — a documented
behaviour change, acceptable pre-1.0 — and Spec 01 § Exit codes (code 2 row), Spec 02 § `{build}`
token and Spec 03's two `{build}` blockquotes now say so. Tests: `TestRender_ErrBuildIDRequired`
(sentinel present with a `{build}` token and no build ID, absent with a build ID and absent without
the token) plus a missing-`{version}` guard row, `TestIsBuildIDRequired` (nil, sentinel, wrapped
twice, unrelated, promotion guard), and `TestMissingBuildID_IsConfigError_OnEveryPath` in
`internal/cmd/build_id_exit_test.go`, which pins exit 2 for `version next`, `changelog --dry-run` and
`release --dry-run` on both the auto path (FakeBin `git` answering the per-env tag listing and log)
and the explicit `--set-version` path, so the two can no longer drift; promotion guards keep exit 4,
already pinned by `TestExitCode_PromotionGuard_E003`. Written test-first: the auto-path rows failed
with exit 3 before the change. Mutation checks, each restored afterwards: dropping the
`wrapRunErr` branch fails the three auto-path rows; replacing the sentinel with a fresh error of the
same text fails the `tagfmt` row and the three auto-path rows; making `IsBuildIDRequired` return
false fails the helper's sentinel rows and the three auto-path rows. Verification:
`go test ./...` green, `hk check` clean. This was the last open Phase 54 task, so the phase is now
Done.

---

### Phase 55 — Phase 54 follow-ups

Small items found after Phase 54 closed — most from the T309-T312 reviews and deliberately left
unfiled at the time, plus two (T317, T318) from direct user feedback after using the feature.
None blocks anything, none was a defect in what shipped, and each is independent and can be picked
up alone, in any order.

#### ✦ `[x]` T317: show the real tag in the `stay_at_v0` hold-back warning, and clearer wording

Two problems in the warning `holdMajorAtZero` builds, both raised by the user directly (not a
review finding) after using the feature: (1) "pass --allow-major to release 1.0.0" reads, on
`version next`/`changelog`, as if it were naming the `heraut release` subcommand rather than using
"release" as a verb — identical wording on all three commands invites that misreading; (2) the
versions shown are always bare (`1.0.0 → 0.69.0`), because `holdMajorAtZero`
(`internal/versioning/semver/hold.go`) runs inside the version calculator, which for
`semver-per-env` never sees `tag_format` — so the warning doesn't match the real tag
(`dev/1.0.0 → dev/0.69.0`), and even flat `semver` drops the `v` the printed tag has.

**Decided approach**: reword the static text, and compute the real tag shape one layer up, in
`internal/app`'s `warningResolver` — which already holds the fully rendered `Result.Tag` after
`inner.Resolve()` returns — via the same kind of substring heuristic `NewResolver`'s static path
already uses for stripping a configured prefix (`internal/app/resolver.go`'s "Strip any leading
`v`..." comment). `internal/versioning/semver/hold.go` and `resolver.go` gain ONE new piece of
data (the bare "would-be" major version, alongside the existing warning text) via a new
`(*Resolver).WouldBeVersions() []string`, parallel to the existing `Warnings()`; they still never
see `tag_format` and stay unchanged otherwise. `perenv.VersionCalculator` stays untouched (same
reasoning ADR-0063 already gives for not widening it). This is a refinement of ADR-0063's own
mechanism, not a new decision — amend ADR-0063 in place (no new ADR number), matching the T306
precedent of amending it for the `bumpAfterHold` rename.

Final wording (both bugs fixed together):
```
major bump held back by versioning.bump.stay_at_v0: v1.0.0 → v0.69.0 (re-run with --allow-major to release v1.0.0 instead)
```
```
major bump held back by versioning.bump.stay_at_v0: dev/1.0.0 → dev/0.69.0 (re-run with --allow-major to release dev/1.0.0 instead)
```

**Completion note (2026-09-22).** Implemented as designed. One brief inaccuracy, not a deviation:
`internal/app/resolver_warnings_internal_test.go` already existed (the brief called it a "new
file"); extended it instead of creating a duplicate. `holdMajorAtZero`
(`internal/versioning/semver/hold.go`) now returns a third value, the bare "would-be" major
version, alongside the reworded warning text; `(*semver.Resolver).WouldBeVersions()` exposes it
in parallel to the existing `Warnings()`, reset at the same two points (`Resolve`, `BumpAuto`).
`internal/app`'s `warningResolver` — which already held the fully rendered `Result.Tag` after
`inner.Resolve()` — gained a `wouldBeVersions func() []string` field and a `rewriteHeldTags`
helper that substitutes the real tag shape into each warning's **headline only** (never the
commit-subject lines below it), derived from where the bare "held" version appears as a literal
substring of the real tag, via `strings.NewReplacer` so both token replacements run in one
simultaneous pass over the original text. `perenv.VersionCalculator` is untouched, as decided.
Both `NewResolver` construction sites (`semver`, `semver-per-env`) wire the new field.
Mutation checks: (i) dropping the `wouldBeVersions` wiring at the `semver` case site made that
strategy's "shows the real tag" test fail while `semver-per-env`'s kept passing, confirming both
sites are independently covered; (ii) removing the `version == ""` guard in `rewriteHeldTags` made
the guard's dedicated test fail with visibly corrupted output (`strings.Index(tag, "")` returns 0,
not -1, so the function would proceed instead of bailing out) — a real, not theoretical, failure;
(iii) swapping the single `strings.NewReplacer` pass for two sequential `strings.ReplaceAll` calls
was **not** an unforceable mutation — a contrived but valid `tag_format` (one whose static prefix
happens to contain the wouldBe-version text, e.g. `"1.0.0-{version}"`) makes the two approaches
diverge for real: sequential replacement re-scans its own first-pass output and corrupts the
already-substituted tag. That input is now a permanent regression row in `TestRewriteHeldTags`
(`internal/app/resolver_warnings_internal_test.go`), confirmed to fail under the sequential
mutation before being restored. The pre-existing `TestWarningResolver_KeepsWarningsTheInnerResolverAlreadySet`
stub needed a `wouldBeVersions: func() []string { return nil }` field added — not a weakened
assertion, but a required consequence of the struct's new field, since `warningResolver.Resolve`
now calls it unconditionally on the non-error path. ADR-0063 amended in place (no new ADR number,
per the T306 precedent) — its "The warning" bullet's example and prose now describe the real
mechanism instead of claiming per-env warnings can only ever show bare versions; Spec 04's example
updated to match. `internal/pipeline/resolve_warnings_test.go`'s `heldBackWarning` constant needed
no change (confirmed by reading it): it feeds a synthetic `Result` directly, never through the
real resolver/app layer. Full suite green, `hk check` clean.

#### ✦ `[x]` T318: `--allow-major` wording — say "this run", not "re-run"

Direct user question after using T317's wording: "re-run with --allow-major to release v1.0.0
instead" reads as if a SECOND, later invocation of `--allow-major` can still produce the major
after a minor release has already run for real. It cannot — `resolveAuto`/`BumpAuto`
(`internal/versioning/semver/resolver.go`) check `len(commits) == 0` (commits since the *latest*
tag) before `bumpAfterHold` ever runs; once a real run creates `v0.69.0`, the commit that forced
the major is inside that tag's history, so a later invocation — with or without `--allow-major` —
fails with "no commits since v0.69.0", never reaching the hold-back logic at all. `--allow-major`
only ever takes effect on the invocation it is passed to, decided in advance (preview with
`heraut version next` or `--dry-run`, both side-effect-free, then add the flag to the real run if
wanted) — never as a reaction after a real run already tagged the minor. Also confirmed in this
discussion: `--allow-major` is not "useless" under the non-blocking design (rejected: making it
block would break unattended CI, the whole reason the hold-back warns instead of failing) — it is
a plan-then-apply flag, the same shape as `--dry-run` itself, not a react-after-the-fact one. To
get the major after a minor release has already happened, `--set-version` is the only path (it
bypasses git-history resolution entirely).

New wording: `"major bump held back by versioning.bump.stay_at_v0: %s → %s (pass --allow-major on
this run to get %s instead)"` — drops "release" as a word entirely (sidesteps T317's original
subcommand-name ambiguity too) and "on this run" replaces "re-run with", which implied a future
invocation. Spec 04 also gains one clarifying paragraph on the plan-then-apply pattern, and
ADR-0063's existing Consequences bullet about needing to "know to pass it" gains one clause
recording why (can't react after the fact; `--set-version` is the recovery path).

**Completion note (2026-09-22).** Implemented as designed, wording change only — no logic touched.
`internal/versioning/semver/hold.go`'s `fmt.Fprintf` format string is the sole production edit.
`internal/versioning/semver/stay_at_v0_test.go`'s `TestResolve_StayAtV0_WarningFormat` was the only
row anywhere asserting the literal old phrase against production output; grepped the rest of that
file first — the table-driven `wantWarn`/`notWarn` checks only assert `"--allow-major"` and bare
version pairs, unaffected. Two files were deliberately left untouched, per the brief's reasoning,
confirmed by reading each: `internal/app/resolver_warnings_internal_test.go`'s `TestRewriteHeldTags`
table uses the old phrase purely as arbitrary fixture text for a generic string-rewriting function
(`rewriteHeldTags` doesn't care what the text says) — including its adversarial
`"1.0.0-0.2.0"`-tag row, load-bearing for the `strings.NewReplacer`-vs-sequential-`ReplaceAll`
property, not for wording; and `internal/pipeline/resolve_warnings_test.go`'s `heldBackWarning`
constant feeds a synthetic `Result` directly, bypassing the real resolver/app layer (it doesn't even
carry the current "re-run" wording verbatim — further confirmation it was never meant to track
production text). `internal/cmd/allowmajor_test.go`'s end-to-end assertions all check only the
headline's version pair via `Contains` (e.g. `"! major bump held back by
versioning.bump.stay_at_v0: v1.0.0 → v0.69.0"`) — none assert "release" or "re-run" literally, so no
change was needed there; it was not part of the commit. RED: ran
`TestResolve_StayAtV0_WarningFormat` after updating only the test file — failed on the old vs. new
parenthetical, as expected. GREEN: same test after the `hold.go` edit — passed.
`go test ./internal/versioning/semver/... ./internal/app/... ./internal/cmd/...` and the full
`go test ./...` both green afterward, confirming the two untouched files still pass unchanged.
`docs/specs/04-versioning.md`'s example updated and a new paragraph added on `--allow-major` being
plan-then-apply, not react-after-the-fact (with `--set-version` named as the recovery path);
`docs/adr/0063-hold-major-at-v0.md`'s "The warning" example updated and one clause added to the
Consequences bullet about needing to "know to pass it". T317's own task body and completion note
were left untouched, as instructed — they're the historical record of what T317 itself produced.
`hk check` clean.

#### ✦ `[x]` T313: `perenv` promote hint discards a render error

`internal/versioning/perenv/promote.go:210` — `suggested, _ := tagfmt.Render(srcTF, tagfmt.Tokens{Env:
srcEnv, Version: latestDestVersion})` — builds the "you probably meant to promote from `<tag>`" hint
inside the E002 (`ErrDestinationAhead`) error and silently drops `Render`'s error. If the *source*
environment's `tag_format` contains `{build}` while the destination's does not, the hint would render
an empty or malformed tag instead of failing loudly. Found by the T309→T312 final review; no config
reaching it could be constructed at review time (every attempt failed earlier, at line 172, first),
so it is unreachable today but not provably so for every future `tag_format` combination. Fix: either
propagate the error into `ErrDestinationAhead` (it already carries other derived fields) so a failure
here degrades the hint gracefully instead of the caller silently getting `""`, or add an explicit
comment recording why it is safe to ignore if investigation shows it truly cannot fire. TDD: a test
config with a `{build}`-only source `tag_format` reaching this exact line.

**Completion note (2026-09-22).** Took the graceful-degradation direction: `resolvePromote`
(`internal/versioning/perenv/promote.go`) now captures `tagfmt.Render`'s error via `err :=`
(reusing the name is safe — this is a nested `if` block scope distinct from the outer `err`
declared at function top, so it shadows locally and is read by the very next `if err != nil`
check; the outer `err`'s last use was already checked earlier in the function, and nothing after
this block reads it again, so no behaviour changes and `hk check`'s govet pass, which doesn't
enable the shadow analyzer here, stays clean) and, on failure, replaces the suggested tag with a
placeholder — `<no suggested tag — <srcEnv>'s tag_format needs a build ID>` — instead of leaving
the E002 "How to fix" hint with an empty interpolation. New test
`TestPromotionError_E002_SourceTagFormatNeedsBuildID_SuggestionDegradesGracefully`
(`internal/versioning/perenv/resolver_test.go`) drives a `semver-per-env` config where the source
env's `tag_format` is `dev/{version}-{build}` and the destination is ahead (E002); confirmed the
`tagfmt.GlobPattern` output used in the `MockRunner` queue (`dev/*-*`) by reading
`tagfmt.GlobPattern` directly — it matches the brief as given, no correction needed. RED: run
before the fix, both `assert.Contains` (placeholder text) and `assert.NotContains` (`"git tag
<commit-sha>"` with the double space) failed, since the buggy code never produces the placeholder
at all. GREEN: same test after the fix, passes; full package (38 tests) and full suite (2039
tests) green. Mutation check: reverted `promote.go` to the original `suggested, _ := ...` line,
reran the new test — failed identically to RED — then restored the fix exactly, confirmed via
`git diff` showing only the intended 7-line hunk. `hk check` clean.

#### ✦ `[x]` T314: converge `internal/cmd` naming and exit-code-assertion conventions

Two small inconsistencies, both noted more than once across the Phase 53/54 reviews: (1) file-naming
— `internal/cmd/versionoverride.go` (no underscore) versus its test file
`internal/cmd/version_override_test.go` (underscored); the package uses both spellings elsewhere
(`skiphooks.go` vs `version_sprint.go`), so neither is wrong, but pick one pair and rename to match.
(2) test assertions — some `internal/cmd` tests assert exit codes via `cmd.ExitCode(err)`
(`exit.go`'s own exported wrapper, the package majority), others via `exitcode.Resolve(err)` directly
(`version_override_test.go`, pre-existing before that file even existed); the two are identical
(`ExitCode` is `return exitcode.Resolve(err)`) but the split persists. Pick `cmd.ExitCode` (the
package's own, more-used wrapper) and update the minority. Mechanical; no behaviour change; run the
full `internal/cmd` suite after.

**Completion note (2026-09-22).** Both items landed exactly as scoped: `internal/cmd/versionoverride.go`
was renamed to `internal/cmd/version_override.go` via `git mv` (tracked as a rename, no content change),
and the four `exitcode.Resolve(err)` call sites in `version_override_test.go` (lines ~128, ~141, ~181,
~212) were switched to `cmd.ExitCode(err)` to match the rest of the package; a repo-wide grep for
`exitcode.Resolve` in `internal/cmd` confirmed no other file needed the swap (the only other hit is
`exit.go`'s own `ExitCode` definition, which is the wrapper being converged *to*, not a call site).
Both changes are behaviourally identical to what they replaced — `go build ./...` and `go test ./...`
pass unchanged (2039 tests across 26 packages).

#### ✦ `[x]` T315: reword the `{build}`-without-a-build-ID error for the case where `--set-version` was already given

`internal/versioning/tagfmt/tagfmt.go`'s `ErrBuildIDRequired` (~line 20) always says "pass
`--set-version <version> --set-build-id <id>` to …", even when the user already passed
`--set-version` and only forgot `--set-build-id` — the common case, since the auto path never reaches
`tagfmt.Render` with a version to spare in the first place unless `bump.mode: manual` or per-env
`bump: auto` supplied one already. `tagfmt` is a pure leaf package with no knowledge of which flags
were actually given, so it cannot phrase this conditionally from inside `Render`. Two directions to
choose between before implementing: (a) leave `tagfmt`'s message generic (today's text) and instead
have EACH of the three `wrapRunErr` call sites (`internal/cmd/{release,changelog,version}.go`) append
a targeted hint only when they know `versionOverride != ""` and `buildID == ""` ("… you already passed
--set-version; add --set-build-id <id>"); or (b) reword the sentinel's own text to read naturally
either way, e.g. "pass `--set-build-id <id>` (with `--set-version <version>` if not already given) to
…". (a) is more precise but adds per-command logic outside `tagfmt`; (b) is a one-line, zero-risk
change but slightly less specific. Existing `tagfmt`/`cmd` tests assert the current text — whichever
direction is chosen, update them, never delete a row.

**Completion note (2026-09-22).** The maintainer chose direction (b): reword `ErrBuildIDRequired`'s
own text rather than adding per-command targeted hints, keeping `internal/cmd/{release,changelog,
version}.go` untouched. New wording: "tag format template contains {build} but no build ID was
provided; pass --set-build-id <id> (with --set-version <version> if not already given) to `heraut
changelog`, `heraut release` or `heraut version next`" — it reads naturally whether `--set-version`
was already supplied or not. TDD: added `TestRender_ErrBuildIDRequired_ExactMessage`
(`internal/versioning/tagfmt/tagfmt_test.go`), pinning the exact string — no test pinned the literal
message before. RED: run against the old wording, failed on a string mismatch as expected. GREEN:
same test after the fix, passes. Confirmed `TestRender_BuildRequiredButEmpty` needed no change — it
only asserts `Contains` on six substrings (`{build}`, `--set-build-id`, `--set-version`, `heraut
changelog`, `heraut release`, `heraut version next`), all still present in the new wording, and it
passed unchanged. Full `tagfmt` package (93 tests) and full suite (2040 tests) green; `hk check`
clean. Grep for the old literal phrase (`pass --set-version <version> --set-build-id <id>`) across
`*.go`/`*.md` found no stray quotes elsewhere — the only other match would have been this task's own
description above, but that phrase is markdown-backtick-wrapped there (split across two lines) so
the literal grep didn't even hit it; left untouched regardless, per precedent for historical task
text. A broader grep for `--set-version`/`--set-build-id` co-occurrence turned up unrelated hits
(`internal/cmd/version_override.go`'s distinct `--set-build-id requires --set-version` flag-validation
error, and its mentions across specs/tests/CHANGELOG) — none quote `ErrBuildIDRequired`'s text, so
none needed updating.

#### ✦ `[x]` T316: `internal/app.TestIsBuildIDRequired` should include a case built from a real `tagfmt.Render` call

Found during T312's final review: the mutation "make `tagfmt.Render` return a fresh, same-text
`fmt.Errorf` instead of the `ErrBuildIDRequired` sentinel" is caught by the `tagfmt` and `internal/cmd`
tests but NOT by `internal/app/errors_test.go`'s `TestIsBuildIDRequired`, because that table is built
entirely from hand-constructed errors (`tagfmt.ErrBuildIDRequired`, wrapped copies, unrelated errors)
and never calls `tagfmt.Render` itself. Add one row that calls `tagfmt.Render` with a `{build}`-only
template and asserts `app.IsBuildIDRequired` on its returned error, closing the gap at the layer where
the helper's actual contract (`errors.Is` against what `Render` really returns) is meant to hold.

**Completion note (2026-09-22).** Added a `renderErr` setup (a real `tagfmt.Render("{env}/{version}-{build}",
…)` call with no build ID, asserted non-nil via `require.Error`) above `TestIsBuildIDRequired`'s table,
plus one new row, `"error from a real tagfmt.Render call, not a hand-built sentinel"`, asserting
`app.IsBuildIDRequired(renderErr)` is `true`; `require` was added to the file's import block alongside
the existing `assert` import. Mutation-verified the gap: temporarily changed `tagfmt.Render`'s
`{build}`-without-build-ID branch (`internal/versioning/tagfmt/tagfmt.go`) to return
`fmt.Errorf("%s", ErrBuildIDRequired.Error())` — a fresh, same-text error, not the sentinel — and reran
`TestIsBuildIDRequired`: only the new row failed (`expected: true, actual: false`); all five existing
rows (`nil`, `sentinel`, `sentinel wrapped twice`, `unrelated`, `promotion guard`) still passed,
confirming those hand-built rows would have stayed green even if `Render` stopped returning the real
sentinel, and that only the new row catches it. Restored `tagfmt.go` exactly (`git diff` empty) before
committing — no production code changed. `go test ./internal/app/` and the full `go test ./...` suite
are green; `hk check` clean. No existing row was touched, deleted, or loosened. This was the last open
Phase 55 task, so the phase is now Done; the "Open items" sentence's parenthetical no longer needed to
carve out Phase 55 (it now has zero unchecked items), so it was trimmed to reference only Phase 53/T304.

---

### Phase 56 — Sign the raw binaries with a packslip manifest

#### ✦ `[x]` T319: publish a Sigstore-signed packslip manifest alongside each release

Triggered by an unrelated support conversation (diagnosing the 0.63.0 Homebrew cask URL
regression) that surfaced [jdx/packslip](https://github.com/jdx/packslip) — a signed release
manifest format by mise's author, with mise named as a consumer. Goal: publish
`packslip.sigstore.json` alongside heraut's own GitHub Release so a packslip-aware installer can
verify heraut's raw binaries without trusting the download channel alone.

Files: `.github/workflows/release.yml` only — no Go code, no config schema change.

**Completion note (2026-09-25).** Added two steps to the `release` job, after `Release` (which is
where the real tag/commit/binaries all exist) and before `Publish Homebrew cask`: `Capture release
commit` (`git rev-parse HEAD`, since `heraut release` commits the changelog and pushes the tag,
moving HEAD past `github.sha` — the packslip action's own docs name exactly this
manually-dispatched-workflow case as the reason to pass `commit` explicitly), then `Publish
packslip` (`jdx/packslip@9c1d4ffedc48b129fdd851c47fdd0945d5a9c8fc # v1.3.0`) with `artifacts` set to
the same five raw-binary glob patterns `.config/heraut.yml`'s `release.assets` already uses (not
`checksums.txt`, not the Pkl builtin `heraut@*` package — out of scope), `bin: heraut`, and `tag:
${{ env.VERSION }}` (`github.ref_type` is never `tag` on a `workflow_dispatch` run, so the action
cannot infer it the way a tag-triggered job would; `env.VERSION` is already correctly prefixed
thanks to the same workflow's "Normalize version override" step, added earlier this session
(untracked `ci:` commit `6302dfd`, not a roadmap task — see its own commit message for the
0.63.0 cask-URL regression it fixes). Left `attest` at its
default (`true`) rather than `link`: `link` would reuse heraut's existing `actions/attest`
`subject-checksums` step to avoid a second attestation, but that relies on an unverified assumption
that a `subject-checksums`-based attestation is queryable by the same per-file digest URL a
`subject-path`-based one is; `attest: true` is packslip's own well-tested default and degrades
safely (an unresolvable link "leaves a URL that resolves to nothing" per its docs — not worth
risking for one redundant attestation).

Verified the actual filename-inference behavior empirically rather than trusting docs paraphrase:
downloaded the real `packslip` CLI (v1.3.0, darwin-arm64) locally, generated synthetic zero-byte
files named exactly like heraut's real artifacts (`heraut_0.69.0_linux_amd64`,
`..._darwin_arm64`, `..._windows_amd64.exe`), and ran `packslip create --bin heraut` against them
with a local test key. Confirmed via `packslip show`: `os`/`arch`/`format` are all inferred
correctly from goreleaser-style filenames with no overrides needed (`linux`/`darwin`/`windows`,
`amd64`→`x86_64`/`arm64`→`aarch64`, and `raw` for every artifact including the extensionless `.exe`
— not confused with the Windows `exe`-installer format), and `bin: heraut` resolves correctly to
`{name: "heraut", path: <filename>}` for a bare executable.

One known imprecision, deliberately accepted rather than worked around: packslip always tags a
`linux` artifact `libc: "gnu"` — inferred by default, and not overridable to "absent" via any CLI
flag, override syntax, or manifest field tested (`os/arch/libc` triples, and a TOML manifest
omitting `libc` entirely, both still produced `"gnu"`). heraut's Linux binaries are
`CGO_ENABLED=0` and have no libc dependency at all — they run identically on `gnu` and `musl`
systems — so this claim is technically inaccurate. Impact is narrow (a strict consumer selecting
only `musl`-tagged artifacts could skip a binary that would have worked) and there is no clean fix
available in packslip today; revisit if a future packslip release adds a way to mark an artifact
libc-agnostic.

Verification: `hk check .github/workflows/release.yml` (actionlint, yamlfmt, typos) green. Not yet
verified against a real GitHub Actions run (packslip's `verify`/upload steps, the `commit` capture,
and the `attest: true` provenance linking all need one) — the next real `heraut release` dispatch is
the first live test. Deferred, not built: documenting mise as a verified-install path in this
project's own docs (README/`docs/guides/`) — mentioned by the user as the reason to add this, but
explicitly scoped as a follow-up ("add it to heraut first").

#### ✦ `[x]` T320: fix the immutable-release 422 T319's `gh release upload` hit on its first real run

T319's first real exercise — a manually-dispatched `v0.70.0` release — failed at the `Publish
packslip` step: `HTTP 422: Cannot upload assets to an immutable release`, from that step's own
internal `gh release upload "$TAG" "$BUNDLE" --repo "$REPO"` call (packslip's action default,
`upload: true`). Root cause was already documented, just not connected to this new step at design
time: `internal/platforms/github/platform.go`'s `CreateRelease` doc comment names this exact
GitHub behavior — a *separate* `gh release upload` after a release is published 422s once "Enable
release immutability" is on, which is why heraut bundles every `release.assets` glob into the same
atomic `gh release create` call instead (`LenientAssets`, set automatically whenever a target has
assets — `internal/config/config.go`, `internal/app/pipeline.go`). T319 placed `Publish packslip`
*after* the `Release` step specifically to capture the real post-changelog-commit HEAD for
packslip's `commit` input — directly incompatible with that atomic-upload design, since by the
time packslip's own upload ran, `heraut release` had already published (and thus locked) the
release.

**Completion note (2026-09-25).** Moved `Publish packslip` to run *before* `heraut release`,
immediately after the existing `Attest build provenance` step (both are supply-chain steps that
only need the built binaries, not a published release). Two input changes make this placement
correct instead of just early: `out: dist` writes the bundle to `dist/packslip.sigstore.json`
instead of packslip's default `packslip/` directory, and `upload: false` stops the action from
making its own `gh release upload` call at all. `.config/heraut.yml`'s `release.assets` gained
`"dist/packslip.sigstore.json"`, so `heraut release`'s own atomic upload carries the bundle exactly
like every other release asset — no separate upload, no 422. Dropped the `commit` input and the
now-unneeded `Capture release commit` step entirely rather than finding a way to compute the
post-release HEAD earlier (impossible — that commit doesn't exist until `heraut release` creates
it): the action's own default, `github.sha`, turned out to be the more defensible value anyway,
since it's the commit these binaries were actually compiled from, not the later docs-only
changelog commit the tag happens to point to. `tag: ${{ env.VERSION }}` is unchanged (still needed;
`github.ref_type` is still never `tag` on this `workflow_dispatch` job). `attest: true` and the
`artifacts`/`bin` inputs are unchanged from T319. Verification: `hk check
.github/workflows/release.yml .config/heraut.yml` (actionlint, yamlfmt, typos) green. Still not
verified against a real run — the next dispatch is the first live test of the corrected ordering.

---

### Phase 57 — SBOM generation; shell completions investigated, found not viable yet

Triggered by reviewing [charmbracelet/meta's goreleaser-vhs.yaml](https://github.com/charmbracelet/meta/blob/main/goreleaser-vhs.yaml)
as a reference for release-pipeline ideas. Two candidates: shell completions bundled into the
Homebrew cask, and SBOM generation. User approved both; asked for completions first (its own
commit), then SBOM (its own commit).

#### ✦ `[x]` T321: shell completions for the Homebrew cask — investigated, reverted, not shipped

heraut already ships `heraut completion {bash,zsh,fish,powershell}` and `heraut man` for free
(forge/cli wrapping cobra + fang — no heraut code). The question was purely how to get a Homebrew
cask install to wire them up. Tried both of GoReleaser's `homebrew_casks` mechanisms, verified each
against a real local install via a scratch `local/heraut-test` tap (not just docs), and both are
broken for heraut as currently built:

- `generate_completions_from_executable` (dynamic — Homebrew runs the *installed* binary at
  install time to generate the scripts): installs, but the completion-generation step itself hangs
  indefinitely and gets killed by Homebrew's own timeout. Root cause, confirmed directly: heraut's
  binaries are not Apple Developer ID-signed or notarized (already a known gap — see the README's
  existing Gatekeeper caveat on the "Prebuilt binary" install path), and an unsigned + quarantined
  binary hangs under Gatekeeper when executed non-interactively. Ruled out "any signature helps":
  ad-hoc `codesign -s -` on the same quarantined binary reproduced the identical hang — it's
  specifically the lack of a real notarized identity, not the absence of *a* signature.
  `HERAUT_CHECK_UPDATE=false` was also ruled out as the cause (still hung).
  - Also caught a real config bug in the same investigation, before it got anywhere near
    Homebrew: `shell_parameter_format: cobra` already constructs `completion <shell>` itself, so
    an explicit `args: [completion]` alongside it doubled up into `heraut completion completion
    bash` — harmless in this instance since the quarantine hang came first, but would have been
    its own bug had signing not been the blocker.
- `completions:` (static — reference pre-generated files, e.g. via a `before.hooks` step like
  vhs's own `go run . completion bash >...`): fails outright — `Error: ... the symlink source
  '.../completions/heraut.bash' is not there`. vhs's config works because it ships full archives
  (`archives.files: [completions/*, ...]`) that bundle the generated scripts alongside the binary
  in the same download. heraut ships a bare, unarchived binary per ADR-0013 (`archives.formats:
  binary`, deliberately, so the Homebrew cask installs the plain binary under `heraut`) — there is
  no archive for `completions:`'s referenced path to live inside.

Both paths are blocked by real constraints already on record (ADR-0013's raw-binary decision;
notarization named as a known future gap), not by anything fixable in this task's scope. Reverted
`.goreleaser.yml` to its pre-investigation state — nothing shipped. Two ways forward, neither
attempted here: (a) get heraut's binaries Developer ID-signed and notarized (real infra: Apple
Developer Program, a codesigning cert, a notarization step in CI) unlocks
`generate_completions_from_executable` cleanly with zero ADR-0013 impact; (b) reverse ADR-0013 to
ship real archives instead of bare binaries unlocks static `completions:` (and `manpages:`), but
that reopens a settled architectural decision and needs its own discussion, not a side effect of
adding completions. Recommended next step if this gets picked back up: (a), since it also unblocks
the still-open man-page ANSI-escape bug from the same conversation (separate task, forge/cli) and
is the smaller, more targeted change.

#### ✦ `[x]` T322: SBOM generation for the raw binaries (`syft`, SPDX)

**Completion note (2026-09-25).** Added `sboms: - id: binaries / artifacts: binary` to
`.goreleaser.yml` — GoReleaser's default `cmd: syft` and default SPDX JSON output needed no
overrides. `artifacts: binary` (not the `archive` default) matches ADR-0013's bare-binary shipping,
confirmed directly: a local snapshot build (`goreleaser release --snapshot --clean
--skip=publish,validate,announce`, syft 1.52.0 on `PATH`) produced one real `.sbom.json` per
platform (5 total), each listing heraut's actual Go module dependencies (51 packages, e.g.
`charm.land/bubbles/v2`) — not an empty or templated stub. One filename quirk worth recording:
GoReleaser's default `documents:` template uses `{{ .Binary }}`, which is `heraut.exe` for the
Windows build, so that one file is named `heraut.exe_<version>_windows_amd64.sbom.json` (prefix
differs from the other four's `heraut_<version>_<os>_<arch>.sbom.json`) — `.config/heraut.yml`'s
new `release.assets` entry is the broad `"dist/*.sbom.json"` specifically to catch this without a
second, Windows-specific glob. `syft` added to `.config/mise/config.toml` (`"latest"`, same tier as
`hadolint`/`pkl`/`tombi`) and `.config/mise/mise.lock` regenerated via `mise install syft` — no
`.github/workflows/release.yml` change needed, since `syft` reaches `PATH` the same way
`goreleaser`/`hadolint` already do (via the `Release setup` step's `jdx/mise-action`, which runs
before `Build binaries`). Verification: `hk check .goreleaser.yml .config/heraut.yml
.config/mise/config.toml .config/mise/mise.lock` (yamlfmt, mise fmt, tombi_format, typos) green.
Not yet verified against a real CI run.

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
