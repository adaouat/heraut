# Héraut — GitHub Client Roadmap (replace `gh` for publishing)

> Status: Done (drafted 2026-10-25; T423–T426 done 2026-10-26, T427–T430 and T432 done 2026-10-27)
> Decision: [ADR-0077](../adr/0077-github-publish-transport.md) (Accepted: `go-github`).
> Precedent: ADR-0069 and Phase 61 (T364–T368), which moved GitLab publishing from `glab` to `client-go`.
> Main roadmap: tracked as Phase 80 in [`roadmap.md`](roadmap.md).

## Conventions

- Task IDs **continue the global sequence** (`T423+`).
- This file is the **single source of truth** for task status. Markers: `[ ]` not started, `[x]` done.
  Two-step flow ([`workflow.md`](../../.claude/rules/workflow.md)): failing test first, implement, flip the
  marker and add a completion note.
- One task per session. Every behaviour change is sensitivity-checked: revert the production change and
  confirm the new tests fail.
- Commits go straight to `main`; never push without asking. Lane B (`e2e_forge`) runs only when the user
  says so, against `bchatard/heraut-testing`.
- **The GitLab driver is out of scope.** Only `internal/platforms/github`, the shared `internal/platforms`
  helpers, `internal/config`, `internal/forge/github` (T430), `app` wiring, the harness and docs change.
