# SPEC-006 Completion Summary

**Spec:** SPEC-006 · A learner sees a writing task and homework, and a future-tense table
**Initiative:** INIT-012 (`INIT-012-lesson-type-coverage`)
**Domain:** frontend · **Autonomy:** auto_tests
**Status:** complete
**Authored by:** `INIT-012/SPEC-006`

## Change Produced

Added `shared/ui/blocks/assignment.js` (`HebrewBlockAssignment` with prompt, minimum-sentence counter, word chips calling `playWord` when a clip exists, ephemeral homework checklist, and Done nudge), extended `shared/ui/blocks/conjugation-table.js` for `tense: "future"` (nine rows in past order, verified-only drill queue, unverified `N14` display), registered `assignment` in `shared/ui/plan-render.js`, `shared/plan/index.html`, and `shared/app-routes.js`, added `.plan-assignment*` styles in `shared/worksheet.css`, and added tests in `test/plan-render-assignment.test.js` and `test/render-conjugation.test.js`.

| File | Change | Provenance |
|------|--------|------------|
| `shared/ui/blocks/assignment.js` | New `assignment` block player (`render`, `bind`, `countSentences`) | `INIT-012/SPEC-006` |
| `shared/ui/blocks/conjugation-table.js` | Added `future` tense support in study and drill modes | `INIT-012/SPEC-006` |
| `shared/ui/plan-render.js`, `shared/plan/index.html`, `shared/app-routes.js` | Registered and loaded `assignment.js` | `INIT-012/SPEC-006` |
| `shared/worksheet.css` | Added `.plan-assignment*` responsive styles using CSS custom properties | `INIT-012/SPEC-006` |
| `test/plan-render-assignment.test.js`, `test/render-conjugation.test.js` | New and updated test suites including headless Chrome resize + reload test (46/46 pass) | `INIT-012/SPEC-006` |

## AC Verification

**Tier:** `full` · pre-flight: `gate_actions` `[]` PASS · `security_review=standard` FAIL · `domain=frontend` FAIL · `autonomy_level=auto_tests` FAIL => `light_tier_eligible=false` · skipped check: `None`.

Independent verifier returned all-PASS on Loop 2 after one remediation loop fixing `playAssignmentWord` so chips without a clip return quietly without triggering browser TTS fallback. Full report: [SPEC-006-eval-report.md](SPEC-006-eval-report.md).

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `test/plan-render-assignment.test.js:211-235` |
| ac-2 | PASS | `test/plan-render-assignment.test.js:238-252` |
| ac-3 | PASS | `test/plan-render-assignment.test.js:255-292` (`shared/ui/blocks/assignment.js:157-172`) |
| ac-4 | PASS | `test/plan-render-assignment.test.js:295-315,355-372,488-515` |
| ac-5 | PASS | `test/plan-render-assignment.test.js:317-352` |
| ac-6 | PASS | `test/plan-render-assignment.test.js:328-350` and `test/render-conjugation.test.js:144-165` |
| ac-7 | PASS | `test/plan-render-assignment.test.js:352-375` and `test/render-conjugation.test.js:167-182` |
| ac-8 | PASS | `test/plan-render-assignment.test.js:377-395` |
| ac-9 | PASS | `test/plan-render-assignment.test.js:398-520` |

## Security Review

All authored strings pass through `escapeHtml`, learner textarea input is read via `.value` and never written as HTML, and zero network or `localStorage` calls occur.
Critical: 0 · High: 0 · Medium: 0 · Low: 0.

## Gates & Controls

- **Autonomy:** `auto_tests`
- **One-way gate actions:** none (`gate_actions: []`)

## Harness Adaptation

| Site | Capability | Tier | What was done instead | Impact |
|------|------------|------|-----------------------|--------|
| `orchestration` dispatch | `subagent_dispatch` / persona | T2 | Dispatched `generalPurpose` on `composer-2.5-fast` in parallel with `SPEC-005` | Isolated sub-agent context preserved |
| `phase_4.5_ac_verifier` | `subagent_dispatch` | T2 | Dispatched fresh `generalPurpose` verifier on `composer-2.5-fast` with read-only constraint | Independent verification preserved |

## Trace Emission Degradation

| Site | Event | Reason | What was lost | Impact |
|------|-------|--------|---------------|--------|
| Phase 0 / On return / Phase 5 | `trace_open`, `dispatch`, `spec_lifecycle`, `trace_complete` | `scripts/dev/lib/sdd_trace` is not installed in this repository | Per-spec `.sdd-trace/SPEC-006.jsonl` events | Disclosed per `execute-spec.md`; timer and eval report provide the audit trail |

## Deviations

- None.
