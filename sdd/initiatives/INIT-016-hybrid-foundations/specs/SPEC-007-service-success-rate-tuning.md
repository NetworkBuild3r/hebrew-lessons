# SPEC-007 · service: Success-rate tuning in the brief

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-016
title: "Success-rate tuning in the brief"
domain: service
status: planned
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: opus
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-001]
blocks: []
```

## Description

**Context:** The brief (`internal/brief`) mixes struggle, new and comfortable words with fixed limits. It does not look at how the learner has been doing. The hybrid rule: aim at roughly 8 in 10 right — lean on comfortable material when the learner is missing more, add new material when they are cruising, without swinging.

**Scope (in):**

- `RecentAccuracy`: correct ÷ checked over the learner's last 40 checked attempts (from `attempts`), with the count.
- A `Load` setting derived from it, with hysteresis: below 0.70 → `ease`; above 0.90 → `stretch`; otherwise `steady`. A change of `Load` needs the rate to cross the threshold by 0.03, and fewer than 15 attempts always means `steady`.
- `ease`: new words at the low end of the existing range (5, or 3 for a beginner), struggle words capped at 3, comfortable words at the high end. `stretch`: new words at the high end (8, or 4), comfortable at the low end. `steady`: today's behaviour.
- The brief payload records `load` and `recentAccuracy`; `Version` goes up by one; the validator accepts both versions.
- `get_lesson_brief`'s description explains `load` to authors.

**Scope (out):** Changing FSRS target retention. Changing activity types per load.

## Acceptance Criteria

- [ ] ac-1: A table test over attempt histories pins `Load` for each threshold, the 0.03 hysteresis, and the 15-attempt minimum. (REQ-007)
- [ ] ac-2: The same learner model with `ease` versus `stretch` produces briefs whose new-word counts differ as specified and stay within the existing limits. (REQ-007)
- [ ] ac-3: A brief of the previous version still validates; a new brief carries `v`, `load` and `recentAccuracy`.
- [ ] ac-4: Selection stays deterministic for the same day and evidence.

## Deliverables

- [ ] `internal/brief/load.go`, changes in `build.go`, `limits`, payload version
- [ ] `internal/mcp/tool_lesson_brief.go` description
- [ ] Tests

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none. **Rollback:** revert; old briefs keep validating.

## Security

- **Sensitivity:** standard — reads one learner's attempts, scoped by user id.

## Verification Strategy

- **Claim:** the brief responds to recent accuracy within bounds, without oscillating.
- **Check + executor:** mechanized — `go test ./internal/brief/... ./internal/mcp/...`.
- **Pass condition:** ac-1 to ac-4 tests pass.

## Integration Points

- INIT-019/SPEC-002 adds personal mistakes to the same brief.
- **Provenance tag:** `INIT-016/SPEC-007`
