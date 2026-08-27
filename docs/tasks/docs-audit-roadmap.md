# Héraut — Documentation Audit Roadmap

> Status: Active
> Source: four parallel Opus audit passes (2026-08-26) — specs vs. code, ADRs vs. code, and
> `CLAUDE.md`/`.claude/rules/`/`README.md`/`schema.json`/`heraut.sample.yml` vs. code.
> Main roadmap: tracked as Phase 27 in [`roadmap.md`](roadmap.md)

The audit found **142 mismatches** between documentation and current code. Most are pure
documentation drift — the codebase moved on (native-only generator, `forges:`/`release.targets:`,
release-block atomicity, `internal/forge/` HTTP clients) and the docs didn't fully follow. **Seven
are real code bugs** the audit surfaced incidentally, independent of any doc claim being wrong —
those are broken out first and kept separate from doc-only work so they can be prioritized,
reviewed, and tested on their own merits.

## Conventions

- Task IDs **continue the global sequence** (`T222+`) so they never collide with the main roadmap
  or the other dedicated roadmaps (`forge-abstraction-roadmap.md`, `native-generator-roadmap.md`,
  `release-config-roadmap.md`).
- This file is the **single source of truth** for task status. Same checkbox markers as the other
  roadmaps: `[ ]` not started, `[x]` done. Follow the two-step flow
  ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD for code tasks — failing test
  first; doc tasks are verified by re-reading the corresponding code, not by a test), then flip
  `[ ]` → `[x]` and add a one-paragraph completion note.
- **No real data** anywhere (samples, docs, tests): synthetic placeholders only.
- Code-bug tasks (T222–T228) and doc-reconciliation tasks (T229–T238) are independent of each
  other unless a task's own **Dependencies** line says otherwise — a doc task that describes a
  behavior a bug task is about to change should land *after* that bug task, so it documents the
  fixed behavior rather than the bug.
- The main `roadmap.md` Phase 27 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Task  | Description                                                                          | Status      |
|-------|---------------------------------------------------------------------------------------|-------------|
| T222  | `--version`/`--build` override ignores `tag_prefix` / per-env `tag_format`            | Done        |
| T223  | Per-environment `release:` never gets `Notes` default-populated (ADR-0046 gap)        | Done        |
| T224  | Per-driver `rendering.excludes` is never consumed                                     | Not started |
| T225  | Root `versioning.bump` enum and `sprint:` requiredness are unvalidated                | Done        |
| T226  | Environment promotion doesn't filter pre-release source tags like auto-resolve does   | Done        |
| T227  | `heraut init --defaults` overwrites an existing config with no confirmation           | Done        |
| T228  | Asset-glob failure semantics for `release.targets[].assets` (direction TBD)           | Done        |
| T229  | `docs/specs/01-overview.md` reconciliation                                            | Not started |
| T230  | `docs/specs/02-configuration.md` reconciliation                                       | Not started |
| T231  | `docs/specs/03-commands.md` reconciliation                                            | Not started |
| T232  | `docs/specs/04-versioning.md` reconciliation                                          | Not started |
| T233  | `docs/specs/05-generators-and-platforms.md` reconciliation                            | Not started |
| T234  | `docs/specs/06-dx-and-testing.md` reconciliation                                      | Not started |
| T235  | `schema.json` + `docs/heraut.sample.yml` cleanup                                      | Not started |
| T236  | `CLAUDE.md` + `README.md` reconciliation                                              | Not started |
| T237  | `.claude/rules/{coding,testing,workflow}.md` reconciliation                           | Not started |
| T238  | `docs/adr/README.md` + affected ADR bodies — status annotations and stale references  | Not started |

No fixed order within each group. Suggested sequencing: bug tasks first (T222–T228, any order,
disjoint code areas), then doc tasks (T229–T238) so they describe the post-fix behavior rather
than needing a second pass.

---

## Code bugs

#### `[x]` T222: `--version`/`--build` override ignores `tag_prefix` / per-env `tag_format`

**Found while auditing `docs/specs/04-versioning.md` and `03-commands.md`'s claims about manual
version overrides.** `internal/app/resolver.go:27-43` short-circuits any non-empty
`versionOverride` straight to `versioning.NewStaticResolver(tag, version)` with `tag :=
versionOverride` used **verbatim** — `semver.Resolver.SetVersionOverride`'s prefix-normalization
logic (`internal/versioning/semver/resolver.go:68-72`) is never called in production (grep confirms
it has no call site outside its own definition). Consequence: `heraut release --version 1.2.3`
under `tag_prefix: "v"` creates tag `1.2.3`, not `v1.2.3`; under a per-env `tag_format` it creates
`1.2.3`, not e.g. `prod/1.2.3`. `--build` is the one path that *does* render correctly, because it
routes through `tagfmt.Render` (`internal/versioning/tagfmt/tagfmt.go`) instead.

**Direction:** `NewStaticResolver`'s call site (or the resolver itself) needs to render the override
through the same `tagfmt`/prefix logic used elsewhere, so `--version` produces a tag consistent
with the active strategy's configured format — not a bypass of it. Decide whether this belongs in
`internal/app/resolver.go` (call `tagfmt.Render` before constructing the static resolver) or inside
`versioning.NewStaticResolver` itself.

**Files (expected):** `internal/app/resolver.go`, `internal/versioning/static.go` (or equivalent),
`internal/versioning/tagfmt/tagfmt.go` (if reused). **Scope:** S–M. **Dependencies:** none.

Implemented in `internal/app/resolver.go` only — no change needed to `static.go` or `tagfmt.go`.
`NewResolver` now computes the effective tag format once (`cfg.EffectiveTagFormat(env)` when no
`--build`, or the existing `effectiveTagFmt` validation when `--build` is set) and branches: when a
`tag_format` is in effect, the override renders through `tagfmt.Render` exactly like the `--build`
path already did, so a per-env `{version}`/`{env}` template now applies to plain `--version` too.
When no `tag_format` applies, a new unexported `defaultTagPrefix(strategy)` supplies the correct
strategy-specific default ("v" for SemVer-based strategies, "" for CalVer-based ones — mirroring
`semver.Resolver.prefix()` and `calver.Resolver.prefix()`, which stay untouched and are not called
from this path) so an explicit `versioning.tag_prefix` is honoured, and the *actual* configured
prefix is stripped from a full-tag override instead of a hardcoded `"v"` — fixing double-prefixing
and non-stripping for non-default prefixes. `TestNewResolver_VersionOverride_SemverPerEnv` asserted
the pre-fix (buggy) behavior directly; updated to the corrected expectation and its `TagFormat`
fixture corrected from a non-existent `${version}` token to the real `{version}` token, per this
project's own testing rule that a changed assertion needs its behavior change stated, not just
deleted — this is a plain bugfix, no ADR needed (same precedent as T221). Added
`TestNewResolver_VersionOverride_CalverPerEnv`, `_CustomTagPrefix_BareVersion`,
`_CustomTagPrefix_FullTag`, and `_EmptyTagPrefix` to cover the strategy-default and explicit-prefix
cases the bug report called out. `go test ./...` and `hk check` both clean.

