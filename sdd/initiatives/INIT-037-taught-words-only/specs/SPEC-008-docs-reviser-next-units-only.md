# SPEC-008 · docs: the reviser rebuilds only the next one or two units

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-037
title: "The reviser rebuilds only the next one or two units"
domain: docs
status: complete
primary_prompt: none
complexity: low
complexity_reason: skill and guide text only, no code
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-005, SPEC-006, SPEC-007]
blocks: [SPEC-011]
```

## Description

The reviser skill revises every untouched unit in position order (`.cursor/skills/hebrew-lesson-reviser/SKILL.md:20-26`). Each brief reads the learner as they are at that moment, so one session rebuilt 22 units from one snapshot.

- **Reviser.** Rebuild at most the next two untouched units. Leave the rest as drafts with `unpublish_unit`, and stop (ADR-0032 D6).
- **Author skill.** Respect a "review first" brief reply (SPEC-007): write no unit, and say why in the coach note. Never paste a tutor's whole sentence into a contrast. Build the contrast line from words the brief lists as known or learning.
- **Author and reviser skills.** Name the new rules from SPEC-005 and SPEC-006 and what each asks the author to do.

**Scope (out):** rule code (SPEC-005, SPEC-006); brief code (SPEC-007).

## Acceptance criteria

- [x] The reviser skill says "at most two" and "leave the rest as drafts", and names `unpublish_unit`.
- [x] The author skill and `authoring.md` describe the "review first" reply, and `unit.target_untaught`, `unit.meta_text`, `unit.story_repeat` and `brief.checkpoint_coverage`.
- [x] `go test ./internal/mcp/...` passes, including the tests that pin the guide's rule list.

## Verification Strategy

- **Claim:** the skills tell an agent to rebuild forward one or two units at a time, and to follow the new rules.
- **Check + executor:** mechanized for the guide's rule-list parity (`go test ./internal/mcp/...`); human for the skill wording, read by Brian.
- **Pass condition:** the tests pass, and Brian approves the skill diff.
