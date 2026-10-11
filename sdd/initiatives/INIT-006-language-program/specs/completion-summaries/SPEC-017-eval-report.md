# SPEC-017 Eval Report — Progress AI-profile panel and admin switch

**Traceability:** INIT-006/SPEC-017
**Verifier:** implementing session (not a fresh subagent)
**Verdict:** evidenced PASS on mechanized checks · independent confirmation not recorded here
**Date:** 2026-10-10

This report records commands run while implementing the spec. It is not an independent all-PASS. The completion summary keeps the spec `in_review` for that reason.

## Per AC

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS (in-session) | `node test/chrome-ai-profile-smoke.js` exit 0. The hidden wire node's `textContent` equals the `GET /api/me/profile` body. The visible raw block equals `JSON.stringify(JSON.parse(body), null, 2)`. Parsing both yields the same JSON. |
| ac-2 | PASS (in-session) | The same smoke. Progress `[data-ai-disclosure]`, the open account menu, the sign-in page, and the admin confirm dialog all contain the ADR-0009 English sentence. The account menu and the sign-in page still contain "Admins can see all your activity, including every answer." `test/admin-users-view.test.js` checks the same sentence is in `shared/app-shell.js`, `shared/login.js`, `login/index.html`, and the confirm dialog HTML. |
| ac-3 | PASS (in-session) | The smoke. Progress has no `[data-ai-planning-toggle]`. A learner `PATCH /api/admin/users/:id/ai-planning` returns 403. Opening `/admin/` as that learner returns the plain "Forbidden" page and no toggle. |
| ac-4 | PASS (in-session) | Unit test: a 403 leaves `ai_planning` false, restores the row, and shows "Forbidden". A 200 with `ai_planning: true` is what the row shows. A 200 that omits the boolean does not flip the flag. The smoke: while the confirm dialog is open, `aria-pressed` stays `"false"`; Cancel leaves it false and `GET /api/admin/users` still reports false; Confirm, after the response, sets `aria-pressed` to `"true"` and the list reports true. |

## Null strength

The smoke builds a profile whose `he_en.strength` is `null` and whose word id is `<img src=x onerror=alert(1)>`. The rendered node has `data-strength="null"`, its text does not contain the word "null", and the host has no `img` element. The wire text of that fixture equals the JSON string that was passed in.

## Tests run

- `node --test test/admin-users-view.test.js test/progress-view.test.js test/app-shell.test.js test/design-tokens.test.js` — pass, exit 0
- `node test/chrome-ai-profile-smoke.js` — exit 0 (`chrome-ai-profile-smoke: PASS`)
- `node test/chrome-progress-smoke.js` — exit 0 (existing progress client smoke)

- `npm test` — 840 pass, 0 fail, exit 0

## Remediation

The first Chrome run treated a cached or still-authenticated learner `GET /admin/` as a failure to find the in-app "admins only" sentence. The server already answers that request with plain `Forbidden` and no toggle. The smoke now accepts that page, disables the HTTP cache, and sends the admin cookie on the later navigation. No acceptance criterion was weakened.

## Final

Mechanized checks pass. Independent verifier: not run in this session.
