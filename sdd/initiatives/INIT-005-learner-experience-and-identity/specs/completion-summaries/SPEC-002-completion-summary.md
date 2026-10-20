# SPEC-002 completion summary

**Spec:** SPEC-002 · Shell and router spike  
**Initiative:** INIT-005  
**Completed:** 2026-10-12  
**Status:** in_review. All criteria pass except ac-5 (operator click-through), which is open.  
**Provenance:** `INIT-005/SPEC-002`

## What shipped

The shell, router, route table and five views already existed (see `INIT-005-shell-spike-decision.md`). This pass audited them against the spec and closed one gap.

- **Gap closed: unknown path (ac-2, REQ-006).** Before, a signed-in user who typed `/nope/` got a plain-text `Not found` and no shell.
  - `server/spa.js` adds `isUnknownAppPath(rel)`. It is true for a path that is not an app screen, is not under `/shared/`, `/worksheets/`, `/login/` or `/api/`, and has no file extension.
  - `server/web.js` answers such a path (for a browser that wants HTML) with the shell and HTTP 404, `no-store`. Real file misses, `/worksheets/` misses and API misses keep the plain 404.
  - `shared/router.js` adds `showNotFound()`. `start()` calls it when the URL matches no route. It shows a heading and a "Back to Today" link inside the shell, sets title and announcement, moves focus to the h1, clears `aria-current`, and sets `data-route-ready="not-found"`.
  - Signed-out users are unchanged: any app or unknown path redirects to `/login/?next=<safe path>`.
- **Small router fix.** Under `prefers-reduced-motion`, `swap()` now removes a leftover `route-enter` class instead of leaving it (CSS already forced `animation: none`).

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass | `test/chrome-navigation-smoke.js`: no document reload across every nav click, one h1, `aria-current` moves. `test/spa-shell.test.js`: shell paints from the embedded user with no loading placeholder. |
| ac-2 | Pass | Direct URL: `test/spa-shell.test.js`, `test/router-a11y.test.js`. Unknown path: server tests in `test/router-a11y.test.js`; router test; Chrome smoke loads `/nope/`. |
| ac-3 | Pass | Full suite green. |
| ac-4 | Pass, with a caveat | The decision note records the fallback (full page). It rests on ADR-0007's pre-approved fallback and the global-collision risk. `my-morning` was not mounted as an experiment. |
| ac-5 | **Open** | The decision note still says "pending operator click-through". Not ticked on the operator's behalf. |

## Understood, deliverables, security

All ticked. und-3 note: there is no feature flag. "Easy revert" means reverting the change. Embedded user fields match `/api/me` (`id`, `username`, `display_name`, `role`, `must_change_password`), with `<` escaped and no session token.

## Tests

- `npm test` (`SRS_SCHEDULER=ladder`): 1010 pass, 0 fail.
- New in `test/router-a11y.test.js` for this spec: `isUnknownAppPath` table, signed-in 404 shell (GET and HEAD, with and without trailing slash), signed-out redirect, real-file 404s unchanged, `/worksheets/` and `/admin/` gating unchanged, every route path still 200, router not-found behaviour, direct URL.
- `node test/chrome-navigation-smoke.js`: PASS, now also covers the not-found view and its link.

## Not closed

- ac-5: the operator has not clicked through or said "go".