# SPEC-004 · api: Worksheet-type templates and the unit's type

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-017
title: "Worksheet-type templates and the unit's type"
domain: api
status: planned
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-002]
blocks: []
```

## Description

**Context:** Worksheet types are one way into practice: topic conversation, review sheet, verb workshop, short story with true/false, foundations and reading. Each should be a reusable template so a new class becomes a practice set quickly. Our own templates, not the class sheets (D8).

**Scope (in):**

- Five MCP resources `hebrew://templates/{type}`: the activity sequence, counts, which brief fields feed which step, and a filled example in our own words.
- `put_unit` and `validate_unit` take an optional `worksheetType` (one of the five); stored on the unit (a column added in the next free migration, or folded into 013 if SPEC-001 has not landed).
- Validator rule `unit.worksheet_shape`: a unit that declares a type follows its template's required steps (for example, a short story with true/false has a `story` step and at least three true/false items).
- `get_context` lists the templates.

**Scope (out):** A browse-by-type page (follow-up once units carry types).

## Acceptance Criteria

- [ ] ac-1: Each of the five templates is readable over MCP and contains no tutor or learner names. (REQ-014)
- [ ] ac-2: A unit declaring a type that lacks a required step fails `unit.worksheet_shape`. (REQ-014)
- [ ] ac-3: The unit's type is stored and returned by `get_unit`.

## Deliverables

- [ ] `internal/mcp/resources/templates/*.md`, registration, `put_unit`/`validate_unit` field, validator rule, migration column
- [ ] Tests

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none (the column is additive; ride an existing migration gate). **Rollback:** revert.

## Security

- **Sensitivity:** standard — schema extension from MCP input; enum-checked.

## Verification Strategy

- **Claim:** authors can build a unit from a type template and the shape is enforced.
- **Check + executor:** mechanized — `go test ./internal/mcp/... ./internal/units/...`.
- **Pass condition:** ac-1 to ac-3 tests pass.

## Integration Points

- INIT-016/SPEC-004 runbook maps class sheets to types.
- **Provenance tag:** `INIT-017/SPEC-004`
