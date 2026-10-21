# SPEC-019 · service: `LessonBrief`

## Metadata

```yaml
spec_id: SPEC-019
initiative_id: INIT-015
title: "LessonBrief"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-013]
blocks: [SPEC-020, SPEC-021]
```

## Description

**Context:** North-star section 7.1 defines the word plan for the next lesson. Existing unit budgets live in `internal/units/rules.go` and `internal/sessionlimits`.

**Scope (in):** A new package, `internal/brief`.

- `Build(ctx, userID, theme string) (Brief, error)` returns:
  - **New words:** 5 to 8, from the lowest band that still has `unseen` words. They fit the theme when a theme is given, using lexicon tags. They are never `struggling`. Existing budgets apply.
  - **Struggle words:** the top 3 to 5 by the model's struggle score. Each has its failing skill and a required count of 3 or more.
  - **Comfortable words:** up to 150 `known` or `solid` words, preferring recent and themed ones.
  - **Skill focus:** the weakest skill overall.
  - **Activities:** the SPEC-015 types that train the skill focus and each struggle word's skill.
  - **Targets:** coverage ≥ 0.95, and each new word used at least 3 times.
- The brief is stored in `lesson_briefs` with a random id, and it expires after 7 days.
- `Get(ctx, userID, briefID)` refuses a brief that belongs to another learner.

**Scope (out):** Validation against the brief (SPEC-020). MCP exposure (SPEC-021).

## Understood (agent's read of the intent)

- [ ] und-1: This is how a static lesson becomes "a lesson that changes words to match my needs": new words, struggle words reused, and comfortable words carrying the story.

## Acceptance Criteria

- [x] ac-1: For a fixture learner, the brief has 5–8 new words from the lowest band with unseen words, and none of them is struggling. — covered: yes, see completion summary
- [x] ac-2: Each struggle word carries the skill the model diagnosed and a count of 3 or more. — covered: yes, see completion summary
- [x] ac-3: Two calls on the same day without new evidence return the same word sets. Selection is seeded by user and day. — covered: yes, see completion summary
- [x] ac-4: `Get` with another learner's brief id returns not found. — covered: yes, see completion summary

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Theme matching | Lexicon tags first, then gloss keywords. With no match, the brief ignores the theme and says so in a `notes` field. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] `internal/brief/{doc,build,store}.go`
- [x] Tests: `build_test.go`, `store_test.go`, `limits_test.go`

## Technical Requirements (domain-specific)

- [x] The brief is a versioned JSON shape (`"v": 1`), so validators and clients can check it.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/brief/...`.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: learner data.
- **Security acceptance criteria:**
  - [ ] Brief ids are 128-bit random values. Lookup is always scoped by user id.
- **Review depth:** quick · **focus areas:** scope.

## Verification Strategy

- **Claim:** the brief follows section 7.1 and is scoped to its learner.
- **Check + executor:** mechanized — `go test ./internal/brief/...`.
- **Pass condition:** ac-1 to ac-4 pass.

## Integration Points

- SPEC-020 validates units against a brief. SPEC-021 serves it as `get_lesson_brief`.
- **Provenance tag:** `INIT-015/SPEC-019`
