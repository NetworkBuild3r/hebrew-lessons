# SPEC-004 completion summary

**Initiative:** INIT-017 · **Spec:** SPEC-004 (api) · **Status:** complete · **Date:** 2026-10-23

## Change

Five worksheet shapes are MCP resources under `hebrew://templates/`. A unit can store one of them. `validate_unit` blocks a type that is missing the step that shape needs. `get_unit` returns the stored type.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestPatternToolsAdminLearnerAndPath` reads all five resources. None names a tutor or a learner. |
| ac-2 | PASS | A short story without a story step and three questions fails `unit.worksheet_shape`. `TestWorksheetShape` covers the five shapes. |
| ac-3 | PASS | A review sheet is stored and `get_unit` returns `review-sheet`. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- The column rides migration 013, with SPEC-001, instead of a migration of its own.
