# SPEC-008 · mcp: the lesson author writes `order` steps and reads the results

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-029
title: "The lesson author writes order steps and reads the results"
domain: mcp
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-002, SPEC-007]
blocks: []
```

## Description

`hebrew://catalog/activities` lists `order` and what it uses `text`, `translation`, and `accept` for. `hebrew://rules` lists the new rule ids. `get_context` with `learnerId` adds `practiceResults`: for each kind of practice, the share right on a later day and the count, or "not enough yet". The server instructions in `internal/mcp/server.go` mention the new field in one sentence.

`internal/mcp/resources/authoring.md` and `.cursor/skills/hebrew-lesson-author/SKILL.md` tell the author to:

- Add one or two `order` steps per lesson, on lines made of known words, after the new words are introduced.
- Keep the English cue short, and list any other natural word order in `accept`.
- Read `practiceResults`. When one kind of practice is clearly behind for this learner and has enough answers, use less of it. When results say "not enough yet", keep the default mix.

## Acceptance criteria

- [ ] The resources test finds `order` in the activity catalog and the new rule ids in the rule catalog.
- [ ] `get_context` with a learner returns `practiceResults`, and without a learner it omits the field.
- [ ] A smoke-token learner can read only their own `practiceResults`.
- [ ] The authoring guide and the skill describe when to use `order` and how to read the results.