- Lint and hooks run under `mise exec --` (the shell's Go and golangci-lint can be off-pin).

## Progress at a glance

| Phase | Theme | Tasks | Status |
|---|---|---|---|
| 80 | GitHub publishing without `gh` | T423–T430, T432 | Done — T423–T430 and T432 done (exit note below) |

### Phase 80 — GitHub publishing without `gh`

#### `[x]` T423: gate — measure `net/http` against `go-github` and probe the publish flow live

In a scratch worktree, implement the publish flow twice (option A, plain `net/http`; option B,
`google/go-github` at its current major): create draft, upload one asset to `upload_url`, publish, list
releases, delete. Record for each: stripped binary size (darwin/arm64, as T364 did), direct and indirect
modules added, `go.sum` lines, and lines of code. Probe the sandbox live: `upload_url` host shape for
github.com, a slash tag (`uat/0.0.1`), `prerelease`, a draft left as a draft, the 422 behaviour when
uploading to a published release with immutable releases on, and what deleting a draft leaves behind (the
tag must survive). Fill the transport line of ADR-0077's Decision and move it to Accepted. Restore the
sandbox to a bare `main`. No production code.

**Completion note:** the user chose option B, `go-github` v93.0.0, over the recommended option A. Measured
(go1.27.3, stripped, darwin/arm64, each option linked into heraut through a build-tagged `init`): A adds
51 KB (+0.2%) and no module, 217 lines; B adds 4.6 MB (+19.8%), one direct module and no indirect one, two
`go.sum` lines, 82 lines; `go-github` shipped 33 majors between v60 (2024-02) and v93 (2026-10). v93 already
limits the token to the configured API and upload origins on every request and refuses an upload body to any
other origin (`ErrUntrustedDestination`), so T426 relies on it instead of a hand-written `CheckRedirect`. Live
(both options identical): on `heraut-testing`, slash and `+` tags, `prerelease`, a `draft: true` target left as
a draft (`html_url` is `untagged-…`), and a failed upload (duplicate name, 422) deleting the draft while the
tag survives; `upload_url` is on `uploads.github.com`. On a throwaway private repo with immutable releases on
(`bchatard/heraut-t423-immutable`, created because a published immutable tag cannot be deleted; the user
deletes it, the token lacks `delete_repo`): draft, upload, publish works and returns `immutable: true`; an
upload to the published release gives 422 `Cannot upload assets to an immutable release.`; deleting its tag
is refused. New finding: GitHub rewrites special characters in asset names (`meta#1.json` becomes
`meta.1.json`); the user chose to lift the `#` rule anyway and warn when the published name differs, now in
ADR-0077 and T427. `heraut-testing` was restored to a bare `main`. The probe code lived in a scratch worktree
and was not committed.

#### `[x]` T424: `config` helper for the GitHub API root

`config.GitHubAPIURL(baseURL)` (name settled in the task): github.com gives `https://api.github.com`,
`*.ghe.com` gives `https://api.<host>`, any other host gives `<base_url>/api/v3`; `GITHUB_API_URL` wins
only when `GITHUB_SERVER_URL` has the same origin as `base_url`; a non-secure URL is refused through
`IsSecureURL`. Table tests: trailing slash, port, path prefix, loopback http, `localhost.evil.com`, mixed-case
host, Actions on github.com publishing to a GHES. Not wired yet.

**Completion note:** `config.GitHubAPIURL(baseURL, getenv)` in `internal/config/githubapi.go` takes `getenv` like
`forge.Resolve`, so it reads no process environment itself. It returns the root without a trailing slash (go-github
wants one; T426 adds it). Two choices the task left open, approved by the user and recorded in ADR-0077: `GITHUB_API_URL` is used only when `GITHUB_ACTIONS=true`
as well as a same-origin `GITHUB_SERVER_URL` (outside Actions stray variables are ignored), and an insecure
`GITHUB_API_URL` that passes the origin test is an error rather than a silent fallback. Same origin compares
scheme, host name (case-insensitive) and port (an omitted port is the scheme's default). The github.com and
`*.ghe.com` cases ignore any path in `base_url`; GHES keeps it as a prefix (`/github/api/v3`). Not handled: a
`base_url` that already ends in `/api/v3` gets it twice (not seen in configs; YAGNI). Tests: 14 derivation rows,
5 insecure bases, 7 Actions rows, 1 insecure `GITHUB_API_URL`. Sensitivity: eight mutations (same-origin test,
`GITHUB_ACTIONS` test, `.ghe.com` case, its leading dot, the two `IsSecureURL` checks, default-port
normalization, host case folding) each fail at least one row.

#### `[x]` T425: open-once asset helper in `internal/platforms` (NEW-002)

A helper that resolves the asset globs (lenient, same warnings as today), checks confinement on the real
path, opens it, and confirms with `os.SameFile` that the descriptor is the checked file. It returns open
handles with name and size, and the caller closes them. Duplicate basenames are refused. Tests: the existing
`globs_test.go` rows keep passing; new rows for a symlink retargeted between check and open (simulated
through a test hook, no sleeps), a file replaced by a directory, and duplicate basenames. Not wired yet.
GitLab's `client.go` is not touched (see T431 in `roadmap.md`).

**Completion note:** `platforms.OpenAssets(patterns, warn) ([]Asset, error)` in `internal/platforms/assets.go`, with
`Asset{Path, Name, Size, File}` and `CloseAssets`. It globs, skips directories and warns like `ResolveGlobsLenient`
(left as is for the `gh` driver and GitLab), checks confinement on the real path, then opens the checked real path
through an `os.Root` on the repository root and requires `os.SameFile` between the first stat and the open file.
Deviation from the task text: `os.Root` (Go 1.24+) was added on top of `SameFile`, because `SameFile` alone misses a
path that points outside when stat'ed and opened but inside during the check; a test pins that case.
`os.Root` also refuses absolute symlink targets, which is harmless since the opened path has no symlinks left
unless swapped. `requireInside` now delegates to a new `realPathInside`, which returns the relative real path.
Duplicate basenames are refused before anything is opened, including one file matched by two patterns. The name
is the basename of the match, so a symlink publishes under the link's name as with `gh`; a leading `-` needs no
`./` prefix. On any error no file stays open. Swaps are driven by two package hooks (`afterStat`, `beforeOpen`) in
`assets_internal_test.go`, no sleeps. Tests: 12 behaviour rows (mirroring the `globs_test.go` rules, which all
still pass) and 7 swap rows (file to an outside symlink, parent directory to an outside symlink, inside retarget,
file renamed over, file replaced by a directory, a later asset refused after an earlier one opened, the
three-step case). Sensitivity: eight mutations (plain `os.Open`, no `SameFile`, no duplicate check, no confinement
error, no directory skip, no warning, name from the real path, size zeroed) each fail at least one row. Not
covered: closing the earlier files on error is not observable from a test. Follow-up the same day, at the
user's request: anything but a regular file is refused at the check, before any open (a FIFO would block
the open, and the release after the tag push), and on Unix the open uses `O_NONBLOCK` so a FIFO swapped in
after the check fails `SameFile` instead of hanging (`assets_unix.go`; Windows has no filesystem FIFOs). Two
Unix-only rows, guarded by a 5 s watchdog; removing either defence fails its row. Not wired yet.

#### `[x]` T426: GitHub publish client (create, upload, publish, cleanup)

`internal/platforms/github/client.go` on `go-github` v93 (ADR-0077): add the module, then create, upload from
a descriptor (`UploadReleaseAssetFromRelease`, reader and size), publish, delete on failure, list for `Check()`.
`WithAuthToken` plus `WithURLs` for the API and upload origins; heraut's own `upload_url` allowlist runs first
(`uploads.github.com`, or the GHES host or `uploads.<host>`), and the upload origin is configured from the
accepted value. Return the published asset names so the driver can warn on a rewrite. Contract tests on
`httptest.Server`: request order and bodies, no-asset single call, `draft: true` skips the PATCH, `prerelease`,
slash and `+` tags, upload refused for an unexpected origin (no request reaches it), a redirect to another
origin or to http carries no token, a failed upload or PATCH deletes the draft and returns the original error
(and a failed delete is appended), 1 MB notes, error text readable without Go struct dumps. Not wired yet.

**Completion note:** `internal/platforms/github/client.go` adds `NewClient(baseURL, repository, token, getenv)`,
`CreateRelease(tag, notes, draft, prerelease, assets []platforms.Asset) ([]string, error)` (it returns the names
GitHub published) and `VerifyReleasesAccess()`, on `go-github` v93.0.0 (one direct module and no indirect one, as
T423 measured). The create request sets `name` to the tag, as in ADR-0077's API table; T429 should confirm the
title matches what `gh` published. go-github fixes its upload origin when the client is built, so once
`checkUploadURL` accepts the returned `upload_url` (`uploads.github.com` for github.com, otherwise `base_url`'s
origin or `uploads.<host>` on the same scheme and port, never with userinfo), a second go-github client is built
with that origin; go-github's `ErrUntrustedDestination` gate and per-request token rule then apply to it.
`config.sameOrigin` became `config.SameOrigin` for that check. go-github's `ErrorResponse.Error()` prints its
error list as Go structs, so errors are wrapped in an `apiError` that reads `422 Validation Failed: name
already_exists` (or the server's message, such as the immutable-release one) and still unwraps to the go-github
error. The repository must be `owner/repo` in GitHub's character set, which also keeps `/`, `?` and `#` out of the
paths go-github builds. Two checks were dropped as duplicates of go-github: an empty token (`WithAuthToken`
refuses it) and copying the `http.Client` (`WithHTTPClient` copies it; a test pins that). Contract tests use real
host names: a dialer routes every connection to a local TLS stub (plain HTTP on port 80), so github.com, GHES with
and without subdomain isolation, a `*.ghe.com` tenant and an Actions `GITHUB_API_URL` all run with no network.
Tests: 5 no-asset rows (draft, prerelease, slash and `+` tags), 1 MB notes, the draft-upload-publish sequence
with bodies, sizes and Content-Types, a draft target, a rewritten name, 4 host rows, 5 refused upload origins
(the draft is deleted, no upload request is made), 19 `checkUploadURL` rows, failed create, upload, publish and
delete, 3 redirect rows (another host, https to http, another port) with no token on the second hop, the access
check, 14 construction rows. Sensitivity: 15 mutations; 14 fail at least one row (draft flag, publish skipped or
forced, no delete, delete error replacing the original, no allowlist, no github.com case, userinfo accepted, raw
go-github errors, upload origin not configured, no release name, no `per_page`, dot names, shared `http.Client`
after the test was added); the empty-token one led to dropping the duplicate check. Not handled: requests have no
timeout (as with `gh`), and retries are go-github's. Not wired yet.

#### `[x]` T427: swap the GitHub driver onto the client; port `Check()`, tokens and Lane A

`platforms/github` stops using `port.Runner` (`New(cfg)`; `app` wiring in the same commit). Token chain per
ADR-0077 for both `Check()` and publishing; the Actions `GITHUB_TOKEN` special case goes. `CheckAssets`
drops the `#` refusal, and publishing warns when GitHub returns an asset name other than the file's basename. `hostEnv`, `tokenEnvSlice` and the notes temp file go. The platform tests move from
`MockRunner` to a stub server row by row (the `gh`-missing row is obsolete; new rows for each token-chain
branch). Lane A: `harness.Repo.FakeGitHub()`, a loopback stub logging `github [METHOD] [/path]` into the
shared call log, replaces `FakeCLI("gh")` in the `release` scenarios (the forge `base_url` points at it), plus
scenarios for an asset path with `#`, a refused upload leaving no draft, and the exit code. Run the whole suite
behind a logging proxy to prove no test reaches github.com. ADR-0076 gets a note that its 2026-10-24 addendum
is superseded by ADR-0077.

**Completion note:** `github.New(cfg)` takes no runner; the driver keeps `getenv`, a warnings writer and an
optional `*http.Client` as unexported fields so the contract tests (`platform_internal_test.go`) reuse T426's
TLS stub with real host names. The token chain lives in `tokenEnvs`/`token` and is shared by `Check()` and
publishing; a table runs each of its 10 rows through both. A missing token and a missing repository are reported
together before any request. `CreateRelease` opens the assets through `platforms.OpenAssets` and warns on stderr
when GitHub publishes a different name. `config` gains `EnvGitHubEnterpriseCLIToken` and
`EnvGitHubEnterpriseToken` (also in `CIEnvVars`, so `ClearCIEnv` clears them), and `EnvGitHubCLIToken`'s comment
is reworded. The `gh` platform rows became stub rows; the binary-only `gh` fallback of `heraut check runtime` stays
for T428. Lane A: `harness.Repo.FakeGitHub()` answers as a GHES without subdomain isolation (API under `/api/v3`,
uploads under `/api/uploads`, one loopback origin) and can refuse releases or uploads; the `release`, `check
runtime` and maintenance scenarios point the forge's `base_url` at it. New scenarios: a path with `#` is published
with a warning, a refused upload deletes the draft and exits Runtime. The obsolete "missing gh" `check runtime`
row became "git and a token pass". Beyond the task text: four tests outside the driver's package (`internal/app`
`TestRuntimeCheck_WithGitHubPlatform` and `TestBuildTargetPlatforms_TargetOwnAssetsAreLenient`, `internal/cmd`
`TestCheckRuntime_AllGood` and `TestRelease_ForeignFile_ShortSummaryNotRepeated`) reached the real
api.github.com once `gh` was no longer faked; they now use a loopback stub. Spec 02 and Spec 05 describe the
requests and the token chain; the `gh` wording about the runtime tool, Spec 06's and `testing.md`'s `MockRunner`
example (`github.New(mr, …)` no longer compiles) are left for T428. Proof of no network: the whole suite ran
behind a logging proxy that refuses every request, with the harness temporarily forwarding `HTTPS_PROXY` to the
binary (not committed), and logged nothing; a control run of the binary with no `base_url` was logged
(`CONNECT api.github.com:443`). Sensitivity: 14 mutations of the driver (token_env falling through, chain order on
GHES, `GITHUB_TOKEN` on GHES, `*.ghe.com` as GHES, no `GITHUB_TOKEN` on github.com, no API call in `Check()`, no
rename warning, repository from the process environment, `GITHUB_API_URL` ignored, the hint's variable, the
repository error dropped, draft dropped, unmatched-pattern warning dropped, `#` refused again) each fail at least
one row. Not done: the preflight `CheckAssets` does not refuse duplicate basenames or non-regular files; those are
refused at publish time, after the tag push.

#### `[x]` T428: drop `gh` from the runtime check, image and docs

`internal/app/check.go`'s binary-only fallback no longer probes `gh` (decide what, if anything, it reports
with no target). The Dockerfile drops `GH_VERSION` and its tools stage; `e2e.yml` pins `gh` itself (it reads
the version from the Dockerfile today) because Lane B's harness still uses it. Spec 01/02/05/06, `CLAUDE.md`
(stack table, layout, bundled CLIs, ADR count), README, the guides, `.claude/rules/testing.md` (contract layer
wording) and `config/forgeenv.go`'s comment drop `gh` as a runtime tool. `docs/audit/SECURITY_FINDINGS.md`:
NEW-002 resolved for GitHub (open for GitLab), NEW-004 resolved, SEC-004 residual closed. **Needs the user's
OK before touching the Dockerfile and CI.**

