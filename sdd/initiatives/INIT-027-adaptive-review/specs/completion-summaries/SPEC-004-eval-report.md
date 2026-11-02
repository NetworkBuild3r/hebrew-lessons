# SPEC-004 Eval Report

**Verifier:** independent generalPurpose read, model grok-4.7-high. Not the implementer.
**Result:** PASS (ac-1 through ac-8).

| AC | Verdict | Evidence |
| --- | --- | --- |
| ac-1 | PASS | Not-due known cards are skipped; 5 due plus 40 later queue only the 5 |
| ac-2 | PASS | Every due reviewed card is struggle or known; none are dropped |
| ac-3 | PASS | Known read moves up to recall when the recall card is clean and not queued |
| ac-4 | PASS | Breaking any one move-up condition keeps the card's own skill |
| ac-5 | PASS | Implied Good writes `review_log.implied = 1` and no attempts row |
| ac-6 | PASS | Wrong or Hard writes nothing for the own card; a wrong answer requeues on the own skill |
| ac-7 | PASS | `Implied` refuses a non-Good rating and another learner's card |
| ac-8 | PASS | Tally counts answered, right, moved up, cleared, slow, and new met |

The three orchestrator adjustments (learner id on `Implied`, move-up counted only after success, clears counted from the write) were judged inside the spec.
