# Contract suite

Black-box tests that freeze the Node server's HTTP and MCP behavior so a later Go port can be gated on the same goldens. The harness is a separate Go module. It uses the standard library only (`go test` and `net/http`) and does not import the MCP SDK or any server code.

PR1 covers the harness, transport and auth basics, `initialize`, and the canonical `tools/list` for all 42 tools. Later PRs add the tool calls, error matrix, OAuth, web routes, and the Go server.

## Run

From the repo root, with Node 22 and Go installed:

```sh
contract/run.sh
```

The runner pins `TZ=UTC`, creates a fresh `DATA_DIR`, starts `CONTRACT_SERVER_CMD` (default `node server/start.js`), waits until the web port answers and `POST /mcp` returns 401, runs `go test ./...`, and deletes the data directory. Web listens on `PORT` (default 18080) and MCP on `MCP_PORT` (default 18787).

`CONTRACT_SERVER_CMD` is any shell command: a binary (`./bin/hebrew-server`), a binary with arguments, or a wrapper script. The runner executes it as `bash -c "$CONTRACT_SERVER_CMD"` with the working directory set to the repo root, and with the environment below. The process has to stay in the foreground in the process group the runner created. A server that daemonizes is not stopped on teardown, and the ports stay held.

On Linux the runner uses `setsid` when it is on `PATH`, so the command is its own session and process group. macOS has no `setsid`. There the runner turns on bash monitor mode for the launch only, which gives the command its own process group, then turns monitor mode off again. Teardown is the same on both: signal that group, signal any descendant that left the group, then `SIGKILL` if it is still alive. No extra packages are required.

`LESSONS_DIR` is left unset. Node then serves the repo `worksheets/` directory in place. Do not point `LESSONS_DIR` at a partial tree: `server/start.js` copies any missing seed lesson into `LESSONS_DIR`. There is no `contract/fixtures/lessons` tree in this suite. Nothing read it. A later seed phase can add a trimmed tree; that tree must omit the adult `anal-sex-vocab` lesson and audio blobs.

If `CONTRACT_WEB_URL`, `CONTRACT_MCP_URL`, `CONTRACT_MCP_TOKEN`, and `CONTRACT_MCP_PUBLIC_URL` are already set, the runner does not boot a server. It only runs the tests.

## Environment the server must honor

The runner exports these before it starts the command. A Go server dropped in via `CONTRACT_SERVER_CMD` has to follow the same contract or the goldens and transport checks will fail.

| Variable | Runner default | What the server must do |
| --- | --- | --- |
| `TZ` | `UTC` | Honor it for date-sensitive behavior |
| `DATA_DIR` | fresh temp directory | Put writable state here (database, media, OAuth files) |
| `HOST` | `127.0.0.1` | Bind the web listener and the MCP listener |
| `PORT` | `18080` | Web HTTP |
| `MCP_PORT` | `18787` | MCP HTTP |
| `MCP_AUTH_TOKEN` | `contract-test-token` | Require this bearer. A missing or wrong token is HTTP 401 with body `Unauthorized` |
| `MCP_PUBLIC_URL` | `http://127.0.0.1:$MCP_PORT` | When set, the 401 `WWW-Authenticate` header includes `resource_metadata` for this origin |
| `MCP_ALLOWED_HOSTS` | `127.0.0.1` | Extra `Host` values accepted besides loopback. Any other host is HTTP 403 body `Forbidden host` |
| `ADMIN_USERNAME` | `contract-admin` | Create the first admin when the database has none |
| `ADMIN_PASSWORD` | `contract-admin-pass` | Must satisfy the password policy (at least 10 characters, not a common password) |
| `ADMIN_DISPLAY_NAME` | `Contract Admin` | Optional display name for that admin |

Readiness, which is all the runner waits for:

- `GET http://127.0.0.1:$PORT/` returns any HTTP status (a connection error means not ready)
- `POST http://127.0.0.1:$MCP_PORT/mcp` with a JSON body and no `Authorization` returns 401

These are exported for the test process, not read by the server: `CONTRACT_WEB_URL`, `CONTRACT_MCP_URL`, `CONTRACT_MCP_TOKEN`, `CONTRACT_MCP_PUBLIC_URL`, `CONTRACT_DATA_DIR`, `CONTRACT_ADMIN_USER`, `CONTRACT_ADMIN_PASS`.

