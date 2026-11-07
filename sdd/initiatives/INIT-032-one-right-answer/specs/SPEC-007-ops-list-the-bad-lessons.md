# SPEC-007 · ops: list every published unit the new rules warn on

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-032
title: "List every published unit the new rules warn on"
domain: ops
status: planned
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-006]
blocks: [SPEC-008]
```

## Description

Over the MCP connection, for every published unit (shared, and each learner's), read it with `get_unit` and pass the document to `validate_unit` as a dry run. Keep the findings for `unit.cloze_context`, `unit.cloze_translation`, `unit.lesson_repeat`, and `unit.drill_share`.

Write the list to `sdd/initiatives/INIT-032-one-right-answer/bad-lessons.md`: unit id, scope, position, title, progress state from `get_context`, and the rule ids with their pointers. Record counts only for activity text. Do not copy whole lessons into the file.

This spec only reads. It calls no `put_unit`, `publish_unit`, `unpublish_unit`, or `delete_unit`.

## Acceptance criteria

- [ ] Every published unit was checked, and the file says how many.
- [ ] Unit 12 is on the list with its four `unit.cloze_context` findings and two `unit.lesson_repeat` findings.
- [ ] No write tool was called.
