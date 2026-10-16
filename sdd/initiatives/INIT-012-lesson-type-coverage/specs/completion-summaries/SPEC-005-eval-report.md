# SPEC-005 eval report

**Spec:** `INIT-012/SPEC-005` (`SPEC-005-frontend-unscramble-translate-dictation.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `composer-2.5-fast`, read-only constraint, did not author the code).

## Test runs (blocking checks)

```text
node --test test/plan-render-compare.test.js test/plan-render-exercise-typed.test.js test/plan-render-exercise.test.js
-> tests 58 | pass 58 | fail 0 | exit 0

node test/chrome-exercise-smoke.js
-> chrome-exercise-smoke: ok (exit 0)
```

## Per-AC verdicts (Loop 3 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-render-exercise-typed.test.js:222-256` — `ac-1: Unscramble accepts main order and every alt, and rejects any other order` (`shared/ui/blocks/exercise.js:562-569`). |
| ac-2 | PASS | `test/plan-render-compare.test.js:90-114` and `test/plan-render-exercise-typed.test.js:192-205,523-543` — deterministic `shuffleWords` (`shared/ui/blocks/compare.js:116-217`), double render identical, and start order never equals `words` (including 3-word duplicate-token sentences). |
| ac-3 | PASS | `test/plan-render-compare.test.js:41-44` and `test/plan-render-exercise-typed.test.js:331-337` — pointed and unpointed answers score the same (`compare.js:28-30,69-77`). |
| ac-4 | PASS | `test/plan-render-compare.test.js:46-56` and `test/plan-render-exercise-typed.test.js:299-311` — plene scores correct when in `accept` and wrong when absent. |
| ac-5 | PASS | `test/plan-render-exercise-typed.test.js:259-295,314-329` — 1st wrong shows try-again with reveal hidden; 2nd wrong reveals the pointed answer (`shared/ui/blocks/exercise.js:643-672`). |
| ac-6 | PASS | `test/plan-render-exercise-typed.test.js:368-377` and `test/chrome-exercise-smoke.js:63-69,130-136` — zero audio calls on bind; plays only on button tap (`shared/ui/blocks/exercise.js:696-736`). |
| ac-7 | PASS | `test/plan-render-exercise-typed.test.js:380-384` and `test/chrome-exercise-smoke.js:144-159` — failed audio shows plain sentence (`Audio could not load`) and enables Skip (`shared/ui/blocks/exercise.js:684-694,738-749`). |
| ac-8 | PASS | `test/plan-render-exercise-typed.test.js:388-447` — `<img src=x onerror=alert(1)>` in field, word buttons, `accept`, and `audioPath` rendered as text (`shared/ui/blocks/exercise.js:37-56,197,247-248`). |
| ac-9 | PASS | `test/plan-render-exercise-typed.test.js:450-468` — unscramble completed via `Enter` and `Space` on word buttons (`shared/ui/blocks/exercise.js:815-836`). |
| ac-10 | PASS | `test/plan-render-exercise-typed.test.js:472-519` — visible `<label for=...>`, `lang="he" dir="rtl"` on translate and dictation inputs, and first-try summary announced in live region (`shared/ui/blocks/exercise.js:231-234,359-365,444-451`). |

## Remediation-loop log

- **Loop 1:** `ac-2`, `ac-3`, `ac-5`, `ac-6`, `ac-7`, `ac-9` PASS; `ac-1`, `ac-4`, `ac-8`, `ac-10` had missing mechanized test cases (`alt` order scoring, positive plene-in-`accept`, unscramble word-button XSS, dictation label/lang/dir).
- **Remediation 1:** Fix sub-agent added all missing tests in `test/plan-render-compare.test.js` and `test/plan-render-exercise-typed.test.js`.
- **Loop 2:** `ac-1`, `ac-3`–`ac-10` PASS; `ac-2` FAIL on the duplicate-word edge case when a naive swap exchanged two indices holding the same word string.
- **Remediation 2:** Fix sub-agent updated `shuffleWords` fallback in `shared/ui/blocks/compare.js:151-217` to only swap/rotate across distinct word strings and added `assert.notDeepEqual(shuffled, words)` on the 3-word duplicate-token test.
- **Loop 3 (fresh independent verifier):** all-PASS (58/58 node tests pass; `chrome-exercise-smoke: ok`).

**Overall:** all-PASS

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=frontend` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false`.
