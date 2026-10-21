# SPEC-014 completion summary

**Spec:** SPEC-014 · Frontend: history, diff, restore panel and edit-state display  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code, unit tests, and the Chrome smoke done; `security_review: standard` has no independent reviewer yet)  
**Provenance:** `INIT-008/SPEC-014`

## What shipped

The admin can see who changed the plan, compare any two states, and put a value back in one confirmed click, and nothing on the screen stops them editing. The panel is a client of four routes and holds no diff logic and no restore logic.

- **`shared/admin/plan-history.js` (`HebrewPlanHistory`).** A card in the plan view, closed until opened (it reads nothing while closed).
  - **List.** `GET /api/plans/:id/revisions`, newest first. Each row: `#revision`, the writer as text plus an icon from the brand sprite (`admin` for a person with the account name, `settings` for AI with the tool, `review` for the system), the time, the operation, and the path and field. A "Written by" filter sends the server's `actorKind`; "Load older" sends `beforeRevision` of the oldest row shown.
  - **Open.** "Open" on a row reads `GET .../revisions/:n` and shows what that revision changed against the one before it (`diff?from=n-1&to=n`).
  - **Compare.** Two selects (a revision, the working copy, the published copy), a Compare button, a "working copy vs. published" shortcut, an "against the working copy" button on each row, and an optional path filter. One `GET .../diff` per comparison; one entry per server entry, grouped by node path, with a before box and an after box side by side. Hebrew values are `dir="rtl"` `lang="he"`, English `ltr`/`en`; a value the server cut is marked; a truncated result says so and the path filter narrows it; `no_snapshot` is a plain sentence.
  - **Restore, one step.** "Restore this value" (field) and "Restore this part" on a changed entry, "Restore this part" on a removed node, and "Restore the whole plan to this revision" on every row. Each shows one confirmation inline, under the control that was pressed: Hebrew over English, saying what comes back, that it becomes a new revision, and that history is kept; it warns when the form holds unsaved text. Confirm sends exactly one `POST .../restore` with `fromRevision`, `path`, `field`, and the builder's last `expectedRevision`; Cancel sends nothing. A restore takes its value from the earlier side of the comparison, so none is offered when that side is the working copy, none for an added node, and none for a moved list. A refusal appears in the confirmation: a 409 as `revision_conflict` with the stored revision and a Reload button that reads the plan again; a validation refusal with its rule codes; `assign_required` explained. After a success the builder reads the plan again at the new revision (the editor repaints, unsaved text is dropped, and the history shows the `restore` row).
