# SPEC-011 completion summary

**Spec:** SPEC-011 · Frontend: schema-driven editor core  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code, unit tests, and the Chrome smoke done; `security_review: standard` has no independent reviewer yet)  
**Provenance:** `INIT-008/SPEC-011`

## What shipped

The admin plan builder now edits any part of a plan from the plan JSON Schema. `shared/admin/plan-form.js` fetches `GET /api/plan-schema` once per mount and builds a form for the plan root, a unit, a lesson, a block, or a bucket item (a word, paragraph, line, or question). Each property gets a widget from `shared/admin/plan-widgets.js`. Anything the generator has no widget for gets the generic JSON widget, and a node whose block type the schema does not know opens as raw JSON, so nothing in the schema is uneditable. The hints that give labels and languages are SPEC-012's `plan-hints.js`.

- **Widget registry.** `{ name, isDefault, test(info, ctx) -> priority, render(info, ctx) -> handle }`, highest priority wins; a hint can name a widget. Defaults: string → input, or a textarea above 200 characters (or when the hint says so); enum → select; boolean → checkbox; integer → number; array of strings → string list; array of objects → object list; object with properties → nested object (optional objects have Add and Remove); a block's `words`, `paragraphs`, `lines`, `questions` → bucket list; read-only and `const` → read-only; everything else → JSON (priority 1, always matches).
- **Saving is one operation.** A field sends `PATCH /api/plans/:id/fields` with the id path, field, whole value, and the revision the server last returned (`state.revision` in `plan-builder.js`, never a value read from the page). An object, a list, and the JSON widget send the whole value of that one property. A bucket item is its own form at its own id path, so its save names the item and the history keeps item-level precision; adding, removing, and moving items sends the block's whole list. Raw node JSON sends `PUT .../parts`.
- **An emptied optional field is removed.** No operation unsets a field, so an optional field that is emptied (or removed with its button) is sent as a `PUT .../parts` of the node as loaded minus that one property. That is a node, not a plan document, except at the plan root.
- **Errors at the field.** A 409 keeps the typed text, shows `revision_conflict` and the stored revision in the conflict box and at the field, and Reload takes the server's copy (and drops unsaved text). A 400 validation list is mapped from the validator's RFC 6901 `path` to the id path with `splitPointer` (the client twin of the server's `toIdPath`) and shown at the field with its rule code, severity, and message; `aria-invalid` is set on a blocking one. Other refusals show their code (`id_mismatch`, `assign_required`, and so on). A refused save does not repaint the page, so nothing typed is lost.
- **Unsaved text survives a repaint.** `captureDrafts()` before each paint and `renderNode` after it; sections open and closed, focus, and scroll are kept too. `canLeave()` asks if any field is unsaved.
- **Layout of the page.** Plan, unit, and lesson forms are sections the admin opens (lesson open by default; the body is built only while open). The selected block's form sits under the outline.
- **Listeners.** The builder's three delegated listeners and the editor's three go on the panel with an `AbortSignal` where the browser has one, and unmount aborts and removes them (the picker's click listener too).
- **Server text is text.** Every string from the server is set with `textContent` or `.value`; a test renders `<img onerror=…>` in a title, a meaning, and an issue and finds no element. The only `innerHTML` left in the builder is the existing learner-renderer preview, which escapes plan text.

## The editor module API (for SPEC-013 to SPEC-015)

```js
var editor = HebrewPlanForm.createEditor({
  doc,            // document
  host,           // the panel; delegated click/input/change listeners go here
  signal,         // AbortSignal or null
  schema,         // parsed schema or text (optional; else editor.loadSchema(HebrewApi))
  transport,      // { setField(path, field, value, hooks), putPart(path, value, hooks) } -> Promise<{ ok, status, data }>
  track,          // fn(promise): the builder's settled() hook
  getPlan,        // fn() -> the current plan document, for pointer mapping
  onToggle,       // fn(path): an item section was opened or closed (the builder repaints)
  onValidation,   // fn(data): a 400 validation body (optional)
  confirm         // fn(message) -> boolean (default window.confirm)
});
editor.loadSchema(HebrewApi)                      // Promise<boolean>, once per mount
editor.renderNode({ path, node, kind?, def?, plan? })
                                                  // -> { el, path, kind, def, node, fields: { prop: handle }, issues }
editor.renderDraft(def, value, onlyProps?)        // a form for a value not in the plan yet; handle.getValue(), handle.check()
editor.setIssues(findings, plan)                  // inline issues; a finding is { rule, severity, message, path (pointer) }
                                                  // or { ..., path (id path), field, pointer } as GET .../issues returns
editor.captureDrafts() / reset() / discardDrafts() / hasUnsaved()
editor.focusKey() / restoreFocus(key)
editor.forms() / handles() / tools()              // for tests and panels
editor.destroy()
HebrewPlanForm.newBlock(type, id[, schema]) · blockTypeList() · enumOf(def, prop) · splitPointer(plan, pointer)
HebrewPlanForm.kindOfPath(path) · nodeAt(plan, path) · createTools(schema) · findUncoveredSchemaNodes(schema, registries)
HebrewPlanBuilder.editor()                        // the live editor of the mounted plan view
```

Hooks: `data-plan-form`, `data-plan-node` (id path), `data-plan-kind`, `data-plan-def`, `data-plan-field` (top-level property) and `data-plan-sub` (nested), `data-plan-widget`, `data-plan-row`, `data-plan-save` (with the old `data-plan-save-field` index and `data-plan-field-index` kept), `data-plan-clear`, `data-plan-json` (`node` for raw JSON), `data-plan-issues` and `data-plan-issue` (the rule code), `data-plan-section`, `data-plan-disclose`, `data-plan-dirty`, `data-plan-focus-key`.

SPEC-013 can render its assign control as a sibling card and reuse `transport`-style calls; the plan form already renders `audience` and `assigneeId` read-only with a help line pointing at it. SPEC-014 can call `renderNode` for any path and `setIssues` for per-field badges; SPEC-015 can feed `GET .../issues` into `setIssues` unchanged (the `pointer` key marks the split form) and render the preview from `getPlan()`.

## Tests

- `test/plan-editor-core.test.js`: 28 tests, 28 pass. Single operations for a string, number, select, string list, object list, nested object, bucket item, bucket add/move/remove, an emptied optional field, and a required field emptied; the JSON fallback (invalid sends nothing, valid sends one, raw node JSON, an unknown block type, a mutated schema property); 409 and validation at the field with the text kept; read-only fields; escaped rendering; drafts, focus, listener removal; pointers.
- `test/plan-builder.test.js`: 13 tests (6 new): schema fetched once after the plan and lesson; the revision sent is the server's; other unsaved text survives a repaint; the new-block form; a refused insert; unmount removes listeners.
- `test/chrome-plan-editor-smoke.js`: passes (see SPEC-012's summary for what it covers; one script serves both specs).
- Full `npm test` with `SRS_SCHEDULER=ladder`: **1335 tests, 1335 pass, 0 fail** (1278 before). All 20 `test/chrome-*-smoke.js` pass. The real database was not opened or migrated.

## Existing tests and files changed

- `test/plan-builder.test.js` and `test/plan-builder-media.test.js`: the vm loader now also runs `plan-hints.js`, `plan-widgets.js`, and `plan-form.js`, and the fake `fetch` answers `GET /api/plan-schema` (the builder fetches it). In `plan-builder.test.js` the test that called `fieldsForBlock` and the old six-type `newBlock` was rewritten (it mounts, then reads the editor form and the schema-generated `newBlock`); no test was removed and no other assertion changed. The old hooks those tests query (`data-plan-edit`, `textarea[data-plan-field="…"]`, `data-plan-save-field`, `data-plan-field-index`, `data-plan-conflict`, and the rest) still exist. The old source greps (no `replace_part` and no `JSON.parse` in `plan-builder.js`) still hold: the transport method is `putPart`, and the JSON work is in `plan-form.js`.
- `test/chrome-guides-smoke.js` and `test/chrome-verb-workshop-smoke.js`: each called `publishPlan` without the write context SPEC-002 made mandatory, so both were already failing; they now pass a `system` context. Nothing else in them changed.
- `shared/app-routes.js`: the admin route loads the three new scripts before `plan-builder.js` (this is where scripts are listed; `shared/app.html` needed no change).
- `shared/worksheet.css`: one new block, `INIT-008/SPEC-011 + SPEC-012`, tokens only.

## Deviations

- **Save per field, not per form.** Each top-level field has its own Save button (as the old builder did), so the page has many 44 px buttons. A "save all changes" for a node is a possible later addition.
- **An emptied optional field uses `replace_part`** (above); the spec said set_field or replace_part only for JSON. I kept it because otherwise an optional field could never be removed from the UI.
- **Hooks use the spec's new names plus the old ones.** `data-plan-save-field` (the old index hook) is numbered in creation order, with the block form first, so a block's first field is `0` as before.
- **Preview unchanged.** The preview is still the existing renderer string for the selected block (SPEC-015 owns the live preview). It now scrolls inside its own box instead of widening the page on a phone.

## Not closed

1. **No independent review** (`security_review: standard`).
2. **Unsaved text in the new-block form is lost on a repaint** (the form's type, id, and index are not kept across a repaint; the filled required fields are kept only until the next paint).
3. **`explainerTable.rows` and `gradedStoryBlock.glosses` are JSON textareas**, as the spec lists; a table grid and a key/value editor would be nicer.
4. **The older restyle smoke's 360 px check cannot see overflow**: it uses Chrome's mobile emulation, which widens the layout viewport to fit the content. The new smoke uses a non-mobile 360 px viewport and found two real overflows (the picture-upload file input and the preview), now fixed. The old script is unchanged.
5. **Screenshots** at 360 and 1280 px are under `spikes/plan-editor-shots/` (untracked, not served); see SPEC-012's summary for what they show.

## Files

New: `shared/admin/plan-form.js`, `shared/admin/plan-widgets.js`, `shared/admin/plan-hints.js` (SPEC-012), `test/plan-editor-core.test.js`, `test/chrome-plan-editor-smoke.js`, `test/helpers/fake-dom.js`, `test/helpers/plan-editor-fixtures.js`, this summary.  
Edited: `shared/admin/plan-builder.js`, `shared/app-routes.js`, `shared/worksheet.css`, `test/plan-builder.test.js`, `test/plan-builder-media.test.js`, `test/chrome-guides-smoke.js`, `test/chrome-verb-workshop-smoke.js`, the SPEC-011 file (status `in_review`, boxes ticked, notes).  
No commit. No change to `worksheets/`, the database, or any `.mp3`.

## Provenance

`INIT-008/SPEC-011`
