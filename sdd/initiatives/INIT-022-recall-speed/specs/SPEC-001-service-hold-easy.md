# SPEC-001 · service: Hold Easy until the card is in review

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-022
title: "Hold Easy until the card is in review"
domain: service
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 2 hours
blocked_by: []
blocks: []
```

## Description

**Context:** A fast right answer is Easy. On a card that does not exist yet, Easy schedules the next review 16 days out. The word was just taught.

**Scope (in):** `holdEasyUntilReview` in the recorder. Easy becomes Good when the card is missing, new, or learning. Review and relearning keep Easy. Tests for the clamp and for `RecallSpeed`.

**Scope (out):** The latency ceiling (SPEC-002). The coach query (SPEC-003).

## Acceptance Criteria

- [x] ac-1: A fast answer on a new card returns Good and the card is due within 15 minutes.
- [x] ac-2: A fast answer on a card already in review returns Easy.
- [x] ac-3: `RecallSpeed` is empty without a time or a median, fast under the median, steady up to twice the median, and slow past that.

## Deliverables

- [ ] `internal/evidence/recorder.go`
- [ ] `internal/evidence/recorder_test.go`
- [ ] `internal/evidence/grade_test.go`
- [ ] `internal/evidence/doc.go`

## Verification Strategy

- **Claim:** a word just taught is not sent weeks away for a fast answer.
- **Check + executor:** mechanized — `go test ./internal/evidence`.
- **Pass condition:** ac-1 to ac-3 tests pass.

## Integration Points

- **Provenance tag:** `INIT-022/SPEC-001`
