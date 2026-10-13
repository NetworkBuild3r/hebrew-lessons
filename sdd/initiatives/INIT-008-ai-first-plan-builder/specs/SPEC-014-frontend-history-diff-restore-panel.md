# SPEC-014 · Frontend: History, diff, restore panel and edit-state display

## Metadata

```yaml
spec_id: SPEC-014
initiative_id: INIT-008
title: "See who changed what, compare two revisions, restore a revision, part, or field in one click"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-010, SPEC-011]
blocks: [SPEC-016]
```

---

## Description

**Context:** D2(c) gives both actors history, a diff, and a one-step restore: the MCP has `list_plan_revisions`, `get_plan_revision`, `diff_plan`, and `restore_plan_revision` (SPEC-009); the admin has the same four routes (SPEC-010). This spec is the admin's side. There is no lock to show: the screen shows who last wrote a field and what a person replaced, and makes undoing it one button. ADR-0011 Decisions 3 and 4 are the contract. The admin sees `by` (the account name) on human entries; the MCP does not.

**Scope (in):**

- **History panel** in the plan view: a list from `GET /api/plans/:id/revisions` (newest first, a "Load older" control past 50), each row with the revision number, the operation, the path and field, the time, and an actor mark: **a person** (with the account name) or **AI** (with the tool name) or **system**. The mark is text plus a small icon from the existing sprite (`shared/brand/icons/`), never colour alone. A filter by actor kind. Selecting a row opens its details.
- **Compare:** pick any two rows, or "this revision against the working copy", or "working copy against published", and call `GET /api/plans/:id/diff`. The result is a list of entries grouped by node path: the field, the value before, the value after, side by side, with Hebrew values in `dir="rtl"` `lang="he"` and English in `dir="ltr"`. A truncated diff says so and offers a path filter. A value cut by the server is marked.
- **Restore, one step:** next to each diff entry and each history row, "Restore this value" (field), "Restore this part" (path), and, on a history row, "Restore the whole plan to this revision". Each asks the admin to confirm once in plain words (what will change and that history is kept), then sends one `POST /api/plans/:id/restore` with the last `expectedRevision`. A 409 shows `revision_conflict` with the server's revision and a reload; a `validation` error shows the rule codes and writes nothing; `assign_required` is explained. After success the plan view reloads at the new revision and the history shows the restore row.
- **Edit-state display:** the plan view, from the `editState` in `GET /api/plans/:id/parts`, marks each field whose last writer is a person with a small "edited by a person" mark and, on activation, shows what it replaced (`previousValue`) with a "Restore the earlier value" button that sends the same restore. If an AI later overwrites a person's field, the mark disappears on the next reload and the history shows both writes. There is **no lock, no pin, and no disabled field**: every field is always editable.
- **Hooks:** `data-plan-history`, `data-plan-revision` (value: revision), `data-plan-actor` (value: `human`, `ai`, `system`), `data-plan-diff`, `data-plan-diff-entry`, `data-plan-restore`, `data-plan-restore-confirm`, `data-plan-edited-mark`. Bilingual labels, 24 px and 44 px floors, tokens, reduced motion, `textContent` for server text.
- **Smoke additions** to `test/chrome-plan-editor-smoke.js`: with a fixture plan edited once by a stub `ai` actor and once by the admin through the editor, open History and see two rows with the right actor marks; compare the two and see the field's before and after; restore the earlier value from the diff and see a third row `restore`; restore a whole revision and see the plan view at the new revision; trigger a 409 and see the message; assert no control is disabled by provenance.

**Scope (out):** issues and preview (SPEC-015); pruning history; a three-way merge screen (ADR-0003 rejected merging).

---

## Understood (agent's read of the intent)

- [x] und-1: Every action on this screen is one of the SPEC-010 routes. The browser holds no diff logic and no restore logic.
- [x] und-2: The confirm step lives in the UI, not the service. The MCP restores in one call (SPEC-005 und-2).
- [x] und-3: The "edited by a person" mark is information. It never disables, hides, or warns away an edit.

## Acceptance Criteria

