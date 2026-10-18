# Go image

`.github/workflows/image-go.yml` builds `Dockerfile.go` and pushes

`ghcr.io/networkbuild3r/hebrew-lessons-go:sha-<full commit sha>`

on a push to `main` that touches `go/**`, `Dockerfile.go`, or the workflow, and on `workflow_dispatch`. A pull request for those paths builds the same tag and does not push it. `ghcr.io/networkbuild3r/hebrew-lessons-go:buildcache` is the buildx registry cache (`mode=max`). It is not a deploy tag.

The Node image is unchanged: `ghcr.io/networkbuild3r/hebrew-lessons:sha-<full sha>` from `image.yml`.

The staging Deployment `hebrew-next` in home_k3 consumes `hebrew-lessons-go`. This repository does not define that Deployment, and this image workflow does not change deploy, cluster, or Cloudflare config.

## Process

The default process is `/usr/local/bin/hebrew` (`go/cmd/hebrew`). A pod selects MCP with `command: ["/usr/local/bin/hebrew-mcp"]` once `go/cmd/hebrew-mcp` is in the module. Every directory under `go/cmd/` is built into `/usr/local/bin/`. The image user is distroless `nonroot` (uid 65532). The binaries are world-executable.

Node listens on `8080` (web) and `8787` (MCP). The Go-1a task uses a different default for the skeleton health server, for example `8090`, so it does not take those ports. The image documents all three. The binary's actual port comes from its config, not from `EXPOSE`.

## What the image does not contain

No Node asset build. The Go-1 server serves `GET /healthz` only. The TypeScript player (`web/player/` to `shared/dist/player.js`) is a later phase and is served from disk, not from `go:embed`.