Done 2026-10-27. With no target resolving, `heraut check runtime` shows one advisory `publish` row (`no release
target resolves to github or gitlab`, or with no config `no config found, destinations not checked`) instead of
probing `gh`; the user chose this over an empty section or a hard error. With no config only git is required, so a
git-only machine passes (the e2e row "with no config every tool is required" became "only git is checked"), and
the header reads `no config found at <path> — checking git only`. Four mutations (warn turned into a failure, the
two reasons swapped, a `gh` probe put back, the row dropped) each fail a test. With the user's OK: the Dockerfile
drops the mise tools stage and both `GH_VERSION` and `MISE_VERSION` (the latter had no other use); the image holds
heraut, git and CA certificates, 230MB on a local `arm64` build, checked with `docker build` and `--version`.
`e2e.yml` pins `GH_VERSION=2.92.0` itself and Renovate tracks it there; the Dockerfile's `MISE_VERSION` rule went
with the arg. Docs: `CLAUDE.md` (stack table now lists go-github, layout, the external-CLI section), README
prerequisites (git only, plus the token chain), Spec 01, 03 (`check runtime`, publish steps), 05, 06 (error
example, contract layer, no-network rule), `.claude/rules/testing.md`, the e2e, hooks and failure-recovery guides
(the last keeps its manual `gh`/`glab` commands and says heraut does not need them), `SKILL.md`, and
`SECURITY_FINDINGS.md` (NEW-002 resolved for GitHub, NEW-004 resolved, SEC-004 residual closed). ADR-0016 gets an
amendment note for both `gh` (ADR-0077) and `glab` (ADR-0069, which had not updated it). The Spec 06 contract
example is a real test (`TestCreateRelease_SendsTheRelease`) so it cannot rot again. Left as is:
`internal/config/forgeenv.go`'s comment, which names `gh` only as the origin of the token names; the harness's
generic `FakeCLI`, now used only by its own tests (they fake a binary called `gh`); `platform.go`'s comment on the
token chain's compatibility with the old driver; ADRs and plans, which are history.

