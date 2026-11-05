# SPEC-004 · content: validator rules for story length, drill share, and recycling

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-030
title: "Validator rules for story length, drill share, and recycling"
domain: content
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-003]
blocks: [SPEC-006]
```

## Description

`internal/units` adds rules that run with a brief, on a learner unit. `BriefFacts` gains the new brief fields it needs. Rule ids are stable and listed in `doc.go`.

- `brief.story_length` (block): each `lesson` has a `story`, or a `dialogue` whose lines together reach `minStoryTokens` Hebrew words. A checkpoint is exempt.
- `unit.drill_share` (block with a brief, warn without): at most 40% of a unit's activities are single-word drills. A single-word drill is a `shadow`, `listen_pick`, `recall`, or `produce` whose text is one Hebrew token.
- `brief.pattern` (block): the unit's pattern equals the brief's `nextPattern`. When `consolidate` is true, the unit names no new pattern, or names a `reusePatterns` slug.
- `brief.recycle_previous` (block): at least four distinct newWords of the brief's `previousUnits` appear in this unit's meaning steps. A meaning step is a story, a dialogue line, a multi-word `listen_pick`, `cloze`, or `say`, or an `order`. When the previous units have fewer than four new words in total, all of them must appear.
- `brief.consolidation` (block): when `consolidate` is true, `newWords` is empty.
- `unit.same_shape` (warn): the sequence of activity types is at least 80% the same as the first previous unit's sequence, compared by edit distance over the type list.

`budget.lesson.few` does not warn on a consolidation unit.

## Acceptance criteria

- [ ] Units 30 and 34 from learner 1, replayed as fixtures against a matching brief, fail `brief.story_length` and `unit.drill_share`.
- [ ] Each new rule has a passing and a failing table case.
- [ ] A consolidation brief blocks a unit with new words, and passes one with none and a 12-word story.
- [ ] `hebrew://rules` lists every new rule id.
