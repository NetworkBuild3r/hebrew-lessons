# SPEC-002 · service: Say-it-aloud activity type

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-020
title: "Say-it-aloud activity type"
domain: service
status: planned
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [INIT-017/SPEC-002]
blocks: [SPEC-003]
```

## Description

**Context:** "Being made to speak" is what a tutor does that the app does least. `shadow` (repeat after the audio) exists; answering aloud from a cue does not. Class phrases carry an English speaking prompt.

**Scope (in):**

- `units.ActivitySay = "say"`: `prompt_en` (the cue, required), `text` (the model answer, pointed, required), optional `target`.
- Validator: `say.prompt_required`, `say.audio_required` (the text has a clip), and the text's words obey the brief's coverage rule like any line.
- Grading: the learner self-checks against the model ("I said it" / "Not yet"). It records an attempt with `activity = say`, `skill = recall`, `source = self` and feeds FSRS as a self-graded answer (north star D3: self-grading only where nothing can be checked).
- MCP schema and authoring resource document the type; `put_unit` accepts it.

**Scope (out):** Speech recognition. Recording the learner's voice. The player (SPEC-003).

## Acceptance Criteria

- [ ] ac-1: A unit with a valid `say` step validates; one missing `prompt_en` or audio fails with the rule id. (REQ-011)
- [ ] ac-2: A self-check writes one attempt with `activity = say` and `source = self`, and a review log row. (REQ-011)
- [ ] ac-3: The MCP schema test pins the new type and fields.

## Deliverables

- [ ] `internal/units` type, fields, rules; `internal/evidence` source; `internal/mcp` schema and resource
- [ ] Tests

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none. **Rollback:** revert; no stored data depends on it until units use it.

## Security

- **Sensitivity:** standard — new unit fields from MCP; lengths capped (`prompt_en` ≤ 200).

## Verification Strategy

- **Claim:** a say step can be authored, validated and graded.
- **Check + executor:** mechanized — `go test ./internal/units/... ./internal/evidence/... ./internal/mcp/...`.
- **Pass condition:** ac-1 to ac-3 tests pass.

## Integration Points

- SPEC-003 plays it; SPEC-004 reuses the self-check for dialogue lines.
- **Provenance tag:** `INIT-020/SPEC-002`