---

#### `[x]` T223: per-environment `release:` never gets `Notes` default-populated (ADR-0046 gap)

**Found while auditing `docs/specs/02-configuration.md`'s claim that ADR-0046 atomicity applies
"root or per-environment."** `internal/config/loader.go:186-188`'s `normalize()` default-populates
`Release.Notes = &ContentDriver{}` only for the **root** `Release` block. `EnvRelease.Notes` is
never populated by the equivalent per-env path (`internal/app/pipeline.go:196-221`). Consequence: a
config with no root `release:` but `environments.prod.release: {targets: […]}` reaches
`config.EffectiveTargets` with `effectiveNotes == nil`, and the platform driver creates a release
with an **empty body** — exactly the notes/publish split ADR-0046 was written to eliminate, just
one level down.

**Direction:** Mirror T216's root-level fix (`internal/config/loader.go`'s `normalize()`) for the
per-environment case — whichever code path resolves an environment's effective `Release` block
needs the same `Notes = &ContentDriver{}` default whenever that environment's `Release` is
non-nil. Add a contract test asserting a per-env-only `release:` config produces a non-nil,
non-empty effective notes driver.

**Files (expected):** `internal/config/loader.go` or `internal/config/merge.go` (wherever per-env
`Release` resolution lives), `internal/app/pipeline.go` (if the defaulting belongs there instead).
**Scope:** S. **Dependencies:** none.

