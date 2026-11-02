# SPEC-004 · service: Only due cards; a due card leaves; strong words move up

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-027
title: "Only due cards; a due card leaves; strong words move up"
domain: service
status: pending
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-002, SPEC-003]
blocks: [SPEC-005, SPEC-006]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- `assemble` (`internal/session/mix.go:539`) fills the known quota with known cards that are **not due** (`laterK`, line 546) when due known cards run short. At 78% accuracy the known quota is 19 of 20 (`PlanFor`, line 145). This is the "it keeps asking what I know" effect.
- `classify` (line 508) skips a **due** reviewed card that is not weak and not "known". Examples: read with latest Good on a word outside `KnownLexemeIDs`, or a card with `RecentLapse` but latest Good. Such a card stays due and is never asked.
- `annotate` (line 244) asks every visit through the word's weakest skill (`activity.Choose`, `internal/activity/plan.go:180`). The answer grades the card of the skill asked (`internal/db/evidence.go:108`), so the due card the mix chose stays due and comes back next session.
- `Session.Answered(cardID, correct)` (`session.go:590`) closes a checked visit and requeues on a miss. It has no context and writes nothing.
- `db.Store.ApplyReview` (`internal/db/learning.go:208`) writes a schedule update and one review_log row in a transaction. SPEC-002 adds `ReviewLog.Implied` and excludes implied rows from accuracy.
- `Result.Rating` (`internal/activity/types.go:199`) is the grade the engine applied.
- ADR-0023 D3 and D4 (SPEC-001).

**Scope (in):**
- **Only due cards.**
  - `assemble` drops the `laterK` padding. The split between due struggling and due known cards still follows `PlanFor`, and room left under `SessionCap` is still filled from the other due side.
  - A known card that is not due is never queued.
  - Remove `laterK` from `splitPools`, and rewrite the comments that describe the padding.
- **No due card is skipped.** In `classify`, a reviewed card that is due is kindStruggle when it has `StruggleLapses` lapses, a latest Again or Hard, `Confused`, or `RecentLapse`. Otherwise it is kindKnown. A reviewed card that is not due is kindSkip. New-card rules are unchanged.
- **Asked through its own skill, or moved up.**
  - In `annotate`, a study visit is asked through the card's own skill, using a new `activity.ChooseSkill(skill string, gate Gate, f Facts, visits map[string]int) Choice`. It returns the skill's everyday activity, or the match activity on every `MatchEvery`-th visit, as `Choose` does. It returns a Flashcard through the skill when no checked activity fits or the gate is closed.
  - The visit **moves up** to the implying skill (read is implied by recall, listen by spell; `activity.Implies(skill) (string, bool)`) only when all of these hold:
    1. the card is not new;
    2. the word's view of the card's own skill is `Known` or `Solid`;
    3. the gate is open for the implying skill and a checked activity fits it;
    4. the word's card for the implying skill, if it exists, has a latest rating that is not Again or Hard and no `RecentLapse`;
    5. that implying card is not itself queued in this session.
  - The candidate carries what rule 4 needs. `build` fills it from the cards it already loaded, with no new query.
  - `CardRef` gains `MovedUp bool` and `OwnActivity activity.Kind`, the activity through the card's own skill.
  - `activity.Choose` and `WeakestSkills` stay for the lesson brief and other callers. Only the review mix stops using `Choose`.
- **Implied credit.**
  - Change `Answered` to `Answered(ctx context.Context, cardID int64, res activity.Result) (AnswerOutcome, error)`. `res.Correct` replaces the old `correct`.
  - When the served visit is `MovedUp`, `res.Correct` is true, and `res.Rating` is Good or Easy, apply Good to the visit's own card through a new `evidence.Recorder.Implied(ctx, cardID int64, rating fsrs.Rating) error`:
    - rating must be Good;
    - it schedules with `ScheduleNext` at the recorder clock and writes through `ApplyReview` with `Implied = true`;
    - it writes no attempts row and no exposure;
    - it checks the card belongs to the session's learner.
  - `AnswerOutcome{Implied bool, Slow bool}` reports it. `Slow` is `res.Correct && res.Rating == Hard && !res.NearMiss`.
  - A wrong answer on a moved-up visit writes nothing for the own card. Its one requeue is a copy with `MovedUp = false`, `AskSkill` set to the card's skill, and `Activity = OwnActivity`.
  - The single call site, `internal/web/activity.go:162`, passes `r.Context()` and `res`, and ignores the outcome. SPEC-006 uses it. This is the only `internal/web` edit in this spec.
