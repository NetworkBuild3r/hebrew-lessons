# ADR-0002: Accounts, server-side progress, and site map

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Proposed |
| **Date** | 2026-10-08 |
| **Authored by** | `INIT-002/SPEC-001` |
| **Initiative** | INIT-002 · multi-user-progress |
| **Research grounding** | `RSCH-003` (language-learning science) for spacing, recall direction, and metrics; operator decisions for accounts and admin visibility |
| **Supersedes** | [ADR-0001](ADR-0001-learner-review-model.md) in part — REQ-009 constraint 1 (progress only in `localStorage` / no accounts) and the no-account consequence |
| **Superseded by** | Superseded in part by [ADR-0004](ADR-0004-learner-data-media-and-secrets.md) — the MCP learner-progress rule only. Replacement wording is ADR-0004 §11. The sentences in this file are unchanged. Also [ADR-0009](ADR-0009-learner-model-and-language-program.md), Accepted 2026-10-09 (in part — §10 stage threshold under `SRS_SCHEDULER=fsrs`, and the MCP learner-progress rule, whose wording ADR-0009 §9 replaces). |

## Summary

Introduce signed-in accounts (`user` and `admin`), put each learner's review schedule and lesson-round state on the server (authoritative), gate every path except the sign-in page, filter lesson visibility per user, and reshape the site around Today / Lessons / Review / Progress / Admin. ADR-0001's identity key, ladder, caps, signal table, and gloss schema stay in force; only where progress lives and who can see it changes.

---

## Context

### Problem statement

ADR-0001 fixed the review model under a hard constraint: progress only in `localStorage`, no accounts. The operator now needs:

1. Named accounts with two roles and safe session handling on an internet-facing site that includes adult lesson content.
2. Per-user progress that follows the learner across devices, with the server applying the ADR-0001 ladder.
3. Per-lesson visibility (hidden / everyone / selected) enforced on catalogs, lesson files, and APIs.
4. Admin oversight of learner activity (including full answer history), a browser lesson editor, and MCP tools so the AI can author and assign lessons.
5. A shared site layout with a single top bar.

Before any code lands, this record fixes those decisions so SPEC-003 through SPEC-017 implement one agreed model.

### Current state

- Progress: `hebrew-review:v1` and `hebrew-worksheet:<slug>` in the browser (`localStorage`); no accounts.
- Serving: `server/web.js` serves static files GET/HEAD-only from an allowlist; no cookies or auth.
- Lessons: published via MCP `upsert_lesson` (`mcp/store.js`); three lessons on disk including `anal-sex-vocab`.
- Review model: ADR-0001 identity, ladder, caps, signals, and glosses — still the scheduler contract.
- Audio: clip paths under `worksheets/` and shared catalogs; ElevenLabs key in server env only.

### Requirements (must)

| Id | Requirement |
|----|-------------|
| R1 | Local user database with roles `user` and `admin`; no public sign-up; first admin via CLI (REQ-001). |
| R2 | Sign-in required for every page, lesson file, data script, and recording except the login page and its assets (REQ-002). |
| R3 | scrypt password hashes, HttpOnly revocable session cookie, login throttling, CSRF defense, server-side admin checks (REQ-003). |
| R4 | Per-account review schedule and lesson-round progress on the server; fresh accounts — no import or delete of legacy browser keys (REQ-004). |
| R5 | Server is authoritative for scheduling; browser posts signals; ADR-0001 `nextState` with server time; append-only practice events (REQ-005). |
| R6 | Shared top bar and site map: Today, Lessons, Review, Progress, Admin (admins), account menu (REQ-006). |
| R7 | Progress metrics: memory stage and direction accuracy; no streaks or points (REQ-007). |
| R8 | Admin user lifecycle, last-admin protection, audit log (REQ-008). |
| R9 | Admin full view of learner activity including complete answer history; learners told; MCP never reads progress (REQ-009). |
| R10 | Lesson visibility modes with seed and filters (REQ-010). |
| R11 | Browser lesson editor through the same publish path as MCP (REQ-011). |
| R12 | Audio invariants: no path/URL change, archive-before-overwrite, inventory/verify gate (REQ-012). |
| R13 | `DATA_DIR` outside every served root; backup; one-replica limit (REQ-013). |
| R14 | Carried ADR-0001 constraints 2–7 remain binding; constraint 1 superseded (REQ-014). |
| R15 | Schema hooks for a later OAuth/OIDC provider (REQ-015). |
| R16 | MCP `list_users` / `upsert_lesson` / `set_lesson_visibility` with `MCP_AUTH_TOKEN` required; no instructor role (REQ-016). |

