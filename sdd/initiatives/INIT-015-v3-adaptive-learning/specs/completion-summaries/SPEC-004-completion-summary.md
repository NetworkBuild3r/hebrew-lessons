# SPEC-004 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-004 (data) · **Status:** complete · **Date:** 2026-10-19

## Change

Commit `9fa541f` on local branch `v3-spec-004` (5 files, +744 −8), merged into local `v3` as `dd61bc9` with no conflicts. Not pushed.

- `migrations/007_learner_model.sql`, 123 lines: `daily_snapshots`, `regression_events`, `lesson_briefs`, `coach_notes`, `publish_audit`, `unit_reports`, `weekly_reports`, with indexes. Additive only: no existing table is touched. Every column has an inline comment.
- `internal/db/migrate_007_test.go`, new.
- Existing tests updated for the migration count (6 → 7): `db_test.go` (`TestMigrateEmptyAndIdempotent`, `TestTables`), `learning_test.go` (`TestMigration002AppliesOnTopOf001`), and two version assertions in `migrate_006_test.go`.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestMigrate007AppliesOnEmptyDatabase`, `TestMigrate007AppliesOnSeededDatabaseAndLeavesExistingTablesAlone` (every existing table's rows and `sqlite_master` entries unchanged). |
| ac-2 | PASS | `TestMigrate007DailySnapshotsRejectDuplicate`. |
| ac-3 | PASS | `TestMigrate007CoachNoteBodyLimit` (500 accepted, 501 rejected). |

The orchestrator verified on merged `v3` at `dd61bc9`: `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` all packages ok. It did not read the test bodies or run the migration by hand.

## Decisions and deviations

- **Real-date checks.** `day` and `week_start` must parse as real dates. The first attempt (`date(x) = x`) let `2026-13-45` through, because the comparison yields NULL and a CHECK accepts NULL. The tests caught it and `date(x) IS NOT NULL` was added.
- **`typeof(...) = 'text'`** on capped text columns, so a blob cannot get past `length()`.
- **Caps the spec did not give** (chosen by the worker, easy to change): `coach_notes.body` 1 to 500 (empty rejected), `coach_notes.author` 1 to 100, `unit_reports.reason` 1 to 100, `unit_reports.note` up to 500, `lesson_briefs.theme` 1 to 200.
- **Brief id.** Text of 22 to 64 characters. This fits base64url (22) or hex (32) and rules out counters. SPEC-019 picks the encoding.
- **Foreign keys.** Learner data cascades from `users`. `unit_reports.unit_id` cascades from `units`. `publish_audit.unit_id`, `publish_audit.brief_id` and `lesson_briefs.used_by_unit_id` use `SET NULL`, so audit rows survive a unit delete. `regression_events.lexeme_id` is `RESTRICT`, matching `attempts`.
- **Extra constraints.** `to_state <> from_state`, `revision >= 1`, `retention` in 0 to 1 or null, JSON columns checked with `json_valid`.
- **Not enforced.** `week_start` need not be a Monday.
- **`PRAGMA foreign_key_check`** at the end is advisory only, as in 006.
- **No application code** reads or writes these tables. That belongs to SPEC-013, 014, 017, 019, 020, 021 and 023.
- **No personas, timer, trace or independent AC verifier** (as in earlier specs). `reversibility_class` is `revertible`, since the migration only adds objects.

## Gates

- **`db_migration`: open for the real-database check.** The first run on a copy of a real database is still pending. This migration is additive, so the risk is lower than 006, but the gate covers the whole 006 + 007 sequence. Run both on a copy before any deploy.
- **Do not push `v3`.** A push publishes an image that may auto-deploy.
