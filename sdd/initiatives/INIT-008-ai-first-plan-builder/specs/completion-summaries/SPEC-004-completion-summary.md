# SPEC-004 completion summary

**Spec:** SPEC-004 · Service: Revision log in the write path; list and get revisions  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code and tests done; the real local database is not migrated, by instruction)

## What shipped

- **`server/db/repos/plan-revisions.js`** (SQL only, prepared statements): `append(row)`, `has(planId, revision)`, `get(planId, revision)`, `list(planId, opts)`, `latest(planId)`, `sha256(text)`. The document is gzipped with `node:zlib` on write and stored with the SHA-256 of its UTF-8 text. `get` inflates, verifies the hash, and returns `{ error: 'revision_corrupt' }` for a bad hash or bad gzip bytes. `list` returns metadata only, newest first, with `sinceRevision`, `beforeRevision`, `actorKind`, and `pathPrefix` filters (a `substr(path, 1, length(?)) = ?` compare, never `LIKE`). The file has no UPDATE, no DELETE, and no REPLACE.
- **`server/plans/revisions.js`**:
  - `record(entry, ctx)` appends one row, and first a `baseline` row (actor `system`, name `baseline`, the pre-write document) when the plan has no row for its current revision.
  - `list(planId, { sinceRevision, beforeRevision, actorKind, pathPrefix, limit }, ctx)` returns `{ revisions: [{ revision, op, path, field, summary, actorKind, actorName, tool, client, at }], truncated }`. Default limit 50, cap 200, one extra row read to set `truncated`. A bad filter is `invalid_op`; an unknown plan is `not_found`.
  - `get(planId, revision, ctx)` returns the same metadata plus the parsed `document`, or `not_found` or `revision_corrupt`.
  - `summarize(op, path, field)` makes the value-free summary (at most 200 characters, for example `set_field units/u/lessons/l#title`).
- **`server/plans/write-log.js`**: `logWrite({ planId, before, after, afterText, op, path, field, summary }, ctx)` writes the revision row, the SPEC-006 marks, and the `plan.*` audit row. It is called inside the same `db.transaction` as the working-copy write by `ops.persist` (the six operations and restore) and `publishPlan` (and, in SPEC-007, `createPlan`). If any part throws, everything rolls back.
- **Audience rule:** an `ai` caller sees a `human` row's `actorName` as `human`; an admin or script caller sees the account name.
- `ops.persist` and `publish.js` now go through `logWrite`; the audit rows they wrote before are byte-for-byte the same (the existing audit tests pass unchanged apart from the `seed()` note below).

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 | Pass. All seven operations (`set_field`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `assign`, `publish`) through the real functions: one new row each, revision equal to the plan's new revision, matching `op`, actor kind and name, `tool`, a document equal to the stored working copy, and a revision-row count equal to the audit-row count. |
| ac-2 | Pass. A plan with no log has two rows after one edit (baseline at revision 1, the edit at 2); the second edit adds one row. |
| ac-3 | Pass. Conflict, validation, `id_mismatch`, `assign_required` add no row. A stubbed failure of the revision insert (and separately of the audit insert) rolls back the working copy, the snapshot, the audit row, the marks, and the baseline. |
| ac-4 | Pass. A source search finds no UPDATE, DELETE, or REPLACE in the two files; no summary contains the written value. |
| ac-5 | Pass. Order, all four filters, default 50, cap 200 with `truncated`, `_` and `%` as literals, `get` equal to the stored document, `not_found`, and `revision_corrupt` for a bad hash and for bad bytes. |
| ac-6 | Pass. With an `ai` caller `example-admin` appears nowhere in the list or get result; an admin and a script caller get the name. |
| ac-7 | Pass. Below. |

## Growth record (initiative §16 Q1)

100 scripted `set_field` edits (alternating AI and admin) to one plan, full document gzipped per revision, 101 rows each (100 edits plus the baseline). Measured by `test/plan-revisions.test.js`; sizes in characters and bytes.

| Plan | Compact JSON | Stored, 101 rows | Per revision | 1,000 revisions |
|------|--------------|------------------|--------------|-----------------|
| `beginner-unit-1.plan.json` | 6,532 chars | 183,875 bytes | about 1,821 bytes | about 1.7 MiB |
| `shai-date-indirect-speech.plan.json` (largest fixture) | 11,774 chars | 491,318 bytes | about 4,865 bytes | about 4.6 MiB |

Every revision stores the whole document, so the cost per revision does not depend on the size of the edit. No prune was built (the default is keep everything). Production sizes were not available.

## Tests

`test/plan-revisions.test.js`: 15 tests, 15 pass. New helper `test/helpers/plan-db.js` (temporary `DATA_DIR`, migrate, fresh module cache, `ai`/`human`/`system` contexts).  
Full `npm test` with `SRS_SCHEDULER=ladder`: **1242 tests, 1242 pass, 0 fail** (the 1155 baseline plus 87 new tests across SPEC-004 to SPEC-007).

## Existing tests changed

`test/plan-routes.test.js` and `test/plan-audit.test.js`: their `seed()` now clears the plan's `plan_revisions` and `plan_field_marks` rows first. They re-seed one plan back to revision 1 many times, and the old log rows for revision 2 collided with the next write. Nothing else in them changed.

## Things to know

- **A revision that goes backwards collides.** The log's primary key is `(plan_id, revision)`. A script that rewinds a plan's revision (none exists; only tests did) would make the next write fail loudly and roll back. `scripts/import-lesson-plans.js` and `scripts/seed-starter-content.js` only create missing plans and then use the operations.
- **No migration here** (SPEC-003). Deleting a plan removes its log through the foreign-key cascade.
- The real local database was not opened. The container applies migration 006 on restart; run `node scripts/backup-db.js` first.

## Files

New: `server/db/repos/plan-revisions.js`, `server/plans/revisions.js`, `server/plans/write-log.js`, `test/plan-revisions.test.js`, `test/helpers/plan-db.js`, this summary.  
Edited: `server/plans/ops.js`, `server/plans/publish.js`, `test/plan-routes.test.js`, `test/plan-audit.test.js`, the SPEC-004 file (status `in_review`, boxes ticked, executor notes).  
No commit.

## Provenance

`INIT-008/SPEC-004`
