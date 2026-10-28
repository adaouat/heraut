# ADR-0077: GitHub publish transport — replace `gh` with `go-github`

- **Status**: Accepted
- **Date**: 2026-10-25 (accepted 2026-10-26, after the T423 gate)
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
| Dependency weight | none | measured by T423, below | external binary, a Docker image stage |
| Upgrade churn | none | frequent major versions, measured below | `gh` version drift |
| Contract tests | `httptest.Server` | `httptest.Server` (base and upload URLs are settable) | `MockRunner` argv |

ADR-0069 chose the SDK for GitLab because of job-token auth, multipart uploads and link fields that `glab`
already got right. None of these applies to GitHub, and heraut already has a GitHub client in this style.
**The user asked to decide with measurements**, so T423 built the flow twice in a scratch worktree, linked
each into the heraut binary, and ran both against GitHub.

### Measured (T423, go1.27.3, stripped, darwin/arm64)

| | A. `net/http` | B. `go-github` v93.0.0 |
|---|---|---|
| Binary (baseline 23.32 MB) | 23.37 MB (+51 KB, +0.2%) | 27.93 MB (+4.6 MB, +19.8%) |
| Packages linked (baseline 357) | 359 | 360 (`go-github` is one large package) |
| `go.mod` / `go.sum` | unchanged | +1 direct module, no indirect module, +2 lines |
| Client code, without tests | 217 lines | 82 lines |
| Major versions | n/a | v60 (2024-02) to v93 (2026-10): 33 majors in about 32 months |

`go-github` v93 already does what the Decision asks of the token and the upload body:
`WithAuthToken` attaches the token only to the configured API and upload origins, checked on every request
(redirect hops included; the scheme is part of the origin, so https to http drops it), and
`UploadReleaseAssetFromRelease` refuses to send a body to any other origin (`ErrUntrustedDestination`). The
upload origin has to be configured, so a GHES with subdomain isolation (`uploads.<host>`) needs that origin
known before the upload.

### Observed live (T423, github.com, both options identical)

Sandbox `bchatard/heraut-testing`, then a throwaway private repository with immutable releases on:

- `upload_url` is on `uploads.github.com`. Slash tags (`uat/0.0.1`) and `+` tags (`v0.0.2+a.1`) work.
- No assets: one create call, `prerelease` honoured. With assets: draft, upload, publish works, also with
  immutable releases on (the release comes back `immutable: true`). `draft: true` leaves a draft whose
  `html_url` is `…/releases/tag/untagged-…`, not the tag URL heraut prints (true of the `gh` driver too).
- A failed upload (duplicate name: 422 `already_exists`) followed by deleting the draft leaves the tag
  in place and no release.
- Uploading to a published immutable release: 422 `Cannot upload assets to an immutable release.` Deleting
  the tag of a published immutable release is refused (repository rule violation).
- GitHub rewrites special characters in an asset name: `meta#1.json` is published as `meta.1.json`.

## Decision

Replace `gh` with an in-process client in `internal/platforms/github` on **`github.com/google/go-github`
(option B)**, imported only by that package. The user chose it over option A after the gate: the library
owns the request shapes, the error types and the primary and secondary rate-limit handling, and already
enforces the origin rules below. The cost is +4.6 MB and a major version roughly every month, kept local
because one package imports it.

- **Publish order.** No assets: one create call with `draft` and `prerelease` as configured or derived. With
  assets: create with `draft: true`, upload each file, then `PATCH draft: false`, skipped when the target
  sets `draft: true`. `prerelease` stays derived from the resolved version (ADR-0064).
- **Failure.** When an upload or the final PATCH fails after the draft exists, heraut deletes the draft
  (best effort) and returns the original error, with any delete failure appended, never in its place. A
  re-run starts clean instead of piling up drafts.
