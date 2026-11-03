# SPEC-004 · frontend: the build-the-sentence screen

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-029
title: "The build-the-sentence screen"
domain: frontend
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 5 hours
blocked_by: [SPEC-003]
blocks: []
```

## Description

The ask screen shows the English line, an empty right-to-left answer row, and the shuffled chips. A chip is pointed Hebrew only, with no reading, no English, and no gender chip. The learner drags a chip into the row, or taps it to place it at the end. Tapping a placed chip sends it back. Check is enabled once every chip is placed. The keyboard works: arrow keys move focus, Enter places or removes a chip, and the row has an accessible name.

The answer screen shows the learner's row marked right or wrong. When it is wrong, the right order is shown under it. The line's reading and English are shown and the clip plays. Continue moves on, as on cloze.

Drag reuses the existing drag helper in `internal/web/static`. Without JavaScript, the chips are a form of position selects, so the step still posts.

## Acceptance criteria

- [ ] The ask HTML contains no transliteration and no English for any chip.
- [ ] The answer row is `dir="rtl"`, and placed chips read right to left in the posted order.
- [ ] A browser test places the chips by drag and by tap, checks, and sees the right-order reveal on a wrong answer.
- [ ] The step works with the keyboard alone.
- [ ] The step renders at 360px wide without horizontal scroll.
