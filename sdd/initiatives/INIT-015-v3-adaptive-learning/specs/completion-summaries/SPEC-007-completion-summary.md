# SPEC-007 completion summary: Placement engine v3

- **Status:** complete. Merged into local `v3` as `9de7df1` (worker commit `b1bd820`). Not pushed.
- **Worker:** general-purpose sub-agent in worktree `Hebrew-v3-s007`. No personas, timer or trace scripts exist in this harness, so none were used.
- **Migration:** `migrations/010_placement_v3.sql` (not planned by the spec). `gate_actions` is now `db_migration`: the first run on a copy of a real database needs the operator.

## What shipped

`internal/placement` has a v3 `Engine`: letters, adaptive listening and reading, an optional typing part, no feedback during the run, the guessing correction, and a 28-day re-check. The v2 `Service` moved to `legacy_*.go` because `internal/web/placement.go` still calls it. SPEC-012 rewrites that handler.

## Acceptance criteria

| AC | Proof | Result |
| --- | --- | --- |
| ac-1 | `TestAdaptivePlacesAtLevelThreeAndStopsBandFour` (200 seeds, both skills), `TestAdaptiveBandMoves` | pass |
| ac-2 | `TestAdaptiveNoLexemeAskedTwice` (1,000 seeds x 3 learners), `TestStoreRefusesAWordAskedTwice` | pass |
| ac-3 | `TestSeedOnlyRightAnswersBecomeGoodCards`, `TestSeedLeavesAnExistingCardAlone` | pass |
| ac-4 | `TestNoFeedbackStepsCarryNoCorrectnessField`, `TestNoFeedbackReplyIsTheSameWhateverTheAnswer`, `TestNoFeedbackItemsShowOnlyWhatTheirPartAllows` | pass |
| ac-5 | `TestAdaptiveLettersGateSkipsReading` (0, 4, 5, 6 right) | pass |
| ac-6 | `TestScoreGuessingCorrection` (0.6111) | pass |
| ac-7 | `TestRecheckDueIsFalseAtDay27AndTrueAtDay28` | pass |

The worker ran 26 mutation checks and reports each made a test fail.

## Verified by the orchestrator (not taken from the worker's report)

- Clean tree. `gofmt -l .` empty, `go vet ./...` clean, `go run ./cmd/tokens -check` exit 0.
- Full suite `go test ./... -count=1`: every package ok, on the worker branch and again on the merge result for db, placement, web and app.
- Read `Engine.Answer`: the run is loaded by `userID` and `runID`, the open item must match the dealt item token, the right option stays server-side, and the claim is exclusive. A failed attempt write releases the claim.
- Read migration 010: two new tables, no change to an existing table, a unique index that stops a word being asked twice in listening, reading or letters.
- I did not repeat the worker's mutation checks.

## Deviations from the spec

- Bands are numbered 1 to 5 (deck band plus one), so "level 3" in ac-1 holds. `placement_runs.level` stores the deck number.
- A letters item has no lexeme, so it records no attempt. Its outcome is stored on `placement_items`.
- The `placement_review` exposure is written at finish, not at answer time. A run abandoned halfway writes none.
- Bands 3 and 4 have 0 or 1 adverbs, so adverbs borrow adjectives as distractors, and the reverse. A test proves nothing else crosses part of speech.
- The gloss-overlap rule is copied from `internal/activity` (importing it would be a cycle). Its stemmer has the same blind spot: "house" and "houses" do not count as overlapping.
- Typing is offered only after letters pass (north-star 5.2) and asks at most 8 words. Only an exact answer counts as right.
- `NewRecheck` takes a `ctx` that the spec's signature did not show.

## Risks and open items

- **Not atomic.** Claiming an item and writing the attempt are two steps. A hard crash between them loses that answer. A retry after a failure inside `Finish` could duplicate exposures.
- `Start` resumes an open run of either kind, including an unfinished re-check.
- **Not driven from a browser.** SPEC-012 is the first to drive the engine over HTTP, and `app.App` is not wired to it yet.
- The six letter clips (SPEC-006) have still not been listened to.
- The placement suite takes about 90 s here (200 s with `-race`).

## Public API

`NewEngine(store, rec, clk, opts...)`, `WithSource`, and on `*Engine`: `Start`, `Next`, `Answer`, `StartTyping`, `Finish`, `Result`, `Latest`, `NewRecheck(ctx, userID, skill)`, `RecheckDue`. `Step` carries `Item`, `TypingOffer` or `Done`, and has no correctness field. Errors: `ErrNotYourRun`, `ErrWrongItem`, `ErrBadAnswer`, `ErrFinished`, `ErrNotReady`, `ErrNoAudio`, `ErrDeckShort`, `ErrNoPlacement`.
