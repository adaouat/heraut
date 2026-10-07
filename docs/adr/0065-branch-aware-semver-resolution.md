# ADR-0065: Branch-aware SemVer resolution

- **Status**: Accepted
- **Date**: 2026-10-07
- **Deciders**: bchatard
- **Design doc**: [`docs/superpowers/specs/2026-10-05-maintenance-branches-design.md`](../superpowers/specs/2026-10-05-maintenance-branches-design.md)

---

## Context

heraut resolves every SemVer surface from the whole repository's tag list, not the current
branch's history. Take a repo where `main` has `v1.3.0 → v1.3.1 → v1.4.0 → v2.0.0` and
`release/1.3` was cut from `v1.3.1`:

- **Version.** A `fix:` on `release/1.3` bases on `v2.0.0` and releases `v2.0.1`, not `v1.3.2`,
  and `git log v2.0.0..HEAD` picks up every branch commit since the fork, including fixes already
  shipped in `v1.3.1`.
- **`version current`** on `release/1.3` reports `v2.0.0`.
- **Release notes** of a final pick the previous tag by precedence only, so it can be a tag that
  was never in the release's history.
- **Changelog.** `--regenerate` on `main` walks every tag by precedence. A `v1.3.2` cut on
  `release/1.3` and never merged into `main` gets its own section in `main`'s `CHANGELOG.md` and
  becomes `v1.4.0`'s lower bound, giving a range that drops commits `v1.4.0` actually contains.

Pre-releases already avoided this: since ADR-0064 Phase 2 they resolve their previous tag from
`git tag --merged HEAD`. ADR-0064 filed the general problem as T344.

Switching *version resolution* to reachable tags unconditionally is not safe. heraut cannot tell a
maintenance branch from a repo whose release tags live on branches that are never merged back
(`main` would then see only old tags and could silently mint a wrong, possibly lower, version), or
from a shallow CI clone (every tag outside the fetched depth disappears and resolution falls back
to `initial_version`).

### Prior art

Both tools below declare maintenance lines through **branch name plus config** and never infer
them from tag placement. That is the core of this decision.