### Constraints

- Plan-only for this spec: decisions only; no application code.
- Do not put secrets, learner names, or transcript text in this document.
- ADR-0001 identity, ladder, caps, signal table, and gloss schema remain binding unless this ADR explicitly supersedes a point.
- One server replica is a hard operational limit (in-memory throttle, single SQLite writer).
- Content-Security-Policy is out of scope for this initiative (existing inline scripts); recorded as a follow-up.

---

## Options considered

### A. Storage engine

#### A1 — `node:sqlite` behind `server/db/` wrapper; `better-sqlite3` documented fallback — **chosen**

**Description:** Use Node 22's built-in `node:sqlite` through one wrapper module. Document `better-sqlite3` as the fallback if the built-in engine proves unsuitable. `DATA_DIR` defaults to `<root>/data` and must sit outside every served root.

**Pros:** No native dependency for the happy path; matches the container's Node 22; single wrapper keeps a later swap localized.

**Cons:** `node:sqlite` is still experimental; API surface may shift — mitigated by the wrapper and the documented fallback.

#### A2 — `better-sqlite3` only

**Description:** Add the native dependency from day one.

**Pros:** Mature sync API.

**Cons:** Adds a native build dependency and a `dependency_add` gate; unnecessary while `node:sqlite` is available. Rejected as primary; kept as fallback.

#### A3 — JSON files under `DATA_DIR`

**Description:** Per-user JSON documents on disk.

**Pros:** Simple to inspect.

**Cons:** Weak concurrency, no transactional batch apply for signals, awkward indexes for metrics and admin history. Rejected.

### B. Progress authority

#### B1 — Server authoritative; browser posts batched signals — **chosen**

**Description:** Browser keeps a pending queue per user and posts idempotent signal batches. Server applies ADR-0001 `nextState` with server `now`. Lesson-round state is a per-user JSON blob with a revision number.

**Pros:** Matches `RSCH-003` spacing across days (**High**); prevents clock tampering from advancing due dates; one ladder implementation (`shared/player/review.js`).

**Cons:** Requires connectivity for durable progress beyond the pending queue.

#### B2 — Keep localStorage as source of truth; optional sync

**Description:** Browser remains authoritative; server is a backup.

**Pros:** Offline-first.

**Cons:** Conflicts with multi-device and admin metrics; rejected by REQ-004/005.

### C. Visibility default for new lessons

#### C1 — `hidden` until assigned — **chosen**

**Description:** A lesson created with no visibility assignment is `hidden`. MCP/AI may pass visibility on `upsert_lesson` or call `set_lesson_visibility`; the tool reply states the result.

**Pros:** Fail-closed for adult or draft content; matches operator intent for "no assignment → not visible".

**Cons:** Authors must set visibility explicitly (acceptable; reply text makes the default obvious).

#### C2 — Default `everyone`

**Description:** New lessons appear for all users until restricted.

**Pros:** Fewer steps for public content.

**Cons:** Risk of leaking unfinished or adult content. Rejected.

---

## Decision

### Chosen options (summary)

- **Storage:** A1 — `node:sqlite` + `server/db/` wrapper; `better-sqlite3` fallback documented.
- **Progress:** B1 — server authoritative; batched idempotent signals; revisioned lesson blobs.
- **Visibility default:** C1 — `hidden` until assigned; first-run seed marks every lesson already on disk as `everyone` once.

### 1. Storage engine

**Options considered:** see §A above — A1 `node:sqlite` + wrapper (**chosen**), A2 `better-sqlite3` only (fallback, not primary), A3 JSON files (rejected).

**Chosen:** `node:sqlite` (Node 22 built-in, experimental) behind a single wrapper at `server/db/`. `better-sqlite3` is the documented fallback if the built-in engine is withdrawn or proves unsuitable in production.

| Rule | Value |
|------|-------|
| Wrapper path | `server/db/` (`open`, `prepare`/`get`/`all`/`run`, `transaction`, `migrate`, `close`) |
| Default `DATA_DIR` | `<repo-root>/data` (env override) |
| Placement rule | Resolved `DATA_DIR` must not equal or lie inside any served root (`shared/`, `review/`, `LESSONS_DIR` / `worksheets/`, static allowlist roots). Startup throws if violated. |
| Setup | WAL, `foreign_keys=ON`, `busy_timeout` |
| Backup | `VACUUM INTO` under `DATA_DIR/backups/` |
| Replica limit | One writer replica documented as hard limit |

