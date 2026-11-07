# SPEC-001 · docs: a graded question shows one right answer

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-032
title: "ADR: a graded question shows one right answer"
domain: docs
status: planned
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 1 hour
blocked_by: []
blocks: [SPEC-003, SPEC-004]
```

## Description

Write `docs/decisions/ADR-0028-one-right-answer.md`. ADR-0027 is reserved by INIT-031/SPEC-001, so confirm 0028 is free before writing it.

The record says:

- A graded question has exactly one right answer given what is on screen. A wrong button that also makes a true sentence is a bug in the question, and the learner is never marked wrong for it.
- On a `cloze`, what is on screen is the Hebrew line with the gap and the English of the line. The buttons stay pointed Hebrew with no reading and no gloss (ADR-0025, INIT-029/SPEC-003).
- A cloze is a sentence: the gap and at least one more word. One word is a `produce`.
- The author lists other words that fit in `accept`. The player never offers them as wrong buttons.
- A lesson does not repeat an earlier lesson of its unit step for step. Checkpoints review on purpose and are exempt.
- New writes block. The current published revision warns and keeps serving until it is rewritten.

It extends ADR-0025 (practice that teaches) and ADR-0026 (lessons that build). It changes no earlier rule.

## Acceptance criteria

- [ ] `docs/decisions/ADR-0028-one-right-answer.md` exists and names the four rules above.
- [ ] It cites the lesson 12 screens as the evidence: "אֲנִי __" with four verbs, and a one-word gap.
- [ ] It links ADR-0025 and ADR-0026.
