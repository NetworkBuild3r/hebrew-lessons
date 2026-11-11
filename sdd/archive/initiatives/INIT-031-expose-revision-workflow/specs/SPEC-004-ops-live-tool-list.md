# SPEC-004 · ops: the live connection shows the same names

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-031
title: "The live connection shows the same names"
domain: ops
status: planned
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: remote_reversible
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-003]
blocks: []
```

## Description

Roll the image that contains SPEC-002 and SPEC-003. Wait until `/version` shows that commit.

Read the tool list from the MCP connection Cursor uses. Do not print the token. Confirm:

- `get_lesson_brief` accepts `unitId` and its result names `nextPattern`, `pace`, and `previousUnits`
- `get_context` names `progress` and says a started unit may be replaced
- `put_unit`, `unpublish_unit`, and `delete_unit` are listed
- `put_unit` does not say a started unit cannot be revised

Write the commit and the check into the completion summary. Do not call `put_unit`, `unpublish_unit`, `delete_unit`, or `get_lesson_brief` on the live server.

A mismatch means the connection is still the old server. Do not rewrite lessons until it matches. Rollback is the previous image. This spec changes no lesson row.

## Acceptance criteria

- [ ] `/version` shows the commit that contains SPEC-002.
- [ ] The live tool list includes every name in the description above.
- [ ] The completion summary records the commit and the check.
- [ ] No unit was created, replaced, unpublished, or deleted.
