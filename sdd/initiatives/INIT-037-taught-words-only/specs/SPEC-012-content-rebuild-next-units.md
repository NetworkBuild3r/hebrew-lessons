# SPEC-012 · content: rebuild learner 1's next units one or two at a time

## Metadata

```yaml
spec_id: SPEC-012
initiative_id: INIT-037
title: "Rebuild learner 1's next units one or two at a time"
domain: content
status: planned
primary_prompt: none
complexity: medium
complexity_reason: live publishes through the fixed pipeline, each checked against the new rules
model: inherit
autonomy_level: human_approval
gate_actions: [publish_live]
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-011]
blocks: []
```

## Description

This spec takes over INIT-032 SPEC-008. Units 12 to 34 are drafts (SPEC-001).

- **Unit 12.** Learner 1 started it, so it can't be revised. Write a fresh unit at its position instead: a new brief, `validate_unit` with the brief, audio, and `put_unit` with `publish: true` and the new `briefId`. Unit 12 stays a draft with its revisions.
- **The rest.** Rebuild the next one or two positions the same way, with the hebrew-lesson-author skill as SPEC-008 changed it. Then stop until learner 1 reaches them (ADR-0032 D6).
- **When a brief says "review first".** Write nothing, and leave a coach note saying so.

**Scope (out):** units 1 to 11, which are finished. A finished unit is rewritten only if a later spec finds learner 1 replaying it.

## Acceptance criteria

- [ ] Each published unit validates with its brief with no block, and SPEC-009 reports `blocks: 0` for it.
- [ ] Each has at least one new word, unless its brief consolidates; no lesson repeats another; and no cloze is one word.
- [ ] No more than two forward units are published ahead of learner 1's current position.
- [ ] A coach note on each unit names its brief and what it practises.

## Verification Strategy

- **Claim:** what learner 1 sees next is built by the fixed pipeline and passes today's rules.
- **Check + executor:** mechanized for `validate_unit` with brief, and SPEC-009's count; human for one lesson played by Brian.
- **Pass condition:** every new unit is at `blocks: 0` with its brief. Brian plays its first lesson and reports no button he did not learn.