#### `[x]` T429: Lane B validation on the GitHub sandbox

Only when the user says so. Run Lane B's GitHub scenarios against `bchatard/heraut-testing` with the swapped
driver, including one with assets, one draft target and one pre-release. Confirm cleanup leaves the sandbox
with no releases or tags. Record observed behaviour in `docs/guides/e2e-tests.md`. heraut's own `Release`
workflow is the first production run of the new driver; note anything it needs (token env) here.

Done 2026-10-27, on the user's go. Lane B ran against `bchatard/heraut-testing` (and `-enrich` for B9) with
the in-process driver: all ten GitHub scenarios pass in 140s (B1–B5, both B7s, B8, B9, B10), covering assets
(B4, B7 "matching files are attached", two files), a draft target (B7, stays a draft) and a pre-release (B2,
flagged). B6, which publishes a GitLab-sourced release to GitHub as a second target, was run on its own with the
GitLab sandbox too and passes; no GitLab code changed. No `[remote rejected]` push was retried, and no rename
warning appeared. Afterwards both GitHub sandboxes and the GitLab one hold no release, tag, package or branch
besides `main`. The scenarios needed no change: the harness already sets `token_env: GH_TOKEN`, so the default
token order is not exercised live (it is covered by `TestPlatform_TokenChain`). The sandbox keeps immutable
releases off on purpose (a published immutable tag cannot be deleted, so cleanup would leak); that path was
probed in T423 and is exercised by heraut's own releases. heraut's `Release` workflow needs nothing new:
forge's `release-publish` action (v0.19.7) already sets `GH_TOKEN` and `GITHUB_TOKEN`, and
`.config/heraut.yml` has no `token_env`, so the driver reads `GH_TOKEN`; the next release of heraut is the first
production run of the new driver. `docs/guides/e2e-tests.md` records the observed behaviour.

