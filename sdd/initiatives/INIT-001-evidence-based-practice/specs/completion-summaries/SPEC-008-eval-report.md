# SPEC-008 Eval Report — Validator nudges

**Traceability:** INIT-001/SPEC-008  
**Verifier:** fresh explore Task (`composer-2.5-fast`, agent `62a2e1a0-a792-4d37-97da-348f6ec1dd42`) — did **not** author the code  
**Date:** 2026-10-08  
**Overall:** **PASS** (all-PASS) · remediation loops: 0

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` PASS · security_review=`standard` FAIL (not light-eligible) · domain=`service` FAIL · autonomy_level=`auto_tests` FAIL ⇒ light_tier_eligible=`false` · **checks skipped:** None

| AC | Verdict | Evidence |
| -- | ------- | -------- |
| ac-1 | PASS | `validate.js` recycle count + warn `<3`; test `ac-1: warns when recycled words < 3; silent when ≥ 3` ✔ |
| ac-2 | PASS | encounters `<2` warn for non-recycled vocab; test `ac-2: warns when a new word appears fewer than 2 times` ✔ |
| ac-3 | PASS | coverage estimate `%` warn `<90%`; test `ac-3: warns with estimated coverage %…` ✔ |
| ac-4 | PASS | malformed glosses → error; stray keys → warn; test `ac-4: malformed glosses…` ✔ |
| ac-5 | PASS | my-morning / shai / anal-sex-vocab exit 0; suite `existing published lessons` ✔; `npm test` 59/59 |
| ac-6 | PASS | `formatUpsertHead` + store WARN parse; test `ac-6: upsert_lesson reply head…` ✔ |

**Commands:** `npm test` → 59 pass / 0 fail; manual validate on three published slugs → exit 0 (warnings only).
