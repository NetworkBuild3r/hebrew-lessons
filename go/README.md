# hebrew-lessons Go server

One module, `github.com/NetworkBuild3r/hebrew-lessons/go`, for the server port
(ADR-0016). Build with `CGO_ENABLED=0`.

The Go version is 1.25.0 because `github.com/modelcontextprotocol/go-sdk` v1.8.0
requires it. ADR-0016 says not to pin lower.

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

## Checks

From `go/`:

```
gofmt -l .
go vet ./...
staticcheck ./...
go test ./...
```

`gofmt -l` must print nothing.
