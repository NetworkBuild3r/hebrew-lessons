# SPEC-009 · web: "My answer was also right", and Report a problem voids

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-038
title: "\"My answer was also right\", and Report a problem voids"
domain: web
status: in_review
primary_prompt: none
complexity: medium
complexity_reason: a learner action that changes the learner's own record, which must be limited to their own answers
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: full
estimated_effort: 4 hours
blocked_by: [SPEC-004]
blocks: [SPEC-014]
```

## Description

- **The tap.** A "My answer was also right" button appears on the feedback after any wrong graded answer, in lessons and in review (`internal/web/learn.go`, `internal/web/lesson.go`, and the player views).
  - The handler finds the attempt from the signed-in learner's own session.
  - It refuses an attempt that belongs to anyone else, and is CSRF-checked.
  - It calls SPEC-004's void with reason `also_right` and the learner as actor.
  - The feedback then says the answer was taken back.

  It only voids; it never credits a right answer.
- **Report a problem voids.** The Report a problem handler calls `ReportUnit` (`internal/db/learner_publish.go:296`) and then SPEC-004's unit void, so a reported lesson's wrong answers stop counting. It writes one void batch per report.
- **No MCP path.** No MCP tool can void, so the read-only smoke token cannot void.
- **Generated code.** Templates are regenerated with `templ generate`, never edited by hand.

**What Brian will see differently:** after a wrong mark he disagrees with, one tap takes it back, and reporting a lesson clears its wrong answers instead of only hiding the lesson.

**Removes:** the state where a reported unit is hidden but its wrong answers still count.

**Scope (out):** learning from the taps, such as an exclusion list (Stage 2); the void engine (SPEC-004).

## Acceptance criteria

- [ ] After a tap, the card equals its state before the answer (SPEC-004's test, run through the handler).
- [ ] The tap appears only after a wrong graded answer, never after a right one or an ungraded reveal.
- [ ] A request naming another learner's attempt voids nothing and returns an error.
- [ ] A request without a valid CSRF token voids nothing.
- [ ] Reporting a unit voids that learner's wrong answers in it, in one batch with reason `report`, and hides the unit as before.
- [ ] Each tap leaves one void row with reason `also_right`, so the exit's tap rate can be counted.

## Verification Strategy

- **Claim:** the learner can take back their own wrong grade exactly, and no one else's.
- **Check + executor:** mechanized. `go test ./internal/web/...` with two fixture learners, and handler tests for CSRF and ownership.
- **Pass condition:** every acceptance case passes, and the Phase 3.5 checklist in SPEC-014 finds no open Critical or High.
