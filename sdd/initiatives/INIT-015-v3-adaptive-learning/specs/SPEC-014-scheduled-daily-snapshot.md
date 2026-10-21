# SPEC-014 · scheduled: Daily snapshot and regression job

## Metadata

```yaml
spec_id: SPEC-014
initiative_id: INIT-015
title: "Daily snapshot and regression job"
domain: scheduled
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-013]
blocks: [SPEC-017, SPEC-018, SPEC-021, SPEC-023]
```

## Description

**Context:** Progress and regression need history. The model (SPEC-013) is computed from the current evidence. v2 runs as one process with no job runner.

**Scope (in):** A new package, `internal/jobs`.

- `Daily.Run(ctx, day)`:
  - For each learner, writes `daily_snapshots` rows (counts per skill and state, est_known, and retention).
  - Compares with the previous snapshot. For each word whose state moved from known or solid to regressed or struggling, it writes `regression_events` with the cause.
  - It is idempotent: a second run for the same day writes nothing new.
- **Band slip:** when a band's share of known words drops 15 points or more since the snapshot 7 days earlier, write a `band_slip` event (a `regression_events` row with `lexeme_id` null).
- **Scheduler:** a goroutine started by `cmd/hebrew serve`. It wakes at 03:30 in the server's time zone. On start, it runs any missed days (up to 14) so idle days are still filled. A process-level lock (a `meta` row) stops two replicas from running at once.
- A CLI command, `hebrew jobs daily --day YYYY-MM-DD`, runs it by hand.

- **Migration 009 (added 2026-10-19 after the worker found a plan defect, operator approved):** `migrations/009_daily_job.sql` adds a `job_state` table (`key` PK, `value`, `updated_at`) for the process lock, per-learner last-day word states, and per-learner band-share history. It rebuilds `regression_events` so `lexeme_id`, `skill`, `from_state`, `to_state` are nullable, the `to_state <> from_state` check applies only when both are set, and a `kind` column (`word` or `band_slip`) exists. The "a `meta` row" lock above means a `job_state` row; no `meta` table exists. Because this is a migration, the first run on a copy of a real database needs the operator (db_migration gate), together with 006 to 008.

**Scope (out):** The weekly report (SPEC-023).

## Understood (agent's read of the intent)

- [x] und-1: "Can track regression, can track progression." That needs a row a day, even on days without study.

## Acceptance Criteria

- [x] ac-1: Running the job twice for the same day leaves the same rows. — covered: yes, see completion summary
- [x] ac-2: After 5 idle days and a restart, snapshots exist for every missed day. — covered: yes, see completion summary
- [x] ac-3: A crafted known-to-struggling word produces one `regression_events` row with its cause. — covered: yes, see completion summary
- [x] ac-4: A fixture with a 20-point drop in band 2 over 7 days writes one `band_slip` event. — covered: yes, see completion summary

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Time zone | `TZ` from the environment, defaulting to `America/Chicago`. The day boundary matches the learner's evening study. | assumed | yes |

> **Readiness** = 100.

## Deliverables

- [x] `internal/jobs/{doc,daily,scheduler}.go`, and the `jobs` subcommand in `cmd/hebrew`
- [x] Tests: `daily_test.go` and `scheduler_test.go` (fake clock)

## Technical Requirements (domain-specific)

- [x] The scheduler stops cleanly on context cancel and never blocks shutdown for more than 5s.
- [x] Each day's writes happen in one transaction per learner.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/jobs/...`.
- **Rollback plan:** disable the scheduler with `HEBREW_JOBS=off`. The rows are harmless.

## Security

- **Sensitivity:** standard — triggers matched: background writes.
- **Security acceptance criteria:**
  - [ ] The CLI runs only against the local data directory. It has no network entry point.
- **Review depth:** quick · **focus areas:** none.

## Verification Strategy

- **Claim:** a snapshot exists for every day, regressions are recorded once, and runs are idempotent.
- **Check + executor:** mechanized — `go test ./internal/jobs/...` with a fake clock.
- **Pass condition:** ac-1 to ac-4 pass.

## Integration Points

- SPEC-018 charts the snapshots. SPEC-021 serves them through `get_progress`. SPEC-023 builds the weekly report from them.
- **Provenance tag:** `INIT-015/SPEC-014`
