# Staging smoke

Read-only checks against a running hebrew server. The tests do not create users, lessons, or tokens. The MCP check sends `initialize` and `tools/list` only. It does not call a tool.

The files are behind the `smoke` build tag, so `go test ./...` does not run them.

## Run

```bash
go test -tags smoke ./test/smoke/...
```

When the variables below are unset, each check skips and the command exits 0.

The host check in `guard.go` runs first, including from `go test ./...` without the smoke tag. `TestMain` refuses to start when `SMOKE_WEB_URL` or `SMOKE_MCP_URL` is set to a host outside the allow-list, unless `SMOKE_ALLOW_PROD=1`. Allowed without that flag: `hebrew-next.ibhacked.us`, `hebrew-mcp-next.ibhacked.us`, `localhost`, and `127.0.0.1` (any port). Production is `hebrew.ibhacked.us` and `hebrew-mcp.ibhacked.us`. Any other host is refused the same way. The flag must be the string `1`. An unparseable URL is refused even when the flag is set. An empty URL is fine, because the checks then skip.

## Variables

| Variable | Staging value |
| --- | --- |
| `SMOKE_WEB_URL` | `https://hebrew-next.ibhacked.us` |
| `SMOKE_MCP_URL` | `https://hebrew-mcp-next.ibhacked.us/mcp` |
| `SMOKE_MCP_TOKEN` | the deployment's `MCP_READONLY_TOKEN` |
| `SMOKE_ALLOW_PROD` | unset. Set to `1` only when the URL is production or another non-staging host on purpose |

`SMOKE_WEB_URL` covers `GET /healthz`, `GET /`, and the OAuth metadata documents. `SMOKE_MCP_URL` covers `GET /healthz` on that host. `initialize` and `tools/list` also need `SMOKE_MCP_TOKEN`; without the token that check skips even if the URL is set.

```bash
export SMOKE_WEB_URL=https://hebrew-next.ibhacked.us
export SMOKE_MCP_URL=https://hebrew-mcp-next.ibhacked.us/mcp
export SMOKE_MCP_TOKEN=...
go test -tags smoke ./test/smoke/...
```

A production URL fails before any request:

```bash
SMOKE_WEB_URL=https://hebrew.ibhacked.us go test -tags smoke ./test/smoke/...
```

```bash
SMOKE_ALLOW_PROD=1 SMOKE_WEB_URL=https://hebrew.ibhacked.us go test -tags smoke ./test/smoke/...
```
