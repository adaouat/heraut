# Héraut — Changelog Rotation Roadmap

> Status: Active
> Design: [`docs/superpowers/specs/2026-08-28-changelog-rotation-design.md`](../superpowers/specs/2026-08-28-changelog-rotation-design.md)
> ADRs: new ADR-0047 ("Changelog output resolves after version, not at config-build time" — written in T249)
> Main roadmap: tracked as Phase 29 in [`roadmap.md`](roadmap.md)

This roadmap breaks the **rotating changelog file naming** design (T243) into incrementally
shippable tasks. It lives in its own file per the design doc's own placement decision, matching the
`release-config-roadmap.md` / `docs-audit-roadmap.md` precedent.

`changelog.output` (root and per-env) gains optional versioning-strategy tokens — `{YYYY}`, `{MM}`,
`{DD}`, `{WW}`, `{QQ}`, `{SS}`, `{SPRINT}` for CalVer strategies; `{MAJOR}`, `{MINOR}` for SemVer
strategies — so a project's changelog can rotate into one file per calendar period or per release
line (e.g. `CHANGELOG_{YYYY}.md`, `CHANGELOG_{MAJOR}.md`). A literal `output` with no tokens behaves
exactly as today. Tag-scoping for the rotated file is auto-derived from the same tokens, never a
second field to hand-maintain. Per-env strategies and the `heraut init` wizard are explicitly out of
scope for this pass (see design doc "Non-goals").

## Conventions

- Task IDs **continue the global sequence** (`T244+`) so they never collide with the main roadmap
  or any other dedicated roadmap.
- This file is the **single source of truth** for task status. Same checkbox markers as the other
  roadmaps: `[ ]` not started, `[x]` done. Follow the two-step flow
  ([`workflow.md`](../../.claude/rules/workflow.md)): implement (TDD: failing test first), then flip
  `[ ]` → `[x]` and add a one-paragraph completion note.
- **No real data** anywhere (samples, docs, tests): synthetic placeholders only.
- Additive config change — no breaking migration, no removed keys. `output` keeps working literally
  for every project that doesn't opt into tokens.
- The main `roadmap.md` Phase 29 block is a navigable index only; it carries no checkboxes.

## Progress at a glance

| Task | Description                                                                          | Status |
|------|---------------------------------------------------------------------------------------|--------|
| T244 | `calver`: generalize `periodKey` into a caller-scoped prefix-key + literal-prefix-regex helper | Done |
| T245 | `semver`: add `MAJOR`/`MINOR` extraction helper                                        | Not started |
| T246 | `internal/config/validator.go`: static token-vocabulary + prefix-order + per-env-rejection checks | Not started |
| T247 | `internal/app`: `port.Generator` rotation decorator + wiring into `buildChangelogPipelineConfig` | Not started |
| T248 | Integration test: real-git-repo rotation run across a simulated period boundary        | Not started |
| T249 | Docs: `schema.json`, `docs/heraut.sample.yml`, spec, new ADR-0047                       | Not started |

Sequencing follows the design doc's "Roadmap placement": T244/T245 are pure-function unit tests with
no dependency on each other (can build in either order or in parallel). T246 depends on both (needs
the prefix-order/vocabulary logic they expose). T247 depends on T244–T246 (needs validated,
computable tokens to substitute). T248 depends on T247 (exercises the real wiring end to end). T249
documents the settled behavior and lands last, alongside ADR-0047.

---

#### `[x]` T244: `calver` — caller-scoped prefix-key + literal-prefix-regex helper

`internal/versioning/calver/resolver.go`'s existing `periodKey(tokens, values)` builds a key from
*every* non-`PATCH`, non-literal token in the format, in format order — used today only for
`PATCH`-reset comparison. Rotation needs the same idea generalized to a caller-chosen **subset** of
leading tokens (e.g. just `YYYY` even when the format is `YYYY.MM.PATCH`), plus a way to render that
prefix as a **literal regex** matching any tag in the same bucket (see design doc §4: render the
literal prefix — tokens up to and including the last requested token, with their known rendered
values, plus the literal separators between them — anchored, `regexp.QuoteMeta`'d, with a
digit-boundary guard against partial-digit false matches).

Two new exported functions (naming TBD at implementation time, e.g. `BucketKey`/`BucketPattern`),
built alongside the existing `ParseFormat`/`ParseVersion`/`RenderVersion`/`periodKey` in
`internal/versioning/calver`. Must also validate/report when a requested token set is **not** a
contiguous prefix of the format's own token order (design doc §1) — the hard constraint that keeps
tag-scoping derivable from the version string alone, no git-date lookups ever.

**Files (expected):** `internal/versioning/calver/resolver.go` (or a new file in the same package)
+ tests. **Scope:** S. **Dependencies:** none.

