# SPEC-003 completion summary

**Spec:** SPEC-003 · Data: Migration 006, plan revisions and field marks  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code and tests done; the real local database is not migrated, by instruction)

## What shipped

- **`server/db/migrations/006-plan-revisions.js`** (`VERSION = 6`, `up(db)`, exported `down(db)`, same file shape as 005). The runner (`server/db/index.js` `migrate()`) picks it up by filename and wraps `up()` in one transaction. No runner change was needed.
  - `plan_revisions`: the columns, primary key `(plan_id, revision)`, CHECKs (`actor_kind`, `op`, `revision >= 1`, summary length at most 500, SHA-256 length 64), foreign key to `plans(id)`, and index `idx_plan_revisions_plan_created (plan_id, created_at)`.
  - `plan_field_marks`: the columns, primary key `(plan_id, path, field)`, CHECKs (`last_actor_kind`, `previous_writer_kind` including `unknown`, revision floors, `previous_value_json` at most 32,768 bytes), foreign key to `plans(id)`, and index `idx_plan_field_marks_last_actor (plan_id, last_actor_kind)`.
  - **Foreign-key convention** (header states it): 002 declares none; 005 uses `ON DELETE CASCADE` for learner rows. Both new tables use `REFERENCES plans(id) ON DELETE CASCADE`. Nothing deletes a plan today.
  - **No column or table named `pinned`, `locked`, `pin`, or `lock`.**
  - `down(db)` drops both indexes and both tables and removes the version-6 row. `plans` and `audit_log` are never named.
- **`test/plan-revisions-migration.test.js`** (10 tests), on a temporary `DATA_DIR`. It brings a temp database to version 5 by hand, seeds `plans` and `audit_log` rows, then lets the real runner apply 006.
- **`test/db.test.js`**: one added assertion for version 6 and the version range raised from 5 to 6. The old test would have failed on any sixth migration.

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 | Pass. Columns compared by name, type, not-null, and primary-key position; both index definitions compared; no column name or CREATE sql contains `pin` or `lock`. |
| ac-2 | Pass. Every pre-existing table's columns, CREATE sql, row count, and a SHA-256 of all rows (`plans` with 2 rows and `audit_log` with 1 row included) are identical before and after `up`. `schema_migrations` is excluded from that comparison because it gains the version-6 row by design; the runner tests cover it. |
| ac-3 | Pass. `down` removes both tables and indexes and leaves the pre-006 schema and rows identical, and removes the version-6 row. `up` after `down` is clean through the runner. Running the runner twice records one `schema_migrations` row for version 6. A forced failure inside `up()` rolls the tables back and records nothing. |
| ac-4 | Pass. `actor_kind` `robot`, `op` `pin`, `previous_writer_kind` `bot`, a zero revision, an over-long summary, a short hash, and a 32,769-byte previous value fail their CHECKs. A duplicate `(plan_id, revision)` and a duplicate mark key fail the primary key. A row for a missing plan fails the foreign key. All ten `op` values and the `unknown` writer kind are accepted. Deleting a plan cascades. |
| ac-5 | Pass. Sizes below. Production sizes were **not** available: no copy of the production database was supplied, and the live file was never opened. |

## Measured sizes (retention input, initiative §16 Q1)

Compact JSON is what a revision stores; gzip is `node:zlib` defaults.

| Plan (fixture) | Pretty | Compact | Gzip |
|----------------|--------|---------|------|
| `anal-sex-vocab.plan.json` | 18,074 | 11,017 | 3,292 |
| `beginner-unit-1.plan.json` | 16,037 | 7,844 | 1,809 |
| `my-morning.plan.json` | 14,756 | 7,908 | 2,459 |
| `shai-date-indirect-speech.plan.json` | 26,975 | 15,558 | 4,852 |
| `plans-1.1/*.plan.json` (10 files) | 1,434 to 4,860 | 890 to 2,299 | 459 to 800 |

**Worst case at the N9 ceiling (512 KiB = 524,288 bytes):** gzip cannot make a document much larger than its input. For incompressible content a revision is about 524,471 bytes (stored-block overhead). For ordinary repetitive plan text a 512 KiB document gzips to roughly 6 KB in a quick test (not a real plan, so treat it as an upper-bound illustration only). At the fixture sizes, 1,000 revisions of the largest sample plan is about 4.9 MB. The table only grows; no prune was built (the default is keep everything).

## Verification

- `node --test test/plan-revisions-migration.test.js`: 10 tests, 10 pass.
- Full `npm test` with `SRS_SCHEDULER=ladder`: see the SPEC-002 summary for the counts of the shared run.
- Test hygiene: the test sets `DATA_DIR` to a temp directory, and a last test compares size and mtime of `data/hebrew.sqlite` (and `-wal`, `-shm`) before and after. They are unchanged. The test restores `DATA_DIR` and `HEBREW_ROOT`.
- Not done, by instruction: the real local database was **not** migrated and `scripts/backup-db.js` was not run, because nothing touched a real database. The container applies migration 006 on restart. When the operator does that, run `node scripts/backup-db.js` first (the waiver removes the sign-off, not the sensible backup).

## Deviations

- **The `db_migration` gate is waived** (operator, 2026-10-12). Nothing in this spec waited for approval. Leftover text in the initiative, SPEC-017, and the `RSCH-009` addendum still says the gate is kept; it is listed in the SPEC-001 summary.
- `previous_value_json` is bounded in **bytes** (`length(CAST(... AS BLOB))`), as the spec says "32 KiB", not in characters.

## Files

New: `server/db/migrations/006-plan-revisions.js`, `test/plan-revisions-migration.test.js`, this summary.  
Edited: `test/db.test.js` (version 6 assertion), `SPEC-003-data-plan-revisions-and-field-marks.md` (status `in_review`, boxes ticked).  
No commit.

## Provenance

`INIT-008/SPEC-003`
