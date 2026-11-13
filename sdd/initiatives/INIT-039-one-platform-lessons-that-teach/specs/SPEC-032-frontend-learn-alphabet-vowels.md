# SPEC-032 · frontend: Learn gets the alphabet and the vowels

## Metadata

```yaml
spec_id: SPEC-032
initiative_id: INIT-039
title: "Learn gets the alphabet and the vowels"
domain: frontend
status: complete
primary_prompt: none
complexity: medium
complexity_reason: two reference pages over a table that already exists, plus one authored vowel list
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 4 hours
blocked_by: []
blocks: []
```

## Description

Brian asked on 2026-11-12 for Learn to widen out to vowels, the alphabet, and the background he needs for Isaiah. Learn today is five guides and a list of pages; there is no page that shows the letters or the vowel marks, and Isaiah has no verses loaded, so it stays off.

The letter table is already in the app: `views.Alefbet` holds all 22 letters, the five final forms, the dagesh pairs, and the shin and sin forms, each with a name, a sound, and tell-apart notes. This spec gives it a page, and adds one page for the vowel marks.

## What to build

### `GET /learn/alefbet`

New `internal/web/views/alefbet_page.templ`, handler in `internal/web/learn_page.go`.

- Title "The alphabet".
- One sentence: the 22 letters, five of which change shape at the end of a word.
- A grid, in Hebrew reading order (right to left):
  - one cell per entry from `views.AlefbetForms()`;
  - each cell shows the letter large, its name pointed, its translit, and its sound;
  - a cell whose `Cue` is not empty shows the tell-apart line under the letter.
- Below the grid, one line naming the three dagesh letters (ב, כ, פ) and the two shin dots, each linking to its cell's anchor.
- Register `GET /learn/alefbet` before `GET /learn/{slug}` so the fixed path wins. The router already prefers the more specific pattern; test it.

### `GET /learn/vowels`

New data in `internal/web/views/vowels.go`, new template `internal/web/views/vowels_page.templ`.

- `type Vowel struct { Mark string; Name string; Translit string; Sound string; Example string; ExampleGloss string }`
- One table with these entries, written from the app's existing niqqud use. Every `Example` is a word already in the course list, pointed:

| Mark | Name | Sound |
| --- | --- | --- |
| ַ | pataḥ | a as in father |
| ָ | qamats | a as in father (o in some words) |
| ֵ | tsere | e as in bed |
| ֶ | segol | e as in bed |
| ִ | ḥiriq | i as in machine |
| ֹ | ḥolam | o as in more |
| ֻ | qubuts | u as in rule |
| ְ | sheva | a short e, or nothing |
| ּ | dagesh | doubles the letter, or hardens ב כ פ |

- Example words, all already on the course list: `אַתָּה` (you, pataḥ), `לֹא` (not, ḥolam), `הִיא` (she, ḥiriq), `הֵם` (they, tsere), `אֶת` (object marker, segol), `אֲנַחְנוּ` (we, ḥataf-pataḥ under the alef), `אַתֶּם` (you, dagesh in the tav).
- A note, in one short paragraph, that the app shows points on new words and most Israeli writing leaves them out.
- Register `GET /learn/vowels`.

### Learn index

- `internal/web/learn_page.go` `serveLearn` prepends two fixed cards to the groups: "The alphabet" and "The vowels", each with a cover from `guideCovers` and a link. They are not database rows, so a test checks they appear with no explainers stored.
- `views.LearnGroup` already has `Slug`, `Title`, `Summary`, `Example`, `Cover`; use `Href` if it exists, else add `Href string` and render it when set.

## Tests

New `internal/web/learn_reference_test.go`:

- `TestAlefbetPage`. The page shows all 22 letter names, `בּ`, `שׁ`, `שׂ`, and at least three tell-apart cues.
- `TestAlefbetBeforeSlugRoute`. `/learn/alefbet` returns the alphabet page, not a 404 from the slug route.
- `TestVowelsPage`. Every one of the nine marks appears, with its name and one example word.
- `TestVowelExamplesAreOnTheCourseList`. Each example word's `LemmaKey` is on the frequency list, checked with `speech.LookUp`.
- `TestLearnIndexShowsTheReferenceCards`. With no explainers stored, the index still shows "The alphabet" and "The vowels".

Then run the layout audit against `/learn`, `/learn/alefbet`, and `/learn/vowels`.

## Acceptance criteria

- [ ] `/learn/alefbet` shows all 27 forms with names and sounds and the tell-apart notes.
- [ ] `/learn/vowels` shows nine marks with names, sounds, and a pointed example word that is on the course list.
- [ ] The Learn index shows both pages even with no guides stored.
- [ ] The tests and the layout audit pass.

## Verification Strategy

- **Claim:** Learn explains the letters and the vowels from data already in the app.
- **Check + executor:** mechanized for presence and for the example words; the layout audit for the screens.
- **Pass condition:** all pass.
