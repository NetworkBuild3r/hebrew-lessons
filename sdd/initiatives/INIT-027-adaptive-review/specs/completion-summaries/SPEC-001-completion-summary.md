# SPEC-001 Completion Summary

**Spec:** SPEC-001 — ADR-0023: review that moves on when you know it
**Initiative:** INIT-027-adaptive-review
**Domain:** docs
**Status:** complete
**Provenance:** INIT-027/SPEC-001
**Worktree:** `/tmp/hebrew-fix/.claude/worktrees/adaptive-review` (branch `adaptive-review`)

## Change Produced

| File | Action |
| --- | --- |
| `docs/decisions/ADR-0023-review-that-moves-on.md` | created (INIT-027/SPEC-001) |
| `docs/north-star.md` | §4.2 drill and implied rows; §4.5 letter-drill row; §6 letter pairs; session mix paragraph |

## What shipped

- ADR-0023 is Accepted, extends ADR-0020 and ADR-0022, and records D1 through D6 plus Consequences.
- North Star describes due-only sittings, drills that clear, move-up, and accuracy caps in present tense.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=light PASS · domain=docs PASS · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

Independent verifier [Verify SPEC-001](e582f0c4-e5d2-43ce-b26d-9698581165eb) returned PASS on ac-1 through ac-4. Implementer was [SPEC-001 ADR-0023 docs](8d2fa31c-68f4-4e01-9438-d3caa5f98af6).

## Security Review

Docs only. Critical 0 / High 0 / Medium 0 / Low 0.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, because `.claude/agents` is absent | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |
| AC verify | subagent_dispatch | T2 | fresh generalPurpose, read-only prompt | grok-4.7-high | verifier did not edit files |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`. No `dispatch` or `spec_lifecycle` event was written.

## Deviations

- After the PASS, two sentences were added to ADR-0023 so later specs cannot drift. D1 names the median window of the last 50 timed right drill answers. D2 says letter-pair keys store base letters only, so כּ/ק is stored as כ/ק and שׂ/ס as ס/ש. SPEC-003 and SPEC-004 were aligned to those sentences. ac-1 and ac-2 still hold.
- No ADR index exists under `docs/decisions/`.
- No git commit in this step. The orchestrator commits with the rest of the initiative.
