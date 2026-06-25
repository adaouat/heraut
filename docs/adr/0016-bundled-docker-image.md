# ADR-0016: Bundled Docker Image (Full Release Runner)

- **Status**: Accepted
- **Date**: 2026-05-25
- **Deciders**: bchatard

---

## Context

ADR-0013 established raw binaries as the primary distribution format. GoReleaser was
also configured to push a minimal Alpine container image to GHCR — just `ca-certificates`
and the heraut binary. That image cannot actually run a release: heraut orchestrates
`git`, `git-cliff`, `gh`, `glab`, and `communique`, none of which are present.

The thin image has no value over `go install` or downloading the raw binary. Its only
realistic use case — running heraut in a container-based CI job — fails immediately
because the external CLIs are missing.

## Decision

Replace the thin GoReleaser-managed image with a single **bundled image** that includes
all tools heraut needs to run a full release. This image:

- Builds heraut from source in a dedicated builder stage (same ldflags as ADR-0013).
- Installs all external CLIs at pinned versions via a `mise`-based stage, then copies
  the plain binaries into the final image (no mise runtime in production).
- Uses `debian:trixie-slim` as the final base to satisfy the glibc requirements of
  dynamically-linked tools (`gh`, `glab`, `communique`).
- Is built and pushed by its own steps in the release workflow using
  `docker/build-push-action` + `docker/metadata-action`, **not** by GoReleaser. The
  `dockers:` block in `.goreleaser.yml` is removed; `Dockerfile.goreleaser` is deleted.

### Bundled tool versions (pinned as Dockerfile ARGs)

| Tool          | Version     | Source                        |
|---------------|-------------|-------------------------------|
| git-cliff     | `2.13.1`    | `.config/mise/config.toml`    |
| glab          | `1.99.0`    | chosen for image              |
| gh            | `2.92.0`    | chosen for image              |
| communique    | `1.1.3`     | chosen for image              |

`git` and `ca-certificates` are installed via `apt` from the Debian package index (no
version pin; they track the Debian stable release for the chosen base).

### Tagging policy

On each `vX.Y.Z` release tag, the workflow pushes four cascading image tags via
`docker/metadata-action`'s built-in semver pattern:

| Tag                 | Example for `v1.4.2`                    |
|---------------------|-----------------------------------------|
| `MAJOR.MINOR.PATCH` | `ghcr.io/adaouat/heraut:1.4.2`          |
| `MAJOR.MINOR`       | `ghcr.io/adaouat/heraut:1.4`            |
| `MAJOR`             | `ghcr.io/adaouat/heraut:1`              |
| `latest`            | `ghcr.io/adaouat/heraut:latest`         |

`latest` and bare `MAJOR` are only pushed from the default branch — pre-releases do not
move them (handled automatically by `metadata-action`'s `is_default_branch` condition).

### Architecture

`linux/amd64` and `linux/arm64` via `docker buildx` + QEMU in CI.

### Base image choice: debian:trixie-slim over alpine

`gh`, `glab`, and `communique` ship dynamically-linked glibc binaries; they do not
provide musl builds. Alpine (musl libc) would require either static builds (not
available for all tools) or a compatibility shim (added complexity, fragile). Debian
trixie-slim (glibc 2.40+) is the simplest compatible base. The `golang:trixie` builder
stage is kept consistent with the final image to avoid glibc version surprises.

## Consequences

**Self-contained CI runner.**  
`docker run --rm -v $(pwd):/workspace ghcr.io/adaouat/heraut:latest release` works
without any setup — all required tools are present at known versions.

**Version isolation.**  
Tool versions are pinned as Dockerfile ARGs. Upgrading git-cliff means bumping one
line; the change is reviewable and trackable in git history.

**GoReleaser scope reduced.**  
GoReleaser now handles only binary builds and GitHub Release creation. Container
publishing is fully decoupled from the binary release pipeline.

**Image size.**  
The image is larger than the previous thin image. This is intentional and unavoidable —
bundling five CLIs costs ~200–400 MB depending on compression. Users who only want the
binary use the raw binary (ADR-0013).

**Tool update lag.**  
Pinned versions mean the image may lag behind the latest patch releases of bundled
tools. This is a conscious trade-off: reproducibility and stability matter more for a
release runner than having the absolute latest patch of every dependency.

**mise not present at runtime.**  
The tools stage uses the mise image purely as a convenient installer; only the plain
binaries are copied into the final image. mise itself is absent from the production
layer — no shims, no version management overhead, no PATH manipulation.
