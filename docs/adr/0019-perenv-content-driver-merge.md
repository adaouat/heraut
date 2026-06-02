# ADR-0019: Per-environment content-driver overrides deep-merge

- **Status**: Accepted
- **Date**: 2026-06-02
- **Deciders**: bchatard

---

## Context

A per-environment block can override the changelog and release-notes generators:

```yaml
changelog:
  generator: git-cliff
  output: CHANGELOG.md

environments:
  prod:
    bump: promote
    source: vali
    changelog:
      tag_pattern: "..."   # wanted: just override this
```

Until now `environments.<env>.changelog` (and `release.notes`) **replaced** the top-level
`ContentDriver` wholesale (`internal/app/pipeline.go`: `effectiveChangelog =
envCfg.Changelog`). The consequence: to override a single field you had to re-declare every
field, and `generator` is `required` by the validator, so a partial block like the one
above was rejected with `environments.prod.changelog.generator: required`.

The original motivating case was overriding `tag_pattern` per environment. That motivation
shrank after [ADR-0009-adjacent work] auto-derived `tag_pattern` per environment from the
effective `tag_format` (roadmap T61), so an explicit per-env `tag_pattern` is rarely needed
now. What remains are genuine but narrower cases: a different git-cliff `config` file, a
different `output` path, or a different `template` for one environment.

## Decision

Per-environment `changelog` and `release.notes` blocks **deep-merge field-by-field** over
the top-level driver instead of replacing it. A field set in the per-env block wins; an
unset (zero-value) field inherits from the top-level driver.

**Generator switch is an exception — it triggers full replacement.** If the per-env block
sets a `generator` that differs from the top-level one, the per-env block is used as-is with
no inheritance. Rationale: generator-specific fields (`config`, `template`) are meaningless
across generators — inheriting a git-cliff `config` into a `communique` driver would be
wrong. The rule reads naturally: *same generator (or unset) → tweak fields; different
generator → fresh config.*

The merge is implemented as a pure `config.MergeContentDriver(base, override) *ContentDriver`
helper, used in two places:

1. **Pipeline** — `internal/app/pipeline.go` resolves the effective driver via the merge.
2. **Validation** — `internal/config/validator.go` validates the *merged* per-env driver
   (not the raw block), so an inherited `generator` satisfies the `required` check while a
   driver that has no generator from either level still fails.

### Scope and non-goals

- **Lists stay replace.** `release.platforms` is still replaced wholesale per environment —
  merging lists by index or by type is ambiguous and error-prone. `release.assets` is not
  overridable per environment today (the `EnvRelease` struct has no `assets` field); that is
  unchanged.
- **No explicit "unset".** Because empty means inherit, a per-env block cannot blank out a
  value the top-level set (e.g. force "no output" when the top-level sets `output`). This is
  an accepted limitation; it has not come up for the string-only `ContentDriver` fields.

## Consequences

**Positive**

- Partial per-env overrides work: `environments.prod.changelog: { config: cliff.prod.toml }`
  inherits the top-level `generator`/`output`.
- The validator no longer rejects partial per-env blocks; it validates the effective driver.
- One small, pure, well-tested helper; no change to the wire format or schema.

**Negative / trade-offs**

- A behavioural change: a config that *replaced* the top-level driver and relied on omitted
  fields being empty (e.g. expecting `output` to be unset for one env while the top-level
  set it) now inherits those fields. In practice per-env blocks re-declared every field they
  cared about, so the merged result is identical for any block that set all its fields. The
  only observable difference is for blocks that previously omitted a field *expecting it to
  be empty* — now it inherits. This is the intended improvement.
- The "generator differs → full replace" branch is a small special case readers must know
  about; it is documented in the spec and enforced by tests.

## Alternatives considered

- **Generator-only inherit** (make just `generator` inheritable, keep everything else
  replace): smaller surface, but leaves the other partial-override cases (`config`,
  `output`, `template`) still forcing a full re-declaration. Rejected as a half-measure.
- **Drop the change** (keep full replace, document "repeat generator"): viable after T61
  reduced the motivation, but the remaining `config`/`output`/`template` cases are real and
  the merge helper is cheap. Rejected.
- **Always field-merge, even across generators**: simpler rule, but inherits nonsensical
  generator-specific fields when switching generators. Rejected in favour of the
  differ→replace exception.