**Rationale:** Avoids a native dependency while keeping a clean swap path; REQ-013 forbids serving the database file.

### 2. Schema outline

**Options considered:** (1) normalized SQLite tables as below — **chosen**; (2) one JSON document per user — rejected (weak indexes for metrics and admin history); (3) defer OAuth columns until needed — rejected (REQ-015 asks for hooks now).

Tables created by the initial migration (SPEC-003):

| Table | Purpose | Key columns / notes |
|-------|---------|---------------------|
| `users` | Accounts | `role` ∈ `user`,`admin`; `status` ∈ `active`,`disabled`; `auth_provider` default `local`; `provider_subject` nullable; unique (`auth_provider`,`provider_subject`) when subject set (REQ-015); `must_change_password`; password hash never returned by APIs |
| `sessions` | Server sessions | Store **SHA-256 of token only**; `created_at`, `last_seen_at`; FK user `ON DELETE CASCADE` |
| `review_items` | Per user × word × direction | ADR-0001 fields (`due`, `step`, `streak`, `lapses`, `lastResult`, `lastAt`); sources as lesson slugs; index (`user_id`,`due`) |
| `practice_events` | Append-only answer log | `id` (client idempotency key), word, direction, result, `at`, slug, activity; index (`user_id`,`at`) |
| `lesson_progress` | Per user × lesson round blob | JSON state + `rev`; size cap 64 KB; allowlisted keys |
| `lesson_visibility` | Per lesson mode | `hidden` \| `everyone` \| `selected` |
| `lesson_grants` | Selected-user grants | (`slug`,`user_id`) |
| `audit_log` | Admin / MCP actions | actor, action, target, time; also records admin reads of learner event history |
| `meta` / `schema_migrations` | Flags and migration versions | Includes one-time visibility seed flag |

Timestamps are ISO-8601 UTC strings, matching ADR-0001 `due` / `lastAt`.

**Rationale:** Covers accounts, progress, visibility, and audit without a schema rewrite for later OAuth hooks.

### 3. Passwords

**Options considered:** (1) `crypto.scrypt` with N=2^15, r=8, p=3 and a self-describing stored format — **chosen**; (2) argon2id via an npm package — rejected (extra dependency for this stack); (3) bcrypt — rejected (weaker cost model / awkward in Node relative to scrypt). Policy options: length-only + short deny list (**chosen**) vs composition rules (rejected as usability theater for this audience).

**Chosen parameters (stated once — implementing specs cite these):**

| Item | Value |
|------|-------|
| Algorithm | `crypto.scrypt` (Node built-in) |
| Cost | N = 2^15, r = 8, p = 3 (OWASP-listed equivalent class) |
| Salt | 16 random bytes per hash |
| Stored format | Self-describing string encoding parameters + salt + derived key (so cost can rise later) |
| Verify | Timing-safe compare |
| Policy — minimum length | 10 characters |
| Policy — maximum length | 200 characters |
| Policy — composition | None (no required classes) |
| Policy — deny list | Reject a short common-password list (e.g. trivial dictionary entries); exact list lives in code, not in this ADR |
| Admin reset | Sets `must_change_password`; forces change at next sign-in before navigation continues |
| Throttle (sign-in) | 5 failures per username → 30 s backoff, doubling to a 15 min lockout; per-IP cap 20 failures / 15 min; counts in memory |
| Enumeration | One generic error; unknown usernames run verify against a dummy hash so timing matches |

**Rationale:** Meets REQ-003 without new crypto packages; policy favors length over composition rules (usable passwords, still blocks trivial ones).

### 4. Sessions

**Options considered:** (1) opaque server-side sessions (random token, hashed at rest, fixed lifetime from sign-in) — **chosen**; (2) JWT access tokens in localStorage — rejected (XSS exfiltration, harder revocation); (3) short idle timeout plus remember-me — rejected (operator: 30 days from sign-in, no idle timeout, no remember-me).

| Item | Value |
|------|-------|
| Token | 32 bytes (256-bit) cryptographically random, encoded base64url for the cookie |
| Storage | Only SHA-256 of the token in `sessions` |
| Cookie name | `hebrew_session` when `COOKIE_SECURE` is unset; `__Host-hebrew_session` when `COOKIE_SECURE=1` |
| Flags | `HttpOnly`; `SameSite=Lax`; `Path=/`; `Secure` only when `COOKIE_SECURE=1` |
| Lifetime | **30 days from sign-in**; no separate idle timeout; no remember-me toggle |
| `last_seen_at` | Updated at most once per minute (admin display only; does not extend lifetime) |
| Rotation | New session at every successful sign-in; prior cookie replaced |
| Revocation | On sign-out, password change/reset, disable, or role change — revoke all sessions for that user (password change may keep the current session only if explicitly implemented; default: revoke others at self-change, revoke all at admin reset) |

