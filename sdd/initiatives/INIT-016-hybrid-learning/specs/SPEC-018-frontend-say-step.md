# SPEC-018 · frontend: Say-it-aloud step in the player

## Metadata

```yaml
spec_id: SPEC-018
initiative_id: INIT-016
title: "Say-it-aloud step in the player"
domain: frontend
status: planned
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: remote
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-016, SPEC-017]
blocks: [SPEC-024]
```

## Description

**Context:** The step must make the learner produce Hebrew aloud before hearing it, then compare.

**Scope (in):**

- Ask: the English cue large, "Say it in Hebrew", one button "I said it". No Hebrew, no audio on screen.
- Reveal: the model as cards (the `cardsIn` rule applies: say is a recall-type step, so words are cards), the line orb plays the model once automatically only if the learner pressed "I said it" (one explicit action), and two buttons "Got it" and "Not yet" in the existing neutral grade style.
- Without the voice clip (SPEC-016 not done): the step does not ship to learners; the validator already refuses a say step without audio.
- No-JS: native audio bar, buttons are form posts.

**Scope (out):** Recording, recognition.

## Acceptance Criteria

- [ ] ac-1: The ask screen contains no Hebrew text and no audio element. (REQ-011)
- [ ] ac-2: After "I said it", the model and its audio appear; "Got it" and "Not yet" post a self-check. (REQ-011)
- [ ] ac-3: The step passes at 390 and 1440, light and dark, keyboard-only, and without script.

## Deliverables

- [ ] Player branch in `internal/web/lesson.go` and `views/player.templ`; CSS in the existing files
- [ ] Render tests; browser test with screenshots

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** deploy. **Rollback:** revert and redeploy.

## Security

- **Sensitivity:** light — no new data; no microphone.

## Verification Strategy

- **Claim:** the learner says the Hebrew before seeing or hearing it, then compares.
- **Check + executor:** mechanized — `go test ./internal/web/...`; browser suite with `-tags browser`.
- **Pass condition:** ac-1 to ac-3 tests and checks pass.

## Integration Points

- SPEC-024 uses the same ask/reveal for dialogue lines.
- **Provenance tag:** `INIT-016/SPEC-018`