The defaulting and the root/per-env merge were already duplicated in two places:
`config.EffectiveReleaseNotes` (`internal/config/platforms.go`, unused in production — a leftover
from before `internal/app/pipeline.go`'s `buildReleasePipelineConfig` grew its own inline copy) and
that inline copy itself, both carrying the identical gap. Fixed the shared helper once — a
`released` bool now tracks whether `release:` is present at either level and default-populates
`Notes` to `&ContentDriver{}` only when neither level set one explicitly — then rewired
`buildReleasePipelineConfig` to call it instead of re-deriving the merge inline, removing the
duplicate (buggy) copy entirely rather than fixing it twice. Added a `TestEffectiveReleaseNotes`
table row for the env-only case, plus `TestBuildReleasePipelineConfig_EnvOnlyReleaseGetsNotes`
exercising the actual production call site. `go test ./...` and `hk check` both clean.

---

#### `[ ]` T224: per-driver `rendering.excludes` is never consumed

**Found while auditing `docs/specs/02-configuration.md`, `schema.json:333-336`, and
`heraut.sample.yml:270-274`**, all three of which document a per-`ContentDriver` `rendering.excludes`
that layers on top of the global `rendering.excludes`. `internal/app/pipeline.go:421,445-447` reads
only the **global** `cfg.Rendering.Excludes` — `ContentDriver.Rendering.Excludes` has no read site
anywhere in `internal/`. The validator doesn't flag it either, so a user setting it gets silent
no-op behavior with no error and no effect.

**Direction:** Either wire the per-driver value into the exclude-pattern resolution actually used
by `changelog`/`release.notes` generation (additive with the global list, matching how
`rendering.templates` already merges per-driver), or — if per-driver excludes were never meant to
be a real axis — remove the field from `config.go`/`schema.json`/the sample and document that
excludes are global-only. Needs a direction decision at implementation time; the audit only
confirms the field is currently dead, not which resolution is intended.

**Files (expected):** `internal/app/pipeline.go`, `internal/config/config.go`,
`internal/config/commits.go` (`EffectiveExcludes`), `schema.json`, `docs/heraut.sample.yml`.
**Scope:** S–M. **Dependencies:** none.

---

#### `[x]` T225: root `versioning.bump` enum and `sprint:` requiredness are unvalidated

**Found while auditing `docs/specs/02-configuration.md`'s field-by-field validation claims.** Two
related validation gaps in `internal/config/validator.go`:

- `versioning.bump`'s enum is declared in `schema.json:264-271` but `validateEnums`
  (`validator.go:441-469`) checks only `strategy` and `tag_type` at the root level — `bump:
  nonsense` passes `heraut check config` and is silently treated as `auto` by the resolver. The
  per-environment `bump` field *is* validated (`validator.go:628-634`); the root field's validation
  was apparently never added to match.
- `sprint:` is documented as "required when `format` contains the `SPRINT` token"
  (`docs/specs/02-configuration.md:79`) but nothing enforces it — no check in `validator.go`, and
  `schema.json:276-280` has only `minimum: 1`, no conditional `required`. `format:
  "YYYY.SPRINT.PATCH"` with no `sprint:` set passes `heraut check config` and silently renders
  `2026.0.0`.

**Direction:** Add the missing root-level `bump` enum check (mirror the per-env one). Add a
semantic validation rule: if `format` contains the `SPRINT` token, `sprint` must be set and `>= 1`
— surfaced as a `ValidationErrors` entry with `Path`/`Message`/`Hint`, not a schema-only constraint
(JSON Schema can't express "required if another field contains a substring").

**Files (expected):** `internal/config/validator.go` (+ tests), `docs/specs/02-configuration.md` if
the requiredness wording needs tightening after the fix. **Scope:** S. **Dependencies:** none.

Added a `validBumpModes` enum map (`{"auto", "manual"}`) checked in `validateEnums` alongside the
existing `strategy`/`tag_type` checks — same style, root-level, strategy-agnostic (mirrors how
those two are already checked without gating on which strategy is active). Added the `sprint`
semantic check inside `validateStrategySpecific`'s existing `calver`/`calver-per-env` branch,
right after the format-required check it already has: `format` containing the literal `SPRINT`
substring with `Sprint < 1` (covers both "field omitted" and "explicitly set to a non-positive
value," which `schema.json`'s own `minimum: 1` already rejects) now produces a
`versioning.sprint` error. Left `schema.json` untouched — a cross-field conditional requirement
("required only if another field's *value* contains a substring") isn't expressible as a JSON
Schema constraint, so this had to be a semantic-validator check, not a schema one; the spec
doc's existing "required when format contains the SPRINT token" wording (`02-configuration.md:79`)
was already correct, just previously unenforced. `go test ./...` and `hk check` both clean.

---

#### `[x]` T226: environment promotion doesn't filter pre-release source tags like auto-resolve does

**Found while auditing `docs/specs/04-versioning.md`'s bump-mode and pre-release sections.**
`internal/versioning/perenv/auto.go:37`'s `resolveAuto` filters candidate tags with
`semver.IsBareVersion` before picking the latest. `resolvePromote`
(`internal/versioning/perenv/promote.go:157`) takes `srcTags[0]` **raw**, with no equivalent filter.
Consequence: if a pre-release tag (e.g. `dev/1.3.0-rc.1`) sorts first in the source environment's
tag list, `bump: promote` will promote it to the destination environment — a tag `auto` mode would
never have selected as a release candidate in the first place.

**Direction:** Apply the same `semver.IsBareVersion` filter (or equivalent) to the source-tag
selection in `resolvePromote`, so promotion and auto-resolution agree on what counts as a
promotable/releasable tag. Add a table-driven test case: source env has both a pre-release and a
bare tag: `resolvePromote` must select the bare one.

**Files (expected):** `internal/versioning/perenv/promote.go` (+ tests).
**Scope:** S. **Dependencies:** none.

`resolvePromote`'s tag-selection loop now mirrors `resolveAuto`'s exactly: iterate the
`--sort=-version:refname` source-tag list, skip any tag whose `{version}` doesn't parse or isn't
`semver.IsBareVersion`, and take the first survivor. `ErrNoSourceTags`/E003 is now also returned
when tags exist but none are bare releases (previously only the "zero tags at all" case hit it) —
reusing the existing sentinel rather than adding a new one, since from promotion's perspective
there is equally no usable source tag either way, and E003's message/remediation ("create a source
release first") already reads correctly for both. Added
`TestResolve_Promote_SkipsPrereleaseSourceTag`, mirroring the existing
`TestResolve_Auto_Semver_SkipsPrereleaseTag` fixture shape. `go test ./...` and `hk check` both
clean.

---

#### `[x]` T227: `heraut init --defaults` overwrites an existing config with no confirmation

**Found while auditing `docs/specs/03-commands.md`'s init-command flag table.**
`internal/cmd/init.go:46` — the interactive overwrite prompt is gated on `if fileExists &&
!defaults`. With `--defaults` passed, the write at `init.go:109` proceeds unconditionally: an
existing `.heraut.yml` is silently replaced, with no prompt and without requiring the global
`--force` flag. This is inconsistent with the rest of the CLI's irreversible-action posture (e.g.
promotion guards require `--force` per ADR-0007).

**Direction:** Decide the intended UX — either `--defaults` should still refuse to overwrite an
existing config without `--force` (consistent with other destructive-by-default guards), or the
behavior is intentional for non-interactive/CI use of `--defaults` and only needs documenting.
Given `--defaults` is explicitly the non-interactive path (CI-friendly scaffolding), lean toward
documenting-as-intentional unless the user says otherwise — but confirm before implementing either
way, since this is a behavior change either direction.

**Files (expected):** `internal/cmd/init.go` (+ tests), `docs/specs/03-commands.md`.
**Scope:** S. **Dependencies:** none.

Decided (user confirmed): require `--force` to overwrite, matching the rest of the CLI's
destructive-action posture. `heraut init --defaults` on an existing config now errors immediately
("`<path>` already exists (use --force to overwrite with --defaults)") and leaves the file
untouched, before any prompt/generation logic runs; `--force` bypasses it exactly like the
interactive path already did. `TestInitCmd_DefaultsWithExistingNoForceOverwrites` pinned the old
(buggy) silent-overwrite behavior directly — renamed to
`TestInitCmd_DefaultsWithExistingNoForceErrors` and rewritten to assert the new behavior, per this
project's testing rule that a changed assertion needs its behavior change stated, not deleted; a
plain bugfix per explicit user decision, no ADR needed. Left `docs/specs/03-commands.md` for T231,
which already scoped this exact bullet as dependent on T227 landing first. `go test ./...` and
`hk check` both clean.

---

#### `[x]` T228: asset-glob failure semantics for `release.targets[].assets` (direction TBD)

**Found while auditing `docs/specs/02-configuration.md:403`, `schema.json:432-438`, and
`heraut.sample.yml:294-299`**, all of which describe asset-glob failures as a warn-and-skip
(lenient) behavior. `internal/app/pipeline.go:287,302` sets `LenientAssets = true` only for globs
sourced from the top-level `release.assets`; `release.targets[*].assets` goes through
`ResolveGlobs` (`internal/platforms/globs.go:11-25`), which hard-errors with `no files matched
asset pattern %q` on any non-match. Because target resolution happens **after** the tag has already
been created and pushed (`internal/pipeline/release.go`'s step order), a target-level asset-glob
typo aborts the release in a partially-completed state: tag exists, no GitHub/GitLab release was
created.

**Direction (exact shape TBD at implementation time):**
- Option A: make target-level assets lenient too, matching top-level — simplest, but silently
  drops assets a user explicitly scoped to one target, which may be worse than failing loudly.
- Option B: keep target-level assets strict, but move glob resolution earlier in the pipeline (before
  tagging) so a bad pattern fails at preflight instead of after the tag is live.
- Whichever direction is chosen, update the docs (`02-configuration.md`, `schema.json`,
  `heraut.sample.yml`) to state the real behavior precisely, since all three currently describe
  only the lenient case as if it were universal.

**Files (expected):** `internal/app/pipeline.go`, `internal/pipeline/release.go`,
`internal/platforms/globs.go`, `docs/specs/02-configuration.md`, `schema.json`,
`docs/heraut.sample.yml`. **Scope:** S–M. **Dependencies:** none.

Decided (user confirmed): Option A — target-level assets are now lenient too, matching top-level
`release.assets`. Implemented entirely in `internal/app/pipeline.go`'s `buildTargetPlatforms`:
`platCfg.LenientAssets` is now set to `len(platCfg.Assets) > 0` unconditionally, after the existing
inherit-from-`releaseAssets` step, rather than only inside that step's own branch — so it covers
assets from either source uniformly. No change needed in `internal/pipeline/release.go` or
`internal/platforms/globs.go`; both already branch on `LenientAssets` correctly; only the wiring
decision of when to set it was wrong. Updated `config.Platform.LenientAssets`'s doc comment (it
previously claimed to be top-level-origin-only). Added
`TestBuildTargetPlatforms_TargetOwnAssetsAreLenient`, verified red without the fix (temporarily
reverted, confirmed it fails with `no files matched asset pattern`) and green with it. Left the
doc-side corrections (`02-configuration.md`, `schema.json`, `heraut.sample.yml` all currently
describe only the lenient case as if universal, which is now actually true) for T230/T233, which
already scoped those exact bullets as dependent on this landing. `go test ./...` and `hk check`
both clean.

---

## Documentation reconciliation

#### `[ ]` T229: `docs/specs/01-overview.md` reconciliation

- **Architecture diagram / hexagonal prose (lines ~22-52)**: describes `internal/adapter/exec/` as
  the Runner adapter — that package doesn't exist; the concrete runner (`CmdRunner`, `DryRun` flag)
  lives in `github.com/adaouat/forge/exec`, aliased at `internal/port/runner.go:7`. Same section
  never mentions `internal/forge/{github,gitlab,azure}` (direct `net/http` PR/MR-enrichment
  clients, ADR-0043) — a whole layer missing from the tool's own architecture picture.
- **§ Key concepts, dry-run (line ~80)**: states dry-run performs "no git operations." Contradicts
  `06-dx-and-testing.md`'s own documented exception (version resolution runs on a real runner during
  dry-run so the printed next-version is accurate) — spec 01 is the side that's wrong; align it
  with spec 06's description.
- **§ Boundaries (line ~100)**: "Two platforms: GitLab, GitHub" — `internal/config/validator.go:19`
  also accepts `azure_devops` as a `forges[].platform` (metadata-only, no publish driver — see
  ADR-0043/ADR-0046 addendum). Reword to distinguish forge types (3) from publish platforms (2).

**Files:** `docs/specs/01-overview.md`. **Scope:** S. **Dependencies:** none (informational only —
no code change).

---

#### `[ ]` T230: `docs/specs/02-configuration.md` reconciliation

- **Lines 56-57**: design-principle bullet claims changelog/`release.notes` independence "a
  project can have one, both, or neither" — false since ADR-0046; contradicts the spec's own §
  `release` further down. Rewrite to state the release block's one-intent atomicity.
- **Lines 349-352**: "no config-expressible way to get one without the other, root or
  per-environment" — false per-environment today (see T223); once T223 lands, this becomes true and
  should stay as-is, but land this doc fix *after* T223, not before.
- **Lines 58-60**: "Platform sections are opaque to heraut… adding driver-specific fields does not
  require changes to the core" — false; `forgeconfig.Decode` uses `KnownFields(true)` and both
  `config.go` structs and `schema.json` (`additionalProperties: false`) reject unknown keys.
  Rewrite to describe the actual (typed, closed) config surface.
- **Line 596 / YAML comment at 575**: a type with `render` omitted "renders the capitalized type
  name" — actually joins the `"💼 Other"` catch-all group (`internal/generators/native/group.go:96-99`).
  `schema.json:133` and `heraut.sample.yml:141` already have this right; fix spec 02 to match.
- **Line 79**: `sprint` "required when `format` contains `SPRINT`" — currently unenforced (T225);
  land this doc fix alongside or after T225 so the claim is true when read.
- **Missing**: default `rendering.excludes` (`^chore\(release\):`, `^chore\(deps.*\)`,
  `^chore\(pr\)`, `^chore\(pull\)` — `internal/config/commits.go:116-129`) are undocumented, and the
  sample's own example pattern is already a built-in default, making it look meaningful when it's a
  no-op. Document that `EffectiveExcludes` **prepends** built-ins (augment, not replace).
- **Missing**: `--env auto` (`internal/app/env.go:16-61` — resolves the active env from the current
  git branch via each env's `branch:`).
- **Missing**: `release.assets` has no dedicated field entry in § `release` (lines 346-380 cover
  only `notes`/`targets`); document it alongside the strict-vs-lenient distinction from T228 once
  that's resolved.
- **Missing**: § Content generation table (lines ~671-676) omits the per-driver `rendering` key.
- **Missing**: § `rendering` (lines 647-663) omits `rendering.templates` (nine overridable blocks,
  documented in `schema.json:179-221`/sample/spec 05, but not here).
- **Missing**: § Top-level structure (lines 36-51, both code block and table) lists 5 of 8 top-level
  keys — `commits`, `rendering`, `forges` are absent from the doc's own index.
- **Wrong**: two contradictory `token_env` defaults for GitHub — line 487 says offline fallback is
  `GITHUB_TOKEN`; lines 727-728 say "defaults to `GH_TOKEN`". Both are true for different
  subsystems (`internal/forge/detect.go:18` enrichment identity vs.
  `internal/platforms/github/platform.go:16` publish driver) — document which applies where.
- **Missing**: tag signing via git's `tag.gpgSign` (`internal/app/pipeline.go:57-66`,
  `internal/pipeline/git.go:59-67` — signing silently overrides `versioning.tag_type: lightweight`).
- **Missing**: `types_heading_level`'s default (3, i.e. `###` — `internal/generators/native/render.go:224-229`)
  is shown only in an example, never stated as the documented default.
- **Missing**: top-level `versioning.tag_format` (`internal/config/config.go:58`,
  `internal/config/tagformat.go:7`) and `versioning.tag_type` (`config.go:59`,
  `internal/app/pipeline.go:275,383`) are absent — see T232 for the corresponding gap in spec 04;
  pick one spec as the canonical home and cross-reference from the other.
- **Missing/inconsistent**: the branch guard "refuses any `--env <env>` command" (line 162) is
  skipped in `--dry-run` for `release`/`changelog` but runs unconditionally for `version
  next`/`current` — not uniform as implied.
- **Nits** (see `/private/tmp/claude-501/-Users-bchatard-Developer-Adaouat-heraut/76bab33a-9dcf-44ab-bd57-b4ed5e2d623c/scratchpad/audit-config-commands.md`
  for full text): stale T214 comment on `config.EffectiveReleaseNotes`; zero-config example omits
  `release:`.

**Files:** `docs/specs/02-configuration.md`. **Scope:** M. **Dependencies:** land the `sprint`
bullet after T225; land the per-env notes-independence bullet after T223; land the `release.assets`
bullet after T228.

---

#### `[ ]` T231: `docs/specs/03-commands.md` reconciliation

- **Lines 42-47**: init wizard flow still shows two separate "generate release notes?" / "publish
  releases?" questions — collapsed into one by T220 (`internal/scaffold/wizard.go:284-292`,
  commit `a871e70`). Same block has the sprint prompt in the wrong position and omits three real
  prompts: changelog output file, common per-env tag format, GitLab-only API-mode select
  (`wizard.go:627-636`).
