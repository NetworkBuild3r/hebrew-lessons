# SPEC-004 Completion Summary

**Spec:** SPEC-004 — Snapshots and weekly reports on each learner's day
**Initiative:** INIT-026-tutor-data-loop
**Domain:** scheduled
**Status:** complete
**Provenance:** INIT-026/SPEC-004

## Change Produced

`internal/jobs` (`daily.go`, `weekly.go`, `scheduler.go`, tests) and `cmd/hebrew/jobs.go`.

`NewDaily` takes a `studyday.Resolver` and `WithAppZone`. A nil resolver panics. `CatchUp` fills each learner through yesterday in that learner's zone. `Run` skips a learner whose day has not ended and counts `NotYet`. The scheduler wakes at the next top of the hour and runs the daily catch-up, then the weekly catch-up. `WeeklyPayload.Zone` is the learner's zone name.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=light PASS · domain=scheduled FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

The orchestrator re-ran:

```text
/tmp/go/bin/go test -count=1 ./internal/jobs
ok github.com/NetworkBuild3r/hebrew-lessons/internal/jobs 2.762s
```

| AC | Evidence |
| --- | --- |
| ac-1 | `TestCatchUpWritesEachLearnersYesterday` |
| ac-2 | `TestCatchUpSkipsDenverBeforeTheirDayEnds` |
| ac-3 | `TestCatchUpZoneChangeWritesNoDuplicateDay` |
| ac-4 | `TestNextWake`, `TestSchedulerCatchesUpAfterAnIdleRestart` |
| ac-5 | `TestWeeklyCatchUpUsesLearnerZone` |
| ac-6 | `TestNewDailyRequiresResolver` |

## Security Review

Light. Critical 0 / High 0 / Medium 0 / Low 0.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, no `.claude/agents` | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`.

## Deviations

- `WithAppZone` is required beside the resolver, because the resolver does not expose the app location it was built with.
- A missing resolver panics rather than returning an error. The test covers that failure.
- No git commit yet.
