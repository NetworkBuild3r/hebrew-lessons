# SPEC-004 Completion Summary

**Spec:** SPEC-004 — Only due cards; a due card leaves; strong words move up
**Initiative:** INIT-027-adaptive-review
**Domain:** service
**Status:** complete
**Provenance:** INIT-027/SPEC-004
**Worktree:** `/tmp/hebrew-fix/.claude/worktrees/adaptive-review` (branch `adaptive-review`)

## Change Produced

| File | Action |
| --- | --- |
| `internal/session/mix.go` | due cards only; own skill or move-up |
| `internal/session/session.go` | `Answered` applies implied Good; `Tally` |
| `internal/activity/plan.go` | `ChooseSkill`, `Implies` |
| `internal/evidence/recorder.go` | `Implied` |
| `internal/db/learning.go` | `RecentAgainOrHard` on the study-card query |
| `internal/web/activity.go` | call site passes the checked result |

## What shipped

- A session queues due cards and new cards. A known card that is not due waits.
- A due card is asked through its own skill. When that skill is known or solid and the harder card has had no Again or Hard in 7 days, the visit moves up. A right Good or Easy answer schedules the easier card as implied Good. A wrong answer comes back on the card's own skill.
- The sitting tally counts answers, rights, move-ups, cleared pairs, slow answers, and new words met.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=light PASS · domain=service PASS · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

Independent verifier [Verify SPEC-004](bc98532b-6b12-46b7-b7d8-ab6ebb7fedd0) returned PASS on ac-1 through ac-8. Implementer was [SPEC-004 due cards move up](38ef1a49-92a0-44c7-858b-3d336868bb12).

## Security Review

Light. `Implied` refuses a rating other than Good and a card that is not the session learner's. No new HTTP surface. Critical 0 / High 0 / Medium 0 / Low 0.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, because `.claude/agents` is absent | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |
| AC verify | subagent_dispatch | T2 | fresh generalPurpose, read-only prompt | grok-4.7-high | verifier did not edit files |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`. No `dispatch` or `spec_lifecycle` event was written.

## Deviations

- `Implied` takes the learner id as well as the card id, so another learner's card is refused. The session passes its own user id.
- `MovedUp` counts only a successful implied Good. A wrong or Hard answer on a moved-up visit does not.
- `PairsCleared` increments inside the clear write, so a pattern drill graded through `Record` counts.
- A failed implied write rolls the answer tally back, so a retry does not count the visit twice.
- No git commit in this step. The orchestrator commits with the rest of the initiative.
