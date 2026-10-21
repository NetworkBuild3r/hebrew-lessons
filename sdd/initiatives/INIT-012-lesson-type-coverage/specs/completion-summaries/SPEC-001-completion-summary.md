# SPEC-001 Completion Summary

**Spec:** SPEC-001 · Record the lesson-type table, the Exercise and Assignment blocks, and future tense
**Initiative:** INIT-012 (`INIT-012-lesson-type-coverage`)
**Domain:** docs · **Autonomy:** plan_only
**Status:** complete
**Authored by:** `INIT-012/SPEC-001`

## Change Produced

Added `ADR-0014` defining the lesson-type table, decisions D2–D13, the `exercise` and `assignment` blocks, `tense: "future"` on `conjugation-table`, validator rules `N20`–`N23`, the `schemaVersion` `1.2.0` guard and auto-raise rules, the two-family rationale (`O1`), and answer disclosure (`D6`), along with machine-parseable example blocks in `ADR-0014-assets/exercise-examples.json`.

| File | Change | Provenance |
|------|--------|------------|
| `docs/decisions/ADR-0014-lesson-type-coverage.md` | +262 lines (new ADR) | `INIT-012/SPEC-001` |
| `docs/decisions/ADR-0014-assets/exercise-examples.json` | +122 lines (new JSON fixture of 5 exercise kinds + assignment + future conjugation table) | `INIT-012/SPEC-001` |

## AC Verification

**Tier:** `light` · pre-flight: `gate_actions` `[]` PASS · `security_review=light` PASS · `domain=docs` PASS · `autonomy_level=plan_only` PASS => `light_tier_eligible=true` · skipped check: spec-specific test suite (Deliverables name no test file).

Independent verifier returned all-PASS on Loop 2 after one remediation on `ac-2` (adding a concrete pointed Hebrew string and English gloss to the `dictation` example in `ADR-0014` §3.2). Full report: [SPEC-001-eval-report.md](SPEC-001-eval-report.md).

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `docs/decisions/ADR-0014-lesson-type-coverage.md:49-56`; mechanized block check against `lesson-plan.schema.json:212-226` -> `Mechanized orphan blocks: none` |
| ac-2 | PASS | `docs/decisions/ADR-0014-lesson-type-coverage.md:87-153` and `docs/decisions/ADR-0014-assets/exercise-examples.json:5-76` (`JSON parse OK`) |
| ac-3 | PASS | `docs/decisions/ADR-0014-lesson-type-coverage.md:191-199` (`N20`–`N22` block, `N23` warn) |
| ac-4 | PASS | `docs/decisions/ADR-0014-lesson-type-coverage.md:72,217-233` quoting `ADR-0003:176` and `ADR-0009:400`, plus `type-it` guard gap note |
| ac-5 | PASS | `docs/decisions/ADR-0014-lesson-type-coverage.md:68,250-254` |

## Security Review

Documentation and static JSON examples only; no markup (`<` or `>`), no secrets, no learner PII.
Critical: 0 · High: 0 · Medium: 0 · Low: 0.

## Gates & Controls

- **Autonomy:** `plan_only`
- **One-way gate actions:** none (`gate_actions: []`)

## Harness Adaptation

| Site | Capability | Tier | What was done instead | Impact |
|------|------------|------|-----------------------|--------|
| `orchestration` dispatch | `subagent_dispatch` / persona | T2 | Dispatched `generalPurpose` on `composer-2.5-fast` (`docs-agent` persona file is not in this repo) | Isolated sub-agent context preserved |
| `phase_4.5_ac_verifier` | `subagent_dispatch` | T2 | Dispatched fresh `generalPurpose` verifier on `composer-2.5-fast` with explicit read-only instructions | Independent verification preserved |

## Trace Emission Degradation

| Site | Event | Reason | What was lost | Impact |
|------|-------|--------|---------------|--------|
| Phase 0 / On return / Phase 5 | `trace_open`, `dispatch`, `spec_lifecycle`, `trace_complete` | `scripts/dev/lib/sdd_trace` and `validate_run_trace_invariants.py` are not installed in this repository | Per-spec `.sdd-trace/SPEC-001.jsonl` events | Disclosed per `execute-spec.md`; timer and eval report provide the audit trail |

## Deviations

- Work stayed in the main checkout per the initiative `**Branch:**` note (uncommitted `INIT-009`–`INIT-011` work in tree); no branch or commit was created.
