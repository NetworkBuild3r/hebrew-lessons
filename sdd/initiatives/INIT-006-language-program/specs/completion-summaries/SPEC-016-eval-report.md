# SPEC-016 Eval Report — Verse-reading, letter-intro, and the Guides shelf

**Traceability:** INIT-006/SPEC-016
**Verifier:** implementing session (not a fresh subagent)
**Verdict:** in-session all-PASS · independent verification not run · spec status `in_review`

A fresh read-only verifier was not dispatched. This session is already a subagent, and nesting another one was skipped. The verdicts below are the author's own test runs. They are evidence, not an independent grade.

## Per-AC

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS (in-session) | `node --test test/render-verse-letter.test.js` exit 0. Hostile verse and letter HTML contain the escaped payload, not a raw `<script` or `<img`. Hebrew uses `dir="rtl"`. |
| ac-2 | PASS (in-session) | `node test/chrome-guides-smoke.js` printed `chrome-guides-smoke: PASS`. Ready signal `guides`. Explainer section `roots` shows "Three letters". Hidden title absent. |
| ac-3 | PASS (in-session) | Fixture order assertions in `test/render-verse-letter.test.js` passed for `verse.render` and `renderScreens`. |
| ac-4 | PASS (in-session) | `test/spa-shell.test.js` passed inside `npm test` (833 pass, 0 fail). `node test/chrome-navigation-smoke.js` printed `chrome-navigation-smoke: PASS` with a `guides` screen. |

## Tests run

- `node --test test/render-verse-letter.test.js test/render-explainer.test.js test/plan-render.test.js test/spa-shell.test.js test/app-shell.test.js` — 59 pass, exit 0
- `node test/chrome-guides-smoke.js` — exit 0
- `node test/chrome-navigation-smoke.js` — exit 0
- `npm test` — 833 pass, 0 fail, exit 0

## Remediation loops

None. The first hostile-string assertions failed because an escaped `<img ... onerror=...>` still contains the letters `onerror=`. The assertions were changed to require the escaped handler and reject a raw tag. Renderer code was not weakened.

## Final

Independent all-PASS is not claimed. The orchestrator can mark the spec `complete` after a fresh verifier agrees.
