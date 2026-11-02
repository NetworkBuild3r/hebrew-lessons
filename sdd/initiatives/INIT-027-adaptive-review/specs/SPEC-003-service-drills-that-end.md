# SPEC-003 · service: Drills that are graded, timed, capped, and end

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-027
title: "Drills that are graded, timed, capped, and end"
domain: service
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-002]
blocks: [SPEC-004, SPEC-006]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- `drillCandidates` (`internal/session/session.go:288`) adds `DrillSteps` (4) steps for every pair with `MinMissesForDrill` (2) misses in `ConfusionWindow` (7 days), up to `DrillCap` (20) (`internal/session/mix.go:103-113`).
- The grade func drops every drill grade: `if cardID <= 0 { return nil }` (`session.go:262`). So a drill can never end. It only ages out of the 7-day window.
- Candidate `Kind` is `letter` or `pattern` (`drillKind`, `session.go:340`). The store kind is `fsrs.MissLetterPair` or `fsrs.MissAgreement`. Candidate `Key` is `pair.A + "/" + pair.B`, the `confusion_misses` key.
- A typed answer within `MaxPairDistance` logs each substituted letter as a pair (`internal/activity/check.go:193-198`, `LetterPairs` in `align.go:115`, key from `pairKey`, bare runes in code-point order). That turns typos like ו/מ into drills.
- The drill cue table is `alefbetPairs` in `internal/web/views/alefbet.go:107` (letters with dagesh variants and cue text).
- SPEC-002 adds `drill_answers`, `confusion_clears`, and the store calls `InsertDrillAnswer`, `DrillAnswersSince`, `CorrectDrillLatencies`, `InsertConfusionClear`, `LatestConfusionClear`, `LastConfusionMiss`. `RecentConfusionPairs` already counts only misses after the latest clear.
- ADR-0023 D1 and D2 (SPEC-001).

**Scope (in):**
- `internal/letters` (new, no imports from web or session):
  - `LookAlike` and `SoundAlike`, each a list of bare-letter pairs.
    - LookAlike is every pair of `alefbetPairs` with points removed, deduplicated, dropping pairs that become one letter (שׁ/שׂ), plus ו/י.
    - SoundAlike is the ADR-0023 D2 pairs as base-letter keys, because a letter-pair key stores only U+05D0 through U+05EA: א/ע, ט/ת, כ/ק (כּ/ק), ח/כ, ב/ו, ס/ש (שׂ/ס).
  - `Confusable(key string) bool`: true when the `a/b` key, in either order, is in either set. Malformed keys are false.
  - A test in `internal/web/views` checks every bare pair of `alefbetPairs` is in `letters.LookAlike`, so the cue table and the set cannot drift.
- `internal/activity/check.go` typed path: append a letter pair only when `letters.Confusable(key)`. The wrong-pick path (`confusedOf`) and the whole-word mix-up path are unchanged.
- `internal/session/mix.go` constants:
  - `DrillSteps` = 3 and `DrillCap` = 6.
  - New `DrillClearStreak` = 3, `DrillMedianMin` = 10, `DrillMedianWindow` = 50.
  - `DrillSlowFactor` = `evidence.SlowFactor`, or 2 with a comment naming it if importing causes a cycle.
  - Update the doc comments and the tests that pin 4 and 20.
