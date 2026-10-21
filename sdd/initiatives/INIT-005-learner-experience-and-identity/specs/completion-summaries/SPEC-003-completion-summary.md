# SPEC-003 completion summary

**Spec:** SPEC-003 · Port Review and Progress; route a11y  
**Initiative:** INIT-005  
**Completed:** 2026-10-12  
**Status:** in_review  
**Provenance:** `INIT-005/SPEC-003`

## What shipped

Review and Progress were already views with `mount`/`unmount`, and the router already did title, focus, announce and scroll. This pass audited that against the spec, added the tests the spec asks for, and fixed two small things.

- **Verified, not changed.** Listeners: Review's `document` keydown uses `ctx.signal`; Admin's `hashchange` uses `ctx.signal`; the rest are on elements inside the view root and go away with it. No view starts a timer or interval.
- **Route transition (ac-3).** `#app-main.route-enter` plays a 160 ms fade with a 6 px rise, CSS only. The block at the end of `shared/worksheet.css` (`/* === INIT-005/SPEC-009 route transition + sound toggle === */`) sets `animation: none; transform: none` under `prefers-reduced-motion: reduce`. The earlier fade-only rule in the SPA helpers block stays; the later keyframes override it.
- **Router fix.** Under reduced motion `swap()` now clears a leftover `route-enter` class.
- **Tests.** `test/router-a11y.test.js`.

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass | Full suite green. Chrome smokes pass: navigation, review, progress, sign-in, words, today-lessons. |
| ac-2 | Pass | Title, focus on the h1, polite live region text (cleared first, then set), scroll reset to 0, Back restores the saved `scrollY` immediately and again after mount, Forward, and a Back to a non-app path forcing a real load: `test/router-a11y.test.js`. Same behaviour in a real browser: `chrome-navigation-smoke`. |
| ac-3 | Pass | Unit test reads the CSS: duration at most 200 ms, no `infinite`, reduce override with `animation: none`. The router adds no class under reduce. Chrome smoke with emulated reduced motion sees `animation-name: none` after a route change, and `hebrew-route-enter` without it. |

## Tests

- `npm test` (`SRS_SCHEDULER=ladder`): 1010 pass, 0 fail.
- `test/router-a11y.test.js`: 26 pass.
- `node test/chrome-navigation-smoke.js`: PASS.

## Known failure outside this spec

`node test/chrome-production-recall-smoke.js` fails with "Hebrew was visible before reveal" in the lesson player words step. It fails the same way on a clean checkout of HEAD, so it predates this work and has nothing to do with routing. It needs its own fix.