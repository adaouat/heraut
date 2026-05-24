FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY . .

# ldflags must match .goreleaser.yml — GoReleaser is the source of truth
ARG VERSION=dev
RUN go build \
    -ldflags "-s -w \
        -X main.Version=${VERSION} \
        -X main.ProjectURL=https://github.com/adaouat/heraut \
        -X main.LatestURL=https://api.github.com/repos/adaouat/heraut/releases/latest" \
    -o /heraut \
    ./cmd/heraut/

FROM alpine:3
RUN apk add --no-cache ca-certificates
COPY --from=builder /heraut /usr/local/bin/heraut
ENTRYPOINT ["heraut"]
