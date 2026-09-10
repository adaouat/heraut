# ADR-0052: `versioning.bump` becomes an object with mode + per-commit overrides

- **Status**: Accepted
- **Date**: 2026-09-10
- **Deciders**: bchatard

---

## Context

`DetermineBump` (`internal/versioning/semver/bump.go`) defaulted every release to at least a
patch bump the moment any parseable conventional commit existed since the last tag — a lone
`chore(deps): bump foo` forced a patch release exactly like a `fix:` would. There was no way to
say "these commit types don't count toward the bump," and no way to change which level a type
maps to (e.g. promote `fix:` to a minor bump, or demote breaking changes to minor pre-1.0).

Separately, `BumpVersion`'s switch had no explicit `BumpNone` case — it fell to the same
`default: patch++` branch as `BumpPatch`, so even a correct "no bump" determination would have
silently patch-bumped the version anyway. This was latent: `BumpNone` was previously only ever
returned by the "no tags yet" and manual-override paths, neither of which calls `BumpVersion`.

`versioning.bump` was a plain string (`"auto"` | `"manual"`). Reusing the existing `Exclude`
matcher shape (`type`/`regex`, as used by `rendering.excludes`) for bump exclusions doesn't fit
inside a string field, and a design that also wants configurable bump *levels* (not just
exclusion) needs a value alongside the matcher besides.

## Decision

`versioning.bump` becomes an object:

```yaml
versioning:
  bump:
    mode: auto                 # auto (default) | manual
    overrides:
      - type: chore
        bump: none
      - regex: '^chore\(deps.*\)'
        bump: none
      - type: fix
        bump: minor
      - breaking: true
        bump: minor
```

`config.Versioning.Bump` changes from `string` to `*BumpConfig`:

```go
type BumpConfig struct {
    Mode      string     `yaml:"mode,omitempty"`
    Overrides []BumpRule `yaml:"overrides,omitempty"`
}

type BumpRule struct {
    Type     string `yaml:"type,omitempty"`
    Regex    string `yaml:"regex,omitempty"`
    Breaking *bool  `yaml:"breaking,omitempty"`
    Bump     string `yaml:"bump"` // major | minor | patch | none
}
```

Two nil-safe accessors, `Versioning.BumpMode()` and `Versioning.BumpOverrides()`, keep every
call site from having to nil-check `Bump` directly — omitting the whole `bump:` block is
equivalent to `{mode: auto}` with no overrides, matching today's default behavior exactly.

**Per-commit resolution** (`DetermineBump(commits []string, overrides []config.BumpRule)`), in
order:

1. The first matching rule in `overrides` wins — a rule matches when every condition it sets
   (`type`, `regex` against the commit subject, `breaking`) holds; unset conditions are
   wildcards. `type`/`regex` are mutually exclusive, like `Exclude`; `breaking` can combine with
   either. A rule can therefore override the "breaking commits are always major" default too
   (e.g. `{breaking: true, bump: minor}`) — nothing is hardcoded as unconditional anymore.
2. Otherwise, the built-in defaults: breaking → major, `feat` → minor, any other conventional
   commit → patch.
3. A non-conventional commit matched by nothing contributes nothing (`none`).

The release's bump is the highest level any commit contributes. `BumpVersion` gained an explicit
`case versioning.BumpNone` (no-op, version unchanged) fixing the latent bug above. When the
overall result is `BumpNone` with real commits present, `resolveAuto` and `BumpAuto` (the same
calculator perenv's "auto" environments share) now return a dedicated error instead of silently
falling through:

```
no releasable commits since v0.62.0: 3 commit(s) since then are excluded from the version bump
  - chore: bump deps
  - docs: fix typo
  - chore(ci): update workflow
```

This is distinct from the pre-existing "no commits since %s at all" error (still returned when
`git log` finds zero commits) — the new error fires when commits exist but none of them qualify.
The commit list shows subject lines only, not hashes: getting hashes into the error would require
changing the `git log --format=%B%x00` call *and* the `VersionCalculator.BumpAuto` interface that
`internal/versioning/perenv` depends on (it fetches commits independently and passes plain
message strings across that interface) — out of scope for a display detail.

## Consequences

- **Breaking config-shape change, deliberately not migrated.** Every `.heraut.yml` (and this
  repo's own `.config/heraut.yml`) with a bare `bump: auto`/`bump: manual` must become
  `bump: {mode: auto}` / `bump: {mode: manual}`. heraut is pre-v1.0 — no back-compat shim
  (dual-form `UnmarshalYAML` accepting both a scalar and a mapping) was built. `schema.json`,
  `docs/heraut.sample.yml`, `docs/specs/02-configuration.md`, `docs/specs/04-versioning.md`,
  `README.md`, and every `testdata/config/valid/*.yml` fixture with a top-level `versioning.bump`
  were updated. `environments.<env>.bump` (per-env `"auto"`/`"promote"`, a different field
  despite the shared name) is untouched.
- **Default behavior is unchanged when `bump:` is omitted or has no `overrides`.** This is
  additive for every existing config that doesn't opt in — the only forced migration is the
  scalar → mapping shape for the `mode` value itself.
- **No calver/per-env change.** `calver` and `perenv` resolvers never call `DetermineBump` —
  bump-level overrides are inherently a SemVer concept (calver has no "bump amount," it derives
  the next value from date/sprint tokens). `semver-per-env`'s "auto" environments get the feature
  for free since they share `semver.Resolver.BumpAuto`.
- **One new failure mode for `heraut release`.** A repo that previously always got a patch bump
  from chore-only activity will now error once `bump.overrides` excludes those types, requiring a
  releasable commit before the command succeeds. This is intentional — see Decision — but is a
  behavior change for anyone who opts in.

## Alternatives considered

- **A standalone git-cliff-style `no_increment_regex` list, independent of `commits.types`.**
  Rejected in favor of the `type`/`regex` matcher shape shared with `rendering.excludes` — one
  mental model for "match a commit by type or subject regex" instead of two.
- **Keep `bump` a string; add a sibling `bump_excludes` field.** Considered as the
  zero-breakage option (see conversation leading to this ADR), but the team preferred a single
  nested `bump: {mode, overrides}` block now, accepting the pre-v1.0 break, over two flat
  top-level fields whose relationship is only implied by naming.
- **Hardcode "breaking commits are always major," not overridable.** Rejected — explicitly
  requested for full configurability (e.g. treating breaking changes as minor pre-1.0). Modeled
  as one more optional matcher condition (`breaking *bool`) on the same rule shape rather than a
  separate mechanism.
- **Include commit hashes in the "no releasable commits" error.** Rejected: would require
  changing the `git log` format string in two independent call sites (`semver.resolveAuto` and
  `perenv`'s own fetch feeding `BumpAuto`) and widening the `VersionCalculator` interface for a
  display-only improvement. Subject lines are sufficient to identify the excluded commits.
