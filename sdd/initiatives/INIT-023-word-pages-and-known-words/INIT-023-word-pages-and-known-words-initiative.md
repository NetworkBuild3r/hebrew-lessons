# INIT-023: Word pages, working play buttons, and words that count as known

**Initiative ID:** INIT-023 · **Status:** on_track · **Created:** 2026-10-27 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** Brian's review of Progress and Lessons on 2026-10-27 (three screenshots), and the learner data read over MCP the same day.

## What we found

**Play buttons on Progress.** The "You can now say" and "Almost there" rows render a bare play button with `data-src` and no `<audio>`. `clips.js` finds a clip through `data-audio` or the enclosing `.clip`. It finds neither, so the first tap paints the error state, the speaker with an X. The Isaiah verse button (`isaiah.templ`) has the same fault.

**Zero known words.** Learner 1 has 35 words with evidence. By skill:

| Skill | Seen | Learning | Known or solid | Struggling |
| --- | --- | --- | --- | --- |
| Read | 6 | 17 | 11 | 1 |
| Recall | 6 | 25 | 2 | 0 |
| Listen | 29 | 0 | 0 | 0 |
| Spell | 14 | 0 | 0 | 0 |

There are three causes, and they stack:

1. Lessons never make a listening card. `enqueueNewWords` (`internal/db/lesson.go`) gives each new word a `read` card and a `recall` card. A lesson `listen_pick` sets `SkipSchedule`, so its answer is stored with no card. With no card the state rule stops at `seen` (rule 4 of `StateOf`). Listening can never reach learning or known. אֲנִי has 3 listen answers and stays `seen`.
2. The overall state is the weakest core skill with evidence, and an answered skill with no card counts as `seen`. אֲנִי is known in read and recall and shows `seen` overall. Every word with a lesson listen answer is held at `seen`.
3. Progress opens on the Listen tab, the one skill that cannot move. Read shows 11 known words on its tab.

Known also needs a card with 3 successful spaced reviews and the newest 3 answers right. Most recall cards are at `learning` because their reviews are still due. Finishing reviews moves them.

**Verb and noun forms.** `verb_forms` (migration 014) already holds tense, person, number, and gender, and `upsert_verb_forms` writes them. They show only on `/me/verbs` as a flat list, with no audio and no tense grouping. Lexemes carry `pos`, `root`, `binyan`, `gender`, and `plural`. There is no store for noun construct or possessive forms, and no page for a single word.

## What you get

- Every play button on Progress and Isaiah plays. A word with no clip shows no button.
- Listening answers in lessons build a listening card, so listening moves from seen to learning to known. Past listen answers are replayed into cards once.
- Progress opens on an overview with the known count for every skill side by side. A skill that cannot move yet says why.
- A page for each word at `/words/{id}`:
  - **Verbs:** infinitive, root, binyan, and meaning at the top. The present tense is a four-column grid (m. sg., f. sg., m. pl., f. pl.). Past and future are grids by person, number, and gender, followed by the imperative and the infinitive. Every cell plays its own clip and marks the forms you know. Each tense is labelled. Lines from your lessons that use any of the forms show under "In your lessons".
  - **Nouns:** gender, singular and plural (irregular marked), the construct forms, the definite form, and the possessive forms (my, your, his, her, our, your pl., their). Every form plays.
  - **Adjectives:** the four agreement forms, in the same grid as the present tense.
- Word cards, the Words list, the Verbs page, and Roots link to the word page.
- The authoring agent can write noun and adjective forms over MCP, and `lookup` returns a word's forms.
- The verbs and nouns already taught have their forms and audio, starting with לְהַחְלִיט, לִבְחוֹר, לְהַאֲמִין, לְנַצֵּחַ, and לְלַמֵּד.

## Specs

| Spec | Domain | Title | Blocked by | Effort | Status |
| --- | --- | --- | --- | --- | --- |
| [SPEC-001](specs/SPEC-001-frontend-play-buttons.md) | frontend | Play buttons that carry their audio | — | 1h | complete |
| [SPEC-002](specs/SPEC-002-service-listen-cards.md) | service | Lesson listening answers build a listening card | — | 3h | complete |
| [SPEC-003](specs/SPEC-003-service-listen-replay.md) | service | Replay past listening answers into cards once | SPEC-002 | 2h | complete |
| [SPEC-004](specs/SPEC-004-frontend-progress-overview.md) | frontend | Progress opens on every skill at once | — | 2h | complete |
| [SPEC-005](specs/SPEC-005-data-noun-forms.md) | data | Store noun and adjective forms | — | 2h | complete |
| [SPEC-006](specs/SPEC-006-mcp-word-forms.md) | mcp | Write and read word forms over MCP | SPEC-005 | 3h | complete |
| [SPEC-007](specs/SPEC-007-frontend-verb-page.md) | frontend | Word page with the verb view | SPEC-001 | 6h | complete |
| [SPEC-008](specs/SPEC-008-frontend-noun-page.md) | frontend | Noun and adjective views | SPEC-005, SPEC-007 | 5h | complete |
| [SPEC-009](specs/SPEC-009-content-forms-and-audio.md) | content | Forms and audio for the words already taught | SPEC-006, SPEC-007 | 3h | blocked |
| [SPEC-010](specs/SPEC-010-docs-states-and-word-page.md) | docs | Record the listening card and the word page | SPEC-002, SPEC-004, SPEC-007 | 1h | complete |

**Total:** 28h. Two tracks can run side by side: the known-words track (SPEC-002, SPEC-003, SPEC-004) and the word-page track (SPEC-001, SPEC-005 to SPEC-009). SPEC-010 closes both.

## Decisions to confirm

- **Overall state stays strict.** The North Star (section 4.3) defines a word as known when its weakest core skill is known. This plan keeps that rule and fixes listening so it can advance. The overview in SPEC-004 shows each skill's count so read and recall progress is visible today. The alternative is to leave answered skills with no card out of the overall state. That is a North Star change and is not planned here.
- **Replay instead of a flood of reviews.** SPEC-003 builds listening cards from the answers already given, so the cards come out at the right point in their schedule. Seeding all 29 as new cards due now would add 29 reviews today.
- **Daily snapshots stay as recorded.** The estimated-words chart keeps the zeros it stored. New days use the fixed states.

## Depends on / unblocks

- Sits on the uncommitted word-card redesign. Run `templ generate` and `go test` on that work before starting.
- SPEC-005 adds migration 019.
- Unblocks the lesson author asking for full forms whenever a verb or noun is introduced.
- The noun half of SPEC-009 moved to INIT-024. The verb half stays here.

## Done when

- Tapping a word under "You can now say" plays it. No orb on Progress or Isaiah reaches the error state when its clip exists.
- A right `listen_pick` answer in a lesson creates or moves the word's `listen` card.
- After the replay, אֲנִי has a `listen` card and its listen state is `learning` or better.
- Progress shows read, recall, listen, and spell known counts on first load.
- `/words/{id}` for לִבְחוֹר shows the present grid בּוֹחֵר, בּוֹחֶרֶת, בּוֹחֲרִים, בּוֹחֲרוֹת, and each cell plays.
- `/words/{id}` for a noun shows singular, plural, construct, and possessive forms, and each plays.
- `go test ./...` and the browser checks pass, and `templ generate` leaves no drift.
