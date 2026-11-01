# SPEC-001 Completion Summary

**Spec:** SPEC-001 — ADR-0022: each learner's day, and outside teaching that schedules but never grades
**Initiative:** INIT-026-tutor-data-loop
**Domain:** docs
**Status:** complete
**Provenance:** INIT-026/SPEC-001
**Worktree:** `/tmp/hebrew-fix/.claude/worktrees/tutor-data-loop` (branch `tutor-data-loop`)

## Change Produced

| File | Action |
| --- | --- |
| `docs/decisions/ADR-0022-learner-day-and-outside-teaching.md` | created (INIT-026/SPEC-001) |
| `docs/north-star.md` | §4.2 outside-teaching paragraph; §8 tool rows (INIT-026/SPEC-001) |

## What shipped

- ADR-0022 is Accepted, extends ADR-0018, and records D1 learner day, D2 outside teaching, D3 what the app does not store, and D4 learner coach notes.
- North Star §4.2 states that outside teaching schedules new cards and never grades or moves an existing card.
- North Star §8 lists `set_time_zone`, `get_study_digest`, `record_taught_words`, `remove_taught_words`, and `hebrew://tutor`. `add_coach_note` names the learner.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=light PASS · domain=docs PASS · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

Independent verifier [Verify SPEC-001](eb076ba2-4c9a-4d6c-b8c0-c589fce83c12) returned PASS on ac-1 through ac-4. The orchestrator then corrected one sentence the verifier flagged as a note, not an AC failure: the `record_taught_words` row no longer says that tool stores dated corrections. Dated corrections stay on `record_mistakes`.

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

- No git commit. The orchestrator owns the commit, and this batch is not committed yet.
- The worker did not write `sdd/`. This summary was written by the orchestrator.
