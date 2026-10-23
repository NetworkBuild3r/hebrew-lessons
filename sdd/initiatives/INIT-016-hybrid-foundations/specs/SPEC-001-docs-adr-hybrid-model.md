# SPEC-001 · docs: ADR-0019, the hybrid model

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-016
title: "ADR-0019: the hybrid model"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 2 hours
blocked_by: []
blocks: [SPEC-002, SPEC-004, SPEC-005, INIT-017/SPEC-001, SPEC-007, INIT-021/SPEC-001]
```

## Description

**Context:** The design doc `docs/design/learner-without-a-tutor.md` records the hybrid model and decisions D1–D10. ADR-0018 governs v3 (learner model, brief, MCP). The hybrid model extends it and needs its own record so later specs can cite one source.

**Scope (in):** `docs/decisions/ADR-0019-hybrid-learning-model.md`: context, the three sources that decide a lesson, decisions D1–D10 with alternatives considered, consequences, and the spec list. Link it from the design doc and from `docs/north-star.md` row "6 · Hybrid".

**Scope (out):** Any code.

## Acceptance Criteria

- [ ] ac-1: ADR-0019 exists, states D1–D10 each with the option chosen and at least one alternative rejected, and cites the design doc and ADR-0018. (REQ-all)
- [ ] ac-2: The design doc and the north star link to ADR-0019.
- [ ] ac-3: The ADR names no tutor or learner.

## Deliverables

- [ ] `docs/decisions/ADR-0019-hybrid-learning-model.md`
- [ ] Links in `docs/design/learner-without-a-tutor.md` and `docs/north-star.md`

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none. **Rollback:** revert the commit.

## Security

- **Sensitivity:** light. No code, no data.

## Verification Strategy

- **Claim:** the ADR records every decision the later specs rely on.
- **Check + executor:** human — the operator reads the ADR against the design doc's Decisions list. Mechanized — `rg -n "D10" docs/decisions/ADR-0019-*.md` finds the last decision.
- **Pass condition:** each of D1–D10 in the design doc has a matching entry in the ADR, and the operator approves.

## Integration Points

- Every other spec cites ADR-0019.
- **Provenance tag:** `INIT-016/SPEC-001`
