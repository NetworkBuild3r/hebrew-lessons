# syntax=docker/dockerfile:1
#
# Go server image. Build context is the repository root.
# The module is go/ (github.com/NetworkBuild3r/hebrew-lessons/go).
#
# Default GO_VERSION matches go/go.mod on cursor/go-1a-skeleton (PR #13, go 1.26.0).
# .github/workflows/image-go.yml passes GO_VERSION from that file on each build.
# contract/go.mod is a separate module and stays on Go 1.22; it is not this image.
#
# No Node stage. go/cmd/hebrew serves GET /healthz and does not serve the
# TypeScript player. ADR-0016 serves shared/dist/player.js from disk in a
# later phase (go:embed cannot reach outside go/). See docs/operations/go-image.md.
#
# go/cmd/hebrew-mcp is optional. This image includes /usr/local/bin/hebrew-mcp
# only when that directory is in the module. PR #13 does not have it.

ARG GO_VERSION=1.26.0
FROM golang:${GO_VERSION} AS builder

WORKDIR /src/go

COPY go/ ./

# Quoted heredoc: the Dockerfile parser must not expand $ in this script.
# BuildKit leaves $$ as a shell $$, so an inline $(...) loop does not run.
COPY <<'ENDSCRIPT' /tmp/build-go.sh
#!/bin/sh
set -eu

if [ ! -d cmd ]; then
  echo "go/cmd is missing; nothing to build" >&2
  exit 1
fi
if [ ! -d cmd/hebrew ]; then
  echo "go/cmd/hebrew is missing" >&2
  exit 1
fi
if [ -d cmd/hebrew-mcp ]; then
  echo "go/cmd/hebrew-mcp is present; building the MCP binary"
else
  echo "go/cmd/hebrew-mcp is absent; MCP binary is optional and will not be built"
fi

version=${1:-dev}
commit=${2:-unknown}

mkdir -p /out
n=0
for dir in cmd/*; do
  if [ ! -d "$dir" ]; then
    continue
  fi
  name=${dir#cmd/}
  echo "building $name"
  CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${version} -X main.commit=${commit}" \
    -o "/out/${name}" "./${dir}"
  n=$((n + 1))
done
if [ "$n" -eq 0 ]; then
  echo "no main packages under go/cmd" >&2
  exit 1
fi
if [ ! -f /out/hebrew ]; then
  echo "go/cmd/hebrew did not produce the server binary; built:" >&2
  ls -1 /out >&2
  exit 1
fi
ls -l /out
ENDSCRIPT

ARG VERSION=dev
ARG COMMIT=unknown

# Cache mounts are local to the shared docker-builder. The workflow also
# exports a registry cache (mode=max) for the image layers.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    sh /tmp/build-go.sh "$VERSION" "$COMMIT"

# uid 1000, not distroless nonroot (65532).
#
# Data files are owned by uid 1000. hebrew-next staging sets runAsUser and
# fsGroup to 1000, matching deploy/k8s-database.md. SQLite WAL creates
# hebrew.sqlite-wal and hebrew.sqlite-shm next to the database, including
# when the server opens it read-only, so that directory has to be writable
# by the process. 65532 cannot write a directory owned by 1000.
# The nonroot base stays so dropping this USER line does not become root.
FROM gcr.io/distroless/static-debian12:nonroot

# Go uses the working directory when HEBREW_ROOT is unset. Set both.
WORKDIR /app

# PORT is the same number as EXPOSE. Go-1a binds HOST:PORT and defaults to
# 0.0.0.0:8090 when those variables are unset. hebrew-next sets PORT=8080
# in the Deployment; that overrides this ENV.
ENV HEBREW_ROOT=/app
ENV HOST=0.0.0.0
ENV PORT=8090

COPY --from=builder --chown=1000:1000 --chmod=755 /out/ /usr/local/bin/

EXPOSE 8090

USER 1000:1000

# A pod selects MCP, when that binary was built, with
# command: ["/usr/local/bin/hebrew-mcp"]
ENTRYPOINT ["/usr/local/bin/hebrew"]
