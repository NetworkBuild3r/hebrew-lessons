# SPEC-015 · Frontend: Issues panel and live preview as the learner

## Metadata

```yaml
spec_id: SPEC-015
initiative_id: INIT-008
title: "List every issue with a one-click fix where one is known; preview the unsaved edit as the learner would see it"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-010, SPEC-011, SPEC-012]
blocks: [SPEC-016]
```

---

## Description

**Context:** D3 makes issues first-class so a person can fix what the AI got wrong, and D5 adds a live preview as the learner. SPEC-008 produces issues with stable codes, id paths, fields, and fix hints; SPEC-010 serves them at `GET /api/plans/:id/issues`. Today the builder prints the findings of the last save or publish and previews a block from the last server read; unsaved text is not previewed (`RSCH-009` §1.4, High). The learner view of a block is rendered by `shared/ui/plan-render.js` (ADR-0005 §6: component functions that return escaped strings). The `lexicon-form` finding carries `fix.suggested`, so "use the lexicon form" needs no extra route (`RSCH-009` §1.1 design consequence: show the verified form beside the AI's, one click to take it; adapted from [10][14]). ADR-0011 Decisions 5 and 8 are the contract.

**Scope (in):**

- **Issues panel** in the plan view: fetched from `GET /api/plans/:id/issues` after load, after every successful save, and after a restore. Grouped hard errors first (marked "blocks publish"), then warnings ("does not block publish"), with a toggle to show informs. Each item shows the stable code (for example `pointing-form`, `field-language`, `lexicon-form`, `pointing-consistency`, `audio-stale`, `pointed-word`), the message, and the path and field as a link that opens that node's form and focuses the field. Counts are shown in the panel heading; an empty state says "no issues". A `truncated` result says so.
- **One-click fix:** where an issue has `fix.suggested`, a button "Use this value" shows the current and suggested values side by side (Hebrew in `dir="rtl"` `lang="he"`) and sends one `set_field` through the same save path as SPEC-011. `fix.candidates` (`pointing-consistency`) shows the forms as choices and sends the chosen one. A `fix` with neither (the `audio-stale` regenerate note) shows the instruction text only. After a fix the panel refreshes and the issue is gone, or the server's new finding appears.
- **Live preview as the learner:** a preview pane beside the form that renders the **unsaved form state** of the selected node (a block, a lesson, or an item) with the same component functions the learner view uses (`shared/ui/plan-render.js`), escaped, on each input event (debounced), in the learner's RTL layout with the Hebrew type size the player uses (RSCH-006 floors, ADR-0010 word cluster). The saved state is read through the existing admin `GET /api/plans/:id/parts`; **no new route** (SPEC-010 aud-1, initiative §16 Q4). The preview is read-only, makes no request on its own beyond that read, and is labelled "Preview — not saved" while the form differs from the stored value. It never reaches a learner: the learner route is unchanged and still answers 404 for a draft.
- **Publish panel text:** the publish control (in `plan-builder.js`, unchanged in behavior) shows the count of hard errors and warnings from the issues panel before the click and states plainly that warnings do not stop publishing. It asks for no approval; the AI may publish through the MCP and so may the admin.
- **Hooks:** `data-plan-issues`, `data-plan-issue` (value: code), `data-plan-issue-severity`, `data-plan-issue-link`, `data-plan-fix`, `data-plan-fix-choice`, `data-plan-preview`, `data-plan-preview-state` (value: `saved` or `unsaved`). Bilingual labels, 24 px and 44 px floors, tokens, reduced motion, `textContent` for server text and the escape helper for the preview strings.
- **Smoke additions** to `test/chrome-plan-editor-smoke.js`: with a fixture plan holding a non-NFC word, a word that differs from a seeded verified lexicon entry, and a published plan whose edited sentence kept its `audioPath`, open the plan and see the three codes as warnings; click "Use this value" on the `lexicon-form` item and see it disappear; type in a Hebrew field and see the preview change before any save, and see `data-plan-preview-state` `unsaved`, then save and see `saved`; assert the learner route still answers 404 for the draft from the same browser session.

**Scope (out):** a real-player preview route; changing any rule or its severity; audio generation (the `audio-stale` item shows the regenerate path text SPEC-008 produced); history (SPEC-014).

---

## Understood (agent's read of the intent)

- [x] und-1: The panel shows what the server reports. It recomputes no rule and no cap in the browser.
- [x] und-2: A warning is never presented as a blocker. The words "blocks publish" appear only on hard errors.
- [x] und-3: The preview renders the unsaved state locally with the shared components. It is not a second renderer and not a second markup language.

## Acceptance Criteria

- [x] ac-1 (REQ-013, REQ-008): the panel renders every issue in a stubbed `GET .../issues` result with its code, message, severity group, and a link that opens the node's form and focuses the field; hard errors are labelled "blocks publish" and warnings "does not block publish"; the heading shows the counts; `truncated` shows its notice. _(Met. Grouped by severity (hard first, then warnings, then notes behind a toggle that re-reads with `severity=all`), then by code. The same issues also show inline at their fields, from the same answer. A link opens the node's section, selects the block, opens an item's form, and focuses the field: block, item, lesson, unit and plan paths are each tested.)_
- [x] ac-2 (REQ-013): "Use this value" sends exactly one `set_field` with the issue's path, field, and `suggested`, through the SPEC-011 save path, then refetches issues; a `pointing-consistency` choice sends the chosen candidate; an `audio-stale` item has no apply button and shows the instruction text. _(Met. A fix on a top-level field is that field with the suggested value. A fix inside a field (`newWords/1`, `cloze/items/0/answer`) sends that one field whole with the sub-value put in place, since `set_field` takes a field of a node: `fixRequest` in `plan-issues.js` copies the one field from the page's copy of the working copy and changes the one place; nothing else is built or sent. That is a small local patch of a field value, which INIT-004 SPEC-015 ac-1 forbids for the plan document; the plan document is never patched in the browser.)_
- [x] ac-3 (REQ-013): typing in a Hebrew field changes the preview pane within the debounce interval with no network request; `data-plan-preview-state` is `unsaved` while the form differs from the stored value and `saved` after a successful save; a value containing `<img onerror=…>` appears as text in the preview. _(Met, with one honest qualification. Typing makes no API request (the unit test sees zero requests; the smoke sees none to `/api/`). In Chrome, redrawing the learner's markup makes the browser revalidate `/shared/brand/sprite.svg`, the sound icon the learner renderer uses, because the server marks static files `no-cache`; that single same-origin GET of an icon file is the only request, and the smoke allows exactly that file. The preview scope is the selected block (with its item forms), or the lesson card when no block is selected; `plan-render.js` gained one export, `lessonCard`, for the lesson view. The five `1.1.0` block renderers are now loaded in the admin route, before `plan-render.js`, so every block type previews.)_
- [x] ac-4 (REQ-013, REQ-017): the page makes no request to a new preview route (none exists), and `GET /api/plans/:id/published` answers 404 for the draft in the same session; only same-origin requests are made. _(Met. The page makes no request to a preview route (none exists); the smoke reads `GET /api/plans/<draft>/published` from the same browser session and gets 404; every request in the run is same-origin. Clicks on a link or button inside the preview are stopped (`guardClick`), so the learner's "continue" links cannot change the admin page's hash.)_
- [x] ac-5 (REQ-013, REQ-019): the publish control states the hard-error and warning counts, says warnings do not stop publishing, and has no approval step; with a hard error present the server's refusal is shown with its codes. _(Met. The line above Publish reads the counts from the same issues answer and says warnings do not stop publishing; Publish sends `{ expectedRevision }` and nothing else. After a publish the notice shows the new revision and the counts. A refusal keeps the existing validation panel (`snapshot was not published: validation` and the findings with their rule codes) and `pending_media` shows its code.)_
- [x] ac-6 (REQ-013, REQ-017): `node test/chrome-plan-editor-smoke.js` exits 0 with the added issues, fix, preview, and 404 checks; contrast, target-size, and reduced-motion checks still pass; each control has an accessible name; no new string has a point, streak, or joke.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Preview of unpublished work | Rendered in the editor with the learner components; no new server route (initiative §16 Q4). | ambiguous | no — planning default applied; a real-player preview would be a follow-up |
| aud-2 | Preview scope | The selected node (block, lesson, item), not the whole plan at once. | resolved | yes — keeps the work within the spec size |
| aud-3 | Debounce | 250 ms, a product choice. | resolved | yes |

> **Readiness** = 100 against the stated defaults.

---

## Deliverables

- [x] `shared/admin/plan-issues.js`, `shared/admin/plan-preview.js`; changes to `shared/admin/plan-builder.js` (publish text, panel mounts) and `shared/admin/plan-form.js` (focus links: `focusField` and `open`; input events needed no change, the builder's panel-level `input` and `change` listeners schedule the preview)
- [x] Tests: `test/plan-editor-issues-preview.test.js`; added checks in `test/chrome-plan-editor-smoke.js`

---

## Technical Requirements (domain-specific)

- [x] The preview uses `shared/ui/plan-render.js` and the existing escape helper; no `innerHTML` with unescaped text; no second renderer.
- [x] One HTTP call per fix, through the same save path as every edit. No rule logic in the browser.
- [x] No dependency; no build step. Hebrew fixtures are hand-written.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-editor-issues-preview.test.js`, `node test/chrome-plan-editor-smoke.js`, and `npm test`.
- **Rollback plan:** revert; the plan view loses the panel and the pane.

---

## Security

- **Sensitivity:** standard. Triggers matched: admin UI rendering untrusted plan text in a learner-style view.
- **Review depth:** standard · **focus areas:** escaped preview strings, no draft reachable by a learner, no request outside the origin.

---

## Verification Strategy

- **Claim:** an admin can see every issue the server reports, fix the ones with a known fix in one click, and see an unsaved edit as the learner would, without anything unpublished reaching a learner.
- **Check + executor:** `mechanized` — the commands above.
- **Pass condition:** exit 0; ac-1 through ac-6 each asserted; the smoke observes the three warning codes, the cleared `lexicon-form` item, the `unsaved` then `saved` states, and the learner 404.
- **Test levels:** unit with a fake DOM through `vm`; Chrome smoke against the signed-in test server.

---

## Integration Points

- SPEC-016 drives the same fixes from the MCP (`list_plan_issues`, `set_field`) and from this panel. SPEC-017 documents both.
- Produces **Change:** issues panel and live preview.
- **Provenance tag:** `INIT-008/SPEC-015`
