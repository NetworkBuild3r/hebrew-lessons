# SPEC-008 Completion Summary

**Spec:** SPEC-008 — Tutor writes: taught words, dated corrections, your own coach notes
**Initiative:** INIT-026-tutor-data-loop
**Domain:** api
**Status:** complete
**Provenance:** INIT-026/SPEC-008

## Change Produced

`record_taught_words` and `remove_taught_words` call `evidence.Recorder`. `record_mistakes` takes an optional day and rejects a day after today or more than 30 days back. `add_coach_note` is learner-callable on the caller's own id, stored as `author_role=learner`, capped at 20 notes in 24 hours. The brief quotes those notes as "The learner wrote:". All four tools are on the learner, learner-write, and own-id allowlists.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=required FAIL · domain=api FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

The orchestrator re-ran:

```text
ok github.com/NetworkBuild3r/hebrew-lessons/internal/mcp 2.467s
ok github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore 0.004s
ok github.com/NetworkBuild3r/hebrew-lessons/internal/brief 0.392s
```

| AC | Test |
| --- | --- |
| ac-1 | `TestTaughtWordsCreatesCardsAndListsUnknown` |
| ac-2 | `TestTaughtAndCoachWriteSecurity` |
| ac-3 | `TestTaughtWordsRejectsBadBatchAndOn` |
| ac-4 | `TestTaughtWordsRemoveKeepsReviewed` |
| ac-5 | `TestMistakeOnDefaultsAndStores` |
| ac-6 | `TestCoachNoteLearnerRoleCapAndBrief`, `TestBriefQuotesLearnerCoachNote` |
| ac-7 | script body stored as text; brief JSON escapes it |
| ac-8 | `TestToolsResourcesAndIdempotency`, `TestResourcesAuthoringNamesEveryTool` |

## Security Review

Required. Another learner's id is refused. The read-only token cannot call the four writes. Critical 0 / High 0 / Medium 0 / Low 0.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, no `.claude/agents` | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`.

## Deviations

- The lesson-brief builder now constructs a study-day resolver inside mcpstore, because SPEC-003 requires one and `internal/app` was left unchanged.
- Taught tools reach the recorder through the course store, not a new app option.
- No page renders coach notes. The script-tag check is the stored text plus JSON escaping in the brief.
- No git commit yet.
