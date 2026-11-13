# SPEC-034 · docs: the five sections, recorded

## Metadata

```yaml
spec_id: SPEC-034
initiative_id: INIT-039
title: "The five sections, recorded"
domain: docs
status: complete
primary_prompt: none
complexity: low
complexity_reason: one ADR section and the initiative table
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 1 hour
blocked_by: [SPEC-029, SPEC-030, SPEC-031, SPEC-032, SPEC-033]
blocks: []
```

## Description

The app now has five sections with one job each. That is a product decision, and the next agent to open the code should find it written down next to the lesson decisions it changes. This spec records it and closes the initiative's course work.

## What to change

### `docs/decisions/ADR-0035-five-sections.md` (new)

- **Context:** the Lessons page listed four class-sheet units; the common-word course lived only inside review, so nothing visible walked the word list. Brian asked on 2026-11-12 for five sections with one job each.
- **D1. Lessons is the course.** Numbered lessons of five signed common words, in frequency order. A lesson is studied in the review player; there is no second player. A prefix row is not a lesson word.
- **D2. Tutor is the class.** Class sheets loaded over MCP are listed, marked against the course, and studyable. The page never invents a class, and an unloaded section says so.
- **D3. Review mixes both and names the source.** A card says Common words, From your tutor, or From an earlier lesson. Ordering, counts, and grading do not change.
- **D4. Progress states the mix, then the detail behind it.** The brief view keeps one can-do sentence, adds three short lines and two links. No charts, percents, streaks, or points in the brief view. The detail view stays as it is.
- **D5. Learn is reference, not a course.** The alphabet and the vowels are pages over data already in the app. Isaiah leaves Learn when its verses are not loaded.
- **What this does not decide:** conjugation, listening, a voice, speech scoring, retiring the old class-sheet units.
- **Consequences:** `/lessons` no longer lists the old units. Tutor carries them. Nothing is deleted.

### `docs/decisions/ADR-0034-...md`

Add one line under Consequences: "Amended 2026-11-12: the sections are Lessons, Tutor, Review, Progress, and Learn (ADR-0035). Lessons is where the course is listed."

### The initiative

- Add SPEC-028 to SPEC-034 to the spec table with their status and a one-line outcome each.
- Update the total effort.
- Note in **Done when** that the five sections run end to end on a local copy: a lesson opens and studies its five words, a class study works with a saved sheet, review names its sources, Learn shows the letters and vowels, and Progress shows the three lines.

## Acceptance criteria

- [ ] ADR-0035 exists with the five decisions and the non-decisions.
- [ ] ADR-0034 has the amendment line.
- [ ] The initiative table lists SPEC-028 to SPEC-034 with real statuses.
- [ ] No existing sentence in ADR-0034 was removed.

## Verification Strategy

- **Claim:** the five-section decision is written down where the next agent looks.
- **Check + executor:** manual read, plus `git diff --stat docs/decisions/` showing insertions only for ADR-0034.
- **Pass condition:** the diff has no deleted lines in ADR-0034.
