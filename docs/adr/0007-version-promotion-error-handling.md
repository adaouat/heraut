# ADR-0007: Version Promotion Error Handling (E001 / E002 / E003)

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

The `semver-per-env` and `calver-per-env` strategies support a `bump: promote` mode
where a version from one environment (e.g. `dev/1.0.2`) is copied to another
(e.g. `prod/1.0.2`). Three edge cases can produce contradictory or unsafe states during
promotion. A decision was needed on how the tool should behave in each case.

## Decision

**Hard-fail on all three edge cases.** No silent degradation, no automatic recovery.
Provide rich, actionable error messages (Biome-style: what was found, why it is wrong,
how to fix it). Offer a `--force` flag as an explicit escape hatch that must be passed
consciously.

The three errors are wired as sentinel error values (`ErrTargetExists`,
`ErrDestinationAhead`, `ErrNoSourceTags`) in `internal/versioning/perenv/` so callers can
detect them via `errors.Is`. They map to exit code 4 (see
[Spec 01 — Exit codes](../specs/01-overview.md#exit-codes)): the cmd layer recognises them
through `app.IsPromotionGuard` and wraps them with `exitcode.Promotion`, which
`cmd/heraut` resolves to `4` (T27).

### Edge case A — target tag already exists (E001)

Tags are immutable. A duplicate tag would overwrite history or be rejected by the
remote. Fail immediately without attempting any write operation.

```
error[E001]: tag already exists

  Promoting dev/1.0.2 would create prod/1.0.2, but that tag already exists.

  Found:
    latest dev tag  →  dev/1.0.2  (promotion candidate)
    prod/1.0.2      →  already exists ✗

  Tags are immutable. Creating a duplicate tag overwrites history.

  How to fix:
    · If this version was already promoted, no action is needed.
    · If you need to re-release, remove the existing tag first:
        git tag -d prod/1.0.2
        git push origin :refs/tags/prod/1.0.2
    · To bypass this check: heraut release --env prod --force
```

### Edge case B — destination is ahead of the candidate (E002)

Promoting an older source version when the destination already has a higher version
would cause a regression. This can happen when a hotfix is tagged directly on the
destination environment without a corresponding source tag.

```
error[E002]: version regression detected

  Promoting dev/1.0.2 would create prod/1.0.2, but prod/1.0.3 already exists.
  This would move prod backwards.

  Found:
    latest dev tag   →  dev/1.0.2  (promotion candidate)
    latest prod tag  →  prod/1.0.3 (higher than candidate ✗)

  How to fix:
    · Check whether a hotfix was applied directly to prod without a dev tag.
      If so, create the corresponding dev tag first:
        git tag dev/1.0.3 <commit-sha>
        git push origin dev/1.0.3
    · Or promote from a newer dev version once one exists.
    · To bypass this check: heraut release --env prod --force
```

### Edge case C — no source tags exist (E003)

`bump: promote` has nothing to resolve from. Fail with an educational message
explaining the required workflow.

```
error[E003]: no source tags found

  Cannot promote to prod: no dev/* tags exist in this repository.

  bump: promote requires at least one release in the source environment before
  promoting.

  How to fix:
    1. Create a source release first:
         heraut release --env dev
    2. Then promote to the destination:
         heraut release --env prod

  Note: --force cannot bypass this error (there is no version to promote).
```

> Note: `--force` is explicitly **not available** for edge case C — there is no version
> to resolve, so there is nothing to force through.

### Force matrix

| Code | Bypassed by `--force`?    |
|------|---------------------------|
| E001 | Yes                       |
| E002 | Yes                       |
| E003 | No                        |

> **Implementation status**: the sentinel errors, the `--force` bypass matrix above, and
> the SemVer/CalVer regression check (E002) are implemented. The rich multi-line message
> format shown in the three examples above is **not yet implemented** — the current
> errors are concise single-line messages (e.g. `E001: target tag already exists: tag
> "prod/1.0.2" already exists (pass --force to bypass)`). The Biome-style format remains
> the target and is tracked by roadmap **T30**.

## Consequences

**Positive**
- Prevents silent corruption of the release history.
- Rich error messages reduce back-and-forth debugging; users know exactly what to do.
- `--force` provides an escape hatch for exceptional situations without making it easy
  to reach by accident (must be an explicit flag).
- Edge case C's error doubles as onboarding documentation for the per-env workflow.

**Negative / trade-offs**
- More implementation work than a simple "let it fail downstream" approach.
- Edge case B requires the tool to fetch and compare tags across two namespaces — adds
  a git operation to the happy path (acceptable cost).
- The check applies symmetrically to SemVer and CalVer. A single comparator
  (`compareVersionStrings` in `promote.go`) handles both: it compares dot-separated
  integer components, which works for SemVer (`1.2.3`) and CalVer (`2026.05.3`) alike, so
  no per-strategy comparator is needed and the error message text is identical.
