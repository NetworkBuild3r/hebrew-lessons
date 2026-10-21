# SPEC-005 · Service: Field-level diff and one-step restore

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-008
title: "Diff two revisions (or working copy against published) by id-addressable node; restore a revision, part, or field as a new revision"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-004]
blocks: [SPEC-006]
```

---

## Description

**Context:** with the revision log (SPEC-004) both actors can read any past document. D2(c) asks for a diff and a **one-step restore** usable by both the admin and the MCP. Plans address parts by id path (ADR-0003 §4): `units/{unit}`, `.../lessons/{lesson}`, `.../blocks/{block}`, and `.../{words|paragraphs|lines|questions}/{id}` under a block (`server/plans/ops.js` `locate`). Validator findings use array-index pointers; the diff uses id paths so an AI can act on what it reads.

**Scope (in):**

- `server/plans/diff.js` — a pure function `diffDocuments(a, b)` returning entries `{ path, field, kind, before, after }`:
  - Walk the addressable nodes of both documents by id: the plan (path `''`), units, lessons, blocks, and bucket items. For each node present in both, compare every non-child field with a stable JSON equality (key order ignored). A difference is `kind: 'changed'` with `before` and `after` (the field's whole JSON value). Arrays that are not id-addressable buckets (for example `newWords`, `rules`, `cells`, `spans`, `highlights`) are one field each, compared whole.
  - A node only in `b` is `added` (`field: ''`, `after` the node, capped); only in `a`, `removed`. A change in the relative order of the nodes common to a sibling list is one `moved` entry per list (`path` the parent, `field` the list name, `before` and `after` the id order).
  - Output is capped at 500 entries and 64 KiB of `before`/`after` text, with `truncated: true`, so a caller can narrow with `path`. A value over 4000 characters is cut with a `…` marker and `valueTruncated: true` on that entry.
- `server/plans/revisions.js` additions:
  - `diff(planId, from, to, { path }, ctx)` where `from` and `to` are an integer revision, `'working'` (the current working copy), or `'published'` (the snapshot). A missing snapshot for `'published'` returns `no_snapshot`; a missing revision returns `not_found`. `path` restricts entries to that node and below.
  - `restore(store, planId, { fromRevision, path, field, expectedRevision }, ctx)`:
    - **No path:** the whole document of `fromRevision`, with `id` kept, `revision` set to the stored revision + 1, `status` set to `draft`, **`audience` and `assigneeId` kept from the current working copy** (restore never changes them), and `schemaVersion` the higher of the current and the restored value (a `1.1.0` plan never returns to `1.0.0`, `ops.finish`).
    - **A path:** the node at `path` in `fromRevision` replaces the node at `path` now. If the node no longer exists now but its parent does, it is re-inserted at its former index, clamped to the list length. If the parent is also gone, `not_found` with `reason: 'parent_missing'`. If the node did not exist in `fromRevision` (it was added later), `not_found` with `reason: 'absent_in_revision'` (undo an insert with `remove_block`).
    - **A path and a field:** that one field is set to its value in `fromRevision`; if the field was absent then, it is removed now. On the plan path, a field of `audience` or `assigneeId` returns `assign_required`.
    - It goes through the **same** `mutate` path as every write: stale `expectedRevision` is `revision_conflict`; the result is validated and refused with the same `WRITE_BLOCK_RULES`; one new revision row (op `restore`, summary `restore <path>#<field> from r<N>`), one audit row `plan.restore`, one revision increment. Both actors may call it. It never edits or deletes history.
    - The result carries `restoredFrom: N` and the new `revision`.

**Scope (out):** marks and `editState` (SPEC-006); tools and routes (SPEC-009, SPEC-010); any UI (SPEC-014).

---

## Understood (agent's read of the intent)

- [x] und-1: Restore is a new write, so it is itself in history and can itself be restored. There is no "undo undo" special case.
- [x] und-2: "One step" means one call and one new revision, with no confirmation round-trip in the service. The admin UI may ask the person to confirm before it calls (SPEC-014); the service never does.
- [x] und-3: A restore that would produce a document the validator refuses is refused with the validation error, never written.

## Acceptance Criteria

