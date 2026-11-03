# SPEC-002 · service: one question, and a blank that is really blank

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-028
title: "One question per word, and a blank that is really blank"
domain: service
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003]
```

## Description

A review sitting asks each due word once, in the skill that is weak, as a question the server can grade.

`blankAskedLetter` today returns the original example when the asked letter is not an exact slice of it. That step must not be served. The prompt that is stored and the letter that is graded are the same blanked string. A choice that still occurs in the prompt is not one of the options.

`RequeueGap` does not put a missed review question back into the same sitting. The answer is scheduled and the next card is a different word. The speed rule already maps a checked answer onto Again, Hard, Good, and Easy. The learner does not send a grade label.

## Acceptance criteria

- [ ] A sitting built from due cards contains each word and skill once.
- [ ] Answering a card wrong does not place that card later in the same sitting.
- [ ] A letter step whose example cannot be blanked is omitted, and no grade is written for it.
- [ ] The graded letter is absent from the served prompt, and every served choice is absent from that prompt except the one that fills the blank.
- [ ] A right answer and a wrong answer update the card through the existing speed rule, with no posted grade label.
