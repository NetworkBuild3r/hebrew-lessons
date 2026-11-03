# SPEC-006 · frontend: pattern pick screen, and no self-grade frame in review

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-029
title: "Pattern pick screen, and no self-grade frame in review"
domain: frontend
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 3 hours
blocked_by: [SPEC-005]
blocks: []
```

## Description

A pattern drill uses the two-option pick layout that the letter drill already uses, posting to `/_hx/review/drill`. The cue is either the English gloss or the blanked example. After the answer, both forms are shown with their glosses and the clip plays.

After this change, no review mode renders `flashFrame` or `GradeLabels`. Both stay for the flashcard sitting.

## Acceptance criteria

- [ ] A pattern drill renders two options and no Show answer, Forgot, Hard, Got it, or Easy.
- [ ] A test that walks every review mode finds no self-grade button.
- [ ] The flashcard sitting still shows its four buttons.