#### `[x]` T430: `internal/forge/github` onto the shared API-root helper

Switch the enrichment client's `apiBase` to the T424 helper so publishing and enrichment agree on the API
root, which adds `*.ghe.com` support to enrichment. Behaviour change for `*.ghe.com` hosts only; table tests
first.

Done 2026-10-27. Wider than the task text, on the user's decision: forge resolution copied `GITHUB_API_URL`
into every GitHub identity whenever the CI type was GitHub, so a GHES or `*.ghe.com` forge run on a github.com
runner sent its enrichment queries, with its own token, to api.github.com. Now `forge.Resolve` takes a GitHub
identity's `APIURL` from config `api_url` when set, else from `config.GitHubAPIURL(host, getenv)`, the helper
publishing uses, after `requireSecure`; the CI value is no longer copied for GitHub (explicit and zero-config
paths). An insecure `GITHUB_API_URL` is refused there. `internal/forge/github`'s `apiBase` falls back to the same
helper (with no environment) for an identity built without `APIURL`, and returns an error instead of a string;
`graphqlEndpoint` gives `{api root}/graphql` for api.github.com and `api.*.ghe.com`, and keeps `/api/graphql`
beside `/api/v3` for GHES. Behaviour changes: `*.ghe.com` enrichment (was `<host>/api/v3`), and a forge run in
Actions on another instance. GitLab and Azure resolution is unchanged. Tests first: a 9-row table on the
resolved API root, an insecure `GITHUB_API_URL`, two `*.ghe.com` GraphQL rows and an insecure host; six mutations
(CI value copied again for explicit or zero-config forges, the helper given no environment, no derivation, no
`*.ghe.com` GraphQL rule, the old host fallback) each fail a test. Spec 02 § Identity resolution and Spec 05's
self-hosted bullet say so.

