# SPEC-007 · content: revise learner 1's untouched units

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-030
title: "Revise learner 1's untouched units"
domain: content
status: deferred
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: [production_content_write]
reversibility_class: remote_reversible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-006]
blocks: []
```

## Description

After the new image is live (`/version` shows the commit), run the reviser skill against learner 1 through the MCP. Each untouched unit is revised in position order. Started and finished units are left alone.

Before the first write, record each untouched unit's id and its current published revision, so any one can be republished.

Write a before-and-after table to the completion summary, one row per revised unit:

- Story tokens per lesson, before and after.
- Single-word drill share, before and after.
- Previous-unit words reused.
- The pattern taught.
- Whether the brief asked for consolidation.

Then call `get_lesson_brief` for a new unit. With no untouched units left, it builds. Its three mistakes differ from the last two units' three.

## Acceptance criteria

- [ ] Every untouched unit has a new published revision that passed `validate_unit` with its revision brief.
- [ ] No started or finished unit has a new revision.
- [ ] The table is in the completion summary, with the prior revision of each unit.
- [ ] A new brief after the run names a next pattern and a pace, and its mistakes rotated.
