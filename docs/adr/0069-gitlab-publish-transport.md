# ADR-0069: GitLab publish transport — replace `glab` with `client-go`

- **Status**: Accepted
- **Date**: 2026-10-11
- **Deciders**: bchatard

---

## Context

`internal/platforms/gitlab` publishes by shelling out to `glab release create` through
`port.Runner`. ADR-0043 deferred the publish transport ("stdlib vs SDK") to its own decision;
its P3 shipped as config unification only (ADR-0044). PR/MR enrichment, by contrast, already talks to
GitLab directly over `net/http` (`internal/forge/gitlab`, `JOB-TOKEN` / `PRIVATE-TOKEN`).

Motivation for revisiting it (T347): drop `glab` as a runtime dependency (`heraut check runtime`,
the bundled Docker image, the Lane A fake-CLI harness), stop tracking `glab` version drift (the
image pins 1.99.0, the version probed for this ADR was 1.121.0), and own auth and error mapping in
one place. The original extra motivation, a package-registry version without `/` for per-env tags,
is mostly moot: T346 removed the `--use-package-registry` path, and today assets ride as project
uploads, which accept any tag shape (T335).

### What `glab release create` does (probed live, gitlab.com, `glab` 1.121)

1. Creates the tag from the default branch when it is missing (heraut tags and pushes first, so
   this never fires in a heraut run).
2. Uploads each asset with `POST /projects/:id/uploads` (multipart): project uploads, no package
   registry.
3. Creates the release (`POST /projects/:id/releases`, notes as `description`) and adds one asset
   link per upload (Release-links API).

Reproduced with raw REST (`curl`): upload, then create-release with `assets.links[]`. Setting
`filepath: "/<file>"` on a link yields the same stable `/releases/<tag>/downloads/<file>` URL
`glab` produces. A `/` in the tag works end to end. All of it worked with `PRIVATE-TOKEN`.

### Measured: the CI job token (T364, real GitLab CI on gitlab.com, shared runner)

| call under `CI_JOB_TOKEN` (`JOB-TOKEN` header) | result |
|---|---|
| `POST /projects/:id/uploads` | 401 |
| `GET /user` | 401 |
| `GET /projects/:id` | 404 |
| Generic Packages `PUT` | 201 |
| Generic Packages `PUT`, `/` in the version | 400 `package_version is invalid` |
| `POST /releases` (slash tag, tag created from the SHA) | 201 |
| release link to a Generic Packages URL with `direct_asset_path` | 201, stable `/downloads/` URL |
| `GET` / `DELETE` of a release | 200 |

`glab release create <tag> a.txt` in the same CI, `glab` 1.99.0 (the pinned version) and 1.121.0
behaved identically:

- With `GLAB_ENABLE_CI_AUTOLOGIN=true`: the release is created, then the upload fails with 401 and
  `glab` exits 1, leaving a release without assets.
- Without it, which is what heraut runs today (it never sets the variable): 404, nothing created.

So **the current GitLab CI path with assets is already broken**; only a release without assets works
(autologin on). A port is not a regression, and it can fix this: the Generic Packages API accepts
the job token. Not covered: `glab` with a project access token in `GITLAB_TOKEN` in CI (heraut
skips token injection under `inCIAutologin()`, so it is expected to fail the same way).

### Measured: `client-go` cost (v1.46.0, stripped, darwin/arm64)

Probe touching `Releases`, `ReleaseLinks`, `ProjectMarkdownUploads`, `Users` and the job-token client
(`GenericPackages` was not in the probe, but it hangs off the same `Client`, so the delta is representative):
binary 19.6 MB to 23.1 MB (+3.5 MB, +18%); 348 to 355 packages; `go.mod` +1 direct and +5 indirect
modules (`go-querystring`, `go-cleanhttp`, `go-retryablehttp`, `oauth2`, `x/time`); `go.sum`
136 to 159 lines. `glab` itself is never bundled into the heraut binary; it is a separate install
(and a ~47 MB layer in the Docker image).

## Options

