# ADR-0082: "Nothing to release" exits 5

- **Status**: Accepted
- **Date**: 2026-11-08
- **Deciders**: bchatard

---

## Context

When there is nothing to release, heraut stopped with exit 3 (`runtime`), the code of a real
failure: no commits since the last release tag, commits that are all non-releasable (`Update readme`,
or types `versioning.bump.overrides` set to `none`), or `heraut version current` with no release tag
yet. A CI job that runs `heraut release` on every push to `main` went red after every release commit,
and a script could not tell "nothing to do" from "git failed".

### Options

1. **A new exit code, 5, and `error.code: "nothing_to_release"`.** Still non-zero, since nothing was
   released, but distinct.
2. **Exit 0 with a notice.** Easiest in CI, but `heraut version next` would print no version and
   succeed, which misleads a script reading its output.
3. **Keep exit 3, add the `error.code` only.** Scripts without `--json` still can't tell.

## Decision

Option 1. Exit code **5** means "nothing to release", in every command that resolves a version
(`release`, `changelog`, `version next`) and in `version current` when no release tag exists yet. In
`--json` output, `error.code` is `nothing_to_release`. The message is unchanged and names the case.

The conditions share one sentinel, `versioning.ErrNothingToRelease`, matched with `errors.Is`; the
errors that carry it keep their own text and identity.

## Consequences

- A CI job that releases on every push accepts it in one line, e.g.
  `heraut release || [ $? -eq 5 ]`; the exit-codes reference gives the recipe.
- Exit codes 0–5 and 70 are now in use; 5 is part of ADR-0080's stable surface.
- A CalVer release with no new commit (T510) and a pre-release that would sort below an existing one
  are separate cases, decided on their own.
