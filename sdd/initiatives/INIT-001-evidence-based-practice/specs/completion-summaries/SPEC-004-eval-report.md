# SPEC-004 Eval Report — Review today page

**Traceability:** INIT-001/SPEC-004  
**Verifier:** fresh readonly `composer-2.5-fast` subagents (not the implementer)  
**Remediation loops:** 1  
**Final verdict:** **PASS** (all-PASS after loop 1)

## Verification tier

**Verification tier:** `full` · **pre-flight:** gate_actions `[]` PASS · security_review=`standard` PASS · domain=`frontend` FAIL · autonomy_level=`auto_tests` PASS => light_tier_eligible=`false` · **checks skipped:** None

## Per-AC verdicts

| AC | Loop 0 | Loop 1 | Evidence |
| ---- | ------ | ------ | -------- |
| ac-1 | PASS | PASS | `dueList` + interleave test; `review-page.js` queue |
| ac-2 | FAIL | PASS | reveal-before-grade source asserts; Knew/Missed `record` tests |
| ac-3 | FAIL | PASS | `filterQueue` aud-1 test; `typedMatches` without nikud |
| ac-4 | FAIL | PASS | home card same aud-1 filter; hide when `dueCount === 0` |
| ac-5 | FAIL | PASS | script-tag/no-fetch test; bilingual + `VOWELS_DEFAULT` test |
| ac-6 | FAIL | PASS | `vocabListPresent` + `review-no-index` bilingual message |
| XSS | FAIL | PASS | `escapeIndexText('<img onerror…>')` → `&lt;img` |

## Tests run

| Command | Result |
| ------- | ------ |
| `npm test` | 78/78 pass (exit 0) after remediation |
| `PORT=8081 node test/chrome-review-smoke.js` | ok (reveal, grade, store advance) — outside npm script |

## Remediation log

1. **Loop 0 FAIL** — home due count omitted `filterQueue`; tests lacked Missed/typed/bilingual/missing-index/XSS-through-API coverage.
2. **Fix** — align home filter with aud-1; export `escapeIndexText` / `typedMatches` / `vocabListPresent`; expand `test/review-page.test.js`.
3. **Loop 1** — fresh verifier → **all-PASS**.
