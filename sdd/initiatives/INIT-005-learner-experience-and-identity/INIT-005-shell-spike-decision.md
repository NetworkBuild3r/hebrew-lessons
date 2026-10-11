# INIT-005 shell decision

**Specs:** SPEC-002 (spike), SPEC-003 (Review + Progress), SPEC-004 (Admin + legacy routes)  
**Date:** 2026-10-09  
**Status:** built and tested; pending operator click-through

## What was built

The operator rejected the first spike, which covered Today and Lessons only, because Review, Progress and Admin still reloaded. All five signed-in screens now share one document. SPEC-002, SPEC-003 and SPEC-004 landed together.

- **One page.** `shared/app.html` is served by `server/spa.js` and `server/web.js` for every path in `APP_PATHS`: `/`, `/lessons/`, `/review/`, `/progress/`, `/admin/`, plus their `index.html` forms. The five per-screen HTML files are deleted. Admin is still refused server-side (403) for non-admins before the shell is sent.
- **Route table.** `shared/app-routes.js` is the only place a screen is declared. Each entry gives the heading, body class, view root id, extra scripts and module global.
- **Layout template.** `shared/layout.js` renders the heading, actions slot and view root for every screen. Views no longer hand-write headings.
- **Router.** `shared/router.js` is hand-built on the History API. Its jobs:
  - Intercept left-clicks on app links. Leave hash-only changes alone, so admin tabs keep working.
  - Load each screen's scripts once, then prefetch the other screens' scripts when idle.
  - Give each view an `AbortSignal` that drops its listeners when the screen closes.
  - Ask `canLeave()` before leaving, including on browser Back, which protects unsaved lesson edits.
  - Update the title, `aria-current`, focus on the h1, the screen-reader announcement and scroll position.
  - Set `#app-main[data-route-ready]` once a view has mounted.
- **Page modules.** Today, Lessons, Review, Progress and Admin expose `mount(el, ctx)` and `unmount()`, and none of them auto-run. Review puts its vowels toggle in the heading's actions slot. Admin also exposes `canLeave()`, which delegates to the lesson editor.
- **Boot.** `shared/app-boot.js` paints the shell once from the user the server embedded (`__HEBREW_ME__`), with no `/api/me` round trip and no placeholder. It calls `HebrewSync.init()` once and hands the result to every view as `ctx.ready`.
- **Caching.** Static files send a weak ETag and `Last-Modified`, and return 304 when unchanged. The shell itself is sent `no-store` because it embeds the user.
- **Rollback.** There is no flag. Rolling back means reverting the change.

## Legacy lessons

**Decision: fallback (full page).** `/worksheets/<slug>/` stays a full document and is not in the route table. A lesson link loads the worksheet as its own page, with the existing per-page shell mount.

Reason: the three published lessons set `window.WORKSHEET` via script tag. Mounting two of them in one document without a proven reset risks colliding globals. ADR-0007 Decision 7 pre-approved this fallback. INIT-004's API-served plans remove the problem for new lessons.

## Tests

- `test/spa-shell.test.js` covers:
  - The server's app paths match the client route table.
  - Every route module exists, exports `mount` and `unmount`, and does not auto-run.
  - The router's lifecycle: signal abort, `canLeave` on link and Back, admin-only, hash-only popstate, click rules and the ready signal.
  - Shell nav painting.
  - ETag/304.
  - Every screen URL returns the shell.
- `test/chrome-navigation-smoke.js` is new. It signs in as admin and clicks Today, Lessons, Review, Progress, Admin, Today, then Back. At each step it asserts:
  - The document never reloads.
  - The path is right and `aria-current` has moved.
  - There is exactly one heading, and it has focus.
- The Review, Production-recall and Sign-in smokes now wait for `data-route-ready`. They seed signed-in state through the server or `HebrewSync`, not `localStorage`, because the server is authoritative when signed in.

## Operator gate

Please try at `http://127.0.0.1:8080`, signed in as an admin:

1. Click through **היום / Today**, **שיעורים / Lessons**, **חזרה / Review**, **התקדמות / Progress** and **ניהול / Admin**. The top bar should stay put, and only the content below should change.
2. Press the browser Back button a few times. Each screen should come back in place, at the scroll position you left.
3. Refresh on any screen. The same screen should come back.
4. In Admin, switch between the Users and Lessons tabs, then open a lesson in the editor and change something without saving. Click Today: you should be asked before leaving.
5. Open a lesson from Lessons. It loads as its own page and still plays.

Reply **go** to continue with SPEC-005 (research) and SPEC-006 (identity), or name what to change.
