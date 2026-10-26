# Héraut — GitHub Client Roadmap (replace `gh` for publishing)

> Status: In progress (drafted 2026-10-25; T423 done 2026-10-26)
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
| 80 | GitHub publishing without `gh` | T423–T430 | In progress — T423 done |

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

#### `[ ]` T424: `config` helper for the GitHub API root

`config.GitHubAPIURL(baseURL)` (name settled in the task): github.com gives `https://api.github.com`,
`*.ghe.com` gives `https://api.<host>`, any other host gives `<base_url>/api/v3`; `GITHUB_API_URL` wins
only when `GITHUB_SERVER_URL` has the same origin as `base_url`; a non-secure URL is refused through
`IsSecureURL`. Table tests: trailing slash, port, path prefix, loopback http, `localhost.evil.com`, mixed-case
host, Actions on github.com publishing to a GHES. Not wired yet.

#### `[ ]` T425: open-once asset helper in `internal/platforms` (NEW-002)

A helper that resolves the asset globs (lenient, same warnings as today), checks confinement on the real
path, opens it, and confirms with `os.SameFile` that the descriptor is the checked file. It returns open
handles with name and size, and the caller closes them. Duplicate basenames are refused. Tests: the existing
`globs_test.go` rows keep passing; new rows for a symlink retargeted between check and open (simulated
through a test hook, no sleeps), a file replaced by a directory, and duplicate basenames. Not wired yet.
GitLab's `client.go` is not touched (see T431 in `roadmap.md`).

#### `[ ]` T426: GitHub publish client (create, upload, publish, cleanup)

`internal/platforms/github/client.go` on `go-github` v93 (ADR-0077): add the module, then create, upload from
a descriptor (`UploadReleaseAssetFromRelease`, reader and size), publish, delete on failure, list for `Check()`.
`WithAuthToken` plus `WithURLs` for the API and upload origins; heraut's own `upload_url` allowlist runs first
(`uploads.github.com`, or the GHES host or `uploads.<host>`), and the upload origin is configured from the
accepted value. Return the published asset names so the driver can warn on a rewrite. Contract tests on
`httptest.Server`: request order and bodies, no-asset single call, `draft: true` skips the PATCH, `prerelease`,
slash and `+` tags, upload refused for an unexpected origin (no request reaches it), a redirect to another
origin or to http carries no token, a failed upload or PATCH deletes the draft and returns the original error
(and a failed delete is appended), 1 MB notes, error text readable without Go struct dumps. Not wired yet.

#### `[ ]` T427: swap the GitHub driver onto the client; port `Check()`, tokens and Lane A

`platforms/github` stops using `port.Runner` (`New(cfg)`; `app` wiring in the same commit). Token chain per
ADR-0077 for both `Check()` and publishing; the Actions `GITHUB_TOKEN` special case goes. `CheckAssets`
drops the `#` refusal, and publishing warns when GitHub returns an asset name other than the file's basename. `hostEnv`, `tokenEnvSlice` and the notes temp file go. The platform tests move from
`MockRunner` to a stub server row by row (the `gh`-missing row is obsolete; new rows for each token-chain
branch). Lane A: `harness.Repo.FakeGitHub()`, a loopback stub logging `github [METHOD] [/path]` into the
shared call log, replaces `FakeCLI("gh")` in the `release` scenarios (the forge `base_url` points at it), plus
scenarios for an asset path with `#`, a refused upload leaving no draft, and the exit code. Run the whole suite
behind a logging proxy to prove no test reaches github.com. ADR-0076 gets a note that its 2026-10-24 addendum
is superseded by ADR-0077.

#### `[ ]` T428: drop `gh` from the runtime check, image and docs

`internal/app/check.go`'s binary-only fallback no longer probes `gh` (decide what, if anything, it reports
with no target). The Dockerfile drops `GH_VERSION` and its tools stage; `e2e.yml` pins `gh` itself (it reads
the version from the Dockerfile today) because Lane B's harness still uses it. Spec 01/02/05/06, `CLAUDE.md`
(stack table, layout, bundled CLIs, ADR count), README, the guides, `.claude/rules/testing.md` (contract layer
wording) and `config/forgeenv.go`'s comment drop `gh` as a runtime tool. `docs/audit/SECURITY_FINDINGS.md`:
NEW-002 resolved for GitHub (open for GitLab), NEW-004 resolved, SEC-004 residual closed. **Needs the user's
OK before touching the Dockerfile and CI.**

#### `[ ]` T429: Lane B validation on the GitHub sandbox

Only when the user says so. Run Lane B's GitHub scenarios against `bchatard/heraut-testing` with the swapped
driver, including one with assets, one draft target and one pre-release. Confirm cleanup leaves the sandbox
with no releases or tags. Record observed behaviour in `docs/guides/e2e-tests.md`. heraut's own `Release`
workflow is the first production run of the new driver; note anything it needs (token env) here.

#### `[ ]` T430: `internal/forge/github` onto the shared API-root helper

Switch the enrichment client's `apiBase` to the T424 helper so publishing and enrichment agree on the API
root, which adds `*.ghe.com` support to enrichment. Behaviour change for `*.ghe.com` hosts only; table tests
first.

**Phase 80 exit:** `go test -count=1 ./...` passes, `gh` appears only in Lane B's harness and historical
docs, each behaviour change confirmed to fail when reverted, lint clean, and Lane B green on GitHub (T429).
