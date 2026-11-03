# SPEC-007 · service: later-day results by kind of practice

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-029
title: "Later-day results by kind of practice"
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
blocks: [SPEC-008]
```

## Description

For one learner and one kind of practice (a lesson activity type such as recall, cloze, produce, order, or say, or a review activity such as read pick or type it), consider each word practiced that way on some study day. Take the first checked review answer for that word on a later study day in the learner's zone. The result is the share of those answers that were right, and how many answers there were.

A word practiced several ways on one day counts once for each way. Results look back 60 days. Below 20 answers, the result is "not enough yet". The query reads the existing attempt log and adds no table. A migration is allowed only for an index, if the query needs one.

## Acceptance criteria

- [ ] A test with seeded attempts across days gets the expected share and count for each kind of practice.
- [ ] A same-day review does not count as later-day.
- [ ] Study-day boundaries follow the learner's zone.
- [ ] Fewer than 20 answers reports "not enough yet" with the count.
- [ ] One learner's attempts never appear in another learner's results.