- **Lines 49-53**: "Update warning" block describes a pre-wizard dropped-fields warning that
  doesn't exist — `internal/scaffold/dropped.go:50-52`'s `DroppedFields()` returns `nil`
  unconditionally. `commits.tickets`/`release.assets` are carried through verbatim, not dropped.
- **Missing**: `heraut init` silently drops many fields with no warning at all —
  `internal/scaffold/generate.go:41-143` never emits `versioning.initial_version`,
  `versioning.bump`, `versioning.tag_type`, `changelog.tag_pattern`, `changelog.template`,
  `changelog.rendering`, `rendering.*`, `commits.types`, `commits.scopes`,
  `commits.scopes_restricted`, `commits.types_heading_level`, `release.notes`, `forges[].api_url`.
  Document this as a known limitation (or file a separate follow-up task if the fix is to actually
  preserve these on re-run — that's out of this audit's scope to decide).
- **Lines 226-228**: "`version next`… build-id flow is changelog-only" — false; `heraut release
  --build` exists (`internal/cmd/release.go:122`) and is documented as supported in spec 02
  (306-319). The runtime error text at `internal/versioning/tagfmt/tagfmt.go:24-26` has the same
  stale claim and should be corrected alongside the doc.
- **Line 82 (and 158)**: "`--version`: an optional leading `v` is stripped and the rest is used
  verbatim as the tag/version" — the `v` is stripped only from the *version*, the *tag* keeps it
  (`internal/app/resolver.go:30-31`). Reword precisely once T222 is decided (the fix may change
  what's true here — land after T222).
- **Lines 112-113**: release step 7.1 says notes are passed via `--notes` — both drivers actually
  write a temp file and pass `--notes-file` (`internal/platforms/github/platform.go:169`,
  `internal/platforms/gitlab/platform.go:213`).
- **Line 266**: `version sprint bump` "requires confirmation" — it doesn't
  (`internal/cmd/version_sprint.go:21-39`, writes immediately).
- **Missing**: `--offline` absent from the global-flags table (lines 10-18) despite being a root
  persistent flag (`internal/cmd/root.go:26`).
- **Missing**: `heraut commit check --from-latest-tag` undocumented (`internal/cmd/commit.go:70,76-78,104-115,129`).
- **Missing**: § `heraut release` (lines 72-117) never states the "requires at least one resolvable
  publish destination" gate (`internal/cmd/release.go:77-81`) — present in spec 02 and CLAUDE.md,
  referenced *from* the changelog section, but absent from release's own section.
- **Wrong**: release step 1 "Preflight — run `heraut check config` + `heraut check runtime`" (line
  96) — actual: `config.Validate` always, but branch/runtime preflight (`CheckBranch`,
  `app.PreflightCheck`, `pipe.Check()`) only when `!dryRun`; no working-tree check anywhere in this
  path.
- **Wrong**: release steps 6-7 (lines 106-113) imply notes are generated once, before any platform
  call — with multiple targets, notes are regenerated per-target with that target's `LinkContext`
  (`internal/pipeline/release.go:175-206`).
- **Stale**: line 106-107 phrasing "if `release.notes` is configured" implies omitting `notes:`
  skips notes generation — false since ADR-0046 (root case; see T223 for the per-env gap).
- **Missing**: init's write-destination resolution order (`--config` → `HERAUT_FILE` →
  `config.InitDest()`) — spec documents only `--config` and the `.config/` heuristic.