- [x] ac-1 (REQ-005): `diffDocuments` on two fixture plans reports a changed `meaning` as one `changed` entry with the word's id path, `field: 'meaning'`, and both values; an added block as `added`; a removed word as `removed`; a reordered block list as one `moved`; an unchanged plan as an empty list; key order differences as no change.
- [x] ac-2 (REQ-005): `diff` between two revisions, `'working'` against `'published'`, and an integer against `'working'` each return the expected entries on a plan edited by three writes; `'published'` with no snapshot returns `no_snapshot`; an unknown revision returns `not_found`; `path` narrows the result.
- [x] ac-3 (REQ-005): the 500-entry and size caps set `truncated`; a 5000-character value is cut with `valueTruncated`.
- [x] ac-4 (REQ-005): after an AI edit then a human edit of one field, `restore` with that field from the AI's revision sets it back, creates exactly one new revision (op `restore`), leaves every other field unchanged, and `list` shows the restore row after the human row. The same call made by an `ai` context and by a `human` context both succeed.
- [x] ac-5 (REQ-005): restoring a whole old revision keeps the current `audience` and `assigneeId`; restoring a `1.0.0` revision onto a `1.1.0` plan keeps `schemaVersion` `1.1.0`; `status` becomes `draft`.
- [x] ac-6 (REQ-005): restoring a removed block re-inserts it at its former index (clamped); with its parent gone returns `parent_missing`; restoring a node absent in the revision returns `absent_in_revision`.
- [x] ac-7 (REQ-005, REQ-002): a field restore of `audience` or `assigneeId` returns `assign_required`; a stale `expectedRevision` returns `revision_conflict` and writes nothing; a restore whose result fails a write-time block rule returns the validation error and writes nothing and adds no revision, audit, or mark row.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Diff granularity | Id-addressable nodes plus their non-child fields; nested non-bucket arrays compared whole. A finer array diff is not built. | resolved | yes — keeps the output addressable by an AI and by the editor |
| aud-2 | Restore and visibility | A restore may change a lesson's `visibility` (it is a lesson field), exactly as any write may. It is audited and itself restorable. | resolved | yes — D1 (no gate); recorded as the accepted residual risk |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/plans/diff.js`
- [x] Changes to `server/plans/revisions.js`
- [x] Tests: `test/plan-diff.test.js`, `test/plan-restore.test.js`

---

## Technical Requirements (domain-specific)

- [x] `diffDocuments` is pure: no I/O, no mutation of its inputs.
- [x] Restore reuses `ops.mutate`'s open, validate, persist path. It does not edit JSON in an adapter and does not write SQL outside the repos.
- [x] Temporary database in tests. Fixtures use `example-admin`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** the two test files and `npm test`.
- **Rollback plan:** revert the commit; no schema or data changed.

---

## Security

- **Sensitivity:** standard. Triggers matched: a write that replaces live content from old content.
- **Review depth:** standard · **focus areas:** restore cannot change `audience` or `assigneeId`; restore runs the same validation and audit as any write.

---

## Verification Strategy

- **Claim:** both actors can see exactly what changed and put any part or field back in one call, with the same validation and audit as any write, and history stays intact.
- **Check + executor:** `mechanized` — `node --test test/plan-diff.test.js test/plan-restore.test.js`, then `npm test`.
- **Pass condition:** exit 0; ac-1 through ac-7 each asserted; ac-4 reads the revision list to confirm no row was altered or removed.
- **Test levels:** unit for `diffDocuments` (property-style: `diffDocuments(a, a)` is empty for every fixture); integration on a temporary database for `diff` and `restore`.

---

## Integration Points

- SPEC-006 reuses `diffDocuments` to compute which fields a write changed. SPEC-009 and SPEC-010 expose `diff` and `restore`. SPEC-014 renders the entries.
- Produces **Change:** diff and restore.
- **Provenance tag:** `INIT-008/SPEC-005`

---

## Executor notes (2026-10-12)

- `diffDocuments(a, b, options)` returns `{ entries, truncated }` (an object, because the caps need somewhere to report), and accepts `{ path, full }`. `full: true` turns the caps off; SPEC-006's marks use it. The plan's own `revision` counter is never reported; every other plan field, `status` included, is. A field that is new has no `before` key and a removed field has no `after` key. A list whose items lack distinct ids is compared as one field, so a malformed document cannot throw.
- Restore is one more internal op (`restore`) inside `ops.apply`, reached only through `revisions.restore`; `ops` now also exports `mutate`, `refusesWrite`, and `validationError`. `not_found` results carry a `reason` (`parent_missing`, `absent_in_revision`, `node_missing`) and the `path`.
- A restore sets `status` to `draft` and leaves the published snapshot alone, so learners keep reading the last published snapshot until the next publish.
