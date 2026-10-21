# SPEC-009 Completion Summary

**Spec:** SPEC-009 · One original sample lesson per lesson type, built by MCP and by hand, played in a browser
**Initiative:** INIT-012 (`INIT-012-lesson-type-coverage`)
**Domain:** testing · **Autonomy:** auto_tests
**Status:** complete
**Authored by:** `INIT-012/SPEC-009`

## Change Produced

Added five original sample lesson fixtures (Types A–E plus combined `five-lesson-types.plan.json` and edge fixtures) under `test/fixtures/lesson-types/`, the matrix test suite `test/plan-lesson-types-matrix.test.js`, the MCP end-to-end build test `test/plan-lesson-types-mcp.test.js`, the signed-in headless Chrome smoke `test/chrome-lesson-types-smoke.js` (covering `/editor/` hand build and learner playback at 390px and 1280px in English and Hebrew), `down()` rollback support in `server/db/migrations/010-lexicon-future-cells.js`, and the operator deliverables `SPEC-009-not-held.md` and `SPEC-009-forms-to-check.md`.

| File | Change | Provenance |
|------|--------|------------|
| `test/fixtures/lesson-types/*` | Five original lesson fixtures (A–E), combined `1.2.0` plan, `lexicon-ledaber.json`, edge fixtures, and `fixture-lib.js` | `INIT-012/SPEC-009` |
| `test/plan-lesson-types-matrix.test.js` | Matrix, renderer guard, edge cases, multi-surface XSS (`ac-8`), `ac-11` baseline check, and `ac-12` forms-to-check validation | `INIT-012/SPEC-009` |
| `test/plan-lesson-types-mcp.test.js` | Builds and publishes all 5 lessons over MCP in a temp DB, logs tool calls (`ac-2`), verifies `verified = 0` (`ac-12`) and `data/hebrew.sqlite` untouched | `INIT-012/SPEC-009` |
| `test/chrome-lesson-types-smoke.js` | Signed-in Chrome smoke: `/editor/` Type B hand build + learner playback of all 5 exercise kinds, assignment, and future table at 390px and 1280px in `en` and `he` | `INIT-012/SPEC-009` |
| `server/db/migrations/010-lexicon-future-cells.js`, `008-dictionary-glosses.js`, `009-dictionary-entries.js`, `test/plan-revisions-migration.test.js` | Implemented clean `down()` rollback on migration 010 so `test/rollout-preflight.test.js` (20/20) and `test/plan-revisions-migration.test.js` (10/10) pass | `INIT-012/SPEC-009` |
| `sdd/initiatives/INIT-012-lesson-type-coverage/specs/completion-summaries/SPEC-009-not-held.md` | Line-by-line record of unheld activities (`ac-10`) and full-suite baseline (`ac-11`: 43 pre-existing failures vs 51 pre-SPEC-001) | `INIT-012/SPEC-009` |
| `sdd/initiatives/INIT-012-lesson-type-coverage/specs/completion-summaries/SPEC-009-forms-to-check.md` | Pointed future forms of `לְדַבֵּר` (`1s`..`3p`) beside dictionary past forms, all `verified = 0` (`ac-12`) | `INIT-012/SPEC-009` |

## AC Verification

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=testing` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false` · skipped check: `None`.

Independent verifier returned all-PASS on Loop 3 after two remediation loops. Full report: [SPEC-009-eval-report.md](SPEC-009-eval-report.md).

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-lesson-types-matrix.test.js:118-147`, `test/plan-lesson-types-mcp.test.js:77-146` |
| ac-2 | PASS | `test/plan-lesson-types-mcp.test.js:148-159` |
| ac-3 | PASS | `test/chrome-lesson-types-smoke.js:188-198,249-348` |
| ac-4 | PASS | `test/plan-lesson-types-matrix.test.js:149-177` |
| ac-5 | PASS | `test/chrome-lesson-types-smoke.js:364-396,499-503` |
| ac-6 | PASS | `test/chrome-lesson-types-smoke.js:398-408` |
| ac-7 | PASS | `test/chrome-lesson-types-smoke.js:411-435` |
| ac-8 | PASS | `test/plan-lesson-types-matrix.test.js:368-413`, `test/chrome-lesson-types-smoke.js:203-213,437-472` |
| ac-9 | PASS | `test/chrome-lesson-types-smoke.js:199-200,361-362,499-503` |
| ac-10 | PASS | [SPEC-009-not-held.md](SPEC-009-not-held.md) |
| ac-11 | PASS | `test/plan-lesson-types-matrix.test.js:415-425`, [SPEC-009-not-held.md](SPEC-009-not-held.md) |
| ac-12 | PASS | `test/plan-lesson-types-matrix.test.js:353-366`, [SPEC-009-forms-to-check.md](SPEC-009-forms-to-check.md) |

## Security Review

XSS payloads are verified across editor, preview, and player for all block types, learner network guards confirm zero requests while answering exercises or typing assignments, and `data/hebrew.sqlite` `mtimeMs` is verified unchanged.
Critical: 0 · High: 0 · Medium: 0 · Low: 0.

## Gates & Controls

- **Autonomy:** `auto_tests`
- **One-way gate actions:** none (`gate_actions: []`)

## Harness Adaptation

| Site | Capability | Tier | What was done instead | Impact |
|------|------------|------|-----------------------|--------|
| `orchestration` dispatch | `subagent_dispatch` / persona | T2 | Dispatched `generalPurpose` on `composer-2.5-fast` | Isolated sub-agent context preserved |
| `phase_4.5_ac_verifier` | `subagent_dispatch` | T2 | Dispatched fresh `generalPurpose` verifier on `composer-2.5-fast` with read-only constraint | Independent verification preserved |

## Trace Emission Degradation

| Site | Event | Reason | What was lost | Impact |
|------|-------|--------|---------------|--------|
| Phase 0 / On return / Phase 5 | `trace_open`, `dispatch`, `spec_lifecycle`, `trace_complete` | `scripts/dev/lib/sdd_trace` is not installed in this repository | Per-spec `.sdd-trace/SPEC-009.jsonl` events | Disclosed per `execute-spec.md`; timer and eval report provide the audit trail |

## Deviations

- None.
