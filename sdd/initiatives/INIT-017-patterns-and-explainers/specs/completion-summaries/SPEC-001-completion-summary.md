# SPEC-001 completion summary

**Initiative:** INIT-017 · **Spec:** SPEC-001 (data) · **Status:** complete · **Date:** 2026-10-23

## Change

Migration 013 adds grammar patterns, prerequisites, and explainers, and gives each unit a pattern and a worksheet type. A prerequisite cycle is refused in the store and the batch is rolled back. A published-only read hides a draft.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestPatternsMigrationKeepsExistingRows` applies 013 to a database at version 12 and keeps the seeded user. |
| ac-2 | PASS | `TestPatternCycleRollsBack` refuses a cycle and leaves the tables empty. |
| ac-3 | PASS | `TestExplainerPublishedOnly`: `ExplainerFor(slug, true)` misses a draft. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- Explainers live in `internal/db/patterns.go` with the pattern store. The spec named a separate `explainers.go`.
- The worksheet-type column from SPEC-004 is in this migration, because 013 had not landed yet.
- `publish_audit` is rebuilt so its action check can name `upsert_patterns` and `upsert_explainer`. Existing rows are copied. Later migration tests set that rebuilt table aside when they compare "nothing else changed."
