# SPEC-004 · Service: Revision log in the write path; list and get revisions

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-008
title: "Write one revision row with every plan write; list and read revisions"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-002, SPEC-003]
blocks: [SPEC-005, SPEC-006, SPEC-007]
```

---

## Description

**Context:** after SPEC-002 every plan write has a context and writes one audit row inside the same transaction. After SPEC-003 the tables `plan_revisions` and `plan_field_marks` exist. This spec fills `plan_revisions` from the one place every write goes through, and reads it back. ADR-0011 Decision 2 is the contract.

**Scope (in):**

- `server/db/repos/plan-revisions.js` — SQL only, prepared statements: `append(row)`, `get(planId, revision)`, `list(planId, { sinceRevision, actorKind, pathPrefix, limit, beforeRevision })`, `latest(planId)`. The document is gzipped with `node:zlib` `gzipSync` on write, inflated on read, with `document_sha256` of the uncompressed UTF-8 text; a read verifies the hash and returns `{ error: 'revision_corrupt' }` on a mismatch.
- `server/plans/revisions.js` — the service: `record(txnCtx, { row, before, after, op, path, field }, ctx)`, `list(planId, opts, ctx)`, `get(planId, revision, ctx)`.
- **Write path.** `persist()` in `server/plans/ops.js` and the transaction in `server/plans/publish.js` call `revisions.record` **inside the same `db.transaction`** as `saveWorkingCopy` and the SPEC-002 audit row. One row per successful write: op `set_field`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `assign`, `publish`. `revision` is the plan's new revision integer. `summary` is value-free and at most 200 characters, for example `set_field units/u/lessons/l#title`; it never contains a field value or plan text.
- **Lazy baseline.** Before the first logged write of a plan that has no row for its **current** revision, append one `baseline` row (actor `system`, name `baseline`) holding the pre-write document at its current revision, in the same transaction. After that, history from the plan's first post-migration edit is complete and a restore to "before the edit" works. Plans created by scripts before migration 006 have no earlier history.
- **List and get.** `list` returns newest first: `{ revisions: [{ revision, op, path, field, summary, actorKind, actorName, tool, client, at }], truncated }` with a default limit 50, a maximum 200, and filters `sinceRevision`, `actorKind`, `pathPrefix` (a prefix compared by string `substr`, never `LIKE` with an unescaped id). `get` returns the same metadata plus `document` (the parsed plan as it was at that revision). **Audience:** when the caller's context actor kind is `ai`, `actorName` for a `human` row is returned as `human` and the account username is omitted; an admin caller receives the name. (initiative §16 A2.)
- **Growth record.** Measure and record the stored size of the new rows from a scripted run of 100 edits to one fixture plan and to the largest fixture; state the per-revision size and a projection at 1,000 revisions in the completion summary (initiative §16 Q1).

**Scope (out):** diff and restore (SPEC-005); marks (SPEC-006); tools and routes (SPEC-009, SPEC-010); pruning.

---

## Understood (agent's read of the intent)

- [x] und-1: History is append-only. No function in this spec updates or deletes a `plan_revisions` row.
- [x] und-2: A write that is refused (conflict, validation, `id_mismatch`, `assign_required`) writes no revision row, exactly as it writes no audit row.
- [x] und-3: The log is a copy, not the source of truth. `plans.working_copy` stays the live document; the log is for history, diff, and restore.

## Acceptance Criteria

