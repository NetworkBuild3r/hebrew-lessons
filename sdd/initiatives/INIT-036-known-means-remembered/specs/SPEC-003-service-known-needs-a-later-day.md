# SPEC-003 · service: the known rule needs a later day, rule version 4

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-036
title: "The known rule needs a later day, rule version 4"
domain: service
status: complete
primary_prompt: none
complexity: high
complexity_reason: changes a rule that the headline, brief tier, coverage check and history all read, plus a history rebuild
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-001, SPEC-002]
blocks: [SPEC-006]
```

## Description

`internal/learner/state.go`: `Evidence` gains the success-day count from SPEC-002. `IsKnown` and `StateOf` keep their rules and add one: a skill is known, and so solid, only when checked successes fall on at least two study days (`KnownStudyDays = 2`).

A skill that passes everything else but has one day is `Learning`, and counts in the model's "waiting for a second day" summary field.

- The headline "Words you know" (known or solid in any skill, ADR-0027 D1) uses the new rule with no change of its own.
- The tier (`internal/brief/build.go:534`) and the coverage check (`internal/mcpstore/learner.go:51-71`) read the learner model, so they follow. Tests pin this; neither file should need an edit beyond tests.
- Bump the learner-model `rule_version` to 4 and rebuild history the way ADR-0030 did for 3 (`internal/db/daily.go`, `internal/web/progress.go` already handle mixed versions). `review_log` is never edited.
- Past say self-check rows stay in `review_log` and stop counting.

**Scope (out):** screens (SPEC-006), the say step (SPEC-004).

## Acceptance criteria

- [x] A skill with three same-day right answers, recall chance 0.9 and newest three right is `Learning`, not `Known`.
- [x] The same skill with one more checked right answer on a later study day is `Known`, and `Solid` once stability is 21 days.
- [x] Struggling and regressed rules are unchanged and still win in their existing order.
- [x] The model summary reports how many words are waiting for a second day.
- [x] The brief tier and the validator coverage set drop a day-one word and regain it after day two, covered by tests in `internal/brief` and `internal/mcpstore`.
- [x] The rebuild stamps rows with rule version 4, leaves `review_log` byte-identical, and a second run changes nothing.
- [x] Progress history already stamped 1 to 3 still renders.

## Verification Strategy

- **Claim:** the known state needs two study days, and every consumer follows it.
- **Check + executor:** mechanized. `go test ./internal/learner/... ./internal/brief/... ./internal/mcpstore/... ./internal/db/...`, plus an idempotence test on the rebuild.
- **Pass condition:** all acceptance cases pass; the rebuild test compares `review_log` before and after and finds no difference.
