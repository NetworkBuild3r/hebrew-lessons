# SPEC-007 · service: Placement engine v3

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-015
title: "Placement engine v3"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-003, SPEC-005, SPEC-006]
blocks: [SPEC-012, SPEC-017]
```

## Description

**Context:** `internal/placement` runs a self-graded pass: `BandMax = 20`, `StopMinRated = 10`, `StopAgainRate = 0.60`, `SittingSize = 20`, `CardCap = 100`. Every answer becomes a first review. North-star section 5.2 replaces that with a test. D1: no feedback during the test, with a review at the end. D2: only right answers become cards.

**Scope (in):** `internal/placement`.

- **Parts:**
  - Part 0 is letters, six items. Five or more right skips ahead. Fewer means `reading_ready = false`, and reading is not tested.
  - Part 1 is listening and Part 2 is reading. Each has four options and "I don't know".
  - Part 3 is typing, and optional. It asks only words that were right in Parts 1–2, and its answers are graded with `fsrs.GradeTyped`.
- **Adaptive rule per skill:**
  - Start in band 1 and deal six items.
  - 5–6 right: move up a band.
  - 3–4 right: deal six more in the same band, then decide on all twelve (8 or more right moves up).
  - 0–2 right: stop that skill.
- **No repeats.** A word asked in listening is never asked in reading in the same run.
- **Distractors** come from the same band and part of speech and never include the answer's own gloss.
- **Scoring per skill and band:** known share = max(0, (right − wrong ÷ 3) ÷ asked). "I don't know" is neither right nor wrong. Estimated words known = sum over bands of share × band width. Level = the highest band with a share of at least 0.5.
- **Writes:**
  - Every item writes an attempt through `evidence.Recorder` with `source = placement`, with no scheduling.
  - A right answer seeds the card for that skill as Good.
  - A wrong answer or "I don't know" writes an exposure with `context = placement_review`, and creates no card.
- **Result:** listening and reading levels and estimates, the start unit, and the misses list. Each miss carries lexeme, skill, what was picked, the right gloss, the pointed form, and the audio id.
- **Re-check:** `NewRecheck(learner, skill)` builds a 3-minute sitting of the band at the boundary only. `RecheckDue(ctx, userID)` reports a check-in as due from 28 days after the last placement or re-check. The window is soft up to 42 days. Today (SPEC-017) offers it.
- **Answer handling:** the answer endpoint returns no correctness. It returns only the next item, or the result once the run is done.

**Scope (out):** Screens (SPEC-012). The Today entry point for a re-check (SPEC-017).

## Understood (agent's read of the intent)

- [ ] und-1: The test measures. It must not teach mid-test, and it must not schedule failures.
- [ ] und-2: "I don't know" exists to replace guessing. It is never penalized.

## Acceptance Criteria

- [x] ac-1: A scripted learner who is right on bands 1–3 and at chance on band 4 places at level 3 for that skill, and the run stops band 4 within 12 items. — covered: yes, see completion summary
- [x] ac-2: Across 1,000 seeded runs, no lexeme is asked twice in one run. — covered: yes, see completion summary
- [x] ac-3: After a run, cards exist only for right answers, all rated Good, and every wrong or "I don't know" item has an exposure row. — covered: yes, see completion summary
- [x] ac-4: The answer handler's response body contains no field that reveals correctness before the run ends. — covered: yes, see completion summary
- [x] ac-5: Fewer than 5 right in Part 0 skips Part 2 and records `reading_ready = false`. — covered: yes, see completion summary
- [x] ac-6: The score for 6 asked, 4 right, 1 wrong, and 1 "I don't know" is (4 − 1⁄3) ÷ 6 ≈ 0.611. — covered: yes, see completion summary
- [x] ac-7: `RecheckDue` is false at day 27 and true at day 28 after a run. — covered: yes, see completion summary
## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Old v2 runs in `placement_runs` | Kept and readable. A new run uses the v3 engine. | decided | yes |
| aud-2 | Twelve-item decision threshold | 8 of 12 moves up, the same two-thirds bar as 4 of 6. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `internal/placement/{types,deck,service,score,recheck}.go`
- [ ] Tests: `score_test.go`, `service_test.go`, and `adaptive_sim_test.go` (seeded simulations)

## Technical Requirements (domain-specific)

- [ ] Randomness comes from an injected `rand.Source`, so tests are reproducible.
- [ ] Replace the v2 constants with the v3 ones. Keep the test that pins the tuning numbers.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/placement/...`.
- **Rollback plan:** revert the commit.

## Security

- **Sensitivity:** standard — triggers matched: learner data writes.
- **Security acceptance criteria:**
  - [ ] An answer is accepted only for the item the server dealt to that run. Item ids from the client are checked against the run.
- **Review depth:** quick · **focus areas:** answer tampering.

## Verification Strategy

- **Claim:** the engine adapts by band, never repeats a word, scores with guessing correction, and seeds only right answers.
- **Check + executor:** mechanized — `go test ./internal/placement/... -run 'Adaptive|Score|Seed|NoFeedback'`.
- **Pass condition:** ac-1 through ac-7 each map to a named test that passes.

## Integration Points

- SPEC-012 renders the items, the picked state, the results, and the misses review.
- SPEC-013 reads placement attempts as evidence.
- **Provenance tag:** `INIT-015/SPEC-007`
