# SPEC-006 Completion Summary

**Spec:** SPEC-006 — Letter pick drill, done-screen changes, and the study-day queue
**Initiative:** INIT-027-adaptive-review
**Domain:** frontend
**Status:** complete
**Provenance:** INIT-027/SPEC-006
**Worktree:** `/tmp/hebrew-fix/.claude/worktrees/adaptive-review` (branch `adaptive-review`)

## Change Produced

| File | Action |
| --- | --- |
| `internal/web/learn.go` | study-day rollover; letter pick view model |
| `internal/web/drill.go` | `POST /_hx/review/drill` |
| `internal/web/web.go`, `activity.go` | reveal and grade refuse a two-letter step; feedback lines |
| `internal/web/views/activity.templ`, `pages.templ` | pick frame and done-screen rows |

## What shipped

- A review sitting lasts until 4:00 AM in the learner's zone. An answer at 5:30 PM Denver time is graded.
- A two-letter drill is a timed pick. The front shows the example word with a blank, or the asked letter's clip when there is no example. It does not play another letter's clip beside the example.
- The done screen lists words moved up, letter pairs cleared, slow answers, and new words, and omits zero rows.

## AC Verification

**Tier:** full. Pre-flight: gate_actions [] PASS · security_review=light PASS · domain=frontend PASS · autonomy_level=auto_tests FAIL => light_tier_eligible=false.

Independent verifier [Verify SPEC-006](81571615-7595-4462-be79-d6fb2faf7551) passed ac-1, ac-2, and ac-4 through ac-8 on the first read. ac-3 failed because an example word with no clip fell back to a pair letter's audio. The orchestrator removed that fallback and added `TestLetterPickExampleDoesNotPlayALetterClip`. The same verifier then passed ac-3. Implementer was [SPEC-006 review screens](12e837e5-8e07-4f2f-8cef-b6d917ce94e4).

## Security Review

Light, handler only. The pick is checked against the served pair, latency is measured on the server, and the queue must belong to the signed-in learner. CSRF and Origin stay on `/_hx/`. Critical 0 / High 0 / Medium 0 / Low 0. The initiative review is separate.

## Harness Adaptation

| site | capability | tier | what | model | impact |
| --- | --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 | generalPurpose, because `.claude/agents` is absent | cursor-grok-4.5-high-fast | no persona tool-tier enforcement |
| AC verify | subagent_dispatch | T2 | fresh generalPurpose, read-only prompt | grok-4.7-high | verifier did not edit files |

## Trace Emission Degradation

Skipped. This repo has no `scripts/dev/lib/sdd_trace`. No `dispatch` or `spec_lifecycle` event was written.

## Deviations

- ac-3 needed one fix after the first verification. A pair letter's clip is not used when an example word is on the screen.
- No git commit in this step. The orchestrator commits with the rest of the initiative.
