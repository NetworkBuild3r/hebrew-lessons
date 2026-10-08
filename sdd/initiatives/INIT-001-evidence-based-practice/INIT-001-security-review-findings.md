# Security Review: INIT-001 Evidence-Based Practice (initiative-wide)

**Provenance tag:** `INIT-001/SECURITY-REVIEW`
**Date:** 2026-10-08
**Reviewer:** Inline engine run (`sdd-code-security`, Option B), targeted at the initiative's changed code
**Target:** Initiative diff against `HEAD` (79f0381): `server/`, `mcp/`, `shared/` client sinks, `review/`, `.cursor/skills/hebrew-worksheet/scripts/validate.js`, `docker-compose.yml`, `Dockerfile`, `.gitignore`, `.dockerignore`
**Review Depth:** thorough-equivalent for the code paths listed under Coverage; not a full endpoint-by-endpoint OWASP API sweep (the initiative adds no new HTTP API endpoints)
**Overall Risk Level (after remediation):** Medium, with one operator action open (SEC-OPS-1)

---

## Executive Summary

The initiative was reviewed for code execution, path handling, DOM injection, secret handling, and the container and MCP surface. One High finding was confirmed: lesson `worksheet.js` files were evaluated with Node's `vm` module, which is not a security boundary. A constructor-chain escape reached the host `process` object, including `process.env`. It was remediated in this pass: lesson and catalog data are now parsed as literals and never executed, with regression tests (114/114 pass). A second High is open and operational: the ElevenLabs API key appeared in tool output during this work, so the operator must rotate it. One Medium remains open: compose runs the MCP write tool with no bearer token. No unresolved Critical findings remain.

---

## Findings Summary

| SECFIND ID | ID | Severity | OWASP | Title | Status |
| ---------- | -- | -------- | ----- | ----- | ------ |
| not assigned (registry absent) | HIGH-1 | High | API8 / CWE-94 | vm sandbox escape when evaluating lesson data | remediated (pending independent re-review) |
| not assigned (registry absent) | SEC-OPS-1 | High (operational) | — | ElevenLabs API key exposed in tool output | open, operator action required |
| not assigned (registry absent) | MED-1 | Medium | API2 | MCP write tool runs with no bearer token (`MCP_ALLOW_NO_AUTH: "1"` in compose) | open, follow-up |
| not assigned (registry absent) | LOW-1 | Low | API8 | Container runs as root (pre-existing, unchanged by this initiative) | open, follow-up |
| not assigned (registry absent) | LOW-2 | Low | API9 | Static web server has no authentication; serves non-secret lesson content (pre-existing; `/review/` inherits it) | open, follow-up |

**Totals:** 0 Critical, 2 High (1 remediated, 1 open operator action), 1 Medium, 2 Low

**Registry note:** `sdd/security-reviews/` does not exist in this repo and `scripts/dev/security_scan_indexer.py` is absent, so no `SECSCAN-###` or `SECFIND-###` ids were minted or deduplicated. Assign them when the registry is available.

---

## High-Risk Findings

### HIGH-1: vm sandbox escape when evaluating lesson data (remediated)

**OWASP Category:** API8 (unsafe evaluation / security misconfiguration), CWE-94 (code injection)
**VULN class ID:** not assigned (`vulnerability-taxonomy.md` not consulted in this pass)
**Affected code (before):**
- `mcp/store.js` `loadWindow`, on the `getLesson`, `listWords`, and `readCatalog` paths used by MCP tools (unchanged by this initiative; the pattern predates it)
- `server/audio.js` `loadWindow` (unchanged by this initiative; the pattern predates it)
- `scripts/rescue-lesson.js` catalog read (unchanged by this initiative)
- `server/vocab-index.js` `loadWindow` (new in INIT-001/SPEC-002, same pattern)
- `.cursor/skills/hebrew-worksheet/scripts/validate.js` `loadWindow` (modified by INIT-001/SPEC-008)

The first pass of this review scanned only `mcp/tools.js` and missed `mcp/store.js` and `scripts/rescue-lesson.js`. A repo-wide search for `vm` found them. The finding's scope is wider than the first pass recorded.

**Description:** each loader read a lesson `worksheet.js` or catalog into `vm.runInContext`. Node documents `vm` as not a security mechanism. Confirmed in this review: code inside the context reached the host `process` object via `this.constructor.constructor('return process')()`. That gives read access to `process.env`, including `ELEVENLABS_API_KEY`, and the ability to run arbitrary code in the server process.

**Exploit scenario:** `upsert_lesson` writes `worksheets/<slug>/worksheet.js` and then calls `generateLessonAudio` (`mcp/tools.js:113,121,125`) and `rebuildVocabIndexSafe` (`mcp/tools.js:135`), which evaluate that file. The lesson skill turns class notes and transcripts into lessons, so a prompt injection inside a transcript could make the agent write a worksheet containing the escape payload. Evaluation then runs it in the MCP host process. Preconditions: attacker-controlled text reaches the authoring agent. Blast radius: the MCP host process, its environment, and the local filesystem.

