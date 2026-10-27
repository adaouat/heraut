# syntax=docker/dockerfile:1

# Tool versions — bump here to upgrade across all stages.
ARG GO_VERSION=1.27
ARG HERAUT_VERSION=dev

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

# ── Stage 2: final image ──────────────────────────────────────────────────────
# debian:trixie-slim over alpine: `git` and `ca-certificates` come from Debian's `apt`
# package index, which requires a glibc-based distro. heraut itself is a static binary
# (CGO_ENABLED=0) and publishes over HTTPS in-process, so git is the only external tool
# (ADR-0077). `golang:trixie` (the builder stage) is kept consistent with this base to
# avoid glibc version surprises.
FROM debian:trixie-slim

ENV HERAUT_CHECK_UPDATE=false

# hadolint ignore=DL3008
RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /heraut /usr/local/bin/heraut

WORKDIR /workspace
ENTRYPOINT ["heraut"]
