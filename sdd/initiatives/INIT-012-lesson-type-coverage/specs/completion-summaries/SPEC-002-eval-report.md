# SPEC-002 eval report

**Spec:** `INIT-012/SPEC-002` (`SPEC-002-data-version-1-2-and-exercise-block.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `composer-2.5-fast`, read-only constraint, did not author the code).

## Test run (blocking checks)

```text
node --test test/plan-exercise-schema.test.js test/plan-schema-version-1-2.test.js \
  test/activity-menu-schema.test.js test/plan-editor-drift.test.js test/plan-validate.test.js \
  test/plan-revisions.test.js test/plan-ops.test.js test/plan-language-1-1.test.js
-> tests 205 | pass 205 | fail 0 | exit 0
```

## Per-AC verdicts (Loop 2 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-exercise-schema.test.js:57-63` — `ac-1: each ADR-0014 exercise kind validates in a 1.2.0 document` (5 blocks from `exercise-examples.json`, `result.ok === true`). Schema: `lesson-plan.schema.json:1011-1081` (`exerciseBlock`, per-kind `if`/`then`). Validator: `server/plans/validate.js:1133-1181` (`checkExerciseBlocks`). |
| ac-2 | PASS | `test/plan-exercise-schema.test.js:66-73` — 1.1.0 shell refused; message `this block type needs schemaVersion 1.2.0`. Message source: `validate.js:275-276,654-655`. Guards: `lesson-plan.schema.json:973-976,997-1001`. |
| ac-3 | PASS | `test/plan-schema-version-1-2.test.js:121-140` — `insertBlock of exercise into a 1.1.0 plan sets schemaVersion 1.2.0 in the same revision`; `deepEqual` confirms only expected deltas (`+block`, `revision`, `status`, `schemaVersion`). Never lowers: `:144-166` (restore keeps 1.2.0), `:168-187` (`setField` -> `invalid_op`). Ops: `server/plans/ops.js:153-166,220-221`. |
| ac-4 | PASS | `test/plan-exercise-schema.test.js:76-80` — `N20` when `answer` not in `choices`. Rule: `validate.js:140-143,1143-1147`. |
| ac-5 | PASS | `test/plan-exercise-schema.test.js:83-92` — two words -> failure; bad `alt` -> `rule === 'N21'`. Rule: `validate.js:144-147,1148-1165`. |
| ac-6 | PASS | `test/plan-exercise-schema.test.js:95-104` — missing `audioPath` -> `N22`/`schema`; `audio/../secret.mp3` -> `N22`/`audio-path`. Rule: `validate.js:1167-1173`; `lesson-plan.schema.json:109-113,1155-1161`. |
| ac-7 | PASS | `test/plan-exercise-schema.test.js:107-116` — pointed `accept` refused (`/accept/`); `english` with `<` -> `html`. Schema: `acceptString` `lesson-plan.schema.json:1003-1009`; `validate.js:1175-1178`. |
| ac-8 | PASS | `test/plan-exercise-schema.test.js:119-140` — 12 items pass; 13th -> `N10`; oversized block/plan -> `N9`. |
| ac-9 | PASS | `shared/admin/activity-map.js:24,37,53` (`Exercise` -> `exercise`, menu order, hints); `test/activity-menu-schema.test.js:43-53`; `test/plan-editor-drift.test.js:58-68`; `shared/admin/plan-hints.js:527-579`. |
| ac-10 | PASS | `test/plan-schema-version-1-2.test.js:55-61` — four `ADR-0003` samples stay `1.0.0` with unchanged validator outcomes; `test/plan-language-1-1.test.js` passes in full. |

## Remediation-loop log

- **Loop 1:** `ac-1`–`ac-7`, `ac-9`, `ac-10` PASS; `ac-8` FAIL (missing mechanized `N9` oversized-exercise fixture and 12-item boundary test), Security AC FAIL (missing `deepEqual` test on `insertBlock` version raise), and Verification Strategy edge cases missing named tests (`alt` equals main order, vowel-only `choose` mismatch, `accept` plene+defective, restore keeping `1.2.0` floor).
- **Remediation 1:** Fix sub-agent added all missing tests in `test/plan-exercise-schema.test.js:119-164` and `test/plan-schema-version-1-2.test.js:135-166`, aligned `buildsOn` (`maxItems` removed in `lesson-plan.schema.json:191-194`, `test/plan-validate.test.js:403` restored to `undefined`), and expanded `.cursor/skills/hebrew-worksheet/schema.md:47-55`.
- **Loop 2 (fresh independent verifier):** all-PASS (205/205 tests pass, exit 0).

**Overall:** all-PASS

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=data` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false`.