#### `[x]` T432: refuse duplicate basenames and non-regular assets before anything is committed

`platforms.OpenAssets` refuses two matches with the same basename and any match that is not a regular file,
but only at publish time, after the tag is pushed. `platforms.CheckAssetGlobs`, which `CheckAssets()` runs
before the changelog commit, applies neither rule, so such a release fails half-way. Move both rules into the
preflight: share one check between `CheckAssetGlobs` and `OpenAssets`, without opening anything in the
preflight (a FIFO must still never block). `CheckAssetGlobs` is shared with the GitLab driver, so its
preflight gains the same refusals; GitLab's publish path itself is not touched (T431). Table tests first in
`internal/platforms`, and one Lane A scenario showing that a duplicate basename stops `heraut release` with no
commit and no tag.

Done 2026-10-27, before T431 on the user's choice. `OpenAssets`' matching loop became `matchAssets` (glob,
skip directories, refuse anything but a regular file, confinement, duplicate basename, warn on no match), and
`CheckAssetGlobs` calls it instead of `ResolveGlobsLenient`; it opens nothing, so a FIFO is refused by its mode
and cannot block. The same file matched by two patterns stays refused, as `OpenAssets` already did. These
refusals are never wrapped in `port.ErrAssetUnmatched`, so a dry run fails on them too. `ResolveGlobsLenient`,
GitLab's publish path, is unchanged: until T431, GitLab's preflight refuses these cases but its publish step
would not. Tests first: a duplicate table (two directories, one pattern matching both, the same file twice;
with and without `allow_missing_assets`), distinct names pass, a FIFO refused without blocking, and a Lane A
scenario for GitHub and GitLab, real and dry runs: exit Runtime, no commit, no tag, only read requests reached
the stand-ins. Against the old code the GitLab real run committed and tagged. Three mutations (no regular-file
rule, no duplicate rule, a match error wrapped as unmatched) each fail tests. Spec 03 step 3 and Spec 05 § asset
resolution list the rules.

**Phase 80 exit:** `go test -count=1 ./...` passes, `gh` appears only in Lane B's harness and historical
docs, each behaviour change confirmed to fail when reverted, lint clean, and Lane B green on GitHub (T429).

Exit check, 2026-10-27: met, with one leftover. `go test -count=1 ./...` passes, lint is clean, every task's
mutations fail tests, and Lane B is green on GitHub. `gh` still appears outside Lane B's harness and history in
one place: Lane A's generic `harness.Repo.FakeCLI`, which no scenario uses any more, is unit-tested with a fake
binary named `gh` (`e2e/harness/harness_test.go`). Removing or renaming it is harness cleanup, not publishing
behaviour, and is left for the user to schedule. GitLab's half of NEW-002 is T431 (Phase 81).
