# ADR-0077: GitHub publish transport — replace `gh` with an in-process client

- **Status**: Proposed (the transport choice is settled by the T423 gate; everything else below is decided)
- **Date**: 2026-10-25
- **Deciders**: bchatard

---

## Context

`internal/platforms/github` publishes by shelling out to `gh release create` through `port.Runner`, and
`Check()` runs `gh --version` and `gh api repos/<repo>/releases`. ADR-0069 moved GitLab publishing from
`glab` to an in-process client and left GitHub out of scope ("GitHub gets its own decision if the same
motivation appears"). Three motivations have now appeared:

- **NEW-002 (CWE-367, Info).** `platforms.ResolveGlobsLenient` checks that each asset's real path is inside
  the repository, then hands the *path* to `gh`, which opens it later. A symlink swapped in between escapes
  the confinement. The fix is to open each file once and read from the descriptor, which a CLI taking a path
  cannot do. The finding was deferred until this epic (`docs/audit/SECURITY_FINDINGS.md`).
- **`GH_HOST` scheme.** For a self-hosted `base_url` the driver passes `gh` only the host (`GH_HOST`);
  which scheme `gh` then uses was never verified (SEC-004 residual).
- **Runtime dependency.** `gh` is a separate install: `heraut check runtime` probes it, the Docker image
  carries a `gh` stage, and Lane A fakes it with `FakeCLI("gh")`.

### What `gh release create <tag> <files>` does

It is not one atomic call. With files attached, `gh` creates the release as a **draft**, uploads each file to
the release's `upload_url`, then publishes the draft (`PATCH draft=false`). That ordering is what avoids
GitHub's 422 "Cannot upload assets to an immutable release" (T346), so any replacement must keep it.

`gh` also reads more credentials than heraut gives it: `GH_TOKEN`, `GITHUB_TOKEN`, and for an enterprise host
`GH_ENTERPRISE_TOKEN`, `GITHUB_ENTERPRISE_TOKEN`, then its keyring. heraut never relied on the keyring: the
release pipeline runs `Check()` first (`pipeline/release.go`), and `Check()` refuses a run where
`$<token_env>` is unset. heraut copies the `token_env` value into `GH_TOKEN`, and on a self-hosted host also
into `GH_ENTERPRISE_TOKEN`; it never reads the latter. In GitHub Actions on github.com, `Check()` validates
`GITHUB_TOKEN` while publishing uses `token_env`, so the check and the publish can test different tokens.

### API surface needed

| Step | Call |
|---|---|
| `Check()` | `GET /repos/{owner}/{repo}/releases?per_page=1` |
| create | `POST /repos/{owner}/{repo}/releases` (JSON: `tag_name`, `name`, `body`, `draft`, `prerelease`) |
| upload | `POST {upload_url}?name=<basename>` (raw body, `Content-Type`, `Content-Length`; no multipart) |
| publish | `PATCH /repos/{owner}/{repo}/releases/{id}` (`draft: false`) |
| cleanup | `DELETE /repos/{owner}/{repo}/releases/{id}` |

`upload_url` is on another host than the API: `uploads.github.com` for github.com; `{host}/api/uploads` on
GitHub Enterprise Server, or `uploads.{host}` with subdomain isolation.

### Layering

`platforms/*` cannot import `forge/*`. `internal/forge/github` already has a hand-rolled `net/http` client
with the token-safe redirect rule (T419) and the API root derivation (`api.github.com`, `{host}/api/v3`).
The derivation moves to `internal/config`, a leaf both packages may import.

## Options (transport)

| | A. Native `net/http` | B. `github.com/google/go-github` | C. Stay on `gh` |
|---|---|---|---|
| Removes `gh` dependency | yes | yes | no |
| Fixes NEW-002 | yes | yes (upload takes an `*os.File`) | no |
| Request code owned by heraut | ~5 small calls, JSON and a raw body | none | none |
| Fit with existing code | same style as `forge/github` | a second GitHub HTTP stack | n/a |
| Dependency weight | none | to be measured (T423) | external binary, a Docker image stage |
| Upgrade churn | none | frequent major versions | `gh` version drift |
| Contract tests | `httptest.Server` | `httptest.Server` (base and upload URLs are settable) | `MockRunner` argv |

ADR-0069 chose the SDK for GitLab because of job-token auth, multipart uploads and link fields that `glab`
already got right. None of these applies to GitHub, and heraut already has a GitHub client in this style.
**The user asked to decide with measurements**: T423 spikes A and B against the GitHub sandbox and records
binary size, module and `go.sum` growth, lines of code, and the live behaviour of the flow below. Its
result fills in the transport line of the Decision and moves this ADR to Accepted.

## Decision

Replace `gh` with an in-process client in `internal/platforms/github`. Transport: **decided by T423**
(A or B). The rest holds for either.

- **Publish order.** No assets: one create call with `draft` and `prerelease` as configured or derived. With
  assets: create with `draft: true`, upload each file, then `PATCH draft: false`, skipped when the target
  sets `draft: true`. `prerelease` stays derived from the resolved version (ADR-0064).
- **Failure.** When an upload or the final PATCH fails after the draft exists, heraut deletes the draft
  (best effort) and returns the original error, with any delete failure appended, never in its place. A
  re-run starts clean instead of piling up drafts.
- **Upload URL.** The `upload_url` from the create response is used, but the token is sent only to an
  expected origin over https: `uploads.github.com` for github.com, else the configured host or
  `uploads.<host>`. Any other origin is refused before a byte is sent. The redirect rule applies on top:
  the token is dropped on a host change or an https to http drop, and redirects stop after 10.
- **API root.** A `config` helper derives it from the platform's `base_url`: github.com gives
  `https://api.github.com`; a `*.ghe.com` host (GitHub Enterprise Cloud with data residency) gives
  `https://api.<host>`; any other host (GHES) gives `<base_url>/api/v3`. In GitHub Actions, `GITHUB_API_URL`
  wins only when `GITHUB_SERVER_URL` has the same origin as `base_url`, so an Actions run on github.com
  publishing to a GHES does not take the runner's API. `config.IsSecureURL` is enforced (plain http only
  for loopback addresses, for tests). This closes the `GH_HOST` scheme question.