**Rationale:** Long-lived sessions match the operator decision; hashed storage limits DB leak impact; `__Host-` when Secure is on ties the cookie to the host.

### 5. CSRF and Origin policy

**Options considered:** (1) JSON `Content-Type` + Origin/Referer check + `SameSite=Lax` — **chosen**; (2) synchronizer CSRF tokens — workable but extra round-trips for a same-site JSON API; deferred; (3) `SameSite=Strict` only — rejected (breaks top-level GET navigations after external links without solving API CSRF alone).

| Rule | Detail |
|------|--------|
| State-changing methods | `POST` / `PUT` / `PATCH` / `DELETE` allowed **only** under `/api/` |
| Content-Type | State-changing `/api/` requests must send `Content-Type: application/json` |
| Origin check | `Origin` (or `Referer` fallback) must match `PUBLIC_ORIGIN` or the request `Host`; mismatch → 403 before handlers |
| Non-API writes | Methods other than GET/HEAD outside `/api/` → 405 |
| Cookie | `SameSite=Lax` as defense-in-depth (not the sole CSRF control) |
| Body limits | 64 KB default; lesson save routes may raise to 1 MB; signal batch ≤ 200 items |
| Security headers (every response) | `X-Content-Type-Options: nosniff`; `Referrer-Policy: same-origin`; `X-Frame-Options: DENY`; `Cache-Control: no-store` on API and per-user filtered responses |
| CSP | Out of scope this initiative (follow-up) |

**Rationale:** Classic cookie CSRF is blocked by requiring a non-simple Content-Type plus Origin check; no separate CSRF token needed for this JSON-only API surface.

### 6. Authorization matrix (role × route)

**Options considered:** (1) two roles (`user`, `admin`) plus MCP actor for AI tooling — **chosen**; (2) add an `instructor` role — rejected by operator; (3) client-only hiding of Admin UI without server checks — rejected (not a control).

Roles: **`user`**, **`admin`**. No instructor role. Actor **`mcp`** (authenticated MCP process) is not a browser role; it is recorded on audit rows when MCP tools mutate lessons/visibility.

**Last-admin protection:** the last active admin cannot be demoted or disabled; an admin cannot disable themselves. Violations → 409.

**Audit log:** every mutating admin action writes one row (actor, action, target, time). Reading a learner's full answer history (`…/events`) also writes one audit row naming the admin and target user. MCP mutations audit actor `mcp`.

| Route group | Routes (representative) | Signed out | `user` | `admin` | Notes |
|-------------|-------------------------|------------|--------|---------|-------|
| **Session** | `POST /api/session`, `DELETE /api/session` | Sign-in allowed (throttled); sign-out no-ops or 401 | Sign-in rotates session; sign-out revokes | Same | Generic errors on failed sign-in |
| **Me** | `GET /api/me`, `POST /api/me/password` | 401 | Own profile; change password (current required) | Same | Password change revokes other sessions; `must_change_password` surfaced |
| **Review** | `GET /api/review/state`, `POST /api/review/signals` | 401 | Own data only; visible lessons only | Own data only (admin is also a learner for these routes) | No user id in URL; invisible slug signals rejected |
| **Lesson progress** | `GET`/`PUT /api/lessons/:slug/progress` | 401 | Own blob; 404 if cannot see slug; 409 on stale `rev` | Same | |
| **Metrics** | `GET /api/me/metrics` | 401 | Own metrics | Own metrics | Admin views of others are under admin progress |
| **Admin users** | `GET`/`POST /api/admin/users`, `PATCH …/:id`, `POST …/:id/password` | 401 | 403 | Allowed; last-admin / self-disable guards | Never returns hashes |
| **Admin progress & activity** | `GET /api/admin/users/:id/progress`, `GET /api/admin/users/:id/events?before=&limit=` | 401 | 403 | Full metrics, trouble words, paged events (newest first, limit 1–200) | Events read audited; MCP has **no** equivalent |
| **Admin lessons** | `GET/POST/PUT` lesson and visibility routes under `/api/admin/lessons…` | 401 | 403 | Create/edit/validate/set visibility | Publish via `mcp/store.upsertLesson` path |
| **Admin audio** | `GET …/audio-plan`, `POST …/audio` | 401 | 403 | Dry-run then generate | Throttle: 1 in-flight generation per slug; ≤ 10 runs per admin per hour |
| **MCP tools** | `list_users`, `upsert_lesson`, `set_lesson_visibility` | N/A (token auth) | N/A | N/A | Refuse unless `MCP_AUTH_TOKEN` is set and request presents it; `list_users` returns usernames + display names of **active** accounts only; never progress/events |

