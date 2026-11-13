# SPEC-029 · frontend: the Lessons page is the course

## Metadata

```yaml
spec_id: SPEC-029
initiative_id: INIT-039
title: "The Lessons page is the course"
domain: frontend
status: complete
primary_prompt: none
complexity: medium
complexity_reason: one page rewritten from the course lessons, one lesson page, and one POST route
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-028]
blocks: [SPEC-032]
```

## Description

`GET /lessons` (`internal/web/shell.go` → `lessonRows`) lists the learner's published units, which today are four class-sheet units from the old pipeline. Brian opened that page on 2026-11-12 and saw no lessons that help him learn. This spec makes it the course: numbered lessons of five common words in frequency order, each one marked done, next, or later, and each openable.

The old class-sheet units are not deleted. They move to the Tutor section (SPEC-030), which is where a class taught by a tutor belongs.

## What to build

### `GET /lessons`

Change `serveLessons` and `views.LessonsView`:

- `serveLessons` builds the rows from `speech.CourseLessons()` joined to `Store.CourseLexemes` by `Order`, instead of `s.lessonRows`.
- `views.LessonRow` gains `Number int` and `Words []string`. `Title` becomes "Lesson {number}". `Gloss` is the five English glosses joined with " · ".
- State, from `Store.CourseCardLexemes`:
  - `done` when every word in the lesson has a card;
  - `next` for the first lesson that is not done;
  - `later` for the rest.
- `Href` is `/lessons/{number}` for every lesson.
- Add `Summary string` to `LessonsView`: "Lesson {next} of {total}" when a next lesson exists, or "All {total} lessons done." when every lesson is done. Empty when there are no lessons.
- In `internal/web/views/me.templ` (`templ Lessons`), under the `h1`, render `v.Summary` in a `<p class="lessons__summary">` when it is not empty. In each row, render `row.Words` joined with " · " under the title, in `dir="rtl"`, in a `<p class="lesson-row__words">`. Keep `data-state` so the existing styles apply.
- When there are no ready course words, keep the existing empty state and leave `Summary` empty. Do not invent a lesson.

### `GET /lessons/{number}`

New file `internal/web/course_lesson.go`.

- `serveCourseLesson` builds the lessons the same way, finds the one whose `Number` matches the path value, and calls `notFound` for an unknown number.
- The page, in a new `internal/web/views/course_lesson.templ`:
  - title "Lesson {number}";
  - one row per word: pointed form, English, and "{X} times in every 1,000 words" from `speech.Lookup(lemma)`. Show the frequency line only when a lookup succeeds;
  - a `data-heard` paragraph only when every word in the lesson has a stored clip (`store.LexemeByID(...).AudioID` is set, or `store.` has a clip for its text). Otherwise no heard line, because there is no accepted voice (INIT-039 decision A5);
  - a form that POSTs to `/_hx/lessons/{number}/study` with `_csrf`, whose button reads "Study these five", or "Study these again" when the lesson is done.
- `POST /_hx/lessons/{number}/study`:
  - needs the session and the CSRF token, as `postBandReview` does (the middleware covers the token; check the session key here);
  - reads the lesson from the server-side list, never from the form;
  - calls `s.study.BuildLesson(ctx, user.ID, ids)`;
  - `replaceQueue`, then `http.Redirect` to `/review`.
- Register `GET /lessons/{number}` and `POST /_hx/lessons/{number}/study` in `shell.go`. The existing `GET /lessons/{id}/report` route keeps working; the more specific path wins in the mux.

### One link from the course page

In `internal/web/views/speech.templ`, after the start button, add a link to `/lessons` reading "All lessons".

## Tests

New `internal/web/course_lesson_test.go`, using the `openShell` and `addLearner` helpers:

- `TestLessonsPageListsTheCourse`. Sync six course words. The page shows "Lesson 1", "Lesson 2", the six pointed words in order, and `href="/lessons/1"`.
- `TestLessonsPageMarksDoneAndNext`. Give the learner cards for lesson 1's words: lesson 1 renders `data-state="done"`, lesson 2 `data-state="next"`, and the summary reads "Lesson 2 of 2".
- `TestLessonsPageAllDone`. Cards for every word: the summary reads "All 2 lessons done."
- `TestCourseLessonPage`. Lesson 1 shows its five words, a frequency line, and the study form.
- `TestCourseLessonUnknownNumber`. `/lessons/99` is 404.
- `TestStudyCourseLessonQueue`. Posting to `/_hx/lessons/1/study` introduces the lesson's words and redirects to `/review`; the first served card is one of them.
- `TestStudyCourseLessonNeedsCSRF`. A post without the token is refused.
- `TestStudyCourseLessonIgnoresPostedIds`. Posting `id=999` adds no such word.

Then run the whole `./internal/web/` package, and the layout audit `internal/web/testdata/layout-audit.mjs` against `/lessons` and `/lessons/1` on a copy of `data\local-dev`.

## Acceptance criteria

- [ ] `/lessons` lists numbered lessons of five common words in order, with done, next, and later, and a progress line.
- [ ] `/lessons/{n}` shows the words and studies exactly them, behind CSRF.
- [ ] The old class-sheet units no longer appear on `/lessons`.
- [ ] Nothing is deleted from the database.
- [ ] The tests pass and the layout audit reports no new issues.

## Verification Strategy

- **Claim:** the Lessons section teaches the most common words first, five at a time.
- **Check + executor:** mechanized. The tests above, plus the layout audit on the local server.
- **Pass condition:** all pass, and the audit reports no new issues.
