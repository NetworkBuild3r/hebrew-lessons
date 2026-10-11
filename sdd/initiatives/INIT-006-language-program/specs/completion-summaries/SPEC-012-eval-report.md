# SPEC-012 Eval Report — Four-grade flashcards on Review

**Traceability:** INIT-006/SPEC-012
**Verifier:** implementing session (not a fresh subagent)
**Verdict:** evidenced PASS on mechanized checks · independent confirmation not recorded here
**Date:** 2026-10-10

This report records commands run while implementing the spec. It is not an independent all-PASS. The completion summary keeps the spec `in_review` for that reason.

## Per AC

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS (in-session) | `node test/chrome-flashcards-smoke.js` exit 0. Output: `last_grade` 3 on `קָם` / `en_he`, one Good among the seeded cards. |
| ac-2 | PASS (in-session) | Same command. `signals: [1, 2, 3, 4]`. Interval `data-days` on the revealed card matched the due-deck payload (the smoke throws if they differ). |
| ac-3 | PASS (in-session) | Same command checks stage text and `data-speak`, `data-vowels`, `data-audio`, `data-key` before Space. `node test/chrome-review-smoke.js` exit 0: answer element absent before the click; afterwards `innerHTML === textContent` and `children.length === 0`. |
| ac-4 | PASS (in-session) | `node --test test/flashcard-answer.test.js` — 5 passed, exit 0. |
| ac-5 | PASS (in-session) | `node test/chrome-navigation-smoke.js` exit 0. Review screen `path: /review/`, ready signal observed (script prints `chrome-navigation-smoke: PASS`). |

## Tests run

- `node --test test/flashcard-answer.test.js test/review-page.test.js` — 17 pass, exit 0
- `npm test` — 787 pass, 0 fail, exit 0
- `node test/chrome-flashcards-smoke.js` — exit 0
- `node test/chrome-review-smoke.js` — exit 0
- `node test/chrome-navigation-smoke.js` — exit 0

## Remediation

One smoke false-positive: `innerHTML` contained `my-morning`, which includes the gloss substring `morning`. The check was narrowed to visible text and the attributes that would hold the answer. `data-key` was removed from the card so an English-to-Hebrew answer is not sitting in an attribute before reveal. Re-ran `chrome-flashcards-smoke.js` and `chrome-review-smoke.js`; both exit 0.

No acceptance criterion was weakened.

## Final

Mechanized checks pass. Independent verifier: not run in this session.
