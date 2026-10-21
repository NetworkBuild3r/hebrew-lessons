# SPEC-008 · API: API foundation, sign-in endpoints, content gating

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-002
title: API foundation, sign-in endpoints, content gating
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 6 hours
blocked_by: [SPEC-004, SPEC-006]
blocks: [SPEC-009, SPEC-010, SPEC-011, SPEC-012]
```

---

## Description

**Context:** `server/web.js` serves static files, GET/HEAD only, from an allowlist (`/index.html`, `/manifest.webmanifest`, `/icon.svg`, `/shared/`, `/worksheets/`, `/review/`). It has no cookies or auth, and a bare `/review/` returns 404. REQ-002 requires sign-in for everything except the login page. REQ-003 requires sessions, CSRF defense, and server-side checks. REQ-010 requires per-user filtered data and hidden lesson paths.

**Scope (in):**
- **`server/http/`.**
  - `cookies.js`: parse and serialize, with the ADR-0002 cookie name and flags; `Secure` and the `__Host-` prefix when `COOKIE_SECURE=1`.
  - `body.js`: JSON-only body reader with a size limit (64 KB default, overridable per route).
  - `security.js`: Origin/Referer check against `PUBLIC_ORIGIN` or the Host. Security headers on every response: `X-Content-Type-Options: nosniff`, `Referrer-Policy: same-origin`, `X-Frame-Options: DENY`, and `Cache-Control: no-store` on API and per-user responses.
- **`server/api/router.js`.**
  - A tiny method + path router mounted at `/api/`, with `requireSession` and `requireAdmin` guards.
  - Allowed methods: POST/PUT/PATCH/DELETE on `/api/` only, and each must carry `Content-Type: application/json` and pass the Origin check.
  - **Errors:** a JSON error shape. 401 for no session, 403 for the wrong role, and 404 for a lesson the user can't see.
- **`server/api/auth-routes.js`.**
  - `POST /api/session` signs in. It sets the cookie and rotates the session; when the account has `must_change_password`, the response says so.
  - `DELETE /api/session` signs out.
  - `GET /api/me` returns id, username, display name, role, and `must_change_password`.
  - `POST /api/me/password` changes the password: it requires the current one and revokes other sessions.
- **`server/web.js` gating.**
  - **Public (ADR-0002 table):** `/login/index.html` and its own assets.
  - **Signed out:** an HTML request gets a 302 to `/login/?next=<path>`, where `next` must be a same-site absolute path (no scheme, no `//`, no backslash). Any other request gets 401.
  - **Filtered data scripts:** `catalog.js`, `_review/vocab-index.js`, `_audio/words/catalog.js`, and `shared/word-audio-catalog.js` come from SPEC-006's filters.
  - **Lesson paths:** `/worksheets/<slug>/*` returns 404 when `canSeeLesson` is false.
  - **Admin pages:** `/admin/*` requires the admin role.
  - **New page directories** on the allowlist: `/lessons/`, `/progress/`, `/admin/`, and `/login/`. `/dir/` serves `/dir/index.html`.
- `server/start.js`: open the database and migrate before listening. The server fails to start if `DATA_DIR` is unsafe (SPEC-003).

**Scope (out):** progress and admin routes (SPEC-009/010); pages themselves (SPEC-012 and later).

---

## Understood (agent's read of the intent)

- [ ] und-1: Gating is decided on the decoded, normalized, resolved file path. It is never decided on the raw URL string.
- [ ] und-2: The MCP server on 8787 is unchanged by this spec. Its auth is a separate follow-up (INIT-001 MED-1).

## Acceptance Criteria

