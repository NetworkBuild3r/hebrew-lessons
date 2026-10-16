# SPEC-002 · Service: Operations hardening, actor context, and audit rows

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-008
title: "Close the id-mismatch and assignee defects; put an actor context on every plan write; audit every plan mutation"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 6 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004, SPEC-009]
```

---

## Description

**Context:** `RSCH-009` §3 found two defects by reading and probing `server/plans/ops.js`. (1) `replace_part` writes `value` over the node at a path without comparing `value.id` to the node's id (`server/plans/ops.js:209`–`:224`); `mutate` checks only the plan-level id (`:394`). A part moved to a different id breaks every id path that pointed at it. (2) `set_field` on the plan blocks only `id`, `revision`, and `status` (`:187`), and `replace_part` with an empty path takes `audience` and `assigneeId` from the value (`:220`–`:223`), so a write can hand a personal plan to any account name and skip the active-account check in `server/plans/personal.js:49`–`:80`. Nothing records who made a plan edit: `rg audit server/plans` finds no use, and `publishPlan` writes no row. ADR-0011 (SPEC-001) is the contract.

**Scope (in):**

- **Defect 1.** In `apply()`, `replace_part` on any node that is not the plan refuses when `value.id !== found.node.id`, returning `{ error: 'id_mismatch', path, revision }`. The plan-level check in `mutate` stays. `insert_block` is unchanged: its new block's id is its own.
- **Defect 2.** `set_field` on the plan with `audience` or `assigneeId`, and `replace_part` on the plan whose value changes either, return `{ error: 'assign_required', revision }` and write nothing. A `replace_part` of the plan whose `audience` and `assigneeId` equal the stored values is accepted. A new exported `ops.assignPlan(store, id, { audience, assigneeId, expectedRevision }, ctx)` is the only function that writes those two fields; `personal.assign` calls it after its account check (and replaces its current `ops.replacePart` call). `assignPlan` is not an adapter-callable generic edit: adapters reach it only through `personal.assign`. Behavior of `personal.assign` for its existing inputs is unchanged (`invalid_audience`, `assignee_refused`, shared stores `assigneeId` null).
- **Context.** New `server/plans/context.js`: `makeContext({ channel: 'mcp'|'admin'|'system', tool, username, name, requestId, client })` returns `{ actor: { kind, name, tool?, client? }, requestId? }`; `requireContext(ctx)` throws `TypeError` when a write has no valid context. Mapping: `mcp` → kind `ai`, name `mcp`; `admin` → kind `human`, name = the session username; `system` → kind `system`, name = the script or subsystem name. `actorString(ctx)` returns `admin:<username>`, `mcp`, or `system:<name>` for the audit row. A context is **built by the adapter from the channel**; no request field, MCP argument, or HTTP body key can set the actor. Every mutating function gains a trailing `ctx`: `replacePart`, `insertBlock`, `moveBlock`, `removeBlock`, `setField`, `assignPlan`, `publishPlan`, `personal.assign`. `readPart` accepts an optional `ctx` (used later for audience). A write with no context fails with the `TypeError`, so a forgotten adapter cannot write silently.
- **Audit.** `persist()` in `ops.js` and the transaction in `publish.js` write one `audit_log` row per mutation through `server/db/repos/audit.js` `append`, **inside the same `db.transaction` as the working-copy write** (`persist` becomes transactional; `publishPlan` already is). Actions: `plan.set_field`, `plan.replace_part`, `plan.insert_block`, `plan.move_block`, `plan.remove_block`, `plan.assign`, `plan.publish`. `target` = the plan id. `detail` = compact JSON `{ revision, path, field, tool, requestId }` — never a field value, never document text. A refused write (conflict, validation, id mismatch, assign required) writes **no** audit row and no working-copy change.
- **Adapters.** `server/api/plan-routes.js` builds an `admin` context from `ctx.user.username` for every write route; `mcp/tools.js` builds an `mcp` context with the tool name for each plan tool. Both pass it to the functions. No new route or tool in this spec; the two adapters change by one argument each.
- **Callers.** Search for every caller of the changed functions (`rg "ops\.|replacePart|setField|publishPlan|personal\.assign" --glob '!sdd/**'`) and update each. `scripts/import-lesson-plans.js` and `scripts/seed-starter-content.js` call the store directly and are not changed; their writes are not audited and are not in the revision log (initiative §16 A5).

**Scope (out):** the revision log (SPEC-004); provenance marks (SPEC-006); new tools and routes (SPEC-009, SPEC-010); the `audience` and `assigneeId` fields' appearance in the editor (SPEC-013).

---

## Understood (agent's read of the intent)

- [x] und-1: Error names `id_mismatch` and `assign_required` are new and stable. Adapters already map an unknown `error` string to HTTP 400 with that code (`sendMapped`) and to an MCP error; neither adapter needs a new mapping.
- [x] und-2: The actor is a fact about the channel, not a claim by the caller. A test sends `actor` in a body and as a tool argument and sees it rejected or ignored.
- [x] und-3: Audit rows hold no plan text. The path and field name are enough to find the revision (SPEC-004).
- [x] und-4: Existing tests that call ops functions without a context are updated in this change, not left to fail.

## Acceptance Criteria

- [x] ac-1 (REQ-002): `replace_part` at `units/u/lessons/l/blocks/b` with a value whose `id` is not `b` returns `id_mismatch`, leaves the document and revision unchanged, and writes no audit row. The same call with `id: 'b'` succeeds. Same check at a word path (`.../words/<id>`).
- [x] ac-2 (REQ-002): `set_field` of `audience` or `assigneeId` at the empty path returns `assign_required`; `replace_part` of the plan with a changed `audience` or `assigneeId` returns `assign_required`; `replace_part` of the plan with those two unchanged succeeds. Nothing is written in the refused cases.
- [x] ac-3 (REQ-002): `personal.assign` with a disabled or unknown account returns `assignee_refused` and writes nothing; with an active account it stores `audience: 'personal'` and `assigneeId` equal to that account's username through `ops.assignPlan`; with `audience: 'shared'` it stores `assigneeId` null. Its existing tests (`test/personal-plan.test.js`) pass with only the added `ctx` argument. *Executor note: not exactly. Every test and assertion kept its outcome, but the three spies that watched `ops.replacePart` now watch `ops.assignPlan`, because that is the function `personal.assign` calls. See the completion summary.*
- [x] ac-4 (REQ-003): every mutating function throws `TypeError` when called without a valid context, and writes nothing.
- [x] ac-5 (REQ-003): a successful `setField` through the HTTP adapter writes one `audit_log` row with actor `admin:example-admin`, action `plan.set_field`, target the plan id, and `detail` JSON holding `revision`, `path`, `field`, `tool` or `requestId`, and **no** key holding the new value. The same call through the MCP adapter writes actor `mcp`. `publishPlan` writes `plan.publish`.
- [x] ac-6 (REQ-003): a revision conflict, a validation refusal, `id_mismatch`, and `assign_required` each leave `audit_log` unchanged. A forced failure of the audit insert (a stub that throws) rolls the working-copy write back, so no mutation exists without its audit row.
- [x] ac-7 (REQ-003): a request body key `actor`, `by`, or `kind` is rejected by the HTTP routes (unknown key, 400, as the existing `allowedKeys` does), and no MCP plan tool's input schema has a field that can set the actor.
- [x] ac-8 (REQ-017): `npm test` shows no existing plan, personal-plan, MCP, route, or import test removed; each changed only by the new `ctx` argument or an updated error expectation named in the completion summary. *Executor note: the changes are named in the completion summary (two test bodies changed beyond the added argument, plus one migration-count line in `test/db.test.js` from SPEC-003).*

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Error names | `id_mismatch` and `assign_required`. | resolved | yes — planning choice, stable names |
| aud-2 | MCP `client` label | Recorded only if the installed MCP SDK passes `clientInfo` to the handler. `RSCH-009` did not check. The executor checks and records the result; if unavailable, `client` is omitted. The actor is `mcp` either way. | resolved | yes — best-effort label, not an identity |
| aud-3 | Scripts and the audit | The two import scripts are not audited and not logged. | resolved | yes — initiative §16 A5; baseline row covers the first edit |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/plans/context.js`
- [x] Changes to `server/plans/ops.js`, `server/plans/publish.js`, `server/plans/personal.js`, `server/api/plan-routes.js`, `mcp/tools.js`
- [x] Tests: `test/plan-ops-hardening.test.js`, `test/plan-audit.test.js`; updated `test/plan-ops.test.js`, `test/plan-routes.test.js`, `test/plan-mcp.test.js`, `test/personal-plan.test.js` as needed

