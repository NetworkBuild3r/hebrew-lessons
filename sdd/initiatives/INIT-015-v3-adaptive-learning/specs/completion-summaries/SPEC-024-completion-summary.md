# SPEC-024 completion summary: Visual and usability regression checks

- **Status:** complete. Worker commit `5a0fb4e`, merged into local `v3` as `844f16d` (clean, no conflicts). Not pushed. No migration (the next free number is still 012).
- **Worker:** [SPEC-024 worker](edb75221-60a6-46a2-b6b0-d60623af00b7), a general-purpose sub-agent in worktree `Hebrew-v3-s024`.

## What shipped

- `internal/web/shots_test.go` and `testdata/shots.mjs` replace `placement_shots_test.go` and `placement-shots.mjs`. The test captures 63 screens in light and dark at 360, 390, 1024 and 1440 px (504 PNGs), plus a reduced-motion pass at all four widths (252 more cells, no PNG). It writes `testdata/shots/index.html` and `results.json`.
- Screens: placement 13, activity 34 (six kinds in ask, wrong and right, flash front and back, flash-recall front and back), dialogue 2, shell 19 (Today in seven states, Lessons, Review, Review empty, Me, sign-in, lesson story, ask, result, report sheet, and the no-script report page), Progress 9.
- Checks on every capture: axe-core 4.10.2 (WCAG 2.2 AA, vendored under `testdata/axe/` with the MPL-2.0 licence and notice), the one-gold-action rule, no `--go`/`--stop` outside feedback, 44 px targets, no horizontal scroll, and no running animation under reduced motion.
- `testdata/figma/`: 19 Figma exports. The index shows "no frame" for every screen without one. Comparison is by inspection only.
- The capture output (59.6 MB) is git-ignored by `testdata/shots/.gitignore`; the test regenerates it. The index generator and the Figma PNGs are committed.
- `TestShots` skips with a clear message when `MOTION_PW_DIR` is unset, node is missing, or there is no playwright. It needs `-tags browser`. `SHOTS_PARALLEL`, `SHOTS_DIR`, `SHOTS_ONLY` and `SHOTS_SCREENS` narrow or tune a run (a narrowed run is partial).

## Acceptance criteria

| AC | Proof | Result |
| --- | --- | --- |
| ac-1 | `TestShots` fails with "MISSING capture: ..." for any absent cell; passes on the full matrix | pass |
| ac-2 | zero serious or critical axe violations on all 504 captures | pass |
| ac-3 | gold, feedback-colour, target-size, no-scroll and reduced-motion checks clean, with one named exception (below) | pass |
| ac-4 | `testdata/shots/index.html` exists with each capture beside its Figma export; reviewer notes below | pass |

## Rules the test applies

- **Gold:** a visible Kind=Primary button, the gold orb, or any pressable thing painted `--lamp`. A screen never has more than one. Per screen: "one" has exactly one Primary button, "orb" has exactly one orb and no gold button, "none" has no gold at all. This resolves the SPEC-012 ac-1 deviation: pick screens are the "orb" kind.
- **Targets:** a radio or checkbox with a visible label counts as the label's box. The default floor is 44x44.
- **Contrast:** axe cannot resolve contrast over the body's radial gradient, so it is flattened for the axe run. A few elements stay "needs review" (hero card gradient, SVG chart text, text under the dock). They are listed as notes in the index.

## Real defect found and fixed

On placement typing at 360 px the page scrolled sideways by 4 px. The visually hidden label `.vh` had no clipping, so its nowrap text overran its 1 px box. Fixed in `base.css` with `overflow: clip` (`csslint_test.go` forbids `overflow: hidden` there).

## The one named exception

Hebrew keyboard keys on `activity-type_it-ask`, `activity-dictation-ask` and `placement-typing`, at 360 and 390 px only. Reason: SPEC-011 aud-1 decided that keys are 44 px tall and at least 28 px wide, because ten or more keys cannot be 44 px wide on a phone. The test requires the narrowest key to be at least 28x44 and allows at most 40 keys per screen per width. It fails if the exception covers nothing or the count grows. `TestShotsExceptions` proves that without a browser. At 1024 and 1440 px the keys are fine.

