# SPEC-014 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-014 (scheduled) · **Status:** complete, with one open gate (db_migration on a real-database copy) · **Date:** 2026-10-19

## Change

Commit `8aebff0` on local branch `v3-spec-014`, merged into local `v3` as `a5fee9b` with no conflicts. Not pushed.

The first worker run stopped without writing code: the spec assumed tables migration 007 does not have (per-word state history, a `regression_events` row with no word, a `meta` lock table). The operator approved migration 009 (Gate 7), the spec was amended, and the same worker was resumed.

- **`migrations/009_daily_job.sql`:** new `job_state` table (key, value, updated_at); `regression_events` rebuilt with nullable `lexeme_id`, `skill`, `from_state`, `to_state`, a `kind` column (`word` or `band_slip`), and CHECKs that keep word rows complete and band-slip rows wordless. No table, view or trigger references `regression_events` (checked by the worker, by the orchestrator over all migrations, and pinned by a test), so no temp copy was needed.
- **`internal/jobs`:** `Daily.Run`, `Daily.CatchUp` (at most 14 days), `Scheduler` (03:30 in `TZ`, default `America/Chicago`; `HEBREW_JOBS=off` disables), lock as a `daily.lock` row in `job_state` with a 30-minute takeover.
- **`internal/learner`:** `Model.Snapshot` returns the summary, per-word states and per-band known counts from one evaluation. `Summary` now delegates to it.
- **`internal/db/daily.go`:** job queries and the per-learner transaction helper.
- **CLI:** `hebrew jobs daily [--day YYYY-MM-DD]`; without `--day` it runs the same catch-up as the scheduler (beyond the spec).
- **Shared files edited:** `cmd/hebrew/main.go` (8 lines: the `jobs` dispatch and starting and stopping the scheduler around `serve`), and the pinned migration lists in `db_test.go`, `learning_test.go`, `migrate_006/007/008_test.go` (8 to 9, `kind` column, `job_state` excluded from the "unchanged tables" check).

## Acceptance criteria

| AC | Result | Evidence |
| --- | --- | --- |
| ac-1 | PASS | `TestRunTwiceLeavesTheSameRows`, `TestJobsDailyCommandWritesTheDay` |
| ac-2 | PASS | `TestCatchUpFillsEveryMissedDay`, `TestSchedulerCatchesUpAfterAnIdleRestart`, `TestCatchUpIsCappedAtFourteenDays` |
| ac-3 | PASS | `TestRegressionIsATransitionBetweenSnapshots` (9 subtests; struggling beats regressed; no repeat events) |
| ac-4 | PASS | `TestBandSlipTwentyPointDrop`, `TestBandSlipThreshold`, `TestSlippedArithmetic`, `TestBandSlipIsNotRepeatedEveryDay` |

Migration tests (`migrate_009_test.go`): rows and ids survive from a seeded 001..008 database, indexes return, `foreign_key_check` clean, nullable and `kind` behaviour, `from_state = to_state` still refused, the full list applies to an empty database, a failure after the DROP rolls back to 008.

Orchestrator checks on merged `v3` at `a5fee9b`: `gofmt -l .` empty, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `go test -count=1 ./...` all packages ok, `go test -race ./internal/jobs/` ok. The orchestrator also built the real binary in Docker and ran it on a fresh temp `DATA_DIR`: migrations 001 to 009 applied, `hebrew jobs daily --day 2026-01-01` ran twice with the same result, a future day was refused (`day is in the future`), and the no-flag catch-up ran. That database had no learners, so the run proves wiring and migration only; the data cases are the unit tests.

## Deviations and risks

- **Plan defect, fixed by operator decision:** the spec's `meta` row became a `job_state` row.
- **Catch-up days get run-time state.** Cards cannot be rolled back, so a missed day is filled with today's state, and events from a multi-day gap land on the first missed day. Disclosed in the `internal/jobs` package doc.
- **Stricter CHECKs than "nullable"**, to keep 007's guarantees for word rows.
- **`-race` on `internal/learner`:** `TestListFirstPageOver50kAttempts` (200 ms limit) takes about 2.1 s under the race detector. The worker reproduced it on the previous commit; it is not caused by this spec. It is the same cost SPEC-013 flagged (about 86 to 90 ms per read of all attempts without the detector). Treat it as a risk for SPEC-017 and SPEC-021, which read the model on page loads.
- **Not tested:** `serve` with the scheduler running end to end, and two real processes sharing one SQLite file (the lock is tested with two job instances on one connection).
- No personas, timer or independent verifier.

## Gates

- **db_migration (open):** the first run of migrations 006 to 009 on a copy of the real database needs the operator. Waiting on a path to a copy. Never run against the real database from here.
- Do not push `v3`: a push publishes an image that may auto-deploy, and 006 to 009 are one-way.
