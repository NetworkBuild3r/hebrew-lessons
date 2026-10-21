# SPEC-005 completion summary

**Initiative:** INIT-015 · **Spec:** SPEC-005 (service) · **Status:** complete (ac-4 narrowed by operator decision) · **Date:** 2026-10-19

## Change

Commit `7f48905` on local branch `v3-spec-005` (13 files), merged into local `v3` as `51933bc` with no conflicts. Not pushed.

- New package `internal/evidence`: `Recorder.Attempt`, `Recorder.SelfGrade`, `Recorder.Expose`, `Grade`, `Median`.
- `internal/db/evidence.go`: `WriteAttempt` writes the attempt, the card update, the `review_log` row and any confusion row in one transaction. Also `CorrectLatencies`, `ListAttempts`, `InsertExposures`.
- `internal/fsrs`: `(*Review).ScheduleNext`.
- Callers changed: `internal/session/session.go` (review path) and `internal/web/lesson.go` (lesson player).
- No migration. No existing test was edited; all added tests are new files.

## Grading rule as implemented

| Case | Rating |
| --- | --- |
| wrong, or "I don't know" (wins even if marked correct) | Again |
| right with a hint | Hard |
| right, timed, strictly faster than the median, and the median is above 0 | Easy |
| any other right answer | Good |

The median is taken over the learner's last 50 correct, timed attempts for that activity. Until 10 exist it is 0, so Easy is never given. Placement: a right answer seeds a Good card only if the word has none; a wrong answer or "I don't know" writes the attempt and no card (D2).

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestGrade`, `TestMedian`, `TestEasyNeedsTenCorrectAndABeatenMedian`, `TestAttemptRatings`. |
| ac-2 | PASS | `TestAttemptWritesOneAttemptAndOneReview`, `TestAttemptIsAtomic` (a trigger fails each later step and every table stays empty). |
| ac-3 | PARTIAL | Review path: `TestRecordWritesAnAttempt`. Lesson path: `TestLessonChecksWriteAttempts`. The typed path is untestable: the orchestrator confirmed `fsrs.GradeTyped` has no caller outside tests, so v2 has no typed activity to route. |
| ac-4 | PASS, narrowed | A confusion row is written only when the wrong pick differs from the right word by exactly one letter. The existing `confusion_misses.kind` check allows only `letter_pair` and `agreement`. A row for any other pair needs a new kind, so a migration. |

The orchestrator ran the merged `v3` at `51933bc` itself: `gofmt -l .` empty, `go vet ./...` clean, `go test -count=1 ./...` all packages ok.

## Deviations and disclosures

- **Lesson hook uses `SkipSchedule: true`.** Lesson checks write an attempt but do not change any card. Scheduling there would have changed v2 behaviour and broken `TestLessonPlayer`. Cards for new words still come from `enqueueNewWords` when a lesson ends. SPEC-013 or SPEC-015 decides whether lesson answers should drive FSRS.
- **Lesson attempts store `latency_ms = 0`**, because `lesson_runs` has no question-shown time. Zero means untimed. It is never Easy and is ignored by the median.
- **The two lesson writes are not one transaction.** If the attempt write fails after the lesson step was applied, the step returns a 500 and that attempt is lost.
- **Placement service not rewired.** SPEC-007 owns it. The recorder already follows D2 for when SPEC-007 calls it.
- **Not verified.** The Hebrew test words were not checked against a dictionary. No `-race` run. No end-to-end HTTP call to the review endpoint.
- **No personas, timer, trace or independent AC verifier** (as in earlier specs).

## Gates

- **Gate 5 (2026-10-19):** the operator accepted the narrower ac-4 now. SPEC-013 adds an other confusion kind in a small migration and widens the rule. SPEC-013's file records this.
- **Do not push `v3`.** A push publishes an image that may auto-deploy.
