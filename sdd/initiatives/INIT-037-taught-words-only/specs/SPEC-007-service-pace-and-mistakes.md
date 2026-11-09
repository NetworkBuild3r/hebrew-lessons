# SPEC-007 · service: one consolidation unit at a time, and mistakes that rotate

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-037
title: "One consolidation unit at a time, and mistakes that rotate"
domain: service
status: complete
primary_prompt: none
complexity: medium
complexity_reason: changes two brief functions that every new unit reads
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-002]
blocks: [SPEC-008, SPEC-011]
```

## Description

- **Pace.**
  - **Today.** `PaceFor` (`internal/brief/build.go:413-429`) returns a zero-new-word consolidation brief whenever more than 40 words are learning, or accuracy is under 0.75 with at least one review. It does this for every position in a row, so 23 units in a row consolidated.
  - **Change.** When the learner already has one untouched consolidation unit queued ahead, the brief builder returns no unit brief. The reply says "review first" and gives the due-card count (ADR-0032 D4). The thresholds are unchanged.
  - **Ordering.** INIT-036 SPEC-003 will raise the Learning count (it reads `build.go:573-575`). Ship this first, so that change cannot fill the path with consolidation units.
- **Mistakes.**
  - **Today.** `fixMistakes` (`internal/brief/progression.go:213-246`) skips a pair only when its right form is in both previous units' final checkpoints (`skipRightForms`, `:250-265`). With a small mistake bank, that cycles two sets A A B.
  - **Change.** Skip a pair whose right form is in any of the previous three units' final checkpoints. When no pair is left, the brief carries no `fixMistakes`, and so no fix checkpoint.

**Scope (out):** the reviser skill (SPEC-008); the validator (SPEC-005, SPEC-006); INIT-036's known rule.

## Acceptance criteria

- [x] With 60 learning words and no queued consolidation unit, the brief consolidates as today.
- [x] With 60 learning words and one untouched consolidation unit queued ahead, `get_lesson_brief` returns no unit brief, with a reason naming review and the due count.
- [x] With a bank of four pairs, no pair is used by two units within any three consecutive positions.
- [x] With every pair used in the last three units, the brief has no `fixMistakes`.
- [x] Existing `PaceFor` and progression tests pass unchanged, except those this spec deliberately updates.

## Verification Strategy

- **Claim:** at most one consolidation unit is queued at a time, and mistake pairs do not recur within three units.
- **Check + executor:** mechanized. `go test ./internal/brief/... ./internal/mcp/...`, with a fixture path of ten positions built in sequence.
- **Pass condition:** the ten-position fixture has at most one queued consolidation unit at any moment, and no pair appears twice in any window of three.
