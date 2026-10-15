# ADR-0073: `--set-version` always probes for an existing tag

- **Status**: Accepted
- **Date**: 2026-10-15
- **Deciders**: bchatard

---

## Context

`--set-version` bypasses bump resolution, so nothing in the resolver looks at git. ADR-0065 added a
tag-collision probe to it, but only for `semver` repos that declare `versioning.branches`. Every other
`--set-version` run (plain `semver`, `calver`, both per-env strategies) reached the tag step with a
version nobody had checked. `heraut release` then committed and pushed the changelog before `git tag`
failed on the duplicate, leaving a pushed commit with no tag. The hardening review recorded this as
RULE-046 and the maintainer ruled that the probe should always run.

## Decision

1. **Every run that tags probes.** `heraut release` and `heraut changelog --tag` (and, as before, a run
   with `--commit`) probe once during resolution. A taken tag fails with `tag already exists` (Runtime),
   before the changelog is written or anything is pushed. The probe is `git tag -l <tag> <tag>+*` under
   the SemVer strategies (build metadata is not a different release, ADR-0064) and an exact
   `git tag -l <tag>` under the CalVer ones.
2. **`--force` keeps its per-env meaning.** Under `semver-per-env` and `calver-per-env` it already
   means "go on although the candidate tag exists" (E001), so it skips the probe there. Under `semver`
   and `calver` it never did and still does not.
3. **`version next` is the exception.** It prints a tag and changes nothing, and is sometimes run where
   there is no repository, so it probes only where it always did: `semver` with a `versioning.branches`
   block. `heraut changelog` without `--tag` also skips the probe, as in ADR-0065: re-rendering the
   changelog of a released version is not a collision.
4. The branch/version mismatch check of ADR-0065 is unchanged and still needs a `branches` block.

## Consequences

- A `--set-version` run needs a readable git repository, as the rest of `release` already does. The
  probe also works in a shallow clone, where it can only miss tags, never invent one.
- Tests that stood for "`--set-version` makes no git call" now say which part of that still holds: the
  static resolver itself makes none (they run with the guard off), and the guard is covered by its own
  table. `TestNewResolver_SetVersion_CollisionGuard` lost no row; the "no branches block" row now
  expects the probe.
- Spec 03 and Spec 04 state the rule.
