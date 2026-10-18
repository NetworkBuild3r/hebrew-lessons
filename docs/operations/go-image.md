# Go image

`.github/workflows/image-go.yml` builds `Dockerfile.go` and pushes

`ghcr.io/networkbuild3r/hebrew-lessons-go:sha-<full commit sha>`

on a push to `main` and on `workflow_dispatch`. Those are the same triggers as `image.yml`: no path filter and no `pull_request`. `ghcr.io/networkbuild3r/hebrew-lessons-go:buildcache` is the buildx registry cache (`mode=max`). It is not a deploy tag.

If `go/go.mod` is not in the commit, the job prints a notice and succeeds. `go vet`, `go test`, and the image build are skipped.

The Node image is unchanged: `ghcr.io/networkbuild3r/hebrew-lessons:sha-<full sha>` from `image.yml`.

The staging Deployment `hebrew-next` in home_k3 consumes `hebrew-lessons-go`. This repository does not define that Deployment, and this image workflow does not change deploy, cluster, or Cloudflare config.

## Process

The default process is `/usr/local/bin/hebrew` (`go/cmd/hebrew`). Every directory under `go/cmd/` is built into `/usr/local/bin/`. `go/cmd/hebrew-mcp` is optional: the image contains `/usr/local/bin/hebrew-mcp` only when that directory is in the module. A pod selects it with `command: ["/usr/local/bin/hebrew-mcp"]`. PR #13 (Go-1a) has `go/cmd/hebrew` and does not have `go/cmd/hebrew-mcp`.

The image runs as uid/gid 1000 (`USER 1000:1000`). distroless `nonroot` is 65532, and the data volume's files are owned by 1000 (`hebrew-next` `runAsUser` and `fsGroup`, the same uid as `deploy/k8s-database.md`). SQLite WAL creates `-wal` and `-shm` beside `hebrew.sqlite`, including for the read-only open Go-1a uses, so the database directory has to be writable by this uid.

`HEBREW_ROOT=/app`. When that variable is unset, Go uses the working directory; the image also sets `WORKDIR /app`. `HOST=0.0.0.0` and `PORT=8090`. `EXPOSE` is 8090, the same port. Go-1a binds `HOST:PORT` and uses those same defaults when the variables are unset. `hebrew-next` sets `PORT=8080` in the Deployment, which overrides the image env.

## What the image does not contain

No Node asset build. The Go-1 server serves `GET /healthz` only. `200` is JSON `{ok, ledger}`. A ledger mismatch or a failed database ping is `503`. The TypeScript player (`web/player/` to `shared/dist/player.js`) is a later phase and is served from disk, not from `go:embed`.
