# SPEC-007 Completion Summary

**Spec:** SPEC-007 · A teacher builds an exercise, an assignment, and a future table without JSON
**Initiative:** INIT-012 (`INIT-012-lesson-type-coverage`)
**Domain:** frontend · **Autonomy:** auto_tests
**Status:** complete
**Authored by:** `INIT-012/SPEC-007`

## Change Produced

Added `exercise-items`, `assignment-words`, and `assignment-homework` widgets and future-tense support in `table-cells` in `shared/admin/plan-widgets.js`, added `exercise`, `assignment`, and `type-it` block definitions, kind-change double confirmation (`aud-1`), single-language teacher chrome (`ac-8`), and hidden `id`/`type`/JSON metadata (`ac-6`) in `shared/admin/plan-form.js`, added localized hints in `shared/admin/plan-hints.js`, updated `shared/admin/plan-builder.js` and `server/plans/hebrew-checks.js`, added responsive `<900px` stacking rules in `shared/worksheet.css`, and added unit + headless Chrome tests (`test/plan-editor-exercise.test.js`, `test/plan-editor-assignment.test.js`, `test/chrome-plan-editor-exercise.js`).

| File | Change | Provenance |
|------|--------|------------|
| `shared/admin/plan-widgets.js` | Added `exercise-items` (all 5 kinds, right-choice radio, audio-clip picker for dictation), `assignment-words`, `assignment-homework`, and 9-row `future` support in `table-cells` | `INIT-012/SPEC-007` |
| `shared/admin/plan-form.js` | Added `BLOCKS`/`newBlock` for `exercise`, `assignment`, `type-it`; `aud-1` kind-change double confirm; single-language chrome and hidden `id`/`type`/JSON toggle | `INIT-012/SPEC-007` |
| `shared/admin/plan-hints.js` | Added localized Hebrew/English hints and widget bindings for `exercise`, `assignment`, and `future` | `INIT-012/SPEC-007` |
| `shared/admin/plan-builder.js`, `server/plans/hebrew-checks.js` | Wired lesson-screen activity seeding and field-language classifications | `INIT-012/SPEC-007` |
| `shared/worksheet.css` | Added `<900px` responsive stacking for exercise and assignment editor rows | `INIT-012/SPEC-007` |
| `test/plan-editor-exercise.test.js`, `test/plan-editor-assignment.test.js`, `test/chrome-plan-editor-exercise.js`, `test/plan-editor-one-language.test.js`, `test/plan-editor-drift.test.js`, `test/plan-editor-widgets.test.js`, `test/helpers/plan-editor-fixtures.js` | New and updated test suites (96/96 node tests + Chrome smoke pass) | `INIT-012/SPEC-007` |

## AC Verification

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=frontend` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false` · skipped check: `None`.

Independent verifier returned all-PASS on Loop 3 after two remediation loops (Loop 1 hid `id`/`type`/JSON metadata rows on teacher activity forms, localized Add/Save buttons for single-language chrome, and added E2E form tests; Loop 2 added DOM and widget reopen assertions for `unscramble`, `translate`, and `dictation`). Full report: [SPEC-007-eval-report.md](SPEC-007-eval-report.md).

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-editor-exercise.test.js:162-183` |
| ac-2 | PASS | `test/plan-editor-exercise.test.js:186-320` |
| ac-3 | PASS | `test/plan-editor-exercise.test.js:339-349` |
| ac-4 | PASS | `test/plan-editor-assignment.test.js:115-149` |
| ac-5 | PASS | `test/plan-editor-exercise.test.js:417-445` |
| ac-6 | PASS | `test/plan-editor-exercise.test.js:148-158,387-394` |
| ac-7 | PASS | `test/plan-editor-exercise.test.js:207-216`, `test/plan-editor-assignment.test.js:172-189`, `test/chrome-plan-editor-exercise.js:74-78` |
| ac-8 | PASS | `test/plan-editor-exercise.test.js:397-414`, `test/plan-editor-assignment.test.js:192-209`, `test/plan-editor-one-language.test.js:418-484` |
| ac-9 | PASS | `test/plan-editor-assignment.test.js:212-217`, `test/chrome-plan-editor-exercise.js:81-87` |
| ac-10 | PASS | `test/plan-editor-exercise.test.js:367-384` |
| ac-11 | PASS | 96/96 pass across all 7 plan-editor test suites |

## Security Review

`/editor/` remains gated by admin session role (`server/web.js:423-425`), all writes include `expectedRevision`, and `plan-widgets.js` uses `createElement`/`textContent`/`.value` with zero `innerHTML`.
Critical: 0 · High: 0 · Medium: 0 · Low: 0.

## Gates & Controls

- **Autonomy:** `auto_tests`
- **One-way gate actions:** none (`gate_actions: []`)

## Harness Adaptation

| Site | Capability | Tier | What was done instead | Impact |
|------|------------|------|-----------------------|--------|
| `orchestration` dispatch | `subagent_dispatch` / persona | T2 | Dispatched `generalPurpose` on `composer-2.5-fast` in parallel with `SPEC-008` | Isolated sub-agent context preserved |
| `phase_4.5_ac_verifier` | `subagent_dispatch` | T2 | Dispatched fresh `generalPurpose` verifier on `composer-2.5-fast` with read-only constraint | Independent verification preserved |

## Trace Emission Degradation

| Site | Event | Reason | What was lost | Impact |
|------|-------|--------|---------------|--------|
| Phase 0 / On return / Phase 5 | `trace_open`, `dispatch`, `spec_lifecycle`, `trace_complete` | `scripts/dev/lib/sdd_trace` is not installed in this repository | Per-spec `.sdd-trace/SPEC-007.jsonl` events | Disclosed per `execute-spec.md`; timer and eval report provide the audit trail |

## Deviations

- Added `type-it` to `BLOCKS` and `newBlock` in `shared/admin/plan-form.js` alongside `exercise` and `assignment` so `test/plan-editor-drift.test.js` passes with zero failures (`ac-11`).
