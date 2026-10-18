# SPEC-013 · Frontend: Plan list, create-plan flow, assign control

## Metadata

```yaml
spec_id: SPEC-013
initiative_id: INIT-008
title: "A real list of plans, a New plan flow with optional recipe scaffold, and the assign control"
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

**Context:** the admin Plans tab is a plan-id picker (`PlanBuilder.mountPicker`, `shared/admin/admin-shell.js:150`–`:154`), and nothing creates a plan from the UI. D5 wants an admin to create a plan from scratch or modify any plan. SPEC-007 and SPEC-010 added `listPlans`, `createPlan`, and `assign` and their routes (`GET /api/plans`, `POST /api/plans`, `POST /api/plans/:id/assign`). The `PlanBuilder.mount`/`mountPicker`/`unmount` seam stays. ADR-0011 Decisions 7 and 8 are the contract.

**Scope (in):**

- **Plan list** replacing the picker in `mountPicker`: a table from `GET /api/plans` with title, id, audience (and, for an admin, the assignee), status (draft or published), revision, "unpublished changes" when set, and the count of fields a person last edited (`humanEditedFields`). Each row opens `#plan/<id>`. An empty list says so and points at "New plan". Rows are links with an accessible name; the list has a heading and is a real table (header cells, scope). A load error shows the server's code and a retry.
- **New plan flow:** a form with `id` (the schema slug pattern, checked in the browser with the same pattern for fast feedback, and by the server regardless), `title`, and — per initiative §16 Q5, default yes — an optional recipe picker over the six recipes with a one-line note that a recipe sets a starting shape and writes no Hebrew. It sends `POST /api/plans` and opens the new plan on success. `already_exists` shows a plain message and changes nothing. The picker is built from the schema's recipe enum, not a copy of the list.
- **Assign control** in the plan view: audience (shared or personal) and, for personal, an assignee chosen from active accounts (the existing admin users read, `GET /api/admin/users`, already used by the Users view) — typed free text is not accepted. It sends `POST /api/plans/:id/assign` with `expectedRevision`. `assignee_refused` is shown plainly. This is the only place `audience` and `assigneeId` can be changed from the UI.
- **Hooks:** `data-plan-list`, `data-plan-row` (value: plan id), `data-plan-new`, `data-plan-new-id`, `data-plan-new-title`, `data-plan-new-recipe`, `data-plan-create`, `data-plan-assign`, `data-plan-assign-audience`, `data-plan-assign-user`. Bilingual labels, 24 px and 44 px floors, tokens, reduced motion, and `textContent` for server text, as SPEC-011.
- **Smoke additions** to `test/chrome-plan-editor-smoke.js`: with two fixture plans, the list shows both with correct status; create a plan with a recipe through the form and land on its editor; create with an existing id and see the message; assign a plan to a fixture active account and see `personal`, then back to `shared`; assign to a disabled account and see the refusal.

**Scope (out):** history (SPEC-014); issues and preview (SPEC-015); generating any lesson content; the legacy lesson editor.

---

## Understood (agent's read of the intent)

- [x] und-1: Creating a plan in the UI is the same function as `create_plan`. The form adds no field the tool lacks.
- [x] und-2: The assignee is picked from accounts, so the UI cannot send a name that does not exist. The server still checks.
- [x] und-3: If the operator answers "no" to recipe scaffolds (§16 Q5), only the picker is removed; `recipe` stays an optional argument of the route and tool.

## Acceptance Criteria

