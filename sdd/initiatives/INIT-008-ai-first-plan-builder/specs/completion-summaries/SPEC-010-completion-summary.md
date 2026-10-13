# SPEC-010 completion summary

**Spec:** SPEC-010 · API: HTTP twins, the schema route, and the parity test  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code and tests done; `security_review: required` is a `complete-initiative` phase and no independent reviewer has run)  
**Provenance:** `INIT-008/SPEC-010`

## What shipped

Nine new routes in `server/api/plan-routes.js` (`registerBuilderRoutes`). Every one is `requireSession(requireAdmin(...))`: signed out is 401 `no_session`, a role `user` is 403 `forbidden`. Each calls the same function as its MCP tool, looked up on the module at call time, with an `admin` context built from the session username (the `tool` label is the MCP tool's name). Bodies reject unknown keys, so `actor`, `by`, and `kind` are 400 `bad_request`. Query strings accept only the listed keys, each once, with whole numbers parsed from digits only. The three new writes join `WRITE_ROUTES`, and the CSRF gate in `server/web.js` (Origin plus a JSON content type) covers them like every `/api/` write.

| Method and path | Calls | Request | 200 body |
|-----------------|-------|---------|----------|
| `GET /api/plans` | `create.listPlans` | query `limit?` | `{ plans: [{ id, title, audience, assigned, assigneeId, status, revision, updatedAt, published, unpublishedChanges, humanEditedFields }], truncated }` (`assigneeId` is present for an admin) |
| `POST /api/plans` | `create.createPlan` | body `{ id, title, recipe?, unitId?, lessonId? }` | `{ plan, revision: 1, status: 'draft', validation }` |
| `POST /api/plans/:id/assign` | `personal.assign` | body `{ audience: 'shared' or 'personal', assigneeId?, expectedRevision }` | `{ plan, revision, status, validation }` |
| `GET /api/plans/:id/revisions` | `revisions.list` | query `sinceRevision?`, `beforeRevision?`, `actorKind?`, `pathPrefix?`, `limit?` | `{ revisions: [{ revision, op, path, field, summary, actorKind, actorName, tool, client, at }], truncated }` (`actorName` is the account name for a person's row) |
| `GET /api/plans/:id/revisions/:revision` | `revisions.get` | no query | the metadata above plus `document` |
| `GET /api/plans/:id/diff` | `revisions.diff` | query `from`, `to` (digits, `working`, or `published`), `path?` | `{ from: { ref, revision }, to: { ref, revision }, entries, truncated }` |
| `POST /api/plans/:id/restore` | `revisions.restore` | body `{ fromRevision, path?, field?, expectedRevision }` | `{ plan, revision, status, validation, restoredFrom, overwrote? }` |
| `GET /api/plans/:id/issues` | `issues.list` | query `severity?` (`block`, `warn`, `inform`, `all`, or a comma list), `code?`, `pathPrefix?`, `limit?` | `{ revision, status, counts, issues: [{ code, severity, hard, path, field, pointer, message, fix? }], truncated }` |
| `GET /api/plan-schema` | `validate.schemaText` | none | the schema file text, byte for byte |

- **Errors** keep the existing mapping: 409 `{ error: 'revision_conflict', revision }`; 404 `{ error, code: 'not_found' }`; 400 `{ error: 'validation', revision, blocks, warnings, informs }`; any other service error is 400 `{ error: 'Invalid request', code }`. Codes the UI will meet: `already_exists`, `invalid_id`, `invalid_recipe`, `invalid_audience`, `assignee_refused`, `assign_required`, `id_mismatch`, `no_snapshot`, `revision_corrupt`, `invalid_op` (with `field` for a bad `severity` or `limit`), `pending_media`, `bad_request`. A 404 from a restore may carry `reason: 'parent_missing'` or `'absent_in_revision'`. No error echoes request text.
- **The existing routes** keep their paths, bodies, and status mapping. `GET .../parts` already returned `editState` (SPEC-006); a restore result now also forwards `restoredFrom`.
- **The schema route** returns the text of the file the validator loads, from a new `validate.schemaText()` read once per process. Headers: `application/json; charset=utf-8`, `Cache-Control: private, no-cache`, an `ETag` (and 304 on `If-None-Match`), the standard security headers, and no cookie.
- **Per-block hint data.** SPEC-011 needs only the schema from the server. SPEC-012's hints are a client file (`shared/admin/plan-hints.js`), and its test compares them with `FIELD_LANGUAGE`, which the test can import from `server/plans/hebrew-checks`. So no hint route was added.
- **No preview route.** The learner route `GET /api/plans/:id/published` is untouched and still 404 for a draft, a hidden lesson, and an unassigned personal plan.

## The parity test

`test/helpers/plan-parity-table.js` holds initiative §12 as data (17 rows: operation, module, function, tool, route, write flag, exemption), the human-only list (verify a form, the AI-planning switch, picture approval), and a pure checker `findParityProblems(rows, registry)`. `test/plan-builder-parity.test.js` (9 tests) asserts:

- The real table has no problem against the registered tools (stub `registerTool`), the registered routes (a router that holds the plan, admin, and media routes), and `WRITE_ROUTES`.
- For each of the 16 rows with both sides, a spy on the function sees exactly one call through the tool and one through the route, with deep-equal arguments (store and context aside), a context `{ ai, mcp, tool }` on one side and `{ human, example-admin, tool }` on the other. I broke the diff route's `path` forwarding by hand to confirm this fails, then restored it.
- The schema row has no tool and a written exemption; no tool name mentions the schema.
- The checker, run on mutated copies, names a removed tool, a removed route, a row with a tool and no route (and the reverse), a write route missing from `WRITE_ROUTES`, a missing function, and a row with neither side.
- Every mutating `/api/plans*` route registered is in `WRITE_ROUTES` and the reverse; the two learner reads are the only open plan routes.
- No handler (the eight new tools, the seven earlier plan tools, the nine new routes, the seven earlier routes) contains `JSON.parse`, `JSON.stringify`, a plan collection, an array edit, a store write, a database call, or an assignment into a plan, and none mentions `assignPlan`. The scan is itself run on a seeded bad snippet and finds each kind.
- The human-only acts: each route exists, `set_lexicon_verified` and `set_ai_planning` answer `forbidden`, and no tool name mentions approval.
- The learner 404 for a draft, a hidden lesson, and an unassigned personal plan.

## Tests

- `test/plan-builder-routes.test.js`: 15 tests, 15 pass. The 401, 403, and 200 matrix over all nine routes, with the tables unchanged on a refusal; CSRF (no Origin, a foreign Origin, no JSON type) on each write; `actor`, `by`, `kind` as body keys on each write and `actor` as a query key; a route write is `human` with `example-admin` and audit actor `admin:example-admin`; create, assign, list, revisions, diff, restore, issues, and schema shapes and refusals; `overwrote` only for an ai or script write; the oversized body (413 or an aborted socket, as the other body-limit tests allow); no echo of a marker string in any refusal.
- `test/plan-builder-parity.test.js`: 9 tests, 9 pass.
- Full `npm test` with `SRS_SCHEDULER=ladder`: **1278 tests, 1278 pass, 0 fail** (1242 before; 36 new across SPEC-009 and SPEC-010). The real database was not opened or migrated.

## Existing tests changed

`test/plan-routes.test.js` only, two edits, both because the write-route list grew from 6 to 9 (the spec's "read the longer list"): the "each write route calls only its function" test now asserts 9 routes and filters to the six operation routes; and `WRITE_BODIES` gained `createPlan`, `assign`, and `restore` bodies so the 403 loop can send them. No assertion about an existing route changed.

## Deviations

- **Extra optional inputs**, mirrored on both channels: `limit` on `GET /api/plans`, `beforeRevision` on the revisions list, and `unitId` and `lessonId` on create (see SPEC-009).
- **`diff` requires both `from` and `to`**; there is no default. A missing or malformed one is 400 `bad_request`.
- **`severity` on the issues route is a comma list**; the MCP tool takes a list. Both reach `issues.list` as the same array.
- **The two `field` and `reason` keys** on a 400 or 404 body are new (SPEC-010 und-3 said no new mapping was needed; the codes needed none, the extra keys are additive).
- **`sendMapped` was split** into `sendFailure` (the shared status mapping), `sendMapped` (writes), and `sendRead` (reads that return their result as the body). Existing bodies are byte-identical.

## Not closed

1. **Independent security review** (`security_review: required`) has not run. The authorization matrix, the CSRF checks, and the actor-key rejection are tested; a reviewer has not read the routes.
2. **A revision list shows the admin's account name to any admin.** That is the design (A2), and the MCP never sees it, but there is only one admin role.
3. **The parity table's argument examples live in the test file**, not the helper, so SPEC-016 will need its own or to copy them.
4. **The schema route has no learner-facing twin and no tool.** If the operator later wants an AI to read the schema through the MCP, it is one more row.

## Files

New: `test/helpers/plan-parity-table.js`, `test/plan-builder-routes.test.js`, `test/plan-builder-parity.test.js`, this summary.  
Edited: `server/api/plan-routes.js`, `server/plans/validate.js` (the `schemaText` export), `test/plan-routes.test.js` (the two edits above), the SPEC-010 file (status `in_review`, boxes ticked, notes).  
No commit. No change under `shared/`.

## Provenance

`INIT-008/SPEC-010`
