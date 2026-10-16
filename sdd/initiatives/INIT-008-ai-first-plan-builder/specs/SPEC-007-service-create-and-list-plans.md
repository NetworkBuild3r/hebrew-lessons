# SPEC-007 · Service: `createPlan` and `listPlans`

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-008
title: "Plans start in the MCP: create a draft plan; list plans with status and publish state"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-004]
blocks: [SPEC-006]
```

---

## Description

**Context:** no tool or route creates a plan; only `scripts/import-lesson-plans.js`, `scripts/seed-starter-content.js`, and tests write a new row, and the `hebrew-worksheet` skill says so (`RSCH-009` §1.5). There is no list of plans either: the admin Plans tab uses a plan-id picker. D4 adds `create_plan` and `list_plans`. This spec writes the two functions; SPEC-009 and SPEC-010 are their adapters. ADR-0011 Decision 7 is the contract.

**Scope (in):**

- `server/plans/create.js`:
  - `createPlan(store, input, ctx)`; `input` is `{ id, title, recipe?, unitId?, lessonId? }`. `id` must match the schema `slug` pattern (`^[a-z0-9]+(?:-[a-z0-9]+)*$`); `title` is validated as the schema's text type. A plan that already exists returns `{ error: 'already_exists' }` and is not touched, so a retried call cannot overwrite a plan.
  - The new plan is the **smallest document the schema accepts as a draft**: `status: 'draft'`, `revision: 1`, `audience: 'shared'`, `assigneeId: null`, `schemaVersion` `1.0.0` (or `1.1.0` when `recipe` is given, because `recipe` is a `1.1.0` field), one unit and one lesson with `visibility: 'hidden'` (ADR-0003 §7, fail closed), empty `buildsOn`, `newWords`, `recycles`, and `blocks`. The executor reads the schema's `required` lists and `minItems` at execution and fixes the exact shape; if the schema requires a non-empty block list or non-empty text, the executor reports it and uses the least placeholder that passes, never generated Hebrew (ledger aud-1).
  - `recipe`, when given, must be one of the six schema recipes and is set on the lesson. Where the schema accepts empty block shells for that recipe's card (the block order on the recipe cards in the `hebrew-worksheet` skill's `recipes.md`), they are created with ids and empty required fields; otherwise only `recipe` is set. This is the "starting shape, not generated Hebrew" of initiative §16 Q5.
  - It validates with `validatePlan` and refuses on a write-time block rule (the same `refusesWrite`), writes through the SPEC-004 persist path (working copy at revision 1, a revision row with op `create`, an audit row `plan.create`), and returns `{ plan, revision: 1, status: 'draft', validation }`.
  - A plan is always created `shared`. Personal assignment is a separate call (`personal.assign`, SPEC-002), which keeps the account check in one place.
- `server/plans/store.js` gains the only new plan SQL, `listPlans()`: id, `revision`, `status`, `audience`, `assignee_id`, `updated_at`, the working-copy title (parsed in JS), and `hasSnapshot` with the snapshot's own `revision` (`json_extract` if available, else parsed in JS), capped at 200 rows ordered by `updated_at` descending.
- `server/plans/create.js` `listPlans(store, opts, ctx)`: returns `{ plans: [{ id, title, audience, assigned, status, revision, updatedAt, published, unpublishedChanges }], truncated }`. `published` is whether a snapshot exists; `unpublishedChanges` is whether the working revision is above the snapshot's revision. For an `ai` caller `assigned` is a boolean and `assigneeId` is omitted; for an admin caller `assigneeId` is returned too. The count of human-edited fields per plan (`humanEditedFields`) is added to this result by SPEC-006, which owns the marks.

**Scope (out):** assignment (SPEC-002); tools and routes (SPEC-009, SPEC-010); the creation UI (SPEC-013); generating lesson content.

---

## Understood (agent's read of the intent)

- [x] und-1: Creating a plan is the start of the AI-first loop. It needs no person and no approval.
- [x] und-2: `createPlan` never overwrites. A second call with the same id changes nothing and says so.
- [x] und-3: The scaffold is a shape. The AI or the admin writes the content afterward through the six operations.

## Acceptance Criteria

- [x] ac-1 (REQ-007): `createPlan` with a valid `id` and `title` stores a plan at revision 1, `status` `draft`, `audience` `shared`, one lesson with `visibility: 'hidden'`, passes `validatePlan` with no block finding, and writes one `plan_revisions` row (op `create`) and one `audit_log` row (`plan.create`, actor from the context). `read_part` with an empty path then returns the same document.
- [x] ac-2 (REQ-007): `createPlan` with an existing id returns `already_exists` and leaves the plan, its revision, and the log and audit tables unchanged. An id that is not a slug returns `invalid_id`; a title over the schema limit returns the validation error; nothing is written in either case.
- [x] ac-3 (REQ-007, Q5): `createPlan` with each of the six recipes sets that `recipe` on the lesson and `schemaVersion` `1.1.0`; the result validates with no block finding for every recipe; with no `recipe` the document stays `1.0.0`.
- [x] ac-4 (REQ-007): `listPlans` over a database holding a created plan, a published plan with later edits, and a personal plan returns one row each with correct `published`, `unpublishedChanges`, and `revision`; the result is capped at 200 with `truncated`; with an `ai` caller no row has `assigneeId` and a personal plan has `assigned: true`; with an admin caller `assigneeId` is present.
- [x] ac-5 (REQ-007): `listPlans` runs no SQL outside `server/plans/store.js` (source search), and does not read any learner table.
- [x] ac-6 (REQ-017): a created plan is not readable by a learner: `GET /api/plans/:id/published` answers 404 until the plan is published (the existing route, asserted here once).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Exact minimal shape | Read from the schema's `required` and `minItems` at execution. If the schema forces placeholder text, use the least that validates and say so in the completion summary. | resolved | yes — the schema is the authority |
| aud-2 | Recipe scaffolds in the UI | `createPlan` accepts `recipe` regardless. Whether the "New plan" screen offers it is initiative §16 Q5 (default yes, SPEC-013). | ambiguous | no — planning default applied |
| aud-3 | `assigneeId` to the MCP | Not returned; a boolean `assigned` instead. | ambiguous | no — planning default applied (initiative §16 A2) |

> **Readiness** = 100 against the stated defaults.

---

## Deliverables

- [x] `server/plans/create.js`
- [x] `listPlans` in `server/plans/store.js`
- [x] Tests: `test/plan-create-list.test.js`

---

## Technical Requirements (domain-specific)

- [x] The plan id and title are validated by the schema, not by a second copy of the pattern.
- [x] Prepared statements only. No new dependency. Temporary database in tests.
- [x] No learner name, typed answer, or transcript in a created plan or a test fixture.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-create-list.test.js` and `npm test`.
- **Rollback plan:** revert the commit; created rows remain valid plans.

