# SPEC-006 completion summary

**Spec:** SPEC-006 · Service: Provenance marks and the advisory edit state in `read_part`  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code and tests done; SPEC-009 and SPEC-010 will add the new tools and routes)

## What shipped

- **`server/db/repos/plan-field-marks.js`** (SQL only): `upsert(mark)`, `forPlan(planId)`, `forPathPrefix(planId, prefix)`, `deleteUnder(planId, prefix)` (both compare with `substr`, never `LIKE`), `humanCounts()` (one grouped query, a `Map` of plan id to count), `lastHuman(planId)`.
- **`server/plans/provenance.js`**:
  - `changesOf(before, after)`: the SPEC-005 diff, uncapped, minus the plan's `status` (bookkeeping, not an edit).
  - `recordWrite(planId, before, after, ctx, revision, entries?)`: for each changed field upserts a mark `(plan_id, path, field)` with the last actor, revision, time, and a SHA-256 of the field's stable JSON. For a person it also sets `human_revision`, `human_at`, `human_actor_name`, `previous_value_json` (the value replaced; NULL over 32 KiB; the text `null` when the field was new) and `previous_writer_kind` (the earlier mark's last writer, or `unknown`). An `ai` or `system` write keeps the person's columns. A removed node deletes the marks under it; an added or moved node writes none.
  - `overwrittenBy(planId, changed, ctx)`: the one place that returns `overwrote: [{ path, field, humanRevision, valueSetByPerson }]` (max 20) for an AI or script write that replaces a field a person last wrote. Called from one line in `write-log.js`. Remove both and nothing else changes (initiative §16 A3).
  - `editStateFor(planId, path, ctx)`: `{ advisory: true, lastHumanEdit: { revision, at } | null, humanEdited: [{ path, field, revision, at, previousValue, previousWriter, previousValueOmitted?, by? }], more }`. `humanEdited` lists the marks at or below the path whose current last writer is a person, newest first, at most 50, with `more` the rest. `lastHumanEdit` is the newest person edit anywhere in the plan.
- **`ops.readPart(store, id, input, ctx?)`** now returns `{ part, revision, editState }`. `part` and `revision` are as before.
- **`write-log.js`** (SPEC-004) calls `overwrittenBy` then `recordWrite` inside the write's transaction, so a failed marks upsert rolls back the working copy, the revision row, the audit row, and the marks together.
- **`listPlans`** (SPEC-007) carries `humanEditedFields` from `humanCounts()`.
- **Adapters, minimal:** the admin read route now passes an admin context and the `read_part` tool passes an `mcp` context, so `editState` reaches both; `sendMapped` forwards `editState` and `overwrote`; the `read_part` description mentions the note. No tool or route was added.

## The name rule

`showNames(ctx)` (one condition): an admin or a script context gets `by` on each `humanEdited` entry; an `ai` context, or a read with no context, gets none. The same condition hides the account name in `revisions.list` and `revisions.get`.

## No refusal

No function in this spec returns an error, throws, or changes a write's outcome because of a mark. `test/plan-no-refusal.test.js`:

- Property over all 12 combinations of actor (`ai`, `human`, `system`) and last writer (`ai`, `human`, `system`, none), for `set_field`, a part `replace_part`, a lesson `replace_part`, and `restore`: the same write is accepted every time and the 12 resulting documents are equal (4 properties, 48 writes).
- Eight failing writes run under every combination; the set of errors seen is exactly `revision_conflict`, `validation`, `id_mismatch`, `assign_required`, `not_found`, `invalid_op`.
- A scan of every error literal in the plan modules and the plan routes (excluding the validator, the Hebrew checks, and the issues module, which another agent is finishing) finds none that mentions a mark, a person, a pin, a lock, an edit, or a writer.
- A search of `server/plans/`, `server/db/repos/plan-*.js`, `server/db/migrations/006-*`, and `server/api/plan-routes.js` finds no `field_pinned`, `keepPinned`, `pinned_by`, `pinned_at`, and no column, key, or property named `pinned` or `locked`; `PRAGMA table_info` for `plans`, `plan_revisions`, and `plan_field_marks` agrees; the registered plan tools' names, titles, descriptions, and input names hold none either.

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 | Pass. The AI sets `meaning` (r2), an admin changes it (r3): an `ai` read of the word shows `humanEdited` with the word's id path, `meaning`, revision 3, `previousValue` the AI's text, `previousWriter: 'ai'`, `lastHumanEdit.revision` 3, and no username anywhere. An admin context and a script context also get `by: 'example-admin'`. A read with no context gets no name. |
| ac-2 | Pass. After the AI overwrites (r4): the field is no longer in `humanEdited`, `lastHumanEdit` is still 3, the r4 write result carries `overwrote` with the path, field, revision 3, and the person's value, and the history lists r2, r3, r4 with their actors. A person's own write and an unrelated write carry no `overwrote`; a script write, a whole-part replace, and a restore do when they replace a person's value. |
| ac-3 | Pass. Content with no mark that a person edits gets `previousWriter: 'unknown'` and `previousValue` `coffee`; a clean plan returns `humanEdited: []` and `lastHumanEdit: null`. A person after a script gets `system`, a person after a person gets `human`. |
| ac-4 | Pass (above). |
| ac-5 | Pass (above). |
| ac-6 | Pass. A removed block and a removed word delete their marks and keep a sibling that only shares a prefix (`listen-2`, `listen_x`, `words/3`); an added and a moved block write none; an admin restore records `human` marks with the AI's value as the previous one. |
| ac-7 | Pass. A value over 32 KiB written over by a person stores NULL and the note says `previousValue: null`, `previousValueOmitted: true`; 60 human-edited fields give 50 entries, `more: 10`, newest first. |
| ac-8 | Pass. A stubbed `upsert` that throws rolls back the working copy, the revision rows (the baseline too), the audit row, and the marks; the next write works. |
| ac-9 | Pass. Two human-edited fields, a field a person edited and an AI overwrote, and a never-edited plan list as 2, 0, 0, and a statement spy sees exactly one query on `plan_field_marks`, with `GROUP BY`. |

Also covered end to end through the in-process HTTP server and the MCP handlers: the admin `GET /api/plans/:id/parts` returns exactly `part`, `revision`, `editState` with `by`; the MCP `read_part` returns `editState` with no `by` and no username anywhere in its text; the MCP write result carries `overwrote` and no username.

## Tests

`test/plan-provenance.test.js` (18 tests, 18 pass; 17 on a temporary database, 1 through the harness) and `test/plan-no-refusal.test.js` (9 tests, 9 pass).  
Full `npm test` with `SRS_SCHEDULER=ladder`: 1242 tests, 1242 pass, 0 fail.

## Deviations and things to know

- `overwrote` is present on a write result only when it is not empty.
- The plan's `status` never gets a mark (a publish would otherwise count as a person editing it).
- A field a person newly adds has `previous_value_json` `'null'`; `previousValueOmitted` is reserved for a value over 32 KiB.
- `editStateFor` and the diff-based marks only know about edits made after migration 006. Older content shows `previousWriter: 'unknown'`, as designed.
- Nothing was run against the real database.

## Files

New: `server/db/repos/plan-field-marks.js`, `server/plans/provenance.js`, `test/plan-provenance.test.js`, `test/plan-no-refusal.test.js`, this summary.  
Edited: `server/plans/ops.js` (`readPart`), `server/plans/write-log.js` (new in SPEC-004), `server/api/plan-routes.js` (read route context, `sendMapped` keys), `mcp/tools.js` (`read_part` context and description), the SPEC-006 file (status `in_review`, boxes ticked, executor notes).  
No commit.

## Provenance

`INIT-008/SPEC-006`