**Remediation (applied):**
- New `server/lesson-data.js`: a literal-only parser. It accepts `window.NAME = <literal>;` with object and array literals, quoted strings (no `${}`), finite numbers, `true`/`false`/`null`, comments, and trailing commas. It also accepts the one exact `Object.assign({}, window.NAME || {}, {literal})` merge shape that the generated word-audio catalogs use. Anything else throws `SyntaxError`. Object keys are written with `defineProperty`, so `__proto__` cannot change a prototype.
- `server/audio.js`: lesson and catalog reads use `loadDataWindow` (lines 293, 328). The one remaining `vm` call, `loadTrustedWindow` (line 46), loads only the trusted repo file `shared/player/utils.js`, and is named to make that boundary explicit.
- `server/vocab-index.js` (line 20), `validate.js` (line 23), and `mcp/store.js`: `loadWindow` delegates to `loadDataWindow`, and the `vm` import is removed.
- `scripts/rescue-lesson.js`: the catalog read calls `loadDataWindow`.
- Repo-wide check after the change: the only `vm` call left in non-test code runs on the fixed trusted path `shared/player/utils.js`. The other `require` calls load repo modules (`shared/activities/catalog.js`), not lesson data.
- `test/lesson-data.test.js`: 20 tests. They cover the exact escape payload, `require`, `process.env`, `Object.assign` look-alikes, template interpolation, IIFEs, and a `__proto__` key. They also parse every real lesson, catalog, and shared catalog file in the repo.

**Test-gap note:** the existing `hostile` vocab-index fixture only checked that a bare `require` call failed. The constructor-chain escape passed that check. The new tests close that gap.

**Verification:** `node --test test/*.test.js` = 114 pass, 0 fail (was 94). The `hostile` fixture now fails inside the parser. An independent re-review of `server/lesson-data.js` is still recommended.

**Effort estimate:** completed in this pass.

---

### SEC-OPS-1: ElevenLabs API key exposed in tool output (operator action required)

**OWASP Category:** not applicable (operational secret handling, not a code defect)
**Affected:** `.env` key `ELEVENLABS_API_KEY`
**Description:** the key value was printed into agent session output during this initiative: once by an earlier file read, and once more by a search during this review. The `.env` file is correctly excluded: `.gitignore:2` and `.dockerignore:4` both list `.env`, and `git ls-files` does not track it. This review did not write the key to any tracked file, image, or log. The exposure is to the agent session transcript and the model provider's input, not to git.

**Exploit scenario:** anyone with access to those session transcripts can use the key until it is revoked, which can incur charges against the account.

**Remediation (operator):** rotate `ELEVENLABS_API_KEY` in ElevenLabs, update `.env` locally, and update the vault secret used by k8s. Use a key-presence check instead of printing `.env` in future sessions.

**Status:** open. The initiative's security gate treats this as unresolved until the operator rotates the key or records a risk acceptance.

---

## Medium-Risk Findings

### MED-1: MCP write tool runs without authentication in compose

**OWASP Category:** API2 (broken authentication)
**Affected code:** `docker-compose.yml` (`MCP_ALLOW_NO_AUTH: "1"`; `MCP_AUTH_TOKEN` empty by default); `mcp/http.js` `listen` (refuses non-loopback binding without a token unless `MCP_ALLOW_NO_AUTH=1`); `mcp/tools.js` `upsert_lesson` (writes lesson files)
**Description:** the MCP server exposes `upsert_lesson`, which writes lesson files and triggers audio generation and the evaluation path. In compose it runs with `HOST: 0.0.0.0` inside the container and `MCP_ALLOW_NO_AUTH=1`, so no bearer token is required. Host ports are published on `127.0.0.1` only, so the exposure is to processes on the same machine. The Host-header check (`allowedHost`) limits DNS-rebinding access. HIGH-1 is now fixed, which limits the impact. The remaining impact is that any local process or user can add or overwrite lessons.
**Exploit scenario:** a local process running as another user calls `upsert_lesson` on `127.0.0.1:8787` and overwrites a lesson, or adds a worksheet that later appears in the learner's player.
**Remediation:** set `MCP_AUTH_TOKEN` in `.env` (never committed), remove `MCP_ALLOW_NO_AUTH` from compose, and configure the MCP client with the bearer token. Compose passes `MCP_AUTH_TOKEN: ${MCP_AUTH_TOKEN:-}`, so an empty value must not be relied on.
**Effort Estimate:** 1 hour, including client configuration.

---

## Low-Risk Findings

### LOW-1: container runs as root (pre-existing)

