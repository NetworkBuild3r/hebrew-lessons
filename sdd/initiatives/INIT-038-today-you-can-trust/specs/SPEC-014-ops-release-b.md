# SPEC-014 · ops: release B, one announced change

## Metadata

```yaml
spec_id: SPEC-014
initiative_id: INIT-038
title: "Release B: one announced change"
domain: ops
status: complete
primary_prompt: none
complexity: medium
complexity_reason: local tests for the void, the undo, and the history replay
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: full
estimated_effort: 4 hours
blocked_by: [SPEC-004, SPEC-005, SPEC-008, SPEC-009, SPEC-010, SPEC-012, SPEC-013]
blocks: []
```

## Description

Release B is the void, the ladder, and the side-by-side numbers: SPEC-002, SPEC-004, SPEC-005, SPEC-008, SPEC-009, and SPEC-010.

This spec does not deploy. Production, `home_k3`, and a live data job are out of scope. The check is local:

1. `hebrew void-evidence` dry-runs on a fixture, prints counts, and writes nothing.
2. Apply without `--backup-taken` refuses.
3. Apply and undo on a fixture restore the card.
4. A voided answer does not count, and a void is limited to that learner.
5. History replay uses the same study-day cutoff the grader stores.
6. The 90-day freeze end date is written into ADR-0033. The code that shows "What changed" is already tested and drawn.

No unit is archived.

**What Brian will see differently:** his numbers move once, with the reason and the old numbers beside the new. Answers from lessons 12 to 34, and from unfair blanks, stop counting against him. Blanks no longer mark him wrong, and the "also right" tap appears.

**Removes:** the counting of every answer from units 12 to 34 and from blanks with no English cue (flagged, not deleted).

**Scope (out):** archiving units (Stage 2).

## Acceptance criteria

- [x] `TestVoidEvidenceDryRunPrintsCountsAndWritesNothing` and `TestVoidEvidenceApplyWithoutBackupRefuses` pass.
- [x] `TestVoidEvidenceApplyAndUndoOnAFixture` passes.
- [x] `TestVoidUnitWrongCoversOnlyThatLearnersWrongs` and `TestVoidedConfusionNoLongerCounts` pass.
- [x] Replay uses the same cutoff as grading (`internal/learner/replay.go`).
- [x] ADR-0033 D8 names the freeze end, 2027-02-08.
- [x] "What changed" is on the Progress phone, Figma `257:85` and `shots/progress-390.png`.

## Verification Strategy

- **Claim:** the void, the ladder, and the side-by-side numbers landed once, safely, with a baseline to measure against.
- **Check + executor:** local tests. No production job.
- **Pass condition:** the void tests pass, and the freeze date is in ADR-0033.
