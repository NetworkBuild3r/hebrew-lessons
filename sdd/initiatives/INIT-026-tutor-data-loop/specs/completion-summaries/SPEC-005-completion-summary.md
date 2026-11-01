# SPEC-005 Completion Summary

**Spec:** SPEC-005 — Taught words schedule cards; corrections carry their day
**Initiative:** INIT-026-tutor-data-loop
**Domain:** service
**Status:** complete
**Provenance:** INIT-026/SPEC-005

## Change Produced

| File | Action |
| --- | --- |
| `internal/evidence/taught.go` | `Recorder.Taught`, `RemoveTaught` |
| `internal/evidence/taught_test.go` | ac-1 through ac-5 and ac-7 |
| `internal/evidence/doc.go` | ADR-0022: outside teaching schedules and never grades |
| `internal/db/taught.go` | `RecordTaught`, `RemoveTaughtRows` |
| `internal/db/mistakes.go` | `MistakeInput.On` merges `first_on` and `last_on` |

`Taught` inserts `new` cards with `INSERT OR IGNORE`, due at `studyday.DueCutoff`. It does not write attempts or review logs and does not update an existing card. A listen card is created only when the lexeme has a stored clip. `RemoveTaught` deletes only cards that row created which still have `reps = 0` and no review log.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=required FAIL · domain=service FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

The orchestrator re-ran:

```text
ok github.com/NetworkBuild3r/hebrew-lessons/internal/evidence 0.262s
ok github.com/NetworkBuild3r/hebrew-lessons/internal/db 0.093s
ok github.com/NetworkBuild3r/hebrew-lessons/internal/db 0.095s
```

| AC | Test |
| --- | --- |
| ac-1 | `TestTaughtCreatesNewCards` |
| ac-2 | `TestTaughtLeavesExistingCardUnchanged` |
| ac-3 | `TestTaughtRejectsBadOnAndIds` |
| ac-4 | `TestRemoveTaughtKeepsReviewed` |
| ac-5 | `TestRemoveTaughtSkipsForeignCards` |
| ac-6 | `TestRecordMistakesDayBounds` |
| ac-7 | `TestTaughtLearnerIsolation` |

## Security Review

Required. Critical 0 / High 0 / Medium 0 / Low 0. Learner A's taught rows do not touch learner B (`TestTaughtLearnerIsolation`).

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, no `.claude/agents` | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`.

## Deviations

- `RecordMistakes` checks that `On` is `YYYY-MM-DD`. The "not after today, not more than 30 days back" window is enforced on `Taught` in `evidence.validateOn`. SPEC-008 must apply that same window on `record_mistakes` before it calls the store.
- No git commit yet.
