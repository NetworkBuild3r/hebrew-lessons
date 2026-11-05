# SPEC-005 · mcp: unit progress, revision briefs, and the backlog cap

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-030
title: "Unit progress in get_context, revision briefs, and the backlog cap"
domain: mcp
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-002, SPEC-003]
blocks: [SPEC-006]
```

## Description

`get_context` with `learnerId` adds a `progress` object to each outline row: `state`, `lessonsFinished`, `lessonsTotal`, and `lastFinishedAt`. Without `learnerId`, it is omitted.

`get_lesson_brief` accepts an optional `unitId`:

- With a `unitId`, it builds a revision brief for that learner unit, as of that unit's position (SPEC-003).
- The unit must be in scope `learner:<learnerId>` and `untouched`. Otherwise it is a tool error that says the unit is started, finished, shared, or another learner's.

Without a `unitId`, a new brief is refused when the learner has three or more `untouched` learner units. The error names the earliest one, so the agent can revise it.

`put_unit` with a revision brief and that same `unitId`:

- It publishes a new revision of that unit, at the same position.
- It checks `IsUntouched` again inside the publish transaction. If a run has started, it refuses with `brief.revise_started` and publishes nothing.
- A revision brief used on another unit id is `brief.not_found`.

The server instructions get one sentence on revising untouched units before adding new ones.

## Acceptance criteria

- [ ] Outline rows carry progress with `learnerId`, and omit it without.
- [ ] A revision brief for an untouched unit builds. Started, finished, shared, and another learner's units are each refused.
- [ ] Three untouched units refuse a new brief and name the earliest. Two allow it.
- [ ] A run that starts after the revision brief makes `put_unit` refuse with `brief.revise_started`, and no revision is published.
- [ ] A revision brief cannot publish a different unit.
- [ ] A smoke-token learner sees only their own progress and cannot call the revision path.