**OWASP Category:** API8
**Affected code:** `Dockerfile:1` (`FROM node:22-alpine`, no `USER` directive)
**Description:** the image runs `node server/start.js` as root. This predates INIT-001 and is unchanged by it.
**Remediation:** add `USER node` after the `COPY` steps and before `CMD`. The `node:22-alpine` base image includes the `node` user.
**Effort Estimate:** 15 minutes, plus a smoke test.

### LOW-2: static web server has no authentication (pre-existing)

**OWASP Category:** API9 (improper inventory / exposure)
**Affected code:** `server/web.js` `publicFile` (allowlist `/`, `/shared/`, `/worksheets/`, `/review/`); `docker-compose.yml` publishes `127.0.0.1:8080`
**Description:** the static server has no authentication. It exposes lesson content, shared JS, and word audio, not secrets. The `/review/` route added here is in the same allowlist and inherits this exposure. Host publishing is loopback-only in compose. `publicFile` rejects `..` and dotfiles, confines `/worksheets/` to the lessons directory, and accepts only GET/HEAD.
**Remediation:** confirm the k8s ingress does not expose port 8080 beyond the intended audience. If it does, add an auth layer in front.
**Effort Estimate:** 0.5 hours to confirm; more if auth is needed.

---

## Positive Security Observations

- `publicFile` in `server/web.js` rejects `..`, dotfiles, NUL bytes, and non-allowlisted prefixes, and checks resolved paths against the lessons root. Only GET and HEAD are accepted.
- DOM sinks escape their inputs. `shared/review-page.js` routes every lesson-sourced value through `esc()`, and the `data-*` attributes are escaped too. `shared/activities/story.js`, `fill.js`, and `pattern.js` use `U.escapeHtml` or `ctx.esc` for dynamic text. `U.escapeHtml` covers `& < > "`.
- The ElevenLabs key is read from `process.env` (populated from `.env` without overriding existing variables) and sent only in the `xi-api-key` header. It is never logged. Error messages carry the upstream response detail (first 500 characters), not the request headers.
- `.env` is excluded from git (`.gitignore:2`) and from the Docker build context (`.dockerignore:4`).
- `docker-compose.yml` does not set `ELEVENLABS_API_KEY` (an empty value would override `env_file`), and documents the k8s path to inject it from the vault.
- `review-page.js` and the review store use `Object.create(null)` for id maps in the merge and index paths.
- `server/vocab-index.js` `resolveLessonDir` throws if a lesson name escapes the lessons base directory.
- The MCP server refuses non-loopback binding without a token unless `MCP_ALLOW_NO_AUTH=1` is set, and applies a Host-header check.

---

## Coverage

**Examined:** `server/web.js` (all), `server/audio.js` (key loading, TTS call, loaders), `server/vocab-index.js`, `server/lesson-data.js` (new), `server/start.js`, `mcp/tools.js` (upsert path), `mcp/store.js` (loaders and lesson read paths), `mcp/http.js` (`listen` guard and Host check only), `scripts/rescue-lesson.js` (catalog read), `shared/review-page.js`, `shared/player/review.js` (store and id maps), `shared/player/utils.js` (`escapeHtml`), `shared/activities/story.js`, `fill.js`, `pattern.js`, `review/index.html`, `.cursor/skills/hebrew-worksheet/scripts/validate.js`, `docker-compose.yml`, `Dockerfile`, `.gitignore`, `.dockerignore`.

**Not examined in depth:** the MCP OAuth handler and bearer-token verification in `mcp/http.js` (the `oauth.handle` path); the k8s manifests (`deploy/k8s-lessons-volume.md` is referenced but was not read in this pass); the Hebrew normalization logic beyond escaping; and the `taxonomy` and `capec` corpus mapping. No stack-specific focus area (frontend XSS or agentic) was loaded, so the frontend and agentic checks are from first principles only.

**Not run:** the `security_scan_indexer.py` registry claim and dedup (script absent); any SAST, dependency, or secret scanner (none configured in this repo).

---

## Recommended Next Steps

1. **Immediate (before completion is marked final):** rotate `ELEVENLABS_API_KEY` (SEC-OPS-1).
2. **Short-term:** have an independent reviewer re-check `server/lesson-data.js`, the one part of this review that changed code (HIGH-1).
3. **Short-term:** set `MCP_AUTH_TOKEN` and drop `MCP_ALLOW_NO_AUTH` in compose (MED-1).
4. **Short-term:** add `USER node` to the Dockerfile (LOW-1).
5. **Medium-term:** confirm that ingress exposure of the web server is intended (LOW-2).
6. **When available:** record this scan in the registry and assign SECFIND ids.

---

## Compliance Mapping

Not applicable: this is a single-user, local-first learning tool with no personal data stores beyond the learner's own localStorage progress.
