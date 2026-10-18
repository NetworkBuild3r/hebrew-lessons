# Contract suite

Black-box tests that freeze the Node server's HTTP and MCP behavior so a later Go port can be gated on the same goldens. The harness is a separate Go module. It uses `go test` and `net/http` and does not import the MCP SDK or any server code. The schema dump uses pure-Go `modernc.org/sqlite` v1.60.1 (no cgo), the same version as `go/go.mod`.

PR1 covers the harness, transport and auth basics, `initialize`, and the canonical `tools/list` for all 42 tools. Later PRs add the tool calls, error matrix, OAuth, web routes, and the Go server.

## Run

From the repo root, with Node 22 and Go installed:

```sh
contract/run.sh
```

The runner pins `TZ=UTC`, creates a fresh `DATA_DIR`, and starts `CONTRACT_SERVER_CMD` (default `node server/start.js`). A normal run copies the seeded database into that directory first and skips the HTTP seed. `-update` leaves the directory empty, seeds through HTTP and MCP, and rewrites the fixture. The runner waits until the web port answers and `POST /mcp` returns 401, runs `go test ./...`, and deletes the data directory. Web listens on `PORT` (default 18080) and MCP on `MCP_PORT` (default 18787).

`CONTRACT_SERVER_CMD` is any shell command: a binary (`./bin/hebrew-server`), a binary with arguments, or a wrapper script. The runner executes it as `bash -c "$CONTRACT_SERVER_CMD"` with the working directory set to the repo root, and with the environment below. The process has to stay in the foreground in the process group the runner created. A server that daemonizes is not stopped on teardown, and the ports stay held.

On Linux the runner uses `setsid` when it is on `PATH`, so the command is its own session and process group. macOS has no `setsid`. There the runner turns on bash monitor mode for the launch only, which gives the command its own process group, then turns monitor mode off again. Teardown is the same on both: signal that group, signal any descendant that left the group, then `SIGKILL` if it is still alive. No extra packages are required.

Lessons come from `fixtures/lessons` (only `my-morning`, `shai-date-indirect-speech`, `_template`, and a catalog that lists those two). The runner copies that tree and sets both `LESSONS_DIR` and `HEBREW_ROOT/worksheets` to the copy. Startup's copy-missing step is then a no-op, so lessons that live only in the repo `worksheets/` directory are not filled back in. `shared/` is symlinked from the repo so the web app still finds its files. `HEBREW_ROOT` has no `.env`, so a developer `.env` in the repo is not loaded. The adult worksheet slug and audio blobs are not in that tree.

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

The runner unsets `ELEVENLABS_API_KEY`, `ELEVENLABS_KEY`, `OPENAI_API_KEY`, `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`, `GOOGLE_CLIENT_ID`, and `GOOGLE_CLIENT_SECRET`. Audio and image tools must not call out. `HEBREW_ROOT` is the temp lesson tree, so `server/start.js` does not load a developer `.env` from the repo.

If the command exits before those probes succeed, the runner prints `server exited before it was ready`, the command, and the server log, and exits 1. It does not wait out the ready timeout.

| Variable | Meaning |
| --- | --- |
| `CONTRACT_WEB_URL` | Web app base URL |
| `CONTRACT_MCP_URL` | MCP endpoint, including `/mcp` |
| `CONTRACT_MCP_TOKEN` | Static `MCP_AUTH_TOKEN` |
| `CONTRACT_MCP_PUBLIC_URL` | `MCP_PUBLIC_URL` the server was booted with. The 401 `resource_metadata` value is checked against it. |
| `CONTRACT_DATA_DIR` | Server data directory (read by later DB checks) |
| `CONTRACT_SERVER_CMD` | Shell command that starts the server. Unset means `node <repo>/server/start.js` |
| `CONTRACT_ADMIN_USER`, `CONTRACT_ADMIN_PASS` | Bootstrap admin on an empty database (`-update`). Defaults are the documented fixture passwords. |
| `CONTRACT_SEED_DB` | Fixture file copied into `DATA_DIR` on a normal run. Default `fixtures/db/seeded.sqlite`. |
| `UPDATE_GOLDEN=1` | Seed from scratch and rewrite goldens and the fixture, instead of booting from the fixture. |

