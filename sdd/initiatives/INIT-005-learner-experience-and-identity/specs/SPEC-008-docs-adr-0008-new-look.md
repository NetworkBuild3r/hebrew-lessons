# SPEC-008 · Docs: ADR-0008 new look, tokens, feedback rules

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-005
title: "ADR-0008: new look, tokens and feedback rules"
domain: docs
status: ready
primary_prompt: .claude/agents/principal-architect/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-007]
blocks: [SPEC-009, SPEC-010]
```

---

## Description

**Context:** Operator picked a look in SPEC-007. This ADR records the pick, the new tokens, and the feedback rules. It supersedes the look part of ADR-0006. The 18 RSCH-006 interface rules, accessibility numbers and test-hook contract stay unless a named row changes.

**Scope (in):** ADR-0008; updated `tokens.json` (or a new assets file referenced by the ADR); ADR-0006 "Superseded in part" note; feedback rules from REQ-013–015 and RSCH-007.

**Scope (out):** applying tokens to CSS (SPEC-009/011); icons in shell (SPEC-010).

---

## Understood

- [ ] und-1: Dark mode still ships. Contrast and target sizes stay.
- [ ] und-2: Points, streaks, jokes, mascot stay out.

## Acceptance Criteria

- [ ] ac-1 (REQ-017): ADR-0008 names the pick and lists every token with light and dark values and measured contrast.
- [ ] ac-2 (REQ-013–015): feedback rules (correct/wrong with correct form, audio indicator, complete moment, sounds off by default, no points/jokes) are recorded.
- [ ] ac-3: ADR-0006 is marked superseded in part for the look only.

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | ADR number | 0008 by directory scan | resolved | yes — after 0007 |

> **Readiness** = 100.

---

## Deliverables

- [ ] `docs/decisions/ADR-0008-learner-look-and-feedback.md`
- [ ] Tokens asset
- [ ] ADR-0006 note
- [ ] Completion summary

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** files present; ADR number unique; ADR-0006 note.
- **Rollback plan:** delete ADR-0008; revert ADR-0006 note and tokens.

---

## Security

- **Sensitivity:** light.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** new look and feedback rules are recorded; ADR-0006 superseded in part.
- **Check + executor:** `manual`.
- **Pass condition:** ac-1 through ac-3.
- **Provenance tag:** `INIT-005/SPEC-008`
