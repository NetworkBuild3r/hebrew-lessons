# SPEC-003 · mcp: the registered tool list must name the data and the actions

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-031
title: "The registered tool list must name the data and the actions"
domain: mcp
status: planned
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-002]
blocks: [SPEC-004]
```

## Description

A test lists the tools the MCP server registers, the JSON a client receives, and fails if any of these are absent:

- `get_lesson_brief` input `unitId`
- `get_lesson_brief` output `nextPattern`, `pace`, `previousUnits`
- `get_context` output `progress.state` and `summary`
- `get_context` description says a started unit may be replaced
- `put_unit`, `unpublish_unit`, and `delete_unit` are present
- `put_unit` description does not contain "cannot be revised"
- `delete_unit` description says `lesson_runs` are kept

The test does not call a network host and does not write a unit.

## Acceptance criteria

- [ ] The test passes after SPEC-002.
- [ ] A description that still says a started unit cannot be revised fails the test.
- [ ] Dropping `delete_unit` from the tool list fails the test.
