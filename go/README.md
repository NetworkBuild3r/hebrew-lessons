# hebrew-lessons Go server

One module, `github.com/NetworkBuild3r/hebrew-lessons/go`, for the server port
(ADR-0016). Build with `CGO_ENABLED=0`.

The Go version is 1.26.0. `github.com/modelcontextprotocol/go-sdk` v1.8.0
requires 1.25.0, and ADR-0016 says not to pin lower. `modernc.org/sqlite`
v1.60.1 requires 1.26.0, so the module sits at that higher floor.

## Layout

Server code lives under `internal/`. Package names are short and lowercase.
Commands live under `cmd/`.

Do not add these packages in this module until the owner of that work lands
them. They are reserved:

| Package | Owner |
|---|---|
| `internal/plans`, `internal/jsstr` | Validators (parallel PR) |
| `internal/lessondata` | Lesson-file parser (parallel PR) |
| `internal/mcp` | MCP server |
| `internal/web` | Frontend. Views in `internal/web/views/`, page handlers in `internal/web/pages/` |

## Dependencies

Stdlib first. `golang.org/x` is allowed. Anything else needs a justification in
the pull request. `modernc.org/sqlite` is the pure-Go SQLite driver
(`CGO_ENABLED=0`). Do not add a second SQLite driver.

## What this module serves

`cmd/hebrew` loads config, opens `DATA_DIR/hebrew.sqlite` read-only, checks the
migration ledger once, and serves `GET /healthz` on `HOST:PORT`. `200` is
`{"ok":true,"ledger":...}`. A ledger mismatch is `503` with `ok` false, and the
process still starts. A missing database exits. Each `/healthz` returns the
cached ledger and pings the database; it does not re-read `schema_migrations`.

Go does not migrate and does not open a writer. The process does not need a
shell, Node, or the repo tree. The image is a static binary (`CGO_ENABLED=0`)
and runs as distroless `nonroot` (uid 65532).

| Variable | Default |
|---|---|
| `HOST` | `0.0.0.0`, so Kubernetes probes and the Service can connect. Set `HOST=127.0.0.1` for local use. |
| `PORT` | `8090` when unset. Staging sets `PORT=8080`. Node's own default is 8080; MCP uses 8787. |
| `HEBREW_ROOT` | Working directory when unset. Node's unset fallback is the parent of `server/`, which is not in the image. The image sets `HEBREW_ROOT=/app`. Relative `DATA_DIR` and `LESSONS_DIR` resolve against the working directory, as in Node. |
| `DATA_DIR` | `$HEBREW_ROOT/data` |
| `LESSONS_DIR` | `$HEBREW_ROOT/worksheets` |
| `COOKIE_SECURE` | false unless the value is exactly `1` |
| `TRUSTED_PROXIES` | empty: trust nobody. Comma-separated CIDRs or host addresses (ADR-0016). Node does not read this variable. |

`DATA_DIR` must not sit inside `shared/`, `review/`, or `LESSONS_DIR`.
The database file is `$DATA_DIR/hebrew.sqlite`. Media is `$DATA_DIR/media`.
`internal/paths` hard-codes those rules and does not read `server/paths.js`.
`go test` golden-checks them with `testdata/golden-paths.js`, which is CI-only.
The migration helper under `internal/db/testdata/` is also CI-only.

## Checks

From `go/`:

```
gofmt -l .
go vet ./...
staticcheck ./...
go test ./...
```

`gofmt -l` must print nothing.
