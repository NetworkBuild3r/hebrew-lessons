# SPEC-017 AC verification

**Verifier:** orchestrator (not the author). Re-ran `node --test test/admin-lesson-editor.test.js` (8 pass), `node test/chrome-admin-lessons-smoke.js` (ok: true), and `npm run audio:verify` (65 recordings, exit 0). Read the save path in `shared/admin/lesson-editor.js`: baseline JSON is compared before save, and audio confirm is a separate request.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=standard FAIL · domain=frontend FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | After a lesson is limited to one user, the other learner's list is empty of it and the selected learner still sees it. |
| ac-2 | PASS | A changed meaning is still there after reopen. The lesson `index.html` hash is unchanged. |
| ac-3 | PASS | An unknown activity type is linked to the activities section. Save stays disabled. Worksheet and index hashes are unchanged. |
| ac-4 | PASS | The plan and the credit warning show first. Cancel sends no generate request. Confirm uses fake TTS. Temp-tree audio verify exits 0. |
| ac-5 | PASS | A new lesson is `hidden` and stays invisible to a learner until an admin shares it. |
| ac-6 | PASS | `<script>alert(1)</script>` is text in the editor and in the player. No live script tag. |
| ac-7 | PASS | Vocab and activity rows have Up buttons. The audio dialog opens and closes on Escape. |

Real-tree audio verify still accounts for 65 recordings. No remediation loop.
