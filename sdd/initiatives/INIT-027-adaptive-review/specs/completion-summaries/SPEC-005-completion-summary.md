# SPEC-005 Completion Summary

**Spec:** SPEC-005 — New words follow accuracy and come from the frequency list
**Initiative:** INIT-027-adaptive-review
**Domain:** service
**Status:** complete
**Provenance:** INIT-027/SPEC-005
**Worktree:** `/tmp/hebrew-fix/.claude/worktrees/adaptive-review` (branch `adaptive-review`)

## Change Produced

| File | Action |
| --- | --- |
| `internal/sessionlimits/limits.go` | accuracy tiers replace the 0.25 share |
| `internal/session/mix.go` | `newAllowance(accuracy, available)` |
| `internal/session/supply.go` | seeds placement-list words before the card load |
| `internal/evidence/introduce.go` | creates new cards, never grades |
| `internal/db/cards_new.go`, `taught.go` | shared `INSERT OR IGNORE` helper |
| `internal/placement/items.go` | `Deck()` |
| `internal/mcp/schema.go`, `coach.go`, `tools.go` | mix target reports the tiers |

## What shipped

- A sitting introduces 2 new words below 80%, 5 from 80% to 90%, and 8 above 90%.
- At or above the 85% target, when fewer new cards are waiting than that cap, review adds the next verified placement-list words, lowest band first, as read and recall cards (and listen when a clip exists and the gate is open).

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=light PASS · domain=service PASS · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

Independent verifier [Verify SPEC-005](aeaf9889-f316-459d-934c-a5da1d3fc0bc) returned PASS on ac-1 through ac-6, and confirmed drills, due-only mix, implied credit, and the confusable filter were still present. Implementer was [SPEC-005 new words by accuracy](f1447e0a-e51c-40cd-8f55-1d181e4eae74).

## Security Review

Light. Seeding uses the session's learner id, `INSERT OR IGNORE`, and never changes an existing card. No new HTTP surface. Critical 0 / High 0 / Medium 0 / Low 0.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, because `.claude/agents` is absent | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |
| AC verify | subagent_dispatch | T2 | fresh generalPurpose, read-only prompt | grok-4.7-high | verifier did not edit files |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`. No `dispatch` or `spec_lifecycle` event was written.

## Deviations

- The first implementation report named files that were not in this worktree. They had been written under `/tmp/hebrew-lessons/.claude/worktrees/adaptive-review`. The orchestrator copied them here and re-ran the seeding tests before verification. The verifier read this worktree only.
- No git commit in this step. The orchestrator commits with the rest of the initiative.
