# SPEC-010 · docs: Record the listening card and the word page

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-023
title: "Record the listening card and the word page"
domain: docs
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 1 hour
blocked_by: [SPEC-002, SPEC-004, SPEC-007]
blocks: []
```

## Description

**Context:** The North Star says each skill has its own state and the overall state is the weakest core skill. It does not say which activity creates a listening card, and nothing describes the word page.

**Scope (in):**
- `docs/north-star.md` section 4.2: a lesson `listen_pick` schedules the `listen` card, and a lesson's new word with audio gets one.
- Section 4.3: one sentence noting that an answered skill with no card is `seen`, so a skill without a card holds the overall state at `seen`.
- Section 9: the Progress overview and the word page.
- ADR-0021 records the listening card, the one-time replay, and why the overall rule stays strict.

**Scope (out):** ADR-0018. It is not staged or edited.

## Acceptance Criteria

- [ ] ac-1: The North Star sections 4.2, 4.3, and 9 agree with SPEC-002, SPEC-004, and SPEC-007.
- [ ] ac-2: ADR-0021 exists with Context, Decision, and Consequences.

## Deliverables

- [ ] `docs/north-star.md`
- [ ] `docs/decisions/ADR-0021-listening-cards-and-word-page.md`

## Verification Strategy

- **Claim:** the docs say what the app does.
- **Check + executor:** manual — read the sections against the merged code.
- **Pass condition:** ac-1 and ac-2 hold.

## Integration Points

- **Provenance tag:** `INIT-023/SPEC-010`
