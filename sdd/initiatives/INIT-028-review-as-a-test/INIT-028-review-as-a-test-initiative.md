# INIT-028: Review is a test of what you know

**Initiative ID:** INIT-028 · **Status:** on_track · **Created:** 2026-11-02 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main`
**Source:** Brian, 2026-11-02, from a live review sitting. The open word card for אֵת showed the Hebrew, "at", "you", and Word details, and did not say what kind of word it is. The same sitting asked for a missing letter while the word on screen was complete. Picking a consonant that was visibly in that word was marked wrong. The same question came back two cards later. The grade buttons were Forgot, Hard, Got it, and Easy. He wants review to be a test, and the self-grade buttons to stay on flashcards only. Evidence: [RSCH-003](../../research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md) — retrieval beats rereading (Moderate), spacing beats massing (High), a same-session repeat is practice and not the review (checklist line 173, already used by INIT-027), and recognition versus recall is an open question (Low). The format of a review sitting is his decision, not a research finding.

This plan is written in the current checkout. It does not start the work.

## What we found

INIT-027 made letter drills end and stopped padding a sitting with cards that are not due. The sitting is still a flashcard. `flashFrame` is the frame for every review mode except a two-letter pick, and after Show answer the buttons are Forgot, Hard, Got it, and Easy (`internal/web/views/activity.templ`, `GradeLabels`). The learner reports how it felt. The app does not check an answer.

A letter step that should hide one letter calls `blankAskedLetter` (`internal/web/learn.go`). When the asked letter is not an exact slice of the example, the function returns the example unchanged. The screen then shows the whole word, including the letter the question is about. The buttons are still the pair, so a letter the learner can see in the word can be the wrong button.

A miss is put back into the same sitting after `RequeueGap` cards, which is 3 (`internal/session/mix.go`). That is the "two questions later" repeat.

Gender on an open word card is the stored `m` or `f`, and only inside Word details (`CardMeta` in `internal/web/views/player.go`). The face of the card is the Hebrew and the English. A word with no gender, such as the particle אֵת, looks the same as a noun whose gender was left off.

## What you get

- A review sitting is a checked test. Each card asks one thing the learner can get right or wrong: the meaning, the Hebrew, the form, or the missing letter. The app grades it. Forgot, Hard, Got it, and Easy are not on this screen.
- Those four buttons stay on a flashcard, and a flashcard is not the review sitting.
- The schedule still moves. A wrong answer is Again. A right answer is Good, or Easy when it was fast, or Hard when it was slow, using the speed rule that already exists. The learner never picks those words.
- A word is asked once per skill in a sitting. A miss shows the right answer and waits for a later day. It does not return two cards later.
- A missing-letter question shows a real blank. If the asked letter cannot be removed from the example, that step is not served. A choice that is still visible in the prompt is never offered.
- The open word card says masculine or feminine in English when the word has a gender, on the face, beside the meaning. A word with no gender says what it is, such as preposition, in the same place. Word details stays for the rest.

## Specs

| Spec | Domain | Title | Blocked by | Effort | Status |
| --- | --- | --- | --- | --- | --- |
| [SPEC-001](specs/SPEC-001-docs-review-is-a-test.md) | docs | ADR: review is a checked test | — | 1h | complete |
| [SPEC-002](specs/SPEC-002-service-one-question-real-blank.md) | service | One question per word, and a blank that is really blank | SPEC-001 | 4h | planned |
| [SPEC-003](specs/SPEC-003-frontend-checked-review.md) | frontend | Checked review screens, and gender on the word card | SPEC-001, SPEC-002 | 5h | planned |

**Total:** 10h. **Critical path:** SPEC-001 → SPEC-002 → SPEC-003 = 10h.

**Order.** SPEC-002 changes what a sitting contains and how an answer is graded. SPEC-003 is the only spec that changes the review screen and the word card.

## Decisions

- **Review and flashcards are different sittings.** Review is the test. Flashcards keep Show answer and the four self-grade buttons. Today’s review link opens the test.
- **No same-sitting repeat.** `RequeueGap` does not apply to a review test. A wrong answer is scheduled and the sitting continues.
- **The blank is a gate.** `blankAskedLetter` returning the original text means the step is skipped, not shown. The served prompt and the graded letter are the same string.
- **Gender is a word, on the face.** "masculine" and "feminine", not `m` and `f`, and not only inside Word details. No gender means the part of speech in English, in that same spot.
- **Build on a known word.** A review question uses a word the learner has already met. The new part is the skill that is weak: meaning, reading, gender, form, or one confusable letter. A letter drill is a blank inside a word they know, not a bare letter and not a complete word.

## Threat model

- **No new trust boundary.** The signed-in learner posts their own review, through the existing CSRF- and Origin-checked review handlers. The learner id comes from the session.
- **Input.** A choice posts the option id the server rendered. Right or wrong is decided against the answer stored when the card was served. A posted letter that was not one of the served choices is 400 and writes nothing.
- **Integrity.** Skipping a step that cannot be blanked writes no grade. A sitting does not contain the same word and skill twice.
- **Output.** Screens stay templ-escaped.

## Depends on / unblocks

- Builds on the checked lesson activities (recall, produce, cloze), the speed rule (INIT-022, ADR-0020), the due-card mix (INIT-027, ADR-0023), and the word card (the open panel on the page).
- Out of scope: changing FSRS weights, a new word source, noun paradigms (INIT-024), and putting the four self-grade buttons back onto the review test.

## Done when

- A review card has choices or a real blank, and no Forgot, Hard, Got it, or Easy button.
- A missed card does not appear again in that sitting.
- A letter question never shows the asked letter in the prompt, and picking a letter that is still visible cannot be the graded answer.
- An open noun card shows masculine or feminine next to the meaning. אֵת, which has no gender, shows what kind of word it is in that same place.
