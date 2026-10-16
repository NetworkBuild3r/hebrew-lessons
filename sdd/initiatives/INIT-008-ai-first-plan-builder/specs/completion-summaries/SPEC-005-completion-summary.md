# SPEC-005 completion summary

**Spec:** SPEC-005 · Service: Field-level diff and one-step restore  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code and tests done; SPEC-009 and SPEC-010 will add the tool and route)

## What shipped

- **`server/plans/diff.js`**: `diffDocuments(a, b, { path?, full? })` returns `{ entries, truncated }`. Pure: no I/O, and it accepts deep-frozen inputs. Nodes are the plan, units, lessons, blocks, and the items of a block's `words`, `paragraphs`, `lines`, and `questions`, each addressed by id path exactly as `locate()` in `ops.js` does. Entry kinds: `changed` (`{ path, field, kind, before?, after? }`), `added` and `removed` (`field: ''`, the whole node), and `moved` (one per list whose common nodes changed relative order, with `before` and `after` the ids). Every other array is one field. Caps: 500 entries, 64 KiB of text, a value over 4,000 characters cut with `…` and `valueTruncated: true`; `truncated: true` says the list was cut.
- **`server/plans/revisions.js`** additions:
  - `diff(planId, from, to, { path }, ctx)`: `from` and `to` are an integer revision, `'working'`, or `'published'`. Returns `{ from: { ref, revision }, to: { ref, revision }, entries, truncated }`, or `no_snapshot`, `not_found`, `revision_corrupt`, `invalid_op`.
  - `restore(store, planId, { fromRevision, path?, field?, expectedRevision }, ctx)`: whole document, one part, or one field, as a new revision through the same `ops.mutate` path as any write. The result is the write result plus `restoredFrom`. Both actors may call it.
- **`server/plans/ops.js`**: one more internal op, `restore`, handled by `applyRestore`. `mutate`, `refusesWrite`, and `validationError` are now exported. `not_found` results can carry a `reason`.

## Restore rules, as built

- **Whole document:** `id`, `audience`, and `assigneeId` stay as they are now; `revision` is the stored revision plus 1; `status` becomes `draft`; `schemaVersion` stays `1.1.0` if it is `1.1.0` now. The published snapshot is not touched, so learners keep reading the last published copy.
- **A path:** the node replaces the node at that path; if the node is gone and its parent is not, it goes back at its former index (clamped to the list length). Parent gone: `not_found`, `reason: 'parent_missing'`. Node not in that revision: `not_found`, `reason: 'absent_in_revision'`.
- **A path and a field:** the field takes its old value, or is removed if it did not exist then. `audience` and `assigneeId` on the plan are `assign_required`; `id`, `revision`, and `status` are `invalid_op`.
- A stale `expectedRevision` is `revision_conflict`; a result that fails a write-time block rule is `validation`; neither writes a revision, audit, or mark row.

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 | Pass. Changed meaning, added block, removed word, one `moved`, empty for an unchanged plan and for key-order differences. Property tests over all 15 fixtures (four samples, ten 1.1.0 files, a base plan): `diffDocuments(a, a)` is empty, key order does not count, and a one-field edit gives exactly one entry. |
| ac-2 | Pass. Between two revisions, `'published'` against `'working'`, and an integer against `'working'` on a plan edited by four writes; `no_snapshot`, `not_found`, `invalid_op`; `path` narrows (as a node and below, never a string prefix). |
| ac-3 | Pass. 600 changed entries give 500 and `truncated`; bulky values stop at the size cap; a 5,000-character value gives 4,001 characters with `valueTruncated`; `full` turns the caps off. |
| ac-4 | Pass. After an AI edit then an admin edit, restoring the field from the AI's revision creates exactly one new `restore` revision, changes nothing else, and shows after the admin row in the list. Both an `ai` and a `human` context succeed. The history rows before it are untouched. |
| ac-5 | Pass. A whole restore keeps `personal` and the assignee, sets `draft` from `published`, keeps `1.1.0`, and leaves the snapshot as it was. |
| ac-6 | Pass. A removed block comes back at its index; with the list shortened it is clamped; a lesson comes back into its unit; a block whose lesson is gone is `parent_missing`; an inserted block is `absent_in_revision`. |
| ac-7 | Pass. `assign_required`, a stale `expectedRevision` (and a missing one), and a restore that would put back a title with markup (`html` rule) each write nothing: working copy, log, audit, and marks unchanged. |

## Tests

`test/plan-diff.test.js` (16 tests, 16 pass) and `test/plan-restore.test.js` (15 tests, 15 pass).  
Full `npm test` with `SRS_SCHEDULER=ladder`: 1242 tests, 1242 pass, 0 fail.

## Deviations

- `diffDocuments` returns an object (`{ entries, truncated }`), not a bare list, because the caps need to report. The `full` option is mine: SPEC-006 needs the uncapped list.
- The plan `revision` counter is not reported as a change (it always differs); `status` is.
- A list whose items do not all have distinct ids is compared as one field, so a malformed document cannot throw.

## Files

New: `server/plans/diff.js`, `test/plan-diff.test.js`, `test/plan-restore.test.js`, this summary.  
Edited: `server/plans/revisions.js`, `server/plans/ops.js`, the SPEC-005 file (status `in_review`, boxes ticked, executor notes).  
No commit.

## Provenance

`INIT-008/SPEC-005`
