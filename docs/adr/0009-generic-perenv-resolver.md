# ADR-0009: Generic Per-Env Resolver Design (`perenv` + `tagfmt`)

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

Heraut supports two per-environment strategies: `semver-per-env` and `calver-per-env`.
Most of their logic is identical:

- Listing tags for the active environment via a glob derived from `tag_format`
- Parsing tags back to their bare version via the same `tag_format`
- Promote-mode resolution against a source environment
- E001 / E002 / E003 guards
- `source:` field resolution and cycle detection

The two strategies differ only on:

- **How "next version" is computed in auto mode** — SemVer scans conventional commits
  and bumps the appropriate component; CalVer derives from the clock and resets /
  increments `PATCH`.
- **How tags are ordered** — SemVer order vs. CalVer (parsed-tokens) order.

A naive design would implement both strategies as separate packages, leading to ~94%
identical code, two places to fix every per-env bug, and a strong incentive for the
two implementations to drift apart over time. A `{version}` / `{env}` substitution
helper would also be duplicated alongside.

## Decision

Consolidate the two per-env packages into a single generic implementation, and extract
the tag-format helpers into a shared package.

### `internal/versioning/tagfmt/`

A small package, no dependencies on `config` or the resolvers:

```go
package tagfmt

// Render substitutes {version} and {env} tokens in a template.
// Returns an error if template has no {version} token.
func Render(template, env, version string) (string, error)

// ParseVersion extracts the bare version from a tag given its template.
// Returns an error if the tag does not match the template structure.
func ParseVersion(template, tag string) (string, error)

// GlobPattern produces the git tag glob pattern for `git tag -l`.
// Replaces {version} with `*` and {env} with the literal env name.
func GlobPattern(template, env string) string
```

Both per-env resolvers go through this package; nothing else does.

### `internal/versioning/perenv/`

```go
package perenv

// VersionCalculator is implemented by the underlying single-env resolvers
// (internal/versioning/semver/ and internal/versioning/calver/).
type VersionCalculator interface {
    // BumpAuto computes the next version from existing tags + conventional
    // commits. Used by the auto-mode path. tags is already filtered to the
    // active environment's namespace.
    BumpAuto(tags []string, commits []string) (string, error)

    // BumpFromDate computes the next version from existing tags + the clock.
    // Used by the auto-mode path. Commits are ignored.
    BumpFromDate(tags []string) (string, error)
}

func New(runner port.Runner, cfg config.Versioning, env string, force bool, calc VersionCalculator) (*Resolver, error)

func (r *Resolver) Resolve() (versioning.Result, error)
```

Inside `perenv/`:

- `auto.go` — listing tags via `tagfmt.GlobPattern`, sorting by the calculator's
  ordering, calling `BumpAuto` or `BumpFromDate` depending on `cfg.Strategy`.
- `promote.go` — resolving the source environment, extracting the bare version via
  `tagfmt.ParseVersion`, rendering under the destination format, and applying the
  E001/E002/E003 guards from [ADR-0007](0007-version-promotion-error-handling.md).

### Wiring

`internal/app/resolver.go::NewResolver` selects the right `VersionCalculator` based on
the strategy:

| Strategy             | VersionCalculator                                   |
|----------------------|-----------------------------------------------------|
| `semver-per-env`     | wraps `internal/versioning/semver.Resolver`         |
| `calver-per-env`     | wraps `internal/versioning/calver.Resolver`         |

The non-per-env strategies (`semver`, `calver`) bypass `perenv/` entirely and return the
single-env resolver directly.

## Consequences

**Positive**
- One implementation of: tag listing, source resolution, promote logic, E001/E002/E003
  checks, cycle detection. Bugs are fixed in one place.
- `tagfmt` is a tiny dependency-free package — extractable into a shared library later
  if other Go CLIs need it.
- New per-env strategies (e.g. a future `pep440-per-env`) require only implementing
  `VersionCalculator`, not duplicating the whole per-env machinery.

**Negative / trade-offs**
- One extra level of indirection at the resolver layer (the `VersionCalculator`
  interface) — a small cognitive cost compared to the duplication it removes.
- The interface has two methods that are mutually exclusive in practice (semver
  calculators never use `BumpFromDate`, calver calculators never use `BumpAuto`). An
  alternative shape — passing a single `Bump(tags, mode, commits) (string, error)`
  function — was rejected as less explicit about which axes are relevant per strategy.

## Test layout

`internal/versioning/perenv/resolver_test.go` is parameterised over the choice of
`VersionCalculator` so the full per-env test matrix runs against both the SemVer and
CalVer backends. Every per-env scenario — auto mode, promote mode, E001/E002/E003,
`source:` resolution, cycle detection — has coverage under both backends.
