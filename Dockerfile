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
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/hebrew ./cmd/hebrew

# Empty data directory owned by uid 65532. The image is smoked with no volume
# and no extra env, so this path has to be writable by the nonroot user.
# A mounted volume should use the same uid (fsGroup 65532).
RUN mkdir -p /empty-data \
    && touch /empty-data/.keep \
    && chown -R 65532:65532 /empty-data \
    && chmod 755 /empty-data

# distroless static, nonroot (uid 65532).
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/hebrew /usr/local/bin/hebrew
COPY --from=builder /src/internal/placement/NOTICE /usr/share/hebrew/NOTICE.placement
COPY --from=builder --chown=65532:65532 /empty-data /data

ENV DATA_DIR=/data \
    HTTP_ADDR=:8080 \
    MCP_ADDR=:8787

EXPOSE 8080 8787

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/hebrew"]
