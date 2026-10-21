# SPEC-010 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/api-admin.test.js`. Read `addAdmin` in `server/api/admin-routes.js` (every route is `requireSession` then `requireAdmin`) and the events handler (one audit row before the response).
**Tier:** full. Pre-flight: gate_actions PASS · security_review=required FAIL · domain=api FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS (8 pass, 0 fail)

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | The test walks the exported route table. Signed out is 401. A user role is 403. |
| ac-2 | PASS | Demoting or disabling the last admin, and an admin disabling themselves, return 409. A disabled user's next request is 401. |
| ac-3 | PASS | Each mutating route in the test writes one audit row. Account changes use the existing service audit. |
| ac-4 | PASS | Progress includes metrics and trouble words. Events are that user only, newest first, and the limit clamps at 200. A user role gets 403. |
| ac-4b | PASS | Each events read writes one audit row with the admin and the target user. |
| ac-5 | PASS | A validator error returns 422 and leaves files unchanged. A valid save is then served. |
| ac-6 | PASS | Audio generation uses a fake TTS call. A second request for the same lesson while one is running returns 409. |
| ac-7 | PASS | Response bodies do not contain `password_hash`, a session token, or the test ElevenLabs key. |

The worker's real-tree audio verify reported 65 recordings. This re-run did not repeat that command; the tests use a temporary lessons directory. No remediation loop.
