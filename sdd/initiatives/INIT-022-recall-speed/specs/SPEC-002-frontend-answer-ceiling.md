# SPEC-002 · frontend: Ignore a meaning answer left open too long

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-022
title: "Ignore a meaning answer left open too long"
domain: frontend
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 1 hour
blocked_by: []
blocks: []
```

## Description

**Context:** `asked_at` is set when the question is shown. A learner who leaves and comes back the next day would otherwise store a day as their recall time.

**Scope (in):** `answerLatency` treats a gap longer than 60 seconds as untimed. Lesson tests for that gap, for a fast new word, and for a slow right answer.

**Scope (out):** Resetting `asked_at` on a later GET. The player still measures from the original show time, and a gap past a minute stores 0.

## Acceptance Criteria

- [x] ac-1: A meaning answer two hours after the question was shown stores `latency_ms` 0.
- [x] ac-2: With ten earlier timed right answers, a fast answer on a new word is Good and due within 15 minutes.
- [x] ac-3: A right answer slower than twice that median is Hard.

## Deliverables

- [ ] `internal/web/lesson.go`
- [ ] `internal/web/lesson_attempts_test.go`

## Verification Strategy

- **Claim:** a closed tab is not a slow recall, and a new word stays close.
- **Check + executor:** mechanized — the three tests named in the criteria.
- **Pass condition:** ac-1 to ac-3 tests pass.

## Integration Points

- **Provenance tag:** `INIT-022/SPEC-002`
