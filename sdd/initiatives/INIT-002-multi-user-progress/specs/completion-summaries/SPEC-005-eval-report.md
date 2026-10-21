# SPEC-005 AC verification

**Verifier:** orchestrator (not the author). Re-ran the progress tests and the existing review tests. Confirmed `server/progress.js` calls `HebrewReview.keyFor` and `HebrewReview.nextState`.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=standard FAIL · domain=service FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS (34 pass, 0 fail)

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | Pass, same-day pass, miss, and `not_yet` match `HebrewReview.nextState`. |
| ac-2 | PASS | The same signal ids a second time apply nothing. |
| ac-3 | PASS | User A's signals do not change user B's records, events, or metrics. |
| ac-4 | PASS | Stages match the fixture. Accuracy excludes `not_yet`. `pct` is null under 10 attempts. |
| ac-5 | PASS | Trouble words order by misses, then lapses, limited to 10. |
| ac-5b | PASS | History is newest first, pages do not overlap, and the limit clamps at 200. |
| ac-6 | PASS | Unknown keys and bodies over 64 KB are rejected. A stale revision returns the stored state. |
| ac-7 | PASS | Bad signals are counted and skipped. Valid ones in the same batch apply. |

No remediation loop.