- **Wrong**: init flags — `heraut init --defaults` overwrites an existing config unconditionally
  (see T227); also `--force` here is the *global* root flag, but the global-flags table (line 16)
  documents it only as the promotion-guard bypass, never its init-overwrite meaning. Land after
  T227 lands (or is documented-as-intentional).
- **Missing**: `check runtime` (lines 425-438) omits the advisory working-tree check
  (`internal/app/check.go:99-112`) and the `forge` resolution-failure row (`check.go:119-131`).
- **Missing**: `--verbose` also enables structured debug logging
  (`internal/cmd/release.go:56` → `forgelog.LevelFor` → `PipelineOpts.Logger`), documented only as
  the `[exec]` echo.
- **Missing**: `heraut version current [--env] [--bare]` omits `--force` in its own usage line
  (line 235), unlike `version next` (line 217) which lists it.

**Files:** `docs/specs/03-commands.md`, `internal/versioning/tagfmt/tagfmt.go` (error-text fix, if
picked up here rather than as its own tiny follow-up). **Scope:** M–L. **Dependencies:** the
`--version`/tag-prefix bullet after T222; the init-overwrite bullet after T227.

---

#### `[ ]` T232: `docs/specs/04-versioning.md` reconciliation

- **Line 231**: E002 threshold described as `>=`; code uses strict `>` (`internal/versioning/perenv/promote.go:202`)
  — an equal destination version doesn't trip E002 (E001 catches it instead). Fix the doc to match
  code, unless the team decides `>=` was the intended behavior — that would be a code change, not a
  doc one; confirm intent before touching either.
- **Lines 282-283**: claims CalVer per-env E002 "uses CalVer ordering instead of SemVer ordering" —
  there is one `compareVersionStrings` function used identically for both strategies
  (`internal/versioning/perenv/promote.go:262`); no CalVer-specific comparator exists.
- **Line 203**: `bump: auto` described as resolving "the latest source-env tag" — it actually globs
  the *active* env's own `tag_format` (`internal/versioning/perenv/auto.go:14-27`); `source:` is
  only consumed by the promote path.
- **Line 77**: manual-mode failure "fails with a config error" — actually a runtime error, exit
  code 3, not `exitcode.Config` (2) (`internal/versioning/semver/resolver.go:67`,
  `internal/cmd/exit.go:25`). Decide with the team whether this is a doc fix or a code fix (missing
  `--version` in manual mode arguably *is* a config problem) before landing either.
- **Line 107**: "`PATCH` is mandatory and always the last component" — a trailing literal (e.g.
  `YYYY.MM.PATCH-rc`) parses fine; the actual check is `lastNonLiteral != KindPATCH`
  (`internal/versioning/calver/parser.go:105`). Reword to "the last non-literal token."
- **Line 37**: bump table implies `fix:` commits are what produce a patch bump — `DetermineBump`
  (`internal/versioning/semver/bump.go:13-28`) only inspects `Breaking` and `Type == "feat"`;
  `BumpPatch` is the unconditional floor, so unparsable/non-conventional commits also yield patch.
  Reword to describe the actual floor-based mechanism.
- **Lines 237-244**: § Version resolution logic lists "check E001/E002/E003 → render the
  candidate" — code renders the candidate tag *before* checking E001/E002 (E003 is checked
  earliest) (`internal/versioning/perenv/promote.go:147,165,171-216`). Fix the step order.
- **Lines 247-249**: git-cliff `tag_pattern` advisory — git-cliff is gone (ADR-0045); `tag_pattern`
  is now a native regex scoping the tag walk (`internal/config/config.go:93`,
  `internal/generators/native/generator.go:108-131`), unrelated to the git-cliff glob this note
  describes. Rewrite or remove.
- **Missing**: `{build}` tag-format token entirely undocumented
  (`internal/versioning/tagfmt/tagfmt.go:13` — `buildToken`, consumed by `Render`, `GlobPattern`,
  `ParseVersion`, `DeriveHeadingVersionPattern`, `DeriveTagPattern`, `ValidateBuildID`).
- **Missing**: top-level `versioning.tag_format` / `versioning.tag_type` — see T230 for the
  parallel gap in spec 02; pick one canonical home.
- **Missing**: `--version` bypasses **all four** strategies via `NewStaticResolver`
  (`internal/app/resolver.go:27-43`), not just `bump: manual` — document as a cross-strategy
  override; land after T222 so the documented tag shape is correct.
- **Missing**: promote path's missing pre-release filter (see T226) — document the fixed behavior
  after T226 lands.

**Files:** `docs/specs/04-versioning.md`. **Scope:** M. **Dependencies:** land the manual-mode-exit-code
bullet only after a team decision (doc vs. code fix); land the `--version` bullet after T222; land
the promote-filter bullet after T226.

---

#### `[ ]` T233: `docs/specs/05-generators-and-platforms.md` reconciliation

- **Line 309**: `gh release create` invocation shown with `--notes <notes>` — code writes a temp
  file and passes `--notes-file <path>` (`internal/platforms/github/platform.go:169`); `--notes` is
  never used.
- **Line 341**: `glab release create` shown with `--notes <notes> -R <project>` — code passes
  `--notes-file <tmpfile> --repo <proj>` (`internal/platforms/gitlab/platform.go:213`); neither
  `--notes` nor `-R` are used.
- **Line 342**: `glab release upload` shown with `-R` — actual flag is `--repo`
  (`internal/platforms/gitlab/platform.go:262`).
- **Line 270**: "`Validate()` is called by `heraut check config` and before the pipeline runs" —
  grep finds zero production call sites for `Generator.Validate()`; `check.go` calls only
  `config.Validate(cfg)`, and `Pipeline.Check` calls `Generator.Check()`, never `Validate()`.
  Either wire up the dead call site (code fix — flag to the team as a possible gap, not just a doc
  fix) or correct the doc to describe what's actually called.
