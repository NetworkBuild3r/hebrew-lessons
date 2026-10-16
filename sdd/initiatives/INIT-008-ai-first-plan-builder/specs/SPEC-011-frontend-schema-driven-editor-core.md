# SPEC-011 · Frontend: Schema-driven editor core

## Metadata

```yaml
spec_id: SPEC-011
initiative_id: INIT-008
title: "Render an edit form for any plan node from the plan JSON Schema; widget registry; generic JSON fallback"
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
blocked_by: [SPEC-010]
blocks: [SPEC-012, SPEC-013, SPEC-014, SPEC-015]
```

---

## Description

**Context:** the admin builder is `shared/admin/plan-builder.js`, an IIFE in the `var` style that mounts through the seam `shared/admin/admin-shell.js` already calls (`PlanBuilder.mount(panel, planId)`, `mountPicker(panel)`, `unmount()`). It holds hand-written lists — six block types, per-type fields, per-item fields, a field whitelist (`shared/admin/plan-builder.js:20`–`:50`) — so it has no form for the five `1.1.0` blocks, the lesson `recipe`, or for pattern, cloze, support, glosses, `newWords`, `buildsOn`, `recycles`, or `visibility` (`RSCH-009` §3, High). Every edit it makes is an HTTP call; it holds no plan JSON edit logic and must not start to (INIT-004 SPEC-015 ac-1). ADR-0011 Decision 8 and `RSCH-009` §1.3 (a schema-derived form plus a generic fallback) are the contract. No dependency and no build step: the renderer is plain browser JavaScript served like the other `shared/` files.

**Scope (in):**

- `shared/admin/plan-form.js` (IIFE, same global-attachment style as `plan-builder.js`) — the renderer. It reads the plan JSON Schema from `GET /api/plan-schema` (SPEC-010) once per mount, resolves `$ref` against `#/definitions/` only (no remote refs), and for a selected plan node builds a form: for the plan (every property except `units`), a unit, a lesson, a block (the `oneOf` member chosen by its `type` const), or a bucket item (`wordEntry`, `paragraph`, `line`, `question`). The sub-schema for a node comes from walking the schema along its id path.
- `shared/admin/plan-widgets.js` — the widget registry and the default mapping. A widget is `{ test(schemaNode, ctx) -> priority, render(...) }`, picked by highest priority (the pattern JSON Forms documents, `RSCH-009` [41], hand-written here). Defaults by schema shape: string → single-line input, or a textarea when `maxLength` > 200; `enum` → select; boolean → checkbox; integer → number input; array of strings → string-list editor; array of objects that each have an `id` → repeating item editor; **anything else → the generic JSON widget**.
- **Generic JSON fallback.** For any node or field with no widget (an unmatched `oneOf`, a map such as `glosses`, an array of arrays, `migration`), the form shows the value as pretty-printed JSON in a textarea with a Save button. A JSON syntax error is shown inline and nothing is sent. Valid JSON is sent as `set_field` (one field) or `replace_part` (a whole node) and the server validates. **No node of the schema is ever uneditable.**
- **Saving.** A scalar or list field edit sends `PATCH /api/plans/:id/fields` (`set_field`) with the node's id path, the field, the whole new value, and the last `expectedRevision` the server returned. Adding, removing, or reordering items inside a block sends `set_field` for that bucket field with the whole new array, preserving ids and generating a unique new id for an added item. Block insert, move, and remove keep using the existing routes (SPEC-012 adds the new-block form). The editor builds **no** local patch of the plan document and edits no JSON itself. A 409 keeps the typed text in the field, shows `revision_conflict` and the server's revision, and offers "Reload" (the existing `data-plan-conflict` behavior). A validation or other error shows the server's rule codes and messages and keeps the text.
- **Mount.** `plan-builder.js` keeps mounting, publishing, and its media and picture panels with their hooks unchanged (`data-plan-pictures`, `data-plan-media*`, `data-plan-upload*`, `data-plan-gen*`, `data-plan-approve*`), and delegates the outline and field forms to `plan-form.js`. `shared/app.html` and the admin shell load the two new files in the same way the existing admin scripts load.
- **Markup.** Bilingual labels, Hebrew over English, per ADR-0001 #6: a field label is the schema property name from the hints (SPEC-012), falling back to the property name itself. Hebrew-language fields are inputs with `dir="rtl"` and `lang="he"`; English fields `dir="ltr"` and `lang="en"`; the language comes from the hints (SPEC-012), defaulting to `dir="auto"` here. All text from the server is set as text, never as HTML. New hooks use new names: `data-plan-form`, `data-plan-field` (value: the field name), `data-plan-widget` (value: the widget name), `data-plan-json`, `data-plan-node` (value: the id path), `data-plan-save`. Controls meet the ADR-0008 and ADR-0010 floors (24 px AA, 44 px primary) and use the existing tokens in `shared/worksheet.css`; no new raw hex. Reduced motion is respected.
- **Headless Chrome smoke.** Create `test/chrome-plan-editor-smoke.js` on the pattern of `test/chrome-restyle-a11y-smoke.js` (`test/helpers/chrome-cdp.js`, `test/helpers/signed-in-server.js`): sign in as the test admin, open `#plan/<id>` for a fixture plan, assert the form for a lesson renders with a widget per schema property, edit one field and see the saved value after reload, trigger a 409 and assert the text is kept, and assert the page makes only same-origin requests. SPEC-012 to SPEC-015 add their checks to this same script in the same change as their hooks.

