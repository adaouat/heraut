# End-to-end test suite — design (T345)

Status: draft for review · Date: 2026-10-07 · Roadmap: `docs/tasks/roadmap.md` → Phase 60, T345

## Why

heraut is now used on many projects, so a regression that ships costs more than it did pre-v1.0.
The existing layers cannot see two classes of bug:

1. **A forge rejecting what heraut sends.** `MockRunner` and `httptest.Server` only prove heraut sends
   the arguments it intends to. T332's manual smoke run proved the gap: it found T335 (GitLab's
   package registry rejects per-env `/` tags), which no automated test could.
2. **Wiring bugs between layers that unit tests each pass.** Real binary, real `git`, real config
   file, real history: ldflags, flag plumbing, `os.Exit` mapping, hook staging, branch detection.

GitHub already gets one implicit end-to-end run per heraut release (the Release workflow dogfoods
`heraut release`). GitLab gets none, and no version-resolution rule (`stay_at_v0`, pre-release
lifecycle, promotion guards, maintenance ranges, CalVer period change) is exercised through the
shipped binary.

## Understanding agreed with the user

- Goal: raise heraut's quality; success means "a failing scenario means a real regression or a real
  forge rejection".
- Both forges from the start (GitHub and GitLab), plus compound scenarios: cross-forge publish
  (GitLab source → GitHub mirror), builds, pre-releases, per-env, maintenance branches.
- Version-resolution rules must be covered end to end too: `stay_at_v0`, pre-release lifecycle,
  CalVer (which needs a simulated clock).
- Fixed sandbox repos, a per-run tag namespace and a per-run branch deleted afterwards. Creating
  more sandbox repos is acceptable when a scenario needs one.
- Forge scenarios run in their own workflow: `workflow_dispatch` plus nightly, advisory, never on pull
  requests. Sandbox coordinates and tokens come from CI variables, never from the repo.

## Two lanes

The scenarios split by what they need, and the split decides where they run.

| | Lane A — hermetic binary | Lane B — forge sandbox |
|---|---|---|
| Drives | the built `heraut` binary | the built `heraut` binary |
| Git | local repo + local bare remote in `t.TempDir()` | fresh shallow clone of a sandbox repo |
| Network | none | GitHub / GitLab APIs and git remotes |
| Publish | `--dry-run`, or fake `gh`/`glab` via `exectest.FakeBin` | real `gh` / `glab` / enrichment HTTP |
| Build tag | none (runs in `go test ./...`) | `e2e_forge` |
| Runs | every PR and locally | `workflow_dispatch` + nightly, advisory |
| Rule impact | none: deterministic, no network, `t.TempDir()` only | needs a testing-rule amendment (ADR-0066) |

Lane A is the answer to "ensure `stay_at_v0` etc.": version-resolution behaviour needs no forge, so it
should gate every PR, not wait for a nightly. Lane B only carries what needs a real forge.

Both lanes live in a top-level `e2e/` package so they share the harness (build the binary, create a
repo, seed commits, run heraut, assert on stdout/exit code/tags/files).

## Simulated clock (CalVer)

CalVer resolution depends on `time.Now` (`internal/app/resolver.go` passes it to `calver.New` in two
places; `native.Generator` has its own injected clock for `GeneratedAt`). To test period boundaries
the binary under test needs a controllable clock.

| Option | Verdict |
|---|---|
| `libfaketime` / `LD_PRELOAD` | Rejected. Go reads the clock through the vDSO / direct syscalls, not libc, so it is unreliable on Linux and blocked by SIP on macOS. |
| A production `HERAUT_NOW` env var or `--now` flag | Rejected. A user-visible knob that changes release versions is a footgun and a new CLI surface (spec + schema + ADR) for a testing need. |
| **A test-only clock compiled in by a build tag (chosen)** | `internal/app/clock.go` returns `time.Now`; `internal/app/clock_testclock.go` (`//go:build heraut_testclock`) returns a clock read from `HERAUT_TEST_NOW` (RFC 3339). The e2e harness builds its binary with `-tags heraut_testclock`; released binaries never contain the code. |

Cost of the chosen option: the e2e binary differs from the shipped one by that one file. The harness also
builds a normal binary for the non-CalVer scenarios, so only CalVer scenarios use the tagged binary.
Changes in `internal/app` follow the layer rules (no new imports). `native` receives the same clock from
`app` instead of defaulting to `time.Now` when the tag is set.

## Harness (`e2e/harness`)

- `Build(t, tags...)` — `go build` once per `TestMain` per tag set into a temp dir, using the same
  `-ldflags` shape as `.goreleaser.yml` (`main.Version`), so the ldflags invariant is exercised.
