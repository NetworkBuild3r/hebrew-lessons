# SPEC-024 · frontend: Dialogue — play one side

## Metadata

```yaml
spec_id: SPEC-024
initiative_id: INIT-016
title: "Dialogue: play one side"
domain: frontend
status: planned
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: remote
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-018]
blocks: []
```

## Description

**Context:** The conversation gap is filled in steps; the first is scripted dialogues where the learner plays one side. Dialogues already exist as a teach step with two voices.

**Scope (in):**

- A dialogue step can be played in "your turn" mode, chosen by the author (`play: "b"` on the dialogue activity, validated in `internal/units`) or offered after the learner has seen the dialogue once.
- The other speaker's lines play and show as usual. On the learner's line: the English cue only, "I said it", then the line as cards with its audio, and "Got it" / "Not yet" (SPEC-017's self-check, one attempt per line).
- Progress through the dialogue is per line; leaving and returning resumes at the next line.

**Scope (out):** Branching role-plays and an AI partner (later initiatives).

## Acceptance Criteria

- [ ] ac-1: On the learner's line, no Hebrew text or audio for that line is shown before "I said it". (REQ-015)
- [ ] ac-2: Each learner line writes one self-check attempt; the dialogue finishes after the last line. (REQ-015)
- [ ] ac-3: The step passes at 390 and 1440, light and dark, keyboard-only and without script.

## Deliverables

- [ ] `internal/units` field and rule; player changes in `internal/web/lesson.go`, `views/player.templ`, `static/js/dialogue.js`
- [ ] Render tests; dialogue browser test extended with screenshots

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** deploy. **Rollback:** revert and redeploy.

## Security

- **Sensitivity:** standard — new unit field from MCP, enum-checked.

## Verification Strategy

- **Claim:** the learner produces their side of the dialogue before hearing it.
- **Check + executor:** mechanized — `go test ./internal/web/... ./internal/units/...`; `TestDialogue*` browser suite with `-tags browser`.
- **Pass condition:** ac-1 to ac-3 tests and checks pass.

## Integration Points

- Uses SPEC-017/018. Needs SPEC-016's second voice for two speakers.
- **Provenance tag:** `INIT-016/SPEC-024`
