# SPEC-006 Completion Summary

**Spec:** SPEC-006 — Progress, Lessons, and review compute the learner's day
**Initiative:** INIT-026-tutor-data-loop
**Domain:** frontend
**Status:** complete
**Provenance:** INIT-026/SPEC-006

## Change Produced

Progress, the slipped-word window, and the review pair ask use `studyday.Today` and `studyday.DueCutoff` for the signed-in learner. `web.Options.Zones` is optional. When it is nil, the server builds a resolver from the store and `studyday.AppZone(TZ)`. `internal/web/studyday.go` is deleted. No layout or template change. `internal/app/app.go` was not edited.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=light PASS · domain=frontend FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

The orchestrator re-ran:

```text
ok github.com/NetworkBuild3r/hebrew-lessons/internal/web 2.927s
```

`internal/web/studyday.go` is gone. `progress.go` and `shell.go` no longer call `now().UTC()` for the day. `shell.go` starts the slipped window from `studyday.Today`. Review calls `studyday.DueCutoff`.

## Security Review

Light. Critical 0 / High 0 / Medium 0 / Low 0. The zone comes from the signed-in learner's row.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, no `.claude/agents` | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`.

## Deviations

- No git commit yet.
- `sameStudyDay` in `learn.go` still compares UTC dates. It is not the Progress or Lessons day. Left as it was.
