# SPEC-012 · API: Media and image-generation MCP tools

## Metadata

```yaml
spec_id: SPEC-012
initiative_id: INIT-004
title: "MCP tools for media and image generation"
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-005, SPEC-008, SPEC-009]
blocks: [SPEC-018]
```

---

## Amended 2026-10-09 by INIT-006/SPEC-001

`get_learner_snapshot` is dropped. It is superseded by INIT-006's `get_learner_profile` (ADR-0009 §1, Accepted 2026-10-09), which INIT-006 SPEC-010 registers. Effort went from 5h to 4h. The dependency on SPEC-010 is gone.

---

## Description

**Context:** ADR-0004 §4 gives the MCP column for media. These tools call the same services as SPEC-011. They do not add a second ingest or a second image client. The plan tools from SPEC-005 stay registered.

**Scope (in):**

| Tool | Behavior |
|------|----------|
| `upload_media` | Token required. Calls ingest. |
| `plan_image` | Token required. Dry-run only. |
| `generate_image` | Token required. `confirm: true`, caps, pinned model. |

- Approval is not a successful MCP tool. A test calls the HTTP approval route with the MCP token and without an admin session and expects 403. No `approve_media` tool is registered.
- Missing token: refusal and no image body.
- The eight legacy tools and the seven plan tools remain.

**Scope (out):** rewriting `upsert_lesson`; the skill text (SPEC-018).

---

## Understood (agent's read of the intent)

- [x] und-1: The MCP actor cannot approve pixels or alt text.

## Acceptance Criteria

- [x] ac-1 (REQ-004): tool names `upload_media`, `plan_image`, and `generate_image` are registered, and the SPEC-005 plan tools are still registered.
- [x] ac-2 (REQ-007): `plan_image` does not call the transport. `generate_image` without confirm does not call the transport. Both require the token.
- [x] ac-3 (REQ-006): no registered tool name approves media. HTTP approval with only the MCP token returns 403.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Fetch tool | No MCP fetch tool. Bytes are fetched by the signed-in HTTP route while a lesson renders. | resolved | yes — ADR-0004 matrix has no MCP fetch cell that returns bytes to the model beyond upload |

> **Readiness** = 100.

---

## Deliverables

- [x] Tools added in `mcp/tools.js`
- [x] Tests: `test/media-generation-mcp.test.js`

---

## Technical Requirements (domain-specific)

- [x] Same token check as `origin/main:mcp/tools.js` uses for `list_lessons`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/media-generation-mcp.test.js test/plan-mcp.test.js` and `npm test`.
- **Rollback plan:** unregister the three tools. Plan tools from SPEC-005 stay.

---

## Security

- **Sensitivity:** required. Triggers matched: upload, generation key path.
- **Security acceptance criteria:**
  - [x] A bad or missing token returns no image body.
  - [x] The tool list has no approve tool.
  - [x] Errors do not contain the API key.
- **Review depth:** thorough · **focus areas:** authn, secrets in responses, capability parity with the matrix.

---

## Verification Strategy

- **Claim:** the MCP media tools match ADR-0004 §4 and expose no approval.
- **Check + executor:** `mechanized` — `node --test test/media-generation-mcp.test.js`.
- **Pass condition:** exit 0; confirm-less generate has a zero transport count; no tool name approves media.
- **Test levels:** tool-handler integration, temporary database, injected transport. No live OpenAI call. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- Completes capability parity with ADR-0004. SPEC-018 documents these names for the authoring agent.
- Produces **Change:** three MCP tools.
- **Provenance tag:** `INIT-004/SPEC-012`