---

## Technical Requirements (domain-specific)

- [x] Bound parameters only. `audit.append` is the only audit writer.
- [x] No new dependency. `node:sqlite` only. Temporary database in tests; no test touches `data/hebrew.sqlite`.
- [x] Fixtures use `example-admin` and `example-learner`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none (no schema change).
- **Blocking checks:** the two new test files, the four updated ones, and `npm test`.
- **Rollback plan:** revert the commit. No schema or data changed.

---

## Security

- **Sensitivity:** required. Triggers matched: an authorization bypass fix (`audience` and `assigneeId`); identity of the writer derived for audit; admin routes.
- **Security acceptance criteria:**
  - [x] No input from a client sets the actor.
  - [x] The only writer of `audience` and `assigneeId` is `assignPlan` behind the account check.
  - [x] An audit row never contains plan text or a field value.
- **Review depth:** thorough · **focus areas:** actor derivation, the `assign_required` paths, audit atomicity.

---

## Verification Strategy

- **Claim:** the two defects are closed, every plan write is attributed to a channel-derived actor, and no mutation exists without exactly one audit row.
- **Check + executor:** `mechanized` — `node --test test/plan-ops-hardening.test.js test/plan-audit.test.js`, then `npm test`.
- **Pass condition:** exit 0; ac-1 through ac-8 each asserted; the rollback assertion in ac-6 observes the working copy unchanged after the stubbed audit failure.
- **Test levels:** unit for `apply()`; integration on a temporary database for audit atomicity, the HTTP adapter, and the in-process MCP adapter.

---

## Integration Points

- SPEC-004 appends to the same transaction. SPEC-009 and SPEC-010 build contexts the same way. SPEC-013's assign control calls the route SPEC-010 adds over `assignPlan`.
- Produces **Change:** hardened operations; `ctx` on every write; `plan.*` audit rows.
- **Provenance tag:** `INIT-008/SPEC-002`