Static HTML under `/admin/*` requires an admin session (403 for `user`, redirect/401 rules as in §7 for signed-out).

### 7. Route gating

**Options considered:** (1) only `/login/` and its own assets public; everything else requires a session — **chosen**; (2) public read of non-adult lessons — rejected (REQ-002; adult content on an internet-facing host); (3) 403 for invisible lessons — rejected in favor of 404 to avoid confirming slug existence.

**Public paths only** (everything else is gated):

| Public path | Notes |
|-------------|-------|
| `/login/` | Serves `/login/index.html` |
| `/login/index.html` | Sign-in page |
| `/login/*` assets referenced by that page only (same directory tree; no shared lesson/audio catalogs) | Scripts, CSS, and images owned by the login page |
| `/manifest.webmanifest`, `/icon.svg` | Added 2026-10-08. Identical for everyone, no user data. Browsers fetch the manifest without cookies, so gating it returned a 401 on every page. |
| `/shared/fonts/*` | Added 2026-10-08. Self-hosted Rubik (SIL OFL) replacing the Google Fonts link. Identical for everyone, and the sign-in page needs it before anyone is signed in. The rest of `/shared/` stays gated. |

**Gated responses:**

| Case | HTML navigation (`Accept` / navigational GET to a page) | Other (API, scripts, audio, data) |
|------|----------------------------------------------------------|-----------------------------------|
| Signed out | `302` → `/login/?next=<path>` where `next` is a same-site absolute path only (must start with `/`, no scheme, no `//`, no `\`) | `401` |
| Signed in, `user`, path under `/admin/` | `403` | `403` |
| Signed in, lesson slug not visible | Treat as missing: **`404`** (not 403) for `/worksheets/<slug>/*` and related lesson APIs | `404` |
| Invalid / hostile `next` after sign-in | Fall back to `/` | — |

Path gating runs on the decoded, normalized, resolved filesystem path (reject `%2e%2e`, encoded slashes, NUL, etc.).

Allowlisted page directories once gated: `/`, `/lessons/`, `/review/`, `/progress/`, `/admin/`, plus existing `/worksheets/`, `/shared/`, `/review/` assets, `/manifest.webmanifest`, `/icon.svg`. Directory URL `/dir/` serves `/dir/index.html`.

**Fetch scoping (REQ-014):** `fetch` is allowed only for `/api/` JSON calls. Lesson and vocab data continue to load via `<script>` tags (`window.WORKSHEET`, filtered catalogs). The no-fetch rule from ADR-0001 applies to learner lesson/vocab data, not to account/progress/admin APIs.

### 8. Lesson visibility

**Options considered:** see §C above — default `hidden` (**chosen**) vs default `everyone` (rejected). Seed-all-existing-as-`everyone` once (**chosen**) vs leave existing lessons unset (**rejected** — would hide the current library after upgrade).

| Mode | Who sees it |
|------|-------------|
| `hidden` | Admins only |
| `everyone` | Every signed-in active user |
| `selected` | Admins + users with a grant row |

| Rule | Decision |
|------|----------|
| Default when no visibility row / no assignment on create | **`hidden`** until assigned |
| MCP/AI assignment | May pass `visibility` on `upsert_lesson` or call `set_lesson_visibility`; tool reply states the resulting mode (and that default was applied when omitted) |
| First-run seed | Once, at first startup after migration: every lesson folder already on disk with no row is inserted as **`everyone`** (includes all three existing lessons, including `anal-sex-vocab`). Recorded in `meta` so it never re-runs. Later new lessons without a row take the default `hidden`. |
| Filters | Per-user filtering of `worksheets/catalog.js`, `worksheets/_review/vocab-index.js`, `worksheets/_audio/words/catalog.js`, and `shared/word-audio-catalog.js`. Fail closed to empty data if a source cannot be parsed. Admins receive unfiltered data. |
| Accepted residual | Hash-named word mp3 files require sign-in but are **not** filtered per lesson |

**Rationale:** Seed preserves today's library after upgrade; default `hidden` protects new/adult drafts.

### 9. Progress sync

**Options considered:** see §B above — server authoritative with batched idempotent signals (**chosen**) vs localStorage-authoritative optional sync (rejected). Fresh accounts with no legacy import/delete (**chosen**) vs migrate `hebrew-review:v1` (rejected by operator).

| Rule | Decision |
|------|----------|
| Authority | Server is authoritative for review schedule and lesson-round state |
| Signals | Browser posts batches `{ signals: [...] }` with client idempotent `id`s; server de-dupes on (`user_id`,`id`) |
| Scheduler | Server applies ADR-0001 `nextState` via `shared/player/review.js` using **server** `now` (client `at` is logged only) |
| Lesson rounds | Per-user JSON blob + monotonic `rev`; stale `rev` → conflict with stored copy |
| Pending queue | Each browser keeps a pending queue keyed per signed-in user for offline/transient failure |
| Legacy keys | While signed in: do **not** read and do **not** delete `hebrew-review:v1` or `hebrew-worksheet:<slug>` — fresh accounts |
| Visibility | Review state and signals limited to words/lessons the user can see |

**Rationale (`RSCH-003`):** spacing across days remains **High**; server time protects the schedule.

### 10. Metrics definitions

**Options considered:** (1) memory stage + direction accuracy for learners; trouble words + full answer history for admins only — **chosen** (operator); (2) streaks / points on the Progress page — rejected (`RSCH-003` gamification unproven — **Low**); (3) MCP-readable progress for tutoring — rejected (operator: MCP never reads learner progress).

| Metric | Definition |
|--------|------------|
| Memory stage (per word × direction) | **new** = no pass yet; **learning** = step 0–2; **long-term** = step ≥ 3 |
| Direction accuracy | passes ÷ (passes + misses) over the last **30 days**; exclude `not_yet`; hide percentage (`pct: null`) when attempts &lt; **10** |
| Trouble words (admin only) | Top **10** by misses in the last 30 days (tie-break: lapses) |
| Admin full view | Complete answer history from `practice_events`, paged newest first (limit 1–200, default 50), with word meaning and lesson title looked up from lesson data |
| Learner notice | Sign-in page and account menu state that admins can see all activity, including every answer |
| MCP | Never reads learner progress, metrics, events, or trouble words |

**Rationale (`RSCH-003`):** stage and direction accuracy reflect spacing (**High**) and separate recall directions (**Moderate**); streaks/points omitted (gamification unproven — **Low** / "don't").

### 11. Lesson editor and MCP assignment

**Options considered:** (1) admins in browser + authenticated MCP tools sharing `upsertLesson` — **chosen**; (2) MCP-only authoring with no browser editor — rejected (REQ-011 Should); (3) separate draft files beside live lessons — rejected (`hidden` visibility is the draft state).

| Rule | Decision |
|------|----------|
| Roles that create/assign | Browser **admins**; authenticated **MCP/AI**. No third "instructor" role |
| MCP tools | `list_users` (usernames + display names of active accounts only); `upsert_lesson` (optional `visibility`); `set_lesson_visibility` |
| MCP auth | Tools refuse unless `MCP_AUTH_TOKEN` is configured; requests must present it. `MCP_ALLOW_NO_AUTH` is removed (closes INIT-001 MED-1) |
| Audit actor | `mcp` for MCP-originated mutations |
| Editor scope | Forms for metadata, vocabulary, and activity path; validated JSON editor for other sections |
| Drafts | No separate draft entity — `hidden` visibility is the draft state |
| Publish path | Browser and MCP both go through `mcp/store.upsertLesson` (validator + rollback on failure) |
| Audio generation | Only when explicitly chosen, after a dry-run clip count (`audio-plan`); never on save |

### 12. Audio invariants

**Options considered:** (1) immutable paths/URLs + archive-before-overwrite + inventory/verify gate — **chosen** (REQ-012); (2) overwrite in place without archive — rejected (loses paid TTS); (3) content-addressed rename on every regenerate — rejected (breaks existing lesson URLs).

Numbered rules (cite from SPEC-002 and SPEC-007):

1. **No path or URL changes** — published clip paths and public URLs remain stable across regenerations and lesson edits.
2. **Archive before overwrite** — before replacing any existing recording bytes, copy the prior bytes to `audio/_archive/` (or the lesson-local archive convention established by SPEC-002) with a verifying SHA-256 match; identical bytes must not create duplicate archives.
3. **Inventory / verify gate** — every spec that touches `worksheets/`, `server/audio.js`, `mcp/store.js`, or the lesson editor runs `audio:inventory` before work and `audio:verify` after; verify failure blocks completion.

**Rationale:** REQ-012; protects paid TTS output and learner-facing URLs.

### 13. Site map

**Options considered:** (1) Today `/`, Lessons `/lessons/`, Review `/review/`, Progress `/progress/`, Admin `/admin/`, Sign-in `/login/` with one shared top bar — **chosen**; (2) keep the single home-card layout from INIT-001 — rejected (REQ-006); (3) deep-link only via per-lesson `index.html` chrome — rejected (no edits to existing lesson `index.html`).

| Path | Purpose |
|------|---------|
| `/` (Today) | What to do now |
| `/lessons/` | Lesson library (visibility-filtered) |
| `/review/` | Cross-lesson review (ADR-0001 model, server-backed) |
| `/progress/` | Memory stages and direction accuracy for the signed-in learner |
| `/admin/` | Admin console (users, activity, lessons, editor) — admin role only |
| `/login/` | Sign-in (public) |

**Shared top bar:** Today, Lessons, Review, Progress, Admin (admins only), account menu. Bilingual labels (Hebrew with English). Current page uses `aria-current="page"`.

**Lesson player:** entered through `shared/player/core.js` (existing player), never by rewriting lesson `index.html` files. Home/Review links come from the shared top bar injected or shared chrome — existing lesson `index.html` files stay unmodified (REQ-014 / ADR-0001 constraint 3).

### 14. What is no longer supported

**Options considered:** (1) drop `file://` and offline-beyond-pending-queue once accounts land — **chosen**; (2) dual-mode static+account builds — rejected (doubles gating surface); (3) service-worker offline full app — out of scope.

| Capability | Status |
|------------|--------|
| `file://` opening of the app | **Not supported** — session cookies, gated static files, and `/api/` require the HTTP server |
| Offline use beyond the pending queue | **Not supported** — durable progress and catalogs require a signed-in server session |
| Public / anonymous browsing of lessons | **Not supported** |
| Import of legacy `localStorage` progress into accounts | **Not supported** (fresh start) |
| Third "instructor" role | **Not supported** |

### Rationale summary (research)

| Topic | Decision link | `RSCH-003` |
|-------|---------------|------------|
| Server-side spacing / ladder reuse | §9 | Spaced recall across days (**High**) |
| Per-direction metrics | §10 | Each direction builds its own knowledge (**Moderate**) |
| No streaks / points on Progress | §10 | Gamification / points unproven (**Low**) |
| Admin trouble words + full history | §10 | Operator product decision; science does not require gamified learner dashboards |

### Decision matrix (storage engine)

| Criteria | Weight | A1 `node:sqlite` | A2 `better-sqlite3` | A3 JSON files |
|----------|--------|------------------|---------------------|---------------|
| Ops simplicity / no native build | 5 | 5 | 2 | 4 |
| Transactional signal batches | 5 | 5 | 5 | 1 |
| Metrics / history indexes | 4 | 5 | 5 | 2 |
| Maturity / risk | 3 | 3 | 5 | 3 |
| **Weighted total** | | **64** | **55** | **33** |

---

## Consequences

### Positive

- Downstream specs share one auth, session, CSRF, visibility, sync, metrics, and site-map contract.
- ADR-0001 scheduler stays single-sourced; accounts do not fork the ladder.
- Adult and draft lessons can be restricted without a third role.
- MCP can assign lessons safely only when token-authenticated, without reading progress.

### Negative / trade-offs

- `file://` and true offline practice end; learners need the server.
- Hash-named word mp3 residual: sign-in required, but not per-lesson filtered.
- Admin full answer history is sensitive; mitigated by disclosure to learners and audit of history reads.
- `node:sqlite` experimental status → documented `better-sqlite3` fallback.
- In-memory login throttle assumes one replica.

### Carried constraints (ADR-0001 REQ-009 — restated)

| # | Constraint | Status under ADR-0002 |
|---|------------|------------------------|
| 1 | Progress only in `localStorage` — no accounts, no server-side learner profiles | **Superseded** by this ADR (server-side per-user progress + accounts) |
| 2 | No `fetch` for lesson / vocab data — globals load via script tags only | **Still binding** (scoped: progress/account/admin JSON APIs may use `fetch`; lesson/vocab data must not) |
| 3 | No edits to lesson `index.html` files required for existing lessons to keep working | **Still binding** |
| 4 | No root/binyan speed drills added by this initiative | **Still binding** |
| 5 | No forced vowel fading — nikud on by default with the existing learner toggle | **Still binding** |
| 6 | Bilingual UI labels on new surfaces | **Still binding** |
| 7 | No learner name, transcript text, or secrets in lesson files, glosses, the vocab index, or ADRs | **Still binding** |

Also still binding: ElevenLabs API key remains in server env only (never in image layers, responses, or logs).

### Follow-ups (out of scope here)

- Content-Security-Policy once inline scripts are reduced.
- OAuth/OIDC sign-in (schema hooks only here).
- Self-service email password reset.
- Import/export of legacy browser progress.
- Multi-replica deployment (would need shared throttle and SQLite alternative or external lock).

### Rollback

Delete `docs/decisions/ADR-0002-accounts-server-progress-site-map.md` and revert the ADR-0001 metadata / changelog lines. No code lands in this spec.

---

## Implementation plan (downstream)

| Spec | Consumes from this ADR |
|------|------------------------|
| SPEC-003 | Storage engine, `DATA_DIR` rule, schema outline |
| SPEC-004 | Passwords, sessions, throttle numbers, roles, last-admin, audit |
| SPEC-005 | Sync rules, metrics definitions, answer history paging |
| SPEC-006 | Visibility modes, seed, filters, residual |
| SPEC-007 | Editor/MCP assignment, audio dry-run, publish path |
| SPEC-002 | Audio invariants 1–3 |
| SPEC-008 | CSRF/Origin, route gating, public path table, cookies |
| SPEC-009 | Learner review / lesson progress / metrics routes |
| SPEC-010 | Admin users / progress / events / lessons / audio + throttle |
| SPEC-011 | `DATA_DIR`, `COOKIE_SECURE`, `PUBLIC_ORIGIN`, MCP token required |
| SPEC-012–017 | Site map, top bar, pages, admin UI |
| SPEC-018 | Operator guide reflecting these decisions |

---

## Security considerations

### Authentication & authorization

Two roles only; every admin route and `/admin/*` page checked on the server. Learner APIs never take a target user id. MCP tools are token-gated and cannot read progress.

### Data protection

Password hashes and raw session tokens never leave the server. `DATA_DIR` is never under a served root. Per-user and API responses use `Cache-Control: no-store`. Filtered catalogs omit restricted lesson metadata.

### Threat highlights

| Threat | Mitigation |
|--------|------------|
| Credential stuffing | Throttle + dummy hash timing |
| Session theft from XSS | HttpOnly cookie; CSP follow-up still owed |
| CSRF | JSON Content-Type + Origin check + SameSite=Lax |
| Lesson IDOR | 404 when not visible; filters on shared scripts |
| Admin history peeking | Explicit product choice + learner notice + audit on events read |
| DB exposure via static server | Startup refuse if `DATA_DIR` under a served root |

### Residual risks (accepted)

1. **Hash-named word mp3 residual** — authenticated learners may fetch a clip that originated in a lesson they cannot see if they know or guess the hash URL.
2. **Admin full answer history** — highly revealing of study content; accepted with disclosure and audit.
3. **No CSP yet** — XSS would be high impact on session cookies; tracked as follow-up.
4. **Experimental `node:sqlite`** — mitigated by wrapper + documented fallback.
5. **Long 30-day sessions** — no idle timeout; mitigated by revocation on password/role/disable and HttpOnly.

---

## References

- `sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md`
- [ADR-0001: Learner review model](ADR-0001-learner-review-model.md)
- Initiative REQ-001–016 — `sdd/initiatives/INIT-002-multi-user-progress/INIT-002-multi-user-progress-initiative.md`
- OWASP Password Storage Cheat Sheet — scrypt parameter guidance (N/r/p class)

## Changelog

| Date | Change |
|------|--------|
| 2026-10-08 | Initial Proposed record — `INIT-002/SPEC-001`; operator decisions aud-1–aud-7 recorded as chosen options |
| 2026-10-08 | Superseded in part by ADR-0004. Metadata note and this changelog row only; decision sentences in this file are left as written. |
| 2026-10-09 | ADR-0009 (`INIT-006/SPEC-001`, Proposed) would change the §10 stage threshold under `SRS_SCHEDULER=fsrs` (`longTerm` is stability of 14 days or more) and replace the MCP learner-progress wording again (ADR-0009 §1 profile in place of the ADR-0004 §1 snapshot). Metadata note and this row only; decision sentences are left as written. Takes effect only when ADR-0009 is Accepted. |
| 2026-10-09 | ADR-0009 Accepted by the operator. The change in the row above is in force. Metadata note and this row only. |