| | A. Native `net/http` | B. `gitlab.com/gitlab-org/api/client-go` | C. Stay on `glab` |
|---|---|---|---|
| Removes `glab` dependency | yes | yes | no |
| Request-shape risk (multipart, link fields, `filepath` vs `direct_asset_path`) | ours: ~150 lines, tested against `httptest` | low: the library `glab` itself uses | none |
| Auth (`JOB-TOKEN` / `PRIVATE-TOKEN`) | hand-rolled switch, as in `forge/gitlab` | built in (job-token and PAT constructors) | `glab` autologin / env |
| Retries, rate-limit backoff | ours to write | built in | built in |
| Dependency weight | none | large: wraps the whole API for ~3 services; **not yet measured** | none (external binary) |
| Fit with existing code | same style as `forge/gitlab` | second HTTP stack beside the hand-rolled clients | n/a |
| Job-token upload limit | unchanged | unchanged | unchanged |
| Contract tests | `httptest.Server` | `httptest.Server` (the client takes a base URL) | `MockRunner` argv |

A and B differ in who owns the request code, not in server-side behaviour: neither changes what
GitLab accepts for a job token.

### Layering constraint

`platforms/*` may import only `port` and `config`, and `forge/*` likewise; neither may import the
other. So the publish client cannot simply "extend `internal/forge/gitlab`" and be called from
`platforms/gitlab`. Either the client lives inside `platforms/gitlab` (duplicating the small auth
switch), or the shared HTTP/auth helper moves to a leaf both can import. Under option B the
question shrinks to the dependency itself, imported only by `platforms/gitlab`.

### GitHub asymmetry

`gh` stays. GitHub asset upload goes through `gh release create` with files attached, and HTTP 422
on uploads to a published release shaped that flow (T346). Porting GitLab alone leaves heraut with
one forge on CLI publishing and one on HTTP. That is acceptable: the two drivers share only the
`port.Platform` contract, and GitHub gets its own decision if the same motivation appears. This ADR
does not decide it.

## Decision

Replace `glab` with `gitlab.com/gitlab-org/api/client-go` (option B) for create-release, asset
upload and asset links, and port `Check()`, CI auth and self-hosted base-URL handling onto it. The
request shapes are the ones `glab` already uses in production, retries and rate-limit backoff come
built in, and the cost (+3.5 MB, five indirect modules) is small next to dropping a separately
installed CLI. Native `net/http` was the close alternative; it adds nothing to the binary but makes
heraut own the multipart, link and retry code.

- **Client placement:** the dependency is imported only by `platforms/gitlab` (layering constraint
  above); `internal/forge/gitlab` keeps its hand-rolled enrichment clients unchanged.
- **Auth ("autologin"):** when `GITLAB_CI=true` and `CI_JOB_TOKEN` is set (and the base URL is the
  CI instance, the existing `inCIAutologin()` rule), build `NewJobClient(CI_JOB_TOKEN,
  WithBaseURL(CI_SERVER_URL))`; otherwise `NewClient(<token_env>)` with the configured base URL. No
  `GLAB_ENABLE_CI_AUTOLOGIN` equivalent exists to forget.
- **Assets:** one route for every auth type: `GenericPackages.PublishPackageFile`, then a release
  link whose `direct_asset_path` is `/<file>`, so download URLs stay stable. The package version is
  the tag with `/` replaced (per-env tags); the allowed character set beyond `/` is verified in
  T365 before the sanitizer is fixed. `/uploads` is not used: it rejects the job token.
- **`Check()`:** cannot use `GET /user` or `GET /projects/:id` under a job token; it validates with
  a call that token can reach (the Releases list), confirmed in T366.
- **Failure ordering:** upload assets first, then create the release with its links in the same
  request, so a failed upload leaves no asset-less release behind.

## Consequences

- Phased work is T365 to T368 in `docs/tasks/roadmap.md` (Phase 61); T364 (the gate) is done.
- Fixes GitLab publishing with assets in GitLab CI, which fails today.
- `heraut check runtime` stops checking `glab`; the Docker image drops it; the `glab` argv contract
  tests become `httptest` contract tests; Spec 02 and Spec 05 lose their `glab` wording.
- Lane B's GitLab scenarios stay as the regression suite; a GitLab CI job covers the job-token path.
- Asset downloads come from the package registry instead of project uploads (URLs unchanged through
  `direct_asset_path`); existing releases are unaffected.
- Heraut gains `client-go` and its indirect modules in `go.mod`, covered by the OSV scan.
- Self-hosted behaviour needs explicit checks (custom CA and proxy handling, `base_url` versus
  `CI_SERVER_URL`); `glab` provided these implicitly.

## Not decided here

- The GitHub publish transport.
- Fixing the current `glab` path (`GLAB_ENABLE_CI_AUTOLOGIN`, upload 401): it is replaced, not
  patched.