Landed as planned, plus one refinement found during implementation: `ValidateRotationTokens` also
rejects a rotation boundary that isn't immediately followed by a literal separator in the format
(e.g. `YYYYMM.PATCH` rotating by `{YYYY}` alone — `YYYY` runs straight into `MM` with no separating
character). Without this, `BucketPattern` would need a generic non-digit-or-end fallback guard that
can incorrectly reject a valid tag when the token right after the boundary has no separator before
it either (traced through concretely: format `YYYY.MMPATCH` rotating by `{YYYY, MM}` would render
patch values with no separator, e.g. `2026.055` for MM=05/PATCH=5, and the fallback guard rejects
this even though it's in the right bucket). Rejecting the ambiguous case at validation time — a
clear config error — was judged better than shipping a bucket-scoping regex that's correct only for
some patch values. `BucketKey`/`BucketPattern` both call `ValidateRotationTokens` internally, so
this same rule applies uniformly regardless of which one a caller reaches for.

Refactored `RenderVersion` and the existing (unexported, previously untested-in-isolation)
`periodKey` to share a new `renderToken(kind, Values) string` helper instead of duplicating the
same per-token-kind switch three times (`RenderVersion`, `periodKey`, and the new
`BucketKey`/`BucketPattern`) — a pure refactor, confirmed behavior-preserving by every pre-existing
`TestRenderVersion`/`TestResolve_*`/`TestBumpFromDate_*` case passing unchanged. `go test ./...` and
`hk check` both clean.

#### `[ ]` T245: `semver` — `MAJOR`/`MINOR` extraction helper

Given a bare SemVer version string, extract `MAJOR` and `MINOR` as separate values for substitution
into `{MAJOR}`/`{MINOR}` tokens. Simpler than T244 — no format-token parsing needed, SemVer's
`X.Y.Z` shape is fixed. Cover pre-release/build-metadata suffixes if `IsBareVersion`-style versions
in this codebase can carry them (check `internal/versioning/semver/bump.go`'s existing
`IsBareVersion` for what "bare" means here before assuming plain `X.Y.Z`).

**Files (expected):** `internal/versioning/semver/` (new file or extend `bump.go`) + tests.
**Scope:** XS. **Dependencies:** none.

#### `[ ]` T246: config validation — token vocabulary, prefix-order, per-env rejection

At `heraut check config` / load-time validation (`internal/config/validator.go`), parse `{TOKEN}`
placeholders out of `changelog.output` (root and any per-env override) and reject:

- A token outside the active strategy family's vocabulary (e.g. `{YYYY}` under `semver`).
- For CalVer: a token not present in `versioning.format`, or a requested set that isn't a prefix of
  the format's token order (uses T244's helper).
- Any use under a `*-per-env` strategy — explicit config error, not a silent ignore (design doc
  "Non-goals").

Add one valid fixture per strategy family with a tokenized `output` to `testdata/config/`, plus one
invalid fixture per rejected case.

**Files (expected):** `internal/config/validator.go` + `internal/config/validator_test.go` +
`testdata/config/` fixtures. **Scope:** S. **Dependencies:** T244, T245.

#### `[ ]` T247: `internal/app` rotation decorator + wiring

New unexported `port.Generator` decorator (design doc §3) wrapping the constructed
`native.Generator`. On `Generate(tag, ctx)`: no tokens present in the raw `Output` → delegate
straight through unchanged (zero behavior change for every existing config, must be covered by a
test asserting exact passthrough). Tokens present → parse `tag` into its bare version, compute
concrete `Output`/`TagPattern` via T244/T245's helpers, construct a fresh `native.Generator` with
those concrete values (reusing the existing `buildGenerator` constructor), delegate `Generate(tag,
ctx)` to it. Wire into `buildChangelogPipelineConfig` (`internal/app/pipeline.go`) — changelog only,
never the release-notes config-building path (design doc "Non-goals": `release.notes.output` is
never written to disk, rotation doesn't apply). Derived `TagPattern` only applies when the user
hasn't already set an explicit one, same precedence `withEnvDerivations` already uses.

**Files (expected):** new `internal/app/changelog_rotation.go` + test, `internal/app/pipeline.go`.
**Scope:** M. **Dependencies:** T244, T245, T246.

#### `[ ]` T248: integration test — rotation across a simulated period boundary

Using the existing real-git-repo integration harness, drive a changelog run producing two tags in
different rotation buckets (e.g. two CalVer tags a year apart) and assert two separate output files
exist, each containing only its own bucket's entries — proving native's bootstrap-on-missing-file
path produces correct rotation with zero changes to its own algorithm (design doc §5).

**Files (expected):** an integration test file alongside the existing pipeline integration tests.
**Scope:** S. **Dependencies:** T247.

#### `[ ]` T249: docs — schema, sample, spec, ADR-0047

Update `schema.json` (`changelog.output` description/pattern examples), `docs/heraut.sample.yml`
(a commented rotation example per strategy family), and wherever `changelog.output` is documented
in `docs/specs/` today. Write ADR-0047 ("Changelog output resolves after version, not at
config-build time") per the design doc's outline — the decorator pattern and why the two more
obvious-looking placements (native itself; config-build time) are both wrong.

**Files (expected):** `schema.json`, `docs/heraut.sample.yml`, `docs/specs/02-configuration.md` (or
wherever applicable), `docs/adr/0047-changelog-output-resolves-after-version.md`,
`docs/adr/README.md`. **Scope:** S. **Dependencies:** T247.
