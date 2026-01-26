# syntax=docker/dockerfile:1
ARG GO_VERSION=1.25.0

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -buildvcs=false -trimpath -ldflags="-s -w -X github.com/algolia/docli/pkg/cmd/root.version=v0.84-docker" -o /out/docli ./main.go

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /out/docli /usr/local/bin/docli

USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/docli"]
