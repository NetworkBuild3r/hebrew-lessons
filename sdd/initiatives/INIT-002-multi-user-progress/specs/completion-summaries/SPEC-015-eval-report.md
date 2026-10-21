# SPEC-015 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/progress-view.test.js` (8 pass) and `node test/chrome-signin-smoke.js` (ok: true, including `/progress/`). Read `shared/progress-page.js` and `server/progress.js` (`pct` is `pass / attempts`, a fraction). `formatPct` multiplies that fraction by 100.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=light PASS · domain=frontend FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | Stage bars show the fixture counts as text, not color alone. |
| ac-2 | PASS | A fraction such as `0.8` renders as `80%` with the attempt count. `pct: null` shows the "not enough answers yet" sentence. |
| ac-3 | PASS | A new account on `/progress/` is the empty state and links to Today (`/`) and Lessons (`/lessons/`). |
| ac-4 | PASS | Rendered HTML and source contain no streak, points, badge, or leaderboard. |
| ac-5 | PASS | Labels are Hebrew with English, including the long-term meaning and both "why this matters" lines. |

The existing sign-in assertions in the smoke still pass. No remediation loop.
