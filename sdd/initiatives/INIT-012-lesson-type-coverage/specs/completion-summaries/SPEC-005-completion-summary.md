# SPEC-005 Completion Summary

**Spec:** SPEC-005 · A learner can put words in order, type a Hebrew answer, and type what they hear
**Initiative:** INIT-012 (`INIT-012-lesson-type-coverage`)
**Domain:** frontend · **Autonomy:** auto_tests
**Status:** complete
**Authored by:** `INIT-012/SPEC-005`

## Change Produced

Added `unscramble`, `translate`, and `dictation` to `shared/ui/blocks/exercise.js` and `shared/ui/blocks/compare.js` (`shuffleWords`, geresh/gershayim normalization, `sameHebrew` with `accept[]`, gesture-only dictation audio playback with load-failure skip, and first-try block summary), plus unit tests (`test/plan-render-compare.test.js`, `test/plan-render-exercise-typed.test.js`) and headless Chrome smoke (`test/chrome-exercise-smoke.js`).

| File | Change | Provenance |
|------|--------|------------|
| `shared/ui/blocks/compare.js` | Added deterministic `shuffleWords` (distinct-token fallback), geresh/gershayim normalization, `sameHebrew` with `accept[]` | `INIT-012/SPEC-005` |
| `shared/ui/blocks/exercise.js` | Added `unscramble`, `translate`, and `dictation` render + bind, audio-on-tap + load-fail skip, first-try summary | `INIT-012/SPEC-005` |
| `test/plan-render-compare.test.js` | Table-driven compare and shuffle tests | `INIT-012/SPEC-005` |
| `test/plan-render-exercise-typed.test.js` | Tests for `ac-1`–`ac-10` and edge cases | `INIT-012/SPEC-005` |
| `test/chrome-exercise-smoke.js` | Real Chrome smoke playing all 5 kinds + dictation mouse gesture + audio-fail skip | `INIT-012/SPEC-005` |

## AC Verification

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=frontend` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false` · skipped check: `None`.

Independent verifier returned all-PASS on Loop 3 after two remediation loops (Loop 1 added mechanized tests for `alt` scoring, plene-in-`accept`, word-button XSS, and dictation label/lang/dir; Loop 2 made `shuffleWords` distinct-string-aware on duplicate-word sentences). Full report: [SPEC-005-eval-report.md](SPEC-005-eval-report.md).

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-render-exercise-typed.test.js:222-256` (`shared/ui/blocks/exercise.js:562-569`) |
| ac-2 | PASS | `test/plan-render-compare.test.js:90-114` and `test/plan-render-exercise-typed.test.js:192-205,523-543` |
| ac-3 | PASS | `test/plan-render-compare.test.js:41-44` and `test/plan-render-exercise-typed.test.js:331-337` |
| ac-4 | PASS | `test/plan-render-compare.test.js:46-56` and `test/plan-render-exercise-typed.test.js:299-311` |
| ac-5 | PASS | `test/plan-render-exercise-typed.test.js:259-295,314-329` |
| ac-6 | PASS | `test/plan-render-exercise-typed.test.js:368-377` and `test/chrome-exercise-smoke.js:130-136` |
| ac-7 | PASS | `test/plan-render-exercise-typed.test.js:380-384` and `test/chrome-exercise-smoke.js:144-159` |
| ac-8 | PASS | `test/plan-render-exercise-typed.test.js:388-447` |
| ac-9 | PASS | `test/plan-render-exercise-typed.test.js:450-468` |
| ac-10 | PASS | `test/plan-render-exercise-typed.test.js:472-519` |

## Security Review

All authored strings (`words`, `alt`, `accept`, `answer`, `english`, `audioPath`) are escaped via `escapeHtml`, `audioPath` is validated against `DICTATION_AUDIO` before building the URL, and no typed learner text is sent over the network.
Critical: 0 · High: 0 · Medium: 0 · Low: 0.

## Gates & Controls

- **Autonomy:** `auto_tests`
- **One-way gate actions:** none (`gate_actions: []`)

## Harness Adaptation

| Site | Capability | Tier | What was done instead | Impact |
|------|------------|------|-----------------------|--------|
| `orchestration` dispatch | `subagent_dispatch` / persona | T2 | Dispatched `generalPurpose` on `composer-2.5-fast` in parallel with `SPEC-006` | Isolated sub-agent context preserved |
| `phase_4.5_ac_verifier` | `subagent_dispatch` | T2 | Dispatched fresh `generalPurpose` verifier on `composer-2.5-fast` with read-only constraint | Independent verification preserved |

## Trace Emission Degradation

| Site | Event | Reason | What was lost | Impact |
|------|-------|--------|---------------|--------|
| Phase 0 / On return / Phase 5 | `trace_open`, `dispatch`, `spec_lifecycle`, `trace_complete` | `scripts/dev/lib/sdd_trace` is not installed in this repository | Per-spec `.sdd-trace/SPEC-005.jsonl` events | Disclosed per `execute-spec.md`; timer and eval report provide the audit trail |

## Deviations

- None.
