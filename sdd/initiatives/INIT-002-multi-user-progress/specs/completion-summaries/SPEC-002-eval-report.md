# SPEC-002 AC verification

**Verifier:** orchestrator (not the author). Re-ran the tests and compared hashes to the planning backup.
**Tier:** full. Pre-flight: gate_actions PASS · security_review=standard FAIL · domain=service FAIL · autonomy_level=auto_tests FAIL ⇒ light_tier_eligible=false.
**Result:** PASS

| AC | Result | Evidence |
|----|--------|----------|
| ac-1 | PASS | `test/audio-preservation.test.js` ac-1 passed on re-run. `archiveExistingClip` copies to `audio/_archive/<stem>.<12 hex>.mp3` and skips an identical second copy. It is called from the lesson-clip write path (`server/audio.js`). |
| ac-2 | PASS | The same test deletes a clip and expects verify to fail, then accepts an `_archive/` copy. `npm run audio:verify` on the repo exited 0: 65 recordings. |
| ac-3 | PASS | The only `unlinkSync` in the new path is inside `copyThenUnlinkVerified`, after size and SHA-256 match. The legacy-migration test passed. |
| ac-4 | PASS | Inventory default is under `data/`, which is gitignored and dockerignored. The test rejects an inventory path inside `LESSONS_DIR`. |
| ac-5 | PASS | Inventory `data/audio-inventory/20261008T140347.json` has 65 files. All 65 SHA-256 values match `SHA256SUMS.txt` in the 2026-10-08 backup. Zero mismatches. |

`node --test test/audio-preservation.test.js`: 4 pass, 0 fail.

Full `npm test`: 117 pass, 1 fail. The failure was `EBUSY` writing `worksheets/_review/vocab-index.js`, a file this spec does not change. The same test passed on an immediate retry. Treated as a transient Windows file lock, not a SPEC-002 defect.

No remediation loop.