## Goldens

Compared files:

- `golden/mcp/initialize.json` — the `initialize` result
- `golden/mcp/tools-list.json` — canonical `tools/list`
- `golden/db/schema.sql` — `sqlite_master`, `schema_migrations` (applied_at masked), `user_version`, `journal_mode`
- `golden/mcp/calls/<tool>/<case>.json` — one read-only tool call. Success and domain errors are `{isError, content}` with `content` parsed when the tool text is JSON. Validation cases are `{category, field, message}` with message `<validation>`. `transportForm` is stored and not compared.

`golden/mcp/tools-list.raw.json` is the server's SSE data line for `tools/list` (the JSON-RPC message, pretty-printed, server key order kept). The suite compares it. Volatile values are masked on the live payload and on the file, then both are encoded with sorted object keys. Key order in the file is not part of the contract. `$schema`, empty `properties`, and the rest of the wire message are. `tools-list.json` stays the portability golden: it is the `result` after schema canonicalization. Update mode rewrites the raw file from the live payload and keeps the server's key order.

`fixtures/db/seeded.sqlite` is the database after the seed, written in update mode. A normal run copies it and does not byte-compare it (password salts change when it is regenerated). It checks that the file's schema matches `golden/db/schema.sql`. See [Seeded database](#seeded-database).

Regenerate, then review the diff. Do not update a golden to hide an unintended server change.

```sh
contract/run.sh -update
# or
UPDATE_GOLDEN=1 contract/run.sh
```

`go test -update` also works when run from this directory against an already-booted server. `run.sh -update` is the path that reaches every package, because it sets `UPDATE_GOLDEN` instead of passing a flag packages do not all define.

Volatile values are masked before a golden is written: UUIDs, ISO timestamps, epoch fields (`exp`, `iat`, `updatedAt`, and the other names in `internal/normalize/mask.go`), OAuth tokens, and session cookie values. Cookie attributes stay. `Date` and `cf-*` headers are dropped. Numeric revision counters are left as-is when the seed order makes them deterministic. A bare integer is not masked. Strings that contain an id or a timestamp are masked even when the id is only part of the sentence. Descriptions, titles, names, enums, defaults, and input schemas are not masked.

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

## Seeded database

Path: `fixtures/db/seeded.sqlite`. `CONTRACT_SEED_DB` overrides it.

A normal `run.sh` (Node or Go) copies that file to `$DATA_DIR/hebrew.sqlite` before `CONTRACT_SERVER_CMD` starts, then logs `contract: using seeded fixture <path>`. The copy does not include `-wal` or `-shm`. Both legs get that temp copy; the Go leg needs no other database setup. The HTTP/MCP seed is skipped. `db_schema_test.go` still dumps the live `hebrew.sqlite` after boot and compares it to `golden/db/schema.sql` and to the fixture file.

The file holds migrations 001–011 and the seed:

- Users `contract-admin` (admin, display name `Contract Admin`), `learner` (AI planning on), and `learner-off` (AI planning off).
- Lesson visibility: `my-morning` is `everyone`; `shai-date-indirect-speech` is limited to `learner`.
- Lexicon rows from the two fixture lessons plus `fixtures/plans/lexicon-ledaber.json`, and library words `לדבר` and `מדבר` for `learner`.
- Published plan `five-lesson-types-sample` (revision 17) and unpublished `draft-plan`.
- No dictionary rows. The snapshot deletes `sessions` before it writes the file, so a boot does not carry the seed process's session.

Contract-only passwords (not production credentials):

| User | Password |
| --- | --- |
| `contract-admin` | `contract-admin-pass` |
| `learner` | `learner-pass-1` |
| `learner-off` | `learner-off-pass-1` |

Tests that need a login call `POST /api/session` with a new cookie jar and then `GET /api/me`. They do not read `sessions`. The MCP token still comes from `MCP_AUTH_TOKEN` / `CONTRACT_MCP_TOKEN` and is not stored in the fixture. `internal/fixturecheck` opens the committed file read-only and checks the session count, the three usernames, empty email columns, and that no table which holds OAuth clients, codes, or tokens has rows.

Regenerate the fixture and the goldens together, from an empty data directory, using these passwords:

