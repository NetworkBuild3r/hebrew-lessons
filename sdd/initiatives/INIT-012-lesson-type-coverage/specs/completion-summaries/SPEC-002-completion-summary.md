# SPEC-002 Completion Summary

**Spec:** SPEC-002 · The plan schema has a 1.2.0 version, and the validator accepts the exercise block
**Initiative:** INIT-012 (`INIT-012-lesson-type-coverage`)
**Domain:** data · **Autonomy:** auto_tests
**Status:** complete
**Authored by:** `INIT-012/SPEC-002`

## Change Produced

Added `schemaVersion` `1.2.0` (`v120BlockType`, `v120Only`, `usesV120`, version rank ordering and restore floor), `exerciseBlock` and its five per-kind item definitions (`choose`, `true-false`, `unscramble`, `translate`, `dictation`) to `lesson-plan.schema.json`, validator rules `N20`–`N23` in `server/plans/validate.js`, `Exercise` in `shared/admin/activity-map.js` and `shared/admin/plan-hints.js`, the 1.2.0 and `exercise` tables in `.cursor/skills/hebrew-worksheet/schema.md`, and the test suites `test/plan-exercise-schema.test.js` and `test/plan-schema-version-1-2.test.js`.

| File | Change | Provenance |
|------|--------|------------|
| `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` | Added `1.2.0`, `v120BlockType`, `v120Only`, `acceptString`, `exerciseBlock`, and 5 item defs | `INIT-012/SPEC-002` |
| `server/plans/validate.js` | Added `V120`, `N20`–`N23`, `v120Block`/`v120Field` messages, `N10_LISTS` paths, `usesV120`, `N7` count | `INIT-012/SPEC-002` |
| `server/plans/ops.js` | Version rank ordering (`1.0.0` < `1.1.0` < `1.2.0`), `usesV120` auto-raise, restore floor | `INIT-012/SPEC-002` |
| `shared/admin/activity-map.js` | Added `Exercise: 'exercise'`, menu order, and Hebrew/English hint | `INIT-012/SPEC-002` |
| `shared/admin/plan-hints.js` | Added `exercise` block and item field hints (plus `type-it` hints to keep drift tests green) | `INIT-012/SPEC-002` |
| `.cursor/skills/hebrew-worksheet/schema.md` | Documented `schemaVersion` `1.2.0` and `exercise` block + per-kind item fields | `INIT-012/SPEC-002` |
| `test/plan-exercise-schema.test.js` | New test suite covering `ac-1`–`ac-8`, `N20`–`N23`, and edge cases | `INIT-012/SPEC-002` |
| `test/plan-schema-version-1-2.test.js` | New test suite covering `1.2.0` guards, auto-raise `deepEqual`, no-downgrade, restore floor, `ac-10` | `INIT-012/SPEC-002` |
| `test/activity-menu-schema.test.js`, `test/plan-editor-drift.test.js`, `test/plan-language-1-1.test.js` | Updated for `1.2.0` and `exercise` | `INIT-012/SPEC-002` |

## AC Verification

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=data` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false` · skipped check: `None`.

Independent verifier returned all-PASS on Loop 2 after one remediation loop (adding mechanized `N9` oversized-exercise tests, 12-item boundary test, `deepEqual` assertion on `insertBlock` auto-raise, and edge-case tests for `alt` equals main, vowel-only `choose` mismatch, plene+defective `accept`, and restore floor). Full report: [SPEC-002-eval-report.md](SPEC-002-eval-report.md).

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-exercise-schema.test.js:57-63` (`ac-1: each ADR-0014 exercise kind validates in a 1.2.0 document`) |
| ac-2 | PASS | `test/plan-exercise-schema.test.js:66-73` (`this block type needs schemaVersion 1.2.0`) |
| ac-3 | PASS | `test/plan-schema-version-1-2.test.js:121-187` (auto-raise in same revision + `deepEqual` + no-downgrade + restore floor) |
| ac-4 | PASS | `test/plan-exercise-schema.test.js:76-80` (`N20` choose answer not in choices) |
| ac-5 | PASS | `test/plan-exercise-schema.test.js:83-92,143-147` (`N21` unscramble words count & `alt` permutation) |
| ac-6 | PASS | `test/plan-exercise-schema.test.js:95-104` (`N22` & `audio-path` on dictation) |
| ac-7 | PASS | `test/plan-exercise-schema.test.js:107-116` (pointed `accept` & `<`/`>` markup refused) |
| ac-8 | PASS | `test/plan-exercise-schema.test.js:119-140` (`N10` 12/13 items & `N9` block/plan byte caps) |
| ac-9 | PASS | `shared/admin/activity-map.js:24,37,53` and `test/activity-menu-schema.test.js:43-53` |
| ac-10 | PASS | `test/plan-schema-version-1-2.test.js:55-61` and `test/plan-language-1-1.test.js` (205/205 pass) |

## Security Review

All new text fields use `text80`/`text120`/`text200`/`acceptString` with markup rejection (`html` rule), `acceptString` rejects U+0591..U+05C7, `audioPath` enforces the existing safe path pattern, `N9` and `N10` cap sizes, and `schemaVersion` auto-raise mutates no unintended field (`deepEqual` verified).
Critical: 0 · High: 0 · Medium: 0 · Low: 0.

## Gates & Controls

- **Autonomy:** `auto_tests`
- **One-way gate actions:** none (`gate_actions: []`)

## Harness Adaptation

| Site | Capability | Tier | What was done instead | Impact |
|------|------------|------|-----------------------|--------|
| `orchestration` dispatch | `subagent_dispatch` / persona | T2 | Dispatched `generalPurpose` on `composer-2.5-fast` (`data-agent` persona file is not in this repo) | Isolated sub-agent context preserved |
| `phase_4.5_ac_verifier` | `subagent_dispatch` | T2 | Dispatched fresh `generalPurpose` verifier on `composer-2.5-fast` with read-only constraint | Independent verification preserved |

## Trace Emission Degradation

| Site | Event | Reason | What was lost | Impact |
|------|-------|--------|---------------|--------|
| Phase 0 / On return / Phase 5 | `trace_open`, `dispatch`, `spec_lifecycle`, `trace_complete` | `scripts/dev/lib/sdd_trace` is not installed in this repository | Per-spec `.sdd-trace/SPEC-002.jsonl` events | Disclosed per `execute-spec.md`; timer and eval report provide the audit trail |

## Deviations

- Added `type-it` hints in `shared/admin/plan-hints.js` and removed the stale `maxItems: 12` on `buildsOn` in `lesson-plan.schema.json` (which `INIT-010/SPEC-002` had already asserted as `undefined` in `test/plan-validate.test.js:403`) so the blocking suites (`test/plan-editor-drift.test.js` and `test/plan-validate.test.js`) pass 205/205 with zero test weakening.
- Work stayed in the main checkout per the initiative `**Branch:**` note; no branch or commit was created.