- **Line 167**: enrichment-forge fallback chain description is backwards — a **non-empty**
  `cfg.Forges` takes `resolveExplicit` (config wins, CI/origin only fill gaps per-field); `resolveAuto`
  (CI → origin → ambient token env) runs only when `cfg.Forges` is empty
  (`internal/forge/resolve.go:33-38,173`). Also: with several forges and no
  `commits.enrichment_forge`, `EnrichmentIndex` defaults to index **0**, not "nil unless exactly
  one."
- **Line 198**: "two or more [ambient tokens] set → ambiguous, run fails" — only fatal when
  `enrichment_policy: required && !force`; otherwise the run continues degraded
  (`internal/app/pipeline.go:511-521`).
- **Line 316**: "non-matching globs fail the run" stated as universal — see T228; there are two
  modes (strict/lenient) and the doc only describes one. Land after T228's direction is decided.
- **Line 205**: the single-bullet "what happens next" framing under a generator-name label is
  vestigial pre-ADR-0045 branch structure now that `native` is the sole generator — simplify to
  drop the implied second-generator branch.
- **Missing**: § Platforms should state the type-level rule that Azure DevOps forges can never be a
  publish target (`internal/app/platforms.go:30-35,80` — `platformBuilders`/`supportsPublish`, the
  T221 fix from this session) — currently only implied by "two platforms supported."
- **Missing**: `forges[].api_url` and `api_mode` absent from the `forges:` YAML example (lines
  149-163) — `api_mode` appears in prose only; `api_url` appears nowhere
  (`internal/config/config.go:162-163`, `internal/config/validator.go:206`).
- **Missing**: `enrichment_policy: disabled` absent from the auto-detection policy discussion
  (lines 202-222), which covers only `optional`/`required`
  (`internal/config/validator.go:25`, `internal/generators/native/enrich.go:58`).
- **Missing**: § Platform interface (lines 392-401) omits `ReleaseURLFromContext` and
  `LinkContext()` (`internal/port/platform.go:5-19`) — these produce the actual post-release URL,
  making the documented `ReleaseURL` line misleading about the live path.
- **Wrong**: line 329 — GitLab `base_url` "defaults to `https://gitlab.com`" — actual chain is
  `cfg.BaseURL` → `CI_SERVER_URL` (when `GITLAB_CI=true`) → scheme+host of `CI_PROJECT_URL` →
  `gitlab.com` (`internal/platforms/gitlab/platform.go:305-320`).
- **Missing**: lenient assets are attached as positional args inside `release create`, not
  uploaded via a separate call — `UploadAssets` becomes a no-op in that mode
  (`internal/platforms/github/platform.go:177-185,200`, `gitlab/platform.go:215-223,245`); the
  GitHub 422 "immutable release" rationale for this is undocumented.
- **Missing**: template data model gaps — `tplRelease`/`tplGroup` expose `.HeadingPrefix`,
  `tplCommit` exposes `.Subject` (`internal/generators/native/templatemodel.go`); `contributors`
  and `stats` render only from the `release-notes` root template, not `changelog`; `--regenerate`
  replaces any file preamble with a fixed `changelogHeader` constant
  (`internal/generators/native/render.go:18`), which the "free-form preamble" description doesn't
  convey.

**Files:** `docs/specs/05-generators-and-platforms.md`. **Scope:** M. **Dependencies:** land the
asset-glob bullet after T228.

---

#### `[ ]` T234: `docs/specs/06-dx-and-testing.md` reconciliation

- **Lines 84, 90-104**: `MockRunner` contract-test example — `internal/testutil/` no longer has
  `mock_runner.go`; the type is `github.com/adaouat/forge/exec/exectest.NewMockRunner`. The example
  code itself is doubly wrong: `github.New(mr, github.Config{...})` — actual constructor is `New(runner
  port.Runner, cfg *config.Platform)`, there is no `github.Config`, and the asserted arg
  (`"--notes", "..."`) should be `"--notes-file", "<path>"`. (The identical stale snippet also
  exists in `.claude/rules/testing.md` — fix both together, see T237.)
- **Line 108**: same relocation issue for `FakeBin` → `github.com/adaouat/forge/exec/exectest.FakeBin`.
- **Line 132**: "Embedded TOML / Tera content" — native generator embeds Go `text/template` files
  (`internal/generators/native/{blocks,changelog,release_notes}.tmpl`); no TOML or Tera exists in
  the tree since ADR-0045.
- **Line 130**: "Self-update tests use `httptest.Server`" — heraut has no self-update
  (ADR-0014 superseded by forge ADR-0005); update checking is `forge/updatecheck`'s responsibility.
- **Lines 152-159**: CI description (`go build` → `go test` → `golangci-lint run`) is stale —
  actual `.github/workflows/ci.yml` delegates to a reusable `adaouat/forge/.github/workflows/go-ci.yml`
  job with an 85% coverage gate, a separate `build` job (`go build ./cmd/heraut/` + `goreleaser
  check`), and an `hk` job. Document the coverage gate and the hk lint job.
- **Lines 112-114**: integration-test claim ("run `heraut release --dry-run` through the binary")
  is aspirational, not real — no test builds or execs the `heraut` binary; existing tests drive
  cobra in-process or exercise the native generator against a real repo via `forge/exec`. Either
  correct the doc, or note this as a real coverage gap worth its own follow-up task (not in this
  audit's scope to decide which).
- **Line 125**: "golden output comparison… via the binary" for `heraut check config` fixtures — no
  such test exists; the only golden-file tests in the repo are for rendered changelog/release-notes
  output (`internal/generators/native/render_internal_test.go:99-144`). `schema_test.go` tests
  fixtures via the validator/schema only.
- **Line 107-108**: coverage-discipline rule cites `generator` as a config value needing a fixture
  — `generator:` is no longer a config key (ADR-0045); update to reference the actual removed-key
  fixture (`testdata/config/invalid/invalid_generator.yml`) and fix the claimed directory structure
  (`testdata/{config,invalid,valid}`, not one flat `testdata/config/`).

**Files:** `docs/specs/06-dx-and-testing.md`. **Scope:** S–M. **Dependencies:** none.

---

#### `[ ]` T235: `schema.json` + `docs/heraut.sample.yml` cleanup

- **`schema.json:386`**: comment still references git-cliff ("matches git-cliff's own
  azure_devops \"owner\" shape") — removed by ADR-0028/ADR-0045. Reword without the git-cliff
  reference.
- **`schema.json:157-160`**: `ScopeRule.remove` described as "**Reserved**: drop this scope (for
  config composition / includes)" — it's implemented (`internal/config/commits.go:163-167`,
  `EffectiveScopes`). Spec 02 and the sample both already describe it as working; fix the schema
  comment to match.
- **`heraut.sample.yml:37-40`**: `bump: manual` described as requiring "an explicit `--bump` flag
  on the CLI" — no `--bump` flag exists; manual mode requires `--version`
  (`internal/versioning/semver/resolver.go:59-66`). Spec 02 (line 77) already has this right.
