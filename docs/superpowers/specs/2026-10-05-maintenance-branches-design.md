# Maintenance branches: branch-aware SemVer resolution

- **Status**: Approved (design), pending implementation plan
- **Date**: 2026-10-05
- **Author**: bchatard (with Claude)
- **Related ADRs**: 0064 (SemVer v2 compliance: §11 order, pre-release `--merged HEAD` rule, the
  section that filed T344), 0063 (`stay_at_v0`), 0038 (incremental changelog)
- **New ADR required**: yes, the next available number (0065 at the time of writing):
  "Branch-aware SemVer resolution."
- **Roadmap**: T344 in `docs/tasks/semver-v2-roadmap.md`, split into sub-tasks under a new
  "Phase 3: Maintenance branches" during implementation planning.
- **Origin**: T344, filed by ADR-0064's Phase 2 status update. The user has a concrete workflow:
  long-lived `release/x.y` branches cut from `main`, with hotfixes tagged there while `main`
  moves on.

---

## Problem

heraut resolves every SemVer surface from the whole repository's tag list, not the current
branch's history. Take a repo where `main` has `v1.3.0 → v1.3.1 → v1.4.0 → v2.0.0` and
`release/1.3` was cut from `v1.3.1`:

- **Version.** `semver.Resolver.resolveAuto` lists `git tag -l v*` globally, so a `fix:` on
  `release/1.3` bases on `v2.0.0` and releases `v2.0.1`, not `v1.3.2`. `git log v2.0.0..HEAD`
  also picks up every branch commit since the fork, including fixes already shipped in `v1.3.1`.
- **`version current`** on `release/1.3` reports `v2.0.0`.
- **Release notes** of a final pick the previous tag by precedence only, so it can be a tag that
  was never on the release's branch.
- **Changelog.** `--regenerate` on `main` walks every tag by precedence. A `v1.3.2` cut on
  `release/1.3` and never merged into `main` gets its own section in `main`'s `CHANGELOG.md`
  and becomes `v1.4.0`'s lower bound, giving a `git log v1.3.2..v1.4.0` range that drops
  commits `v1.4.0` actually contains.

Pre-releases already avoid this: since ADR-0064 Phase 2 they resolve their previous tag from
`git tag --merged HEAD`.

Switching *version resolution* to reachable tags unconditionally is not safe. heraut cannot
tell a maintenance branch from a repo whose release tags live on branches that are never merged
back (`main` would then see only old tags and could silently mint a wrong, possibly lower,
version), or from a shallow CI clone (every tag outside the fetched depth disappears and
resolution falls back to `initial_version`).

## Goals

1. Releasing from a declared maintenance branch produces the next version *on that line*
   (`v1.3.2`), with release notes and changelog sections bounded by tags in the branch's history.
2. A commit that does not fit the line (a `feat:` on a patch-only line) fails loudly, naming the
   range, instead of releasing a version outside it.
3. No existing config changes behaviour for version resolution: without the new config block,
   every branch resolves exactly as today.
4. Changelog and release-notes bounds never use a tag outside the history of the tag they bound,
   on any branch.

## Non-goals

- Per-env strategies (`semver-per-env`, `calver-per-env`): they already tie environments to
  branches (`environments.<env>.branch`). The new block is a config error under them.
- CalVer: it has no version line to cap. CalVer's changelog bounds also stay byte-for-byte
  unchanged (as in T334); the same ancestry fix for CalVer is filed as a follow-up roadmap task.
- Pre-release *branches* or distribution channels (semantic-release's `prerelease:` /
  `channel:`): the config shape leaves room for them; none are added now.
- Automatic forward-merging of fixes or cherry-picking of backports: that stays a git workflow.
- Range syntax beyond `N.x` and `N.N.x` (no `>=1.3.0 <1.5.0` expressions).

## Prior art

Both tools below declare maintenance lines through **branch name plus config** and never infer
them from tag placement. That is the core of this design.

