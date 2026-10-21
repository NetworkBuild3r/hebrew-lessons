# Go image

`.github/workflows/image-go.yml` builds `Dockerfile.go` and pushes

`ghcr.io/networkbuild3r/hebrew-lessons-go:sha-<full commit sha>`

after the `test` workflow succeeds for that exact commit. There is no `push` trigger. The gate matches `image.yml` as rewritten in PR #18.

- `workflow_run` of `test` (`completed`) runs only when `conclusion` is `success`, `event` is `push`, `head_branch` is `main`, and `head_repository.full_name` is this repository. Checkout and the tag are `github.event.workflow_run.head_sha`, not `github.sha`.
- `workflow_dispatch` takes an optional `sha`. Empty means the current `main` HEAD. The job refuses unless a completed successful `test` run exists for that SHA on `main` with event `push`. A pull-request run does not count.
- A docs-only commit (every path is `docs/**`, `sdd/**`, or a markdown file, with the same `lesson-plan.schema.json` exception as `image.yml`) skips the image build. `paths-ignore` does not apply to `workflow_run`. A manual dispatch of a gated SHA still builds.
- If `go/go.mod` is not in the gated commit, the job prints a notice and succeeds. `go vet`, `go test`, and the image build are skipped.

`ghcr.io/networkbuild3r/hebrew-lessons-go:buildcache` is the buildx registry cache (`mode=max`). It is not a deploy tag.

The Node image is unchanged: `ghcr.io/networkbuild3r/hebrew-lessons:sha-<full sha>` from `image.yml`.

The staging Deployment `hebrew-next` in home_k3 consumes `hebrew-lessons-go`. This repository does not define that Deployment, and this image workflow does not change deploy, cluster, or Cloudflare config.

## Process

The default process is `/usr/local/bin/hebrew` (`go/cmd/hebrew`). Every directory under `go/cmd/` is built into `/usr/local/bin/`. `go/cmd/hebrew-mcp` is optional: the image contains `/usr/local/bin/hebrew-mcp` only when that directory is in the module. A pod selects it with `command: ["/usr/local/bin/hebrew-mcp"]`. PR #13 (Go-1a) has `go/cmd/hebrew` and does not have `go/cmd/hebrew-mcp`.

The image runs as uid/gid 1000 (`USER 1000:1000`). distroless `nonroot` is 65532, and the data volume's files are owned by 1000 (`hebrew-next` `runAsUser` and `fsGroup`, the same uid as `deploy/k8s-database.md`). SQLite WAL creates `-wal` and `-shm` beside `hebrew.sqlite`, including for the read-only open Go-1a uses, so the database directory has to be writable by this uid.

`HEBREW_ROOT=/app`. When that variable is unset, Go uses the working directory; the image also sets `WORKDIR /app`. `HOST=0.0.0.0` and `PORT=8090`. `EXPOSE` is 8090, the same port. Go-1a binds `HOST:PORT` and uses those same defaults when the variables are unset. `hebrew-next` sets `PORT=8080` in the Deployment, which overrides the image env.

## What the image does not contain

No Node asset build. The Go-1 server serves `GET /healthz` only. `200` is JSON `{ok, ledger}`. A ledger mismatch or a failed database ping is `503`. The TypeScript player (`web/player/` to `shared/dist/player.js`) is a later phase and is served from disk, not from `go:embed`.