- **`heraut.sample.yml`**: prose calling `tag_prefix` just "prefix" in places — align terminology
  with the actual field name.
- **Nits** (see the scratchpad file referenced in T230 for the remaining two): schema `$schema` URL
  pinning behavior in `heraut init`; stdout-vs-stderr inconsistency for config errors between
  `version next`/`current`.

**Files:** `schema.json`, `docs/heraut.sample.yml`. **Scope:** S. **Dependencies:** land the
`ScopeRule.remove` and per-driver-`rendering.excludes` schema comments after T224's direction is
decided, since T224 may change what's actually true about `rendering.excludes`.

---

#### `[ ]` T236: `CLAUDE.md` + `README.md` reconciliation

- **ADR count**: `CLAUDE.md:34,95` say "45 ADRs" — actual count is 46 (0001–0046, ADR-0046 landed
  in commit `ba8c162` without the count bump). `docs/tasks/roadmap.md` itself self-describes "25
  ADRs" in its overview section — fix that occurrence too while touching this.
- **`CLAUDE.md:67,162-164`**: references `remote_metadata` — renamed to `commits.enrichment_policy`
  by ADR-0043 (`internal/config/commits.go:23-26` literally documents the rename in a comment).
- **Project layout tree (`CLAUDE.md:54-102`)**: missing `internal/forge/` (+ `github`/`gitlab`/`azure`
  subpackages, the newest major layer — `internal/app` imports all four directly),
  `internal/commitwizard/`, `internal/conventionalcommit/`, root `pkl/` (ADR-0029) and
  `pkl_test.go`, `docs/guides/`, `LICENSE.md`. `internal/cmd/` list omits `commit.go`;
  `internal/pipeline/` list omits `git.go`, `linkctx.go`, `warn.go`; `internal/versioning/` list
  shows only `result.go` (also has `resolver.go`, `static.go`); `internal/port/` omits the `Forge`
  interface (`internal/port/forge.go:67`, ADR-0043); workflows list omits `osv-scan.yml`.
- **`CLAUDE.md:16-25` + `README.md:129-139`**: `heraut commit` (verify/check/create,
  `internal/cmd/commit.go`, backed by `internal/commitwizard/`) is a real, `--help`-visible, spec-03-documented
  command family absent from both top-level docs.
- **`CLAUDE.md:27-28` + `README.md:23-25`**: "two platforms" — `schema.json`'s `forges[].platform`
  enum includes `azure_devops` (metadata-only forge, no publish driver). Reword to distinguish forge
  types (3) from publish platforms (2).
- **`CLAUDE.md:148-152`** "Bundled external CLIs": no longer the whole picture — enrichment is
  direct `net/http` now (`internal/forge/{github,gitlab,azure}/`, ADR-0035/ADR-0042), not CLI
  shell-outs; `gh`/`glab` remain publish-only. The narrower claims (nothing bundled, `heraut check
  runtime` verifies PATH+tokens) still hold.
