# SPEC-015 completion summary

**Spec:** SPEC-015 · Frontend: issues panel and live preview as the learner  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code, unit tests, and the Chrome smoke done; `security_review: standard` has no independent reviewer yet)  
**Provenance:** `INIT-008/SPEC-015`

## What shipped

An admin can see every issue the server reports, fix the ones with a known fix in one click, and see an unsaved edit as the learner would, with nothing unpublished reaching a learner. No server route was added.

- **`shared/admin/plan-issues.js` (`HebrewPlanIssues`).** A card in the plan view fed by `GET /api/plans/:id/issues`, read after the plan loads, after every successful write, and after a restore. It recomputes no rule and no cap.
  - **Grouping.** Hard errors first ("חוסמות פרסום (Blocks publish)"), then warnings ("אזהרות — אינן חוסמות פרסום (Warnings — does not block publish)"), then notes behind a "Show notes" toggle that reads again with `severity=all`. Inside a group, by stable code (`pointing-form`, `lexicon-form`, `audio-stale`, `N6`, …), each with a count. The heading shows the counts in Hebrew and English. Empty ("No issues") and truncated states say so; a failed read shows the code.
  - **Each item.** The message (as text), and a link button showing `path#field` that opens the node's section, selects the block, opens an item's form, and focuses the field (`editor.open` and `editor.focusField` in `plan-form.js`, flagged with `data-plan-jump`). Block, item, lesson, unit, and plan paths each work; for a path in another lesson the lesson is loaded first.
  - **One-click fix.** Where the issue has `fix.suggested`, the current and suggested values are shown side by side (Hebrew `rtl`/`he`) with "Use this value"; where it has `fix.candidates` (`pointing-consistency`), each form is a button and the chosen one is sent; where it has only `fix.note` (`audio-stale`) the instruction text is shown and there is no apply button. A fix is one `set_field` through the editor's own transport (the same save path and revision check as any edit), then the issues are read once. A refused fix shows the reason in the panel, and a 409 appears in the builder's conflict box.
  - **Fixes inside a field.** `set_field` takes a field of a node, so a fix at `cloze/items/0/answer` or `newWords/1` sends that one field whole, with the one place changed (`fixRequest`, copied from the page's copy of the working copy). Nothing else is built or sent.
- **`shared/admin/plan-preview.js` (`HebrewPlanPreview`).** A pure module: `compute` takes the saved node and the editor's field handles, lays the unsaved values over a copy, and draws it with the learner's own `HebrewPlanRender.renderBlock` (or the lesson card when no block is selected). A value that is invalid JSON is left out of the picture and still counts as unsaved; a renderer error keeps the last picture. It has no network call and no `innerHTML`; the builder assigns the renderer's escaped output. `guardClick` stops a click on a link or button inside the preview, so the learner's "continue" links cannot change the admin page's hash.
- **`shared/admin/plan-builder.js`.** The preview card (`data-plan-preview-card`) holds a label that says "Preview — not saved" or "Preview — as saved" and the existing `data-plan-preview` element, which now carries `data-plan-preview-state` (`saved` or `unsaved`). The panel's `input` and `change` listeners schedule a redraw 250 ms later; a repaint redraws at once. The line above Publish gives the hard-error and warning counts and says warnings do not stop publishing; Publish sends `{ expectedRevision }` and nothing else; after a publish the notice shows the new revision and the counts. A refusal keeps the existing validation panel with its codes.
- **`shared/ui/plan-render.js`.** One more export, `lessonCard`, for the lesson preview. No behaviour changed.
- **`shared/app-routes.js`.** The admin route now loads the five `1.1.0` block renderers before `plan-render.js` (which looks them up when it loads), so the preview draws all eleven block types; before, the admin builder's preview was blank for the five newer ones.
- **Layout.** On a wide screen (64rem and up) the plan view is two columns: the forms, and beside them a sticky rail of the preview, the issues, the last save's findings, and the history, which scrolls on its own so the preview stays beside the field being typed in. On a phone it is one column in the same order after the forms.

Hooks: `data-plan-issues` (value `panel` on the panel; the editor's inline lists keep the empty value), `data-plan-issue` (the code), `data-plan-issue-severity`, `data-plan-issue-link`, `data-plan-fix`, `data-plan-fix-choice`, `data-plan-preview`, `data-plan-preview-state`, plus `data-plan-issues-count`, `-informs`, `-refresh`, `-state`, `-body`, `-empty`, `-truncated`, `data-plan-issue-group`, `-issue-field`, `-issue-index`, `data-plan-fix-box`, `-fix-current`, `-fix-suggested`, `-fix-note`, `-fix-message`, `data-plan-preview-card`, `-preview-label`, `data-plan-publish-note`, `data-plan-jump`.

## Tests

- `test/plan-editor-issues-preview.test.js`: 43 tests, 43 pass. Pure helpers (`groupIssues`, `countsText`, `publishNote` has no approval wording, `currentValue`, `fixRequest` for a top-level field, a candidate, a nested field and a list element, with no fix, and without touching the page's copy, `forEditor`); the panel (code, message, group, counts; "blocks publish" only on hard errors; links call the host; empty, truncated, failed; the notes toggle; a stale answer is dropped); fixes (one request with path, field, value; read again unless the host did; a refusal's reason; candidates; `audio-stale` has no button and shows its note; a hard error has no fix box); the preview (overlay for a block, an item, a lesson, invalid JSON, an emptied field; `compute` draws the learner's own markup and shows `<img src=x onerror=…>` as text; all eleven block types draw; the lesson card with an unsaved title; a renderer that throws; `guardClick`; no network or `innerHTML` in the module); the builder (the panel and the publish line after load; inline issues at fields; jump to a block, an item, the lesson, a unit, the plan; "Use this value" is one `PATCH` with the builder's revision and then one issues read; a nested fix and a 409 with nothing stored; typing changes the preview after the debounce with no request and `unsaved` until the save, then `saved`; hostile text is text; the lesson card previews an unsaved title; a click on a preview link does nothing; every request is to a plan route, the schema, or the account list; Publish sends one POST with the revision and shows the counts; a refused publish shows its code and findings; unmount removes every listener and the timer; the builder reads each panel only through `root`, so a missing one is skipped).
- `test/chrome-plan-editor-smoke.js` (extended): a plan with a non-NFC word, a word that differs from a verified lexicon entry, and an edited sentence that kept its clip after a publish lists `pointing-form`, `lexicon-form`, and `audio-stale` as warnings (and no warning is called a blocker); the counts and the publish line ("Warnings do not stop publishing", no approval wording) show; a link jumps to the field, focuses it, and leaves it in view; "Use this value" on the `lexicon-form` item is exactly one write, stores the lexicon form, and the item disappears; typing in the passage changes the preview before any save, `data-plan-preview-state` is `unsaved` with the "not saved" label, then `saved` after the save; typed `<img src=x onerror=alert(1)>` is text and no element; `GET /api/plans/<draft>/published` is 404 from the same browser session; no request goes to a preview route; Publish shows `Published · 8 · 0 blocking · 4 warnings` and stores `published`. Layout at 360 and 1280 px with the issues panel populated: no overflow, targets, names, or contrast problems.
- Full `npm test`: **1443 tests, 1443 pass, 0 fail**. All 20 `test/chrome-*-smoke.js` pass.

## Screenshots (looked at)

`spikes/plan-panels-shots/`: `plan-top-`, `issues-`, `mark-` (which shows the preview) at 1280 and 360.

- At 1280 px the issues sit in the left rail under the preview: a card headed "בעיות (Issues)" with the counts in grey ("1 שגיאות חוסמות · 9 אזהרות"), a "Show notes" checkbox and Refresh, then "Blocks publish" with a pink box for the one hard error and a link button holding the path, then "Warnings — does not block publish", each code a bold grey line with its number, each issue a soft box with the message and the link. The messages are the server's English text; the link buttons show "מעבר לשדה (Go to the field)" over the path. The preview card above shows the learner's view: the word, a dark green "השמעה" button with a sound icon, and a wide dark "אפשר להמשיך" button, under the label "Preview — as saved". The plan toolbar shows the publish line in two sentences, a Publish button and a Reload button.
- At 360 px the same cards are full width and stacked; the link buttons wrap their path onto two lines; there is no sideways scroll.
- I did not look at a populated "Use this value" box in a screenshot (the smoke exercises it and the unit tests check its markup); I looked at it only through the DOM.
- Honest remarks: the first issues screenshots showed that the rail hid everything below the preview until it was scrolled (it scrolls on its own), so the history and the issues are a scroll away from the preview on a laptop; the preview is the first thing in the rail on purpose. The toolbar still shows a bare revision number and the word "draft" with no label (existing, not changed here). Not looked at: the dark theme and a screen reader.

## Deviations

- **No network request, with one exception.** Typing makes no API request. In Chrome, redrawing the learner's markup makes the browser revalidate `/shared/brand/sprite.svg` (the learner's sound icon, a `<use>`; the server marks static files `no-cache`). The smoke allows exactly that one file and fails on anything else. The unit test (no browser) sees zero requests.
- **`plan-render.js` gained an export** (`lessonCard`), and the admin route loads the five block renderers it was missing.
- **A fix inside a field patches one field's value in the page** (see above) so it can use `set_field`; INIT-004 SPEC-015 ac-1 forbids a local patch of the plan document, which this is not.
- **The preview scope is the selected block (with its items) or the lesson card.** A unit or the plan itself has no learner view to draw.
- **The editor's inline issues now come from the issues answer** when it has arrived (the last save's findings before that), so a field shows what the server last found for the whole plan, warnings included, not only what its last save returned.
- **Hooks.** `data-plan-issues` and `data-plan-issue` are also the editor's inline per-field lists (SPEC-011); the panel's value is `panel`, and the smoke and tests scope their queries to it.

## Not closed

1. **No independent review** (`security_review: standard`). The preview draws only the renderer's escaped output; the learner route is unchanged and answers 404 for a draft; every request is same-origin.
2. **`audio-stale` has no one-click fix.** There is no tool that regenerates audio for a plan field (SPEC-008), so the item shows the note and nothing else.
3. **A fix whose place is gone sends nothing** and says so ("This fix cannot be applied here"); the issues panel does not say which field to look at.
4. **Hebrew copy not read by a person who reads Hebrew; dark theme and screen reader not tried.** The check that every control has a name is the smoke's, which reads names by the browser's own rules.

## Files

New: `shared/admin/plan-issues.js`, `shared/admin/plan-preview.js`, `test/plan-editor-issues-preview.test.js`, this summary. (`plan-ui.js`, the CSS block, the test helpers, and the smoke are listed in SPEC-013's summary.)  
Edited: `shared/admin/plan-builder.js`, `shared/admin/plan-form.js` (`focusField`, `open`), `shared/ui/plan-render.js` (one export), `shared/app-routes.js`, the SPEC-015 file (status `in_review`, boxes ticked, notes).  
No commit. No change to `worksheets/`, the database, or any `.mp3`. No existing test changed.

## Provenance

`INIT-008/SPEC-015`
