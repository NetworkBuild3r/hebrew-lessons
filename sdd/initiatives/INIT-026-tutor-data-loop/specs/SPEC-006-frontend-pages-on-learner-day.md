# SPEC-006 · frontend: Progress, Lessons, and review compute the learner's day

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-026
title: "Progress, Lessons, and review compute the learner's day"
domain: frontend
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-003]
blocks: []
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- The Progress page takes today from UTC (`internal/web/progress.go:188`) and clamps the range with a UTC day (`:479`).
- The Lessons list's "Slipped back" window starts from a UTC day (`internal/web/shell.go:554`).
- Review picks the asked part with a Denver-only cutoff (`internal/web/studyday.go:18`, called at `internal/web/learn.go:589`).

**Scope (in):**
- These handlers resolve the signed-in learner's `studyday.Zone` once per request: Progress, the Lessons list, and review.
- `progress.go:188` and `:479` use `studyday.Today`.
- `shell.go:554` uses `studyday.Today` for the window start.
- Review uses `studyday.DueCutoff` with the learner's zone. Delete `internal/web/studyday.go` and its Denver constant. Move its test cases to call `studyday.DueCutoff` with Denver.
- No layout, wording, or template change.

**Scope (out):** Reading the browser's zone. Any new section on any page. The MCP side (SPEC-007).

## Acceptance Criteria

- [ ] ac-1: For a Denver learner at 2026-11-01T05:30:00Z, the Progress page's chart and "This week" end on 2026-10-31, and `TestThisWeekAgreesWithGetProgress` still passes.
- [ ] ac-2: For a learner with no zone, the page uses the app zone. With the app zone Chicago, at 2026-11-01T05:30:00Z, that is 2026-11-01 (00:30 CDT).
- [ ] ac-3: The Lessons list's slipped window for the Denver learner starts `slipShowDays - 1` days before 2026-10-31.
- [ ] ac-4: The review pair ask for a Denver learner is unchanged from today. `TestReviewPairAskFollowsTheStudyDay` and `TestReviewPairAskSurvivesRollover` pass without changing their expected values.
- [ ] ac-5: `rg -n 'now\(\)\.UTC\(\)' internal/web/progress.go internal/web/shell.go` finds nothing, and `internal/web/studyday.go` is gone.

## Deliverables

- [ ] `internal/web/progress.go`, `shell.go`, `learn.go`
- [ ] delete `internal/web/studyday.go`
- [ ] `internal/web/progress_test.go`, `shell_test.go`, `review_anki_test.go`

## Verification Strategy

- **Claim:** the pages and review compute the signed-in learner's day, and nothing else about them changes.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 -run 'TestProgress|TestThisWeek|TestLessonsList|TestReviewPair|TestStudyDay|TestFinishedSitting' ./internal/web`, then the `rg` in ac-5. HTTP-level handler tests with the fixed clock, the existing pattern in `progress_test.go`. Not the browser tests.
- **Pass condition:** ac-1 to ac-5 hold. No golden HTML other than the day changes.

## Integration Points

- **Provenance tag:** `INIT-026/SPEC-006`
- Optional. If the owner drops this spec, the pages keep their UTC day, and the Done-when line for the Progress page moves out of this initiative.
