# SPEC-015 Eval Report — Today daily mix

**Traceability:** INIT-006/SPEC-015
**Verifier:** implementing session (not a fresh subagent)
**Verdict:** evidenced PASS on mechanized checks · independent confirmation not recorded here
**Date:** 2026-10-10

This report records commands run while implementing the spec. It is not an independent all-PASS. The completion summary keeps the spec `in_review` for that reason.

## Per AC

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS (in-session) | `node test/chrome-today-mix-smoke.js` exit 0. Output: `{"ok":true,"review":"1","weak":"2","path":"/","oldHref":"/review/"}`. The smoke throws if the answer node exists before reveal, if the meaning is already in the session, if Back does not restore the list, or if the path leaves `/`. |
| ac-2 | PASS (in-session) | Same command. The seeded mix JSON has no `drill` and no `reading`, and the page has neither section node. `node --test test/today-mix.test.js` — the order test drops drill without `HebrewBlockConjugation` and drops a section the payload does not include. |
| ac-3 | PASS (in-session) | `node test/chrome-today-lessons-smoke.js` exit 0. Due `3`, start href `/review/`, continue `my-morning`. Non-`/api/` fetch list empty. |

## Tests run

- `node --test test/today-mix.test.js` — 6 pass, exit 0
- `node test/chrome-today-mix-smoke.js` — exit 0
- `node test/chrome-today-lessons-smoke.js` — exit 0
- `npm test` — 840 pass, 0 fail, exit 0

## Remediation

The first unit assertion rejected a drill label because the escaped text still contained the characters `alert(1)`. The check now requires the label to be escaped (`&lt;img …&gt;`) and requires the card prompt and gloss to be absent from the list HTML. Re-ran `test/today-mix.test.js`; 6 pass.

No acceptance criterion was weakened.

## Final

Mechanized checks pass. Independent verifier: not run in this session.
