# SPEC-002 · data: untouched, started, or finished, per learner unit

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-030
title: "Untouched, started, or finished, per learner unit"
domain: data
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-001]
blocks: [SPEC-005]
```

## Description

`internal/db` gains `UnitProgress(ctx, userID)`, which returns, for every unit in the learner's scope:

- the state: `untouched` (no `lesson_runs` row for this learner), `started` (a row, but not every lesson finished), or `finished` (every lesson has a `finished_at`)
- lessons finished and lessons in total
- the last finished time
- the mean score of the finished lessons

It reuses the query behind `FinishedLessons` and adds a count of any run. It adds no table. A migration is allowed only for an index, if the query plan needs one.

`IsUntouched(ctx, userID, unitID)` is the check that revision runs inside its publish transaction.

## Acceptance criteria

- [ ] A unit with no runs is `untouched`, a unit with one unfinished run is `started`, and a unit with every lesson finished is `finished`.
- [ ] Lesson counts and the mean score match seeded runs.
- [ ] Another learner's runs never change a unit's state.
- [ ] `IsUntouched` reads inside a passed transaction.
