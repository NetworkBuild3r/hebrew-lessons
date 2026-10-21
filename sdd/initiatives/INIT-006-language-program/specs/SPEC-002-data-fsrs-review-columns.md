# SPEC-002 · Data: FSRS columns on review rows and graded events

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-006
title: "Additive FSRS state on review_items and a grade on practice_events"
domain: data
status: complete
primary_prompt: .claude/agents/database-architect/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: one_way
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004]
```

---

## Description

**Context:** `review_items` stores a ladder `step`, `streak`, `lapses`, `last_result`, `last_at` (`server/db/migrations/001-initial.js:42`–`:58`). `practice_events` stores `result` only (`:60`–`:73`). The migration runner applies files in `server/db/migrations/` in order and records them in `schema_migrations` (`server/db/index.js:105`–`:139`). ADR-0009 §2 is the contract.

**Scope (in):**

- Next free migration number at execution time (INIT-004 may have taken some). Name `<n>-fsrs-review-state.js`.
- `review_items` new nullable columns: `stability REAL`, `difficulty REAL`, `reps INTEGER NOT NULL DEFAULT 0`, `fsrs_state TEXT` with `CHECK (fsrs_state IN ('new','learning','review','relearning'))`, `last_grade INTEGER` with `CHECK (last_grade BETWEEN 1 AND 4)`, `suspended INTEGER NOT NULL DEFAULT 0 CHECK (suspended IN (0,1))`.
- `practice_events` new nullable columns: `grade INTEGER CHECK (grade BETWEEN 1 AND 4)`, `kind TEXT NOT NULL DEFAULT 'word' CHECK (kind IN ('word','form'))`, `lemma TEXT`.
- Backfill in the same migration, per ADR-0009 §2: rows never passed get `fsrs_state='new'`, `stability NULL`; rows with a pass get `stability = [1,3,7,14,30][step]`, `difficulty = 5 + min(lapses, 5) * 0.5`, `reps = streak + lapses`, `fsrs_state='review'`. Ladder columns are not rewritten.
- Repository additions in `server/db/repos/review.js`: read and write the new columns with an explicit column list; `setSuspended(userId, word, bool)` for both directions.
- Down: a second exported `down(db)` that rebuilds the two tables without the new columns (SQLite drop-column where supported, else copy-rename), keeping every original row and value.

**Scope (out):** the scheduler math (SPEC-004); which rows the API shows (SPEC-009).

---

## Understood (agent's read of the intent)

- [x] und-1: Additive only. A learner's existing schedule (`due`) is not changed by the migration.
- [x] und-2: "Ever passed" uses the same test `metrics()` already uses (`server/progress.js:264`–`:268`).

## Acceptance Criteria

- [x] ac-1 (REQ-002): on a fixture database with rows at steps 0–4 and some never passed, the `new` / `learning` / `longTerm` counts from `metrics()` before migration equal the counts from the ADR-0009 stage mapping after migration, per direction.
- [x] ac-2 (REQ-002): every pre-existing `due`, `step`, `streak`, `lapses`, `last_result`, `last_at`, `sources` value is byte-identical after migration.
- [x] ac-3 (REQ-016): `down` restores the original column sets with every row intact; `up` after `down` is clean.
- [x] ac-4: inserting a `grade` of 0 or 5, or a `kind` other than `word`/`form`, fails the CHECK.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Down migration support | The runner has no down step today. This spec exports `down` and tests it directly; wiring a CLI is out of scope. | resolved | yes — rollback is SPEC-020's backup plus this function |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/db/migrations/004-fsrs-review-state.js`
- [x] Changes to `server/db/repos/review.js` and `server/db/repos/events.js`
- [x] Tests: `test/fsrs-migration.test.js`

---

## Technical Requirements (domain-specific)

- [x] Bound parameters only. Backfill in one transaction.
- [x] Temporary database in tests. No test touches `data/hebrew.sqlite`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** `db_migration` — operator approves before the migration file is added to the live runner path.
- **Blocking checks:** `node --test test/fsrs-migration.test.js` and `npm test`.
- **Rollback plan:** `down(db)`; SPEC-020's dated backup.

---

## Security

- **Sensitivity:** standard. No new learner data class; new columns are derived scheduling state.
- **Review depth:** standard · **focus areas:** CHECK constraints, no data loss.

---

## Verification Strategy

- **Claim:** the migration adds FSRS state without changing any learner's due dates or stage counts, and is reversible.
- **Check + executor:** `mechanized` — `node --test test/fsrs-migration.test.js`.
- **Pass condition:** exit 0; ac-1 counts equal; ac-2 values compared column by column; ac-3 round trip.
- **Test levels:** integration on a temporary database.

---

## Integration Points

- SPEC-004 reads and writes the new columns. SPEC-009 uses `suspended`. SPEC-006 reads `grade`, `kind`, `lemma`.
- Produces **Change:** FSRS columns.
- **Provenance tag:** `INIT-006/SPEC-002`
