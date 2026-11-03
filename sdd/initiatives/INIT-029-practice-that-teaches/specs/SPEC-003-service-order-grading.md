# SPEC-003 · service: serve and grade a sentence build

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-029
title: "Serve and grade a sentence build"
domain: service
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-002]
blocks: [SPEC-004]
```

## Description

The lesson player serves an order step as chips in an order shuffled with a seed from the run and the step, the same way `choiceSeed` seeds choices. If the shuffle matches the answer or an `accept` order, it is rotated by one. The served order is reproducible from the seed, so the post handler can rebuild it without trusting the client.

The learner posts a list of chip positions. The answer is right when those positions spell `text` or any `accept` entry, compared token by token in NFC. One try. The attempt is recorded with activity `order` and right or wrong, through the same lesson attempt path as cloze. A lesson attempt leaves every card alone, as today.

`gradedActivity` and `activityPrompt` include `order`. The prompt is "Put the words in order."

## Acceptance criteria

- [ ] The served order is never the right answer and is the same on every load of the same step.
- [ ] A post that uses every served chip once, in the right order or an accepted order, records a right attempt.
- [ ] Any other permutation records a wrong attempt.
- [ ] A post with a missing, repeated, or unknown position is a 400 and writes nothing.
- [ ] The reveal phase carries the line's reading, the English, and the clip.
