# SPEC-011 completion summary

**Spec:** SPEC-011 · Restyle learner views (Today, Lessons, Guides, Words, Review, Progress, player chrome)  
**Initiative:** INIT-005  
**Completed:** 2026-10-12  
**Status:** in_review (self-checked; no independent verifier ran, review depth is light)  
**Provenance:** `INIT-005/SPEC-011`

## What the look is

ADR-0010 is the adopted look (warm paper `#faf8f4`, white 18px cards, 14px tactile buttons that press down, 3px focus ring, informative feedback, no points, streaks, badges, jokes or mascot). The three-prototype contest (SPEC-007) is superseded, so ADR-0008 is not applied. This spec closes the gaps that kept the learner screens from reading as one product: the dark navy shell header, the blue banner on every page heading, and a few views that still had the old 2 px look.

## Header (the known issue)

Measured in headless Chrome with the repo's CDP approach (`spikes/restyle-shots/shoot.js`, not served), sticky shell `.hebrew-shell` height:

| Viewport | Before | After |
|---|---|---|
| 360 px | 205 px | **93 px** |
| 600 / 768 / 1099 px | 205 px | 93 px |
| 1100 px and 1280 px | 205 px (brand, nav in two rows, account button stacked) | **61 px**, one row |

- At 1100 px and up: brand on one side, seven nav links, account pill on the other. All seven links (Admin included) fit.
- Below 1100 px: row 1 is brand and account, row 2 is the nav as one row that scrolls sideways. The next link peeks out at the edge and fades (the cue that it scrolls). `app-shell.js` now scrolls only the nav, never the page, so the current link is in view (`revealCurrentLink`, a no-op where nothing overflows).
- Every link keeps its Hebrew word, its English gloss (now under the word, `lang="en"`), `data-nav`, and `aria-current="page"`. The current link also has a heavier weight and an underline, so it is not colour alone.
- Targets: nav links 44 px, account button 44 px (40 px on a phone), icon buttons and chips 44 px, dense table buttons 36 px, nothing under 24 px.
- Below 1100 px the shell scrolls away with the page (it is no longer sticky); at 1100 px and up it stays sticky. On a phone the lesson player's control row (exit, step, sounds, vowels) is the one sticky bar, so an exercise keeps its height.

## Before and after per screen (all looked at as PNGs at 360 and 1280)

- **Today.** Before: navy header over a blue gradient banner with a "Hebrew Way" kicker, English blurbs with the full stop on the wrong side. After: paper ground, plain heading with a soft icon chip and the English gloss under the subtitle, white cards with the level accent, one primary button per card, English blurb in muted `lang="en" dir="ltr"`, level shown as "בסיסי (Basic)" instead of the raw word "basic".
- **Lessons.** Before: banner, boxy filter chips, raw "intermediate · intermediate" on the private lesson. After: pill filters (selected one has a 2 px border and bold text), consistent cards, the duplicate raw level gone. At 360 px the four chips take two rows.
- **Guides.** Before: a floating card. After: dashed empty state matching the other empty states, gloss in brackets. (No published guides exist in the test data, so a populated Guides shelf was not looked at.)
- **Words.** Before: unaligned selects, cramped two-column layout inside a 40 rem page. After: the page is wider (60 rem), filters sit in one card as an aligned grid with the search full width, the add-a-word form is a grid, empty state is dashed. At 360 px it is a single column with no sideways scroll.
- **Review.** Before: square 2 px deck buttons with a dark-green selected one, gloss glued to the Hebrew ("מעודכן!(All caught up!)"). After: pill deck buttons with count badges and a blue selected state, selects fill the row, the view toggles wrap, no overflow, space before each gloss. The review card and session flow (INIT-007) were not changed; with no due words only the "all caught up" card could be looked at, not a live flashcard.
- **Progress.** Before: AI profile dumped as unstyled text. After: stage bars and counts in cards; the AI panel is a quiet reference card (shaded disclosure note, divided sections, compact lists). Same data, same hooks. Looked at with an empty account, so the filled stage bars and accuracy were not seen in Chrome.
- **Lesson player chrome.** Before: blue banner under the navy header (about 205 + 165 px of chrome at 360 px). After: same paper surface as the shell; the control row sticks and the title and round lines scroll away; path list, primary and "practice again" buttons use the shared button look. The in-lesson activities were already restyled (INIT-007); a Match step was looked at and is unchanged. Not looked at: Story, Speed quiz, Build a sentence, Pattern, Ask a Friend, or a live miss and correction.
- **Dark theme.** Looked at Lessons and Words with `data-theme="dark"` set by script: readable, no UA dark controls. Not exhaustively checked, and the app stays light by default.

## REQ-016 (one language per label)

Fixed: the "דרך עברית • Hebrew Way" kicker (removed from `layout.js`); English-only `aria-label`s (Lessons level filter, Progress stage bar, Words strength bar, Review typed-answer box, Admin tabs) are Hebrew; lesson level is a Hebrew word with a gloss on the Lessons and Today cards and on the player path screen; the Progress empty-state sentence; the Review gloss spacing. Left alone on purpose: English lesson blurbs and subtitle fragments are lesson data (the blurb is now marked as English gloss); `Guide` chip text is asserted by a test.

## Files changed

`shared/worksheet.css` (one new block `/* === INIT-005/SPEC-011 restyle learner === */`, appended at the end; the "Light by default", Words polish, route transition and icons blocks are untouched), `shared/layout.js`, `shared/app-shell.js`, `shared/lessons-page.js`, `shared/today-page.js`, `shared/review-page.js`, `shared/progress-page.js`, `shared/words-page.js`, `shared/guides-page.js`, `shared/player/core.js`, and the SPEC-011 spec file. No test file was edited, no `worksheets/*/index.html` was touched, no dependency or build step was added. Spikes: `spikes/restyle-shots/` (throwaway screenshot and splice scripts, untracked, not served).

## Verification

- `npm test` with `SRS_SCHEDULER=ladder`: **1040 pass, 0 fail** (same count as before; a first run caught one assertion, the Progress empty-state test that wants "(Today)" and "(Lessons)" beside the links, and the sentence was reworked to keep them rather than editing the test).
- Chrome smokes, all exit 0: admin, admin-lessons, ai-profile, audio, flashcards, guides, learner-support, navigation, production-recall, progress, review, signin, story-gloss, today-lessons, today-mix, verb-workshop, words.
- No horizontal overflow at 360, 600, 768, 1099, 1100 and 1280 px on the screens captured. One screenshot run showed a one-off `router.js` boot error (`HebrewLayout` undefined) on one page load; three reruns of that screen and a further full run were clean, so it was not reproduced.

## Not closed

- Reduced motion: no new animation was added; hover lift and press transforms are switched off under `prefers-reduced-motion`. Only checked by reading the CSS, not with emulated reduced motion in Chrome.
- The lesson-complete moment from the spec's scope is not new work here (SPEC-009 left it unbuilt; ADR-0010 forbids confetti).
- A few fixed hex values remain for the green and amber button fills and two progress-bar segments.
- No independent reviewer ran.
