# SPEC-014 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/lesson-status.test.js` (6 pass) and `node test/chrome-today-lessons-smoke.js` (ok: true). Read `HebrewUtils.lessonStatus` in `shared/player/utils.js` (one 24-hour boundary).
**Tier:** full. Pre-flight: gate_actions PASS · security_review=light PASS · domain=frontend FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | Today shows 3 due. Start goes to `/review/`. Continue opens `my-morning`. |
| ac-2 | PASS | A new account sees Welcome and the next lesson, and no Review card. |
| ac-3 | PASS | Lessons lists the two visible lessons with chips `in_progress` and `new`. The advanced filter is empty. Basic shows one lesson. |
| ac-4 | PASS | `anal-sex-vocab` is absent from Today and Lessons for the user without a grant. |
| ac-5 | PASS | Labels are Hebrew with English. Nikud is preserved. All 15 fetches start with `/api/`. |
| ac-6 | PASS | One helper covers new, in progress, due again at 24 hours, and done for now. |

No remediation loop.