- **Token.** When `token_env` is set explicitly, only that variable is read. Otherwise heraut follows
  `gh`'s chain: github.com and `*.ghe.com` read `GH_TOKEN`, then `GITHUB_TOKEN`; GHES reads
  `GH_ENTERPRISE_TOKEN`, then `GITHUB_ENTERPRISE_TOKEN`, then `GH_TOKEN` (last, so a GHES config that
  works today with only `GH_TOKEN` keeps working). The token is sent as `Authorization: Bearer`.
  `Check()` and publishing use the same chain, so the Actions-only `GITHUB_TOKEN` special case goes. There
  is no keyring lookup.
- **Assets (NEW-002).** A helper in `internal/platforms` resolves each match, checks confinement on the
  real path, opens that path, and confirms with `os.SameFile` that the descriptor is the file it checked.
  The upload reads from the descriptor and takes `Content-Length` from its `Stat`. The asset name is the
  basename, and duplicate basenames are refused before any request. `Content-Type` comes from the
  extension, else `application/octet-stream`. The GitLab client keeps its `os.Open` in this epic; adopting
  the helper there is a separate roadmap task.
- **`#` in asset paths.** Lifted. The name travels as a query parameter, so `#` is no longer a label
  separator. This supersedes the 2026-10-24 addendum of ADR-0076 for GitHub and resolves NEW-004.
- **`Check()`.** Validates the token with the releases list (above), and no longer probes a binary.

## Consequences

- Phased work: [`docs/tasks/github-client-roadmap.md`](../tasks/github-client-roadmap.md) (Phase 80,
  T423–T430).
- `gh` stops being a heraut runtime dependency: `heraut check runtime` stops probing it, the Docker image
  loses its tools stage, and the specs, `CLAUDE.md`, README and guides lose their `gh` wording.
  `e2e.yml` reads the `gh` version from the Dockerfile today, so it gets its own pin.
- Lane B's `e2e/forgeharness` keeps shelling out to `gh` to manage sandbox fixtures. That is test tooling,
  the same call T368 made for `glab`.
- User-visible: a self-hosted `base_url` must be https (loopback excepted), which the validator already
  enforces; a token in `GITHUB_TOKEN` alone is now enough on github.com; a GHES token can live in
  `GH_ENTERPRISE_TOKEN`; asset paths may contain `#`; a failed upload leaves no draft behind.
- The `gh` argv contract tests become `httptest` contract tests, and Lane A gets a loopback GitHub stub
  (`harness.Repo.FakeGitHub()`), as T366 did for GitLab.
- `config.EnvGitHubCLIToken`'s comment ("the one the `gh` CLI reads") is reworded; the name stays because
  `GH_TOKEN` stays the default.

## Not decided here

- Moving `internal/forge/github` onto the shared API-root helper (it lacks `*.ghe.com`): a task in this
  epic, but its own commit and behaviour change.
- The GitLab client's descriptor-based read (NEW-002 for GitLab).
- Retries and rate-limit backoff (unless T423 picks option B, which brings its own rate-limit errors):
  revisit only if Lane B shows a need.
