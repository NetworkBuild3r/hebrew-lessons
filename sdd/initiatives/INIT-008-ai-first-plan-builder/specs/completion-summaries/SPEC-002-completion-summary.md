# SPEC-002 completion summary

**Spec:** SPEC-002 · Service: Operations hardening, actor context, and audit rows  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (code and tests done; `security_review: required` is a `complete-initiative` phase and no independent reviewer has run)

## What shipped

- **Defect 1, closed.** `replace_part` on any part refuses a value whose `id` differs from the part's id: `{ error: 'id_mismatch', revision, path }`, nothing written, no audit row. The same check covers word, paragraph, line, and question paths. **One addition beyond the spec:** `set_field` with `field: 'id'` on a part is refused the same way (setting it to the current id is a no-op and is allowed). It was the same defect through a second door.
- **Defect 2, closed.** `set_field` of `audience` or `assigneeId` on the plan, and `replace_part` of the plan with a changed `audience` or `assigneeId`, return `{ error: 'assign_required', revision }` and write nothing. A whole-plan replace with those two unchanged is accepted (a missing `assigneeId` and `null` count as equal).
- **`ops.assignPlan(store, id, { audience, assigneeId, expectedRevision }, ctx)`** is the only writer of those two fields. It checks shape only (`shared` needs no assignee, `personal` needs a non-empty string) and is validated and revision-checked like any write. `personal.assign` does its active-account check (`invalid_audience`, `assignee_refused`, `shared` stores null) and then calls `ops.assignPlan` in place of its old whole-plan `replacePart`. A test scans `server/` and `mcp/` and finds the name `assignPlan` only in `ops.js` and `personal.js`, so no adapter reaches it.
- **`server/plans/context.js`.** `makeContext({ channel, tool, username, name, requestId, client })`, `requireContext`, `isContext`, `actorString`. Channel `mcp` is kind `ai` named `mcp`; `admin` is `human` named by the session username; `system` is `system` named by the script. The returned context is frozen and carries a `Symbol.for` brand that JSON cannot hold, so a plain object, a JSON copy, or a string fails `requireContext`. `actorString` returns `admin:<username>`, `mcp`, or `system:<name>`.
- **`server/plans/audit.js`.** `record(ctx, action, planId, { revision, path, field })` is the only plan audit writer. `detail` is JSON with `revision`, `path`, `field`, `tool`, `requestId` only. It resolves the audit repo at call time so a reloaded database module and its repo stay paired.
- **`ops.js`.** Every mutating function takes a trailing `ctx`: `replacePart`, `insertBlock`, `moveBlock`, `removeBlock`, `setField`, `assignPlan`. A missing or forged context throws `TypeError` before anything is read or written. `persist()` now saves the working copy and appends the `plan.*` row inside one `db.transaction`; if the audit insert throws, the write rolls back. `readPart` takes an optional `ctx` (validated if given).
- **`publish.js`.** `publishPlan(store, id, input, ctx)` appends `plan.publish` inside the existing transaction.
- **Adapters.** `server/api/plan-routes.js` builds an `admin` context from the session username for every write route (`tool` is the operation name). `mcp/tools.js` builds an `mcp` context with the tool name and the JSON-RPC request id from the handler's second argument. No new route or tool. HTTP bodies still go through `allowedKeys`, so `actor`, `by`, and `kind` are unknown keys and get 400. The MCP input schemas have no actor field, and `planInput` forwards only named keys.
- **Callers updated.** `scripts/seed-starter-content.js` (system context `seed-starter-content`, built lazily). `scripts/import-lesson-plans.js` calls the store directly and is unchanged. I searched `server/`, `mcp/`, `scripts/`, `shared/`; the only remaining callers are the two adapters, the seed script, and tests.
- **`client` label (aud-2): checked, not recorded.** The installed SDK does not pass `clientInfo` to a handler, and this server is stateless (a new `McpServer` per POST, no `initialize`), so there is no `client`. The actor is `mcp` either way.
- **Audit rows written by this spec:** `plan.set_field`, `plan.replace_part`, `plan.insert_block`, `plan.move_block`, `plan.remove_block`, `plan.assign`, `plan.publish`. `plan.create` and `plan.restore` are in the allowed list and are written by SPEC-007 and SPEC-005.

## Tests

New:

- `test/plan-ops-hardening.test.js` (10 tests, temp DB): id mismatch (block, word, `set_field` id), `assign_required` on both operations, unchanged whole-plan replace, `assignPlan` shape and revision checks, `personal.assign` through `assignPlan`, `TypeError` for every mutating function with six kinds of bad context (undefined, null, `{}`, a forged literal, a JSON copy, a string), channel-to-actor mapping, and the "only `ops.js` and `personal.js` mention `assignPlan`" scan.
- `test/plan-audit.test.js` (8 tests, signed-in server harness): `admin:example-admin` row for a set_field with no value in any column; the same through the MCP handler gives actor `mcp` and a `requestId`; every write route and tool writes its own action; refused writes (409, validation, `id_mismatch`, `assign_required`, stale publish) leave `audit_log` and the plan untouched; a stub that makes the audit insert throw rolls back `setField`, `moveBlock`, and `publishPlan` (working copy, revision, status, snapshot unchanged); body keys `actor`, `by`, `kind` get 400; no plan tool schema has an actor-like field and an `actor` argument is ignored; a whole-plan replace records only the path.
- `test/helpers/plan-ctx.js`: `testContext`, `withCtx`, `publishWithCtx`, so existing direct calls change by the added argument only.

