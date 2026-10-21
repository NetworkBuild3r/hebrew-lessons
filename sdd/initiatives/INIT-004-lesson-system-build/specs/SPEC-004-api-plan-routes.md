# SPEC-004 · API: Plan routes for admin and learner

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-004
title: "Plan HTTP routes for admin and learner"
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
blocked_by: [SPEC-003]
blocks: [SPEC-014, SPEC-015]
```

---

## Description

**Context:** ADR-0005 §5: the builder and the learner talk HTTP. Handlers authenticate, call `server/plans/ops.js` or `publish.js`, and map the result. They do not edit JSON.

**Scope (in):** the parity rows and the learner read. The MCP tool names are the same operations SPEC-005 registers. This spec implements the HTTP column only.

| Operation | Function | MCP tool | HTTP |
|-----------|----------|----------|------|
| Read a part | `readPart` | `read_part` | `GET /api/plans/:id/parts` |
| Replace a part | `replacePart` | `replace_part` | `PUT /api/plans/:id/parts` |
| Insert a block | `insertBlock` | `insert_block` | `POST /api/plans/:id/blocks` |
| Move a block | `moveBlock` | `move_block` | `POST /api/plans/:id/blocks/move` |
| Remove a block | `removeBlock` | `remove_block` | `DELETE /api/plans/:id/parts` |
| Set a field | `setField` | `set_field` | `PATCH /api/plans/:id/fields` |
| Publish | `publishPlan` | `publish_plan` | `POST /api/plans/:id/publish` |
| Learner read | store read | none (HTTP only) | `GET /api/plans/:id/published` |

- Writes require an admin session, the existing CSRF and Origin checks, and `expectedRevision`.
- A conflict is HTTP 409 `{ "error": "revision_conflict", "revision": <stored> }`.
- `GET /api/plans/:id/published` requires a session. Draft, hidden, `selected` without a grant, or a personal plan whose `assigneeId` is not the caller: **404**. The body is the snapshot only.
- Mount on the existing router in `server/api/`. Do not add a second server.

**Scope (out):** MCP registration (SPEC-005); media routes (SPEC-011); builder HTML (SPEC-015).

---

## Understood (agent's read of the intent)

- [x] und-1: 404 is the response when the caller must not learn that the id exists. 403 is not used for that case.
- [x] und-2: A learner never receives the working copy.

## Acceptance Criteria

- [x] ac-1 (REQ-004): each write route calls the matching function and no other edit implementation. A test spies on the export.
- [x] ac-2 (REQ-002): a stale revision is HTTP 409 with `revision_conflict` and the stored revision, and the stored document is unchanged.
- [x] ac-3 (REQ-008): a signed-in user who is not `assigneeId` receives 404 for a personal plan's published snapshot and for a `selected` lesson they are not granted. The assignee receives the snapshot. A signed-out caller receives 401.
- [x] ac-4 (REQ-014): a non-admin session receives 403 on every write route. The existing worksheet routes still behave as before (`npm test`).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Who may write | Admin session only, same as today's lesson editor. The MCP is the other writer, in SPEC-005. | resolved | yes — ADR-0002 admin authoring; no instructor role |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/api/plan-routes.js` mounted from the existing router
- [x] Tests: `test/plan-routes.test.js`

---

## Technical Requirements (domain-specific)

- [x] Separate checks for session, role, and body. Unknown JSON fields on the operation body are rejected.
- [x] Reuse `server/http/security.js` and the session helper the other `/api/` routes use. Do not invent a second cookie parser.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-routes.test.js` and `npm test`, including `web-gating.test.js` and `http-security.test.js` as part of that suite.
- **Rollback plan:** unmount the router. No snapshot has to be served for rollback of this spec alone.

---

## Security

- **Sensitivity:** required. Triggers matched: authorization on personal-plan reads, admin writes, untrusted plan JSON.
- **Security acceptance criteria:**
  - [x] Every write checks the admin role on the server.
  - [x] Personal, hidden, draft, and ungranted reads are 404 with an empty body that does not echo the plan.
  - [x] CSRF and Origin checks match the existing mutating `/api/` routes.
- **Review depth:** thorough · **focus areas:** authz, personal data, mass assignment.

---

## Verification Strategy

- **Claim:** the seven edit-and-publish routes are thin adapters, and a learner receives a snapshot only when ADR-0003 §7 allows it.
- **Check + executor:** `mechanized` — `node --test test/plan-routes.test.js`.
- **Pass condition:** exit 0; the spy assertions show each route's function; the 404 cases do not contain `newWords` or `assigneeId` in the body.
- **Test levels:** HTTP integration on the existing test server pattern in `test/api-auth.test.js`. No property-based test. No coverage tool is configured; the bar is this file plus `npm test`.
- **Test data:** temporary database, two users, one personal plan and one `selected` lesson.

---

## Integration Points

- SPEC-014 and SPEC-015 call these routes. SPEC-005 must call the same functions; a shared spy test may live in either file, and this spec's spy covers HTTP.
- Produces **Change:** plan HTTP API.
- **Provenance tag:** `INIT-004/SPEC-004`
