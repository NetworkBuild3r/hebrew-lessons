# SPEC-005 · service: One attempt recorder and the grading rule

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-015
title: "One attempt recorder and the grading rule"
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
blocked_by: [SPEC-003]
blocks: [SPEC-007, SPEC-013, SPEC-015]
```

## Description

**Context:** Answers are graded in several places today. `session.(*Session).Record` takes a self-graded `fsrs.Rating`. `fsrs.GradeTyped` grades typed answers (exact is Good, one letter off is Hard). `fsrs.(*Review).RecordMiss` logs confusions. Lesson-player steps grade their own items. North-star section 4 sets one rule for checked answers: wrong is Again, right after a replay or a hint is Hard, right is Good, and right and faster than the learner's own median for that activity is Easy.

**Scope (in):** A new package, `internal/evidence`.

- `Recorder.Attempt(ctx, Attempt) (fsrs.Rating, error)` writes the `attempts` row, derives the rating with `Grade`, applies it to the card for (learner, lexeme, skill) through `fsrs.Review`, and records a confusion when the response names another lexeme. All of this happens in one transaction.
- `Grade(a Attempt, median time.Duration) fsrs.Rating` implements the rule. "I don't know" is Again.
- `Recorder.Expose(ctx, learnerID, lexemeIDs, context, unitID)` writes `exposures`.
- `Recorder.SelfGrade(ctx, cardID, rating)` keeps the flashcard path. It writes an attempt with `activity = flashcard`, `correct = rating > Again`.
- The median is per learner and activity, over the last 50 correct attempts. With fewer than 10 correct attempts, Easy is never given.
- Route the existing callers through it: `session.Record`, the typed grader's callers, and the lesson player's checked steps.

**Scope (out):** New activities (SPEC-015). Placement (SPEC-007 calls this package with `source = placement`).

## Understood (agent's read of the intent)

- [x] und-1: Every checked answer goes through one function, so the evidence can't drift between screens.
- [x] und-2: The latency rule is a product choice, tuned from the learner's own data (north-star section 4).

## Acceptance Criteria

- [x] ac-1: `Grade` returns Again for wrong or "I don't know", Hard for right with a hint, Good for right, and Easy for right and faster than the median once 10 or more correct attempts exist. — covered: yes (TestGrade, TestMedian, TestEasyNeedsTenCorrectAndABeatenMedian, TestAttemptRatings)
- [x] ac-2: A recorded attempt writes exactly one `attempts` row and one `review_log` row, or neither if any step fails. — covered: yes (TestAttemptWritesOneAttemptAndOneReview, TestAttemptIsAtomic)
- [x] ac-3: After the change, the v2 review session, typed answers, and lesson-player checks all write `attempts`, which an integration test confirms for each path. — covered: partial. Review and lesson paths covered (TestRecordWritesAnAttempt, TestLessonChecksWriteAttempts). The typed path has no caller in v2, so there is nothing to test.
- [x] ac-4 (narrowed, widening deferred to SPEC-013): A wrong pick that names another lexeme writes a `confusion_misses` row. — covered: partial. Written only when the two words differ by one letter (TestWrongPickNamingAnotherWordWritesAConfusion). Other pairs need a new confusion kind, so a migration. Operator decision 2026-10-19 (Gate 5): accept the narrower rule now. SPEC-013 adds an 'other' confusion kind in a small migration and widens the rule.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Placement attempts and FSRS | Placement passes `schedule: false`. SPEC-007 decides which cards to seed (D2). | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `internal/evidence/{doc,recorder,grade}.go`
- [ ] Caller changes in `internal/session`, `internal/web`, and the lesson player
- [ ] Tests: `internal/evidence/grade_test.go`, `recorder_test.go`, and one path test per caller

## Technical Requirements (domain-specific)

- [ ] Time comes from `internal/clock`, so tests are deterministic.
- [ ] No import cycle: `evidence` depends on `db` and `fsrs`, not on `session`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./...`.
- **Rollback plan:** revert the commit. The tables stay.

## Security

- **Sensitivity:** standard — triggers matched: learner data writes.
- **Security acceptance criteria:**
  - [ ] `response` is capped at 200 characters before it is stored.
  - [ ] The learner id comes from the session, never from form input.
- **Review depth:** quick · **focus areas:** input bounds, identity source.

## Verification Strategy

- **Claim:** one rule grades every checked answer, and every path records evidence.
- **Check + executor:** mechanized — `go test ./internal/evidence/... ./internal/session/... ./internal/web/...`.
- **Pass condition:** the table test for `Grade` passes, and each caller test finds its `attempts` row with the right skill, activity, and source.

## Integration Points

- SPEC-007, SPEC-013, and SPEC-015 use `evidence.Recorder`.
- **Provenance tag:** `INIT-015/SPEC-005`
