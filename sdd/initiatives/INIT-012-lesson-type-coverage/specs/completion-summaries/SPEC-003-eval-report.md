# SPEC-003 eval report

**Spec:** `INIT-012/SPEC-003` (`SPEC-003-data-assignment-and-future-tense.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `composer-2.5-fast`, read-only constraint, did not author the code).

## Test run (blocking checks)

```text
node --test test/plan-assignment-schema.test.js test/plan-future-tense.test.js \
  test/plan-exercise-schema.test.js test/plan-schema-version-1-2.test.js \
  test/activity-menu-schema.test.js test/plan-editor-drift.test.js \
  test/plan-validate.test.js test/plan-ops.test.js test/lexicon-service.test.js
-> tests 139 | pass 139 | fail 0 | exit 0
```

## Per-AC verdicts (Loop 2 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-assignment-schema.test.js:53-65` — `ac-1: ADR-0014 assignment validates in 1.2.0 and is refused in 1.1.0`. Schema: `lesson-plan.schema.json:1046-1050,1219-1240`. Validator: `server/plans/validate.js:669`. |
| ac-2 | PASS | `test/plan-assignment-schema.test.js:68-99,110-119` — `minSentences` 0/13, 7 `homework` lines, `N10` caps, and markup (`<img src=x>`) in `prompt`, `useWords`, and `homework` refused with `html`. |
| ac-3 | PASS | `test/plan-future-tense.test.js:89-93` — `ac-3: future table with nine lexicon forms validates`. Schema: `lesson-plan.schema.json:774,816-824,845`. |
| ac-4 | PASS | `test/plan-future-tense.test.js:148-155` — past form in a future cell refused by `N13` (`server/plans/validate.js:1302`). |
| ac-5 | PASS | `test/plan-future-tense.test.js:158-171` — unverified future form validates with `N14` warning (`server/plans/validate.js:1300`). |
| ac-6 | PASS | `test/plan-future-tense.test.js:174-177` — `tense: imperative` refused (`lesson-plan.schema.json:774`). |
| ac-7 | PASS | `test/plan-future-tense.test.js:300-331` — `listGrammarConcepts` returns `cell:future.1s`..`3p`; `upsert_lexicon_entries` accepts `3ms` and refuses `ms` (`server/lexicon/validate.js:31-34`, `server/lexicon/service.js:31-41,73-82`). |
| ac-8 | PASS | `test/plan-future-tense.test.js:334-348` — MCP upsert stores `verified = 0` when `verified: true` is sent (`server/lexicon/service.js:206-207`). |
| ac-9 | PASS | `test/plan-future-tense.test.js:229-249` — `insertBlock` of future table into 1.1.0 plan raises `schemaVersion` to `1.2.0` in the same revision (`server/plans/validate.js:438-446`, `server/plans/ops.js:159-168`). |
| ac-10 | PASS | `test/plan-future-tense.test.js:364-370` — 1.1.0 present and past conjugation-table fixtures validate unchanged. |

## Remediation-loop log

- **Loop 1:** `ac-1`–`ac-10` and Security ACs PASS; Verification Strategy edge cases had 3 missing named tests (markup on `useWords`/`homework`, assignment without optional `useWords`/`homework`, missing future lexicon form -> `N13`, and two stored future spellings for the same cell).
- **Remediation 1:** Fix sub-agent added all 4 named tests in `test/plan-assignment-schema.test.js:88-108` and `test/plan-future-tense.test.js:96-146`.
- **Loop 2 (fresh independent verifier):** all-PASS (139/139 tests pass, exit 0).

**Overall:** all-PASS

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=data` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false`.
