# SPEC-005 · service: New words follow accuracy and come from the frequency list

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-027
title: "New words follow accuracy and come from the frequency list"
domain: service
status: pending
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-004]
blocks: [SPEC-006]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- `newAllowance` (`internal/session/mix.go:587`) is min(`MaxNewPerSession` 5, max(`MinNewPerSession` 2, floor(`MaxNewShare` 0.25 × size))) whatever the learner's accuracy. The constants live in `internal/sessionlimits` so MCP reads the same values: `internal/mcp/schema.go:38-50` (`mixTarget`), `internal/mcp/coach.go:89-90` and `:166`, and the tests in `internal/mcp/tool_learner_words_test.go:26-48` and `:101`.
- Review can only show `new` cards that lessons or `record_taught_words` created. When none wait, a learner doing well gets no new words from review.
- The placement deck (`internal/placement/items.json`, 200 rows, bands 0–4) is ordered by frequency inside each band and holds `verified` and `gloss_en`. `db.Store.PlacementItemLexemes` maps item ids to lexeme ids (used by `activity.PlacementBands`, `internal/activity/build.go:27`).
- `db.Store.RecordTaught` (`internal/db/taught.go:80-100`) shows the new-card insert: read and recall, plus listen when `lexemeHasStoredClip`, as `INSERT OR IGNORE` with state `new`. It also writes `taught_words`, which review seeding must not do.
- North star §4.1: before the letters gate opens, no `read` or `spell` card is created.
- `CardRef.Intro` (`New && !Exposed`) already makes the screen show a word before its first question.
- ADR-0023 D5 (SPEC-001). Accuracy excludes implied reviews (SPEC-002).

**Scope (in):**
- **Accuracy tiers.**
  - `internal/sessionlimits`: replace `MinNewPerSession` and `MaxNewShare` with `NewPerSessionLow` = 2, `NewPerSessionMid` = 5, `NewPerSessionHigh` = 8. Also add `NewTierLow` = 0.80 and `NewTierHigh` = 0.90.
  - `MaxNewPerSession` becomes `NewPerSessionHigh`.
  - `session.LowAccuracy` and `HighAccuracy` read the tier bounds, so the two stay equal.
  - `newAllowance(accuracy float64, available int) int` returns the tier cap, capped by available:
    - below 0.80 → 2;
    - 0.80 through 0.90 inclusive → 5;
    - above 0.90 → 8.
  - No history uses `NoHistoryAccuracy`, so 5. The planned-queue size no longer matters.
  - Drop the size-share loop and its comments.
- **Supply.** In `Service.build`, only for a full build (not `BuildSessionFor`):
  - It runs when accuracy ≥ `TargetSuccess` and the learner's due `new` cards number fewer than the tier cap. "Due new" is the same reading `classify` uses.
  - It then seeds up to (cap − waiting) words, before `ListStudyCards`, so the seeded cards are in the same build.
  - Word choice:
    - deck rows in band order, then file order;
    - only `verified` rows with a non-empty `gloss_en`;
    - the row maps to a lexeme through `PlacementItemLexemes`;
    - the lexeme has a non-empty gloss;
    - the learner has no card of any skill for the lexeme.
    - A lexeme reached by two rows is taken once.
  - Add `placement.Deck() ([]DeckRow, error)` returning `ID, Band, Verified, GlossEN` in file order.
- **Writer.** New `evidence.Recorder.Introduce(ctx, learnerID int64, lexemeIDs []int64, skills func(hasClip bool) []string) (IntroduceResult, error)`:
  - It creates `new` cards due at the recorder clock's now, through a new `db.Store.CreateNewCards`.
  - `CreateNewCards` shares the card-insert helper with `RecordTaught`. Factor it out; do not copy it. It uses `INSERT OR IGNORE`, never changes an existing card, and writes no `taught_words`, attempts, exposures, or review_log.
  - The session passes the skills read and recall, plus listen with a clip, each kept only when `gate.Open(skill)`.
  - It returns the cards created per lexeme.
  - The learner id is the session's.
- **MCP mirrors.**
  - `mixTarget` (`internal/mcp/schema.go`) and the coach struct (`coach.go`) drop `maxNewShare`. They keep `maxNewPerSession` (now 8) and add `newPerSessionLow`, `newPerSessionMid`, `newPerSessionHigh`, `newTierLow`, `newTierHigh`.
  - The `coach.go:166` sentence describes the tiers and the supply rule.
  - Update `tool_learner_words_test.go`, and any doc or resource text that names the old formula (`rg -n 'MaxNewShare|maxNewShare|MinNewPerSession|0\.25' internal docs`). List each in the completion summary.
- Tests:
  - the tier boundaries 0.7999, 0.80, 0.90, 0.9001 and no history;
  - at 0.92 accuracy with no new cards waiting, a build seeds 8 words in deck order and queues them with `Intro`;
  - at 0.78 nothing is seeded, and at most 2 new cards are queued;
  - with 6 new cards waiting at 0.92, 2 words are seeded;
  - pre-gate seeding creates no read card;
  - a lexeme with any existing card is skipped;
  - an unverified row is skipped;
  - `BuildSessionFor` seeds nothing;
  - a second build on the same clock seeds nothing more;
  - learner isolation.

**Scope (out):** A word source larger than the placement deck. Loading `FreqRank` into candidates. The lesson brief's new-word counts (`MaxNewWordsPerUnit`, per-lesson limits). Screens.

## Acceptance Criteria

- [ ] ac-1: `newAllowance` returns 2, 5, 5, 8 at accuracy 0.79, 0.80, 0.90, 0.91, never more than available, and 5 with no history.
- [ ] ac-2: At 92% accuracy with no `new` cards, one build creates `new` cards for the first 8 eligible deck words (lowest band first) and queues 8 new visits with `Intro` set.
- [ ] ac-3: Below the 85% target no card is created by a build.
- [ ] ac-4: Seeding never creates a card for a word that already has one, never creates read or spell before the gate, and never writes `taught_words`, attempts, exposures, or review_log.
- [ ] ac-5: `get_learner_words` / the coach `mixTarget` reports `maxNewPerSession` 8 and the tier fields, and no longer reports `maxNewShare`. The MCP tests pass against the shared constants.
- [ ] ac-6: No code or doc in the repo still states the 0.25 share formula for review sessions.

## Deliverables

- [ ] `internal/sessionlimits/*.go`
- [ ] `internal/session/mix.go`, `internal/session/session.go`, and tests
- [ ] `internal/placement/items.go` (`Deck`) and test
- [ ] `internal/evidence/introduce.go` (new) and test
- [ ] `internal/db/taught.go` (shared insert helper) and `internal/db/cards_new.go` (new) with tests
- [ ] `internal/mcp/schema.go`, `internal/mcp/coach.go`, `internal/mcp/tool_learner_words_test.go`, and any doc naming the old formula

## Verification Strategy

- **Claim:** The number of new words follows recent accuracy, and a learner at or above target gets new words from the frequency list even when no lesson made them.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 ./internal/session ./internal/sessionlimits ./internal/placement ./internal/evidence ./internal/db` and `/tmp/go/bin/go test -count=1 -run 'LearnerWords|Coach|MixTarget' ./internal/mcp`.
- **Pass condition:** ac-1 to ac-6 pass. Reverting `newAllowance` to the share rule fails ac-1. Seeding with gate closed creating read fails ac-4.

## Integration Points

- **Provenance tag:** `INIT-027/SPEC-005`
- SPEC-006 counts `NewMet` on the done screen. The existing intro screen shows seeded words.
