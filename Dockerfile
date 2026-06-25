# syntax=docker/dockerfile:1

# Tool versions — bump here to upgrade across all stages.
ARG GO_VERSION=1.26
ARG HERAUT_VERSION=dev
ARG MISE_VERSION=2026.5.15
ARG GIT_CLIFF_VERSION=2.13.1
ARG GLAB_VERSION=1.99.0
ARG GH_VERSION=2.92.0
ARG COMMUNIQUE_VERSION=1.1.3

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

ARG GIT_CLIFF_VERSION
ARG GLAB_VERSION
ARG GH_VERSION
ARG COMMUNIQUE_VERSION

# mise use -g installs each tool globally and makes it active.
# mise which resolves the real binary path (not the shim) for clean extraction.
RUN mise use -g \
        git-cliff@${GIT_CLIFF_VERSION} \
        glab@${GLAB_VERSION} \
        gh@${GH_VERSION} \
        communique@${COMMUNIQUE_VERSION} \
    && mkdir /tools \
    && cp "$(mise which git-cliff)"  /tools/git-cliff \
    && cp "$(mise which glab)"       /tools/glab \
    && cp "$(mise which gh)"         /tools/gh \
    && cp "$(mise which communique)" /tools/communique

# ── Stage 3: final image ──────────────────────────────────────────────────────
# trixie matches the mise tools stage (Debian 13, glibc 2.40+), ensuring
# dynamically-linked binaries like communique resolve their glibc requirements.
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
