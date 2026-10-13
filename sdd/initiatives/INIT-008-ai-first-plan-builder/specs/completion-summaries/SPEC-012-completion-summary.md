# SPEC-012 completion summary

**Spec:** SPEC-012 · Frontend: block hints and widgets for every block type; drift test  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code, unit tests, and the Chrome smoke done; `security_review: standard` has no independent reviewer yet)  
**Provenance:** `INIT-008/SPEC-012`

## What shipped

- **`shared/admin/plan-hints.js`.** One entry per schema property, keyed `definition.property` as the server's `FIELD_LANGUAGE` is, plus `block:<type>` and `def:<definition>`: a Hebrew-over-English label, a one-line bilingual help, the language (`he`, `en`, `latin`, `mixed`, `exempt`), single-line or textarea, a pointing note for pointed Hebrew, enum option labels (for example the six recipes, `visibility`, the cell codes), defaults for new items, and `advanced`. It covers the plan, the unit, the lesson (`recipe`, `visibility`, `buildsOn`, `newWords`, `recycles`), all eleven block types including the five `1.1.0` ones (`explainer`, `conjugation-table`, `personal-question`, `verse-reading`, `letter-intro`), and every item type. A Hebrew field shows its character count against the schema limit and the note "full pointing is fine". `langAttrs` maps a language to `dir` and `lang`: Hebrew `rtl`/`he`, English `ltr`/`en`, a transliteration `ltr`/`he-Latn`, and a mixed field `dir="auto"` with no `lang`.
- **New-block form.** `newBlock(type, id)` is built from the schema's `required` list (an empty string, an empty list, the schema minimum or the hint's default, the first enum value; no Hebrew), for all eleven types, and the block-type select is built from the schema. The new-block card also shows the required fields of the chosen type with the same widgets, so the admin fills them in before inserting (an empty required string would otherwise be refused by the server's `schema` rule); a refused insert lists the server's rule codes and paths in the card and keeps the text.
- **The hand-written lists are gone.** `BLOCK_TYPES`, `BLOCK_FIELDS`, `ITEM_FIELDS`, `FIELD_OK`, `fieldsForBlock`, and the six-type `newBlock` no longer exist anywhere in `shared/admin/` (a test searches the source).
- **The drift test.** `findUncoveredSchemaNodes(schema, { hints, exemptions, widgets })` is a pure function exported from `plan-form.js`. It reports a block type with no hint or exemption, a property of any reachable definition with neither (a property named `id` and a block's `type` const are structural), a hint or exemption that names a path the schema lacks, a hint naming an unknown widget, an exemption with no reason, and a registered non-default widget that no hint names. A property whose hint forces the JSON widget (or marks it read only) covers its subtree, so the walk does not descend into it. There are exactly **two exemptions**, each with a written reason: `plan.migration` (bookkeeping for imported plans, edited as JSON) and `lesson.legacyStorageKey`. The test is also run on a mutated copy of the schema (one new block type, one new lesson property, one new `wordEntry` property) and reports exactly those three.
- **Language agreement.** A test compares every hint's `lang` with `FIELD_LANGUAGE` (imported from `server/plans/hebrew-checks`), in both directions, so the two registries cannot disagree; the browser never fetches the server's table.

## Tests

- `test/plan-editor-drift.test.js`: 7 tests, 7 pass. The real schema has no gap; the mutated schema reports exactly the three planted ones; hints by name for the eleven types, `recipe`, `newWords`, `buildsOn`, `recycles`, `visibility`; two exemptions with reasons; a mutated registry reports a missing path, an unknown widget, a missing reason, and an orphan widget.
- `test/plan-editor-widgets.test.js`: 16 tests, 16 pass. Language agreement; `newBlock` for all eleven types (required keys, id, type, no Hebrew, hint default over minimum); a form for the plan, unit, lesson, each block type, and each bucket item with a widget on every property; every schema definition rendered as a draft form, one row and one registered widget per property; nested structures use the object, object-list, and string-list widgets and not JSON; every Hebrew text control (nested included) is `lang="he" dir="rtl"` and every English one `lang="en" dir="ltr"`; the character count and pointing note; bilingual labels and no points, streak, or joke wording; no `innerHTML` or `eval` in the three editor files.
- `test/chrome-plan-editor-smoke.js` (temp database, a fixture plan with one block of each of the eleven types): passes. It checks the lesson form (a widget per property; `recipe` and `visibility` are selects); all eleven block types render a form with bilingual labels and correct `lang`/`dir` on every Hebrew and English control; it types into a word and into each of the five `1.1.0` blocks, saves, sees the revision go up by exactly one each time, reads the stored plan, then reloads the page and sees the saved values; it saves `recipe` and `visibility`; inserts a new `listening` block through the form; makes a stale save (the AI bumps the revision in between) and sees the 409 keep the typed text, show `revision_conflict` and the stored revision, then Reload, then a retried save that lands; shows a `schema` rule inline at an emptied required field with the text kept and the revision unchanged; and, at 360 and 1280 px for the introduction, listening, and explainer blocks, finds no horizontal overflow, no target under 24 px, no primary button under 44 px, an accessible name on every control, and text contrast of at least 4.5:1. Every request stays on the origin and the page throws nothing.
- Full `npm test` with `SRS_SCHEDULER=ladder`: **1335 tests, 1335 pass, 0 fail**. All 20 `test/chrome-*-smoke.js` pass, including `chrome-restyle-a11y-smoke.js` (contrast, targets, reduced motion), `chrome-admin-smoke.js`, and `test/restyle-gate.test.js` (hook contract, label scan: no new English-only admin label; all new labels have Hebrew with an English gloss).

## Screenshots (looked at)

`spikes/plan-editor-shots/` (untracked, never served): lesson, introduction, explainer, conjugation, listening-error, outline, and new-block views at 360 and 1280 px.

- The forms are one column, Hebrew label on the right with the English gloss under it on the same side, controls full width with a visible border, bilingual option text in the selects (for example "בסיסי (Basic)"), and a blue "שמירת שדה (Save field)" button under each field. A field with unsaved text gets a blue bar on its inline-start edge.
- Nested structures read well: a section holds its paragraphs, examples, table, and callouts as shaded boxes with a "Hebrew (English) N" title and small ↑ ↓ × buttons. Pointed Hebrew shows in the Hebrew control with the count note under it. The conjugation form shows the cell code as a select with "זכר יחיד (הווה) (Masculine singular)" and the form key in a Hebrew control; an N13 finding from an earlier save showed in red at the cells field.
- An emptied required passage shows "block · schema — must not be empty" in a pink box under the field and a blue bar beside it, with the text kept.
- At 360 px everything fits in one column. I did see problems and fixed them: the English gloss sat on the wrong side of the row (fixed), the bidi order of "(Full pointing is fine)" and of an item's summary was scrambled (now isolated), two controls pushed the page 50 px past the screen (the old picture-upload file input and the learner preview), and the first screenshots were clipped because the headless capture's mobile mode had widened the page. Roughly, the page is long: every field has its own Save button, and the form for a block with many nested lists is several screens tall. I judge it usable but not compact.
- Not done: the dark theme was not screenshotted (the contrast checks in the restyle smoke cover the shared tokens, and my new rules use only tokens).

## Deviations

- **Exemptions and the JSON widget.** `plan.migration` is an exemption with `widget: json`; `gradedStoryBlock.glosses` and `explainerTable.rows` are hinted with `widget: json` (not exempt), as the spec lists maps and arrays of arrays as JSON cases.
- **`mixed` language fields** get `dir="auto"` and no `lang`, so the "English control is `lang=en`" rule is for English fields only.
- **`newBlock` passes the `required` check, not full validation** (an empty required string breaks `minLength`); that is why the form asks for the required fields first. A test asserts the `required` keys, not a full schema validation.
- **Hints for read-only identity fields** (`id`, `revision`, and so on) exist for their labels; the checker treats `id` and a block's `type` const as structural.

## Not closed

1. **No independent review** (`security_review: standard`).
2. **A table grid and a key/value editor** for `explainerTable.rows` and `glosses` are not built (JSON for now).
3. **Picture attach, upload, generate, and approve** stay in the existing panels; the `image` field is shown as an object (media id and alt) with a help line pointing at them, as the spec says.
4. **The dark theme and a screen reader were not tried by hand.** The a11y smoke checks names, targets, and contrast in the light theme.

## Files

New: `shared/admin/plan-hints.js`, `test/plan-editor-drift.test.js`, `test/plan-editor-widgets.test.js`, this summary. (`plan-form.js`, `plan-widgets.js`, the smoke, and the two test helpers are in SPEC-011's list.)  
Edited: the SPEC-012 file (status `in_review`, boxes ticked, notes).  
No commit. No change to `worksheets/`, the database, or any `.mp3`.

## Provenance

`INIT-008/SPEC-012`