- [x] ac-1 (REQ-004): after each of the seven write operations through the real adapters' functions, exactly one new `plan_revisions` row exists with `revision` equal to the plan's new revision, the matching `op`, actor kind and name from the context, the `tool`, and a `document_gz` that inflates to the stored working copy. The row count equals the audit row count for the same writes.
- [x] ac-2 (REQ-004): a plan with no log rows, edited once, ends with two rows: a `baseline` row at the old revision holding the pre-edit document, and the edit's row at the new revision. A second edit adds one row, not another baseline.
- [x] ac-3 (REQ-004): a refused write adds no row; a stubbed failure of the revision insert rolls back the working-copy write and the audit row (no mutation without its row, and no row without its mutation).
- [x] ac-4 (REQ-004): no function in `server/db/repos/plan-revisions.js` or `server/plans/revisions.js` issues an `UPDATE` or `DELETE` on `plan_revisions` (asserted by a source search in the test), and `summary` never equals or contains the new value written in the test.
- [x] ac-5 (REQ-005): `list` returns newest first, honors `sinceRevision`, `actorKind`, `pathPrefix`, and `limit` (default 50, capped at 200, `truncated` true when more exist); `get` returns the document at that revision byte-for-byte equal (after parse) to the document saved then; `get` of an unknown revision returns `not_found`; a row whose stored hash no longer matches returns `revision_corrupt`.
- [x] ac-6 (REQ-003, REQ-005): with an `ai` caller, a `human` revision lists `actorName: 'human'` and the username `example-admin` appears nowhere in the result; with an admin caller the name is returned.
- [x] ac-7: the completion summary records per-revision stored size for the two scripted runs and the 1,000-revision projection.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Publish rows | Publish is one revision row, op `publish`, at the revision it creates. The snapshot is identified by that revision. | resolved | yes — `publishPlan` bumps the revision and stores the same document as the snapshot |
| aud-2 | `LIKE` and ids | A `pathPrefix` filter uses `substr(path, 1, length(?)) = ?`, so a `_` or `%` in an id is never a wildcard. | resolved | yes |
| aud-3 | Retention | Keep everything; growth recorded, not pruned (initiative §16 Q1). | ambiguous | no — planning default applied |

> **Readiness** = 100 against the stated defaults.

---

## Deliverables

- [x] `server/db/repos/plan-revisions.js`, `server/plans/revisions.js`
- [x] Changes to `server/plans/ops.js` and `server/plans/publish.js`
- [x] Tests: `test/plan-revisions.test.js`

---

## Technical Requirements (domain-specific)

- [x] Prepared statements only. Gzip and hash from `node:zlib` and `node:crypto`; no dependency.
- [x] Temporary database in tests. Fixtures use `example-admin`.
- [x] A document read from a row is parsed with `JSON.parse` and never evaluated.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none (the migration was SPEC-003's gate).
- **Blocking checks:** `node --test test/plan-revisions.test.js` and `npm test`.
- **Rollback plan:** revert the commit; the two tables stay unused.

---

## Security

- **Sensitivity:** standard. Triggers matched: stores a copy of plan text; returns admin account names to admins.
- **Review depth:** standard · **focus areas:** atomicity, no value in `summary`, the audience rule for human names.

---

## Verification Strategy

- **Claim:** every successful plan write leaves exactly one retrievable revision, in the same transaction as the write, and a person's name does not reach an AI caller.
- **Check + executor:** `mechanized` — `node --test test/plan-revisions.test.js`, then `npm test`.
- **Pass condition:** exit 0; ac-1 through ac-6 each asserted; the rollback case observes the working copy, audit table, and revision table all unchanged.
- **Test levels:** integration on a temporary database through the operation functions (not by writing rows directly).

---

## Integration Points

- SPEC-005 reads revisions for diff and restore. SPEC-006 shares the same persist transaction for marks. SPEC-007's `createPlan` writes a `create` row. SPEC-009 and SPEC-010 expose `list` and `get`.
- Produces **Change:** the revision log.
- **Provenance tag:** `INIT-008/SPEC-004`

---

## Executor notes (2026-10-12)

- `revisions.record(entry, ctx)` takes no `txnCtx`: `db.transaction` is module-wide, so the caller's open transaction is the only one. One new module, `server/plans/write-log.js` (`logWrite`), is what `persist()`, `publishPlan`, `createPlan`, and restore call inside their transactions; it writes the revision row, the SPEC-006 marks, and the audit row, so no writer can skip a part. The audit call moved there from `ops.persist` and `publish.js`; the rows are unchanged.
- The stored summary is at most 200 characters (the migration allows 500).
- A re-seed that rewinds a plan's revision (only tests do this) collides with its old log rows and fails loudly (the write rolls back). `plan-routes.test.js` and `plan-audit.test.js` now clear a plan's log in their `seed()`. Production code never rewinds a revision.
- Growth numbers are in the completion summary.
