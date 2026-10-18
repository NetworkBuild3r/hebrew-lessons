# ADR-0016: Go server port

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Proposed |
| **Date** | 2026-10-18 |
| **Authored by** | — (no INIT/SPEC id yet; [open question 1](#open-questions)) |
| **Initiative** | — (open question 1) |
| **Research grounding** | None. Engineering record. Facts read in code at tree `86e8d50c`, 2026-10-18. |
| **Supersedes** | [ADR-0005](ADR-0005-stack-and-architecture.md) (Node runtime, `node:sqlite`, in-process TypeScript MCP, no build step, no second runtime). [ADR-0007](ADR-0007-single-page-app-structure.md) (single-shell SPA, as screens move). |
| **Superseded by** | — |

The operator has approved leaving Node in principle. This record is Proposed for formal review. ADR-0005 and ADR-0007 keep their **Superseded by** lines until this record is Accepted (same point those lines changed for ADR-0007, 2026-10-09).

## Summary

Move the server from Node 22 to Go one route group at a time, behind a Go front on the same ports. Go uses stdlib `net/http`, `modernc.org/sqlite` on the same `DATA_DIR/hebrew.sqlite` and schema, templ + HTMX for screens as they move, a small TypeScript lesson player, and the official MCP Go SDK on port 8787. No data moves. Sessions, MCP OAuth tokens, audio cache keys, and recording paths stay byte-compatible. A group moves only when Hebrew QA's suite in `contract/` passes against both Node and Go. Node stays the rollback image until the last phase.

---

## Context

### What ADR-0005 decided

ADR-0005 (Accepted 2026-10-08) chose Option A: Node 22, `node:sqlite`, in-process MCP on `@modelcontextprotocol/sdk` 1.32.1, no framework, no build step, no second runtime. Option B led by two points (459 to 457); A won on migration cost. Its carry plan (§3) already required Deployment `hebrew`, `replicas: 1`, `Recreate`, the same PVCs and Secret, ports 8080 and 8787 on one pod, a read-only check before any write, and no recording deleted or overwritten without an archive. It lists Go on the official MCP SDK list.

### What ADR-0007 decided, and what replaces it

ADR-0007 (Accepted 2026-10-09) put signed-in screens in one shell: a hand-built History API router, header painted once, login on its own page, no new dependency and no build step. templ + HTMX replace that shell as each screen moves. A small TypeScript player, compiled to static JS under the existing `/shared/` gate, plays the lesson. Screens not yet moved stay on today's shell behind the proxy. Login stays separate. ADR-0006 `data-*` hooks stay so the Chrome scripts still run.

### Why ADR-0005 no longer holds

The operator is choosing Go. The old cost was carrying the MCP SDK, the database, the tests, and the image. Go is an official SDK, and a pure-Go driver can open the same file. `node:sqlite` is still experimental: `server/db/index.js` swallows the Node 22 warning and re-emits it. Routing, the literal parser, the schema validator, and security headers are hand-written, on two direct dependencies (`@modelcontextprotocol/sdk` `^1.32.1`, `sharp` `0.35.5`). Re-scoring ADR-0005's table is [open question 2](#open-questions). This record does not invent a score.

### Current state (`86e8d50c`)

One process, ports 8080 and 8787. No Go code. The only workflow builds and pushes the image and does not run tests. SQLite is WAL, `foreign_keys=ON`, `busy_timeout=5000`: 20 tables, migrations `001`–`011` recorded by filename, `grammar_concepts` seeded by `005`. Lesson files are non-executed `window.NAME = <literal>;` assignments (`server/lesson-data.js`). The server requires `shared/player/fsrs.js` and `review.js`; `utils.js` runs in `vm` from `server/audio.js` and is required from `content-filter.js` and `learner/coverage.js`. Tests: 155 `test/*.test.js` files, 24 `test/chrome-*.js` scripts.

---

## Decision

### 1. Target stack

| Concern | Choice |
|---|---|
| HTTP | Stdlib `net/http` (1.22 patterns). Exact path before `:param`. JSON 404 under `/api/`. SDK release on 2026-10-18 declares Go 1.25; do not pin lower. |
| Database | `modernc.org/sqlite`, `CGO_ENABLED=0`, same file, WAL, `foreign_keys=ON`, `busy_timeout=5000`. One writer plus read-only handles. `BEGIN IMMEDIATE`. Timestamps as JS `toISOString()` (`YYYY-MM-DDTHH:MM:SS.sssZ`). |
| UI | templ + HTMX per screen. `data-*` hooks stay. |
| Player | Small TypeScript player, compiled to static JS, existing `/shared/` gate (fonts and brand stay public). |
| MCP | `github.com/modelcontextprotocol/go-sdk/mcp` on `:8787` `/mcp`. Same tool names, schemas, OAuth routes. |
| Crypto, text, images | `golang.org/x/crypto/scrypt`, `golang.org/x/text/unicode/norm`, `image/png`, `image/jpeg`, `golang.org/x/image/{webp,draw}`. Keep `sniff.js` animation refusals (PNG `acTL`/`fcTL`/`fdAT`, WebP flag `0x02`). |

### 2. Migrations

Versions 1–11 are frozen. If the ledger already has them, Go runs nothing. On an empty database, Go applies `baseline.sql` and inserts rows 1–11 with the original names (`001-initial.js`, …) so Node can open the file. `baseline.sql` is `sqlite_master` plus the seeded `grammar_concepts` rows after the Node migrations run on empty. A golden test compares the two schemas and seed rows.

012+ are SQL files embedded in the Go binary, same ledger. While Node is the rollback, 012+ migrations stay additive. Whether Node refuses an unknown version is [open question 3](#open-questions).

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

- Cache key: `sha256(voiceId + "\0" + model + "\0" + trim(text)).hex[0:24]`, with JS `String.prototype.trim` (`server/audio.js` `cacheKey`). Go `strings.TrimSpace` differs: JS trim removes U+FEFF and keeps U+0085; `TrimSpace` does the opposite. PR 1's goldens include at least those two.
- Text hash is the full hex of that same input.
- Word files are `<key>.mp3` under `LESSONS_DIR/_audio/words/`. Site paths are `../_audio/words/<file>`.
- Keep the `<key>-*.mp3` lookup, the lesson archive `<lesson>/audio/_archive/<stem>.<first-12-hex-of-file-sha256>.mp3` (never replace an archive that exists), `_audio/words/manifest.json` (`version`, `voiceId`, `model`, `entries`), and `<lesson>/audio/manifest.json` (`version`, `clips`).

Go never overwrites, deletes, or regenerates an existing recording. An existing file is a hit: no ElevenLabs call. A change that would move any existing key or path fails CI.

### 6. Golden-file tests

A Node script at the same commit writes `testdata/golden/**` (input, options, exact output) plus the Node version and SHA. Go compares canonical JSON, order-sensitive arrays, and exact messages. CI regenerates from Node and fails on `git diff`.

PR 1's audio-key goldens include the JS `trim` vs `strings.TrimSpace` cases in §5 (at least U+FEFF and U+0085).

Also covered: `validatePlan` and `checkHebrew` (ADR-0003 samples, `test/fixtures/plans-1.1`, `test/fixtures/lesson-types`, one mutation corpus per rule); FSRS and ladder via `test/fixtures/fsrs/fsrs6-golden.json` (shared with the TypeScript player); `learner/model.js` and `mix.js` at a fixed clock, seed, and DB fixture; audio keys, hashes, and paths; the lesson-data parser on `worksheets/**/*.js` and the §3 writers; the migration schema and `grammar_concepts` seed.

### 7. Strangler cutover

Go listens on `:8080` and proxies every route it has not taken to Node on a loopback port in the same pod. A phase moves a whole route group.

Hebrew QA owns the contract suite. It lives in this repo at `contract/`, with its own `go.mod`. A group moves only when that suite passes against both Node and Go. A failed group returns to the Node proxy by config, with no new image. What the suite asserts per group is [open question 4](#open-questions).

From ADR-0005 §3 and `deploy/k8s-database.md`: `replicas: 1`, `strategy: Recreate`, PVCs `hebrew-data` and `hebrew-lessons` at the same paths, Secret `hebrew-secrets` and the same keys, `runAsUser` and `fsGroup` 1000, ports 8080 and 8787 on that pod (`Dockerfile` `EXPOSE`; the Deployment snippet has no `ports` list). No second replica.

Move these whole. Their state is in the process: auth (in-memory throttle in `server/auth/throttle.js`: 5 username failures, 30 s backoff doubling to 15 min, 20 failures per IP per 15 min); MCP on 8787, including in-memory codes and pending requests; every writer of one lesson file or of `mcp-oauth-state.json`.

Rollback is the Node-only image. Nothing was converted, so Node runs on the files, subject to additive migrations.

### 8. PR sequence

| PR | What lands |
|---|---|
| 0 | This ADR. |
| 1 | Go module, validators, Node golden generator, Go CI. Nothing served. Audio-key goldens include JS `trim` vs `strings.TrimSpace` (U+FEFF, U+0085). |
| 2 | Lesson-data parser and the §3 writers. |
| 3 | Config, paths, DB open, ledger-compatible migrations, read-only on a DB copy. Baseline golden. |
| 4 | Go on `:8080` proxying all routes to Node, plus `GET /api/me`. |
| 5–11 | Auth and sessions; static gate and filtered scripts; read-only content APIs; FSRS, progress, review; mix, library, cards; plans; admin users and lessons. |
| 12–14 | Google sign-in; media (pure Go); audio and word-audio. |
| 15 | MCP and OAuth. 8787 moves whole. |
| 16 | templ + HTMX and the TypeScript player, screen by screen. |
| 17 | Remove the Node proxy and image. Port or retire `scripts/`. |

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
| Two processes, one SQLite file | One writer per table group. `busy_timeout` stays. See [open question 5](#open-questions). |

---

## Security considerations

Session required. Admin role for `/admin` and `/editor` (others 403). A hidden lesson is 404. Mutating `/api/` needs JSON, except multipart `POST /api/media`, plus an allowed Origin. Security headers stay. `contract/` checks these. Node's loopback port stays inside the pod. The proxy forwards cookies and `Origin` unchanged. Secrets stay in `hebrew-secrets`.

---

## Open questions

1. INIT/SPEC ids and the operator gate for this record.
2. Whether to re-score ADR-0005's weighted criteria for Go.
3. Migrations 012+ while Node is the rollback: additive only, a Node twin, or Node refusing unknown versions.
4. What `contract/` asserts per route group. Home and owner are decided in §7.
5. Whether WAL with two processes is safe on the `hebrew-data` volume. WAL needs a local filesystem. The storage class is in the home_k3 repo, which this survey could not read. Pending with the operator.
6. Whether `test/helpers/signed-in-server.js` can drive an external binary, so current HTTP tests can target Go.
7. Whether production `schema_migrations` holds exactly versions 1–11.
8. Production data sizes (dictionary rows, media volume).
9. Kubernetes probe paths. This repo only special-cases a user-agent starting with `kube-probe` (`mcp/http.js`).

## Rollback

Delete this file. No application file, database, or recording changes with it.

## References

- [ADR-0005](ADR-0005-stack-and-architecture.md), [ADR-0007](ADR-0007-single-page-app-structure.md), [ADR-0006](ADR-0006-visual-language-and-interface-rules.md), [ADR-0002](ADR-0002-accounts-server-progress-site-map.md)
- Tree `86e8d50c`: `deploy/k8s-database.md`, `Dockerfile`, `server/{start,web,audio,lesson-data,content-filter}.js`, `server/db/`, `server/auth/`, `server/media/sniff.js`, `server/plans/`, `server/learner/`, `mcp/{http,oauth,store}.js`
- `github.com/modelcontextprotocol/go-sdk`, pkg.go.dev, checked 2026-10-18

## Changelog

| Date | Change |
|------|--------|
| 2026-10-18 | Proposed for formal review. The operator has approved leaving Node in principle. Supersedes ADR-0005 and ADR-0007. |
