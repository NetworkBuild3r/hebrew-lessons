# SPEC-003 Completion Summary

**Spec:** SPEC-003 — Drills that are graded, timed, capped, and end
**Initiative:** INIT-027-adaptive-review
**Domain:** service
**Status:** complete
**Provenance:** INIT-027/SPEC-003
**Worktree:** `/tmp/hebrew-fix/.claude/worktrees/adaptive-review` (branch `adaptive-review`)

## Change Produced

| File | Action |
| --- | --- |
| `internal/letters/letters.go` | look-alike and sound-alike pairs, `Confusable` |
| `internal/activity/check.go` | typed slips log a letter pair only when `Confusable` |
| `internal/session/drill.go` | `DrillAnswered`, clear rule |
| `internal/session/mix.go`, `session.go` | 3 steps, cap 6; a drill grade is stored |
| `internal/web/views/alefbet_letters_test.go` | cue table stays in step with `LookAlike` |

## What shipped

- A letter or pattern drill answer is stored. Three right answers in a row, none slow once ten timed drill answers exist, clear the pair and drop its remaining steps.
- A session holds at most 3 steps of one pair and 6 drill steps.
- A typed slip logs a letter pair only for a look-alike or sound-alike pair. A wrong pick of a one-letter-off word still logs the pair.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=light PASS · domain=service PASS · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

Independent verifier [Verify SPEC-003](9d1c82bc-b89e-4064-b889-f6dc9e618188) returned PASS on ac-1 through ac-8. Implementer was [SPEC-003 drills that end](d1934729-836f-407d-8525-6e29e75267cb). The implementer's mutation checks failed the pattern-Good test when the drill grade was discarded, and failed the ו→מ test when the confusable filter was removed. Both passed after restore.

## Security Review

Light. Drill writes use the session's learner id. `DrillAnswered` refuses a card that was not just served. Critical 0 / High 0 / Medium 0 / Low 0.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, because `.claude/agents` is absent | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |
| AC verify | subagent_dispatch | T2 | fresh generalPurpose, read-only prompt | grok-4.7-high | verifier did not edit files |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`. No `dispatch` or `spec_lifecycle` event was written.

## Deviations

- The clear window starts one nanosecond after the clear. The answer that clears a pair shares that timestamp, and an inclusive query was counting it toward the next streak. The verifier confirmed the spec's "since the later of the last miss and the latest clear" still holds.
- No git commit in this step. The orchestrator commits with the rest of the initiative.
