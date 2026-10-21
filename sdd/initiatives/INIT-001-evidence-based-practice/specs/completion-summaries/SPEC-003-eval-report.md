# SPEC-003 Eval Report — Frontend review store and scheduler

**Traceability:** `INIT-001/SPEC-003`  
**Verifier:** fresh readonly generalPurpose subagents (`composer-2.5-fast`) — did not author the code  
**Verification tier:** full (light_tier_eligible=false — domain=frontend)  
**Date:** 2026-10-08  
**Remediation loops:** 1 (ac-5 strengthened after first FAIL)  
**Overall verdict:** **PASS**

---

## Per-AC verdicts

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | `shared/player/review.js` `nextState` + `LADDER_DAYS` [1,3,7,14,30]. Tests: pass advances, same-day no-advance, miss→step 0, step-4 cap — green in `test/review-scheduler.test.js`. |
| ac-2 | PASS | `dueList` oldest-first, caps 30/10, interleave ≤2 consecutive same lesson. Tests: oldest-first, caps, interleave — green. |
| ac-3 | PASS | `words.js` mark-word → `ctx.review.record(item.word, 'he_en', pass\|miss)`; `core.js` exposes `ctx.review`. |
| ac-4 | PASS | `migrateLessonMarks` + `migratedLessons` guard. Tests: missed/knew mapping per ADR §6; re-run no-op — green in `test/review-store.test.js`. |
| ac-5 | PASS | `writeStorage` fail-soft; try/catch in core load/migrate and words record. Tests: quota keep in-memory; mark+migrate under quota with clean console — green (loop 1). |
| ac-6 | PASS | No `fetch` in review path; `worksheet.js` loads `review.js` before activities; no lesson `index.html` edits for this spec. |

---

## Tests run

| Command | Exit | Result |
| ------- | ---- | ------ |
| `npm test` | 0 | 47/47 pass (after ac-5 remediation) |
| `node --test test/review-scheduler.test.js test/review-store.test.js` | 0 | all pass |

## Remediation log

| Loop | Failing AC | Gap | Fix |
| ---- | ---------- | --- | --- |
| 1 | ac-5 | Quota unit test covered `record` only; no clean-console / mark+migrate under quota | Added `quota on setItem: mark + migrate paths stay runnable with a clean console` in `test/review-store.test.js`; re-verifier PASS |
