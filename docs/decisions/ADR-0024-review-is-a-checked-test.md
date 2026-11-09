# ADR-0024: Review is a checked test

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-11-02 |
| **Authored by** | INIT-028 SPEC-001 |
| **Initiative** | INIT-028 (review-as-a-test), SPEC-001 |
| **Extends** | [ADR-0020](ADR-0020-lesson-word-face-and-speed.md) (speed rule), [ADR-0023](ADR-0023-review-that-moves-on.md) (review that moves on) |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

A review sitting is a checked test of a word the learner already knows. Each card asks one thing the server can grade. Forgot, Hard, Got it, and Easy stay on a flashcard sitting only. Today's review link opens the test. A word and skill are asked once per sitting; a miss shows the right answer and waits for a later day. A missing-letter step is shown only when the asked letter is actually absent from the prompt.

## Context

INIT-027 / [ADR-0023](ADR-0023-review-that-moves-on.md) ended letter drills that never clear, stopped padding a sitting with cards that are not due, limited typing-slip pairs, moved a strong word up with implied credit, capped new words by accuracy, and tied the sitting to the learner's study day. Those rules stand. This record does not reopen them.

The sitting after that change is still a flashcard: Show answer, then Forgot, Hard, Got it, and Easy. The learner reports how it felt; the app does not check an answer. A miss can return in the same sitting. A missing-letter step can show a complete word when the asked letter cannot be blanked, so a visible letter can be the wrong pick.

RSCH-003 supports retrieval over rereading (Moderate) and spacing over massing (High). Checklist line 173 says a same-session repeat is practice, not the review. Recognition versus recall sits at Low: the sources conflict. The checked-test format for a review sitting is Brian's product decision, not a research finding that requires multiple choice.

## Decision

**D1 · Review is a checked test.** A review sitting tests a word the learner already knows. Each card asks one thing the server can grade: the meaning, the Hebrew, the form, or a missing letter. Forgot, Hard, Got it, and Easy are not on this screen. The schedule still moves from the answer: wrong is Again, right is Good, right and fast is Easy, right and slow is Hard, using the speed rule in [ADR-0020](ADR-0020-lesson-word-face-and-speed.md). The learner never picks those words. Rejected: a review screen whose grade is the learner's self-rating.

**D2 · Flashcards are a different sitting.** Show answer and the four self-grade buttons (Forgot, Hard, Got it, Easy) stay on a flashcard sitting. Today's review link opens the checked test, not the flashcard. Rejected: one sitting that is both the review and the self-graded flashcard.

**D3 · Once per sitting.** A word and skill are asked once per sitting. A miss shows the right answer and is scheduled for a later day. It does not return in the same sitting. Same-session repeats are practice, not the review (RSCH-003 checklist line 173). Spacing beats massing (RSCH-003, High). Rejected: putting a missed card back into the same sitting after a short gap.

**D4 · Blank gate.** A missing-letter step is shown only when the asked letter is actually absent from the prompt. If the example cannot be blanked, the step is skipped and no grade is written. A choice that is still visible in the prompt is not offered. Rejected: serving a complete word as a missing-letter question, and rejected: offering a letter the learner can still see in the prompt.

## Consequences

- North Star sections 6 and 12 follow this record for the review sitting; flashcards remain their own activity.
- ADR-0023's drill, due-card, implied-credit, new-word, and study-day rules are unchanged.
- ADR-0020's speed rule for checked answers is unchanged; review answers use it without learner-chosen grade words.
- Recognition versus recall remains Low in RSCH-003; later product work may add typed recall without overturning D1.
- Later code specs (INIT-028) implement one question per word-and-skill, the blank gate, and the checked review screens against this record.
