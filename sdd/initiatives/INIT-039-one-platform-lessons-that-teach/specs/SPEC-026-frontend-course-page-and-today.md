# SPEC-026 · frontend: the course page, and Today when words are waiting

## Metadata

```yaml
spec_id: SPEC-026
initiative_id: INIT-039
title: "The course page, and Today when words are waiting"
domain: frontend
status: complete
primary_prompt: none
complexity: medium
complexity_reason: one page rewritten from read-only facts, one POST route copied from the band review, and one new Today input
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-024]
blocks: []
```

## Description

`GET /speech` (`internal/web/speech_page.go`, `internal/web/views/speech.templ`) is a fixed page about the first twelve words. It records nothing. This spec turns it into Brian's course page, which shows his place on the list, what comes next, what is slipping, and what he did this week. It adds a button to practise only the slipping course words. It also lets Today offer a sitting when nothing is due but course words are waiting. Today stays one action. The main Progress screen is not touched.

RSCH-013 and its source [3] found that a progress bar and a score raised motivation and did not raise recall two to three days later. When that display came first, practice got slower. So the page leads with one can-do sentence and the start button, and the tracking sits below them. The page has no bars and no points.

## What to build

### 1. Read-only facts: `internal/web/course_view.go`

`func (s *server) courseView(ctx context.Context, userID int64) (views.CourseView, error)` reads, and never writes:

- **Met.** Course words (`store.CourseLexemes`) the learner has any card for (`store.LearnerCardLexemeIDs`). Count them. Place is the highest `course_order` among them, or 0.
- **Known.** Met course words whose learner-model state is `known` or `solid` in read or recall. Use `s.model().List` with a `State` filter, paging through the cursor, and intersect with the course ids. Do not call `Word` once per word.
- **Share.** Map known course words back to their list lemma with `speech.Course()`. A lemma counts only when every ready course row with that lemma is known, so knowing אֲנִי alone does not count all of הוּא's 12 percent. Share is the sum of `PerMillion` for those lemmas divided by 1,000,000. Add `func ShareOf(lemmas map[string]bool) (float64, error)` to `internal/speech/list.go`, with a test.
- **Next.** The next up to 8 ready course words with no learner card, in order. Each has pointed, English, `TimesLabel()`, and `ThroughLabel()` from `speech.Lookup(lemma)`.
- **Waiting.** How many words the next sitting would add. Add `func (s *Service) CourseWaiting(ctx context.Context, userID int64) (int, error)` to `internal/session/supply.go`. It is read-only and uses the same numbers as `seedNewFromDeck`: 0 when accuracy is below `TargetSuccess`, else `newAllowance(accuracy, MaxNewPerSession)` minus `CountDueNewCards`, capped by the ready course words with no card. Include a short `Reason` string. When accuracy is below 0.85 it reads "Recent answers are under 85 out of 100 right, so today is review only."
- **Slipping.** Met course words in state `regressed` or `struggling` in any skill, at most 10, most recent first. Each has pointed, English, and the cause from the newest `store.RegressionRows` event for that lexeme in the last 7 days, when one exists, shown through `views.CauseLabel`.
- **This week.** Over the last 7 study days in the learner's zone:
  - minutes is `jobs.ActiveMinutes(store.AttemptTimes(...))`, the same count the weekly report uses;
  - study days are the distinct study days in those attempt times;
  - met this week is course words whose first card for this learner has `created_at` in the window. Add `func (s *Store) CourseCardsCreated(ctx, userID, from, to time.Time) (int, error)` to `internal/db/course.go`;
  - slipped this week is course words with a regression event in the window, from `RegressionRows`.

  Words gained and lost with their cause come from the weekly report. SPEC-027 adds them as a "Last week" section.

### 2. The page: `internal/web/views/speech.templ`

Replace the body. Keep the route and `page.Title = "Common words · " + views.AppName`. In order:

