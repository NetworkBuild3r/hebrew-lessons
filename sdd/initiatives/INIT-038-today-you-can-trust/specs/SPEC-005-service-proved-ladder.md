# SPEC-005 · service: the "proved" ladder at rule version 5

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-038
title: "The \"proved\" ladder at rule version 5"
domain: service
status: complete
primary_prompt: none
complexity: high
complexity_reason: replaces the rule every count reads, keeps the old rule callable for the comparison, and moves the rule version
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-002]
blocks: [SPEC-010, SPEC-014]
```

## Description

- **The ladder.** In `internal/learner/state.go` (the constants at lines 56 to 74, `IsKnown` at 157, `StateOf` at 176) and `internal/learner/standing.go`, each word and skill is in one of four states, counted out of taught words (ADR-0033):
  - **proved:** right on 3 study days spanning at least 7 days, with the latest answer right, using SPEC-002's study-day span;
  - **recalled later:** right at least once on a study day after the taught day, but not proved;
  - **slipping:** proved or recalled before, with the latest answer wrong;
  - **not yet asked:** taught, with no checked answer since.

  There is no calendar decay. FSRS recall probability stays for scheduling only, and the internal scheduler states stay as they are.
- **The second copy of the rule.** `skillOldKnown` (`internal/db/learner.go:565`) calls the learner package's rule instead of restating it, so there is one rule.
- **Rule version 5** (`RuleVersion`, `internal/jobs/daily.go:40`). History is rebuilt by the existing `hebrew history rebuild` (`cmd/hebrew/history.go`) in SPEC-014.
- **The rule-4 function stays callable** for 14 days, so SPEC-010 can show the old number beside the new one. It is removed after that.
- **The AI's view.** The digest and `get_context` report the ladder states. `knownLexemes` reads "recalled or proved", so cold-check passes lift his level without counting as proved.

**What Brian will see differently:** nothing on its own; SPEC-010 shows it to him. The AI's view of him in the digest and in context changes with it.

**Removes:** the "known" thresholds (recall probability 0.85, the last three answers right, success on two days) and the second copy of the rule in `internal/db/learner.go`.

**Scope (out):** `TierFor` and `PaceFor` in `internal/brief`, which only lesson briefs read and which are deferred to Stage 2; the page (SPEC-010).

## Acceptance criteria

- [ ] A word right on 3 study days spanning 7 or more days, with the latest answer right, is proved however late its card is.
- [ ] Right on 3 study days spanning 6 days is not proved.
- [ ] One right answer after the taught day is "recalled later", never proved.
- [ ] A proved word whose latest answer is wrong is "slipping".
- [ ] A voided answer counts for nothing in any state.
- [ ] A card only placement created is not counted as taught.
- [ ] Daily snapshots written after the change are stamped rule version 5, and the rule-4 function still returns the old count.

## Verification Strategy

- **Claim:** every word and skill has exactly one ladder state, by the rule ADR-0033 writes down.
- **Check + executor:** mechanized. `go test ./internal/learner/... ./internal/db/... ./internal/jobs/...` with fixtures for each state and each boundary.
- **Pass condition:** every acceptance case passes, and no code outside the learner package restates the rule.