- **Session tallies.** `Session.Tally() Tally{Answered, Right, MovedUp, PairsCleared, Slow, NewMet int}`, under the session mutex.
  - Updated by `Record`, `Answered`, and `DrillAnswered`.
  - `NewMet` counts served visits whose `CardRef.New` is true.
  - SPEC-006 renders it.
- Tests, rewriting the mix tests that pin the padding, and listing each rewritten test in the completion summary:
  - 5 due known plus 40 not-due known cards queue the 5 and none of the 40;
  - a due read card with latest Good outside `KnownLexemeIDs` is queued;
  - move-up happens when every condition holds, and each condition, broken alone, keeps the own skill;
  - a right moved-up answer leaves both the implying card and the own card with `due` in the future, and the own card's log row has `implied = 1` and no attempts row;
  - a wrong moved-up answer leaves the own card untouched, and its requeue asks the own skill;
  - Hard on a moved-up answer gives no implied credit;
  - the tallies;
  - learner isolation for `Implied`.

**Scope (out):** New-card cap and supply (SPEC-005). Screens (SPEC-006). Changing FSRS, `evidence/grade.go`, or the lesson brief's use of `WeakestSkills`.

## Acceptance Criteria

- [ ] ac-1: A build with 5 due known cards and 40 known cards due tomorrow queues exactly those 5 study cards (plus new cards within the allowance and drills).
- [ ] ac-2: A due reviewed card is never dropped by `classify`. One with `RecentLapse` or `Confused` is struggling; one that is strong is known.
- [ ] ac-3: A due read card of a word whose read view is Known, whose recall card is reviewed Good with no recent lapse and not queued, is served as a recall activity with `MovedUp = true`.
- [ ] ac-4: Breaking any single move-up condition serves the read card through a read activity.
- [ ] ac-5: A right moved-up answer graded Good or Easy schedules the read card Good with a review_log row marked `implied = 1` and no attempts row for read. The recall card is graded by the engine as before.
- [ ] ac-6: A wrong, or Hard, moved-up answer writes nothing for the read card. A wrong one requeues a visit that asks read.
- [ ] ac-7: `Recorder.Implied` refuses a rating other than Good and a card of another learner, and writes nothing.
- [ ] ac-8: `Tally` counts answered, right, moved up, pairs cleared, slow, and new met for a scripted sitting.

## Deliverables

- [ ] `internal/session/mix.go`, `internal/session/session.go`, and tests (`mix_test.go`, `session_test.go`, new `moveup_test.go`)
- [ ] `internal/activity/plan.go` (`ChooseSkill`, `Implies`) and test
- [ ] `internal/evidence/recorder.go` (`Implied`) and test
- [ ] `internal/web/activity.go` (call-site signature only)

## Verification Strategy

- **Claim:** A session holds only due cards. Each due card is answered through its own skill, or through the harder skill when it is strong there, and a right harder answer moves both cards on.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 ./internal/session ./internal/activity ./internal/evidence` and `/tmp/go/bin/go build ./...`.
- **Pass condition:** ac-1 to ac-8 pass. Putting the `laterK` padding back fails ac-1. Removing the implied write fails ac-5.

## Integration Points

- **Provenance tag:** `INIT-027/SPEC-004`
- SPEC-005 changes the new-card allowance in the same `mixCards`.
- SPEC-006 shows `Tally` on the done screen.