The runner unsets `ELEVENLABS_API_KEY`, `ELEVENLABS_KEY`, `OPENAI_API_KEY`, `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`, `GOOGLE_CLIENT_ID`, and `GOOGLE_CLIENT_SECRET`. Audio and image tools must not call out. Node's `server/start.js` still fills any unset key from a repo `.env`, so a checkout that has a `.env` is not a clean contract run.

If the command exits before those probes succeed, the runner prints `server exited before it was ready`, the command, and the server log, and exits 1. It does not wait out the ready timeout.

| Variable | Meaning |
| --- | --- |
| `CONTRACT_WEB_URL` | Web app base URL |
| `CONTRACT_MCP_URL` | MCP endpoint, including `/mcp` |
| `CONTRACT_MCP_TOKEN` | Static `MCP_AUTH_TOKEN` |
| `CONTRACT_MCP_PUBLIC_URL` | `MCP_PUBLIC_URL` the server was booted with. The 401 `resource_metadata` value is checked against it. |
| `CONTRACT_DATA_DIR` | Server data directory (read by later DB checks) |
| `CONTRACT_SERVER_CMD` | Shell command that starts the server. Unset means `node <repo>/server/start.js` |
| `CONTRACT_ADMIN_USER`, `CONTRACT_ADMIN_PASS` | Bootstrap admin. The runner's defaults satisfy the password policy. |
| `UPDATE_GOLDEN=1` | Rewrite goldens instead of comparing |

## Goldens

Compared files:

- `golden/mcp/initialize.json` — the `initialize` result
- `golden/mcp/tools-list.json` — canonical `tools/list`

`golden/mcp/tools-list.raw.json` is the server's SSE data line for `tools/list` (the JSON-RPC message, pretty-printed, server key order kept). The suite compares it. Volatile values are masked on the live payload and on the file, then both are encoded with sorted object keys. Key order in the file is not part of the contract. `$schema`, empty `properties`, and the rest of the wire message are. `tools-list.json` stays the portability golden: it is the `result` after schema canonicalization. Update mode rewrites the raw file from the live payload and keeps the server's key order.

Regenerate, then review the diff. Do not update a golden to hide an unintended server change.

```sh
contract/run.sh -update
# or
UPDATE_GOLDEN=1 contract/run.sh
```

`go test -update` also works when run from this directory against an already-booted server. `run.sh -update` is the path that reaches every package, because it sets `UPDATE_GOLDEN` instead of passing a flag packages do not all define.

Volatile values are masked before a golden is written: UUIDs, ISO timestamps, epoch fields (`exp`, `iat`, `updatedAt`, and the other names in `internal/normalize/mask.go`), OAuth tokens, and session cookie values. Cookie attributes stay. `Date` and `cf-*` headers are dropped. Descriptions, titles, names, enums, defaults, and input schemas are not masked. Numeric revision counters are left as-is until the domain-error goldens land; a bare integer is ambiguous.

## Schema canonicalizer

`internal/normalize.Canonical` is applied to every `inputSchema` on both sides:

- Drop `$schema`. Drop `additionalProperties: false`. The live Node list does not emit `additionalProperties`; the rule is here so a default `false` from another SDK does not look like a contract change.
- Drop `minimum` / `maximum` when they are the JS safe-integer bounds (±9007199254740991). Real bounds stay.
- Unwrap a single-member `anyOf` or `oneOf`.
- Collapse `anyOf: [{type: X}, {type: "null"}]` and `nullable: true` to `type: [X, "null"]` (type names sorted).
- Collapse an `anyOf` of `{const: v}` members to one `enum`. Enum value order is the order Node emitted. Type names on that enum are a sorted set.
- Sort an `anyOf` that is only type names.
- Treat `type: number` with `multipleOf: 1` as `integer`.
- Sort object keys and the `required` array. Drop an empty `properties` object and an empty `required` array, so they match a missing key.
- An untyped `{}` or a schema whose only payload is a description (Node's `z.any()`, for example `block`, `value`, `worksheet`) becomes `type: "any"`, description kept.

Enum value order is frozen on purpose. `["shared", "personal"]` and `["working", "published"]` are not alphabetical, and a reorder should fail the golden.

`execution: {"taskSupport": "forbidden"}` is kept. Node emits it on every tool. It is not in the canonicalizer's strip list.

## What this PR does not freeze

Two OAuth bugs are known and must not become the contract. `TestTokenChecksClientSecret` and `TestAuthCodeIsBoundToTheRegisteringClient` are skipped and marked expected-to-be-fixed. In-memory authorization codes and pending sign-ins are not part of that skip.

`GET /mcp` (the server holds an SSE stream open) and the over-1MiB body (the socket is destroyed) are not exercised here.
