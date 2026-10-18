# ADR-0016: Go server port

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-18 |
| **Authored by** | `INIT-014/SPEC-001` |
| **Initiative** | INIT-014 · go-server-port |
| **Research grounding** | None. Engineering record. Facts read in code at tree `86e8d50c`, 2026-10-18. |
| **Supersedes** | [ADR-0005](ADR-0005-stack-and-architecture.md) (Node runtime, `node:sqlite`, in-process TypeScript MCP, no build step, no second runtime). [ADR-0007](ADR-0007-single-page-app-structure.md) (single-shell SPA, as screens move). |
| **Superseded by** | — |

Brian Nelson approved moving off Node to Go on 2026-10-18, relayed by Hebrew Dev. He does not approve pull requests himself: Hebrew PR Reviewer approves and merges. This record is Accepted. ADR-0005 and ADR-0007 name it on their **Superseded by** lines (same point those lines changed for ADR-0007, 2026-10-09).

## Summary

Move the server from Node 22 to Go one route group at a time, behind a Go front on the same ports. Go uses stdlib `net/http`, `modernc.org/sqlite` on the same `DATA_DIR/hebrew.sqlite` and schema, templ + HTMX for screens as they move, a small TypeScript lesson player, and the official MCP Go SDK on port 8787. No data moves. Sessions, MCP OAuth tokens, audio cache keys, and recording paths stay byte-compatible. A group moves only when Hebrew QA's suite in `contract/` passes against both Node and Go. Node stays the rollback image until the last phase.

---

## Context

### What ADR-0005 decided

ADR-0005 (Accepted 2026-10-08) chose Option A: Node 22, `node:sqlite`, in-process MCP on `@modelcontextprotocol/sdk` 1.32.1, no framework, no build step, no second runtime. Option B led by two points (459 to 457); A won on migration cost. Its carry plan (§3) already required Deployment `hebrew`, `replicas: 1`, `Recreate`, the same PVCs and Secret, ports 8080 and 8787 on one pod, a read-only check before any write, and no recording deleted or overwritten without an archive. It lists Go on the official MCP SDK list.

### What ADR-0007 decided, and what replaces it

ADR-0007 (Accepted 2026-10-09) put signed-in screens in one shell: a hand-built History API router, header painted once, login on its own page, no new dependency and no build step. templ + HTMX replace that shell as each screen moves. A small TypeScript player, compiled to static JS under the existing `/shared/` gate, plays the lesson. Screens not yet moved stay on today's shell behind the proxy. Login stays separate. ADR-0006 `data-*` hooks stay so the Chrome scripts still run.

### Why ADR-0005 no longer holds

