# SPEC-003 · Data: Migration 006 — plan revisions and field marks

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-008
title: "One additive migration: plan_revisions (history) and plan_field_marks (who last wrote a field)"
domain: data
status: complete
primary_prompt: .claude/agents/database-architect/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: one_way
security_review: standard
estimated_effort: 3 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004]
```

---

## Description

**Context:** the plan store is `plans` with `working_copy`, `snapshot`, and one integer `revision` (`server/db/migrations/002-plan-store.js`; `server/plans/store.js`). It keeps no history, and nothing records who wrote a field. Migrations 001–005 exist; the runner applies files in `server/db/migrations/` in order and records them in `schema_migrations`. ADR-0011 (SPEC-001) fixes the tables. The plan document and its schema are not changed: they reject unknown properties and learners receive the snapshot, so provenance belongs beside the plan, not in it.

**This is the one schema change in INIT-008.** The operator waived the `db_migration` gate on 2026-10-12 (local-only project, no other users). Back up the database before the migration anyway; it is cheap.

**Scope (in):**

- Next free migration number at execution time; expected `server/db/migrations/006-plan-revisions.js`. Re-scan the directory immediately before writing. Match the file shape of `005-lexicon-library-mastery.js` (`VERSION`, `up(db)`, exported `down(db)`).
- Table `plan_revisions`:
  - `plan_id TEXT NOT NULL`, `revision INTEGER NOT NULL`, `PRIMARY KEY (plan_id, revision)`.
  - `actor_kind TEXT NOT NULL CHECK (actor_kind IN ('ai','human','system'))`, `actor_name TEXT NOT NULL`, `tool TEXT`, `client TEXT`, `request_id TEXT`.
  - `op TEXT NOT NULL CHECK (op IN ('create','baseline','set_field','replace_part','insert_block','move_block','remove_block','assign','publish','restore'))`.
  - `path TEXT`, `field TEXT`, `summary TEXT NOT NULL` (value-free, length-capped), `document_gz BLOB NOT NULL`, `document_sha256 TEXT NOT NULL`, `created_at TEXT NOT NULL`.
  - A foreign key to `plans(id)` with the same `ON DELETE` behavior the existing tables use (follow 002/005's convention; state it in the file header).
  - Index on `(plan_id, created_at)` for listing.
- Table `plan_field_marks`:
  - `plan_id TEXT NOT NULL`, `path TEXT NOT NULL`, `field TEXT NOT NULL`, `PRIMARY KEY (plan_id, path, field)`.
  - `last_actor_kind TEXT NOT NULL CHECK (last_actor_kind IN ('ai','human','system'))`, `last_actor_name TEXT NOT NULL`, `last_revision INTEGER NOT NULL`, `last_at TEXT NOT NULL`, `value_hash TEXT NOT NULL`.
  - `human_revision INTEGER`, `human_at TEXT`, `human_actor_name TEXT` — the last time a person wrote this field, kept even after an AI later overwrote it.
  - `previous_value_json TEXT` (the value the most recent human edit replaced; NULL when over 32 KiB), `previous_writer_kind TEXT CHECK (previous_writer_kind IN ('ai','human','system','unknown'))`.
  - Foreign key to `plans(id)`; index on `(plan_id, last_actor_kind)`.
- **No column or table named `pinned`, `locked`, `pin`, or `lock`.** (D2.)
- `down(db)`: drops both tables and their indexes. `plans` and `audit_log` are never altered.
- **Measure before SPEC-004 (retention input, initiative §16 Q1).** Record in the completion summary: the current byte size of each fixture plan, its gzip size via `node:zlib`, and the worst case at the N9 ceiling (512 KiB). If the operator supplies a **copy** of the production database, the same numbers for the real plans; the live file is never opened by this spec.

**Scope (out):** writing rows (SPEC-004, SPEC-006); any change to `plans`, `audit_log`, or the plan schema; a prune function; applying the migration to the real `DATA_DIR` (operator step, initiative §14).

---

## Understood (agent's read of the intent)

- [x] und-1: Additive only. No existing row of any table changes. Plans that exist today have no log rows until their first post-migration write (SPEC-004's baseline).
- [x] und-2: The document is stored gzipped as a BLOB so a revision costs a fraction of N9's ceiling; the hash lets a reader verify an inflated copy.
- [x] und-3: `previous_value_json` exists so `editState` can show what the person replaced without reading old revisions, and is bounded so a large value cannot bloat the table.

## Acceptance Criteria

- [x] ac-1 (REQ-016): on a temporary database migrated through 005, applying 006 creates `plan_revisions` and `plan_field_marks` with exactly the columns, primary keys, CHECK constraints, and indexes above. `PRAGMA table_info` for both tables lists no column whose name contains `pin` or `lock`.
- [x] ac-2 (REQ-016): every pre-existing table's `PRAGMA table_info` and every pre-existing row (counted and hashed per table, `plans` and `audit_log` included) are identical before and after `up`.
- [x] ac-3 (REQ-016): `down` removes both tables and indexes and leaves the pre-006 schema and rows identical; `up` after `down` is clean; applying 006 twice through the runner records one `schema_migrations` row.
- [x] ac-4: inserting `actor_kind` `'robot'`, an `op` outside the list, or `previous_writer_kind` `'bot'` fails its CHECK; a second row with the same `(plan_id, revision)` fails the primary key; a row for a missing plan fails the foreign key (or follows the convention the header states).
- [x] ac-5: the completion summary lists the measured sizes and the worst case named above, and says whether production sizes were available.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Migration number | 006: 001–005 exist as of 2026-10-12. Re-scan before writing. | resolved | yes — directory scan |
| aud-2 | Foreign-key and cascade behavior | Follow the convention in 002 and 005; if neither uses `ON DELETE`, use none and say so in the file header. | resolved | yes |
| aud-3 | Retention | No prune column or job. Default keep everything (initiative §16 Q1). | ambiguous | no — planning default applied; a retention rule is a later migration if the operator wants one |

> **Readiness** = 100 against the stated defaults.

---

## Deliverables

- [x] `server/db/migrations/006-plan-revisions.js`
- [x] Tests: `test/plan-revisions-migration.test.js`

---

## Technical Requirements (domain-specific)

- [x] Bound parameters only. `up` in one transaction.
- [x] Temporary database in tests. No test opens `data/hebrew.sqlite` or any `DATA_DIR`.
- [x] No new dependency; `node:sqlite`; `node:zlib` only for the measurement.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none. The `db_migration` gate was waived by the operator on 2026-10-12. Run `scripts/backup-db.js` first against any real database; this spec itself touches only temporary ones.
- **Blocking checks:** `node --test test/plan-revisions-migration.test.js` and `npm test`.
- **Rollback plan:** `down(db)`; the dated backup from `scripts/backup-db.js` (initiative §14).

---

## Security

- **Sensitivity:** standard. Plan text is stored in a second place (a gzipped copy per revision). It holds nothing a plan may not already hold (schema `html` rule, N9–N11). No learner data, no secret.
- **Review depth:** standard · **focus areas:** CHECK constraints, foreign keys, no data loss.

---

## Verification Strategy

- **Claim:** the migration adds the history and provenance tables without changing any existing table or row, and is reversible.
- **Check + executor:** `mechanized` — `node --test test/plan-revisions-migration.test.js`.
- **Pass condition:** exit 0; ac-1 column lists compared by name and type; ac-2 per-table row hashes equal; ac-3 round trip; ac-4 each constraint rejects.
- **Test levels:** integration on a temporary database. The test-data pattern is the one in `test/fsrs-migration.test.js` and `test/plan-store.test.js`.

---

## Integration Points

- SPEC-004 appends `plan_revisions` rows. SPEC-006 upserts `plan_field_marks` rows. SPEC-017's runbook applies this migration to the real database.
- Produces **Change:** migration 006.
- **Provenance tag:** `INIT-008/SPEC-003`
