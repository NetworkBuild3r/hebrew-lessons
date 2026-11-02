# SPEC-002 Completion Summary

**Spec:** SPEC-002 — Migration 021: drill answers, cleared pairs, implied reviews
**Initiative:** INIT-027-adaptive-review
**Domain:** data
**Status:** complete
**Provenance:** INIT-027/SPEC-002
**Worktree:** `/tmp/hebrew-fix/.claude/worktrees/adaptive-review` (branch `adaptive-review`)

## Change Produced

| File | Action |
| --- | --- |
| `migrations/021_drill_answers_and_implied.sql` | created |
| `internal/db/drills.go`, `drills_test.go`, `migrate_021_test.go` | created |
| `internal/db/learning.go`, `evidence.go`, `placement.go` | `ReviewLog.Implied` written and read |
| `internal/db/coach.go`, `learner.go`, `struggle.go`, `internal/mcpstore/digest.go` | accuracy and retention counts skip implied rows; confusion counts skip misses at or before the latest clear |
| migration version lists in `internal/db` and `cmd/hebrew/ops_test.go` | 20 → 21 |

## What shipped

- `drill_answers` and `confusion_clears` store a learner's drill picks and the moment a pair was cleared.
- `review_log.implied` marks a review that came from another skill. Accuracy and retention counts skip those rows.
- A confusion pair counts only misses after its latest clear.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=light PASS · domain=data PASS · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

Independent verifier [Verify SPEC-002](67735311-8a7e-4e90-992d-3c93554df542) returned PASS on ac-1 through ac-7. Implementer was [SPEC-002 migration 021](5e0bf7a0-be97-41f1-b9ea-5571d6577284). The implementer also ran the mutation checks: dropping `implied = 0` from `RecentReviewRatings` failed the accuracy test, and dropping the clear filter failed the confusion-count test. Both passed again after restore.

## Security Review

Light, data only. No new trust boundary. Learner id is a parameter of store calls the session will pass from the signed-in learner. Critical 0 / High 0 / Medium 0 / Low 0.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, because `.claude/agents` is absent | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |
| AC verify | subagent_dispatch | T2 | fresh generalPurpose, read-only prompt | grok-4.7-high | verifier did not edit files |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`. No `dispatch` or `spec_lifecycle` event was written.

## Deviations

- Card-state lookups still see implied rows: latest rating, recent lapse, the known-lexeme check, `CountReviewLogs`, and the taught-card existence count. Those are not accuracy counts. An implied Good is a real schedule change, and a card with one must not be deleted as never reviewed.
- No git commit in this step. The orchestrator commits with the rest of the initiative.
