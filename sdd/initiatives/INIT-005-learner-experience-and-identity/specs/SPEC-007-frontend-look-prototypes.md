# SPEC-007 · Frontend: Three tap-through look prototypes

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-005
title: "Three tap-through prototypes of one lesson step"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: gated
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-002, SPEC-005, SPEC-006]
blocks: [SPEC-008]
```

---

## Description

**Context:** a-calm-type is rejected. The operator picks a new look from live prototypes, not screenshots. Prototypes sit on the SPEC-002 shell so navigation feel is part of the judgment.

**Scope (in):** three directions in `spikes/look-v2/` covering introduction, correct, wrong-with-elaboration, lesson-complete; real icons from SPEC-006; tap states; motion; dark and light; 360 and 1280. Each is more energetic than a-calm-type while keeping 4.5:1 text and 3:1 UI contrast, 24 px / 44 px targets. Operator picks one.

**Scope (out):** production restyle (SPEC-011); ADR write-up (SPEC-008).

---

## Understood

- [ ] und-1: No points, streaks, jokes or mascot.
- [ ] und-2: Effects constrained by RSCH-007's allowed / product-choice / banned table.
- [ ] und-3: Spikes are not served by the production server (existing `.dockerignore` / `spikes-not-served` rule).

## Acceptance Criteria

- [ ] ac-1 (REQ-012): three working prototypes the operator can open and tap through.
- [ ] ac-2 (REQ-017): each direction meets contrast and target-size numbers (checked with the existing contrast helper or an updated copy).
- [ ] ac-3: operator pick recorded in the completion summary.

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Spike path | `spikes/look-v2/{a,b,c}/` | resolved | yes — planning default |

> **Readiness** = 100.

---

## Deliverables

- [ ] Three prototype directories with README and screenshots
- [ ] Contrast check pass
- [ ] Completion summary with pick
- [ ] Operator gate

---

## Gates & Controls

- **Autonomy level:** `gated`.
- **Blocking checks:** contrast check; operator pick.
- **Rollback plan:** delete `spikes/look-v2/`.

---

## Security

- **Sensitivity:** light.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** three prototypes exist, pass contrast, and one is picked.
- **Check + executor:** `mechanized` — contrast script; `manual` — operator pick.
- **Pass condition:** ac-1 through ac-3.
- **Provenance tag:** `INIT-005/SPEC-007`