- **Upload URL.** The `upload_url` from the create response is used, but the token is sent only to an
  expected origin over https: `uploads.github.com` for github.com, else the configured host or
  `uploads.<host>`. Any other origin is refused before a byte is sent. heraut checks the returned origin
  against that list, then has `go-github` upload with that origin configured, so its own
  `ErrUntrustedDestination` gate and per-request token rule apply as well. The token never follows a
  redirect to another origin (the library's rule, which covers the T419 cases); a redirect test in T426
  pins it.
- **API root.** A `config` helper derives it from the platform's `base_url`: github.com gives
  `https://api.github.com`; a `*.ghe.com` host (GitHub Enterprise Cloud with data residency) gives
  `https://api.<host>`; any other host (GHES) gives `<base_url>/api/v3`. `GITHUB_API_URL` wins only when
  `GITHUB_ACTIONS=true` and `GITHUB_SERVER_URL` has the same origin as `base_url` (scheme, host, port), so
  an Actions run on github.com publishing to a GHES does not take the runner's API, and stray variables
  outside Actions are ignored. `config.IsSecureURL` is enforced (plain http only for loopback addresses,
  for tests); a `GITHUB_API_URL` that passes the origin test but fails it is an error, not a silent
  fallback to the derived root. This closes the `GH_HOST` scheme question.
- **Token.** When `token_env` is set explicitly, only that variable is read. Otherwise heraut follows
  `gh`'s chain: github.com and `*.ghe.com` read `GH_TOKEN`, then `GITHUB_TOKEN`; GHES reads
  `GH_ENTERPRISE_TOKEN`, then `GITHUB_ENTERPRISE_TOKEN`, then `GH_TOKEN` (last, so a GHES config that
  works today with only `GH_TOKEN` keeps working). The token is sent as `Authorization: Bearer`.
  `Check()` and publishing use the same chain, so the Actions-only `GITHUB_TOKEN` special case goes. There
  is no keyring lookup.
- **Assets (NEW-002).** A helper in `internal/platforms` resolves each match, checks confinement on the
  real path, opens that path through an `os.Root` on the repository root (so a symlink swapped in after the
  check cannot lead outside it), and confirms with `os.SameFile` that the descriptor is the file it checked.
  The upload reads from the descriptor and takes `Content-Length` from its `Stat`. The asset name is the
  basename, and duplicate basenames are refused before any request. `Content-Type` comes from the
  extension, else `application/octet-stream`. The GitLab client keeps its `os.Open` in this epic; adopting
  the helper there is a separate roadmap task.
- **`#` in asset paths.** Lifted. The name travels as a query parameter, so `#` is no longer a label
  separator; GitHub rewrites it (and other special characters) to `.` in the published name. heraut
  compares the name GitHub returns with the file's basename and prints a warning when they differ, for any
  rewritten character. This supersedes the 2026-10-24 addendum of ADR-0076 for GitHub and resolves NEW-004.
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
  `GH_ENTERPRISE_TOKEN`; asset paths may contain `#` (published under GitHub's rewritten name, with a
  warning); a failed upload leaves no draft behind.
- Heraut gains `go-github` in `go.mod`, covered by the OSV scan. Renovate will propose a major bump
  about monthly; each one changes the import path in `internal/platforms/github` only.
- The `gh` argv contract tests become `httptest` contract tests, and Lane A gets a loopback GitHub stub
  (`harness.Repo.FakeGitHub()`), as T366 did for GitLab.
- `config.EnvGitHubCLIToken`'s comment ("the one the `gh` CLI reads") is reworded; the name stays because
  `GH_TOKEN` stays the default.

## Not decided here

- Moving `internal/forge/github` onto the shared API-root helper (it lacks `*.ghe.com`): a task in this
  epic, but its own commit and behaviour change.
- The GitLab client's descriptor-based read (NEW-002 for GitLab).
- Retries beyond what `go-github` does for rate limits: revisit only if Lane B shows a need.

## Addendum (2026-10-28): how GitHub renames asset names

The Decision records one rewrite, `#` to `.`. A code review found that the duplicate-name check compared
names as given, so two files GitHub renames into one passed the pre-release check and the release failed
after the tag push (422 `already_exists`). On the user's go, T438 probed github.com: about 60 names
uploaded to a draft release on the GitHub sandbox, published names read back, the draft deleted (no tag
is created for a draft). Observed:

- kept: ASCII letters, digits, `.`, `_`, `-`, `+`, `@`, and the case;
- every other ASCII character, and space, becomes `.`; runs of `.` collapse, original dots included
  (`dd..x.txt` → `dd.x.txt`);
- a trailing `.` or space is dropped (`trail.txt.` → `trail.txt`); a leading `.` gets `default` in
  front (`.hidden` → `default.hidden`); a name left empty becomes `default` (`日本.txt` → `default.txt`);
- letters are transliterated (`é` → `e`, `ß` → `ss`, `ø` → `o`, `æ` → `ae`, `Ł` → `L`); other
  non-ASCII characters (`€`, emoji, CJK) become `.`;
- names compare **without case**: `Case.txt` then `case.txt`, and `Straße.txt` then `strasse.txt`, are
  refused with 422 `already_exists`.

One outlier is not modelled: `x.` was published as `default.x`, while `trail.txt.` lost its dot.
`publishedName` in `internal/platforms/github` encodes the rest (its test table is the observed
list), and the GitHub driver's `CheckAssets` refuses two basenames whose lowercased published names
match. The model only decides collisions; the published name is still read from GitHub's answer.
GitHub documents that it renames such names, not how. If it changes the rules, the model can miss a
collision (the old failure, after the tag push) or refuse two names GitHub would now keep apart; the
remedy for the latter is renaming one file. Rerun the probe before changing the model.