- [x] ac-1 (REQ-011): the list renders one row per plan in the `GET /api/plans` result with the columns above, a link to `#plan/<id>`, and an empty state; at 360 px wide it has no horizontal overflow (it reflows or scrolls inside its own container). _(Met. Six columns: plan (title link and id), status with an "unpublished changes" mark, audience with the assignee, revision, fields a person last wrote, updated. Below 40rem each row becomes a card of label and value pairs; the table keeps its header cells, `scope`, and explicit ARIA roles. The smoke audits the list at 360 and 1280 px: no overflow, targets, names, contrast.)_
- [x] ac-2 (REQ-011): the New plan form sends exactly one `POST /api/plans` with `id`, `title`, and the chosen `recipe` (or none), opens the editor for the returned id, and shows `already_exists` and `invalid_id` as plain messages without losing the typed values; the recipe options equal the schema's recipe enum (a test compares them). _(Met. The id pattern, the title limit, and the recipe list are read from the schema (`GET /api/plan-schema`); with no schema there is no recipe picker, not a copied list. An id the server refuses or a title it refuses is shown at its field or under the form; nothing typed is cleared.)_
- [x] ac-3 (REQ-011, REQ-002): the assign control offers only active accounts, sends one `POST /api/plans/:id/assign`, shows `assignee_refused` plainly, and is the only control that can change `audience` or `assigneeId` (the plan form from SPEC-011 renders both read-only; a test asserts no other control sends them). _(Met. The accounts come from `GET /api/admin/users`, active ones only, in a `<select>`; the control has no text input. A disabled assignee is not offered; one sent anyway is `assignee_refused`, in plain words. Found while building: the schema then refused a username under 8 characters (the harness's `admin` and `learner`) with `validation` / `schema` at `/assigneeId`. That pattern was later relaxed to the users-table username rule, so those names validate. The control still shows a schema finding's path and message, because it has no field to show it at.)_
- [x] ac-4 (REQ-011): a learner (role `user`) never receives the list or the New plan markup (the existing admin guard), asserted once. _(Met in the smoke: signed in as the learner, `/admin/#plans` and `/admin/#plan/<id>` hold none of the list, New plan, assign, history, or issues markup, and the plan routes answer 403.)_
- [x] ac-5 (REQ-011, REQ-017): `node test/chrome-plan-editor-smoke.js` exits 0 with the added list, create, and assign checks; contrast, target-size, and reduced-motion checks in `test/chrome-restyle-a11y-smoke.js` still pass for the admin routes; no new string has a point, streak, or joke.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Recipe scaffolds in "New plan" | Offered, as a starting shape only (initiative §16 Q5). | ambiguous | no — planning default applied; the operator may drop the picker without touching the route or the tool |
| aud-2 | Where the account list comes from | The existing admin users route. No new route. | resolved | yes |
| aud-3 | `humanEditedFields` column | Shown as a number with a short label; clicking the row opens the plan. | resolved | yes |

> **Readiness** = 100 against the stated defaults.

---

## Deliverables

- [x] Changes to `shared/admin/plan-builder.js` (`mountPicker` and the plan view's assign control) and `shared/admin/plan-form.js` for the read-only fields' link to the control _(`plan-form.js` needed no change for this: SPEC-011 already renders both fields read-only with a help line pointing at the control. The assign control is a persistent panel from `plan-list.js` that the builder mounts.)_
- [x] `shared/admin/plan-list.js` (list, New plan, and the assign control), with `shared/admin/plan-ui.js` (helpers the three new panel files share); both are listed in the admin route in `shared/app-routes.js`
- [x] Tests: `test/plan-editor-list-create.test.js`; added checks in `test/chrome-plan-editor-smoke.js`

---

## Technical Requirements (domain-specific)

- [x] Every action is one HTTP call to a route from SPEC-010. No client-side plan JSON, no id-path parsing, no second slug pattern that disagrees with the schema's.
- [x] No dependency; no build step; server text through `textContent`.
- [x] Fixtures use `example-admin` and `example-learner`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-editor-list-create.test.js`, `node test/chrome-plan-editor-smoke.js`, and `npm test`.
- **Rollback plan:** revert; the id picker returns.

---

## Security

- **Sensitivity:** standard. Triggers matched: admin UI that creates content and changes assignment; lists account names to admins.
- **Review depth:** standard · **focus areas:** the admin guard, no free-text assignee, escaped rendering.

---

## Verification Strategy

- **Claim:** an admin can see every plan, create one from the UI with the same function as the MCP tool, and change who a personal plan is for only through the checked assign call.
- **Check + executor:** `mechanized` — the commands above.
- **Pass condition:** exit 0; ac-1 through ac-5 each asserted; the smoke observes the created plan and the assign refusal.
- **Test levels:** unit with a fake DOM through `vm`; Chrome smoke against the signed-in test server.

---

## Integration Points

- SPEC-014 and SPEC-015 add panels in the plan view. SPEC-016 creates a plan from the MCP and finds it in this list.
- Produces **Change:** plan list, create flow, assign control.
- **Provenance tag:** `INIT-008/SPEC-013`
