# SPEC-021 · docs: where a graded meaning comes from

## Metadata

```yaml
spec_id: SPEC-021
initiative_id: INIT-039
title: "Where a graded meaning comes from"
domain: docs
status: complete
primary_prompt: none
complexity: low
complexity_reason: three short amendments to an accepted ADR, with the exact text given here
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 1 hour
blocked_by: []
blocks: [SPEC-022]
```

## Description

ADR-0034 D3 says every graded key is T0, which means the tutor's sheet shows the word with its English. On 2026-11-11 D2 was amended so that words come from the frequency list instead of class sheets. That list has no English, so D3 as written now lets nothing be graded. Brian asked on 2026-11-12 for a full course of the common words. This spec amends D3 to name the new source of a graded meaning, amends D1 to say that the review sitting is where course words are taught, and amends D5 so that Today offers a sitting when course words are waiting.

## What to change

File: `docs/decisions/ADR-0034-stage-2-lessons-built-from-your-classes.md`. Do not delete any existing sentence. Append each amendment to the end of its decision paragraph, in the same style as the 2026-11-11 amendments already there.

Append to **D1**:

> Amended 2026-11-12: course words are taught in the review sitting. Each sitting adds the next ready words on the course list, as many as recent accuracy allows, and review does the rest: return on later days, struggle words, letter drills, and the weakest skill first. No separate course lesson player is built.

Append to **D3**:

> Amended 2026-11-12: for a course word, the graded meaning comes from one row of `internal/speech/course.tsv` marked `ready`. A row is built from the English Wiktionary Hebrew data (CC BY-SA 4.0, through kaikki.org). It is ready only when it passes the entry check and a second agent, which did not write the course builder, has signed it in `internal/speech/course_fixes.tsv` after comparing it with the dictionary senses. A word already in the lexicon keeps its stored meaning. The flag control (D6) voids a grade and suspends the word when a meaning is still wrong. There is still no T1 source for class-sheet words.

Append to **D5**:

> Amended 2026-11-12: when no review is due and ready course words are waiting, Today offers a sitting. Its label says how many new common words it adds. There is no class label, because the words do not come from a class.

Also add one line under **Consequences**:

> - A course word is added to the shared lexicon only when no stored row has its consonant spelling. When exactly one stored row has that spelling, the course uses that row. When two or more stored rows have it, the course skips the word. The course never changes a stored row.

## Acceptance criteria

- [ ] D1, D3, and D5 each end with the amendment text above, word for word.
- [ ] The Consequences list has the new line.
- [ ] No existing sentence in the ADR was removed or reworded.

## Verification Strategy

- **Claim:** the ADR names the source of a graded course meaning.
- **Check + executor:** manual read by the building agent, plus `git diff --stat docs/decisions/ADR-0034-stage-2-lessons-built-from-your-classes.md`, which shows insertions only.
- **Pass condition:** the diff has no deleted lines.
