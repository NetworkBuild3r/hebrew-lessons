# SPEC-003 · api: Report speed from the right answer of the same skill

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-022
title: "Report speed from the right answer of the same skill"
domain: api
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 2 hours
blocked_by: []
blocks: []
```

## Description

**Context:** `recallMs` was the latest timed `recall` attempt for the word, including a wrong one, and it was copied onto both of the word's cards.

**Scope (in):** The coach subquery keeps `correct = 1` and `skill = c.skill`. The MCP descriptions say so. Tests cover the query and `recallSpeed`.

**Scope (out):** A separate median for answers given on a later day.

## Acceptance Criteria

- [x] ac-1: A wrong timed answer leaves `HasRecall` false.
- [x] ac-2: A right meaning answer sets `RecallMs` on the read card only.
- [x] ac-3: `recallSpeed` is null with fewer than ten timed right answers, and `slow` when the latest is past twice the median.

## Deliverables

- [ ] `internal/db/coach.go`
- [ ] `internal/db/coach_test.go`
- [ ] `internal/mcp/coach.go`
- [ ] `internal/mcp/server.go`
- [ ] `internal/mcp/tool_learner_words.go`
- [ ] `internal/mcpstore/coach_test.go`

## Verification Strategy

- **Claim:** the agent sees the latest timed right answer for that skill.
- **Check + executor:** mechanized — `go test ./internal/db ./internal/mcpstore ./internal/mcp`.
- **Pass condition:** ac-1 to ac-3 tests pass.

## Integration Points

- **Provenance tag:** `INIT-022/SPEC-003`
