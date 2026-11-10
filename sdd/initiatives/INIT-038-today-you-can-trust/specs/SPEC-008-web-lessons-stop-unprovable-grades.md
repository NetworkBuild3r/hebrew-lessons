# SPEC-008 · web: lessons stop grading what they cannot prove

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-038
title: "Lessons stop grading what they cannot prove"
domain: web
status: in_review
primary_prompt: none
complexity: medium
complexity_reason: turns three step kinds into ungraded reveals on both the render and post paths, which must keep agreeing
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: [SPEC-014]
```

## Description

A lesson's buttons are chosen at play time from taught words, and only an exact gloss match is kept out (`glossKey`, `internal/web/lesson.go:1678`). So "אַתָּה __ פֹּה?" in unit 1 lesson 2 has three right buttons. ADR-0033 says an answer the app cannot prove wrong records nothing.

- **Blanks become reveals.** A lesson `cloze` that would show buttons uses the existing reveal, the path already used for fewer than three buttons, in every unit, finished units 1 to 11 included. The render path and the post path (`askChoiceCount`) still agree, and the post path writes no attempt.
- **AI-written sentence keys are not graded.** Every unit in Stage 1 is AI-written, so `order` steps and multi-word `listen_pick` steps become ungraded reveals too: their answer key is an AI-written sentence (ADR-0033). Brian may narrow this to blanks only in SPEC-001's review. If he does, drop this item.
- **Fairer wrong buttons.** On the graded asks that remain, a wrong button is dropped when `activity.GlossesOverlap` (`internal/activity/distractors.go:76`) matches the target, instead of only an exact `glossKey` match. This applies in `wideChoices` (`lesson.go:1386`) and `lessonChoices` (`:1557`).
- **Review stays blank-free.** A test pins that review has no sentence-blank kind (`internal/activity/types.go:15-23`).

**What Brian will see differently:** a fill-the-gap step shows the answer and moves on, with no wrong mark, and the remaining buttons no longer include a second right answer.

**Removes:** graded word-choice blanks, graded `order` and multi-word `listen_pick` steps, and the exact-match-only gloss check.

**Scope (out):** the "also right" tap and Report a problem (SPEC-009); one question engine for lessons and review (Stage 2).

## Acceptance criteria

- [ ] Across 50 seeds on every lesson step of a fixture path, no `cloze`, `order`, or multi-word `listen_pick` step writes an attempt.
- [ ] On a fixture like unit 1 lesson 2, no wrong button's gloss overlaps the target's gloss.
- [ ] The render path and the post path agree on every fixture step.
- [ ] A test fails if a sentence-blank kind is added to review.

## Verification Strategy

- **Claim:** a lesson never records a wrong answer it cannot prove wrong.
- **Check + executor:** mechanized. `go test ./internal/web/... ./internal/activity/...` with a 50-seed sweep over a fixture path.
- **Pass condition:** every acceptance case passes.
