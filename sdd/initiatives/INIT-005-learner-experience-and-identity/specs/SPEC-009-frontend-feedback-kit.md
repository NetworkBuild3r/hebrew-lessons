# SPEC-009 · Frontend: Feedback kit and route transition

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-005
title: "Feedback kit: motion, correct/wrong, audio, complete, sound toggle"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-003, SPEC-008]
blocks: [SPEC-011, SPEC-012]
```

---

## Description

**Context:** ADR-0008 and RSCH-008 define what effects are allowed. This spec builds the kit used by every view.

**Scope (in):** CSS motion tokens; a pure function returning escaped feedback markup (correct, wrong with correct form); audio-playing indicator; lesson-complete moment; sound toggle (off by default; never overlaps Hebrew word audio); short route transition; all honour `prefers-reduced-motion`; nothing self-running >5s without pause.

**Scope (out):** restyling whole pages (SPEC-011); icon sprite (SPEC-010).

---

## Understood

- [x] und-1: Elaborated feedback still shows the correct form (RSCH-006). A flash or sound is not a substitute.
- [x] und-2: Sounds are product choice, off by default (REQ-014).

## Acceptance Criteria

- [x] ac-1 (REQ-013, REQ-005): kit exists; reduced-motion path verified by test. _(Closure: `test/feedback-kit.test.js`, `test/player-feedback.test.js`, and a Chrome check of the route transition under emulated reduced motion.)_
- [x] ac-2 (REQ-014): sound default is off; mute control exists; word audio is never ducked by a UI sound in the default path. _(Closure: the visible control is `#btn-sfx` in the lesson player header, a 44 px `aria-pressed` button with a Hebrew label and English gloss, wired to `[data-sfx-toggle]`. The shell has no sound control because no sound plays outside the player.)_
- [x] ac-3: wrong-answer markup includes the correct form.

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Module path | `shared/ui/feedback.js` and tokens in `shared/worksheet.css` | resolved | no — the kit already exists as `shared/player/feedback.js` (INIT-007/SPEC-002); kept there, spec path superseded |

> **Readiness** = 100.

---

## Deliverables

- [x] `shared/ui/feedback.js` (or equivalent) _(Closure: the real module is `shared/player/feedback.js` from INIT-007/SPEC-002; not moved. SPEC-009's assumed path was wrong.)_
- [x] Motion / sound CSS and toggle _(Closure: delimited block at the end of `shared/worksheet.css`.)_
- [x] Tests _(Closure: `test/feedback-kit.test.js`, `test/router-a11y.test.js`.)_
- [x] Completion summary

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the branch.

---

## Security

- **Sensitivity:** light. Escaped markup only.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** feedback kit meets REQ-013–014 and reduced motion.
- **Check + executor:** `mechanized` — `npm test`.
- **Pass condition:** exit 0.
- **Provenance tag:** `INIT-005/SPEC-009`