- `internal/session`, drill answers:
  - `func (s *Session) DrillAnswered(ctx context.Context, cardID int64, correct bool, latency time.Duration) (DrillOutcome, error)`.
    - Valid only for the served visit, when it is a letter or pattern step. Otherwise it returns `ErrNotServed`, or a new `ErrNotDrill`.
    - It marks the serve answered before the write and puts the mark back on error, as `Record` does. A second call for the same serve is refused.
    - It writes one `drill_answers` row for the session's learner, with the store kind mapped back from the candidate kind and the candidate key. Latency is in milliseconds, and negative becomes 0.
    - A wrong answer requeues the step once, as `Answered` does.
  - Clear rule, evaluated after every right answer:
    - Take the drill answers for (learner, kind, key) since the later of `LastConfusionMiss` and `LatestConfusionClear`.
    - The trailing run of right answers counts only answers that are not slow. Slow means latency greater than `DrillSlowFactor` × the median of `CorrectDrillLatencies(learner, DrillMedianWindow)`, and the median applies only when at least `DrillMedianMin` latencies exist. A slow right answer breaks the run.
    - When the run reaches `DrillClearStreak`, write `InsertConfusionClear` at the service clock's now, and remove the pair's remaining steps from the queue after the current position.
    - `DrillOutcome{Cleared bool, Streak int}` reports it.
  - `Record` on a pattern step keeps the self-grade path. When the served visit is a drill step, it calls the same drill-answer code with `correct = rating >= Good` and latency 0, instead of returning nil. Again still requeues once.
  - The store calls go through the `*db.Store` the service holds. The learner id is the session's, never a parameter.
- Tests:
  - `internal/letters`: both sets, order, malformed keys.
  - `internal/activity`: a typed slip ו→מ logs no letter pair; a slip ב→כ logs ב/כ.
  - `internal/session`:
    - drill steps are capped at 3 per pair and 6 per session;
    - three right answers clear the pair and drop its remaining steps;
    - a wrong answer in between resets the run;
    - a slow right answer breaks the run once 10 latencies exist, and does not before that;
    - after a clear, a build with one new miss adds no drill, and with two new misses adds one;
    - pattern Good counts as right;
    - `DrillAnswered` on a study card is refused;
    - learner isolation.

**Scope (out):** The pick screen and server-side latency (SPEC-006). Changing how pick confusions are logged. Any MCP output.

## Acceptance Criteria

- [ ] ac-1: A queue built for three pairs that reached the threshold holds at most 3 steps per pair and at most 6 drill steps.
- [ ] ac-2: Three right drill answers for ב/כ, none slow, write one `confusion_clears` row and remove the pair's later steps from the live queue. The next build adds no ב/כ drill.
- [ ] ac-3: Right, wrong, right, right does not clear. A further right does.
- [ ] ac-4: With 10 timed right answers whose median is 1,000 ms, a right answer of 2,500 ms does not count toward the run. With 9 timed right answers it does.
- [ ] ac-5: A typed answer that differs from the expected word only by ו→מ logs no `confusion_misses` row. One that differs only by ב→כ logs `ב/כ`. A wrong pick of a one-letter-off word still logs its pair.
- [ ] ac-6: Every bare pair in the drill cue table is in `letters.LookAlike` (a test fails if a cue pair is added without it).
- [ ] ac-7: A pattern step graded Good writes a right drill answer; Again writes a wrong one and requeues once.
- [ ] ac-8: `DrillAnswered` on a study card, on a card that was not just served, or twice for one serve is refused and writes nothing.

## Deliverables

- [ ] `internal/letters/letters.go`, `internal/letters/letters_test.go`
- [ ] `internal/web/views/alefbet_letters_test.go` (the mirror test only)
- [ ] `internal/activity/check.go` and its test
- [ ] `internal/session/session.go`, `internal/session/mix.go`, `internal/session/drill.go` (new), and tests

## Verification Strategy

- **Claim:** A drill is graded, ends after three quick right answers, and comes back only on new misses. Typing slips between unlike letters make no drill.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 ./internal/letters ./internal/session ./internal/activity` and `/tmp/go/bin/go test -count=1 -run Alefbet ./internal/web/views`.
- **Pass condition:** ac-1 to ac-8 pass. Restoring `if cardID <= 0 { return nil }` for drills fails ac-7. Removing the `Confusable` filter fails ac-5.

## Integration Points

- **Provenance tag:** `INIT-027/SPEC-003`
- SPEC-006 calls `DrillAnswered` from the letter pick handler with the server-measured latency.
- SPEC-004 edits the same package next.
