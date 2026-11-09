# SPEC-010 · web: Today skips an untouched unit that blocks

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-037
title: "Today skips an untouched unit that blocks"
domain: web
status: complete
primary_prompt: none
complexity: low
complexity_reason: one more condition when Today picks the next lesson, reading SPEC-009's count
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-009]
blocks: [SPEC-011]
```

## Description

Today offers the next published unit (`views.ActionNextLesson`, chosen in `internal/web/shell.go`). Today should not offer an untouched learner unit that has at least one block under SPEC-009's count; it moves on to the next unit, or to review. Leave a started unit alone: the learner can finish what they began. Shared units are unchanged.

**Scope (out):** the count itself (SPEC-009); hiding by report, which already exists (`internal/web/shell.go:330-343`).

## Acceptance criteria

- [x] An untouched learner unit with `blocks > 0` is never Today's next lesson.
- [x] A started learner unit with `blocks > 0` is still offered as "continue".
- [x] When every forward unit is skipped, Today shows review, as it does when no lesson is left.
- [x] `go test ./internal/web/...` passes, including the existing Today action tests.

## Verification Strategy

- **Claim:** Today never starts a learner on a unit that today's rules block.
- **Check + executor:** mechanized. `go test ./internal/web/...` with fixture units at block counts 0 and 1, untouched and started.
- **Pass condition:** every acceptance case passes.
