# SPEC-005 · service: "Can say" read model

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-016
title: "Can say read model"
domain: service
status: planned
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 3 hours
blocked_by: [SPEC-001]
blocks: [SPEC-006]
```

## Description

**Context:** Accountability is a research rule in the hybrid model ("short and daily", a visible "you can now say these things"). The learner model already holds a state per word and skill (`internal/learner`), and daily snapshots exist. "Recall" is English → Hebrew production, the closest measured skill to "say".

**Scope (in):** `learner.CanSay(ctx, userID) (CanSay, error)`:

- `Items`: lexemes whose `recall` state is `known` or `solid`, phrases (multi-token lexemes) first, then by most recent move into known, each with text, translit and gloss.
- `Total` and `AddedThisWeek` (moved into known or solid for recall in the last 7 days, from snapshots or state history; a word that regressed out is not counted).
- `Next`: up to 5 lexemes in `learning` for recall with the highest recent accuracy — "almost there".

**Scope (out):** Pages (SPEC-006). A `say` skill (later, with SPEC-017's attempts).

## Acceptance Criteria

- [ ] ac-1: A word with recall `known` or `solid` is in `Items`; one in `learning`, `struggling` or `regressed` is not. (REQ-002)
- [ ] ac-2: `AddedThisWeek` counts only words that entered known or solid in the last 7 days by the injected clock and are still there. (REQ-002)
- [ ] ac-3: The read is scoped by the user id; another learner's states never appear.

## Deliverables

- [ ] `internal/learner/cansay.go` and tests with fixed-clock fixtures

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none. **Rollback:** revert.

## Security

- **Sensitivity:** standard — reads one learner's data; the user id comes from the session.

## Verification Strategy

- **Claim:** the list and the weekly number match the learner model's states.
- **Check + executor:** mechanized — `go test ./internal/learner/...`.
- **Pass condition:** table tests for each state, a week boundary test, and a two-learner isolation test pass.

## Integration Points

- SPEC-006 renders it. SPEC-015's word bank can reuse it.
- **Provenance tag:** `INIT-016/SPEC-005`