**Scope (out):** per-block hints and widgets, the new-block form, and the drift test (SPEC-012); the plan list and create flow (SPEC-013); history (SPEC-014); issues and preview (SPEC-015).

---

## Understood (agent's read of the intent)

- [x] und-1: Correctness of the form against the schema is the server's job. The editor shows what the schema says and sends operations; the server validates and refuses.
- [x] und-2: The generic fallback is not a degraded mode. It is what guarantees a new block type or field is editable the day it ships.
- [x] und-3: `audience`, `assigneeId`, `id`, `revision`, `status`, and `schemaVersion` render read-only in the plan form. Audience and assignee change only through the assign control (SPEC-013), which calls the route over `assignPlan`.

## Acceptance Criteria

- [x] ac-1 (REQ-010): for each node kind (plan, unit, lesson, each of the 11 block types, each bucket item type) the renderer, given the real schema from `docs/decisions/ADR-0003-assets/lesson-plan.schema.json`, produces a form with one control per schema property except child collections, each with a `data-plan-field` and a `data-plan-widget`; a property with no registered widget gets the JSON widget (a unit test counts them and names none missing). _(Met. 	est/plan-editor-widgets.test.js renders the plan, unit, lesson, all eleven block types, and the four bucket items, and compares the control list with the schema's properties; the only JSON-widget property on a block is the hinted glosses map, and migration on the plan.)_
- [x] ac-2 (REQ-010): editing a string field sends exactly one `PATCH /api/plans/:id/fields` with the id path, field, value, and the last `expectedRevision`; editing a list adds or removes an item with unique ids and sends one `set_field` with the whole array; no request body contains a plan document the editor assembled (a test asserts the request count and bodies against a stub). _(Met. The one deviation: an emptied optional field is sent as a 
eplace_part of the node as loaded minus that one property, because no operation unsets a field; the body is a node, never a plan document, except for the plan root. 	est/plan-editor-core.test.js, 	est/plan-builder.test.js.)_
- [x] ac-3 (REQ-010): invalid JSON in the fallback sends nothing and shows an inline error; valid JSON sends one operation.
- [x] ac-4 (REQ-010): a 409 leaves the typed text in its control, shows the code `revision_conflict` and the server's revision, and keeps focus handling per ADR-0006; a validation error shows the rule codes and keeps the text.
- [x] ac-5 (REQ-017): `audience`, `assigneeId`, `id`, `revision`, `status`, and `schemaVersion` render read-only; every string from the server is set with `textContent`, so a value containing `<img onerror=…>` renders as text (a test).
- [x] ac-6 (REQ-017): the existing hooks that `test/plan-builder.test.js`, `test/plan-builder-media.test.js`, `test/admin-lesson-editor.test.js`, and `test/chrome-admin-smoke.js` query still exist or those tests change in this same change; the legacy lesson editor's `data-lesson-save` hook is untouched; `npm test` shows no removed test. _(Met. Changed in the same change: 	est/plan-builder.test.js and 	est/plan-builder-media.test.js now load the three new scripts and answer /api/plan-schema (the builder fetches it); the test that called ieldsForBlock and the old 
ewBlock was rewritten to read the editor form and the schema-generated 
ewBlock, not removed. 	est/design-tokens.test.js was not touched. Two other smokes, chrome-guides-smoke.js and chrome-verb-workshop-smoke.js, were failing before this work because SPEC-002 made a write context mandatory; they now pass a system context.)_
- [x] ac-7 (REQ-010, REQ-017): `node test/chrome-plan-editor-smoke.js` exits 0: the lesson form renders, a saved edit survives a reload, a 409 keeps the text, no request leaves the origin; and at 360 px wide there is no horizontal overflow, every control is at least 24 px, the primary Save at least 44 px, and each input has an accessible name. _(Met, with the page at 360 px in a non-mobile emulation: Chrome's mobile emulation widens the layout viewport to fit content and hides overflow, which the older restyle smoke does not guard against. Two overflows were found and fixed: the old picture-upload file input, and the learner preview.)_

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Where the editor lives | Inside the existing `#plan/<id>` view, replacing the outline and field forms; `plan-builder.js` keeps media, pictures, and publish. | resolved | yes — the seam `admin-shell.js` already calls |
| aud-2 | Schema subset | `type`, `properties`, `items`, `enum`, `const`, `oneOf` with a `type` const, `$ref` to `#/definitions/`, `minLength`, `maxLength`, `pattern`, `minimum`, `maximum`. Anything else becomes the JSON widget. | resolved | yes — the keywords the validator already lists in `SCHEMA_KEYWORDS` |
| aud-3 | Saving a list | Whole array via `set_field`, not a new item operation. | resolved | yes — no new operation is added (ADR-0003 §4) |

> **Readiness** = 100.

---

## Deliverables

- [x] `shared/admin/plan-form.js`, `shared/admin/plan-widgets.js`
- [x] Changes to `shared/admin/plan-builder.js` (delegate the outline and fields), `shared/app.html` and the admin script list, and `shared/worksheet.css` (tokens only)
- [x] Tests: `test/plan-editor-core.test.js`; `test/chrome-plan-editor-smoke.js`; updated `test/plan-builder.test.js` where a hook moved

---

## Technical Requirements (domain-specific)

- [x] No dependency, no build step, no remote script; self-hosted files only. IIFE and `var` style as the surrounding admin files.
- [x] All dynamic text through `textContent` or the existing escape helper. No `innerHTML` with server data.
- [x] The editor sends `expectedRevision` as the server last returned it, not a value held only in markup.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-editor-core.test.js test/plan-builder.test.js test/plan-builder-media.test.js`, `node test/chrome-plan-editor-smoke.js`, and `npm test`.
- **Rollback plan:** `plan-builder.js` falls back to its previous outline code by revert; routes are unchanged.

---

## Security

- **Sensitivity:** standard. Triggers matched: admin UI that renders untrusted plan text and posts plan JSON.
- **Review depth:** standard · **focus areas:** escaped rendering, CSRF on the posts (the existing admin form pattern), the JSON fallback sending only through the operations.

---

## Verification Strategy

- **Claim:** any plan node can be edited from a form generated from the schema, with a JSON fallback for the rest, and every save is one operation with the server's revision.
- **Check + executor:** `mechanized` — the unit and smoke commands above.
- **Pass condition:** exit 0 for each; ac-1's test counts widgets and JSON fallbacks over the real schema; ac-2's test asserts request bodies against a stub; the smoke asserts the viewport and request-origin checks.
- **Test levels:** unit for the renderer and widget mapping (fake DOM through `vm`, as `test/plan-builder.test.js`); integration through headless Chrome against the signed-in test server (the pattern of `test/chrome-restyle-a11y-smoke.js`).

---

## Integration Points

- SPEC-012 adds hints, the widgets for each block type, the new-block form, and the drift test. SPEC-013, SPEC-014, and SPEC-015 mount panels beside the form and extend the same smoke script.
- Produces **Change:** the schema-driven form renderer.
- **Provenance tag:** `INIT-008/SPEC-011`
