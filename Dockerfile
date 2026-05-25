FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY . .

# ldflags must match .goreleaser.yml — GoReleaser is the source of truth
ARG VERSION=dev
RUN go build \
    -ldflags "-s -w -X main.Version=${VERSION}" \
    -o /heraut \
    ./cmd/heraut/

FROM alpine:3
RUN apk add --no-cache ca-certificates
COPY --from=builder /heraut /usr/local/bin/heraut
ENTRYPOINT ["heraut"]
