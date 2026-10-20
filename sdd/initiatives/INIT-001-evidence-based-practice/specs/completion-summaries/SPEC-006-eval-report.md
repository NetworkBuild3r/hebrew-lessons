# SPEC-006 Eval Report — Retry-before-reveal feedback

**Traceability:** `INIT-001/SPEC-006`  
**Verifier:** fresh readonly generalPurpose subagents (`composer-2.5-fast`) — did not author the code  
**Verification tier:** full (light_tier_eligible=false — domain=frontend)  
**Date:** 2026-10-08  
**Remediation loops:** 1 (ac-4 reload coverage added after first FAIL)  
**Overall verdict:** **PASS**

---

## Per-AC verdicts

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | First wrong shows `נסו שוב (Try again)` without `.correct` on the answer — `pattern.js` / `fill.js` / `listen-pick.js` retry paths. Smoke: wrong-once assertions for all three. |
| ac-2 | PASS | Second wrong reveals correct + speak affordance (`speakButton` / `Speech.play`). Smoke: `correctMarked` + `hasSpeak` for all three. |
| ac-3 | PASS | First wrong → `ctx.review.record(..., 'miss')` — listen-pick `he_en`; fill/pattern `en_he` (ADR “as prompted”). Smoke asserts `lastResult === 'miss'`. |
| ac-4 | PASS | `attempts` in `collect()` → `activityState` → mount/`restoreFeedback`/`show()`. Pattern + listen-pick: localStorage seed + full page reload. Fill: remount from `collect()` (no fill step on my-morning). |

---

## Tests run

| Command | Exit | Result |
| ------- | ---- | ------ |
| `node --test test/retry-before-reveal-smoke.test.js` | 0 | 4/4 pass |
| `npm test` | 0 | all pass |

## Remediation log

| Loop | Failing AC | Gap | Fix |
| ---- | ---------- | --- | --- |
| 1 | ac-4 | Smoke lacked reload/restoration assertions | Added pattern reload + listen-pick reload tests; fill remount-from-collect restore asserts; re-verifier PASS |
