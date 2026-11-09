# SPEC-005 · frontend: the say step without a grade

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-036
title: "The say step without a grade"
domain: frontend
status: complete
primary_prompt: none
complexity: low
complexity_reason: one template state and its copy, no new data
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-004]
blocks: []
```

## Description

The say step today shows the English cue, then "I said it", then the model, then "Got it" and "Not yet" (`internal/web/lesson.go:52-55`). With nothing recorded, the two buttons are misleading.

- Keep the cue, "I said it", and the model line with its clip.
- Replace "Got it" and "Not yet" with one Continue. No score, no right or wrong styling.
- The copy says this is practice, for example "Say it out loud, then hear it."
- Keep the 0.75× control from SPEC-007 available on the model clip once that spec lands.

**Scope (out):** recording the learner's voice and any checking.

## Acceptance criteria

- [x] The say reveal shows one Continue and no Got it or Not yet.
- [x] The step shows no correct or incorrect state and no score change.
- [x] The cue and model clip behave as before.
- [x] The step works at 360px wide with keyboard only, as the other steps do.
- [x] No template or JS still names the removed buttons.

## Verification Strategy

- **Claim:** the say step offers no self-grade and stays usable.
- **Check + executor:** mechanized. The lesson browser test (`internal/web/testdata`, run as the other `*-browser.mjs` tests are) walks a say step; `go test ./internal/web/...`.
- **Pass condition:** the walk finds one Continue and none of the removed controls, and completes the step with the keyboard.
