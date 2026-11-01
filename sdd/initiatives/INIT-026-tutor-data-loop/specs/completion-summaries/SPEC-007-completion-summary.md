# SPEC-007 Completion Summary

**Spec:** SPEC-007 — `set_time_zone`, zone and weekly payload in `get_progress`, confusion pair in `get_learner_model`
**Initiative:** INIT-026-tutor-data-loop
**Domain:** api
**Status:** complete
**Provenance:** INIT-026/SPEC-007

## Change Produced

`set_time_zone` is on the learner, learner-write, and own-id allowlists. `get_progress` returns `zone` and `zoneSource`, and its default end day is the learner's today. Weekly reports include the decoded payload. `get_learner_model` adds `confusedWith` from `DominantPair`. The resolver is built inside the MCP server from the store and `studyday.AppZone`. `internal/app` was not edited.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=required FAIL · domain=api FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

The orchestrator re-ran:

```text
ok github.com/NetworkBuild3r/hebrew-lessons/internal/mcp 2.912s
ok github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore 0.004s
```

| AC | Test |
| --- | --- |
| ac-1 | `TestSetTimeZoneStoresAndShowsInProgress` |
| ac-2 | `TestSetTimeZoneSecurity` |
| ac-3 | `TestProgressRange` |
| ac-4 | `TestProgressOverSQLite` |
| ac-5 | `TestLearnerModelConfusedWithOverSQLite` |
| ac-6 | `TestToolsResourcesAndIdempotency`, `TestResourcesAuthoringNamesEveryTool` |

The tool description no longer tells the caller the days are UTC.

## Security Review

Required. A learner naming another id is refused. The read-only token cannot call `set_time_zone`. Critical 0 / High 0 / Medium 0 / Low 0.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, no `.claude/agents` | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`.

## Deviations

- `DominantPair` returns the key and count. The kind is taken from the same confusion rows after that call.
- Payload JSON is read in `mcpstore`. `WeeklyReportRows` in `internal/db` was left unchanged.
- No git commit yet.