- [x] ac-1 (REQ-012): the history panel renders one row per revision in the `GET .../revisions` result, newest first, each with an actor mark (`data-plan-actor`) that is text plus an icon; a person row shows the account name; "Load older" requests the next page; the actor-kind filter narrows the rows. _(Met. The mark is an icon from the brand sprite (`admin` for a person, `settings` for AI, `review` for the system) plus Hebrew with an English gloss, plus the account name or the tool name. "Load older" sends `beforeRevision` of the oldest row shown; the filter is the server's `actorKind`, not a client filter.)_
- [x] ac-2 (REQ-012): comparing two revisions, a revision against the working copy, and the working copy against published each send one `GET .../diff` with the right `from` and `to` and render one entry per server entry with before and after; Hebrew values have `lang="he"` `dir="rtl"`; a truncated result shows the notice. _(Met. Compare is two selects (a revision, the working copy, the published copy) with a Compare button, a "working copy vs. published" shortcut, and an "against the working copy" button on each row; "Open" on a row reads that revision and shows what it changed against the one before it (`from=n-1&to=n`). A path filter rides along as `path=`.)_
- [x] ac-3 (REQ-012, REQ-005): each restore control sends exactly one `POST .../restore` after one confirmation, with `fromRevision`, the right `path` and `field`, and the last `expectedRevision`; cancelling sends nothing; a 409 shows `revision_conflict` and the stored revision; a validation error shows codes and the plan view is unchanged. _(Met. The confirmation is an inline box (not `window.confirm`) under the control that was pressed, in Hebrew over English, saying what comes back and that history is kept, and warning when the form holds unsaved text. The restore takes the value from the earlier side of the comparison, so no restore is offered when that side is the working copy, and none for an added node or a moved list. A refusal appears inside the box, and a 409 adds a Reload button that reads the plan again.)_
- [x] ac-4 (REQ-012, REQ-006): a field whose `editState` entry exists shows the "edited by a person" mark; activating it shows the previous value and a restore button; **no field, control, or button is disabled, hidden, or blocked because of the mark** (a test walks every control of a plan with human-edited fields and asserts none carries `disabled` or `readonly` other than the six read-only plan fields from SPEC-011). _(Met in two forms. A unit test renders the same block form with and without marks and finds the same disabled controls (only end-of-list move buttons); the smoke walks every control of a block whose fields a person last wrote and finds none disabled or read only except the move buttons and the media panel's own gates, then saves an edit in a marked field. The mark is added by `setEditState` in `plan-form.js`, from `editState.humanEdited` of the admin's `GET .../parts`; the restore button there sends `fromRevision = revision - 1`, the state just before the person's write.)_
- [x] ac-5 (REQ-012): a learner never receives the history markup (the admin guard), asserted once.
- [x] ac-6 (REQ-012, REQ-017): `node test/chrome-plan-editor-smoke.js` exits 0 with the added history, compare, restore, and 409 checks; contrast, target-size, and reduced-motion checks still pass; no new string has a point, streak, or joke; the words "pinned" and "locked" appear in no new string.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Confirmation wording | One short confirmation in English with Hebrew above it, stating what changes and that history is kept. | resolved | yes — ADR-0001 #6 |
| aud-2 | Page size | 50 rows, as the service default. | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [x] `shared/admin/plan-history.js`; changes to `shared/admin/plan-builder.js` and `shared/admin/plan-form.js` (edit-state marks) and `shared/worksheet.css` (tokens only). _(The history rows use `data-plan-revision` as the spec lists. The toolbar's revision number already had that hook with no value; the smoke and the older tests read the first one in the page, which is still the toolbar's.)_
- [x] Tests: `test/plan-editor-history.test.js`; added checks in `test/chrome-plan-editor-smoke.js`

---

## Technical Requirements (domain-specific)

- [x] One HTTP call per action; no client-side diff or restore logic.
- [x] No dependency; no build step; icons from the existing sprite; server text through `textContent`.
- [x] Fixtures use `example-admin`; a stub `ai` actor is created through the MCP adapter's function with a fixture context.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-editor-history.test.js`, `node test/chrome-plan-editor-smoke.js`, and `npm test`.
- **Rollback plan:** revert; the plan view loses the panel.

---

## Security

- **Sensitivity:** standard. Triggers matched: admin UI that shows account names and posts a restore.
- **Review depth:** standard · **focus areas:** admin guard, CSRF on the restore post, escaped values.

---

## Verification Strategy

- **Claim:** an admin can see who changed each field, compare any two states, and put any value back in one confirmed click, and nothing on the screen stops them editing.
- **Check + executor:** `mechanized` — the commands above.
- **Pass condition:** exit 0; ac-1 through ac-6 each asserted; the walk in ac-4 finds zero provenance-disabled controls; the smoke observes the `restore` row.
- **Test levels:** unit with a fake DOM through `vm`; Chrome smoke against the signed-in test server.

---

## Integration Points

- SPEC-016 drives the same restore from the MCP and the UI in one scenario. SPEC-017 documents the screen.
- Produces **Change:** history, diff, restore, and edit-state marks in the admin.
- **Provenance tag:** `INIT-008/SPEC-014`