1. One sentence: "You have met {Met} of the common words. The ones you know cover about {Share} of the words in Hebrew subtitles." With Met 0: "You have not started the common words yet."
2. The start button to `/review`: "Learn {Waiting} new words" when Waiting > 0, else "Review". When Waiting is 0 and Reason is set, show Reason under it.
3. **Next words** (`data-next`): pointed, English, and "{Times} times in every 1,000 words".
4. **Slipping** (`data-slipping`), shown when not empty: each word with its cause, then a form that POSTs to `/_hx/review/course` with `_csrf`. Its button reads "Practise these".
5. **This week** (`data-week`): minutes, study days, met this week, and slipped this week.
6. The climb map and follow note from SPEC-019 stay at the bottom (`data-climb`, `data-follow`). The fixed typing exercise and the ungraded "say" line are removed. They have no learner record behind them.

Regenerate with templ v0.3.960 by running `templ generate ./internal/web/views/speech.templ`. Do not hand-edit `speech_templ.go`.

### 3. `POST /_hx/review/course`

Add `func (s *server) postCourseReview(...)` to `internal/web/speech_page.go`, copying `postBandReview` in `internal/web/shell.go`: the session key, `ParseForm`, the ids, `BuildSessionFor`, `replaceQueue`, and a redirect to `/review`. The ids are the slipping course lexeme ids from `courseView`. They are computed on the server and never read from the form. Register it in `shell.go` beside `POST /_hx/review/band`. The existing CSRF middleware covers it. Check that, as the band review does.

### 4. Today

- `TodayInputs` gains `CourseWaiting int`. `ChooseTodayAction` returns `ActionSession` when `in.Askable || in.CourseWaiting > 0`, in the same position as `Askable`. Placement and check-in still come first.
- In `fillToday`, when signed in and `s.study != nil`, set `in.CourseWaiting` from `s.study.CourseWaiting`. Log and ignore an error, as the other supporting reads do.
- `TodayView` gains `CourseNew int`. When `page.Due == 0` and `CourseWaiting > 0`, the hero title reads "Learn {n} new common words". Otherwise keep `SessionButtonText`. When both are above 0, keep `SessionButtonText` and add a second line: "Includes {n} new common words."
- The idle link text becomes "Common words". It still links to `/speech`.
- Regenerate `today_templ.go` with templ v0.3.960.

## Tests

- `internal/web/shell_test.go`: in the `ChooseTodayAction` table, add the row "course words waiting, nothing due" → `ActionSession`. Add "placement never, course waiting" → placement.
- `internal/web/speech_page_test.go`: replace `TestSpeechLessonAfterReview` with `TestCoursePage`. On a temp store, sync four course rows, give the learner cards for two, and make one of those regressed. Then:
  - the page shows "You have met 2";
  - `data-next` lists the other two in order;
  - `data-slipping` lists the regressed word;
  - "Learn" appears with accuracy at or above 0.85.

  A second case at accuracy 0.80 shows the review-only reason.
- `TestCourseReviewUsesServerIDs`. Posting extra `id` fields to `/_hx/review/course` does not add those words to the queue.
- `TestCourseReviewNeedsCSRF`. A post without the token is refused.
- `internal/session`: `TestCourseWaitingMatchesSeed`. For the same store and clock, `CourseWaiting` equals the number of words `seedNewFromDeck` then introduces.
- `internal/speech`: `TestShareOfNeedsEverySplitRow`. Knowing only אֲנִי from the rank-1 split gives share 0 for הוּא.
- Run the layout audit `internal/web/testdata/layout-audit.mjs` against the local app on `/speech` and `/`, with no new issues.

## Acceptance criteria

- [ ] `/speech` shows, in order: the sentence, the button, next words, slipping words, this week, and the map.
- [ ] Practise-these builds a review set from ids computed on the server, behind CSRF.
- [ ] Today offers a sitting with nothing due when course words wait, and says how many.
- [ ] The main Progress screen is unchanged. `git diff --stat internal/web/views/progress*.templ` is empty.
- [ ] The tests above pass. The local app at `http://127.0.0.1:8099/speech`, signed in with `POST /auth/local`, shows the page on a copy of `data\local-dev`.

## Verification Strategy

- **Claim:** the course page shows place, next, slipping, and this week, and Today offers waiting words.
- **Check + executor:** mechanized. `go test ./internal/web/ ./internal/session/ ./internal/speech/` in Docker `golang:1.26`, plus the layout audit on the local server.
- **Pass condition:** all of them pass, and the audit reports no new issues.
