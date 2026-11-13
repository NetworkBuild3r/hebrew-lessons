# SPEC-033 · frontend: Progress shows the mix

## Metadata

```yaml
spec_id: SPEC-033
initiative_id: INIT-039
title: "Progress shows the mix"
domain: frontend
status: complete
primary_prompt: none
complexity: low
complexity_reason: three short lines and two links added to the existing brief view
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 3 hours
blocked_by: [SPEC-026, SPEC-028]
blocks: []
```

## Description

Brian asked for a Progress that shows the mix of everything: the course, his tutor's classes, review, and learning. The page already has a brief view (one can-do sentence and one next action, `Brief` in `views.ProgressView`) and a full detail view behind `?detail=1` with the ladder, charts, and weekly card. RSCH-013 is why the brief view has no counts.

So the mix goes into the brief view as three short lines and two links, and the detail view is left exactly as it is. Nothing is removed, and 85 existing tests that read the Progress page stay meaningful.

## What to build

### Data

- `views.ProgressView` gains:
  - `CourseLine string`: "Lesson 5 of 94. The words you know cover about 61% of Hebrew subtitles." from `courseView`'s numbers (SPEC-026). When no course words are met: "The common words start with your first lesson."
  - `TutorLine string`: "2 classes, 31 words from your tutor." from `store.ClassSheets`. When no sheets: "No class loaded yet."
  - `ReviewLine string`: "This week: 92 answers, 78 right, 12 words came back." from `jobs.ActiveMinutes`' attempt window and `store.RegressionRows`, the same reads `courseWeek` already does. When there is no activity: "No study yet this week."
- `fillProgress` sets them when `v.Brief` is true. It does not set them on the detail view.

### The brief view

In `internal/web/views/progress.templ`, inside the `if v.Brief` branch, after the can-do line and before the next-action button:

- a `<section class="progress__mix" data-mix="1">` holding the three lines as `<p>` elements, each with its own `data-course`, `data-tutor`, `data-review` attribute;
- then two links: "All lessons" to `/lessons` and "Your tutor's classes" to `/tutor`;
- then, at the end of the brief view, one quiet link "See the detail" to `/progress?detail=1`.

No chart, no percent bar, no streak. The two percentages in `CourseLine` are the subtitle-share figure the course page already shows; if RSCH-013's rule is read strictly, keep only the lesson number. Use the lesson number plus the share, because the share is what Brian asked to see ("this word is used X times… if learned it allows you to speak X").

### Tests

- `internal/web/progress_test.go`:
  - `TestProgressBriefShowsTheMix`. With course words, one sheet, and an answered card: the brief page shows the three lines and both links, and no `data-mix` charts.
  - `TestProgressBriefSaysWhatIsMissing`. A learner with nothing: each line is its empty wording, and the page still renders.
  - `TestProgressDetailUnchanged`. `/progress?detail=1` still shows the ladder and the weekly card, and does not show `data-mix`.
- Run the whole `./internal/web/` package unchanged apart from the new cases.
- Layout audit on `/progress` and `/progress?detail=1`.

## Acceptance criteria

- [ ] The brief Progress view shows the course line, the tutor line, the review line, two links, and one link to the detail.
- [ ] The detail view is unchanged.
- [ ] Existing Progress tests pass without edits.
- [ ] The layout audit reports no new issues.

## Verification Strategy

- **Claim:** Progress shows the mix without turning into a dashboard.
- **Check + executor:** mechanized. The new cases plus the existing suite.
- **Pass condition:** all pass.
