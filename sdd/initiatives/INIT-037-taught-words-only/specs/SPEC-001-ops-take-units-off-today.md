# SPEC-001 · ops: take units 12 to 34 off Today

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-037
title: "Take units 12 to 34 off Today"
domain: ops
status: done
primary_prompt: none
complexity: low
complexity_reason: one existing MCP tool called per unit, no code change
model: inherit
autonomy_level: human_approval
gate_actions: [production_content_write]
reversibility_class: revertible
security_review: light
estimated_effort: 1 hour
blocked_by: []
blocks: []
```

## Description

Call `unpublish_unit` with `learnerId` 1 for every unit ASMT-004 found built from the broken structure: units 12 to 34, positions 11 to 33. Each goes back to draft and keeps its document, and the server writes an audit row with the caller (`internal/db/learner_publish.go:213-261`). The brief each was built from stays used, so a replacement needs a new brief.

Brian approved the gate on 2026-11-08, choosing "all 23, including started unit 12".

**Scope (out):** rewriting any unit (SPEC-012); units 1 to 11, which are finished.

## Acceptance criteria

- [x] Every unit in the list returns `status: draft`, `unchanged: false`.
- [x] `get_unit` 12 returns "unit is not published".

## Result (2026-11-08)

| Unit id | Position | State before | Revision kept |
| --- | --- | --- | --- |
| 12 | 11 | started | 2 |
| 13 | 12 | untouched | 2 |
| 14 | 13 | untouched | 2 |
| 15 | 14 | untouched | 2 |
| 16 | 15 | untouched | 3 |
| 17 | 16 | untouched | 2 |
| 18 | 17 | untouched | 2 |
| 19 | 18 | untouched | 2 |
| 20 | 19 | untouched | 3 |
| 21 | 20 | untouched | 2 |
| 22 | 21 | untouched | 2 |
| 23 | 22 | untouched | 2 |
| 24 | 23 | untouched | 2 |
| 25 | 24 | untouched | 2 |
| 26 | 25 | untouched | 2 |
| 27 | 26 | untouched | 4 |
| 28 | 27 | untouched | 2 |
| 29 | 28 | untouched | 3 |
| 30 | 29 | untouched | 2 |
| 31 | 30 | untouched | 2 |
| 32 | 31 | untouched | 2 |
| 33 | 32 | untouched | 3 |
| 34 | 33 | untouched | 4 |

All 23 calls returned `unchanged: false`, and none had been unpublished already. Units 1 to 11 were not changed and are still published.

## Verification Strategy

- **Claim:** none of units 12 to 34 is published for learner 1, and each is kept as a draft.
- **Check + executor:** mechanized. `get_unit` per id returns "unit is not published"; the audit table has one `unpublish` row per unit.
- **Pass condition:** 23 of 23 not published, and 23 audit rows. Verified for unit 12 by `get_unit` on 2026-11-08; the other 22 by each call's own `status: draft` reply.
