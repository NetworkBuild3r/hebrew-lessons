# SPEC-003 Completion Summary

**Spec:** SPEC-003 — One learner-day rule, used by the lesson brief
**Initiative:** INIT-026-tutor-data-loop
**Domain:** service
**Status:** complete
**Provenance:** INIT-026/SPEC-003

## Change Produced

| File | Action |
| --- | --- |
| `internal/studyday/studyday.go` | created: `ParseZone`, `AppZone`, `Resolver`, `Today`, `DueCutoff` |
| `internal/studyday/studyday_test.go` | created |
| `cmd/hebrew/jobs.go` | `jobsLocation` calls `studyday.AppZone` |
| `cmd/hebrew/ops_test.go` | migration count 19 became 20 so the package still builds on migration 020 |
| `internal/brief/build.go` | `WithResolver`; the brief day is `studyday.Today` |
| `internal/brief/build_test.go` | app-zone resolver; `TestBriefUsesLearnerCivilDay` |

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=light PASS · domain=service FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

The implementer reported an independent pass. The orchestrator then re-ran the verify commands:

```text
ok github.com/NetworkBuild3r/hebrew-lessons/internal/studyday 0.094s
ok github.com/NetworkBuild3r/hebrew-lessons/cmd/hebrew 2.203s
ok github.com/NetworkBuild3r/hebrew-lessons/internal/brief 0.314s
```

`Today` formats `now.In(z.Loc)`. `NewResolver` rejects a nil app zone. The brief errors when no resolver is set. `now.UTC()` in `Build` is the instant only; the day comes from the learner zone.

## Security Review

Light. Critical 0 / High 0 / Medium 0 / Low 0.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, no `.claude/agents` | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`.

## Deviations

- No git commit yet.
- `Daily` still takes one location. SPEC-004 switches the jobs onto the resolver.