- `NewRepo(t, opts)` — init a repo with a bare remote (Lane A) or clone a sandbox and create the
  per-run branch (Lane B); returns helpers `Commit(msg)`, `Tag(name)`, `WriteConfig(yaml)`.
- `Run(t, repo, env, args...)` → `{Stdout, Stderr, ExitCode}`. Environment is scrubbed of CI variables
  (as `testutil.ClearCIEnv` does) and `HOME` points at a temp dir.
- Config under test is built from small YAML fixtures in `e2e/testdata/`; no real names or hosts.
- Assertions on exit codes use `internal/exitcode` so a mapping change is caught.

## Lane A scenarios (hermetic)

Each row is one table entry or one short test; all run the real binary against a local repo.

**SemVer resolution (`version next`, `version current`)**

| Area | Cases |
|---|---|
| Bump from commits | `feat` → minor, `fix` → patch, `feat!`/`BREAKING CHANGE` → major, no releasable commit; `v1.9.0` → `v1.10.0` (never `v1.100.0`) |
| Prefix and shape | default `v`, custom `tag_prefix`, empty prefix; `initial_version` on an untagged repo |
| `bump.mode: manual` | refuses without `--set-version`; `--set-version` accepted |
| `stay_at_v0` | `0.x` + breaking → minor with the warning; `--allow-major` lifts it; no effect at ≥ 1; applies under `semver-per-env`; interaction with `--pre-release` series escalation |
| Pre-release lifecycle | `rc.1` → `rc.2` → final; regression refused (`ErrPreReleaseRegression`); major escalation refused unless `--allow-major`; `--pre-release` rejected with `--set-version`, non-`semver` strategy, manual mode |
| Overrides | `--set-version` with and without prefix; build metadata rejected in the version, accepted via `--set-build-id`; invalid SemVer exit code |

**CalVer (uses the simulated clock)**

| Area | Cases |
|---|---|
| Period change | `PATCH` resets on month / year / ISO-week / quarter boundary (clock set just before and after) |
| Tokens | `YYYY`, `MM`, `DD`, `WW`, `QQ`, `SS`, `SPRINT`, `PATCH`; year-end ISO week (e.g. 2026-12-31 vs 2027-01-01) |
| Sprint | `version sprint bump` rewrites `.heraut.yml`; the next version uses it |
| `calver-per-env` | per-env tag shape, promotion between envs |

**Per-env promotion**

`semver-per-env` and `calver-per-env`: first tag in an env, promotion from source env, `E001`
target exists, `E002` destination ahead, `E003` no source tags, each with and without `--force`;
`tag_format` with `{env}` / `{build}`.

**Maintenance branches (ADR-0065)**

`release/1.3` resolves inside its range; an unlisted branch refused except `--force`/`--dry-run`;
collision guard on `--set-version` (probed only when the run tags, so `changelog` without `--tag` is not refused);
same-commit tags bound each other in the changelog.

**Changelog and release flow, local publish**

`changelog --commit --tag --no-push` against the bare remote: changelog file content, commit
message, annotated tag, history-aware bounds across a maintenance branch, `--regenerate`,
`disable_changelog` per env, hook points (`post_bump`, `pre_changelog`, `pre_tag`, `post_tag`) and
`--no-hooks` / `--skip-hook`. `release` runs with fake `gh`/`glab` binaries that record argv, so a
flag regression is caught without a forge. `--offline` forces `enrichment_policy: disabled`.

**CLI surface**

`check config` / `check runtime` exit codes; `--config` and `HERAUT_FILE` including `~`
expansion (regression for `4d17b19`); `commit verify` on valid/invalid messages; `--version`
prints the injected ldflag; unknown config key reports a line number.

## Lane B scenarios (forge sandboxes)

Run for GitHub (GH) and GitLab (GL) unless noted. Each scenario uses its own tag namespace.

| # | Scenario | Asserts |
|---|---|---|
| B1 | Final SemVer release with notes | release exists, body equals the generated notes, tag points at the release commit |
| B2 | Pre-release (`--pre-release rc`) | GH: `prerelease: true`; GL: created as a plain release |
| B3 | Build metadata (`--set-version` + `--set-build-id`) | `+` tag accepted, release URL resolves (GL URL-escapes `+` and `/`) |
| B4 | Per-env `{env}/{version}` tag with an asset | `/` tag and asset upload accepted; GL regression test for T335/T346 |
| B5 | CalVer release (simulated date) | CalVer tag shape accepted, notes rendered, release created |
| B6 | Cross-forge: GL source, targets GL + GH | one run creates two releases with identical notes; the mirror target uses its own token |
| B7 | Draft release (`draft: true`) | published as draft, not visible as latest |
| B8 | Maintenance branch release | tag cut from `release/*` stays in range; collision guard refuses a taken version against real tags |
| B9 | PR/MR enrichment (ADR-0043) | a per-run PR/MR is created and merged through the API; the changelog entry carries its title/number via the real `net/http` client |
| B10 | `release` with `--dry-run` | no tag, release, or push is created on the forge |

