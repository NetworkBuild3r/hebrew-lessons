# SPEC-006 · mcp: the authoring guide and skills teach the new shape and the revise loop

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-030
title: "The authoring guide and skills teach the new shape and the revise loop"
domain: mcp
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 3 hours
blocked_by: [SPEC-004, SPEC-005]
blocks: [SPEC-007]
```

## Description

`internal/mcp/resources/authoring.md` and `.cursor/skills/hebrew-lesson-author/SKILL.md` are updated together, because `TestResourcesAuthoringSkillMirror` holds them in step.

Changes:

- **Step 0.** Read the outline's `progress`. If an untouched learner unit exists, revise the earliest one instead of writing a new one.
- **Reading a brief.** Explain `nextPattern`, `reusePatterns`, `previousUnits`, `pace`, `consolidate`, and `minStoryTokens`.
- **The shape.** Each lesson:
  1. Opens with a story or dialogue that continues from the previous unit's story, reaches `minStoryTokens`, and uses the reuse patterns.
  2. Asks sentence questions on that text: a multi-word `listen_pick`, `cloze`, `order`, and `say`.
  3. Drills a bare word only for a new or struggle word that needs it, inside the drill cap.

  The checkpoint is a fluency pass. It re-reads or re-hears the unit's text with no new language, then asks checked questions.
- **The examples.** Replace both example units with a progressive pair: a unit that continues from the one before it, and a consolidation unit. Each must pass the new rules in a test, the way the current examples are tested.
- **The rule table.** Add every new rule id.

`.cursor/skills/hebrew-lesson-reviser/SKILL.md` is new. It is the revise loop:

1. Call `get_context`.
2. Pick the earliest untouched unit and read it with `get_unit`.
3. Call `get_lesson_brief` with that `unitId`.
4. Rewrite the unit against the brief and the units before it. Keep its situation when it still fits.
5. Call `validate_unit`, then `generate_audio`, then `put_unit` with the same `unitId`, then `add_coach_note`.
6. Repeat in position order, so each revised unit builds on the one before.

## Acceptance criteria

- [ ] The guide and the skill mirror test pass.
- [ ] Both example units pass `validate_unit` with their example briefs in the existing examples test.
- [ ] The guide names every new rule id.
- [ ] The reviser skill exists, names the tools in order, and says that started units are not revised.
