# SPEC-004 completion summary

**Spec:** SPEC-004 · Port Admin; legacy lessons; route and 401 tests  
**Initiative:** INIT-005  
**Completed:** 2026-10-12  
**Status:** in_review  
**Provenance:** `INIT-005/SPEC-004`

## What shipped

Admin is a view, and legacy lessons follow the SPEC-002 decision (full page). This pass audited both, then closed the test gaps.

- **Admin (ac-1).** Admin still refuses non-admins with 403 before the shell is sent (`server/web.js`). The nav hides the link for non-admins. A learner who reaches `/admin/` in the client gets "No access". Existing admin tests and the navigation smoke pass.
- **Legacy lessons (ac-2).** `/worksheets/<slug>/` is a full page, not in the route table. The player mounts the shared header itself. A test confirms the page is not the shell.
- **401 (ac-3).** `HebrewApi.request` redirects to `/login/?next=<current pathname through safeNextPath>` on any 401 unless the call passes `redirectOn401:false`. No screen module calls `fetch()` or `XMLHttpRequest` directly, so the redirect cannot be bypassed (tested by scan). Opt-outs are sign-in and sign-out, sync, and best-effort enrichment (Today mix, per-lesson progress, library and lexicon lookups, plan catalog).
- **Tests.** `test/api-redirect.test.js` and `test/router-a11y.test.js`.

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass | Full suite green; `chrome-navigation-smoke` opens Admin in place and returns with Back. |
| ac-2 | Pass | `test/router-a11y.test.js`: lesson page is 200 and not the shell. |
| ac-3 | Pass, with a limit | `test/api-redirect.test.js`: default 401 redirect for GET, POST, DELETE and PUT; every route path yields a `next` the server accepts; hostile pathname gives `next=/`; opt-outs, non-401 statuses and non-`/api/` URLs do nothing. Limit: if a session expires while only opted-out calls run, the learner is sent to sign-in on the next reload or the next screen with a default call. I did not widen the opt-outs. |
| ac-4 | Pass | `test/router-a11y.test.js`: direct URL, unknown route (client and server), Back and Forward, and that middle-click, modifier-click, `target=_blank`, `download`, other origins and hash-only links are not intercepted. |

## Security acceptance

- Admin routes remain server-gated: tested (learner 403, admin 200).
- `next` uses `safeNextPath`: 20 hostile and malformed values map to `/`, and the client rule agrees with the server rule on the shared cases.

## Tests

- `npm test` (`SRS_SCHEDULER=ladder`): 1010 pass, 0 fail.
- `test/api-redirect.test.js`: 14 pass. `test/router-a11y.test.js`: 26 pass.

## Follow-ups

- Optional: keep the query string in `next` (today only the pathname is kept, so a 401 on `/words/?binyan=hifil` returns to `/words/`).