Not in slice 1: `heraut init`, `whatsnew` (reads heraut's own releases), Azure DevOps enrichment (no
sandbox), rate-limit tuning, GitLab-over-`net/http` (T347 will need a rerun of B1-B4 and B6).

## Isolation and cleanup (Lane B)

- `run-id`: CI run id, or timestamp plus random suffix locally.
- Per-run branch `e2e/<run-id>` from a fixed seed commit; default branch is never written.
- Tags use `tag_prefix: e2e-<run-id>-`; per-env tags use the same prefix inside `tag_format`.
- `t.Cleanup` deletes (via `gh api` / `glab api`) the remote branch, tags, releases and registry
  packages carrying the `e2e-<run-id>` marker.
- Sweeper (`go run ./e2e/cmd/sweep`): deletes anything matching `e2e-*` older than 24 h; runs first in
  the nightly.
- **Safety guards:** deletions target only resources whose name contains the run marker; the harness
  refuses to run unless the configured repo name matches `HERAUT_E2E_REPO_PATTERN` (default
  `*sandbox*`) and the repo is private.
- **Configuration:** `HERAUT_E2E_GITHUB_REPO`, `HERAUT_E2E_GITLAB_PROJECT`,
  `HERAUT_E2E_MIRROR_GITHUB_REPO` (B6 target), tokens as `GH_TOKEN` / `GITLAB_TOKEN`. A missing
  variable skips the test with an explicit message, so `go test -tags e2e_forge ./e2e/...` on a fresh
  machine is harmless. Tokens need branch/tag/release/package write on the sandbox repos only; no
  repository create or delete scopes.

## CI and rules

- **Lane A** is part of `go test ./...` (it builds the binary in `TestMain`). Its cost is a few
  seconds of `go build`; if it grows, it moves behind `-short`.
- **Lane B** gets `.github/workflows/e2e.yml`: `workflow_dispatch` + nightly `schedule` on `main`, no
  `pull_request` trigger, advisory (failures notify, nothing gates). The Release workflow is not
  modified (ADR-0018 stays as is).
- **ADR-0066** records: the fifth test layer (opt-in forge e2e), its exemption from "no network
  calls" and "no filesystem outside `t.TempDir()`" (it still never touches the source tree), the
  `e2e_forge` and `heraut_testclock` build tags, and that Lane A stays inside the existing rules.
- `.claude/rules/testing.md` gets a short "E2E" section pointing at the ADR; Spec 06 gets a pointer;
  `docs/guides/e2e-tests.md` documents sandbox setup, variables and token scopes with synthetic names only.
- `mise run test:e2e` runs Lane B locally (requires the variables); `mise run test` already
  runs Lane A.

## Task breakdown

T345 is too large for one session; it is replaced by four tasks, one per session, in this order:

| Task | Content | Lane |
|---|---|---|
| T345a | ADR-0066, testing-rule amendment, harness (`Build`, `NewRepo`, `Run`), `heraut_testclock` clock, first scenarios: SemVer resolution + `stay_at_v0` + pre-release lifecycle | A |
| T345b | CalVer with simulated clock, per-env promotion, maintenance branches, changelog/release local flow, CLI surface | A |
| T345c | Forge harness (config, guards, cleanup, sweeper) + B1-B4 on both forges | B |
| T345d | B5-B10 (CalVer, cross-forge, draft, maintenance, enrichment, dry-run), `e2e.yml`, guide, `mise` task | B |

Each task follows the two-step roadmap flow and TDD (a scenario that fails first, then the fix, if a
real bug is found, in its own commit).

## Open questions

1. **Seed commit and sandbox layout.** One sandbox per forge plus one mirror repo (B6) is assumed.
   B9 needs the sandbox to allow merging a PR/MR through the API (branch protection off on the sandbox).
2. **Nightly notifications.** Advisory failures only appear in the Actions tab unless a channel is wired;
   whether to add one is left to T345d.
3. **Lane A runtime.** If building the binary per `TestMain` makes `go test ./...` noticeably slower,
   Lane A moves behind `-short`; decide with measurements in T345a.
