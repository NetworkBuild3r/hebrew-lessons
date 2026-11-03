# SPEC-005 · service: a pattern drill is a checked two-choice pick

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-029
title: "A pattern drill is a checked two-choice pick"
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
blocks: [SPEC-006]
```

## Description

A pattern card's key is two forms, such as הוא/היא. `fillDrill` in `internal/web/learn.go` currently loads both sides with their glosses for a self-graded flashcard. Instead it builds a question with one asked side, chosen the same way the letter drill chooses one with `pickPart`:

1. When the two sides have different glosses, the cue is the asked side's English gloss.
2. When they share a gloss, the cue is the learner's own agreement-miss example (`confusionExample` with the agreement miss kind), with the asked form blanked. The blank follows the letter rule: if the form cannot be removed from the example, this cue is not used.
3. When neither cue exists, the step is not served, and `askable` skips it without writing a grade.

The answer posts one of the two served options. `DrillAnswered` grades it through the speed rule, as it does for a letter pick. A wrong answer is not asked again in that sitting.

## Acceptance criteria

- [ ] A pattern pair with different glosses is asked by gloss and graded right or wrong.
- [ ] A pattern pair with the same gloss and an agreement example is asked with a real blank.
- [ ] A pattern pair with neither is skipped and writes no grade.
- [ ] A posted option that was not served is a 400.
- [ ] No pattern drill in review reaches a self-grade path.
