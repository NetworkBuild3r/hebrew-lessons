# INIT-040: Teach the language, not only the words

**Initiative ID:** INIT-040 · **Status:** in progress · **Created:** 2026-11-12 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main` (in place)
**Builds on:** [INIT-039](../INIT-039-one-platform-lessons-that-teach/INIT-039-one-platform-lessons-that-teach-initiative.md), [ADR-0034](../../../docs/decisions/ADR-0034-stage-2-lessons-built-from-your-classes.md), [ADR-0035](../../../docs/decisions/ADR-0035-five-sections.md)

## Brian's words

> "I want to see this come to life and help me actually learn Hebrew. Focusing on the most common words and working our way through. Having training teaching the vowels, how they work. Teaching me verb conjugations and how it works for each verb type. Biblical Hebrew and how it works. The ways of the Jews, teaching me the ways of the Jew to help me understand Isaiah."

And, on the look of it:

> "That looks like shit. Let's make this look like somebody actually cared."

## What was wrong

1. **Today's one action repeated itself.** The heading said "Today's session, about 20 minutes" and the button under it said the same thing again. A card that says one thing twice looks careless even when it works.
2. **The hero was a wide empty slab.** A desktop minimum height made the card 320px tall for two lines of text, so the page was mostly blank.
3. **The counts were a sentence.** "0 Words you know" sat as loose text instead of reading as a figure with a label.
4. **Two sections shared one icon.** Tutor and Learn both drew the same book.
5. **Nothing taught the language.** Learn had the alphabet and the vowels. It did not explain the binyanim, and nothing explained what Isaiah assumes.

## What this initiative built

### Today, rebuilt

- The heading is the page name with a can-do line under it. The counts are a figure over its label, separated by a hairline, and aligned to the heading baseline.
- The session card carries three things in order: a small kicker with the length, a real title that says what the sitting holds ("11 cards to review" or "Learn 5 new common words"), and one line of why. The gold button says only what it does: "Start".
- On a wide screen the card keeps its own height and a readable width, so it no longer stretches to fill empty space.
- Beside it, and under it on a phone, is a quiet column: the words that are slipping, and two links into the sections that explain what the sitting is doing. Links only, so the session stays the one gold action.

### Verbs: the binyanim, taught from real data

`internal/speech/binyan.go` reads the 853 ready verbs in the signed course list. Every verb there carries a root, a binyan, and the spellings the dictionary lists for it.

- `/learn/verbs` explains the three-root idea, then the seven patterns in teaching order, each with what it does, how it changes the root, one real course verb, and how many verbs of that pattern the course holds.
- `/learn/verbs/word/{lemma}` shows one verb: its root, its pattern, the forms the course list actually holds for it, and the past, present and future rule tables that build the rest.
- **Nothing is guessed.** A form is only shown when it is in the signed row. The tense labels are read from the endings the data holds, so a verb never claims a tense its forms do not cover. `internal/speech/binyan_test.go` asserts this against every one of the 853 verbs.

### The ways of the Jews: reference for Isaiah

`/learn/customs` is the context a reader needs for the prophet, in four groups: Time (Shabbat, Pesaḥ, Shavuot, Sukkot, Yom Kippur, Tish'a b'Av), the Temple (the Temple, korban, kohen, navi), the Prophets (Nevi'im, haftarah, teshuvah, tzedek, mishpat), and Words (she'ar yashuv, kadosh, go'el, or, derekh). Each entry says what the practice is and why it matters for reading Isaiah. It is reference only, never graded, and says so on the page.

### The look

- No repeated label anywhere on Today.
- The session card is content-sized, not a fixed slab.
- Counts are figures, not prose.
- Tutor and Learn now draw different icons.
- Everything uses the Lamplight tokens. No literal colour, no physical property, checked by the CSS linter.

## Figma

A new page, **INIT-039 — Five sections**, holds the board: the five sections side by side, the Today redesign at phone width, and the Learn and Verbs pages. It is built on the file's own Lamplight variables (`bg/paper`, `bg/surface`, `bg/slate`, `accent/lamp`, and the radius and space scales), so it stays linked to the same tokens the code uses rather than copying colours.

## Done when

1. `go test ./...` passes, including the new `internal/speech` checks and the web page tests.
2. Every form on every verb page is a form in the signed course row. A test holds this.
3. Today renders one gold action, no repeated label, and no sideways scroll at 360, 390, 768 and 1440 px.
4. The layout audit reports no issues on `/`, `/learn`, `/learn/verbs` and `/learn/customs`.

## Not in this initiative

Conjugation *practice* (graded drills on a verb's forms), the listening lesson (still needs a voice), speech scoring, and retiring the old unit pipeline. Those are later initiatives.

## Constraints

- **Nothing is deleted.** Units 1 to 34, the old player, and the old validator stay.
- **No production deploy.** Every check is local.
- **No invented Hebrew.** A page shows a form only when the signed data holds it.
- **Tokens only.** The CSS linter and the token tests enforce it.
