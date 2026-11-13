# ADR-0035: five sections, one job each

**Status:** Accepted, 2026-11-12, by Brian Nelson.
**Initiative:** [INIT-039](../../sdd/initiatives/INIT-039-one-platform-lessons-that-teach/INIT-039-one-platform-lessons-that-teach-initiative.md).
**Builds on:** [ADR-0034](ADR-0034-stage-2-lessons-built-from-your-classes.md).
**Amends:** ADR-0034's sections, not its lesson decisions.

## Context

On 2026-11-12 Brian opened `/lessons` and found four class-sheet units from the old pipeline, and no lesson that walked the common words. The course built in SPEC-021 to SPEC-027 lived only inside review, so the app had no visible path from the most common words outward. He asked for five sections with one job each: Lessons teaches the words, Tutor holds what his tutor taught, Review brings back what he missed from either, Progress shows the whole picture, and Learn widens out to the alphabet, the vowels, and the background for Isaiah.

## Decision

**D1. Lessons is the course.** The signed common words form numbered lessons of five, in frequency order. A lesson is studied in the review player; there is no second player. A prefix row is not a lesson word, because it is taught inside a line. Only a lesson whose words are all in the database is listed, so the number a learner sees is a lesson he can open.

**D2. Tutor is the class.** The class sheets loaded over MCP are listed, marked against the course as already had, new, or not on the list, and studyable. The page never invents a class: with nothing loaded it says so and names the one tool call that loads one.

**D3. Review mixes both and names the source.** A card says Common words, From your tutor, or From an earlier lesson. The course wins over a sheet when a word is on both, because the course is what ranks it. Ordering, counts, grading, and requeue do not change.

**D4. Progress states the mix, then the detail behind it.** The brief view keeps one can-do sentence and adds three short lines and three links. The detail view behind `?detail=1` is unchanged. No charts, percent bars, streaks, or points in the brief view.

**D5. Learn is reference, not a course.** The alphabet and the vowels are pages over data already in the app. A guide appears only when there is a guide; Isaiah stays off until its verses are loaded. The alphabet and vowel pages are fixed pages, so they show without a stored guide.

## What this does not decide

Conjugation, the listening lesson, a voice, speech scoring, and retiring the old class-sheet units. The old units stay in the database; they are simply not what Lessons lists.

## Consequences

- `/lessons` no longer lists the old class-sheet units. Tutor carries the classes themselves.
- Tutor is the one section that shows while empty, because it has an instruction to give.
- Three POST routes are added: study a course lesson, study a tutor class, and practise the slipping course words. Each is behind the session and the CSRF token, and each reads its words from the server, never from the form.
- A tutor class can add words to the shared lexicon. A spelling already shared by two stored rows is skipped rather than guessed.
- Nothing is deleted. `db.LearnerUnitProgress` and the unit player stay for the later retirement group.
