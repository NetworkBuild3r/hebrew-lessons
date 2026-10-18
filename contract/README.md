# Contract suite

Black-box tests that freeze the Node server's HTTP and MCP behavior so a later Go port can be gated on the same goldens. The harness is a separate Go module. It uses the standard library only (`go test` and `net/http`) and does not import the MCP SDK or any server code.

PR1 covers the harness, transport and auth basics, `initialize`, and the canonical `tools/list` for all 42 tools. Later PRs add the tool calls, error matrix, OAuth, web routes, and the Go server.

## Run

From the repo root, with Node 22 and Go installed:

```sh
contract/run.sh
```

The runner pins `TZ=UTC`, creates a fresh `DATA_DIR`, starts `CONTRACT_SERVER_CMD` (default `node server/start.js`), waits until the web port answers and `POST /mcp` returns 401, runs `go test ./...`, and deletes the data directory. Web listens on `PORT` (default 18080) and MCP on `MCP_PORT` (default 18787).

`LESSONS_DIR` is left unset, so the server uses the repo `worksheets/` directory in place. Pointing it at `fixtures/lessons` would not stay trimmed: startup copies any missing seed lesson into `LESSONS_DIR`.

If `CONTRACT_WEB_URL`, `CONTRACT_MCP_URL`, `CONTRACT_MCP_TOKEN`, and `CONTRACT_MCP_PUBLIC_URL` are already set, the runner does not boot a server. It only runs the tests.

| Variable | Meaning |
| --- | --- |
| `CONTRACT_WEB_URL` | Web app base URL |
| `CONTRACT_MCP_URL` | MCP endpoint, including `/mcp` |
| `CONTRACT_MCP_TOKEN` | Static `MCP_AUTH_TOKEN` |
| `CONTRACT_MCP_PUBLIC_URL` | `MCP_PUBLIC_URL` the server was booted with. The 401 `resource_metadata` value is checked against it. |
| `CONTRACT_DATA_DIR` | Server data directory (read by later DB checks) |
| `CONTRACT_SERVER_CMD` | Command that starts the server |
| `CONTRACT_ADMIN_USER`, `CONTRACT_ADMIN_PASS` | Bootstrap admin. The runner's defaults satisfy the password policy. |
| `UPDATE_GOLDEN=1` | Rewrite goldens instead of comparing |

Provider keys are unset. Audio and image tools must not call out during a contract run.

## Goldens

Compared files:

- `golden/mcp/initialize.json` — the `initialize` result
- `golden/mcp/tools-list.json` — canonical `tools/list`

`golden/mcp/tools-list.raw.json` is the server's SSE payload, pretty-printed with key order preserved. It is rewritten in update mode and is not compared. Schema canonicalization is what the suite asserts.

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
