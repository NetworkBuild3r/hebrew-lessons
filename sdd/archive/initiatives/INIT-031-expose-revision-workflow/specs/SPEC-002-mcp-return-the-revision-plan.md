# SPEC-002 · mcp: return the learner data and let the agent rewrite its units

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-031
title: "Return the learner data and let the agent rewrite its units"
domain: mcp
status: planned
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003]
```

## Description

**Data the connection returns.**

`lessonBrief` copies `nextPattern`, `reusePatterns`, `previousUnits` (title, situation, pattern, newWords, story and dialogue text), `pace`, and `targets.minStoryTokens` from `brief.Brief` onto `mcp.LessonBrief`. Each field gets a `jsonschema` description.

`get_context` outline rows include `progress` (`state`, `lessonsFinished`, `lessonsTotal`, `lastFinishedAt`, `meanScore`) and a `summary` (pattern slug, new-word count, Hebrew tokens in the stories and dialogues). The description says these are facts for the agent, and a started or finished unit may still be replaced.

`get_lesson_brief` no longer refuses when three untouched units exist, and no longer refuses a started or finished unit. `unitId` still builds the brief as of that unit's position, so a rebuild sees only the lessons before it. The result includes `reviseUnitId` when `unitId` was passed.

**Actions on a lesson the agent built.** Scope must be `learner:<id>`. Shared and another learner are the same not-found answer.

- `put_unit` with that `unitId` and `publish: true` replaces the published document even when the learner has a run. The old revision stays. `brief.revise_started` is not returned. Position stays unless the agent sends a new one.
- `unpublish_unit` stays as it is: the unit leaves Today and returns to draft. The description says the agent may unpublish a unit the learner has started.
- `delete_unit` is new. Admin only. It removes the learner unit's published pointer. `lesson_runs` and `publish_audit` stay. The description tells the agent to read `get_unit` and `progress` first, and says the call does not ask the server whether the learner has started.

`validate_unit` names `brief.story_length`, `unit.drill_share`, `brief.pattern`, `brief.recycle_previous`, and `brief.consolidation` as checks the agent can read and fix. They still block a publish that fails them. They do not choose which unit the agent edits.

`internal/mcp/server.go` gets one sentence: read the learner's progress and the lessons already published, then replace, unpublish, delete, or recombine those learner units.

Do not call these tools against the live learner.

## Acceptance criteria

- [ ] A revision `get_lesson_brief` returns `nextPattern`, `pace`, `previousUnits`, `minStoryTokens`, and `reviseUnitId`. A started unit is not an error.
- [ ] Three untouched units do not refuse a new brief.
- [ ] `put_unit` with the same `unitId` publishes a new revision when that learner has a finished run. The previous revision is still stored.
- [ ] `delete_unit` removes a learner unit from Today and leaves `lesson_runs` in place. A shared unit and another learner's unit are refused.
- [ ] `get_context` returns `progress.state` and the content `summary` for each outline row.
