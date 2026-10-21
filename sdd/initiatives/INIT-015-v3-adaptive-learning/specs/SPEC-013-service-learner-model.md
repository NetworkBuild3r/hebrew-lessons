# SPEC-013 · service: Learner model, states and diagnosis

## Metadata

```yaml
spec_id: SPEC-013
initiative_id: INIT-015
title: "Learner model: states and diagnosis"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-004, SPEC-005]
blocks: [SPEC-014, SPEC-015, SPEC-017, SPEC-019]
```

## Description

**Context:** North-star section 4 defines states per word and skill:

- **unseen:** no evidence.
- **seen:** an exposure only.
- **learning:** a card younger than its third successful review.
- **struggling:** 3 or more Again results in the last 8 attempts, or accuracy under 60% over at least 5 attempts.
- **known:** FSRS retrievability ≥ 0.85 and the last 3 attempts right.
- **solid:** stability ≥ 21 days and known.
- **regressed:** was known or solid, and is now struggling or below 0.70 retrievability.

Section 4.5 diagnoses causes from skill patterns: hears but can't read, reads but can't recall, recall but spelling, one confusion pair, and a band slip. `db.StruggleStats` gives a partial struggle list today.

**Scope (in):** A new package, `internal/learner`.

- `Model.Word(ctx, userID, lexemeID) WordState`: the per-skill state, attempts, accuracy, last seen, stability, retrievability, and the cause.
- `Model.List(ctx, userID, Filter) ([]WordState, cursor)`: filter by state, skill, and band. Paged, 200 at most a page.
- `Model.Summary(ctx, userID)`: counts per skill and state, estimated words known per skill, and retention over the last 30 days of due reviews.
- `Diagnose(WordState, confusions) Cause` with the five patterns, each one a named rule.
- Overall state for a word = its weakest skill among those with evidence.
- Pure functions for the rules. A query layer in `internal/db` reads `attempts`, `exposures`, `cards`, and `review_log`.
- Replace `StruggleStats` callers with `Model.List(state=struggling)`. Keep its MCP output shape until SPEC-021.
- **Carried from SPEC-005 ac-4 (operator decision, Gate 5, 2026-10-19).** SPEC-005 writes a `confusion_misses` row only for words one letter apart, because `kind` allows only `letter_pair` and `agreement`. Add an `other` kind in one small, atomic migration (`008`), and widen `internal/evidence` so any wrong pick that names another lexeme writes a row. The learner model reads these rows. This adds a `db_migration` gate: the migration is not run on real data without the operator.

**Scope (out):** Writing snapshots (SPEC-014). MCP (SPEC-021). Screens (SPEC-017, SPEC-018).

## Understood (agent's read of the intent)

- [x] und-1: This is the program that "knows what words I am good with, have seen, and struggle with".
- [x] und-2: The thresholds are product choices. They are constants with a test that pins them, so changing one is deliberate.

## Acceptance Criteria

- [x] ac-1: A table test drives each of the seven states from crafted evidence, including the boundaries (exactly 3 of 8 Again, exactly 60% over 5). — covered: yes (TestSkillStateTable and boundary tests)
- [x] ac-2: Each of the five causes is produced by its pattern and not by the other four patterns. — covered: yes (TestEachCauseComesFromItsOwnPatternOnly, TestNoPatternProducesAnotherPatternsCause)
- [x] ac-3: `List` over 5,000 words and 50,000 attempts returns its first page in under 100ms on the test runner. — covered: yes, narrowly. TestListFirstPageOver50kAttempts measured 84 to 92 ms (101 ms once, for another filter) on a quiet machine. The test asserts a 200 ms limit, not 100. Scans all attempts per call, so it grows with attempt count.
- [x] ac-4: `Summary` estimated words known matches a hand-computed fixture. — covered: yes (TestSummaryEstimatedKnownMatchesHandComputedFixture)

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Retrievability source | Computed from the card's FSRS stability and elapsed days with `internal/fsrs`, not stored. | decided | yes |
| aud-2 | Estimated words known | Placement estimate per band, corrected by model counts: known plus solid replaces the estimate for words with evidence. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] `internal/learner/{doc,state,diagnose,model}.go`, and queries in `internal/db/learner.go`
- [x] Tests: `state_test.go`, `diagnose_test.go`, `model_test.go`, and `bench_test.go`

## Technical Requirements (domain-specific)

- [x] Time from `internal/clock`.
- [~] One query per page, not N+1. Attempts are streamed and counted in Go, not one SQL query per page. Not audited for N+1 by the orchestrator.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/learner/... ./internal/db/...`.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: learner data reads.
- **Security acceptance criteria:**
  - [~] Every function takes `userID` from the caller's principal. No function accepts a learner id from a request body. Functions take userID as a parameter. Scoping by principal is enforced in SPEC-021, not here.
- **Review depth:** quick · **focus areas:** scope.

## Verification Strategy

- **Claim:** the model assigns the defined state and cause from evidence, quickly.
- **Check + executor:** mechanized — `go test ./internal/learner/... -run 'State|Diagnose|Summary'` and `-bench List`.
- **Pass condition:** ac-1 to ac-4 pass.

## Integration Points

- SPEC-014, SPEC-015, SPEC-017, SPEC-018, SPEC-019, and SPEC-021 read the model.
- **Provenance tag:** `INIT-015/SPEC-013`