Counts: the two new files plus `test/plan-revisions-migration.test.js` are 28 tests, 28 pass. **Full `npm test` with `SRS_SCHEDULER=ladder`: 1155 tests, 1155 pass, 0 fail** (the baseline you gave was 1066; the difference is the 28 tests here and the tests other agents added). An earlier run in the same session had failures in `test/plan-language-1-1.test.js` and `test/plan-validate.test.js`; they came from SPEC-008's work in progress (new `pointing-consistency` findings and a new `hebrew-checks` require) and were gone on the later runs. I did not touch `validate.js` or the issues module.

## Existing tests changed (ac-8)

None removed. Changed:

- `test/plan-ops.test.js`: `ops` and `publishPlan` wrapped with a context (the added argument only). **One test body changed beyond that:** "personal audience and assigneeId are ordinary fields…" asserted that `set_field` of `assigneeId` succeeds, which is the defect. It now asserts `assign_required`, the assignee unchanged, and the revision unchanged. Renamed accordingly.
- `test/plan-language-1-1.test.js`: `ops` and `publishPlan` wrapped; **one line added**: the `replacePart` test replaced block `listen` with a fixture whose id was different (exactly the defect), so it now sets `block.id = 'listen'`.
- `test/personal-plan.test.js`: `personal.assign` wrapped with a context. **Three spies that watched `ops.replacePart` now watch `ops.assignPlan`**, and assertions about `lastReplace.input.value` became assertions about `lastAssign.input`. Outcomes and the "no plan SQL in `personal.js`" checks are unchanged. This is why ac-3's "only the added ctx argument" is not literally true.
- `test/plan-routes.test.js` (four direct `publishPlan` calls) and `test/plan-mcp.test.js` (two direct calls) gained a `testContext()` argument.
- `test/db.test.js`: migration-version range 5 to 6 and a version-6 assertion (SPEC-003).

## Acceptance criteria

| Id | Result |
|----|--------|
| ac-1 | Pass (`plan-ops-hardening`). |
| ac-2 | Pass. |
| ac-3 | Pass, with the spy note above. |
| ac-4 | Pass. |
| ac-5 | Pass. The test admin is `example-admin`; the detail is exactly `{ revision, path, field, tool }`. |
| ac-6 | Pass, including the stubbed audit failure rolling back the working copy. |
| ac-7 | Pass. |
| ac-8 | Pass; the changes are named above. |

Security acceptance criteria: no input from a client sets the actor (tests above); `assignPlan` is the only writer of the two fields and is reached only from `personal.assign`, which does the account check (a test enforces the reach, but `assignPlan` itself does not look up accounts, by the spec's design); an audit row never holds plan text or a field value (asserted).

## Things to know

- **No migration in this spec.** The new `plan_revisions` writes belong to SPEC-004. This spec adds no schema.
- **HTTP error shape.** `id_mismatch` and `assign_required` arrive as HTTP 400 `{ error: 'Invalid request', code: '<name>' }` (the existing `sendMapped` shape), and as an MCP error whose body is `{ error: '<name>', ... }`.
- **Admin username is in the audit row** as `admin:<username>`, as ADR-0004 §10 already does for media. It is not returned to the MCP.
- **Audit transaction nesting.** `db.transaction` does not nest. `ops` writes are not called from inside another transaction anywhere in the repo today. A future spec that wraps a write must call these functions outside its own transaction, or extend `db.transaction`.
- **`publishPlan` and the working-copy store.** Both `ops.persist` and `publishPlan` now require the real database (the audit insert), so a fake store alone is no longer enough in a test.

## Files

New: `server/plans/context.js`, `server/plans/audit.js`, `test/helpers/plan-ctx.js`, `test/plan-ops-hardening.test.js`, `test/plan-audit.test.js`, this summary.  
Edited: `server/plans/ops.js`, `server/plans/personal.js`, `server/plans/publish.js`, `server/api/plan-routes.js`, `mcp/tools.js`, `scripts/seed-starter-content.js`, the tests listed above, `SPEC-002-service-ops-hardening-actor-audit.md` (status `in_review`, boxes ticked, two executor notes).  
Not touched: `server/plans/validate.js`, the issues module, `server/plans/id-path.js` (SPEC-008's), `worksheets/*/index.html`, the real database. No commit.

## Provenance

`INIT-008/SPEC-002`
