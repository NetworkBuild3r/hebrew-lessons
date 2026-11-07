# SPEC-008 · content: rewrite the listed units

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-032
title: "Rewrite the listed units"
domain: content
status: planned
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: [production_content_write]
reversibility_class: remote_reversible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-007, INIT-031/SPEC-004]
blocks: []
```

## Description

For each unit in `bad-lessons.md`, earliest position first, follow `.cursor/skills/hebrew-lesson-reviser/SKILL.md`: get the revision brief for that `unitId`, rewrite the document, `validate_unit` with the brief, `generate_audio`, and `put_unit` with the same `unitId` and `publish: true`.

The rewrite:

- Makes every cloze a sentence of three or more words with its English, and lists any other word that fits in `accept`.
- Turns a one-word cloze into a `produce`, or into a cloze on a story line that has the word.
- Gives each lesson of the unit its own lines. Unit 12's lessons 2 and 3 use the same words as lesson 1 in new sentences, or are removed.

Lesson 12 is started. The server on `main` refuses to revise a started unit, so this spec waits for INIT-031/SPEC-004, the live check that a started unit can be replaced.

Brian approves the list before the first publish (gate `production_content_write`). The old revision stays in `unit_revisions`, so each publish can be rolled back.

## Acceptance criteria

- [ ] Every unit on the list was replaced, or Brian chose to skip it, and the list records which.
- [ ] `validate_unit` with the brief has no block on each replacement.
- [ ] Unit 12 has no one-word cloze and no repeated lesson.
- [ ] Each replacement keeps its `unitId` and position.
