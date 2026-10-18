# Staging smoke

Read-only checks against a running hebrew server. The tests do not create users, lessons, or tokens. The MCP check sends `initialize` and `tools/list` only. It does not call a tool.

The files are behind the `smoke` build tag, so `go test ./...` does not run them.

## Run

```bash
go test -tags smoke ./test/smoke/...
```

When the variables below are unset, each check skips and the command exits 0.

## Variables

| Variable | Staging value |
| --- | --- |
| `SMOKE_WEB_URL` | `https://hebrew-next.ibhacked.us` |
| `SMOKE_MCP_URL` | `https://hebrew-mcp-next.ibhacked.us/mcp` |
| `SMOKE_MCP_TOKEN` | read-only bearer token from CI/CD |

`SMOKE_WEB_URL` covers `GET /healthz`, `GET /`, and the OAuth metadata documents. `SMOKE_MCP_URL` covers `GET /healthz` on that host. `initialize` and `tools/list` also need `SMOKE_MCP_TOKEN`; without the token that check skips even if the URL is set.

```bash
export SMOKE_WEB_URL=https://hebrew-next.ibhacked.us
export SMOKE_MCP_URL=https://hebrew-mcp-next.ibhacked.us/mcp
export SMOKE_MCP_TOKEN=...
go test -tags smoke ./test/smoke/...
```
