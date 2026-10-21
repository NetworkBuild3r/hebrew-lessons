# SPEC-015 · Frontend: Builder outline and block editors

## Metadata

```yaml
spec_id: SPEC-015
initiative_id: INIT-004
title: "Builder outline and block editors"
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
blocked_by: [SPEC-004, SPEC-014]
blocks: [SPEC-016]
```

---

## Description

**Context:** the builder is the human side of the same operations the MCP already calls. ADR-0005 forbids a second edit implementation. This spec is the outline and the block fields. Media and the validation panel are SPEC-016.

**Scope (in):**

- An admin page that loads a part with `GET /api/plans/:id/parts`, shows the lesson's block list, and can insert, move, and remove a block through the SPEC-004 routes.
- Editing a field sends `PATCH /api/plans/:id/fields` with `expectedRevision`. A 409 shows the conflict and does not clear the admin's text until they reload.
- Publish sends `POST /api/plans/:id/publish`.
- The page is admin-only, using the same shell guard as the current lesson editor.
- Preview of one block uses the SPEC-014 renderer on the working copy returned by the read. It is not a second markup language.
- The legacy worksheet editor stays. This page does not post to `upsert_lesson`.

**Scope (out):** media library and the N1–N12 panel (SPEC-016); learner cutover (SPEC-020).

---

## Understood (agent's read of the intent)

- [x] und-1: Move and insert send the index the API expects. They do not renumber block ids.
- [x] und-2: A conflict message names `revision_conflict` and the server's revision.

## Acceptance Criteria

- [x] ac-1 (REQ-010): the outline's insert, move, remove, field save, and publish each call the matching SPEC-004 route and do not contain a local JSON patch of the plan document.
- [x] ac-2 (REQ-002): a 409 response leaves the displayed draft text in the field and shows the stored revision.
- [x] ac-3 (REQ-010): a non-admin session does not receive the builder markup (redirect or the existing admin guard).
- [x] ac-4 (REQ-014): the legacy editor's `data-lesson-save` hook still exists. This spec does not remove it.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Where the page lives | A new admin view beside the current lessons screen, not a rewrite of `shared/admin/lesson-editor.js` in place. | resolved | yes — ADR-0005 §8 step 3, old editor remains |

> **Readiness** = 100.

---

## Deliverables

- [x] Admin builder view for outline and fields
- [x] Tests: `test/plan-builder.test.js`

---

## Technical Requirements (domain-specific)

- [x] Reuse `shared/ui/` for the preview string. Escape field values in the editor the same way.
- [x] Keep ADR-0006 Decision 8 hooks on the legacy editor. New builder hooks use new `data-*` names.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-builder.test.js` and `npm test`.
- **Rollback plan:** hide the builder route. Worksheets still save through `upsert_lesson`.

---

## Security

- **Sensitivity:** standard. Triggers matched: admin-only UI that posts plan JSON.
- **Security acceptance criteria:**
  - [x] The view is not rendered for role `user`.
  - [x] The client sends the revision the server last returned, not a client-invented document version stored only in the DOM as HTML.
- **Review depth:** standard · **focus areas:** authz, CSRF on the posts (the existing admin form pattern).

---

## Verification Strategy

- **Claim:** builder actions are the HTTP operations, and a conflict does not drop the admin's unsaved field text.
- **Check + executor:** `mechanized` — `node --test test/plan-builder.test.js`.
- **Pass condition:** exit 0; each action's request path matches the parity table; the 409 fixture keeps the textarea value.
- **Test levels:** the repo's existing admin-view test style (`test/admin-lesson-editor.test.js`). No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- SPEC-016 adds the media library and the validation list on this page.
- Produces **Change:** plan builder outline.
- **Provenance tag:** `INIT-004/SPEC-015`
