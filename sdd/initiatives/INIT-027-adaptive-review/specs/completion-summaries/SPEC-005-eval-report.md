# SPEC-005 Eval Report

**Verifier:** independent generalPurpose read, model grok-4.7-high. Not the implementer.
**Result:** PASS (ac-1 through ac-6).

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS | `newAllowance` is 2, 5, 5, 8 at 0.79, 0.80, 0.90, 0.91, and 5 with no history |
| ac-2 | PASS | At 92% with no due new cards, one build seeds the first 8 eligible deck words and queues 8 intro visits |
| ac-3 | PASS | Below 85%, `seedNewFromDeck` returns before any insert |
| ac-4 | PASS | Existing cards are skipped; no read card before the gate; no spell; no taught, attempt, exposure, or review_log rows |
| ac-5 | PASS | Coach mix target reports max 8 and the tier fields, and no `maxNewShare` |
| ac-6 | PASS | No review-session code or doc still states the 0.25 share |

Earlier behavior still present: `DrillSteps` 3, no not-due known padding, `Recorder.Implied`, `letters.Confusable`, and `implied = 0` on `RecentReviewRatings`.
