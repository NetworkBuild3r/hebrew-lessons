# SPEC-015 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-015 (service) · **Status:** complete · **Date:** 2026-10-19

## Change

Commit `bba0950` on local branch `v3-spec-015`, merged into local `v3` as `c09ef05` with no conflicts (after SPEC-019 and the SPEC-009 fixes). Not pushed. No migration, no new dependency, no existing test edited.

- **New package `internal/activity`:** `Engine` (`Build`, `Check`, `Expose`), HMAC-signed single-use tokens (30 min, key from `HEBREW_SESSION_SECRET` via HKDF), distractor picker, typing aligner, and a pure planner (`Choose`, `Gate`). Activities: `listen_pick`, `read_pick`, `word_match`, `audio_match`, `type_it`, `dictation`, plus `flashcard` as the fallback.
- **`internal/session`:** `WithGate` and `WithBands` options; `CardRef` gains `Activity`, `AskSkill`, `Intro`; `Session.Answered`; the mix chooses a skill and activity per visit. The confusion signal now comes from `learner.Model` instead of `StruggleStatsAt`.
- **Shared files edited:** `internal/db/evidence.go` (`AttemptWrite.Extra`), `internal/db/learning.go` (study cards now include listen and spell), new `internal/db/activity.go`, `internal/evidence/recorder.go` and `grade.go` (near miss is Hard).
- Orchestrator edit after merge (`v3` commit following `c09ef05`): two stale comments in `internal/session` that still named `StruggleStatsAt`.

## Acceptance criteria

| AC | Result | Evidence |
| --- | --- | --- |
| ac-1 | PASS | `TestBuiltItemsAreFair` (10,000 items), `TestPickIsFair`, `TestPickTakesTheAnswersBandAndPartOfSpeechFirst`. The worker reverted the synonym filter and the band/part-of-speech preference in turn and the test failed each time. |
| ac-2 | PASS | `TestChoose`, `TestMixPicksListenPickForTheWeakestListenAccuracy`, `TestMixNeverAsksAFlashcardWhenACheckedActivityFits` |
| ac-3 | PASS | `TestTypedAnswersAreGradedAndRecorded`, `TestLetterPairs` |
| ac-4 | PASS | `TestWordMatchWithOneSwappedPair` (two confusion rows, Again for both swapped words, Good for the other three) |
| ac-5 | PASS | `TestCheckRefusesAnotherLearnersToken` (refused, nothing written, owner can still answer) |

Orchestrator checks on merged `v3`: `gofmt -l .` empty, `go vet ./...` clean, `go run ./cmd/tokens -check` ok, `go test -count=1 ./...` all packages ok, `go test -race ./internal/activity/ ./internal/session/` ok. Spot checks: `Item` carries no answer field (opaque token, options in shown order), token signature is compared with `hmac.Equal`, and no non-test code outside `db` and a doc comment reads `StruggleStatsAt` any more.

## Deviations and risks

- **A listen or spell card is created on the first checked attempt**, in the same transaction, not at `Build`. An unanswered ask leaves no orphan card.
- **Letters gate** is not decided anywhere. The default gate opens all skills; callers pass the real one with `session.WithGate`. Same hook as SPEC-019's `WithUnlockedSkills`; wire them to one source.
- **Near-miss typing** (one letter off) is stored with `correct` = 1 plus a near-miss flag and rated Hard.
- **Match activities** run every 3rd visit through a skill (`MatchEvery`); this is the worker's choice, since the spec is silent.
- **`Intro` is a flag, not a queue step.** The screen (SPEC-016) must call `Engine.Expose` before the first check.
- **Pending items live in memory**, capped at 4096. A restart drops them.
- **`PlacementBands` repeats about 15 lines** of a private default in `internal/learner`.
- **The unchanged web screen still serves every card as a flashcard.** No screen calls `Build`, `Check`, `Answered` or `Expose` until SPEC-016.
- **SPEC-019 cross-check:** the activity names SPEC-019 puts in a brief match the SPEC-015 kinds exactly.
- **Not measured at scale**, never run against a real database. No personas, timer or independent verifier.

## Gates

None. Do not push `v3`.
