# SPEC-009 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/api-progress.test.js test/api-auth.test.js`. Read `server/api/progress-routes.js` and the `:param` matcher in `server/api/router.js`.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=standard FAIL · domain=api FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS (14 pass, 0 fail)

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | Review state, lesson progress, and metrics use `ctx.user.id`. Injected `userId` is not passed to `applySignals` or `putLessonProgress`. |
| ac-2 | PASS | The second identical signal batch reports `applied: 0` and the same state. |
| ac-3 | PASS | A signal for an invisible slug is counted rejected. GET and PUT on that slug return 404 via `canSeeLesson`. |
| ac-4 | PASS | More than 200 signals returns 413 before `applySignals`. A body over 64 KB changes nothing. |
| ac-5 | PASS | A stale `rev` returns 409 with the stored state. A fresh `rev` succeeds and increments. |
| ac-6 | PASS | `GET /api/me/metrics` returns stages and accuracy, with `pct` null under 10 attempts. |

Existing sign-in tests still pass after the router gained a single `:param` segment. Exact paths are preferred. No remediation loop.
