# SPEC-007 completion summary

**Spec:** SPEC-007 · Service: `createPlan` and `listPlans`  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code and tests done; SPEC-009 and SPEC-010 will add the tool and route)

## What shipped

- **`server/plans/create.js`**
  - `createPlan(store, { id, title, recipe?, unitId?, lessonId? }, ctx)` returns `{ plan, document, revision: 1, status: 'draft', validation }` or an error: `invalid_id` (with `field` for a bad `unitId` or `lessonId`), `invalid_recipe`, `validation` (the same body a write refusal has), or `already_exists`. Nothing is written on an error. The slug rule and the recipe list are read from the plan schema file, so there is no second copy. The existence check and the write are in one `db.transaction`, so two calls with one id cannot both write, and a retry never overwrites.
  - The document is the smallest one the schema accepts as a draft: `schemaVersion` `1.0.0` (or `1.1.0` with a `recipe`), revision 1, `draft`, `shared`, no assignee, one unit, one lesson with `visibility: 'hidden'`, empty `buildsOn`, `newWords`, `recycles`, and one placeholder block (see Deviations). It is written through the write log: one `plan_revisions` row (op `create`) and one `plan.create` audit row, in the same transaction. A plan is always created `shared`.
  - `listPlans(store, { limit? }, ctx)` returns `{ plans: [{ id, title, audience, assigned, status, revision, updatedAt, published, unpublishedChanges, humanEditedFields }], truncated }`, newest update first, at most 200. An `ai` caller gets the boolean `assigned` and no `assigneeId`; an admin or script caller also gets `assigneeId`. `humanEditedFields` comes from SPEC-006's one grouped query.
- **`server/plans/store.js`**: `listPlans(limit)` (the only new plan SQL) and `LIST_MAX`. The title and the snapshot's revision are read with `json_extract`, guarded by `json_valid`, so a stored document that is not JSON lists with a `null` title instead of breaking the list. Neither document is loaded.

## Deviations

- **The scaffold has one placeholder block.** The schema needs at least one block in a lesson (`minItems: 1`), and any block with text needs non-empty text. The one block that needs no text is `fluency-pass`, so a new plan holds `{ id: 'placeholder', type: 'fluency-pass', seconds: 60, source: 'vocab' }`. No Hebrew is generated. The author inserts a real block and then removes the placeholder (a test does exactly that; removing it first is a `validation` refusal because the lesson would have no block). The unit title, the unit theme, and the lesson title all take the plan title, because the schema requires each to be non-empty text.
- **Recipe scaffolds are the recipe only.** The schema does not accept an empty shell for any recipe card's blocks, so `recipe` sets the lesson's `recipe` and the plan's `schemaVersion` to `1.1.0`, and nothing else (the spec's own fallback). All six recipes validate with no block finding. The `New plan` screen's picker (SPEC-013) therefore picks a starting `recipe`, not a block layout.
- `unpublishedChanges` is `false` for a plan that was never published.

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 | Pass. Revision 1, `draft`, `shared`, `hidden` lesson, no block finding, one `create` revision row, one `plan.create` audit row (actor `mcp` for an AI context, `admin:example-admin` for an admin), no marks, and `read_part` with an empty path returns the same document. |
| ac-2 | Pass. `already_exists` leaves the plans, revision, audit, and marks tables byte-identical (compared as one snapshot), also for a seeded plan. Sixteen bad inputs (non-slug ids, 65-character id, missing or non-string id, bad `unitId` and `lessonId`, bad recipe, a title over 4000 characters, with markup, empty, missing, and a number) each return the right error and change nothing. A call with no context throws `TypeError`. |
| ac-3 | Pass. Each of the six recipes sets `recipe` and `1.1.0` and validates with no block finding; no `recipe` stays `1.0.0` with no `recipe` key. Custom unit and lesson ids work. |
| ac-4 | Pass. A created, a published-then-edited (revision 3, `published` true, `unpublishedChanges` true; after a publish, false), and a personal plan list correctly; the cap is 200 with `truncated` (201 plans stored); `limit` works; `ai` rows have no `assigneeId` and the personal plan has `assigned: true`; an admin sees the `assigneeId`. |
| ac-5 | Pass. A source search finds no SQL and no `db.` call in `create.js`; the SQL in `store.listPlans` reads only `plans`; a statement spy shows a listing reads only `plans` and `plan_field_marks`. |
| ac-6 | Pass, with the harness (a second `describe` in the same file). `GET /api/plans/:id/published` is 404 for a new draft (learner and admin), 404 after publish while the lesson is `hidden`, 404 after exposing it until it is republished, and 200 once the exposing revision is published. |

## Tests

`test/plan-create-list.test.js`: 14 tests, 14 pass (13 on a temporary database, 1 through the in-process HTTP server).  
Full `npm test` with `SRS_SCHEDULER=ladder`: 1242 tests, 1242 pass, 0 fail.

## Files

New: `server/plans/create.js`, `test/plan-create-list.test.js`, this summary.  
Edited: `server/plans/store.js` (`listPlans`, `LIST_MAX`), the SPEC-007 file (status `in_review`, boxes ticked, executor notes).  
No commit.

## Provenance

`INIT-008/SPEC-007`
