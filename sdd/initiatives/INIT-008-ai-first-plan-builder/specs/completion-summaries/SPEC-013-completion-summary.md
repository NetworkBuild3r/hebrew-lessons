# SPEC-013 completion summary

**Spec:** SPEC-013 · Frontend: plan list, create-plan flow, assign control  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code, unit tests, and the Chrome smoke done; `security_review: standard` has no independent reviewer yet)  
**Provenance:** `INIT-008/SPEC-013`

## What shipped

The Plans tab is now a real list, with a New plan form, and the plan view has an assign control. All three are clients of the SPEC-010 routes; none holds plan logic.

- **`shared/admin/plan-list.js` (`HebrewPlanList`).** `mount(panel, { api, setHash })` draws the Plans tab and returns a cleanup function; `HebrewPlanBuilder.mountPicker` calls it, so the admin shell's seam (`mount`, `mountPicker`, `unmount`) is unchanged. `createAssign({...})` makes the assign control the plan view mounts.
  - **The list.** `GET /api/plans`, one table row per plan: title (a real link to `#plan/<id>`) with the id under it, status (draft or published) with an "unpublished changes" mark, audience with the assignee, revision, the number of fields a person last wrote (`humanEditedFields`), and the updated time. A plan with no title lists by its id. Rows are real table rows (header cells with `scope`, explicit ARIA roles). Below 40rem each row becomes a card of label and value pairs, so there is no sideways scroll. An empty list says so and opens the New plan form; a truncated answer says so; a failed read shows the server's code and a Retry button. A click on a row (not on its link) opens the plan through the hash.
  - **New plan.** A form with an id, a title, and a recipe picker. The id pattern, the title limit, and the recipe list are read from `GET /api/plan-schema`, so no list is copied (without the schema there is no picker, not a stale one). A bad id or an empty title is refused in the browser, with a code and a sentence, and sends nothing. Otherwise exactly one `POST /api/plans` is sent with `id`, `title`, and `recipe` only when one is chosen; on success the plan opens straight into the editor. `already_exists` and `invalid_id` from the server are plain messages and nothing typed is cleared. The note under the picker says a recipe sets a starting shape and writes no Hebrew (SPEC-007's scaffold sets the recipe only).
  - **Assign control (`data-plan-assign`).** Audience (shared or personal) and, for personal, an account from `GET /api/admin/users`, active accounts only, in a `<select>`; there is no text input. It sends one `POST /api/plans/:id/assign` with the audience, the account (personal only), and the builder's `expectedRevision`. `assignee_refused` is shown in plain words; a 409 is handed to the builder's conflict box; success reloads the plan at the new revision. It is the only code in `shared/admin/` that sends `audience` or `assigneeId` (a test searches for it); the plan form keeps rendering both read-only.
- **`shared/admin/plan-ui.js` (`HebrewPlanUi`).** Small helpers shared by the new panels: element builders, Hebrew-over-English labels, a brand-sprite icon, plain-words messages for the plan routes' codes (`failureText`), and value formatters. Nothing in it talks to the server or builds HTML from server text.
- **`shared/admin/plan-builder.js`.** `mountPicker` now mounts the list; the plan view mounts the assign control under the toolbar and, when it loads, the account list. The old id picker is gone. The plan view is now a two-column layout on a wide screen (see SPEC-014 and SPEC-015); on a phone it is one column.
- **`shared/app-routes.js`.** The admin route loads the new scripts in dependency order (`plan-ui.js` first). No other route loads a plan screen.
- **`shared/worksheet.css`.** One new block, `INIT-008/SPEC-013 + SPEC-014 + SPEC-015`, tokens only, light by default, 44 px primary targets, `[hidden]` honoured inside the builder.

Hooks: `data-plan-list`, `data-plan-row` (the plan id), `data-plan-new`, `data-plan-new-open`, `data-plan-new-id`, `data-plan-new-title`, `data-plan-new-recipe`, `data-plan-create`, `data-plan-assign`, `data-plan-assign-audience`, `data-plan-assign-user`, plus `data-plan-assign-save`, `-summary`, `-message`, `data-plan-open-link`, `data-plan-row-status`, `data-plan-unpublished`, `data-plan-human-fields`, `data-plan-list-refresh`, `-retry`, `-empty`, `-truncated`.

## Tests

- `test/plan-editor-list-create.test.js`: 26 tests, 26 pass, on a fake DOM with a stand-in for the plan routes (`test/helpers/plan-panels-vm.js`). Pure helpers (`checkNew`, `limitsFromSchema`, `rowsOf`, `activeUsers`); the list (one row per plan, every column, links, status, unpublished mark, human count, empty, truncated, error and retry, row click); New plan (recipe options equal the schema's enum, exactly one POST with `id`/`title`/`recipe`, no `recipe` key when none, client refusal sends nothing and keeps values, `already_exists` keeps values, cleanup); the assign control (active accounts only, no free text, personal-only account list, one POST body, shared sends no assignee, `assignee_refused`, 409 to the host, a current assignee who is no longer active stays in the list, marked; the plan form's two fields are read-only and no other module sends them); the admin route's script order; the builder seam (`mountPicker` and `unmount`).
- `test/chrome-plan-editor-smoke.js` (extended; temp database; one script serves SPEC-011 to SPEC-015): the list shows `editor-smoke` (draft), `published-plan` (published) and `edited-plan` (draft with unpublished changes); a row's link opens its plan; New plan with a bad id (browser refusal, nothing stored), with an id that exists (`already_exists`, typed values kept, the existing plan untouched), and for real with the `verb-workshop` recipe, which opens in the editor, stores a `1.1.0` plan with that recipe at revision 1, and takes an edit (revision 2, history shows `create` and `set_field` by a person); the assign control offers `example-learner` and not the disabled `example-retired`, has no text input, saves personal then shared (stored values checked), and shows `assignee_refused` for a disabled account injected into the select; a learner gets none of the list, New plan, assign, history, or issues markup on `/admin/#plans` or `#plan/<id>`, and 403 from the plan routes. Layout at 360 and 1280 px (list with the New plan form open): no horizontal overflow, every target at least 24 px, primary buttons at least 44 px, every control named, contrast at least 4.5:1.
- Full `npm test` with `SRS_SCHEDULER=ladder`: **1443 tests, 1443 pass, 0 fail** (1335 before; 108 new across SPEC-013 to SPEC-015). All 20 `test/chrome-*-smoke.js` pass (see "Not closed" for one flake). The real database was not opened or migrated.

## Screenshots (looked at)

`spikes/plan-panels-shots/` (untracked, never served): `list-1280`, `list-new-1280`, `list-360`, `list-new-360`.

- At 1280 px the list is one card with the heading, a one-line Hebrew intro over its English gloss, a blue "New plan" button beside a ghost "Refresh", and a six-column table, Hebrew headings with English under them, rows with the title as a bold underlined link and the id in grey beneath, status as a small rounded mark (blue border for "Published", dashed for "Unpublished changes"), then audience, revision, the person-edited count, and the date. It reads right to left, with the plan column at the right. The New plan form opens as a soft grey panel above the table: Id, Title and Recipe, each Hebrew label over its English one, a one-line help under the id and under the recipe, and a blue Create button.
- At 360 px each plan is its own card: the title and id on the first line, the status marks on a full-width line (the first screenshots had "Unpublished changes" squeezed into a narrow oval; I made the status span the row and the marks rounded rectangles), then pairs of label over value for audience, revision, edited-by-a-person count, and updated. There is no sideways scroll (the page's scroll width was 345 px at a 360 px window in the overflow probe). Forms are one column with full-width controls.
- Honest remarks: the dates wrap onto two lines in the card ("2026-10-13" over "03:09") because the value column is half the card; the table at 1280 px gives the status column more width than it needs. Neither blocks use.
- Not looked at: the dark theme, and a screen reader. The contrast checks use the shared tokens in the light theme.

## Deviations

- **The smoke additions went into `test/chrome-plan-editor-smoke.js`** as the spec says; the script is now about 800 lines and takes about 26 seconds.
- **The assign control shows a validation message.** The schema's assignee pattern (`^[a-z0-9][a-z0-9-]{7,63}$`) refuses a username under 8 characters, such as the harness's `admin` and `learner`, with `validation` / `schema` at `/assigneeId` (found when the smoke's first assign failed). The control lists every active account, as the spec says, and shows that finding's path and message when the server refuses one. It does not grey out the short names, because that would copy a rule that lives in the schema. The smoke uses `example-learner`.
- **A disabled account cannot be chosen in the UI.** The smoke injects one into the select to see `assignee_refused`; that tests the message and the server, not a reachable path.
- **`plan-form.js` did not change for this spec** (SPEC-011 already shows both fields read-only with a help line pointing at the control).
- **Hooks.** `data-plan-new-id` is also the new-block form's id input in the plan view (SPEC-011); the two live in different views, so no query meets both.
- **The assign control shows for every plan, published or not.** An assignment is a write like any other and makes a new revision.

## Not closed

1. **No independent review** (`security_review: standard`). The guard is the existing admin gate; the plan routes' 403 for a learner is asserted in the smoke.
2. **One flaky smoke in the full run.** `test/chrome-admin-lessons-smoke.js` ("new lesson should start hidden") failed once in the first all-smokes pass and passed in each of the next four runs, alone and in the second all-smokes pass. It tests the legacy lesson editor, which this work did not touch; I did not find the cause.
3. **Hebrew copy** (labels, helps, the confirmation wording) was written by me and has not been read by a person who reads Hebrew.
4. **The dark theme and a screen reader were not tried.**

## Files

New: `shared/admin/plan-ui.js`, `shared/admin/plan-list.js`, `test/plan-editor-list-create.test.js`, `test/helpers/plan-panels-vm.js`, `test/helpers/plan-panels-seed.js`, this summary.  
Edited: `shared/admin/plan-builder.js`, `shared/app-routes.js`, `shared/worksheet.css`, `test/chrome-plan-editor-smoke.js`, the SPEC-013 file (status `in_review`, boxes ticked, notes).  
No commit. No change to `worksheets/`, the database, or any `.mp3`. No existing test changed.

## Provenance

`INIT-008/SPEC-013`
