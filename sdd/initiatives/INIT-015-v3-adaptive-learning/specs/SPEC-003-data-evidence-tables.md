# SPEC-003 · data: Evidence tables and card skills

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-015
title: "Evidence tables and card skills"
domain: data
status: complete
primary_prompt: .claude/agents/database-architect/AGENT.md
supplement_prompts: [.claude/agents/principal-backend-developer/AGENT.md]
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: one_way
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-002]
blocks: [SPEC-004, SPEC-005, SPEC-007]
```

## Description

**Context:** `migrations/001_init.sql` defines `cards.dir TEXT NOT NULL CHECK (dir IN ('he_en', 'en_he'))`. `review_log` holds FSRS reviews. `confusion_misses` (002) and `placement_answers` (003) hold partial evidence. Nothing records what the learner picked or typed for a checked answer, or which words were shown. North-star section 4 defines four skills: listen, read, recall, and spell.

**Scope (in):** `migrations/006_evidence.sql` and the `internal/db` code it needs.

- `attempts`: id, user_id, lexeme_id, skill (listen, read, recall, spell), activity, source (placement, review, lesson, practice), unit_id (nullable), prompt, response, correct (0/1), dont_know (0/1), hint_used (0/1), latency_ms, created_at. Indexes on (user_id, lexeme_id, skill, created_at) and (user_id, created_at).
- `exposures`: id, user_id, lexeme_id, context (story, dialogue, gloss_tap, placement_review, lesson_intro), unit_id, created_at. Index on (user_id, lexeme_id).
- `cards`: rebuild with `skill TEXT NOT NULL CHECK (skill IN ('listen','read','recall','spell'))` in place of `dir`. Map `he_en` to `read` and `en_he` to `recall`. Keep ids, so `review_log` rows still point at the right card. Keep the unique key, which becomes (learner, lexeme, skill).
- Update every `internal/db` query that names `dir`.

**Scope (out):** Learner-model tables (SPEC-004). Writing attempts (SPEC-005).

## Understood (agent's read of the intent)

- [x] und-1: Review history is the most valuable data in the app. The rename may not lose a row or change a card id.
- [x] und-2: `attempts` is append-only evidence. Model state is derived from it, not stored instead of it.

## Acceptance Criteria

- [x] ac-1: Migrating a database seeded with `he_en` and `en_he` cards and review history leaves the same card count, the same ids, `read` and `recall` skills, and an unchanged `review_log` count. — covered: yes (internal/db/migrate_006_test.go)
- [x] ac-2: `attempts` rejects an unknown skill or source through CHECK constraints. — covered: yes (internal/db/migrate_006_test.go)
- [x] ac-3: Every existing `internal/db`, `internal/session`, and `internal/placement` test still passes after the rename. — covered: yes (full go test ./...)
- [x] ac-4: The migration runs inside one transaction. If it is interrupted, the database is left at 005. — covered: yes (TestMigrate006AtomicWhen*, TestMigrate006RunnerRollsBackDDL)

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Placement-seeded cards with `dir` | Same mapping. Placement only seeded `he_en`, so those become `read`. | decided | yes |
| aud-2 | `listen` and `spell` cards for old words | Not created here. SPEC-015 creates them when a word is first asked through that skill. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] `migrations/006_evidence.sql`
- [x] `internal/db` query updates
- [x] Tests: `internal/db/migrate_006_test.go` with a fixture database at 005

## Technical Requirements (domain-specific)

- [x] SQLite table rebuild pattern, adapted: the runner holds foreign_keys on inside a transaction, so review_log is copied to a TEMP table and restored after the drop. `PRAGMA foreign_key_check` is advisory only (the runner discards its rows).
- [x] Use only `modernc.org/sqlite` features. No new dependency.
- [x] Every column carries an inline comment, as in `001_init.sql`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** db_migration. The first run against a real database is on a copy, with the operator told.
- **Blocking checks:** `go test ./internal/db/... ./internal/session/... ./internal/placement/...`.
- **Rollback plan:** restore the pre-migration SQLite file. A down migration is not provided, because the rebuild is one-way.

## Security

- **Sensitivity:** required — triggers matched: migration of learner data.
- **Security acceptance criteria:**
  - [ ] `attempts.response` stores what the learner typed. It is never rendered without escaping and is returned only to that learner's scope (SPEC-021).
- **Review depth:** standard · **focus areas:** data integrity, scope.

## Verification Strategy

- **Claim:** the rebuild keeps every card and review, and the new tables enforce their vocabularies.
- **Check + executor:** mechanized — `go test ./internal/db -run Migrate006`.
- **Pass condition:** the before and after counts match, the card ids match, the skill mapping holds for every row, and inserting a bad skill fails.

## Integration Points

- SPEC-005 writes `attempts` and `exposures`. SPEC-013 reads them.
- **Provenance tag:** `INIT-015/SPEC-003`