- **semantic-release**: [workflow configuration, maintenance branches](https://semantic-release.gitbook.io/semantic-release/usage/workflow-configuration)
  and the [maintenance releases recipe](https://semantic-release.gitbook.io/semantic-release/recipes/release-workflow/maintenance-releases).
  A `branches` list; a branch named `N.x` / `N.N.x` is a maintenance branch whose name becomes its
  range (`1.0.x` is `>=1.0.0 <1.1.0`, fixes only; `1.x` allows features up to the next major). A
  commit pushing the version out of range fails with `EINVALIDNEXTVERSION`. Ranges are unique, and
  it refuses to publish from unlisted branches.
- **GitVersion**: [configuration reference](https://gitversion.net/docs/reference/configuration).
  Branches matched by `regex`, each type (`main`, `release`, `hotfix`, `support`) with its own
  `increment`; `version-in-branch-pattern` reads the version from the branch name. `support`
  branches are its long-lived maintenance lines.

## Decision

**Declare maintenance lines in config (`versioning.branches`) and resolve against the branch's own
history only for branches so declared; make changelog and release-notes bounds history-aware
everywhere.**

**Model.** `versioning.branches` is an ordered list of `{name, range?}` (`semver` only; a config
error under any other strategy). `name` is an exact branch name or a `path.Match` glob; `range` is
`N.x` (`>=N.0.0 <(N+1).0.0`) or `N.N.x` (`>=N.M.0 <N.(M+1).0`). Explicit ranges are unique. The
entry decides the type, not the branch:

| Entry | Type |
|---|---|
| has `range` | maintenance branch, that range |
| glob `name`, no `range` | maintenance branch, range derived from the matched branch name |
| exact `name`, no `range` | release branch (today's behaviour) |

Derivation takes the branch name's last `/`-separated segment and accepts `N.x`, `N.N.x` or `N.N`
with an optional leading `v` (`release/1.3` and `release/1.3.x` give `1.3.x`). An exact branch
literally named `1.3.x` is a release branch unless it sets `range:`: explicit beats magic. The
current branch comes from `git rev-parse --abbrev-ref HEAD`, then on a detached `HEAD` from
`CI_COMMIT_BRANCH`, `GITHUB_REF_NAME` (branch refs only) and `BUILD_SOURCEBRANCHNAME`. Exactly one
entry may match; two is an error.

**Resolution by branch type.**

| Branch | `release`, `changelog --tag` | `version next`, `version current`, `changelog` (no tag) |
|---|---|---|
| no `branches` block | today | today |
| release branch | today (global tags) | today |
| maintenance branch | maintenance rules | maintenance rules, so previews tell the truth |
| unlisted, or unknown (detached, no CI variable) | error; `--force` bypasses it, like the per-env branch guard | today |

**Maintenance rules.** Given the range `[lo, hi)`: the base is the highest-precedence *final* tag
in `git tag -l <prefix>* --merged HEAD` whose core lies in the range (none is an error);
commits are `git log <base>..HEAD`; the normal bump rules apply, `stay_at_v0` first; the result
must lie in the range (`ErrOutOfRange`); `git tag -l <next>` guards against a collision with a tag
cut elsewhere (`ErrTagExists`). `version current` reports the same base. `--pre-release` is
allowed on a maintenance branch, the core subject to the same range check.

**Why hard errors.** A `feat:` on a patch-only line, a version that already exists, and a line
with no in-range release all have a wrong "closest" answer that would silently ship a version
outside the line. A clear error naming the range and the way out (land the commit on a branch
whose range allows it, tag the starting point, pass `--set-version`) costs one failed run; a
silent wrong tag costs a published release. Unlisted branches are refused only on publishing
commands, so previews keep working anywhere.

**History-aware changelog and notes bounds, with or without `branches`.** Whenever a tag order is
set (`semver`, `semver-per-env`):

1. Changelog and release notes always list tags with `--merged HEAD`: a tag never merged into
   `HEAD` gets no section.
2. Each existing section's lower bound comes from its own ancestry
   (`git tag -l --merged <t> --no-contains <t>`), so after `release/1.3` is merged forward into
   `main`, `v1.3.2` gets its own section (`v1.3.1..v1.3.2`) and `v1.4.0` still bounds at `v1.3.1`.
3. The tag being cut does not exist yet: its bound stays the insert-into-the-ordered-list rule
   from ADR-0064 (T341).

This is the one behaviour change that reaches configs without a `branches` block: a bound outside
the bounded tag's history is always wrong, whatever the branch layout. A repo whose tags are all
ancestors of each other (linear history, the common case) renders byte-identical output; only a
repo with tags outside a section's history sees different sections or bounds, and those were wrong
before. Version resolution, by contrast, changes nothing without the block. CalVer is untouched.

**`--set-version` stays the manual escape hatch.** It is not range-checked, and a matched glob
entry whose range cannot be derived from the branch name is not an error under it. This keeps the
"branch is the version" workflow working, a client-fixed `release/7.8.0` branch released with
`--set-version 7.8.0`, in a repo that also declares `release/*` as maintenance branches.
Automating it is filed separately (T348).

**Clarifications settled during implementation (T349-T354).** Where these differ from the design
doc, they govern.

- **`--set-version` collisions.** The override path returns a static resolver and makes no git
  call, so the "collision guard still applies" rule is satisfied by the existing `git tag` failure
  when the tag exists. No new probe.
- **`--dry-run` on an unlisted branch is not refused.** It is a preview; this mirrors the per-env
  branch guard, which also runs only outside dry runs.
- **Per-section bounds are scope-preserving.** A section's lower bound is the highest-precedence
  tag that is both in the section's scoped tag list and an ancestor of the section's tag. Only
  when no in-scope ancestor exists does the existing oldest-in-scope fallback (unscoped
  ancestors) apply. Without this, under `semver-per-env` a `uat/1.3.0` ancestor could bound
  `prod/1.3.0`.
- **Exit codes.** Branch-rule errors raised while building the resolver (ambiguous match,
  underivable range) and the unlisted-branch refusal exit Config. Resolve-time errors (no
  in-range base, out of range, tag exists) exit Runtime.
- **`--pre-release` needs a derivable range.** On a glob-matched branch with no derivable range it
  fails with `ErrUnderivableRange`: auto resolution needs the range, and only `--set-version` is
  exempt.
- **Escalation looks at the line's own series.** With a range set, pre-release escalation
  considers only the line's own open pre-release series, because a higher series on `main`
  (`v2.1.0-rc.1`) must not hide a real escalation on the line (`v1.4.1-rc.1` to `v1.5.0`). The
  counter and per-core monotonicity stay global (ADR-0064), since a maintenance core is distinct
  from every core on `main`.
- **`commit check --from-latest-tag` follows the line.** It uses the branch-aware current tag, but
  on an underivable or ambiguous branch it falls back to the global latest tag: commit linting must
  keep working on, say, a client-fixed `release/7.8.0` branch. `version current` keeps erroring
  there.
- **Forward merges list the fix twice.** After a maintenance branch is merged forward into `main`,
  a later `main` section legitimately also lists the merged-in fix: it is in that section's
  history.

## Consequences

- Releasing from a declared maintenance branch produces the next version on that line, with notes
  and changelog bounded by tags in the branch's history. A commit that does not fit the line fails
  loudly instead of releasing outside it.
- No existing config changes version resolution: without `versioning.branches`, `NewResolver`
  makes no branch-detection call and every existing call sequence is unchanged.
- Changelog and notes bounds change for semver repos with tags outside a section's history, as
  described above. Contract tests whose git call sequences gain `--merged HEAD` were edited in
  place, not deleted.
- Forward-merge and backport stay a git workflow; heraut neither merges nor cherry-picks.
- The branch name is part of the contract: a `release/*` glob needs names carrying the version
  (`release/1.3`), or `--set-version`.
- Shallow clones need the history of the line's base tag; a base missing from the fetched depth
  is reported as "no release in range", not silently replaced.
- CalVer's changelog bounds keep their ancestry-blind behaviour. The CalVer equivalent of the
  history-aware bounds is filed as a follow-up (its zero-padded tags need their own ordering for
  the ancestor pool).

## Alternatives considered

- **Reachable tags everywhere, plus a "tag exists" guard, no config.** Rejected: silently wrong
  for repos whose release tags are never merged back (`main` would resolve from stale tags and
  could mint a lower version) and for shallow clones (tags outside the depth vanish and resolution
  falls back to `initial_version`). heraut cannot distinguish those layouts from a real
  maintenance branch without being told.
- **A per-run `--maintenance` flag.** Rejected: forgetting it reproduces today's bug, and it
  carries no range cap, so nothing stops a `feat:` from leaving the line.
- **Inferring the line from the branch name alone, with no config.** Rejected for the same reason
  as the first: explicit declaration is what both reference tools do, and it is what makes an
  unlisted branch detectable.
- **A "version branch" type, where the version comes from the branch name** (`release/7.8.0`
  gives `7.8.0`, commits never bump it). Deferred, not rejected: today's `--set-version` covers it
  manually (T348).
- **Per-env strategies.** Out of scope: they already tie environments to branches
  (`environments.<env>.branch`), so `branches` is a config error under them.