```sh
contract/run.sh -update
```

That re-seeds through HTTP and MCP and rewrites `fixtures/db/seeded.sqlite`. Review the diff. The adult worksheet slug must not appear under `contract/`; `internal/fixturecheck` fails the run if it does.

## Seed

`internal/seed` runs after boot only when `UPDATE_GOLDEN=1`, through the admin HTTP API and MCP, before the read-only cases. A normal run calls `UseFixture` instead and does not create accounts:

1. Sign in as the bootstrap admin (`POST /api/session`).
2. Create `learner` (`POST /api/admin/users`) and turn AI planning on (`PATCH /api/admin/users/:id/ai-planning`).
3. Create `learner-off` and leave AI planning off, so `planning_off` has an account.
4. `set_lesson_visibility`: `my-morning` is `everyone`; `shai-date-indirect-speech` is `{users: ["learner"]}`.
5. `upsert_lexicon_entries` from `fixtures/plans/lexicon-ledaber.json` (`expectedRev` 0), then `add_library_words` for `learner` (`לדבר`, `מדבר`). Lexicon is written before publish because the verb-workshop lesson will not publish without those keys.
6. `create_plan` id `five-lesson-types-sample` from `fixtures/plans/five-lesson-types.plan.json` (copied from `test/fixtures/lesson-types/five-lesson-types.plan.json`). The first lesson's blocks are inserted with `insert_block` (index 0, reverse order). The placeholder block is removed with `remove_block`. Each lesson's visibility is set to `everyone` with `set_field`. The other four lessons are inserted with `insert_lesson`. `publish_plan` then runs.
7. `create_plan` id `draft-plan` is left unpublished so `diff_plan` can return `no_snapshot`.

`upsert_lesson` is not called. The worksheet files are the fixture, and a call would generate audio unless `audio` is false. No ElevenLabs, OpenAI, or Google key is set.

## DB schema

`db_schema_test.go` opens `hebrew.sqlite` read-only with `modernc.org/sqlite` v1.60.1 (no cgo) after boot and again after the seed step. On a normal run the seed step is the fixture load, so the second dump is the same live file. Both dumps must match `golden/db/schema.sql`, and the committed fixture must match that same text. The harness does not migrate. `internal/fixturecheck` fails if this module's `modernc.org/sqlite` version differs from `go/go.mod`. The module's `go` directive is 1.26.0 because v1.60.1 requires it.

## Read-only tool cases

| Tool | Cases |
| --- | --- |
| `list_lessons` | happy |
| `get_lesson` | happy, bad_slug, missing, missing_slug (validation) |
| `list_words` | happy, one_lesson, unknown_slug |
| `list_activity_types` | happy |
| `list_users` | happy |
| `read_part` | plan, block, unknown_id, missing_path |
| `suggest_glosses` | story, not_a_story, unknown_id |
| `plan_image` | missing_key, missing_prompt (validation) |
| `list_plans` | happy, limit_zero (validation) |
| `list_plan_revisions` | happy, unknown_id |
| `get_plan_revision` | happy, missing_revision, unknown_id |
| `diff_plan` | published_to_working, unknown_id, no_snapshot, missing_from (validation) |
| `list_plan_issues` | happy, unknown_id |
| `get_learner_profile` | happy, planning_off, unknown_user, missing_username (validation) |
| `check_text_coverage` | happy, planning_off, unknown_user, invalid_text |
| `list_grammar_concepts` | happy |
| `list_lexicon` | search, invalid_pos, cursor |
| `get_lexicon_entry` | happy, missing |

`get_learner_profile` and `check_text_coverage` write an audit row and share a 60-read hourly cap. The seed does not call them. The cases above are the whole count for one run.

Goldens for `list_users` assume the runner defaults: admin `contract-admin`, display name `Contract Admin`.

## What this PR does not freeze

Two OAuth bugs are known and must not become the contract. `TestTokenChecksClientSecret` and `TestAuthCodeIsBoundToTheRegisteringClient` are skipped and marked expected-to-be-fixed. In-memory authorization codes and pending sign-ins are not part of that skip.

`GET /mcp` (the server holds an SSE stream open) and the over-1MiB body (the socket is destroyed) are not exercised here.
