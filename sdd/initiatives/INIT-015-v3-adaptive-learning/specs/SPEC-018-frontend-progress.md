# SPEC-018 · frontend: Progress per skill

## Metadata

```yaml
spec_id: SPEC-018
initiative_id: INIT-015
title: "Progress per skill"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-014, SPEC-017]
blocks: [SPEC-024]
```

## Description

**Context:** North-star section 1 asks six questions:

- What do I know?
- What have I seen?
- What do I struggle with?
- Am I improving?
- What did I lose?
- Why?

SPEC-013 answers them now. SPEC-014 answers them over time.

**Scope (in):** `internal/web/views/progress.templ`, its handler, and a Figma frame "App / Phone · Progress" plus a desktop frame, drawn first.

- **Skill tabs:** Listen, Read, Recall, Spell. Each tab shows:
  - a stacked bar of states (unseen to solid);
  - estimated words known over 90 days, as a line chart;
  - retention against the 85–90% target band.
- **This week:** words gained, words lost (regressions with cause), and struggle words that left `struggling`.
- **Placement history:** each run's levels.
- **Weekly report:** the latest `weekly_reports` row as a card at the top, when one exists. SPEC-023 writes the rows.
- **Charts** are server-rendered inline SVG, built from tokens, with a data table fallback (`<details>`) for screen readers.
- **Empty states** use SPEC-010 illustrations ("Practice a few days and your chart starts here").

**Scope (out):** Writing the weekly report (SPEC-023).

## Understood (agent's read of the intent)

- [ ] und-1: The learner and the AI see the same numbers. This page reads the same functions the MCP tools call.

## Acceptance Criteria

- [x] ac-1: With a 90-day fixture, each tab renders its three charts, and the numbers match `learner.Summary` and the snapshots. — covered: no
- [x] ac-2: Every chart has a table fallback with the same values. — covered: no
- [x] ac-3: Chart colors come only from tokens, and green and red are not used for the state bars. — covered: no
- [x] ac-4: With no snapshots, the page shows the empty state and no broken chart. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | State colors | A single-hue ramp from `surfaceSoft` to `slate`, with `lamp` for solid. Green and red stay reserved for feedback. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `progress.templ`, its handler, and `internal/web/chart.go` (SVG builder)
- [ ] Figma Progress frames on page `139:2`
- [ ] Tests: render tests for ac-1 to ac-4

## Technical Requirements (domain-specific)

- [ ] No chart library. SVG paths are built in Go.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/web/...`.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: learner data display.
- **Security acceptance criteria:**
  - [ ] The page shows only the signed-in learner's data.
- **Review depth:** quick · **focus areas:** scope.

## Verification Strategy

- **Claim:** Progress answers the six questions per skill, with accessible charts.
- **Check + executor:** mechanized — render tests against fixtures.
- **Pass condition:** ac-1 to ac-4 pass.

## Integration Points

- SPEC-023 fills the weekly report card's data.
- **Provenance tag:** `INIT-015/SPEC-018`
