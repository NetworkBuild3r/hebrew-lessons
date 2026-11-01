# SPEC-004 · scheduled: Snapshots and weekly reports on each learner's day

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-026
title: "Snapshots and weekly reports on each learner's day"
domain: scheduled
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-003]
blocks: [SPEC-009]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:** `Daily` holds one zone for everyone (`internal/jobs/daily.go:134`). `CatchUp` fills each learner from their last snapshot through the server's yesterday (`:201`). The scheduler wakes at 03:30 server time (`internal/jobs/scheduler.go:78`). The weekly report runs Monday 04:00 server time (`internal/jobs/weekly.go:106`), and `weekWindow` uses the server zone (`:233`).

**Scope (in):**
- `NewDaily` takes the `studyday.Resolver` in place of a zone. There is no nil-to-UTC branch.
- `CatchUp` resolves each learner's zone and fills from the day after their last snapshot through yesterday **in that zone**, still at most `MaxCatchUpDays`.
- `Run(day)` writes that day for each learner whose today, in their zone, is after it. A learner for whom it is not past yet is skipped and counted (`Result.NotYet`). `ErrFutureDay` is returned only when the day is in the future for every learner.
- The scheduler wakes at the top of every hour and runs the daily catch-up, then the weekly catch-up. It is a sweep, and per-learner logic stays in the job. The 03:30 wake and its comment go.
- Weekly: each learner's due week is their last full Monday to Sunday in their zone. It is written at the first sweep after their Monday starts, after the daily catch-up has written their Sunday. `weekWindow` and `activeMinutes` days use the learner's zone, and `WeeklyPayload.zone` holds that zone's name.
- The run log line counts learners by `zoneSource` (`learner`, `app`).
- Existing jobs tests that pin `America/Chicago` keep passing with Chicago as the app zone and no learner zones.

**Scope (out):** Rewriting snapshot days already stored. A per-learner wake time.

## Acceptance Criteria

- [ ] ac-1: One sweep at 2026-11-01T06:30:00Z with three learners writes yesterday as follows. Denver learner: `2026-10-31`. Tokyo learner (`Asia/Tokyo`): `2026-10-31`. Learner with no zone (app zone Chicago): `2026-10-31`. Each learner's row is written once.
- [ ] ac-2: At 2026-11-01T05:30:00Z the same sweep writes `2026-10-31` for the Tokyo learner, but not yet for the Denver learner, whose 2026-10-31 has not ended.
- [ ] ac-3: Changing a learner's zone from `Asia/Tokyo` to `America/Denver` between sweeps writes no duplicate day and fails nothing.
- [ ] ac-4: `Scheduler.Next` returns the next top of the hour, and a restart after idle hours catches up every learner as before (`TestSchedulerCatchesUpAfterAnIdleRestart` still passes, updated for the hourly wake).
- [ ] ac-5: A Denver learner's weekly report for the week of 2026-10-26 is written at the first sweep after 2026-11-02T00:00 Denver, and its payload's `zone` is `America/Denver`. Its minutes count only attempts inside that Denver week.
- [ ] ac-6: `NewDaily` without a resolver fails to build. No `time.UTC` default remains in `internal/jobs` outside tests.

## Deliverables

- [ ] `internal/jobs/daily.go`, `weekly.go`, `scheduler.go` and their tests
- [ ] `internal/jobs/helpers_test.go` (resolver fixture with per-learner zones)
- [ ] `cmd/hebrew/jobs.go` (pass the resolver)

## Verification Strategy

- **Claim:** every learner's snapshot and weekly report cover that learner's own calendar day and week, and the scheduler is an hourly sweep.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 ./internal/jobs ./cmd/hebrew`. These are the existing package tests with the fake clock and timer, plus the new per-zone cases. Not the full suite.
- **Pass condition:** ac-1 to ac-6 pass and no existing jobs test is deleted. A test may be changed only where it asserted the 03:30 wake. Planting the server zone back into `CatchUp` fails ac-1 for the Tokyo learner.

## Integration Points

- **Provenance tag:** `INIT-026/SPEC-004`
- SPEC-009 reads the weekly payload's `zone` and the snapshot days this spec writes.
- The weekly job keeps the daily lock (`TestWeeklyHonoursTheDailyLock`).
