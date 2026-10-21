# SPEC-004 · data: Learner-model tables

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-015
title: "Learner-model tables"
domain: data
status: complete
primary_prompt: .claude/agents/database-architect/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: revertible
security_review: standard
estimated_effort: 3 hours
blocked_by: [SPEC-003]
blocks: [SPEC-013, SPEC-020]
```

## Description

**Context:** The learner model (SPEC-013) is derived from `attempts`, `exposures`, and `cards`. It still needs somewhere to keep daily history, events, briefs, notes, and the audit trail that D5 relies on.

**Scope (in):** `migrations/007_learner_model.sql`. Each table is additive.

- `daily_snapshots`: user_id, day (YYYY-MM-DD), skill, state, count, est_known, retention. Primary key (user_id, day, skill, state).
- `regression_events`: id, user_id, lexeme_id, skill, from_state, to_state, day, cause.
- `lesson_briefs`: id (text, a random 128-bit id), user_id, theme, payload_json, created_at, used_by_unit_id.
- `coach_notes`: id, user_id, author (client name), body (≤ 500 chars), created_at.
- `publish_audit`: id, unit_id, revision, scope, actor, action (auto_publish, unpublish, report_hide), brief_id, issues_json, created_at.
- `unit_reports`: id, unit_id, user_id, reason, note, created_at.
- `weekly_reports`: user_id, week_start (YYYY-MM-DD), payload_json, summary (≤ 600 chars), created_at. Primary key (user_id, week_start).

**Scope (out):** Writing to these tables. That happens in SPEC-013, SPEC-014, SPEC-017, SPEC-019, SPEC-020, SPEC-021, and SPEC-023.

## Understood (agent's read of the intent)

- [x] und-1: Snapshots let Progress draw history even after the state rules change.
- [x] und-2: `publish_audit` is the record that replaces human approval.

## Acceptance Criteria

- [x] ac-1: 007 applies cleanly after 006 on an empty database and on the SPEC-003 fixture. — covered: yes (TestMigrate007AppliesOnEmptyDatabase, ...OnSeededDatabaseAndLeavesExistingTablesAlone)
- [x] ac-2: `daily_snapshots` rejects a second row for the same (learner, day, skill, state). — covered: yes (TestMigrate007DailySnapshotsRejectDuplicate)
- [x] ac-3: `coach_notes.body` longer than 500 characters is rejected by a CHECK constraint. — covered: yes (TestMigrate007CoachNoteBodyLimit)

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Brief id format | Random and unguessable, so a brief id can't be enumerated over MCP. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] `migrations/007_learner_model.sql`
- [x] Tests: `internal/db/migrate_007_test.go`

## Technical Requirements (domain-specific)

- [x] Every column carries an inline comment, as in `001_init.sql`.
- [x] Foreign keys to `users` and `units` with `ON DELETE CASCADE` for learner data. `publish_audit` keeps its rows when a unit is deleted (`ON DELETE SET NULL`).

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** db_migration (additive).
- **Blocking checks:** `go test ./internal/db/...`.
- **Rollback plan:** drop the seven tables. Nothing else depends on them yet.

## Security

- **Sensitivity:** standard — triggers matched: learner data, additive.
- **Security acceptance criteria:**
  - [x] `coach_notes.body` and `unit_reports.note` are plain text, length-capped.
- **Review depth:** quick · **focus areas:** constraints.

## Verification Strategy

- **Claim:** the seven tables exist with their keys and caps.
- **Check + executor:** mechanized — `go test ./internal/db -run Migrate007`.
- **Pass condition:** the migration applies, and the duplicate-snapshot and long-note inserts fail.

## Integration Points

- SPEC-014 writes snapshots and events. SPEC-017 writes unit reports. SPEC-019 writes briefs. SPEC-020 writes the audit. SPEC-021 writes coach notes. SPEC-023 writes weekly reports.
- **Provenance tag:** `INIT-015/SPEC-004`
