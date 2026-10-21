# SPEC-002 eval report

**Verifier:** independent of the author. Orchestrator re-ran the tests after one leftover assertion was updated.

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `lesson-plan.schema.json:223` verseReadingBlock, `:225` typeItBlock. Verse fixture `validatePlan` ok |
| ac-2 | PASS | `node --test test/activity-menu-schema.test.js` — 2 pass, 0 fail |
| ac-3 | PASS | `activity-map.js` ACTIVITY_NAMES includes Writing, Notes, Verse; Match is absent |

**Suite:** `plan-lesson-stack.test.js` failed once on `blockTypeFor('Match')` at line 176 (expected `match`, got `undefined`). Assertion updated to expect absence. Re-run of the three files: 14 pass, 0 fail.

**Overall:** all-PASS

**Tier:** full.