- **`CLAUDE.md:156-158`**: publish-destination wording ("a forge that auto-detects from CI/git
  origin") predates this session's T221 fix — an azure-only auto-detected forge no longer counts as
  a resolvable destination (`internal/app/pipeline.go:160-190`,
  `TestHasResolvablePublishTarget_AzureOnlyIsNotResolvable`). Update the wording to reflect the
  driver-support requirement.
- **`CLAUDE.md:129-130`**: ldflags build-arg shown as `HERAUT_VERSION=${{ github.ref_name }}` —
  actual: `${{ needs.release.outputs.tag }}` (`.github/workflows/release.yml:170-171`). The
  invariant itself (`main.Version` the only ldflag, both files inject it identically) still holds.
- **`CLAUDE.md:114-115,119`**: mise lint-task descriptions and the `hk fix -S golangci-lint`
  example — actual hk step id is `golangci_lint` (underscore, `.config/hk/config.pkl:28`); the
  hyphenated form matches no step.
- **`CLAUDE.md:35,96`**: `docs/tasks/` described as a single roadmap file — actually
  `roadmap.md` + three dedicated roadmaps (`forge-abstraction-`, `native-generator-`,
  `release-config-`, and now this one) + `README.md`.
- **`README.md`**: global-flag list is wrong (verified: `heraut check --version` errors —
  `--version`/`-v` is root-only; on `release`/`changelog` `--version` is a string override, not a
  boolean); `--offline` is missing from the same list; Docker-tag example table stuck at an old
  version (`0.9.0`) while HEAD is `v0.58.0`; "Documentation" section omits `docs/guides/`.

**Files:** `CLAUDE.md`, `README.md`, `docs/tasks/roadmap.md` (ADR-count line only). **Scope:** M.
**Dependencies:** land the publish-destination bullet as-is (T221 already shipped this session).

---

#### `[ ]` T237: `.claude/rules/{coding,testing,workflow}.md` reconciliation

- **`coding.md`** "Embedded assets" section (lines ~85-93): entirely about the removed
  `gitcliff`/`communique` packages and `EffectiveChangelogConfig()`/`EffectiveReleaseNotesConfig()`,
  neither of which exist. Replace with the actual native-generator embed description
  (`internal/generators/native/render.go:20-26`).
- **`coding.md`** architecture diagram (lines ~19,34): names `internal/generators/ (gitcliff,
  communique)` — only `native/` remains (ADR-0045); also says "never calls `gitcliff.New(...)`,
  `github.New(...)`" as if gitcliff were still a live example.
- **`coding.md:23,32`**: `internal/adapter/exec/` doesn't exist — the runner is
  `github.com/adaouat/forge/exec`, imported as `execadapter`. `CLAUDE.md:69` already has this
  right; reconcile `coding.md` to match.
- **`coding.md:6,27`**: claims `main.go` calls `fang.Execute` — actual: `cli.Run(...)` from
  `forge/cli` (fang is now an indirect dependency reached inside `forge/cli/run.go`).
- **`coding.md:26`**: claims three build flags (`Version`, `ProjectURL`, `LatestURL`) — `main.go`
  declares only `Version`. This also contradicts `CLAUDE.md:136` ("`main.Version` is the only
  ldflag") — reconcile both files to state the same fact.
- **`coding.md:103`**: "Global flags on root" list omits `--offline`.
- **`coding.md:41` layer-rules table**: doesn't account for `internal/cmd`'s actual imports
  (`exitcode`, `port` beyond what's listed) or for `internal/{forge,commitwizard,exitcode,testutil}`
  at all — no rows exist for these packages.
- **`testing.md:25-51`**: `MockRunner`/`FakeBin` code sample and paths — see T234's identical
  finding for spec 06; fix both files' examples together so they don't drift again
  (`github.com/adaouat/forge/exec/exectest`, `New(runner port.Runner, cfg *config.Platform)`, no
  `github.Config`, `--notes-file` not `--notes`).
- **`testing.md:94`**: "self-update tests use `httptest.Server`" — no self-update exists in this
  repo (see T236's ADR-0014-superseded note).
- **`testing.md:12-23`**: four-layer table has no home for the `internal/forge/*` HTTP clients,
  which are tested against real `httptest` servers (`gitlab_test.go`, `graphql_test.go`,
  `github_test.go`, `github_internal_test.go`, `azure_test.go`) — a contract style the table's
  CLI-only "Contract" row doesn't describe. Add a row or a note.
- **`testing.md:107-108`**: coverage rule cites `generator` as a live config value — see T234's
  identical finding; fix once, referenced from both files if practical.
- **`workflow.md:74`**: commit-msg hook described as `cog` — actual step is `heraut-commit-lint`
  running `go run ./cmd/heraut commit verify` (dogfooding, ADR-0029); cocogitto was dropped in
  ADR-0028.
- **`workflow.md`**: "Releases are cut by pushing a `v*` tag" contradicts ADR-0018 (correctly
  `Accepted`) and the actual `.github/workflows/release.yml` (`on: workflow_dispatch`,
  heraut-owned, GoReleaser build-only). Also has a `fix(generators/gitcliff)` example scope that no
  longer exists as a package.

**Files:** `.claude/rules/coding.md`, `.claude/rules/testing.md`, `.claude/rules/workflow.md`.
**Scope:** M. **Dependencies:** none.

---

#### `[ ]` T238: `docs/adr/README.md` + affected ADR bodies — status annotations and stale references

**Incorrectly-marked status (add a "Superseded by ADR-00XX" / forward-pointer annotation; the decision
itself doesn't need editing, only the pointer):**
- **ADR-0042** (GitLab GraphQL via `glab api graphql`) — ported to native `net/http` by ADR-0043
  P1 (`internal/forge/gitlab/{graphql,rest}.go`); ADR-0043 already lists 0042 under "Extends /
  supersedes" — just needs the reverse pointer in 0042 itself and the README row.
- **ADR-0040** (`changelog.remote` config block) — the key was removed entirely by ADR-0043
  (`internal/config/loader.go:52`, `removedKeys`). README annotates 0026 and 0035 for this same
  removal but leaves 0040, whose *title* is the removed key, bare.
- **ADR-0022** (fat-injection / thin git-cliff templates) — `internal/generators/gitcliff/` is
  deleted (ADR-0045); `linkEnv`/`HERAUT_REMOTE_URL` no longer exist outside a stale comment. Note:
  ADR-0045's own "mentions git-cliff in passing" classification of 0022 is itself wrong — git-cliff
  templates are 0022's entire subject; fix that classification note in ADR-0045 too, since it's the
  thing steering README annotation decisions.
- **ADR-0024** (ticket linking via git-cliff `link_parsers`) — `tickets` moved to
  `commits.tickets` by ADR-0033; the "git-cliff only" gate and cocogitto/communique error path are
  unreachable with native as sole generator.
- **ADR-0006** (config naming: `release.platforms`, `generator: git-cliff`) — ADR-0043 already
  explicitly lists ADR-0006 under "Extends / supersedes (naming)"; the README index shows no
  supersession for it at all.

**Stale detail inside an otherwise-correctly-`Accepted` ADR (fix the specific claim, not the
status):**
- **ADR-0044**: canonical migration example uses `release.notes.generator` — that key is itself in
  `removedKeys` since ADR-0045; the one copy-pasteable example in the doc doesn't load.
- **ADR-0019**: "generator switch triggers full replacement" — no such branch exists in
  `config.MergeContentDriver`; its own example (`environments.prod.changelog: {config:
  cliff.prod.toml}`) is a hard load error today.
- **ADR-0031**: reads from `commit_lint.types`/`.scopes`/`.ticket` — renamed to `commits.types` /
  `commits.scopes` / `commits.tickets` by ADR-0033. README annotates ADR-0027 for this exact rename
  but leaves 0031 bare.
- **ADR-0041**: uses `commits.remote_metadata` throughout — renamed to `commits.enrichment_policy`
  by ADR-0043. README annotates ADR-0023 for this but leaves 0041 bare.
- **ADR-0033**: defines `commits.remote_metadata` — the key it created was itself renamed by
  ADR-0043; also states git-cliff stays "functional but unreconciled," which ended with ADR-0045.
- **ADR-0032**: "native is additive, not a replacement… git-cliff remains the default" — reversed
  by ADR-0045 (native is now sole). The ADR carries no status annotation at all — add one.
- **ADR-0021**: "Context-injection shape" section (cocogitto adapter, communique adapter, git-cliff
  `HERAUT_REMOTE_URL`) describes three adapters, none of which exist; the core per-platform
  regeneration behavior it introduced does still hold (`internal/pipeline/release.go:171-176`).
- **ADR-0045**: its own Consequences section claims the scaffold wizard still offers
  `git-cliff`/`communique` as live options — false as of commit `a871e70`
  (`internal/scaffold/wizard.go` has no such prompt); also cites `internal/scaffold/cliff.go`,
  which no longer exists.
- **ADR-0003**: claims `main.go` calls `fang.Execute` and that subcommands live under
  `cmd/heraut/*.go` — see T236/T237's identical finding; fix the ADR's Decision section to name
  the actual entry point and file locations, or add a note that the substantive decision
  (cobra+fang under the hood, now via forge) still holds even though the cited paths don't.
- **ADR-0011 / ADR-0012**: both sketch release-notes generation *after* publishing — actual order
  is tag → push → notes → publish (`internal/pipeline/release.go:151-189`, per ADR-0021).

**Index drift (`docs/adr/README.md`):**
- Title mismatch, row for ADR-0034 ("Native Remote Enrichment via Platform CLIs" vs. the file's own
  "Native remote enrichment (Phase 2)") — the invented subtitle asserts a transport the same row's
  annotation then retracts.
- Title mismatch, row for ADR-0016 ("Batteries-Included Docker Image" vs. file's "Bundled Docker
  Image (Full Release Runner)").
- Minor title/status wording drift: rows for ADR-0035, ADR-0033, ADR-0021.

**Broken references (cited path no longer exists):**
- ADR-0035:40 cites `internal/generators/native/enrich_azure.go` — moved to
  `internal/forge/azure/{azure.go,prquery.go}`.
- ADR-0039:49 cites `internal/generators/native/enrich_github.go` — moved to
  `internal/forge/github/graphql.go`.
- ADR-0017:19 cites `cmd/check.go` — actual path is `internal/cmd/check.go`.
- ADR-0026:115 cites `internal/generators/gitcliff/generator.go` (package deleted; ADR-0026 is
  already annotated in the README, so this is low severity — fix opportunistically).

**Files:** `docs/adr/README.md`, `docs/adr/0003-*.md`, `0006-*.md`, `0011-*.md`, `0012-*.md`,
`0017-*.md`, `0019-*.md`, `0021-*.md`, `0022-*.md`, `0024-*.md`, `0026-*.md`, `0031-*.md`,
`0032-*.md`, `0033-*.md`, `0035-*.md`, `0039-*.md`, `0040-*.md`, `0041-*.md`, `0042-*.md`,
`0044-*.md`, `0045-*.md`. **Scope:** L (many small edits across many files — consider splitting
into "index + status annotations" and "stale-detail body edits" sub-passes if it proves too large
for one sitting). **Dependencies:** none.
