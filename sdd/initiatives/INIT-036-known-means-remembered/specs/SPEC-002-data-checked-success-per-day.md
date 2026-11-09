# SPEC-002 · data: checked successes and the days they fall on

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-036
title: "Checked successes and the days they fall on"
domain: data
status: complete
primary_prompt: none
complexity: medium
complexity_reason: two shared queries with golden histories, and a consumer set that must move together
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003]
```

## Description

Give the learner model what it needs to ask "did a checked right answer land on two study days".

- `LearnerEvidence` in `internal/db/learner.go` stops counting self-reported and peeked rows as successes. Today the success count is `rl.rating > 1 AND (rl.implied = 0 OR c.skill = 'spell')` (`:319`); it gains a `peeked = 0` and not-say condition. The same filter applies to the newest-three outcomes.
- It also returns, per card, how many distinct study days carry a checked success. The day comes from `studyday.Today(at, zone)`, the same function `internal/evidence/later_day.go:75` uses, in the learner's stored zone.
- `knownLexemeUnion` (`internal/db/placement.go:614-632`) becomes the strict set: the word's skill is known by ADR-0031, not "latest read review Good or Easy". `knownTexts`, `suggestedStart` and the MCP `knownLexemes` follow it because they read that union.
- A count of words that pass every part of the old known rule but have only one checked-success day, for SPEC-006.

It adds no table. A migration is allowed only for an index, if the query plan needs one.

**Scope (out):** the state rule itself (SPEC-003), the say step (SPEC-004), screens.

## Acceptance criteria

- [x] Three same-day right answers give three successes and one success day; the same three across two study days give two.
- [x] A `peeked` review, and a say self-check, add no success, no success day and no newest-three right.
- [x] A typed-spelling implied credit still adds a success and a day (ADR-0030).
- [x] A day boundary follows the learner's zone and rollover hour: an answer at 02:00 local counts toward the previous study day.
- [x] `knownLexemeUnion` returns a word only when its skill is known under the strict rule. A single right read answer does not return it.
- [x] The "waiting for a second day" count equals the words that pass the old rule and fail only the day condition.
- [x] Another learner's rows never change a result.

## Verification Strategy

- **Claim:** the queries count checked successes and distinct study days exactly as ADR-0031 says.
- **Check + executor:** mechanized. Table-driven Go tests in `internal/db` on seeded `review_log` histories, run with `go test ./internal/db/...`.
- **Pass condition:** every acceptance case above has a seeded history whose expected counts are written in the test, and all pass.
