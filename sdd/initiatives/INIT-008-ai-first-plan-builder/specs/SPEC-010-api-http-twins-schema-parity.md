# SPEC-010 · API: HTTP twins, the schema route, and the parity test

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-008
title: "Admin routes for every new operation, an admin-only plan-schema route, and a parity test over the capability table"
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-009]
blocks: [SPEC-011, SPEC-013, SPEC-014, SPEC-015, SPEC-016]
```

---

## Description

**Context:** the admin editor is a client of the same operations as the MCP (D4). Plan routes are admin-only through `addAdmin(router, method, path, handler)` (`router.requireSession(router.requireAdmin(handler))`) in `server/api/plan-routes.js`; they call one function and map the result with `sendMapped` (a 409 for `revision_conflict`, 404 for `not_found`, 400 with the `error` code otherwise); the write routes are listed in `WRITE_ROUTES` so a test can iterate them and a new write cannot skip the admin check (`server/api/plan-routes.js:23`–`:30`). Mutating CSRF and Origin checks run in `server/web.js` before dispatch, as for every `/api/` write. The learner route `GET /api/plans/:id/published` answers 404 for a draft. SPEC-009 added the MCP tools. This spec adds the HTTP side and the test that keeps both sides in step. ADR-0011 Decision 7 and initiative §12 are the contract.

**Scope (in):** new admin-only routes in `server/api/plan-routes.js`, each calling the same function the matching MCP tool calls, with an `admin` context built from the session username (`example-admin` in tests):

| Method and path | Calls | Notes |
|-----------------|-------|-------|
| `GET /api/plans` | `listPlans` | Admin audience: `assigneeId` included. |
| `POST /api/plans` | `createPlan` | Body `id`, `title`, optional `recipe`. `already_exists` is HTTP 400 with that code (409 stays reserved for `revision_conflict`); no overwrite. |
| `POST /api/plans/:id/assign` | `personal.assign` | Body `audience`, `assigneeId`, `expectedRevision`. |
| `GET /api/plans/:id/revisions` | `revisions.list` | Query `sinceRevision`, `actorKind`, `pathPrefix`, `limit`; unknown keys are 400. |
| `GET /api/plans/:id/revisions/:revision` | `revisions.get` | |
| `GET /api/plans/:id/diff` | `revisions.diff` | Query `from`, `to` (a revision number, `working`, or `published`), optional `path`. |
| `POST /api/plans/:id/restore` | `revisions.restore` | Body `fromRevision`, optional `path` and `field`, `expectedRevision`. |
| `GET /api/plans/:id/issues` | `issues.list` | Query `severity`, `code`, `pathPrefix`, `limit`. |
| `GET /api/plan-schema` | `validate.schemaText` (a small export that returns the schema file text the validator already loads) | Admin only. The editor reads it to build forms (SPEC-011). Cacheable per process; ETag optional. |

- The existing seven plan routes keep their paths, bodies, and status mapping. Their results gain `editState` (`GET .../parts`) and `overwrote` (writes) as additive keys; each passes an `admin` context. The new write routes (`POST /api/plans`, `POST .../assign`, `POST .../restore`) are appended to `WRITE_ROUTES` so the existing admin-check iteration covers them.
- Request bodies reject unknown keys (`allowedKeys`); `actor`, `by`, and `kind` are unknown keys, so a client cannot name its actor. Every route requires a session and the admin role; a learner gets 403, a signed-out caller 401.
- **No new preview route.** The editor previews unsaved form state with `shared/ui/` components and reads the saved working copy through the existing admin `GET /api/plans/:id/parts` (empty path). The learner route is unchanged and still answers 404 for a draft (initiative §16 Q4).
- **The parity test.** `test/helpers/plan-parity-table.js` holds initiative §12 as data: operation, function, MCP tool, HTTP route, and the rows' exemptions (the schema route, "UI plumbing", is the one with no MCP tool; the human-only acts are listed with their `forbidden` result). `test/plan-builder-parity.test.js` asserts: every MCP tool in the table is registered; every route in the table is registered; for each row with both sides, a spy on the function sees the same function called with an equivalent argument through the tool and through the route; no route or tool handler in the plan files edits plan JSON itself (a source search for `JSON.stringify(` / assignments into a plan inside the two adapter files); `WRITE_ROUTES` ⊇ every mutating route in the table; and a row added to the table without both adapters fails the test. The learner route's 404 for a draft is asserted once more here.

**Scope (out):** any UI (SPEC-011 onward); a real-player preview route; resources and prompts.

---

## Understood (agent's read of the intent)

- [x] und-1: Each route is auth, parse, call, shape. The parity test enforces that they stay that way.
- [x] und-2: The schema route is UI plumbing, not a user action. It is the one row of the capability table with no MCP tool, and the table says why.
- [x] und-3: `sendMapped` already maps an unknown `error` to 400 with the code, so `id_mismatch`, `assign_required`, `already_exists`, `no_snapshot`, and `revision_corrupt` need no new mapping. _True for the codes. The mapping moved into `sendFailure` (shared with a new `sendRead` for the read routes), and a 400 or 404 body now also carries `field` or `reason` when a service names one. Existing bodies are unchanged._

## Acceptance Criteria

- [x] ac-1 (REQ-009): every route in the table above answers 401 signed out, 403 for a role `user` session, and 200 or the documented error for an admin session, on a temporary database; the new write routes are in `WRITE_ROUTES` and the existing admin-check iteration test covers them without being edited except to read the longer list. _Met, with two edits to `test/plan-routes.test.js` beyond reading the list: "each write route calls only its function" asserted `length === 6` and iterated every write route, so it now asserts 9 and filters to the six operation routes; and `WRITE_BODIES` gained three bodies (`createPlan`, `assign`, `restore`) so the 403 loop could send them. Behavior for the six routes is unchanged._
- [x] ac-2 (REQ-009): for every row of initiative §12 with both an MCP tool and a route, the parity test's spy sees the same function called through both, with equivalent arguments; the test fails when a row's tool or route is removed, and fails when a handler edits plan JSON itself.
- [x] ac-3 (REQ-003): a body key `actor`, `by`, or `kind` on any new route is 400; the revision row written through a route says `human` with `example-admin`, and the audit row actor is `admin:example-admin`.
- [x] ac-4 (REQ-005): `GET .../diff?from=2&to=working`, `GET .../revisions?sinceRevision=2`, `GET .../revisions/2`, and `POST .../restore` return the shapes SPEC-004 and SPEC-005 define; a stale `expectedRevision` on restore is 409 `revision_conflict`; a restore of `audience` is 400 `assign_required`; an admin caller's revision list carries the account name `example-admin` for a human row.
- [x] ac-5 (REQ-010): `GET /api/plan-schema` returns the same schema text the validator loads (byte-equal) to an admin and 401/403 to others, with a JSON content type and no cookie set. _Also an `ETag` and a 304 on `If-None-Match`; the response is `Cache-Control: private, no-cache`._
- [x] ac-6 (REQ-006): `GET /api/plans/:id/parts` returns `editState` with an admin context (entries carry `by`); a write route's result carries `overwrote` when a person's field is changed by an `ai`/`system` context only (a route write is `human`, so the key is absent or empty there).
- [x] ac-7 (REQ-017): `GET /api/plans/:id/published` answers 404 for a draft plan, a hidden lesson, and an unassigned personal plan, exactly as before; existing `test/plan-routes.test.js` assertions are unchanged except for the added context argument.
- [x] ac-8 (REQ-017): `POST /api/plans` with an existing id changes nothing; an oversized body is refused by the existing body limit; no route echoes request text in an error.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Preview of unpublished work | No new route. The editor uses the existing admin read and the shared render components (initiative §16 Q4). If the operator wants a real-player preview, that is a new route and a follow-up. | ambiguous | no — planning default applied |
| aud-2 | Schema route returns the repository schema file | It serves the same file `validate.js` loads, not a copy. | resolved | yes — one source |
| aud-3 | Route shapes for list endpoints | Query strings only, unknown keys 400, as `GET .../parts` does today. | resolved | yes — existing pattern |

> **Readiness** = 100 against the stated defaults.

---

## Deliverables

- [x] Changes to `server/api/plan-routes.js` (routes, `WRITE_ROUTES`, contexts) and a `schemaText` export in `server/plans/validate.js`
- [x] `test/helpers/plan-parity-table.js` _(also holds the pure `findParityProblems` checker and the human-only list)_
- [x] Tests: `test/plan-builder-routes.test.js`, `test/plan-builder-parity.test.js`; updated `test/plan-routes.test.js` for the context argument _(the context argument was SPEC-002's; this spec's two edits to that file are named under ac-1)_

---

## Technical Requirements (domain-specific)

- [x] Same body helpers as the existing routes (`readOperation`, `allowedKeys`, `sendMapped`). No parsing of an id path, no revision logic, no JSON edit in a handler. _`readBodyKeys` was split out of `readOperation` for the create route, which has no `expectedRevision`; `readOperation` behaves as before. Query values are parsed to numbers and checked against a fixed list; an id path is never parsed._
- [x] No new dependency. In-process server and temporary database in tests; no network.
- [x] Fixtures use `example-admin` and `example-learner`. _The harness's own two sessions are named `admin` and `learner` and are used for the 403 checks; the assignee in the routes and MCP tests is `example-learner`._

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** the two new test files, `test/plan-routes.test.js`, and `npm test`.
- **Rollback plan:** remove the new route mounts and the schema export.

---

## Security

- **Sensitivity:** required. Triggers matched: admin routes that create content, restore content, and change assignment; a route that exposes the plan schema; revision reads that return admin account names.
- **Security acceptance criteria:**
  - [x] Session and admin checks on every new route; a learner and a signed-out caller get 403 and 401.
  - [x] CSRF and Origin checks cover the new write routes (the gate in `server/web.js`, asserted for each).
  - [x] The learner route still answers 404 for unpublished content.
- **Review depth:** thorough · **focus areas:** authorization matrix over the new routes, the actor key rejection, the draft 404.

---

## Verification Strategy

- **Claim:** every user action on a plan has an admin route and an MCP tool that call one function, admin-only, with the existing gates and the learner route unchanged.
- **Check + executor:** `mechanized` — `node --test test/plan-builder-routes.test.js test/plan-builder-parity.test.js test/plan-routes.test.js`, then `npm test`.
- **Pass condition:** exit 0; ac-1 through ac-8 each asserted; the parity test fails when a table row loses a side (checked by a mutated copy of the table inside the test, so it cannot pass by being blind).
- **Contract test:** the parity test is the contract test at this seam (adapters against the operations layer).
- **Test levels:** integration (in-process HTTP and MCP servers, temporary database).

---

## Integration Points

- SPEC-011 reads `GET /api/plan-schema`. SPEC-013 uses list, create, and assign. SPEC-014 uses revisions, diff, and restore. SPEC-015 uses issues. SPEC-016 drives both channels.
- Produces **Change:** the HTTP side of the capability table and its parity test.
- **Provenance tag:** `INIT-008/SPEC-010`