## Verified by the orchestrator

Worker worktree clean. On the merged `v3`: the full Go suite and the seven browser suites (the six existing plus Shots) were re-run by the orchestrator; the result is in the final report. The worker's own final run: `TestShots` 838 s, `TestShotsExceptions`, and the six existing suites all passed; `gofmt`, `go vet` (also with `-tags browser`), tokens check, `templ generate` (no diff) and `go test ./...` clean. I did not repeat the worker's mutations. It ran six, each on a narrowed set of screens, and each failed with the screen, width and mode named, then was restored with an identical `git diff`: axe (a lowered `--muted`), a second gold button, `--go`/`--stop` on a non-feedback element, a 30 px primary button, a 2000 px `h1` (still caught with `overflow-x: hidden` on the body), and an infinite animation outside the media query. Mutations were not run on every screen.

## Not covered

- There is no "Words" destination in v3. The five destinations (Today, Lessons, Review, Progress, Me) are all covered.
- Activity screens are the fixture routes (`/__screen/...`), not a live session, so lesson-player activity steps are not captured.
- The dialogue "line playing" state is tested only with the silent test clip.
- Figma has no frame for placement letters, resume, confirm, break, reading-picked and offer; any activity kind except flash front and back; dialogue, Lessons, Review, Review empty, Me, sign-in, the lesson story, ask and report screens; the other Today states; Progress variants other than the default; every 360 px and 1024 px capture; and the dark variants of most screens.

## Reviewer notes (the worker looked at the captures; I have not reviewed them by eye)

These are observations, not failures of any check.

- **Placement vs Figma:** close on type, orb, answer rows and the dark palette. Copy differs: the intro says "10-12 minutes" and "Start" where Figma has "8 minutes" and "Start listening". The skip action is a bordered button where Figma has a text link. The intro rows use a speaker icon where Figma has headphones. The Typing row blurb differs.
- **Today** has diverged from the Figma frames, which still show a greeting, a search box, "Words" in the nav, a weekly bar chart and a Teacher studio section. None are in v3, and the nav has Me where Words was. On desktop checkin in dark, the hero card stretches to the height of the right column, leaving a large empty middle with the button at the bottom, and the "Nothing due" box beside it is oversized.
- **Dark theme:** every dark page has a brownish radial glow at the top that Figma's flat `#090d1a` does not. In dark Me, the selected state of Light/Dark/Auto and English/Hebrew is only a faint fill. In the dark report sheet the unselected radios are low-contrast dark-grey rings. axe does not catch either. The disabled "Check" button looks muddy brown in dark.
- **Review and activities:** the flashcard front and back lack the frames' "Slipping, last seen 9 days ago" chip, a labelled "Hear it" button, the example sentence, and the interval labels ("10 min", "1 day") on the four grade buttons. Primary buttons are slate in light and gold only in dark and on the hero card, which matches Figma.
- **Hebrew keyboard at 360 px:** keys are about 30 px wide with the Latin hints overlapping the caps. Workable but tight.
- **Small:** the Today start button label wraps to two lines at 360 px. The Lessons page is almost empty with one lesson. Progress in Hebrew reverses the tab order, as expected for RTL. Progress default shows "0 words estimated known" above a chart that rises from 100 to 136; that is the fixture data, not a defect.
- **Sign-in has no gold main action** (Google's neutral button). This is a north-star deviation the owner may want to revisit.

## Follow-ups worth a decision

1. Sign-in has no gold action. Is that intended?
2. The older `TestShellScreenshots`, `TestActivityScreenshots` and `TestProgressScreenshots` and their `*-shots.mjs` files are now redundant with `TestShots`. They were left because earlier completion summaries reference them.
3. The Figma frames for Today, flashcards and the dark variants are out of date against the shipped app. Redraw them or retire them.