- **semantic-release** —
  [workflow configuration § Maintenance branches](https://semantic-release.gitbook.io/semantic-release/usage/workflow-configuration),
  [maintenance releases recipe](https://semantic-release.gitbook.io/semantic-release/recipes/release-workflow/maintenance-releases).
  A `branches` list; a branch named `N.x` / `N.N.x` is a maintenance branch whose name becomes its
  range (`1.0.x` → `>=1.0.0 <1.1.0`, fixes only; `1.x` → fixes and features up to the next
  major). A commit pushing the version out of range fails with `EINVALIDNEXTVERSION`, naming the
  branch the commit belongs on. Ranges must be unique. It refuses to publish from unlisted
  branches.
- **GitVersion** —
  [configuration reference](https://gitversion.net/docs/reference/configuration). Branches
  matched by `regex`, each type (`main`, `release`, `hotfix`, `support`) with its own
  `increment`; `version-in-branch-pattern` reads the version from the branch name
  (`release/1.3` → `1.3`). `support` branches are its long-lived maintenance lines.

Alternatives rejected for heraut (recorded in the ADR):

- **Reachable tags everywhere, plus a "tag exists" guard, no config.** Silently wrong for
  tags-never-merged-back repos and shallow clones.
- **A per-run `--maintenance` flag.** Forgetting it reproduces today's bug; no range cap.

## Design

### 1. Configuration

```yaml
versioning:
  strategy: semver
  branches:
    - name: main                 # release branch: today's behaviour
    - name: release/1.3          # maintenance branch, explicit range
      range: 1.3.x               # >=1.3.0 <1.4.0 (patch only)
    - name: "release/*"          # maintenance branch, range derived per matched branch
```

`versioning.branches` is an ordered list of `{name, range?}` (`config.BranchRule`, wire names
`name` / `range`):

- **`name`**: an exact branch name, or a glob in `path.Match` syntax (`*` does not cross `/`).
- **`range`**: `N.x` (`>=N.0.0 <(N+1).0.0`, patch and minor bumps) or `N.N.x`
  (`>=N.M.0 <N.(M+1).0`, patch only).

Entry type is determined by the entry, not the branch:

| Entry | Type |
|---|---|
| has `range` | maintenance branch, that range |
| glob `name`, no `range` | maintenance branch, range **derived** from the matched branch name |
| exact `name`, no `range` | release branch (today's behaviour) |

Range derivation takes the branch name's last `/`-separated segment and accepts `N.x`, `N.N.x`
or `N.N` (an optional leading `v` is allowed): `release/1.3` and `release/1.3.x` → `1.3.x`;
`release/1.x` → `1.x`. A matched branch with no derivable version (`release/legacy` against
`release/*`) is a runtime error naming the branch and the entry. An exact branch literally named
`1.3.x` is a *release* branch unless it sets `range:` — explicit beats magic.

**Load-time validation** (`internal/config/validator.go`, `ValidationErrors` with hints):

- `versioning.branches` is only allowed under `strategy: semver`.
- Every entry has a non-empty `name`; globs must compile under `path.Match`.
- `range` must parse as `N.x` or `N.N.x`.
- Explicit ranges are unique across entries.
- Unknown keys inside an entry are rejected by the existing strict loader.

**Absent block:** every branch behaves exactly as today. No existing config changes behaviour
for version resolution.

### 2. Current-branch detection

A new `app`-layer helper, `CurrentBranch(runner) (string, bool, error)`:

1. `git rev-parse --abbrev-ref HEAD`; a value other than `HEAD` is the branch.
2. On a detached `HEAD`, fall back in order to `CI_COMMIT_BRANCH` (GitLab, set on branch
   pipelines only), `GITHUB_REF_NAME` when `GITHUB_REF_TYPE=branch` (GitHub Actions), then
   `BUILD_SOURCEBRANCH` with its `refs/heads/` prefix stripped, branch refs only (Azure Pipelines;
   `BUILD_SOURCEBRANCHNAME` is only the ref's last path segment, so it is not read).
3. None available → "unknown" (`ok == false`), not an error; the caller decides.

It runs only when `versioning.branches` is set. `CheckBranch` and `ResolveEnv` keep their own
behaviour (they are per-env features, out of scope).

**Matching:** the detected branch is matched against every entry. Exactly one match → that
entry. More than one → runtime error listing the entries (the same shape as `--env auto`'s
ambiguity error). None → unlisted.

### 3. Resolution by branch type

| Branch | `release`, `changelog --tag` | `version next`, `version current`, `changelog` (no tag) |
|---|---|---|
| no `branches` block | today | today |
| release branch | today (global tags) | today |
| maintenance branch | maintenance rules (§4) | maintenance rules (§4), so previews tell the truth |
| unlisted, or unknown (detached, no CI variable) | **error**: `branch "feature/x" matches no versioning.branches entry` with a hint listing the entries; `--force` bypasses it, like the per-env branch guard | today |

`--force` already exists on `release` and `changelog`; it gains this meaning there, documented
next to its existing per-env one.

### 4. Maintenance rules

Given the matched range `[lo, hi)`:

- **Base.** The highest-precedence *final* tag in `git tag -l <prefix>* --merged HEAD
  --sort=-version:refname` (ordered by `semver.SortTags`, as today) whose core lies in
  `[lo, hi)`. None → error: `no release in range 1.3.x in the history of release/1.3` with a hint
  to tag the branch's starting point or set `--set-version`.
- **Commits.** `git log <base>..HEAD`, as today but from the reachable base.
- **Bump.** The normal rules, including `stay_at_v0` first. The resulting version must lie in
  `[lo, hi)`; otherwise error `ErrOutOfRange`: `feat: … would release 1.4.0, outside
  release/1.3 (>=1.3.0 <1.4.0)` with the hint "land it on a branch whose range allows it, or on
  main". Runtime exit code, like the other resolution errors.
- **Collision guard.** `git tag -l <next-tag>`; a non-empty result → error naming the existing tag
  (it was cut on another branch). Runtime exit code.
- **`version current`** reports the same base.
- **`--pre-release <label>`** is allowed: the core must lie in `[lo, hi)` (same `ErrOutOfRange`),
  and ADR-0064's previous-tag rule (`--merged HEAD`) already fits. `release/1.3` +
  `--pre-release rc` → `v1.3.2-rc.1`. Global per-core monotonicity is unaffected: a maintenance
  core is distinct from every core on `main`.
- **`--set-version`** stays the manual escape hatch: not range-checked; the collision guard still
  applies. Because no range is needed, a matched glob entry whose range cannot be derived from
  the branch name is **not** an error under `--set-version` (§1's derivation error applies only
  to auto resolution). This keeps the "branch is the version" workflow — a client-fixed
  `release/7.8.0` branch released with `--set-version 7.8.0` — working in a repo that also
  declares `release/*` maintenance branches. Automating that workflow is a separate task (see
  Delivery).

The maintenance path is a new resolver mode (`semver.Resolver.SetMaintenanceRange(lo, hi)`),
wired by `internal/app` exactly like `SetAllowMajor` / `SetPreRelease`: `internal/cmd` never sees
the branch logic.

### 5. History-aware changelog and notes bounds

Applies whenever `app.tagOrderFor` returns an order, i.e. `semver` and `semver-per-env`, with
or without a `branches` block, because a bound outside the bounded tag's history is always wrong:

1. **The walk sees only history.** Changelog and release notes always get
   `native.WithReachableFromHead()` (today: pre-release notes only). A tag never merged into
   `HEAD` gets no section.
2. **Each section's lower bound comes from its own ancestry.** The previous tag of an *existing*
   tag `t` is the first entry of `tagOrder(listMergedTags(t))` — the rule T334's oldest-in-scope
   fallback already uses, now for every section, replacing `previousInList` under a tag order.
   After `release/1.3` is merged forward into `main`, `v1.3.2` gets its own section
   (`v1.3.1..v1.3.2`) and `v1.4.0` still bounds at `v1.3.1`.
3. **The tag being cut** does not exist yet: its bound stays T341's rule (insert it into the
   reachable, ordered list, take the next lower entry).

Cost: one `git tag -l --merged <t> --no-contains <t>` per section, only on a `--regenerate` walk;
an incremental run adds one call.

This is the one behaviour change that reaches repos *without* a `branches` block: a semver repo
whose tags are all ancestors of each other (linear history, the common case) renders
byte-identical output; only a repo with tags outside a section's history sees different
sections or bounds, and those were wrong before. The ADR states this explicitly.

### 6. Error handling summary

| Condition | When | Exit |
|---|---|---|
| `branches` under a non-`semver` strategy; bad range; duplicate range; bad glob | load | Config |
| two entries match the branch | run | Config |
| matched glob entry, no derivable version in branch name (auto resolution only; not with `--set-version`) | run | Config |
| unlisted or unknown branch on a publishing command, no `--force` | run | Config |
| no in-range final reachable from HEAD | run | Runtime |
| next version outside the range (`ErrOutOfRange`) | run | Runtime |
| next tag already exists | run | Runtime |

All errors wrap with `%w`; new sentinels (`ErrOutOfRange`, `ErrTagExists`, `ErrUnlistedBranch`)
live at the package boundary that raises them, checked with `errors.Is`.

## Testing

TDD at every layer; every new test must fail before its implementation.

- **Unit.** Range parsing (`1.x`, `1.3.x`, bad forms) and derivation (`release/1.3`,
  `release/1.3.x`, `release/v1.x`, `release/legacy` → error). Config validation (strategy gate,
  duplicate ranges, bad syntax, bad glob) with fixtures in `testdata/config/valid/` and
  `testdata/config/invalid/`, and the schema layer checking the same fixtures. Branch detection
  with `t.Setenv` and the CI-env-clearing helpers: attached branch, detached + each CI variable,
  detached + none. Branch matching: one, none, ambiguous.
- **Contract (`MockRunner`).** Exact git argv: `git tag -l v* --merged HEAD
  --sort=-version:refname` for the maintenance base, `git tag -l v1.3.2` for the collision probe,
  the per-section `git tag -l --merged <t> --no-contains <t> --sort=-version:refname`, and that a
  repo without `branches` issues today's exact call sequence.
- **Integration (real git, `RealGitRepo`).** Replay the worked examples below verbatim, plus
  `--regenerate` on `main` with an unmerged maintenance tag (no section) and a merged-forward one
  (own section, correct bounds). Each must be shown failing against the pre-change code first.
- **Preserved edge cases.** Every existing T334/T341 row stays; any row whose assertion this
  design deliberately changes is edited in place and listed in the task note, per ADR-0065.

### Worked examples (integration fixtures)

History: `main` has `v1.3.0 → v1.3.1 → v1.4.0 → v2.0.0`; `release/1.3` cut from `v1.3.1`;
`release/1.x` cut from `v1.4.0`. Config: `branches: [{name: main}, {name: "release/*"}]`.

| # | Branch / commit | Today | Designed |
|---|---|---|---|
| 1 | `release/1.3`, `fix: x` | `v2.0.1` | `v1.3.2`, notes `v1.3.1..v1.3.2` |
| 2 | `release/1.3`, `feat: y` | `v2.1.0` | `ErrOutOfRange` (1.4.0 outside `1.3.x`) |
| 3 | `release/1.x`, `feat: z` | `v2.1.0` | `v1.5.0` |
| 4 | `release/1.x`, `feat!: …` | `v3.0.0` | `ErrOutOfRange` (2.0.0 outside `1.x`) |
| 5 | `main`, `fix: w`, with `v1.3.2` on `release/1.3` | `v2.0.1` | `v2.0.1` (unchanged) |
| 6 | `release/1.3`, `v1.3.2` already tagged elsewhere | collides | tag-exists error |
| 7 | detached HEAD, `CI_COMMIT_BRANCH=release/1.3` | — | as #1 |
| 8 | `feature/foo`, `heraut release` | `v2.0.1` | unlisted-branch error; `--force` → `v2.0.1` |
| 8b | `feature/foo`, `heraut version next` | `v2.0.1` | `v2.0.1` (unchanged) |
| 9 | `release/1.3`, `heraut version current` | `v2.0.0` | `v1.3.1` |
| 10 | `release/1.3`, `--pre-release rc` | — | `v1.3.2-rc.1` |
| 11 | `release/7.8.0` (matches `release/*`, no derivable range), `--set-version 7.8.0` | `v7.8.0` | `v7.8.0` (no derivation error) |
| 11b | `release/7.8.0`, no `--set-version` | `v2.0.1` | derivation error naming the branch |

## Documentation

- **ADR-0065** "Branch-aware SemVer resolution": the model, prior art with links, the branch-type
  table, why hard errors, history-aware bounds, rejected alternatives. ADR-0064's status section
  gets a one-line pointer (it filed T344).
- `schema.json` (`versioning.branches`, `BranchRule`), `docs/heraut.sample.yml` (commented
  example), Spec 02 (config reference), Spec 04 (versioning: maintenance rules, error table),
  Spec 03 (`--force`'s new meaning on `release` / `changelog`), Spec 05 (changelog/notes bounds).
- A `docs/guides/` how-to, "Maintenance branches", built around the worked-examples table.

## Delivery

T344 is split into sub-tasks during planning, filed under "Phase 3: Maintenance branches" in
`docs/tasks/semver-v2-roadmap.md` (global IDs continue from the current maximum), in roughly this
order: config and validation; branch detection and matching; maintenance resolver and guards;
history-aware bounds; `version current` and pre-releases on maintenance branches; docs, ADR and
guide. Separately, as its own `test:` commit: replace the real host/project fixtures in
`internal/pipeline/release_test.go` and `internal/platforms/gitlab/platform_test.go` with
synthetic placeholders (already recorded on T344). Follow-up tasks are filed for the CalVer
equivalent of §5, and for a "version branch" entry type (the version comes from the branch name,
`release/7.8.0` → `7.8.0`; commits never bump it; re-releases go through `-rc.N` or `+build`),
which today's `--set-version` covers manually.
