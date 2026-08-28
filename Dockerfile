# syntax=docker/dockerfile:1

# Tool versions — bump here to upgrade across all stages.
ARG GO_VERSION=1.27
ARG HERAUT_VERSION=dev
ARG MISE_VERSION=2026.5.15
ARG GLAB_VERSION=1.99.0
ARG GH_VERSION=2.92.0

# ── Stage 1: compile heraut ───────────────────────────────────────────────────
FROM golang:${GO_VERSION}-trixie AS builder

ARG HERAUT_VERSION

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# ldflags must stay identical to builds[0].ldflags in heraut/.goreleaser.yml.
# Both inject main.Version; drift here would produce a Docker image that reports
# a different version than the release binaries built by GoReleaser in CI.
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath \
    -ldflags="-s -w -X 'main.Version=${HERAUT_VERSION}'" \
    -o /heraut \
    ./cmd/heraut/

# ── Stage 2: install external tools via mise ─────────────────────────────────
FROM ghcr.io/jdx/mise:${MISE_VERSION} AS tools

ARG GLAB_VERSION
ARG GH_VERSION

# mise use -g installs each tool globally and makes it active.
# mise which resolves the real binary path (not the shim) for clean extraction.
RUN mise use -g \
        glab@${GLAB_VERSION} \
        gh@${GH_VERSION} \
    && mkdir /tools \
    && cp "$(mise which glab)" /tools/glab \
    && cp "$(mise which gh)"   /tools/gh

# ── Stage 3: final image ──────────────────────────────────────────────────────
# debian:trixie-slim over alpine: `git` and `ca-certificates` come from Debian's `apt`
# package index, which requires a glibc-based distro. The bundled CLI binaries impose
# no such requirement of their own — `gh` and `glab` are statically-linked Go binaries
# (verified via `file` on the tools stage's output). `golang:trixie` (the builder stage)
# is kept consistent with this base to avoid glibc version surprises.
FROM debian:trixie-slim

ENV GLAB_SEND_TELEMETRY=false
ENV GLAB_CHECK_UPDATE=false
ENV HERAUT_CHECK_UPDATE=false

# hadolint ignore=DL3008
RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /heraut          /usr/local/bin/heraut
COPY --from=tools   /tools/          /usr/local/bin/

WORKDIR /workspace
ENTRYPOINT ["heraut"]
