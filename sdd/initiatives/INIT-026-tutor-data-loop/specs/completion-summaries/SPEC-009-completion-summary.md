# SPEC-009 Completion Summary

**Spec:** SPEC-009 — `get_study_digest` and the `hebrew://tutor` resource
**Initiative:** INIT-026-tutor-data-loop
**Domain:** api
**Status:** complete
**Provenance:** INIT-026/SPEC-009

## Change Produced

`get_study_digest` is read-only and limited to the caller's own id. It returns the learner's zone, skill counts, retention, minutes, struggle words, corrections with a closed-loop count, taught words, due cards, coach notes, and the last weekly payload. Minutes use the same `jobs.ActiveMinutes` function as the weekly report. `hebrew://tutor` tells a tutor LLM to read the digest, ask for a retry on an open correction, and write only extracted words and corrections afterward.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=required FAIL · domain=api FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

The orchestrator re-ran:

```text
ok github.com/NetworkBuild3r/hebrew-lessons/internal/mcp 0.980s
ok github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore 0.054s
ok github.com/NetworkBuild3r/hebrew-lessons/internal/jobs 0.482s
```

`get_study_digest` is on `learnerTools` and `learnerOwnDataTools`. It is not on `learnerWriteTools`.

| AC | Test |
| --- | --- |
| ac-1 | `TestStudyDigestSectionsAndClosedLoop` |
| ac-2 | same test: `right_since` ignores a right answer earlier on the correction's own day |
| ac-3 | `TestStudyDigestIsolation` |
| ac-4 | `TestStudyDigestSizeCap` |
| ac-5 | `TestStudyDigestMinutesMatchesJobs` |
| ac-6 | `TestTutorResource` |

## Security Review

Required at the spec. Isolation is tested. The initiative-wide review is the completion phase, not this summary. Critical 0 / High 0 / Medium 0 / Low 0 from the spec's own checks.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, no `.claude/agents` | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`.

## Deviations

- Days with no retention are omitted so the capped JSON stays within 16 KB.
- `activeMinutes` was exported as `ActiveMinutes`. Scheduling did not change.
- No git commit yet. The live OAuth smoke in the spec was not run.