The operator chose Go. The old cost was carrying the MCP SDK, the database, the tests, and the image. Go is an official SDK, and a pure-Go driver can open the same file. `node:sqlite` is still experimental: `server/db/index.js` swallows the Node 22 warning and re-emits it. Routing, the literal parser, the schema validator, and security headers are hand-written, on two direct dependencies (`@modelcontextprotocol/sdk` `^1.32.1`, `sharp` `0.35.5`). [§9](#9-adr-0005-criteria-with-go) re-scores ADR-0005's own criteria. A and B stay 457 and 459. Go scores 383. The table does not pick Go. The operator does.

### Current state (`86e8d50c`)

One process, ports 8080 and 8787. No Go code. The only workflow builds and pushes the image and does not run tests. SQLite is WAL, `foreign_keys=ON`, `busy_timeout=5000`: 20 tables, migrations `001`–`011` recorded by filename, `grammar_concepts` seeded by `005`. Lesson files are non-executed `window.NAME = <literal>;` assignments (`server/lesson-data.js`). The server requires `shared/player/fsrs.js` and `review.js`; `utils.js` runs in `vm` from `server/audio.js` and is required from `content-filter.js` and `learner/coverage.js`. Tests: 155 `test/*.test.js` files, 24 `test/chrome-*.js` scripts.

---

## Decision

### 1. Target stack

| Concern | Choice |
|---|---|
| HTTP | Stdlib `net/http` (1.22 patterns). Exact path before `:param`. JSON 404 under `/api/`. SDK release on 2026-10-18 declares Go 1.25; do not pin lower. |
| Database | `modernc.org/sqlite`, `CGO_ENABLED=0`, same file, WAL, `foreign_keys=ON`, `busy_timeout=5000`. One writer plus read-only handles. `BEGIN IMMEDIATE`. Timestamps as JS `toISOString()` (`YYYY-MM-DDTHH:MM:SS.sssZ`). |
| UI | templ + HTMX per screen. `data-*` hooks stay. Write path under Web and UI. |
| Player | Small TypeScript player. Sources in `web/player/`, built to `shared/dist/player.js`, existing `/shared/` gate (fonts and brand stay public). |
| MCP | `github.com/modelcontextprotocol/go-sdk/mcp` on `:8787` `/mcp`. Same tool names, schemas, OAuth routes. |
| Crypto, text, images | `golang.org/x/crypto/scrypt`, `golang.org/x/text/unicode/norm`, `image/png`, `image/jpeg`, `golang.org/x/image/{webp,draw}`. Keep `sniff.js` animation refusals (PNG `acTL`/`fcTL`/`fdAT`, WebP flag `0x02`). |

### Web and UI

Today a mutating `/api/` call must be JSON, or multipart for `POST /api/media`, and must pass `originAllowed` (`server/web.js` `handleApi`). `requestOrigin` (`server/http/security.js`) reads `Origin`, then `Referer`. An HTMX form post is `application/x-www-form-urlencoded`, so that JSON rule would reject it with 403.

Reads stay on the page URL. When `HX-Request` is set, that URL returns the fragment. There is no new read route.

Writes from HTMX go to `/_hx/`. Those handlers call the same Go services as `/api/`. `/api/` JSON is unchanged.

`/_hx/` accepts `application/x-www-form-urlencoded`. A write needs both of these. If either check fails, the response is 403:

- `Origin`, or `Referer` when `Origin` is absent, matches the site origin.
- A per-session CSRF token. It is a synchronizer token stored with the session, or it is HMAC-derived from the session token. The page sends it as `X-CSRF-Token` through `hx-headers` on the body, or as a hidden field when the post has no JS. The server compares it in constant time.

`GET` under `/_hx/` never mutates. The session cookie stays `SameSite=Lax`.

templ files live in `go/internal/web/views/`: `layout.templ`, `components/`, `pages/`, `blocks/`, and `partials/`. Page handlers live in `go/internal/web/pages/`. Generated `*_templ.go` files are committed. CI runs `templ generate` and fails on a diff. TypeScript player sources live in `web/player/` and build to `shared/dist/player.js`, served from disk behind the existing `/shared/` gate. `go:embed` cannot reach outside the `go/` module, so that built file is not embedded.

### 2. Migrations

Versions 1–11 are frozen. If the ledger already has them, Go runs nothing. On an empty database, Go applies `baseline.sql` and inserts rows 1–11 with the original names (`001-initial.js`, …) so Node can open the file. `baseline.sql` is `sqlite_master` plus the seeded `grammar_concepts` rows after the Node migrations run on empty. A golden test compares the two schemas and seed rows.

012+ are SQL files embedded in the Go binary, same ledger. While Node is the rollback, 012+ migrations stay additive. Whether Node refuses an unknown version is [open question 2](#open-questions).

### 3. Data stays in place

No export, import, dump, or restore. Same `hebrew.sqlite` on PVC `hebrew-data` (`DATA_DIR=/data/app`), same lesson files on PVC `hebrew-lessons` (`LESSONS_DIR=/data/worksheets`). Media stays in `DATA_DIR/media` and `media/_archive`.

Writers Go must match byte for byte:

- `mcp/store.js` `writeJs` (`WORKSHEET`, `HEBREW_CATALOG`) and worksheet saves in `server/authoring.js`: `'window.' + name + ' = ' + JSON.stringify(value, null, 4) + ';\n'`.
- Word-audio catalog (`LESSONS_DIR/_audio/words/catalog.js` and seed mirror `shared/word-audio-catalog.js`): `'window.HEBREW_WORD_AUDIO = Object.assign({}, window.HEBREW_WORD_AUDIO || {}, ' + JSON.stringify(map, null, 4) + ');\n'`.
- `content-filter.js` `emitScript`: `'window.' + name + ' = ' + JSON.stringify(value).replace(/</g, '\\u003c') + ';\n'` (no indent).

### 4. Sessions and OAuth tokens carry over

Cookie names stay `hebrew_session`, or `__Host-hebrew_session` when `COOKIE_SECURE=1` (HttpOnly, SameSite=Lax, Path=/, Max-Age 30 days, Secure with that flag). Reads accept either name. Lookup is SHA-256 hex of the raw token in `sessions`. Lifetime is 30 days from sign-in, not idle. Google and password sessions stay valid.

Password hashes stay `scrypt$N=<n>$r=<r>$p=<p>$<salt_b64url>$<key_b64url>` and verify with the stored `N`, `r`, and `p`.

MCP OAuth (`mcp/oauth.js`):

- Key: `HMAC-SHA256(MCP_AUTH_TOKEN, "hebrew-mcp-oauth-v1")`.
- Opaque client, access, and refresh tokens are looked up by HMAC-SHA256 hex in `DATA_DIR/mcp-oauth-state.json` (`clients`, `access`, `refresh`). Writes stay `<file>.tmp-<pid>`, mode `0600`, then rename.
- HMAC tokens `base64url(JSON) + '.' + base64url(HMAC)` (`sign` / `verify`, types `client`, `access`, `refresh`) stay accepted when the opaque row is missing.
- TTLs: access 3600 s, refresh 30 d, code 600 s, client 400 d. Codes and pending requests stay in memory.
- Tokens issued before 8787 moves stay valid.

### 5. Audio compatibility (hard requirement)

Keys and paths match Node and are golden-tested.

- Cache key: `sha256(voiceId + "\0" + model + "\0" + trim(text)).hex[0:24]`, with JS `String.prototype.trim` (`server/audio.js` `cacheKey`). Go `strings.TrimSpace` differs: JS trim removes U+FEFF and keeps U+0085; `TrimSpace` does the opposite. Go-1's goldens include at least those two.
- Text hash is the full hex of that same input.
- Word files are `<key>.mp3` under `LESSONS_DIR/_audio/words/`. Site paths are `../_audio/words/<file>`.
- Keep the `<key>-*.mp3` lookup, the lesson archive `<lesson>/audio/_archive/<stem>.<first-12-hex-of-file-sha256>.mp3` (never replace an archive that exists), `_audio/words/manifest.json` (`version`, `voiceId`, `model`, `entries`), and `<lesson>/audio/manifest.json` (`version`, `clips`).

Go never overwrites, deletes, or regenerates an existing recording. An existing file is a hit: no ElevenLabs call. A change that would move any existing key or path fails CI.

### 6. Golden-file tests

A Node script at the same commit writes `testdata/golden/**` (input, options, exact output) plus the Node version and SHA. Go compares canonical JSON, order-sensitive arrays, and exact messages. CI regenerates from Node and fails on `git diff`.

Go-1's audio-key goldens include the JS `trim` vs `strings.TrimSpace` cases in §5 (at least U+FEFF and U+0085).

Also covered: `validatePlan` and `checkHebrew` (ADR-0003 samples, `test/fixtures/plans-1.1`, `test/fixtures/lesson-types`, one mutation corpus per rule); FSRS and ladder via `test/fixtures/fsrs/fsrs6-golden.json` (shared with the TypeScript player); `learner/model.js` and `mix.js` at a fixed clock, seed, and DB fixture; audio keys, hashes, and paths; the lesson-data parser on `worksheets/**/*.js` and the §3 writers; the migration schema and `grammar_concepts` seed.

`test/helpers/signed-in-server.js` cannot drive an external binary. `start` writes a temp `DATA_DIR` and fixture `LESSONS_DIR`, then `require`s `server/db`, `server/accounts`, and `server/web` in process. `server/web.js` `listen` returns a Node `http.Server`. The helper sets `PORT=0`, waits for `listening`, and reads `server.address().port`. `stop` calls `server.close()`. There is no `spawn` and no command path. The smallest change that keeps `start`, `stop`, `cookieFor`, and `baseUrl` is a branch inside `start` and `stop`: keep the fixture, migrate, `createUser`, and `signIn` so the session rows are in the temp `hebrew.sqlite`; close that Node connection; spawn the Go binary with `DATA_DIR`, `LESSONS_DIR`, `HEBREW_ROOT`, and `HOST=127.0.0.1`. The binary must print the bound port, or the helper must pass a port it already released. `PORT=0` is not enough, because a child does not expose `server.address()`. `stop` signals the child, then deletes the temps. `cookieFor` already returns `hebrew_session=<token>` from the in-process sign-in. That change does not make every current caller a black box. After `start`, `test/chrome-progress-smoke.js` calls `accounts.createUser` and `signIn`, `test/chrome-plan-editor-smoke.js` requires `server/plans/store`, and `test/chrome-verb-workshop-smoke.js` calls `publishPlan`, all against the same file the server has open. Those stay Node writers beside the process under test, or they move to HTTP.

### 7. Strangler cutover

Go listens on `:8080` and proxies every route it has not taken to Node on a loopback port in the same pod. A phase moves a whole route group.

Hebrew QA owns the contract suite. It lives in this repo at `contract/`, with its own `go.mod`. A group moves only when that suite passes against both Node and Go. A failed group returns to the Node proxy by config, with no new image. What the suite asserts per group is [open question 3](#open-questions).

From ADR-0005 §3 and `deploy/k8s-database.md`: `replicas: 1`, `strategy: Recreate`, PVCs `hebrew-data` and `hebrew-lessons` at the same paths, Secret `hebrew-secrets` and the same keys, `runAsUser` and `fsGroup` 1000, ports 8080 and 8787 on that pod (`Dockerfile` `EXPOSE`; the Deployment snippet has no `ports` list). No second replica.

Move these whole. Their state is in the process: auth (in-memory throttle in `server/auth/throttle.js`: 5 username failures, 30 s backoff doubling to 15 min, 20 failures per IP per 15 min); MCP on 8787, including in-memory codes and pending requests; every writer of one lesson file or of `mcp-oauth-state.json`.

The throttle module takes an `ip` string and reads no header. Node's `server/api/auth-routes.js` `clientIp` passes the first comma-separated field of `X-Forwarded-For`, trimmed, when that header is a non-empty string; otherwise `req.socket.remoteAddress`, or `''` if there is no socket. That leftmost value is client-controlled: Cloudflare appends to whatever the client already sent, so the login throttle can be bypassed. The throttle key is not an external contract, so changing it is in scope and is not a compatibility break. Go does not keep that rule.

Go trusts `CF-Connecting-IP` only when the immediate peer, the socket remote address, is in `TRUSTED_PROXIES`, a CIDR list from env or config (Traefik). Otherwise it uses the socket address. It never uses the leftmost `X-Forwarded-For` value. Empty `TRUSTED_PROXIES` is the default: trust nobody, use the socket address. Whether Traefik passes `CF-Connecting-IP` through untouched, and Traefik's peer address or CIDR, is [open question 11](#open-questions).

Edge. That review found both hostnames on the proxied wildcard, with no tunnel, no Access, no Workers, and no Cloudflare header or path rules. The origin is k3s Traefik in front of the Service. Cutover in front of the pod is a Traefik routing change only. Cloudflare is not changed.

Cloudflare's Free plan, per that review, has a proxy read timeout of 125 s (a response that sends no bytes for longer than that is dropped; the plan cannot raise it) and a 100 MB request-body cap. Go sets its own lower limits: an inbound server timeout, and `MaxBytesReader` per route, including media upload. Node's HTTP servers (`server/web.js`, `mcp/http.js`) set no `timeout`, `requestTimeout`, `headersTimeout`, or `keepAliveTimeout`, so the Go timeout numbers are [open question 9](#open-questions). Body caps that do exist, which Go stays at or under: `readJson` default `64 * 1024` (`server/http/body.js`); plans and lesson saves `1024 * 1024`; lexicon upsert `256 * 1024`; library writes `16 * 1024`; word-audio POST, lexicon-verified PATCH, and ai-planning PATCH `1024`; coverage `8192`; multipart `POST /api/media` `5242880` (`server/media/ingest.js` `MAX_INPUT_BYTES`); MCP POST `1024 * 1024` (`mcp/http.js`); OAuth forms `64 * 1024` (`mcp/oauth.js`). Sharp's 20 s decode timeout and the outbound image call's 60 s timeout are not inbound server timeouts.

Rollback is the Node-only image. Nothing was converted, so Node runs on the files, subject to additive migrations.

WAL, two processes. Node opens `DATA_DIR/hebrew.sqlite` with `PRAGMA journal_mode = WAL`, `busy_timeout = 5000`, and `BEGIN IMMEDIATE` (`server/db/index.js`). WAL adds `hebrew.sqlite-wal` and `hebrew.sqlite-shm` in that same directory. SQLite's WAL page (sqlite.org/wal.html, read 2026-10-18) says every process that uses the database must be on the same host, because the wal-index is that `-shm` file mapped into shared memory, and WAL does not work over a network filesystem. Its locking page (sqlite.org/lockingv3.html, same day) says Unix builds use POSIX advisory locks, and those locks are buggy or missing on many NFS implementations; if the locks do not work, the file can corrupt. The concurrency WAL allows is many readers and one writer at a time. Readers do not block the writer, and the writer does not block readers. A second writer waits out `busy_timeout`.

One pod is one host when both containers mount the same volume. `deploy/k8s-database.md` sets `hebrew-data` to `accessModes: ["ReadWriteOnce"]` and does not set `storageClassName`. RWO with `replicas: 1` keeps the mount on one node. It does not say the class is a local filesystem. A read-only open is not a silent fallback on a directory that cannot create files: since SQLite 3.22.0 (2018-01-22) a read-only WAL database opens when `-wal` and `-shm` already exist or can be created. Node's `openReadOnly` passes `readOnly: true` and does not migrate or checkpoint.

Go never shares the writer. Two safe shapes: Go owns writes for a table group and Node uses `openReadOnly` on that file, or a proxy-only cutover where Go does not open `hebrew.sqlite` and Node is the only process that does. Go-4 is proxy-only until the storage class is confirmed. `busy_timeout` stays 5000 on any connection that does open the file. This tree has no Go module, so Go-3 is the check that `modernc.org/sqlite` and `node:sqlite` lock one local temp file the same way. The storage-class check is with CI/CD ([open question 4](#open-questions)).

### 8. Phase sequence

Labels are Go-0 through Go-17 so they do not collide with GitHub pull-request numbers.

| Phase | What lands |
|---|---|
| Go-0 | This ADR. |
| Go-1 | Go module, validators, Node golden generator, Go CI. Nothing served. Audio-key goldens include JS `trim` vs `strings.TrimSpace` (U+FEFF, U+0085). |
| Go-2 | Lesson-data parser and the §3 writers. |
| Go-3 | Config, paths, DB open, ledger-compatible migrations, read-only on a DB copy. Baseline golden. The local-file lock check in §7. |
| Go-4 | Go on `:8080` proxying all routes to Node, plus `GET /api/me`. Proxy-only: Go does not open `hebrew.sqlite` until the storage class is confirmed ([open question 4](#open-questions)). The check is with CI/CD. |
| Go-5–Go-11 | Auth and sessions; static gate and filtered scripts; read-only content APIs; FSRS, progress, review; mix, library, cards; plans; admin users and lessons. |
| Go-12–Go-14 | Google sign-in; media (pure Go); audio and word-audio. |
| Go-15 | MCP and OAuth. 8787 moves whole. |
| Go-16 | templ + HTMX and the TypeScript player, screen by screen. |
| Go-17 | Remove the Node proxy and image. Port or retire `scripts/`. |

### 9. ADR-0005 criteria, with Go

Same criteria and weights as ADR-0005 Decision 1. A score is an integer from 1 to 5. The weighted total is score times weight. The maximum is 500. A 5 means the criterion is met with the system that already exists, or with a small addition. A 1 means it is met only by replacing the thing it is about. C13 is special: 5 means the smallest migration cost. A–E are copied from ADR-0005 Decision 3 (2026-10-08) and are not re-judged. Go is new. "Judgment" means a reviewer can move that Go cell by one point.

| Id | Weight | A | B | C | D | E | Go |
|----|--------|---|---|---|---|---|-----|
| C1 Blocks | 8 | 4 | 5 | 5 | 4 | 5 | 4 |
| C2 Preview | 7 | 4 | 5 | 5 | 5 | 5 | 4 |
| C3 One ops module | 12 | 5 | 5 | 5 | 3 | 4 | 5 |
| C4 RTL / bidi | 6 | 4 | 4 | 4 | 4 | 4 | 4 |
| C5 Accessibility | 6 | 4 | 4 | 4 | 4 | 4 | 4 |
| C6 Media | 7 | 5 | 5 | 5 | 4 | 4 | 4 |
| C7 SQLite and data | 9 | 5 | 5 | 5 | 3 | 4 | 4 |
| C8 MCP | 9 | 5 | 5 | 5 | 2 | 3 | 4 |
| C9 Tests | 5 | 5 | 4 | 3 | 2 | 3 | 3 |
| C10 Solo maintainer | 8 | 5 | 4 | 3 | 2 | 3 | 3 |
| C11 Agents | 8 | 3 | 4 | 5 | 2 | 5 | 4 |
| C12 Docker, one replica | 7 | 5 | 5 | 4 | 3 | 3 | 4 |
| C13 Migration cost | 8 | 5 | 4 | 3 | 1 | 2 | 2 |
| **Weighted total** | **100** | **457** | **459** | **439** | **295** | **378** | **383** |
| Lead over A | | — | +2 | −18 | −162 | −79 | −74 |

**C1 Blocks (8).** A 4, copied: nested string HTML can render the six blocks and is clumsier than a component. B 5, copied: a Preact function per block is the render. C 5, copied: React can render the blocks. D 4, copied: HEEx can render them, and today's client audio and reveal become events or hooks. E 5, copied: the same React reason as C. Go 4, judgment: templ can emit the blocks, and the TypeScript player keeps the client behavior; this tree has no templ block renderer, so this is not a 5.

**C2 Preview (7).** A 4, copied: a preview is another call with the working copy, and the hand-built editor is already large. B 5, copied: the Preact function is the preview. C 5, copied: React can preview. D 5, copied: a server render that updates after an event is a live preview. E 5, copied: the same React reason as C. Go 4, judgment: an HTMX partial re-renders one part on a request, which is A's shape, and ADR-0005 did not treat a persistent diff as a requirement.

**C3 One ops module (12).** A 5, copied: one module serves both callers in this process. B 5, copied: the server does not change. C 5, copied: the Node server still owns the operations. D 3, copied: one Elixir module can implement them, and the TypeScript SDK cannot call it in-process. E 4, copied: one Node module serves both only if MCP shares the process, which is the eject. Go 5, judgment: the official Go SDK and the builder share one process at the end, which is the property D lacks. Two copies during the strangler are C13.

**C4 RTL / bidi (6).** A through E are 4, copied: `dir` and logical properties are authoring, and no library does bidi. Go 4, same reason: templ is HTML, and `dir` stays the author's job.

**C5 Accessibility (6).** A through E are 4, copied: alt, labels, and focus can be required arguments and still depend on tests. Go 4, same reason.

**C6 Media (7).** A, B, and C are 5, copied: ADR-0004's pipeline stays in this process. D 4, copied: Plug can sniff, re-encode, and gate, as new code. E 4, copied: the pipeline can move into route handlers, and a public cache of a lesson would break the gate. Go 4, judgment: approval, `DATA_DIR/media`, and the archive stay, and the decoder is new Go code, as D's Plug was.

**C7 SQLite and data (9).** A, B, and C are 5, copied: `hebrew.sqlite` and the clip files stay. D 3, copied: the file can remain, and every query and migration is re-proven on another driver. E 4, copied: `node:sqlite` can stay if the edge runtime is forbidden, after the access layer is re-wired. Go 4, judgment: the file, the schema, and the clips stay with no dump, and every query is re-proven on `modernc.org/sqlite`. A 5 would ignore the driver change. A 3 would ignore the frozen baseline and the goldens.

**C8 MCP (9).** A, B, and C are 5, copied: the MCP listener stays. D 2, copied: the official SDK list has no Elixir. E 3, copied: the SDK runs on Node, and co-hosting it is the eject or a second process. Go 4, judgment: Go is on that list and listens on 8787 in-process, and it replaces the TypeScript SDK already integrated, so it is not a 5.

**C9 Tests (5).** A 5, copied: the files and the Chrome scripts keep their runners if `data-*` hooks stay. B 4, copied: the server tests stay, and a new render tree has to keep the hooks. C 3, copied: utility class names break the Chrome selectors even when the hooks stay. D 2, copied: the Node files must be ported or dropped, and the Chrome scripts can stay as a black box. E 3, copied: imports of `server/web.js` are rewritten, and the Chrome scripts are retargeted. Go 3, judgment: the direct Node tests (155 files now, 44 when ADR-0005 was scored) are ported or re-expressed in `contract/`, the hooks stay, and the helper cannot target a binary until the change in §6. Goldens keep Node as the oracle, which is why this is 3 rather than 2.

**C10 Solo maintainer (8).** A 5, copied: the operator already runs this image, in one language, with no build. B 4, copied: two libraries and an import map. C 3, copied: Vite, React, and Tailwind are a second upgrade train. D 2, copied: the operator would debug Elixir and OTP alone. E 3, copied: the App Router cache sits beside the auth rules. Go 3, judgment: one person debugs Go and a TypeScript player build. That is more than A's one language, and it is the runtime the operator chose.

**C11 Agents (8).** A 3, copied: agents mishandle escaped HTML strings. B 4, copied: htm is easier than string concatenation and less common in agent output than compiled JSX. C 5, copied: this is the markup agents write most reliably. D 2, copied: the repo is Node, so an agent has nothing local to copy. E 5, copied: agents write Next and React reliably. Go 4, judgment: agents write Go reliably, templ is less common than JSX, and this tree has no Go module to copy.

**C12 Docker, one replica (7).** A and B are 5, copied: the Dockerfile and `replicas: 1` stay, and B adds no build stage. C 4, copied: the image gains a build stage and the runtime stays one Node replica. D 3, copied: a release can be one replica, and it replaces the Dockerfile, the workflow, and the `node` uid. E 3, copied: Docker is documented, and the current image, command, and replica pin are replaced. Go 4, judgment: `replicas: 1`, `Recreate`, the PVCs, and uid 1000 stay, and the image becomes a static binary with a build.

**C13 Migration cost (8).** 5 means the smallest cost. A 5, copied: the new modules are additive. B 4, copied: the data does not move, and the front end gains a dependency and a second dialect. C 3, copied: the `shared/` UI is rewritten and the server is not. D 1, copied: the server, auth, data access, MCP boundary, UI, tests, and image all move. E 2, copied: the HTTP surface, the gate, and the UI are replaced while the database file can stay. Go 2, judgment: that same list moves, the file is not dumped, and Node stays the rollback image while one group moves at a time. Scoring any full server port as 1 would make this cell 1.

Go's 383 trails A by 74 and leads E by 5. B still leads A by 2, which is under C9's weight of 5, so ADR-0005's close-total rule is unchanged between A and B. Raising every judgment cell by one point (C3, C4, and C5 cannot rise) adds 76 and ties B at 459, a lead of 2 over A, and the close-total rule would still pick the lower migration cost. A's C13 is 5. Dropping Go's C13 to 1 subtracts 8 and puts Go at 375, three under E. The operator's choice of Go is recorded here; the table does not make it. The operator chose Go on 2026-10-18 despite the lower score.

---

## Consequences

Gained: a static binary, no native `sharp`, no experimental driver, image-only rollback, and goldens the validators lack today. Cost: two runtimes and a proxy until the end; FSRS in Go and TypeScript; images not matched to `sharp` bytes; a build for the player and templ; the 155 direct Node tests re-expressed in Go or `contract/`.

| Risk | Mitigation |
|---|---|
| UTF-16 length, key order, NFC, JS `u` regex vs RE2, message numbers, finding order | Surrogate, niqqud, and NFD goldens. Ordered JSON. All 11 schema patterns. |
| FSRS floats, `rngFor` / `seededShuffle`, local dates | Bit-exact PRNG. Fixed seed and clock. Tolerance only where `fsrs6-golden.json` has one. |
| Replacing `sharp` | Keep `sniff.js` refusals. `test/fixtures/media/*` checks dimensions, frames, metadata, pixel tolerance. Node media stays proxied until then. |
| Audio keys | §5 goldens (U+FEFF, U+0085). CI fails if an existing `.mp3` path would change. |
| Go SDK schemas and tokens | Snapshot the tool list. Node-minted tokens must still verify. 8787 moves in one step. |
| Two processes, one SQLite file | Go never shares the writer. Node is read-only for that group, or Go does not open the file (§7). `busy_timeout` stays 5000. The storage-class check is with CI/CD ([open question 4](#open-questions)). |

---

## Security considerations

Session required. Admin role for `/admin` and `/editor` (others 403). A hidden lesson is 404. Mutating `/api/` needs JSON, except multipart `POST /api/media`, plus an allowed Origin. HTMX writes use `/_hx/` and the Origin-plus-CSRF checks in Web and UI. `contract/` checks these. Node's loopback port stays inside the pod. The in-pod proxy forwards cookies and `Origin` unchanged. Secrets stay in `hebrew-secrets`.

Headers Node sets on app responses are `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, and `X-Frame-Options: DENY` (`server/http/security.js`). MCP OAuth HTML also sends a Content-Security-Policy; other responses do not. This repo does not set `Strict-Transport-Security`. The Cloudflare review saw `strict-transport-security: max-age=0; includeSubDomains; preload` on the live response. `max-age=0` disables HSTS and conflicts with `preload`. Go does not copy `max-age=0`. Whether Go sends HSTS, and the max-age, is [open question 10](#open-questions). The hop that adds the live header is not in this tree.

---

## Open questions

1. Resolved in [§9](#9-adr-0005-criteria-with-go). A–E are copied. Go scores 383, 74 behind A. The operator chose Go on 2026-10-18 despite that score.
2. Migrations 012+ while Node is the rollback: additive only, a Node twin, or Node refusing unknown versions.
3. What `contract/` asserts per route group. Home and owner are decided in §7.
4. Narrowed. §7 states the WAL rules. Go never shares the writer. Go-4 is proxy-only until the storage class is confirmed. The storage-class check is with CI/CD: `hebrew-data` must be a filesystem on that node that honors POSIX advisory locks and mmap of `hebrew.sqlite-shm`. `deploy/k8s-database.md` does not name the class.
5. Resolved in §6. The helper cannot drive an external binary. The spawn-and-port change above is the smallest one that keeps its HTTP API. Callers that `require` Node writers after `start` still share the file or move to HTTP.
6. Whether production `schema_migrations` holds exactly versions 1–11.
7. Production data sizes (dictionary rows, media volume).
8. Kubernetes probe paths. This repo only special-cases a user-agent starting with `kube-probe` (`mcp/http.js`).
9. Inbound Go server timeout values. Node's HTTP servers set none (§7). They must be lower than Cloudflare's 125 s proxy read timeout.
10. Whether Go sends `Strict-Transport-Security`, and the max-age. Do not copy `max-age=0` (§ Security). The header is not set in this repo. It comes from the edge, not the app.
11. For the operator: confirm Traefik passes `CF-Connecting-IP` through untouched, and give Traefik's peer address or CIDR for `TRUSTED_PROXIES` (§7).

## Rollback

Delete this file. No application file, database, or recording changes with it.

## References

- [ADR-0005](ADR-0005-stack-and-architecture.md), [ADR-0007](ADR-0007-single-page-app-structure.md), [ADR-0006](ADR-0006-visual-language-and-interface-rules.md), [ADR-0002](ADR-0002-accounts-server-progress-site-map.md)
- [INIT-014](../../sdd/initiatives/INIT-014-go-server-port/INIT-014-go-server-port-initiative.md) `INIT-014/SPEC-001`. Operator approval 2026-10-18, relayed by Hebrew Dev.
- Tree `86e8d50c`: `deploy/k8s-database.md`, `Dockerfile`, `server/{start,web,audio,lesson-data,content-filter}.js`, `server/db/`, `server/auth/`, `server/media/sniff.js`, `server/plans/`, `server/learner/`, `mcp/{http,oauth,store}.js`
- `github.com/modelcontextprotocol/go-sdk`, pkg.go.dev, checked 2026-10-18
- Cloudflare review of zone `ibhacked.us` (hebrew and hebrew-mcp), read-only, 2026-10-18. Not a file in this repo. Ingress and any Traefik header rewriting live in home_k3.
- SQLite WAL and locking, read 2026-10-18: [Write-Ahead Logging](https://www.sqlite.org/wal.html), [File Locking And Concurrency In SQLite Version 3](https://www.sqlite.org/lockingv3.html).

## Changelog

| Date | Change |
|------|--------|
| 2026-10-18 | Proposed for formal review. The operator has approved leaving Node in principle. Supersedes ADR-0005 and ADR-0007. |
| 2026-10-18 | Cloudflare edge notes: Traefik-only cutover, login IP is the first `X-Forwarded-For` hop (not `CF-Connecting-IP`), body caps and the 125 s proxy read timeout, HSTS `max-age=0` not copied. |
| 2026-10-18 | Login IP revised. Go uses `CF-Connecting-IP` only when the socket peer is in `TRUSTED_PROXIES`; otherwise the socket address. Node's leftmost `X-Forwarded-For` is a known throttle bypass and is not kept. |
| 2026-10-18 | Review notes. WAL rules and the two safe shapes are in §7; the storage class stays with Brian. INIT-014/SPEC-001 is proposed, not filed. The signed-in helper cannot spawn a binary. ADR-0005 re-scored with Go at 383. Phase labels are Go-0 through Go-17. |
| 2026-10-18 | Accepted. Brian Nelson approved moving off Node to Go on 2026-10-18, relayed by Hebrew Dev. Hebrew PR Reviewer approves and merges. Authored by `INIT-014/SPEC-001`. Former open question 1 is removed and the rest are renumbered. HTMX writes use `/_hx/` with Origin and a CSRF token. The storage-class check is with CI/CD. Go never shares the writer. |
