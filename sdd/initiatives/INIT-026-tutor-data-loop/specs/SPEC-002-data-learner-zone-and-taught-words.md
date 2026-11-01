# SPEC-002 · data: Migration 020, learner zone, taught words, dated corrections, note author role

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-026
title: "Migration 020: learner zone, taught words, dated corrections, note author role"
domain: data
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: standard
estimated_effort: 2 hours
blocked_by: []
blocks: [SPEC-003]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:** `users` has no zone (`migrations/001_init.sql:5`). `learner_mistakes` records `first_at` and `last_at` as server instants but not the learner's day (`migrations/014_mistakes_verbs.sql:8`). Nothing records a word taught outside the app. `coach_notes` has an author name but no role (`migrations/008_learner_model.sql:63`). The newest migration is `019_noun_forms.sql`.

**Scope (in):**
- `migrations/020_learner_day_and_tutor.sql`. Additive. Header comment in the style of 014: no down migration, and to roll back, restore the pre-migration file.
  - `users.time_zone TEXT`, nullable. CHECK: null, or text of 1 to 64 characters. The app validates the IANA name. The column holds only a validated name.
  - `learner_mistakes.first_on TEXT` and `last_on TEXT`, nullable for old rows. CHECK: null, or `date(x) = x`. These are the learner's calendar day of the first and latest occurrence.
  - `taught_words` table:
    - `id`
    - `learner_id`, references `users` with `ON DELETE CASCADE`
    - `lexeme_id`, references `lexemes` with `ON DELETE RESTRICT`
    - `taught_on`, the learner's day, with CHECK `date(x) = x`
    - `cards_created TEXT NOT NULL DEFAULT ''`: the skills whose cards this row inserted, comma-separated, each one of read, recall, or listen
    - `created_at`, a UTC RFC3339 instant
    - `UNIQUE (learner_id, lexeme_id, taught_on)`, plus an index on `(learner_id, taught_on)`
    - A comment on every column, as in 014
  - `coach_notes.author_role TEXT NOT NULL DEFAULT 'admin' CHECK (author_role IN ('admin', 'learner'))`. Existing notes become `admin`, which they were.
- `internal/db` store calls:
  - `UserTimeZone(ctx, id) (zone string, ok bool, err error)`
  - `SetUserTimeZone(ctx, id, zone string) error`, which refuses an empty value and more than 64 characters
  - `UserTimeZones(ctx) (map[int64]string, error)`, for the jobs. It lists only users with a zone.

**Scope (out):** IANA validation and the day rule (SPEC-003). Writing taught rows and cards (SPEC-005). Tools (SPEC-007, SPEC-008).

## Acceptance Criteria

- [ ] ac-1: A database at 019 migrates to 020 and keeps every row. Old `coach_notes` read `author_role = admin`. Old `learner_mistakes` have null `first_on` and `last_on`.
- [ ] ac-2: Inserting `taught_on = '2026-13-01'`, a duplicate `(learner, lexeme, day)`, or a `time_zone` of 65 characters fails on the constraint.
- [ ] ac-3: Deleting a user deletes their `taught_words`. Deleting a lexeme with a taught row is refused.
- [ ] ac-4: `SetUserTimeZone` then `UserTimeZone` round-trips `America/Denver`, and `UserTimeZones` returns only users that have a zone.
- [ ] ac-5: A fresh database at 020 has the same schema as a migrated one (columns, checks, index).

## Deliverables

- [ ] `migrations/020_learner_day_and_tutor.sql`
- [ ] `internal/db/users_zone.go` (or the existing users file) and test
- [ ] `internal/db/migrate_020_test.go`

## Verification Strategy

- **Claim:** the new columns and table hold only well-formed values, keep every old row, and are owned by their learner.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 -run 'TestMigrate020|TestUserTimeZone' ./internal/db`. Integration tests on a temporary SQLite file, the repo's existing migration-test pattern (`internal/db/migrate_006_test.go`). No coverage tool is wired up, so the bar is one test per AC, each failing on the planted violation it names.
- **Pass condition:** ac-1 to ac-5 pass in that run. Deleting the CHECK on `taught_on` makes ac-2 fail.

## Integration Points

- **Provenance tag:** `INIT-026/SPEC-002`
- `taught_words` is the only new table. No column is removed or rewritten.
