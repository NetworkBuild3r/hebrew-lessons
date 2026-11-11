# ADR-0034: Stage 2, lessons built by code from your classes

**Status:** Accepted, 2026-11-11, by Brian Nelson.
**Initiative:** [INIT-039](../../sdd/initiatives/INIT-039-one-platform-lessons-that-teach/INIT-039-one-platform-lessons-that-teach-initiative.md).
**Source:** [BRAIN-002](../../sdd/brainstorms/BRAIN-002-lesson-types-done-right/decision-document.md), [Lesson types](../design/lesson-architecture.md).
**Supersedes:** the lesson-shape decisions in [ADR-0026](ADR-0026-lessons-that-build.md). Everything else in ADR-0026 stands.
**Builds on:** [ADR-0033](ADR-0033-stage-1-trust-and-measurement.md). Amends its A2 line (see the initiative).

## Context

Stage 1 made grades honest. The app still does not teach from the thing Brian actually studies, his tutor's classes. The tutor audit passed (286 usable lines, 10 of 10 classes loadable), and 742 of 974 class tokens are missing from the lexicon. The research on adult second-language learning (RSCH-003, RSCH-004, RSCH-005) supports four things: spaced retrieval on later days, producing the word with the Hebrew hidden, text where most words are already known, and time on task. Hand-written units and AI-written lessons cannot be proven right, which is how Brian lost trust in them.

## Decision

**D1. Code builds every lesson.** The shape, the words, the questions and the keys come from code. The AI may write at most one optional story per lesson, and nothing it writes is graded. An outside AI on MCP cannot change a lesson's shape because it never supplies it.

**D2. Words come from a ranked pool of tutor-attested words, "T0".** A word is T0 when the tutor's own sheet shows it with its English gloss beside it. The pool spans every loaded class sheet, not only the newest, because Brian often studies days after a class. A sheet joins the pool only after it passes its own check. Ranking prefers words seen in more classes, words in held lines, and words Brian has missed.

**D3. Every graded key is T0.** There is no T1 source. A word with no checked lexicon entry cannot be a key. An entry check runs before a word can be asked: pointed form, gloss, part of speech, and a consonant form that does not collide with a different taught word.

**D4. Two lessons ship first.**
- *Word lesson:* about 4 new words, a grid with Listen, a meaning check with an unrecorded second ask on a miss, a passage of the tutor's own lines (up to 3, about 90 percent taught words), typed Write with the Hebrew hidden, then building that stored sentence. The sentence is one recording of the whole line, played after the check (RSCH-005: the sentence comes after the spelling, and it is not three word clips).
- *Easy lesson:* no new words. A passage of words being learned, then the same checks. Today offers it when too many words are still in learning or recent accuracy is low.
Conjugation is the next initiative. Listening waits for a voice Brian accepts. The spelling lesson is cut, because typed Write does that work (RSCH-005).

**D5. Today puts review first, with a soft cap of about 12 to 15 minutes, then reserves one lesson slot.** It labels the lesson with the class its words came from. If no lesson passes its checks it says so plainly and still offers review.

**D6. One flag control.** After any graded answer, one tap voids the grade, flags the item and suspends it from lessons and review, with Undo. It works on right answers too. It is the safety net that replaces new validator rules.

**D7. Release is judged by three tests, not nine gates** (INIT-039, "Done when"). No validator rule is added. Nothing is deleted. The old player, validator, units and unit tools stay until the new lessons have been on Today for three weeks, then a separate retirement initiative archives units 12 to 34, never deletes them.

**D8. Retire rule and tripwire.** If after 28 days of use the word lesson has not produced more study minutes and a better "proved" share than review alone, or if Brian reports the lessons "feel the same" on two weekly readouts in a row, the lesson type is cut and review stays. The weekly readout (SPEC-009) reports both numbers, and says "too early" while counts are small.

## What this does not decide

Speech scoring (the app does not score speech; speaking happens with Brian's AI and tutor, and a prep page serves it). A second word source. The voice. These stay deferred.

## Consequences

- The class sheet and its lines become data (`class_sheets`, `class_lines`), stored verbatim with the tutor's text and Brian's text in separate fields.
- `record_taught_words` gains a required sheet reference. It stays until class import is proven.
- Attempts gain a nullable `run_id`, so a resumed lesson still groups correctly.
- The old unit pipeline keeps running beside the new one, so there are two ways to make a lesson for about three weeks. That is accepted to avoid deleting anything.
- Words that Brian types in Write are graded with the INIT-038 fold, which accepts the everyday spelling and rejects a different word.

## Alternatives considered

- **Keep AI-written units under a stricter validator.** Rejected by Brian (ASMT-006, Part B). A validator can only reject what it was told to look for.
- **Build all five lesson types at once.** Rejected: listening has no voice, spelling duplicates Write, and conjugation needs the word lesson proven first.
- **Read Drive from the server.** Rejected for now: it adds a credential and an external part for no learning gain.
