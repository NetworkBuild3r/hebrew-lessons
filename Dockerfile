# syntax=docker/dockerfile:1
#
# Image repository: ghcr.io/networkbuild3r/hebrew
# Never tag or push ghcr.io/networkbuild3r/hebrew-lessons. The production
# auto-updater watches that name. scripts/check-image-name.sh is the guard
# CI runs before a push.

ARG GO_VERSION=1.26.9
FROM golang:${GO_VERSION} AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/hebrew ./cmd/hebrew \
    && mkdir -p /out/data/audio \
    && chown -R 65532:65532 /out/data

# distroless static, nonroot (uid 65532). A mounted data volume needs to be
# writable by that uid (fsGroup 65532).
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/hebrew /usr/local/bin/hebrew
COPY --from=builder --chown=nonroot:nonroot /out/data /data

ENV DATA_DIR=/data \
    HTTP_ADDR=:8080 \
    MCP_ADDR=:8787

EXPOSE 8080 8787

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/hebrew"]
