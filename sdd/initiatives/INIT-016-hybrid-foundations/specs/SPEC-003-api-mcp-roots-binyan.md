# SPEC-003 · api: MCP root and binyan on `upsert_lexemes` and `lookup`

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-016
title: "MCP: root and binyan on upsert_lexemes and lookup"
domain: api
status: planned
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 2 hours
blocked_by: [SPEC-002]
blocks: [INIT-019/SPEC-002]
```

## Description

**Context:** Class data arrives through MCP (ADR-0019 D3). `upsert_lexemes` and `lookup` must carry the fields SPEC-002 adds.

**Scope (in):** `lexemeInput` gains `root` and `binyan`; `lexemeInputs` maps them; the tool description says how to write a root and lists the binyan values; `lookup` returns both; `hebrew://authoring` and `internal/mcp/resources/authoring.md` explain them; the `hebrew-lesson-author` skill mentions them.

**Scope (out):** New tools.

## Acceptance Criteria

- [ ] ac-1: An admin `upsert_lexemes` call with `root` and `binyan` stores them; `lookup` returns them. (REQ-001)
- [ ] ac-2: A learner token calling `upsert_lexemes` is still refused (admin only).
- [ ] ac-3: The schema test pins the two new properties and their descriptions.

## Deliverables

- [ ] `internal/mcp/tool_upsert_lexemes.go`, lookup output, `schema.go` if it lists fields
- [ ] `internal/mcp/resources/authoring.md`, `.cursor/skills/hebrew-lesson-author/SKILL.md`
- [ ] Tests in `internal/mcp`

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none. **Rollback:** revert.

## Security

- **Sensitivity:** standard — admin write over MCP. Authorization unchanged; validation is SPEC-002's.

## Verification Strategy

- **Claim:** an MCP client can write and read roots and binyanim.
- **Check + executor:** mechanized — `go test ./internal/mcp/...`.
- **Pass condition:** the round-trip test (upsert then lookup) and the learner-refused test pass.

## Integration Points

- SPEC-004's runbook uses it first. INIT-019/SPEC-002 builds on the same tool file conventions.
- **Provenance tag:** `INIT-016/SPEC-003`
