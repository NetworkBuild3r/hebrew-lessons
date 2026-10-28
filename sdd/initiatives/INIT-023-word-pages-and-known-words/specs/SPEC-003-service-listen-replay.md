# SPEC-003 · service: Replay past listening answers into cards once

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-023
title: "Replay past listening answers into cards once"
domain: service
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: [backup production database before first run]
reversibility_class: data
security_review: none
estimated_effort: 2 hours
blocked_by: [SPEC-002]
blocks: [SPEC-010]
```

## Description

**Context:** The 29 words with lesson listen answers have no listen card. Making each a new card due now would put 29 reviews on Today at once and ignore the answers already given.

**Scope (in):**
- A replay that, for each learner and word with `listen` attempts and no `listen` card, runs the attempts oldest first through `fsrs` the way `Recorder.Attempt` would (Easy held until review, a wrong first answer makes no card). It writes the resulting card and its review log rows in one transaction per word.
- It skips a word that already has a `listen` card, so a second run changes nothing.
- It runs from the daily job (`internal/jobs/daily.go`), once, and records that it ran in `job_state` under the key `replay.listen.v1`.
- A dry-run count is logged before any write.

**Scope (out):** Replaying `read` or `recall`. Those already have cards.

## Acceptance Criteria

- [ ] ac-1: A fixture word with listen answers R, R, W, R and no card ends with the same card that four live `Attempt` calls at the same times produce.
- [ ] ac-2: A word whose answers are all wrong gets no card.
- [ ] ac-3: A second run writes no rows.
- [ ] ac-4: A word that already had a `listen` card is unchanged.
- [ ] ac-5: On a copy of the production database, אֲנִי ends with a `listen` card and its listen state is `learning` or better.

## Deliverables

- [ ] `internal/evidence/replay.go` and `replay_test.go`
- [ ] `internal/db/evidence.go` (read attempts by skill, write a replayed card)
- [ ] `internal/jobs/daily.go` and `daily_test.go` (the one-time run)

## Verification Strategy

- **Claim:** past listening answers count, and nothing else moves.
- **Check + executor:** mechanized — `go test ./internal/evidence ./internal/db`. Manual — run against a copy of the production database in the local container, then read `get_learner_model` with `skill=listen`.
- **Pass condition:** ac-1 to ac-4 tests pass, and ac-5 holds on the copy.

## Integration Points

- **Provenance tag:** `INIT-023/SPEC-003`
