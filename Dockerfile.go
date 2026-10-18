# syntax=docker/dockerfile:1
#
# Go server image. Build context is the repository root.
# The module is go/ (github.com/NetworkBuild3r/hebrew-lessons/go).
#
# Default GO_VERSION matches go/go.mod on cursor/go-1a-skeleton (go 1.25.0).
# .github/workflows/image-go.yml passes GO_VERSION from that file on each build.
# contract/go.mod is a separate module and stays on Go 1.22; it is not this image.
#
# No Node stage. Go-1 serves GET /healthz from go/cmd/hebrew and does not serve
# the TypeScript player. ADR-0016 serves shared/dist/player.js from disk in a
# later phase (go:embed cannot reach outside go/). See docs/operations/go-image.md.

ARG GO_VERSION=1.25.0
FROM golang:${GO_VERSION} AS builder

WORKDIR /src/go

COPY go/ ./

ARG VERSION=dev
ARG COMMIT=unknown

# Cache mounts are local to the shared docker-builder. The workflow also
# exports a registry cache (mode=max) for the image layers.
# $$ keeps shell variables for /bin/sh. VERSION and COMMIT are Docker ARGs,
# exported into this RUN, and expanded by the shell.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -eu; \
    if [ ! -d cmd ]; then \
      echo "go/cmd is missing; nothing to build" >&2; \
      exit 1; \
    fi; \
    mkdir -p /out; \
    n=0; \
    for dir in cmd/*; do \
      if [ ! -d "$$dir" ]; then \
        continue; \
      fi; \
      name=$$(basename "$$dir"); \
      echo "building $$name"; \
      CGO_ENABLED=0 go build -trimpath \
        -ldflags "-s -w -X main.version=$$VERSION -X main.commit=$$COMMIT" \
        -o "/out/$$name" "./$$dir"; \
      n=$$((n + 1)); \
    done; \
    if [ "$$n" -eq 0 ]; then \
      echo "no main packages under go/cmd" >&2; \
      exit 1; \
    fi; \
    if [ ! -f /out/hebrew ]; then \
      echo "go/cmd/hebrew did not produce the server binary; built:" >&2; \
      ls -1 /out >&2; \
      exit 1; \
    fi; \
    ls -l /out

# nonroot is uid 65532. Binaries are world-executable, so a pod runAsUser
# (home_k3 uses 1000 for the Node deployment) can still exec them.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder --chown=65532:65532 /out/ /usr/local/bin/

# 8080 web, 8787 MCP (Node today; Go later). 8090 is the Go-1a task's
# example health port so the skeleton does not bind those two.
EXPOSE 8080 8787 8090

# k8s command replaces this. MCP: ["/usr/local/bin/hebrew-mcp"]
ENTRYPOINT ["/usr/local/bin/hebrew"]
