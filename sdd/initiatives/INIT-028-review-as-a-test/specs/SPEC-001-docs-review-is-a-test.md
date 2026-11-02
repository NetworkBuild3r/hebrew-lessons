# SPEC-001 · docs: review is a checked test

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-028
title: "ADR: review is a checked test"
domain: docs
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 1 hour
blocked_by: []
blocks: [SPEC-002, SPEC-003]
```

## Description

Write the decision that a review sitting is a checked test of a word the learner already knows, and that Forgot, Hard, Got it, and Easy belong on a flashcard sitting only.

The record cites RSCH-003 for retrieval and spacing, and states that the choice of a checked test over self-grading is Brian's, because the research on recognition versus recall conflicts.

It also records the two product rules the screens depend on: a word and skill are asked once per sitting, and a missing-letter step is not shown unless the asked letter is actually absent from the prompt.

## Acceptance criteria

- [ ] A new ADR states the review sitting, the flashcard sitting, the once-per-sitting rule, and the blank gate.
- [ ] North-star text that still describes review as four self-grade buttons is updated to the checked test, in present tense.
- [ ] The ADR names INIT-027 as the previous review change and does not reopen its drill-ending or due-card rules.
