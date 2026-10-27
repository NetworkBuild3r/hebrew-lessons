# ADR-0020: The face of a lesson word, and grading by speed

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-27 |
| **Authored by** | INIT-022 SPEC-004, recording the lesson-player change reviewed the same day |
| **Initiative** | INIT-022 (recall-speed), SPEC-004 |
| **Extends** | [ADR-0018](ADR-0018-v3-adaptive-learning.md) (grading of a checked answer) |
| **Supersedes** | The North Star sentences that put the Latin reading on the face of every taught word, and that gave Easy for any fast right answer |
| **Superseded by** | — |

## Summary

A lesson word is pointed Hebrew on its face. The Latin reading and the meaning sit on the back. A meaning question is timed, and that time changes when the word comes back: a fast right answer waits longer, and a slow one comes back sooner. Easy is withheld until the card has left its first steps, so a word answered quickly because it was just taught is not sent weeks away.

## Decisions

Each decision names the option chosen and one option rejected.

**D1 · The face is pointed Hebrew.** On a lesson card the front is the pointed word. The Latin reading and the meaning open on a tap, and a browser without script still opens them. A fill-the-gap line stays pointed Hebrew until the answer. A word being tested shows neither its reading nor its meaning until then. A help word in a line may show its meaning during the question, and its reading stays on the back. Rejected: printing the Latin reading beside every Hebrew word for the whole lesson.

**D2 · A meaning question is timed, and speed moves the card.** The clock starts when the question is shown. Wrong is Again. Right and faster than the learner's median for that activity is Easy, once ten timed right answers exist. Right and slower than twice that median is Hard. Anything else right is Good. An answer longer than a minute is stored as untimed, so a closed tab is not a slow recall. Rejected: treating every right answer as Good and ignoring the time.

**D3 · Easy waits until the card is in review or relearning.** A fast answer on a missing card, a new card, or a card still in its first steps is stored as Good. The first Good on a new card comes back in minutes. Easy, which comes back in weeks, waits until a later review. Rejected: giving Easy on the first fast answer, which scheduled a word just taught for 16 days.

**D4 · The agent sees the latest timed right answer for that skill.** `recallMs` is that time in milliseconds. `recallSpeed` is fast, steady, or slow against the same median, and stays empty until ten timed right answers exist. A wrong answer is not counted, and a meaning time does not appear on the card for another skill. Rejected: reporting the latest attempt of any kind on every card for the word.

## Consequences

- The North Star sections 2, 4.2, and 6.1 follow this record.
- Fading the Latin reading once a word is known, and fading the vowel points later, stay future work. This record does not turn them on.
- The validator still requires a transliteration and a gloss to be stored for every Hebrew word. What changed is when the player shows them.
