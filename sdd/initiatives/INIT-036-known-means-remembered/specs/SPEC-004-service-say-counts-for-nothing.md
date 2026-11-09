# SPEC-004 · service: the say step writes nothing

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-036
title: "The say step writes nothing"
domain: service
status: complete
primary_prompt: none
complexity: medium
complexity_reason: removes a write path that two lesson handlers and the recorder share
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: standard
estimated_effort: 3 hours
blocked_by: [SPEC-001]
blocks: [SPEC-005]
```

## Description

A self-reported say check is not evidence until something can check what was said. Remove its writes.

- `Recorder.SayCheck` (`internal/evidence/recorder.go:286-326`) no longer writes an attempt, a review or a card. Remove it and `ActivitySay`'s writer use, or reduce it to a no-op that the handlers no longer call.
- The two lesson call sites (`internal/web/lesson.go:365` and `:438`) stop posting a Got it / Not yet result. The step still plays the model and continues.
- A `say` activity stays valid in a unit. It counts as an exposure for the line's words, as `shadow` does, so a unit that uses it still publishes.
- The validator, authoring guide (`internal/mcp/resources/authoring.md`) and the author skill's wording stop describing say as graded.
- Data already written stays. SPEC-002 and SPEC-003 stop it counting.

**Scope (out):** the screen (SPEC-005), recording and speech recognition (a later phase).

## Acceptance criteria

- [x] Finishing a say step writes no `attempts` row, no `review_log` row and no `cards` row.
- [x] A say step still advances the lesson and plays its model line once.
- [x] A unit with a `say` step still validates and publishes.
- [x] `go build ./...` has no remaining caller of the removed write path.
- [x] The authoring guide and the author skill no longer say a say step is self-graded or feeds the scheduler.
- [x] Existing say rows are untouched by this spec.

## Security

- **Sensitivity:** standard. It removes a learner-data write path and adds none. Check that no endpoint still accepts a say result.

## Verification Strategy

- **Claim:** the say step leaves no learner data.
- **Check + executor:** mechanized. A lesson handler test posts the say reveal and continue, then counts rows in the three tables; `go test ./internal/web/... ./internal/evidence/...`.
- **Pass condition:** the three row counts are unchanged after the step, and the handler returns the next step.
