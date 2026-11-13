# SPEC-018 · service: one review sitting of about 20 minutes

## Metadata

```yaml
spec_id: SPEC-018
initiative_id: INIT-039
title: "One review sitting of about 20 minutes"
domain: service
status: complete
primary_prompt: none
complexity: low
complexity_reason: changes the number of questions one review lists, and the mixer already takes that number
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 2 hours
blocked_by: []
blocks: []
```

## Description

Brian asked on 2026-11-11 how to build the review after a sitting showed 3 of 157, and then approved the plan in `review-sitting-plan.md`.

- One review is about 20 minutes. That is the block size in the one study on the books. It is not a finding that 20 minutes beats 10 or 30, and it replaces the 5-minute draft written the same day.
- The list is counted as if each answer takes at least 30 seconds. A faster median does not add questions. One sitting lists at most 40 study questions.
- Cards that do not fit stay due. Opening Review again builds another sitting from what is still due.
- A sitting already in memory that is longer than a full sitting plus its re-queues is dropped on the next load, so a 157-question queue ends without waiting for a new day.
- Today says about 20 minutes. The learning record is not deleted.

**What Brian will see differently:** Review opens as one sitting whose counter stays far below 157, and Today says about 20 minutes.

**Removes:** the 5-minute sitting and its 10-question ceiling.

## Acceptance criteria

- [x] A learner with 200 due cards and a 2-second median gets one sitting of about 20 minutes, and the question count stays under 100.
- [x] A learner with 3 due cards gets those 3 cards.
- [x] With no answer history, the planned length is 20 minutes.
- [x] Today says about 20 minutes.

## Verification Strategy

- **Claim:** one review is about 20 minutes, and a fast tap cannot make the list into the hundreds.
- **Check + executor:** mechanized. `go test` on `./internal/session/` and `TestDailyLine` in `./internal/web/`, in the local Go container.
- **Pass condition:** `TestFastPaceStaysUnderTheCeiling`, `TestShortDueListStaysShort`, `TestDefaultPaceFillsTheBudget`, and `TestDailyLine` pass.
