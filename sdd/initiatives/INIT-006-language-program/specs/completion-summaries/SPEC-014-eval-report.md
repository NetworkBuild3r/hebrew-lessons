# SPEC-014 Eval Report — Explainer, conjugation-table, and personal-question renderers

**Traceability:** INIT-006/SPEC-014
**Verifier:** implementing session (not a fresh subagent)
**Verdict:** evidenced PASS on mechanized checks · independent confirmation not recorded here
**Date:** 2026-10-10

This report records commands run while implementing the spec. It is not an independent all-PASS. The completion summary keeps the spec `in_review` for that reason.

## Per AC

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS (in-session) | `node --test test/render-explainer.test.js test/render-conjugation.test.js` exit 0. Explainer hostile test requires 12 or more escaped copies of `<script>alert(1)</script>` and rejects `<img`, `<script`, and `onerror=`. Conjugation hostile test rejects the raw payload and still shows "not yet checked". |
| ac-2 | PASS (in-session) | `node test/chrome-verb-workshop-smoke.js` exit 0. Server `practice_events`: one `pass` on `חָשַׁבְתִּי` and two `miss` rows on `חָשַׁבְתָּ`, all `kind = 'form'`. `review_items` count unchanged. Unit tests cover `accept[]`, vowel stripping, and the one-retry decision. |
| ac-3 | PASS (in-session) | `test/render-conjugation.test.js` asserts SM, SF, PM, PF in header order and the nine past person labels with nine rows. |
| ac-4 | PASS (in-session) | The smoke records CDP `Network.requestWillBeSent` after Done. One PUT to lesson progress. Body boolean `you: true`. `pq-secret-typed` is absent from the URL, the body, and `lesson_progress.state_json`, and still present in the input. |

## Tests run

- `node --test test/render-explainer.test.js test/render-conjugation.test.js test/plan-render.test.js` — 29 pass, exit 0
- `node test/chrome-verb-workshop-smoke.js` — exit 0
- `npm test` — 809 pass, 0 fail, exit 0

## Remediation

None. The first run of the new unit tests and the Chrome smoke passed.

No acceptance criterion was weakened.

## Final

Mechanized checks pass. Independent verifier: not run in this session.
