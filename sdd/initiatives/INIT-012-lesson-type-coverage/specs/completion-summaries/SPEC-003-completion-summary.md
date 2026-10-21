# SPEC-003 Completion Summary

**Spec:** SPEC-003 · The schema holds an assignment block and a future-tense table, and the checks cover them
**Initiative:** INIT-012 (`INIT-012-lesson-type-coverage`)
**Domain:** data · **Autonomy:** auto_tests
**Status:** complete
**Authored by:** `INIT-012/SPEC-003`

## Change Produced

Added `assignmentBlock` (`type: "assignment"`, `prompt`, `minSentences` 1..12, optional `useWords` 1..12, optional `homework` 1..6) and `tense: "future"` (`futureCell` 9 codes, `1.2.0` version guard) to `lesson-plan.schema.json`, `CELLS.future` and `listGrammarConcepts` future cell concepts in `server/lexicon/`, migration `010-lexicon-future-cells.js` for the SQLite `CHECK` constraint (idempotent, tested in temp DBs), `N10`/`N13`/`N14`/`N16` support in `server/plans/validate.js`, `Assignment` in `shared/admin/activity-map.js` and `shared/admin/plan-hints.js`, and tests in `test/plan-assignment-schema.test.js` and `test/plan-future-tense.test.js`.

| File | Change | Provenance |
|------|--------|------------|
| `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` | Added `assignmentBlock`, `futureCell`, `tense: "future"`, and 1.0/1.1 future-tense guards | `INIT-012/SPEC-003` |
| `server/lexicon/validate.js`, `server/lexicon/service.js` | Added `CELLS.future`, `CELL_ORDER.future`, `FUTURE_CELL_CONCEPTS`, and `listGrammarConcepts` | `INIT-012/SPEC-003` |
| `server/db/migrations/010-lexicon-future-cells.js` | Idempotent migration updating `lexicon_entries` `cell` `CHECK` for future cells | `INIT-012/SPEC-003` |
| `mcp/tools.js` | Wired `list_grammar_concepts` to `lexicon.listGrammarConcepts()` | `INIT-012/SPEC-003` |
| `server/plans/validate.js` | Added `N10` paths for assignment, `N16` satisfaction by `assignment`, `futureCell` and `1.2.0` tense messages | `INIT-012/SPEC-003` |
| `shared/admin/activity-map.js`, `shared/admin/plan-hints.js` | Added `Assignment: 'assignment'`, hints, and future tense hint | `INIT-012/SPEC-003` |
| `.cursor/skills/hebrew-worksheet/schema.md` | Added `assignment` field table and `tense: future` note | `INIT-012/SPEC-003` |
| `test/plan-assignment-schema.test.js`, `test/plan-future-tense.test.js`, `test/plan-editor-drift.test.js` | New and updated test suites (139/139 pass) | `INIT-012/SPEC-003` |

## AC Verification

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=data` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false` · skipped check: `None`.

Independent verifier returned all-PASS on Loop 2 after one remediation loop adding edge-case tests (`useWords`/`homework` markup, optional fields omitted, missing future lexicon row -> `N13`, two future spellings for the same cell). Full report: [SPEC-003-eval-report.md](SPEC-003-eval-report.md).

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-assignment-schema.test.js:53-65` |
| ac-2 | PASS | `test/plan-assignment-schema.test.js:68-99,110-119` |
| ac-3 | PASS | `test/plan-future-tense.test.js:89-93` |
| ac-4 | PASS | `test/plan-future-tense.test.js:148-155` |
| ac-5 | PASS | `test/plan-future-tense.test.js:158-171` |
| ac-6 | PASS | `test/plan-future-tense.test.js:174-177` |
| ac-7 | PASS | `test/plan-future-tense.test.js:300-331` |
| ac-8 | PASS | `test/plan-future-tense.test.js:334-348` |
| ac-9 | PASS | `test/plan-future-tense.test.js:229-249` |
| ac-10 | PASS | `test/plan-future-tense.test.js:364-370` |

## Security Review

All assignment text fields use `text80`/`text200`/`text300` with markup rejection (`html` rule), and `upsert_lexicon_entries` continues to force `verified = 0` on MCP writes.
Critical: 0 · High: 0 · Medium: 0 · Low: 0.

## Gates & Controls

- **Autonomy:** `auto_tests`
- **One-way gate actions:** none (`gate_actions: []`; `data/hebrew.sqlite` was not migrated directly during test runs)

## Harness Adaptation

| Site | Capability | Tier | What was done instead | Impact |
|------|------------|------|-----------------------|--------|
| `orchestration` dispatch | `subagent_dispatch` / persona | T2 | Dispatched `generalPurpose` on `composer-2.5-fast` in parallel with `SPEC-004` | Isolated sub-agent context preserved |
| `phase_4.5_ac_verifier` | `subagent_dispatch` | T2 | Dispatched fresh `generalPurpose` verifier on `composer-2.5-fast` with read-only constraint | Independent verification preserved |

## Trace Emission Degradation

| Site | Event | Reason | What was lost | Impact |
|------|-------|--------|---------------|--------|
| Phase 0 / On return / Phase 5 | `trace_open`, `dispatch`, `spec_lifecycle`, `trace_complete` | `scripts/dev/lib/sdd_trace` is not installed in this repository | Per-spec `.sdd-trace/SPEC-003.jsonl` events | Disclosed per `execute-spec.md`; timer and eval report provide the audit trail |

## Deviations

- Added `server/db/migrations/010-lexicon-future-cells.js` because migration `005`'s SQLite table `CHECK` constraint previously coupled `cell IN ('1s'..'3p')` strictly to `tense = 'past'`; the migration updates the `CHECK` idempotently in temp test databases without mutating `data/hebrew.sqlite` during test runs.