- [ ] ac-1 (REQ-002): signed out, a request to each existing and new path is refused. Covered: `/`, `/index.html`, `/review/`, `/lessons/`, `/progress/`, `/admin/`, a lesson page, a lesson `worksheet.js`, an mp3, `catalog.js`, `vocab-index.js`, and both word-audio catalogs. HTML requests get 302 to `/login/?next=…`; others get 401. `/login/` and its assets get 200.
- [ ] ac-2 (REQ-002): path-trick variants of a gated or hidden path are still gated: `%2e%2e`, `%2f`, mixed case, a trailing dot, double slashes, a backslash, and an encoded NUL. Tested from a table.
- [ ] ac-3 (REQ-003):
  - Sign-in sets an HttpOnly, `SameSite=Lax` cookie, plus `Secure` and the `__Host-` prefix under `COOKIE_SECURE=1`.
  - A pre-existing session cookie is replaced, not reused.
  - Signing out revokes the session on the server, so the old cookie then gets 401.
- [ ] ac-4 (REQ-003): POST/PUT/PATCH/DELETE without `Content-Type: application/json`, or with a foreign `Origin`, gets 403 before any handler runs. Non-GET methods outside `/api/` get 405.
- [ ] ac-5 (REQ-003): `next` values `https://evil.example`, `//evil.example`, `/\evil.example`, and `javascript:` all redirect to `/` after sign-in.
- [ ] ac-6 (REQ-010): a user without a grant gets 404 for every file under a restricted lesson's folder. They also get the filtered versions of the four data scripts, and those responses carry `Cache-Control: no-store`.
- [ ] ac-7 (REQ-003): `/admin/index.html` returns 403 for a user-role session and 200 for an admin.
- [ ] ac-8 (REQ-014): lesson pages still load `worksheet.js` by script tag, and no existing lesson `index.html` is modified.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Content-Security-Policy | out of scope here (existing pages use inline scripts); recorded as a follow-up in ADR-0002 | resolved | yes — plan default |
| aud-2 | `next` on sign-in page | handled server-side on redirect and re-validated client-side in SPEC-012 | resolved | yes — plan default |

---

## Deliverables

- [ ] `server/http/{cookies,body,security}.js`, `server/api/router.js`, `server/api/auth-routes.js`
- [ ] `server/web.js` (gating, filtered scripts, new directories, directory index), `server/start.js` (database open and migrate)
- [ ] Tests: `test/web-gating.test.js`, `test/api-auth.test.js`, `test/http-security.test.js` (spin up `web.listen()` on an ephemeral port with a temp `DATA_DIR` and fixture lessons)
- [ ] `test/helpers/signed-in-server.js`: boots `web.listen()` with a temp `DATA_DIR` and fixture or real lessons, creates users through `server/accounts.js`, and returns a session cookie per user. The HTTP tests in SPEC-009 and SPEC-010 and the Chrome smoke scripts in SPEC-012 to SPEC-017 use it.

---

## Technical Requirements (domain-specific)

- [ ] No framework; plain `node:http`, matching the current `web.js` style.
- [ ] Every API response is JSON with `Cache-Control: no-store`. Errors never include stack traces, hashes, or tokens.
- [ ] Update the existing `test/lessons-dir.test.js` so it signs in first, keeping its intent.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the listed files. Static serving returns to today's behavior.

---

## Security

- **Sensitivity:** required. Triggers matched: authentication, session cookies, CSRF, authorization, information disclosure, external input.
- **Review depth:** full. Security acceptance: ac-1 to ac-7.

---

## Verification Strategy

- **Claim:** every path except the login page requires a valid session. Hidden lessons are invisible to non-granted users. State changes resist CSRF and open redirects, and sessions are rotated and revoked as specified.
- **Check + executor:** `mechanized`. The three HTTP test files run against a live `web.listen()` on an ephemeral port.
- **Pass condition:** all tests pass. The signed-out path table (ac-1) and the path-trick table (ac-2) report every row refused, and the open-redirect table (ac-5) reports every row landing on `/`.

---

## Integration Points

- Consumed by SPEC-009 and SPEC-010 (they register routes on the router), SPEC-011 (env names `DATA_DIR`, `COOKIE_SECURE`, `PUBLIC_ORIGIN`), and SPEC-012 (login page and `/api/me`).
- **Provenance tag:** `INIT-002/SPEC-008`
