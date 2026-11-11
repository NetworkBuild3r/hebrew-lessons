# INIT-038/SPEC-015 completion summary: a full spelling counts as the word

**Status:** complete, 2026-11-10. Written by the orchestrator from [Full spelling grade](0f73c3f4-8529-44e9-8580-68e5e1186603)'s return. Re-run the same day: `internal/fsrs`, `internal/activity`, and `internal/placement` passed.

## What changed

`GradeTyped` is unchanged. `GradeTypedAmong` drops ו and י from both sides and returns Good when that remainder belongs to one lexicon entry and is at least 2 letters. `בית` and `בת` stay on the old distance grade. Review stores a unique fold as correct, not a near miss. Placement accepts it, because placement takes Good only. Past attempts were not re-graded. `hebrew backfill-spell` was not run.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no principal-backend-developer agent is installed | Full tools. Disclosed. |

## AC verification

Tier **full**. Pre-flight: gate_actions empty, pass. security_review light, pass. domain service, fail. autonomy_level auto_tests, fail. light_tier_eligible false.

The worker reported `go test ./internal/fsrs/... ./internal/activity/... ./internal/placement/... -count=1` as 1540 pass, 0 fail. The orchestrator confirmed `GradeTypedAmong`, `dropMatres`, and `LexemeConsonantForms` are in the tree. It did not re-run the tests.

## Trace

The timer launcher and the trace library are not in this kit. No timer and no trace were written.
