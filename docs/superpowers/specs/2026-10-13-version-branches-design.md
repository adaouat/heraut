# Version branches — the branch name is the version

- **Status**: Implemented
- **Date**: 2026-10-13
- **Author**: bchatard (with Claude)
- **Roadmap**: T348 (`docs/tasks/roadmap.md`, Phase 62)
- **Related ADRs**: 0065 (branch-aware SemVer resolution — amended, no new ADR), 0064 (SemVer v2
  and the pre-release lifecycle), 0018 (`--set-version` bypass)
- **Origin**: a mobile-project workflow. The client fixes the version up front, a `release/7.8.0`
  branch is cut, and `feat`/`fix` commits land there for stabilisation without ever changing the
  version. Store builds repeat (`-rc.N`, `+build`) until the final.

---

## Problem

Today this workflow only works by hand: `heraut release --set-version 7.8.0`, with `--pre-release rc`
or `--set-build-id` for repeat builds. `versioning.branches` already recognises `release/*` as a
maintenance glob, but a maintenance line derives a *range* (`1.3.x`) from the branch name and
deliberately refuses `N.N.N`, which names a release rather than a line. A `release/7.8.0` branch
under `release/*` therefore fails with `ErrUnderivableRange`.

GitVersion calls this a "release branch" (version read from the branch name); semantic-release has
no equivalent.

## Decisions

Settled with the user during brainstorming:

1. **Declaration by name shape, no new config key.** A glob entry (`release/*`) whose branch's last
   `/` segment is `N.N.N`, with one optional leading `v` (`release/7.8.0`, `release/v7.8.0`), yields
   a new match kind, `BranchVersion`, carrying that version. `N.x`, `N.N.x` and `N.N` still derive
   maintenance lines, so one `release/*` entry serves `release/1.3` (maintenance) and `release/7.8.0`
   (version). The input was an error before, so no working config changes meaning. Exact-name
   entries stay "release branch" (global resolution); a version branch needs a glob entry.
2. **A plain re-release errors.** When the tag for the branch's version already exists, a plain
   `heraut release` fails with a hint; it never auto-mints an rc or build.
3. **`--set-version` stays an explicit override** and may differ from the branch version. It prints
   a warning in that case (the same escape-hatch rule as maintenance branches).
4. **Ordering: warn, never block.** If a SemVer release higher than the branch version is already
   merged into `HEAD`, print a warning and continue. A higher tag that is *not* in the branch's
   history is legitimate (another line) and produces no output.

Scope: `versioning.strategy: semver` only, like `branches` today.

## Behaviour on a version branch (`release/7.8.0`)

| Invocation | Result |
|---|---|
| `heraut release`, no `v7.8.0` tag | cuts exactly `v7.8.0`; `feat`, `fix` and breaking commits never change it |
| `heraut release`, `v7.8.0` exists | error: tag exists; hint to pass `--pre-release rc` (only before the final) or `--set-build-id` |
| `--pre-release rc` | `v7.8.0-rc.1`, then `-rc.2`, …, using the existing counter logic over this core |
| `--pre-release rc`, final `v7.8.0` exists | error: a pre-release of a released version sorts below it |
| `--set-version 7.8.0 --set-build-id N` | `v7.8.0+N` (SemVer build metadata; `--set-build-id` requires `--set-version`) |
| `--set-version X` | X wins; a warning when X differs from the branch version; the existing collision guard applies |
| `heraut version next` / `version current` | report what a release would cut / the branch line's latest tag |

`--allow-major` and `stay_at_v0` are irrelevant here: nothing is bumped.

Changelog and release-notes bounds need no change. They already come from `HEAD`'s history and
skip pre-release tags (ADR-0065), so the final's notes span back to the previous release.

### Ordering warning

After the branch version is known, list `git tag -l <glob> --merged HEAD`, keep SemVer releases
(no pre-release), and if the highest outranks the branch version, add a `Result.Warnings` entry:
`branch version 7.7.0 is lower than v7.8.0, already in this branch's history`. The run continues. The
existing `warningResolver` path carries it to the UI.

## Design

- **`internal/config/branches.go`**: `DeriveBranchVersion(branch) (core string, ok bool)` — last
  segment, optional `v`/`V`, strict `N.N.N` (no leading zeros, the same digit rules as ranges).
- **`internal/app/branchrules.go`**: new `BranchVersion` kind and `BranchMatch.Version`. In the glob
  case, try `DeriveBranchRange` first, then `DeriveBranchVersion`; only when both fail does
  `needRange` raise `ErrUnderivableRange`, whose message drops the "N.N.N is unsupported" wording
  and lists the accepted shapes. `CheckReleaseBranch` treats `BranchVersion` as a listed branch.
- **`internal/versioning/semver`**: `Resolver.SetFixedVersion(core)`. `Resolve` with a fixed core
  skips bump computation: no pre-release label → the core itself, or `ErrTagExists` with the hint;
  with a label → `resolvePreRelease` over that core (`nextCounter`), erroring when the final tag
  exists. The ordering warning is produced here, from the same tag listing.
- **`internal/app/resolver.go`**: `applyMaintenanceRange` becomes `applyBranchRule` and calls
  `SetFixedVersion` for `BranchVersion`; the manual-bump-mode skip stays. `--set-version` already
  short-circuits before this point; a branch-version mismatch warning is added in that path when
  the branch matches `BranchVersion`.
- **Schema and docs**: no new key. `schema.json` rewords the `branches` description;
  `docs/heraut.sample.yml` and Spec 02 § `versioning.branches` document the third behaviour with a
  mixed `release/*` example; Spec 04/03 mention the new match kind and the error text change.
  ADR-0065 gains an amendment (Consequences and the entry-type table). The
  `docs/guides/maintenance-branches.md` fixed-version recipe replaces the manual `--set-version`
  steps.
- **Layering**: unchanged. `semver` stays under `versioning`, matching under `app`, derivation
  under `config`.

## Testing

- **Unit**: `DeriveBranchVersion` table (`7.8.0`, `v7.8.0`, `release/7.8.0`, `7.8`, `7.8.x`,
  `07.8.0`, `7.8.0-rc.1`, `7.8.0.1`); `MatchBranchRule` rows for the three shapes under one
  `release/*` entry, the ambiguity rules, and the `needRange=false` path.
- **Resolver**: first release; collision with hint; `rc.1` → `rc.2` → final; rc after final errors;
  `+build`; `--set-version` mismatch warning; ordering warning present for a merged higher tag and
  absent for an unmerged one.
- **Real git** (`maintenance_realrepo_internal_test.go`): a `release/7.8.0` branch off `main` with
  a higher unmerged tag elsewhere (silent) and a higher merged tag (warns).
- **E2E (Lane A)**: one scenario on the built binary — branch `release/7.8.0`, release, re-release
  error, rc, final.

## Out of scope

- Other strategies (`calver`, per-env).
- Any ordering *error*, or a check against tags outside the branch's history.
- Detecting that commits imply a larger bump than the branch version (a possible future warning).
- Auto-minting a pre-release or build on re-release.
