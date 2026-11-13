# SPEC-027 · service: the weekly report counts the course

## Metadata

```yaml
spec_id: SPEC-027
initiative_id: INIT-039
title: "The weekly report counts the course"
domain: service
status: complete
primary_prompt: none
complexity: medium
complexity_reason: one optional block in a pure payload builder, one new read, and a section on the course page
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 3 hours
blocked_by: [SPEC-024, SPEC-026]
blocks: []
```

## Description

Brian asked for tracking of his use and the words that slip. The weekly job (`internal/jobs/weekly.go`) already stores, per learner and week:

- words gained, from the baseline word states;
- words lost, with the daily job's cause and whether each is back;
- cleared words;
- retention;
- minutes studied;
- band slips.

This spec adds a course block to that payload: his place on the list, course words met in the week, course words gained and lost (with cause), and study days. The block is shown on the course page under "Last week". The weekly card on Progress keeps its summary text exactly as it is. RSCH-013 says to keep counts off that screen.

## What to build

### 1. The payload: `internal/jobs/weekly.go`

```go
// CourseWeek is the common-word course in one weekly report. It is absent when the
// learner has no course word with a card.
type CourseWeek struct {
	Place       int        `json:"place"`       // highest course_order the learner has a card for
	Met         int        `json:"met"`         // course words with a card, all time
	MetThisWeek int        `json:"metThisWeek"` // course words whose first card was created in the week
	Gained      []WordSkill `json:"gained"`     // the payload's Gained, course words only
	Lost        []LostWord  `json:"lost"`       // the payload's Lost, course words only, cause kept
	StudyDays   int        `json:"studyDays"`   // distinct study days with an attempt, in the learner's zone
}
```

- Add `Course *CourseWeek \`json:"course,omitempty"\`` to `WeeklyPayload`. Keep `payloadVersion` at 1. The field is optional, and old reports stay valid.
- Add to `weekInput`:
  - `course map[int64]int`, mapping lexeme id to course_order, from `store.CourseLexemes`;
  - `cards map[int64]time.Time`, the first card `created_at` per course lexeme for this learner. Add `func (s *Store) CourseCardFirstCreated(ctx context.Context, userID int64) (map[int64]time.Time, error)` to `internal/db/course.go`.

  Read both in `read`.
- In `buildWeekly`, which stays pure with no clock and no database:
  - when `len(in.cards) > 0`, fill `Course`;
  - Place is the max order over `in.cards` keys;
  - Met is `len(in.cards)`;
  - MetThisWeek counts `created_at` inside `weekWindow`;
  - Gained and Lost filter `p.Gained` and `p.Lost` by `in.course`;
  - StudyDays is the distinct `studyday` days of `in.attempts` in `in.loc`.
- Do not change `summarize`.

### 2. A read for the page: `internal/db/progress.go`

Add `Payload string` to `WeeklyReportRow`, scanned from the stored payload column (`payload_json`, per the `WeeklyPayload` comment). Existing callers ignore it.

### 3. The course page

In `courseView` (SPEC-026), read the newest report with `WeeklyReportRows(ctx, userID, "1970-01-01", today, 1)` and unmarshal `Payload` into `jobs.WeeklyPayload`. When `Course` is set, the page shows a "Last week" section (`data-last-week`) after "This week":

- "Week of {date}";
- "{MetThisWeek} new words, {StudyDays} study days, {Minutes} minutes";
- gained, as the pointed words;
- lost, each with its cause in plain words from `views.CauseLabel` (`internal/web/views/progress.go`). The daily job's causes are `confusion`, `decoding`, `production`, `spelling`, `band_slip`, and `unexplained`. Do not write a new mapping.

When no report has a course block, the section is not shown.

## Tests

`internal/jobs/weekly_test.go`, in the style of the existing `buildWeekly` tests:

- `TestWeeklyCourseBlock`:
  - Two course words, one met this week and one met before.
  - One course word is in `Lost` with cause `confusion`. One non-course word is also lost.
  - Two attempts on one study day and one on another.
  - The block has Place, Met 2, MetThisWeek 1, the one lost course word with its cause, and StudyDays 2.
  - The non-course word is not in the block.
- `TestWeeklyNoCourseBlock`. A learner with no course cards has `"course"` absent from the marshalled JSON.
- `TestWeeklySummaryUnchanged`. With and without a course block, `summarize` returns the same string.

`internal/web/speech_page_test.go`:

- `TestCoursePageLastWeek`. A stored report with a course block shows `data-last-week` with the lost word and its cause.

## Acceptance criteria

- [ ] The weekly payload carries the course block when the learner has course cards, and has none otherwise.
- [ ] The Progress weekly card text is unchanged. The summary test proves it.
- [ ] The course page shows last week's course lines.
- [ ] `go test ./internal/jobs/ ./internal/db/ ./internal/web/` passes in Docker `golang:1.26`.

## Verification Strategy

- **Claim:** each weekly report records course place, words met, gains, losses with cause, and study days.
- **Check + executor:** mechanized. The tests above.
- **Pass condition:** they pass. `go vet ./...` is clean.
