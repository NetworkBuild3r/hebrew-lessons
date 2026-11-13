# SPEC-028 · service: numbered lessons from the common words

## Metadata

```yaml
spec_id: SPEC-028
initiative_id: INIT-039
title: "Numbered lessons from the common words"
domain: service
status: complete
primary_prompt: none
complexity: medium
complexity_reason: one deterministic grouping over the course list, one progress read, and one study entry point
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 4 hours
blocked_by: [SPEC-024]
blocks: [SPEC-029, SPEC-032]
```

## Description

Brian said on 2026-11-12 that the app has no lessons that help him learn, and that the point is to learn the most key words first and spread out to less key words more and more. The signed course words are already in the database (`course_words`, 475 ready) and review already adds them, but nothing groups them into lessons he can see and open.

This spec groups the ready course words into numbered lessons of five, in frequency order. A prefix row is not a lesson word: it is taught inside a line, so it is skipped here. A learner has met a lesson when every word in it has a card.

## What to build

### `internal/speech/course.go`

- `const CourseLessonSize = 5`
- `type CourseLesson struct { Number int; Words []CourseWord }`
- `func CourseLessons() ([]CourseLesson, error)` reads the embedded file and groups it:
  - keep a row whose `Status` is `ready` and whose `Kind` is not `prefix`, because a prefix is taught inside a line;
  - keep them in course order;
  - group five at a time;
  - the last group may be shorter;
  - the number is the group's index plus one.
  With the current file that is 468 words in 94 lessons, the last of 3.
- `CourseLessonOf(n int) (CourseLesson, bool, error)` returns the lesson with that number, for the page route.

The grouping lives here, not in `internal/lessons`, so it needs no database and no new package dependency. The web layer joins a lesson's words to `course_words` by `CourseWord.Order`.

### Progress: `internal/db/course.go`

- `func (s *Store) CourseCardLexemes(ctx context.Context, userID int64) (map[int64]bool, error)` returns which course lexeme ids the learner has any card for. One query joining `cards` to `course_words` is enough.

### Studying one lesson: `internal/session/supply.go`

- Move the skills function out of `seedNewFromDeck` into `func lessonSkills(gate activity.Gate) func(bool) []string`, and use it in both places. It returns read and recall when the gate is open, plus listen when the gate is open and the word has a clip.
- `func (s *Service) BuildLesson(ctx context.Context, userID int64, lexemeIDs []int64) (*Session, error)`:
  1. return an error from `s.ready()` when the service or store is nil, as the other exported builds do;
  2. call `s.record.Introduce(ctx, userID, lexemeIDs, lessonSkills(gate))` where `gate` comes from `s.gate(ctx, userID)`; that call leaves an existing card untouched;
  3. return `s.BuildSessionFor(ctx, userID, lexemeIDs)`.
  An empty id list returns the empty session that `BuildSessionFor` builds, with no error.

## Tests

`internal/speech/course_test.go`:

- `TestCourseLessonsGroupsFiveInOrder`. The real file gives lessons of five words in course order, and lesson 1 is the seven pronouns split into the first five and the next ones. Assert the first lesson's pointed forms equal the first five ready word rows.
- `TestCourseLessonsSkipsPrefixes`. No lesson holds a row whose `Kind` is `prefix`.
- `TestCourseLessonsCoverEveryReadyWord`. Summing the lesson lengths equals the count of ready, non-prefix rows, and the numbers run 1..N with no gap.
- `TestCourseLessonOf`. Number 1 is the first lesson, the last number is the last lesson, and 0 or a number past the end is not found.

`internal/db/course_test.go`:

- `TestCourseCardLexemes`. Two course words with a card and one without: the map has two true entries and one false.

`internal/session/course_seed_test.go`:

- `TestBuildLessonIntroducesItsWords`. Three course words with no cards: the built session holds their visits, and all three now have cards.
- `TestBuildLessonLeavesOtherWordsAlone`. A course word outside the given set gets no card.

## Acceptance criteria

- [ ] `CourseLessons` groups the ready words five at a time, in order, without prefixes.
- [ ] `BuildLesson` introduces and queues exactly the words given.
- [ ] `go test ./internal/speech/ ./internal/db/ ./internal/session/` passes in Docker `golang:1.26`.

## Verification Strategy

- **Claim:** the common words form numbered lessons of five, and one lesson can be studied on its own.
- **Check + executor:** mechanized. The tests above.
- **Pass condition:** they pass. `go vet ./...` is clean.
