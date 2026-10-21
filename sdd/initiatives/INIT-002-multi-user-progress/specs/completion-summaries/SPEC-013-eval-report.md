# SPEC-013 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/progress-sync.test.js test/review-scheduler.test.js test/review-store.test.js` (26 pass) and `node test/chrome-progress-smoke.js` (ok: true). Read `shared/player/sync.js`: queue prefix `hebrew-sync:v1:`, cap 1000, pending text as confirmed, flushes to `/api/review/signals`.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=light PASS · domain=frontend FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | Profile A marks a word. Profile B, same account, sees step 1 and a later due date. |
| ac-2 | PASS | A fresh account shows no progress. Legacy `hebrew-review:v1` and `hebrew-worksheet:<slug>` stay byte-identical. |
| ac-3 | PASS | With the network blocked, the top bar shows the pending sentence. After restore, the server gained 3 events, matching the answers given. |
| ac-4 | PASS | User A's queue key is not read or sent while user B is signed in. |
| ac-5 | PASS | A 409 adopts the server lesson state and revision and does not throw. |
| ac-6 | PASS | The smoke logged 13 fetches. Every URL starts with `/api/`. |

The review ladder tests still pass, so the scheduler was not copied or changed. No remediation loop.
