# ADR-0011: Single-Pipeline Release via Version Pre-computation

- **Status**: Accepted
- **Date**: 2026-05-23
- **Deciders**: bchatard

---

## Context

`heraut release` runs as a single linear pipeline in one process. Every step needs to
agree on the version being released:

- The changelog generator (`git-cliff`, `cocogitto`) needs the resolved tag so unreleased
  commits get the right heading.
- The git tag command needs the resolved tag.
- Each platform driver (`gh release create`, `glab release create`) needs the resolved
  tag.
- The release-notes generator needs the resolved tag.

A naive implementation would let each step independently re-resolve "the next version"
by re-reading git tags. Two problems with that approach:

1. **Race conditions.** Between two resolutions, the git state can shift (an external
   tag push, a clock tick crossing a calendar boundary for CalVer). Different steps end
   up using different versions for the same release.
2. **Inconsistent dry-run.** In dry-run mode, the tag write is skipped — so a downstream
   step that re-resolves sees the *previous* tag as "latest", computes the same next
   version it would have, but cannot distinguish "this is the dry-run prediction" from
   "this is a fresh resolution after the previous step failed silently".

## Decision

**Pre-compute the version once at the start of the pipeline; thread it through every
subsequent step.**

```
Pipeline.Run():
   1. version := resolver.Resolve()        ← single source of truth for the run
   2. changelog content := generator.Generate(version.Tag)
   3. git commit + push
   4. git tag version.Tag + git push --tags
   5. for each platform: platform.CreateRelease(version.Tag, notes)
   6. for each platform: platform.UploadAssets(version.Tag)
   7. notes content := releaseNotesGenerator.Generate(version.Tag)
   8. for each platform: attach notes to release
```

No driver re-resolves. The resolved `version.Tag` is the only tag value passed around.
This applies to the dry-run path too: the resolver runs (read-only), the resolved tag is
printed, and every downstream step displays the actions it *would* take against that
same tag.

In `internal/pipeline/`:

- `pipeline.Config{...}` carries the resolved `versioning.Result`.
- `pipeline.New` takes the result; `Run()` does not call the resolver again.
- The `--version X.Y.Z` flag short-circuits step 1 — the user-provided value becomes
  the pipeline's truth without any resolver consultation.

## Consequences

**Positive**
- One pipeline run uses one version, deterministically.
- Dry-run mode prints exactly the actions that would happen with the resolved version
  — no second-guessing what "next" means.
- Each step is testable in isolation against a known tag string; no need to mock the
  resolver in pipeline tests.
- `--version X.Y.Z` is trivial to wire — it bypasses step 1 entirely.

**Negative / trade-offs**
- The version is resolved once *before* the changelog commit lands. If a parallel
  process pushes a tag for the same intended version between step 1 and step 4,
  step 4's `git tag` will fail. Acceptable: collision detection at write time is the
  correct safety net.
- Per-env promotion (E001/E002/E003 from
  [ADR-0007](0007-version-promotion-error-handling.md)) is checked at step 1, so the
  pipeline fails fast before any side effect.