---

## Security

- **Sensitivity:** standard. Triggers matched: a new write that creates content; a list that includes an account name for admins.
- **Review depth:** standard · **focus areas:** no overwrite, hidden-by-default lesson, the audience rule for `assigneeId`.

---

## Verification Strategy

- **Claim:** a plan can be created and listed without a person or a script, a retry cannot overwrite, and a new plan is invisible to learners until published.
- **Check + executor:** `mechanized` — `node --test test/plan-create-list.test.js`, then `npm test`.
- **Pass condition:** exit 0; ac-1 through ac-6 each asserted; the log, audit, and `plans` tables are compared before and after the refused calls.
- **Test levels:** integration on a temporary database; the 404 check uses the in-process HTTP server as `test/plan-routes.test.js` does.

---

## Integration Points

- SPEC-006 adds `humanEditedFields` to the list result. SPEC-009 and SPEC-010 adapt the two functions. SPEC-013 builds the list and the "New plan" flow on them. SPEC-016 starts its end-to-end run with `createPlan`.
- Produces **Change:** plan creation and listing.
- **Provenance tag:** `INIT-008/SPEC-007`

---

## Executor notes (2026-10-12)

- **The schema forces one block.** A lesson needs at least one block (`minItems: 1`), and every block that has text needs non-empty text, so the scaffold holds one `fluency-pass` placeholder (`id: placeholder`, 60 seconds, source `vocab`), which holds no text. The author inserts real blocks, then removes it. The unit title, the unit theme, and the lesson title all take the plan title (the schema requires all three to be non-empty text).
- **Recipe scaffolds.** The schema does not accept an empty shell for any recipe card's blocks (each needs text, a Hebrew line, or a non-empty list), so `recipe` only sets the lesson's `recipe` and the plan's `schemaVersion` to `1.1.0`. This is the spec's own fallback.
- `unitId` and `lessonId` are optional inputs (default `unit-1` and `lesson-1`), validated against the schema's slug rule, which `create.js` reads from the schema file and does not copy. `invalid_recipe` is the recipe error.
- `unpublishedChanges` is `false` for a plan that was never published.
