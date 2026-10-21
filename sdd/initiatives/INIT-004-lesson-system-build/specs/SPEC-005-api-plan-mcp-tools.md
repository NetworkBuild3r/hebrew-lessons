# SPEC-005 · API: Plan MCP tools

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-004
title: "Plan MCP tools for the parity table"
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-003]
blocks: [SPEC-012, SPEC-018]
```

---

## Description

**Context:** the first slice ends here for the AI. The operator's agent edits plans through MCP before the builder exists. Tools register in `mcp/tools.js` and call the same functions as SPEC-004.

**Scope (in):** the same parity rows as SPEC-004. This spec registers the MCP column. SPEC-004 implements the HTTP column.

| Operation | Function | MCP tool | HTTP (SPEC-004) |
|-----------|----------|----------|-----------------|
| Read a part | `readPart` | `read_part` | `GET /api/plans/:id/parts` |
| Replace a part | `replacePart` | `replace_part` | `PUT /api/plans/:id/parts` |
| Insert a block | `insertBlock` | `insert_block` | `POST /api/plans/:id/blocks` |
| Move a block | `moveBlock` | `move_block` | `POST /api/plans/:id/blocks/move` |
| Remove a block | `removeBlock` | `remove_block` | `DELETE /api/plans/:id/parts` |
| Set a field | `setField` | `set_field` | `PATCH /api/plans/:id/fields` |
| Publish | `publishPlan` | `publish_plan` | `POST /api/plans/:id/publish` |

- A missing or wrong `MCP_AUTH_TOKEN` refuses the call and returns no plan body.
- Conflict: the tool error name is `revision_conflict` and includes the stored revision. The document is unchanged.
- The eight tools at `origin/main:mcp/tools.js:148` through `:342` stay registered and keep their current behavior.
- `upsert_lesson` is unchanged and does not accept a plan document.

**Scope (out):** `upload_media`, `plan_image`, `generate_image` (SPEC-012); the learner-profile tool (INIT-006 SPEC-010). Creating a plan id (open; not a tool in this spec).

---

## Understood (agent's read of the intent)

- [x] und-1: Tool names match the operation names in the table. No aliases.
- [x] und-2: This spec does not read `review_items`, `practice_events`, or `lesson_progress`.

## Acceptance Criteria

- [x] ac-1 (REQ-004): each new tool calls the matching function. A test spies on `server/plans/ops.js` and `publish.js` and compares the stored document with the same call made through the function directly. The bytes match.
- [x] ac-2 (REQ-002): a stale `expectedRevision` returns `revision_conflict` and does not change the stored plan.
- [x] ac-3 (REQ-004): with the token unset, `read_part` returns no plan JSON. The eight legacy tools still require the token, as `test/mcp-auth.test.js` already checks.
- [x] ac-4 (REQ-013): `list_lessons`, `get_lesson`, `upsert_lesson`, `set_lesson_visibility`, and `generate_lesson_audio` still register. A test lists tool names.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | New plan ids | Not this spec. The AI edits plans SPEC-007 has inserted. | resolved | yes — ADR-0003 parity table has no create row |

> **Readiness** = 100.

---

## Deliverables

- [x] Plan tools in `mcp/tools.js`
- [x] Tests: `test/plan-mcp.test.js`

---

## Technical Requirements (domain-specific)

- [x] Handlers call the ops functions with the arguments the HTTP body uses, so the two adapters cannot drift in field names (`path`, `expectedRevision`, `index`, `field`, `value`).
- [x] Do not log the token or the plan body.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-mcp.test.js test/mcp-auth.test.js` and `npm test`.
- **Rollback plan:** unregister the seven tools. Legacy tools remain.

---

## Security

- **Sensitivity:** standard. Triggers matched: token gate on new write tools. The snapshot tool is SPEC-012 and is `required` there.
- **Security acceptance criteria:**
  - [x] No tool in this spec returns learner progress fields.
  - [x] A bad token yields no working-copy JSON.
- **Review depth:** standard · **focus areas:** authn, tool parity.

---

## Verification Strategy

- **Claim:** the MCP plan tools are the same functions as the HTTP routes, and the legacy tool list is intact.
- **Check + executor:** `mechanized` — `node --test test/plan-mcp.test.js`.
- **Pass condition:** exit 0; the spy list contains each function once per tool call; conflict leaves the JSON unchanged; tool-name list includes both the seven new names and the eight old names.
- **Test levels:** integration through the tool handler with a temporary database, same pattern as `test/mcp-assignment.test.js`. No network. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- This is the first-slice exit for AI authoring, together with SPEC-007's documents.
- SPEC-012 adds tools in the same file and must not drop these registrations.
- Produces **Change:** seven MCP tools.
- **Provenance tag:** `INIT-004/SPEC-005`
