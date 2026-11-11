# SPEC-001 · docs: ADR-0013 research-shaped plans

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-010
title: Record how a plan delivers the research
domain: docs
status: complete
primary_prompt: .claude/agents/docs-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 2 hours
blocked_by: None
blocks: [SPEC-002, SPEC-003, SPEC-004, SPEC-005]
```

## Description

**Context:** ADR-0012 says a lesson is stored blocks and sent authors away from the lesson editor. The operator corrected that on the evening of 2026-10-14. The decision to record is the Family plan in the initiative, sections 2 through 4.

**Scope (in):** `docs/decisions/ADR-0013-research-shaped-plans.md`. A metadata line on ADR-0012 and ADR-0003 saying ADR-0013 corrects the authoring reading.

**Scope (out):** Code. Changes to the three published worksheets.

## Acceptance criteria

- [ ] ac-1: ADR-0013 states D1–D8, the Family table, no maximum lesson count, the running word list, the learner's place, and practice of reached words and verb forms — covered: no
- [ ] ac-2: ADR-0013 says research checks are warnings and names `later-context`, `N12`, picture-on-Type-it, and the closed word-list warning — covered: no
- [ ] ac-3: ADR-0013 says the review scheduler, not the plan order, is the later day — covered: no
- [ ] ac-4: ADR-0012's extended-by or corrected-by line points at ADR-0013 — covered: no

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | which document is the course | The plan. Worksheets remain only for the three published lessons | resolved | yes |

## Deliverables

- [ ] `docs/decisions/ADR-0013-research-shaped-plans.md`
- [ ] One metadata line each on ADR-0012 and ADR-0003

## Verification

- **Claim:** a reader can rebuild the Family plan from the ADR alone, and can see that warnings do not refuse a save.
- **Check + executor:** `human` — the operator, against initiative sections 2 and 3.
- **Pass condition:** D1–D5, the Family table, the four warning names, and the later-day sentence are all in ADR-0013, and ADR-0012 points at it.

## Gates

No gate action. Execute only when the operator asks.