- **The "edited by a person" mark (`shared/admin/plan-form.js`).** `editor.setEditState(humanEdited)` takes `editState.humanEdited` from the admin's `GET .../parts` and puts a button on each field whose last writer was a person: "נערך על ידי אדם (Edited by a person)" with an icon. It opens to the revision, time, and account name, the value that person replaced (in the field's language, or a note when it was over 32 KiB), and "Restore the earlier value", which sends the same restore (`fromRevision = revision - 1`, the state just before the person's write) through the same confirmation, shown under the mark. A form drawn later gets its marks when it is drawn. The mark is information: it never disables, hides, or warns away a field.
- **`shared/admin/plan-builder.js`.** Mounts the panel and reads `editState` from the first whole-plan read and again (one `GET .../parts`) after each write, so a field an AI later overwrites loses its mark. A restore resets the editor, drops drafts, and calls `loadPlan` (which keeps the open lesson and reloads the panels).
- **`shared/worksheet.css`.** Same block as SPEC-013; tokens only. Long Hebrew-and-English button labels wrap instead of overflowing (`.admin-row-btn` is `nowrap` elsewhere).

Hooks: `data-plan-history`, `data-plan-revision` (the revision), `data-plan-actor` (`human`, `ai`, `system`), `data-plan-diff`, `data-plan-diff-entry`, `data-plan-restore`, `data-plan-restore-confirm`, `data-plan-edited-mark`, plus `-history-toggle`, `-history-list`, `-history-filter`, `-history-more`, `-history-error`, `-history-reload`, `-actor-mark`, `-op`, `-rev-open`, `-diff-from`, `-diff-to`, `-diff-run`, `-diff-quick`, `-diff-path`, `-diff-head`, `-diff-before`, `-diff-after`, `-diff-kind`, `-diff-empty`, `-diff-truncated`, `-diff-error`, `-restore-box`, `-restore-cancel`, `-restore-error`, `-restore-unsaved`, `-edited`, `-edited-detail`, `-prev-value`, `-revision-detail`.

## Tests

- `test/plan-editor-history.test.js`: 39 tests, 39 pass. Pure helpers (`actorMark`, `groupEntries`, `describeRef`, `restoreOffers`, `restoreBody`, `restoreWhat`); the list (closed panel reads nothing, one row per revision with the actor mark as text, a person's account name and an AI's tool, the filter and "Load older" requests, empty and error states, "Open"); compare (one `GET` with the right `from` and `to`, one entry per server entry, `rtl`/`he` and `ltr`/`en`, revision against working and working against published, the path filter, truncated and cut values, `no_snapshot`, restore controls per entry kind); restore (confirmation first and nothing sent, cancel, field / part / whole bodies with the moving `expectedRevision`, one box at a time and no double POST, 409 with the revision and Reload, validation codes, `assign_required`, the unsaved warning, the list read again when the host did not refresh); the mark (shown on the right field, opens to the earlier value with the right `dir`/`lang`, restore asks for revision - 1 and anchors the confirmation under the mark, large value note, set and clear, odd entries do not throw, no control disabled because of a mark: the same disabled set with and without marks, and only end-of-list move buttons); the builder (a mark after load, restore from the mark posts once with the builder's revision and reloads at the new one, a restore from the list that meets a 409 and Reload, typing in a marked field saves); source checks (no "pinned" or "locked" in a new string, no diff or restore logic in the panel, one `POST` call, no points or streak wording).
- `test/chrome-plan-editor-smoke.js` (extended): a plan edited once by an AI context through the service layer and once by the admin through the editor shows three rows with the right marks (`ai`, `human` with the account name, and the `system` baseline), each with an icon and a Hebrew-and-English mark; the filter narrows to the AI row and back; comparing the AI revision with the person's shows the AI value before and the person's after, Hebrew `rtl`/`he`; restoring from the diff asks (nothing stored yet), Cancel changes nothing, Confirm adds revision 4 with the AI's value and a `restore` row by a person; restoring the person's revision as a whole adds revision 5, and the editor shows the restored text; an AI write in between makes the next restore a 409 that names `revision_conflict` and revision 6 and writes nothing, and Reload takes revision 6; the mark on the field opens to the earlier value, Cancel writes nothing, Confirm adds revision 7; no control in the block form is disabled for any reason other than a list-end move button or the media panel's own gates; an edit in a marked field still saves (revision 8). Layout at 360 and 1280 px with History open, a comparison showing, a restore confirmation open, and the mark's detail open: no overflow, targets, names, or contrast problems.
- Full `npm test`: **1443 tests, 1443 pass, 0 fail**. All 20 `test/chrome-*-smoke.js` pass (one earlier flake in an unrelated script; see SPEC-013's summary).

## Screenshots (looked at)

`spikes/plan-panels-shots/`: `history-`, `diff-`, `diff-result-`, `mark-` at 1280 and 360.

- At 1280 px the plan view is two columns. The forms are in the main column at the right (RTL); the history is in the aside at the left, which is its own scrolling rail (sticky, as tall as the window), under the preview and the issues. A history card is a ghost "▾ היסטוריה (History)" bar, a "Written by" select, Refresh, then one bordered card per revision: `#4` and a rounded actor mark ("בינה מלאכותית (AI) · seed-tool" with a small sliders icon, "אדם (A person) · admin" with a shield-and-person icon), the time, the operation, the path, and three small buttons (Open, Against the working copy, Restore the whole plan to this revision). The comparison shows the two selects, the path filter, Compare and the shortcut, then a header ("גרסה 2 (revision 2) → גרסה 3 (revision 3)"), the node path, a "Changed" chip, and two bordered boxes, Before and After, with the Hebrew values large and right-aligned, then the two restore buttons. The confirmation is a box with a blue 2 px border: the Hebrew sentence, its English under it in grey, then Restore (blue) and Cancel.
- The mark sits under the field's Save button: a ghost button "נערך על ידי אדם (Edited by a person)" with a shield icon; opened, it is a soft grey box with the revision, time and name, "The earlier value", the value in Hebrew, and "Restore the earlier value".
- At 360 px the rail becomes part of the page after the forms. Cards are full width; the long button labels wrap onto two lines; the selects are full width; the diff boxes stack. No sideways scroll (the shell's own nav strip scrolls, as it already did).
- Things I changed after looking: the operation and path were run together on one line and scrambled by the bidi order (now two lines); the mark's "revision, time, name" line was scrambled (the time and name are now isolated); the status marks in the list were squeezed ovals.
- Honest remarks: the rail is a nested scroll area at 1280 px, so the history is a scroll away from the preview; the Hebrew sentence in the confirmation puts the English path inside the Hebrew, which reads a little oddly. The English column of a row ("Restore the whole plan to this revision") makes the row buttons tall. Not looked at: the dark theme, a screen reader.

## Deviations

- **Compare is two selects and buttons, not "pick two rows".** It meets the same cases (any two revisions, a revision against the working copy, the working copy against published) with fewer moving parts, and each is testable by name.
- **The confirmation is inline, not `window.confirm`,** so it can carry the Hebrew and English wording, the unsaved-text warning, and the refusal in one place. It is the only step before a restore.
- **`data-plan-revision`** is the spec's history-row hook and also the toolbar's existing, valueless revision number. The first one in the page is the toolbar's, which is what the older tests and the smoke read.
- **One extra `GET .../parts` after each write** (the whole plan, for `editState`), only when the history module is loaded.
- **The stub AI actor is a `mcp` context passed to the service function** (`ops.setField`), the same call the MCP tool makes, rather than going through the tool's adapter.
- **A restore keeps the open lesson** (a small change to `loadPlan`: a lesson that was open stays open when it still exists), so a restore or a Reload does not send a person back to the lesson chooser.

## Not closed

1. **No independent review** (`security_review: standard`). The restore is one POST through `shared/api.js` (JSON, same origin, so the server's Origin and content-type check applies); every server string is set with `textContent`.
2. **The history does not page through the smoke.** "Load older" is tested on the fake DOM; the smoke's plans have fewer than 50 revisions.
3. **"Open" shows who and when and the change against the previous revision, not the whole document of that revision.** The route returns it; the screen does not show it.
4. **Hebrew copy not read by a person who reads Hebrew; dark theme and screen reader not tried.**

## Files

New: `shared/admin/plan-history.js`, `test/plan-editor-history.test.js`, this summary. (`plan-ui.js`, the CSS block, the test helpers, and the smoke are listed in SPEC-013's summary.)  
Edited: `shared/admin/plan-form.js` (the mark, `setEditState`, `focusField`, `open`), `shared/admin/plan-builder.js`, the SPEC-014 file (status `in_review`, boxes ticked, notes).  
No commit. No change to `worksheets/`, the database, or any `.mp3`. No existing test changed.

## Provenance

`INIT-008/SPEC-014`
