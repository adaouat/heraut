# ADR-0069: GitLab publish transport — replace `glab` with a native API client

- **Status**: Proposed
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

### The open risk: the CI job token

Under `CI_JOB_TOKEN` (`JOB-TOKEN` header), GitLab's job-token documentation lists the Releases and
Release-links APIs as accessible but does **not** list `POST /projects/:id/uploads`. `glab` uses
the same upload endpoint (`ProjectMarkdownUploads.UploadProjectMarkdown`) and the same header under
CI autologin, so a native port inherits whatever `glab` does today; it is not a regression.
Neither behaviour has been observed: Lane B runs from a laptop with a PAT, and no GitLab CI
job has exercised either path.

A related unknown on the *current* path: `glab`'s help says CI autologin needs
`GLAB_ENABLE_CI_AUTOLOGIN=true`. Heraut's `inCIAutologin()` checks only `GITLAB_CI=true`, and
neither heraut nor the Dockerfile sets that variable. Whether the pinned 1.99.0 autologins by
default is unchecked.

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

## Decision (proposed)

Replace `glab` with a native API client for create-release, asset upload and asset links, and port
`Check()`, CI auth and self-hosted base-URL handling onto it. The transport (A or B) and the
job-token behaviour are **not decided here**; they are settled by two measurements that gate the
rest of the work (T364):

1. Add `client-go` on a scratch branch and record binary size and new `go.sum` entries. If the
   delta is small, prefer B for its lower correctness risk; if large, prefer A.
2. Run a throwaway GitLab CI job in a sandbox project that uploads and creates a release with
   `CI_JOB_TOKEN`, and also with `glab` at the pinned version with and without
   `GLAB_ENABLE_CI_AUTOLOGIN`. Record the HTTP status of each call.

If `/uploads` rejects the job token, the fallbacks are: (a) require a project access token for
assets in CI, (b) publish through the Generic Packages API (accepts the job token) with a package
version sanitized of `/`, or (c) create the release without file assets and document it. The
result, and the chosen transport, update this ADR to Accepted.

## Consequences

- Phased work is tracked as T364 to T368 in `docs/tasks/roadmap.md` (Phase 61).
- If adopted: `heraut check runtime` stops checking `glab`; the Docker image drops it; the
  `glab` argv contract tests become `httptest` contract tests; Spec 02 and Spec 05 lose their
  `glab` wording.
- Lane B's GitLab scenarios stay as the regression suite for the swap.
- Until T364 reports, nothing changes in behaviour.
- Self-hosted behaviour needs explicit checks (custom CA and proxy handling, `base_url` versus
  `CI_SERVER_URL`); `glab` provided these implicitly.

## Not decided here

- The GitHub publish transport.
- Whether to also fix `inCIAutologin()` / `GLAB_ENABLE_CI_AUTOLOGIN` on the current `glab` path;
  T364's findings decide whether that needs its own task.
