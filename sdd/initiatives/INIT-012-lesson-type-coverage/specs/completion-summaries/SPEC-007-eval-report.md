# SPEC-007 eval report

**Spec:** `INIT-012/SPEC-007` (`SPEC-007-frontend-editor-new-activities.md`)
**Verifier:** independent sub-agent (`generalPurpose`, `composer-2.5-fast`, read-only constraint, did not author the code).

## Test runs (blocking checks)

```text
node --test test/plan-editor-exercise.test.js test/plan-editor-assignment.test.js \
  test/plan-editor-drift.test.js test/plan-editor-one-language.test.js \
  test/plan-editor-lesson-screen.test.js test/plan-editor-widgets.test.js \
  test/plan-editor-core.test.js
-> tests 96 | pass 96 | fail 0 | exit 0

node test/chrome-plan-editor-exercise.js
-> chrome-plan-editor-exercise: PASS (exit 0)
```

## Per-AC verdicts (Loop 3 — final)

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-editor-exercise.test.js:162-183` — `newBlock('exercise')` defaults to `kind: 'choose'` with `item-1`; marking one of three choices right saves `answer` equal to that choice (`shared/admin/plan-form.js:326-328`, `shared/admin/plan-widgets.js:1486-1490`). |
| ac-2 | PASS | `test/plan-editor-exercise.test.js:186-320` — all five kinds (`choose`, `true-false`, `unscramble`, `translate`, `dictation`) added, filled via widgets, saved via transport, reopened with `assertReopened` verifying widget and DOM values, validated with `validatePlan`, and rendered with `HebrewPlanRender`. |
| ac-3 | PASS | `test/plan-editor-exercise.test.js:339-349` — removing the choice marked right is refused with a plain sentence (`Clear the right mark first, or mark another choice.`, `shared/admin/plan-widgets.js:1473-1480`). |
| ac-4 | PASS | `test/plan-editor-assignment.test.js:115-149` — Assignment form saves `prompt`, `minSentences`, three words picked from the plan's word bank (`shared/admin/plan-widgets.js:1742-1804`), and two homework lines, and passes `validatePlan`. |
| ac-5 | PASS | `test/plan-editor-exercise.test.js:417-445` — Pattern with `future` shows nine rows (`1s`..`3p`) and plain teacher-readable N13/N14 dictionary notes (`shared/admin/plan-widgets.js:1897-1938`, `shared/admin/plan-hints.js:465`). |
| ac-6 | PASS | `test/plan-editor-exercise.test.js:148-158,387-394` — `assertNoTeacherLeaks` confirms no `id`, `path`, `index`, or JSON is shown on `exercise` or `assignment` forms (`shared/admin/plan-form.js:644-665,1217-1256`). |
| ac-7 | PASS | `test/plan-editor-exercise.test.js:207-216`, `test/plan-editor-assignment.test.js:172-189`, and `test/chrome-plan-editor-exercise.js:74-78` — preview renders via `HebrewPlanRender` (`shared/admin/plan-preview.js:4-6`). |
| ac-8 | PASS | `test/plan-editor-exercise.test.js:397-414`, `test/plan-editor-assignment.test.js:192-209`, and `test/plan-editor-one-language.test.js:418-484` — English mode shows English-only chrome; Hebrew mode shows Hebrew-only chrome. |
| ac-9 | PASS | `test/plan-editor-assignment.test.js:212-217` and `test/chrome-plan-editor-exercise.js:81-87` — CSS stacks below 900px (`shared/worksheet.css:5391-5402`) and headless Chrome at 390px and 1280px has zero horizontal overflow. |
| ac-10 | PASS | `test/plan-editor-exercise.test.js:367-384` — `<img src=x onerror=alert(1)>` saved and shown as text in form and preview; `plan-widgets.js` uses zero `innerHTML`. |
| ac-11 | PASS | All 96 tests in the editor, drift, one-language, lesson-screen, widgets, and core suites pass with 0 failures. |

## Remediation-loop log

- **Loop 1:** `ac-1`, `ac-3`, `ac-5`, `ac-7`, `ac-9`, `ac-10`, `ac-11` PASS; `ac-2`, `ac-4`, `ac-6`, `ac-8`, and `aud-1` FAIL (metadata `Id`/`Type` rows visible on exercise/assignment forms, mixed-language buttons, and missing E2E form save/reopen tests).
- **Remediation 1:** Fix sub-agent hid metadata rows and raw JSON toggle on teacher activity forms (`shared/admin/plan-form.js:644-676,880-920,1217-1302`), localized widget Add buttons (`shared/admin/plan-widgets.js`), and expanded `test/plan-editor-exercise.test.js`, `test/plan-editor-assignment.test.js`, and `test/plan-editor-one-language.test.js`.
- **Loop 2:** `ac-1`, `ac-3`–`ac-11` PASS; `ac-2` FAIL because `assertReopened` was a no-op stub for `unscramble`, `translate`, and `dictation`.
- **Remediation 2:** Fix sub-agent added full widget and DOM assertions inside `assertReopened` for `unscramble`, `translate`, and `dictation` in `test/plan-editor-exercise.test.js:186-320`.
- **Loop 3 (fresh independent verifier):** all-PASS (96/96 node tests pass; `chrome-plan-editor-exercise: PASS`).

**Overall:** all-PASS

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=frontend` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false`.
