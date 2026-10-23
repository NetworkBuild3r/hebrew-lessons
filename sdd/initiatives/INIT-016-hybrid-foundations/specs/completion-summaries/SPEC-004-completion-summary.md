# SPEC-004 completion summary

**Initiative:** INIT-016 · **Spec:** SPEC-004 (docs) · **Status:** complete · **Date:** 2026-10-23

## Change

`docs/operations/class-data-through-mcp.md` is the runbook. It maps each class table to the tool that loads it, names the spec that adds the tools not built yet, gives the load order, separates shared from personal, and states the privacy rules as checks a loader runs. It records that loading waits for the operator to mark the backfill under way. Nothing was loaded.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | The mapping table covers words, patterns, explainers, class order, phrases, verb forms, and both mistakes lists. Each tool-less row says "not loaded yet" with its spec. A table that matches no row is not loaded. |
| ac-2 | PASS | "Checks before a row is written" are pass-or-skip checks, including no names, no transcripts, and no verbatim answers in shared rows. |
| ac-3 | PASS | The document names no tutor and no learner. |

## Deviations and disclosures

- No personas and no trace, as in SPEC-001.
- The sample's ten tables are not named individually anywhere in the repo. The runbook maps them by role and tells the loader to match each table to a row before loading, skipping any table with no row.
- plan_only: the operator runs the loads. The "under way" line is blank.
