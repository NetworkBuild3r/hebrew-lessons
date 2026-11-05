# SPEC-003 · service: the brief plans the next step

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-030
title: "The brief plans the next step: pattern, pace, previous units, mistakes"
domain: service
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004, SPEC-005]
```

## Description

`internal/brief` raises `Version` and adds these fields to `Plan`:

- `nextPattern`: the slug of the first pattern, in catalog order, that this learner's earlier units have not taught and whose prerequisites they have all taught. A unit teaches the pattern stored on it. It is empty when the brief asks for consolidation.
- `reusePatterns`: the slugs of the two most recently taught patterns, each with its title.
- `previousUnits`: up to two units before this position, each with title, situation, pattern, newWords, and every story and dialogue text.
- `pace`:
  - `newWordCount`
  - `consolidate` (true or false)
  - `reason`, in plain English
  - the inputs: accuracy over the last 50 reviews, and the count of learning words
- `targets.minStoryTokens`: 12 for beginner, 30 for developing, 50 for established, capped by `tier.maxTextTokens` when that is not zero.

Pace rules:

- New words are 2 below 80% accuracy, 5 from 80% to 90%, and the tier's count above 90%.
- `consolidate` is true when learning words exceed 40, or when accuracy is under 75%. Then `newWordCount` is 0 and `newWords` is empty.
- The constants live beside `NewWordCount`.

`fixMistakes` rotation:

- A pair that appeared in each of the previous two units is skipped. A pair "appears" when its `right` form is in that unit's final checkpoint.
- A pair whose `right` form has been answered right three times since the mistake's date is retired.
- The rest are ordered as today, most repeated first, and capped at three.

`Build` takes an optional position and an optional `reviseUnitID`. A revision brief uses the units before that unit's position as "earlier units". It ignores units at or after it.

Selection stays deterministic, following the package's existing seeding rule.

## Acceptance criteria

- [ ] With patterns A, then B (requires A), then C, where A is taught, `nextPattern` is B. With A and B taught, it is C.
- [ ] `reusePatterns` names the last two taught patterns.
- [ ] `previousUnits` carries the two units before the position, with their story and dialogue text.
- [ ] Accuracy of 0.72 gives `newWordCount` 2. 45 learning words gives `consolidate` true and no new words.
- [ ] The same three mistakes in the previous two units are not the brief's three. A mistake answered right three times since its date is gone.
- [ ] A revision brief for the unit at position 10 ignores units at 10 and later.
- [ ] Payload `v` is raised, and an old payload still loads.
