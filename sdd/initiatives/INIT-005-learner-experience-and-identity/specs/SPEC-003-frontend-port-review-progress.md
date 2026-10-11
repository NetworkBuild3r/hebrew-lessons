# SPEC-003 · Frontend: Port Review and Progress; route a11y

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-005
title: "Port Review and Progress; title, focus, scroll, announce"
domain: frontend
status: in_progress
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 6 hours
blocked_by: [SPEC-002]
blocks: [SPEC-004, SPEC-009]
```

---

## Description

**Context:** SPEC-002 proved the shell with Today and Lessons. This spec ports Review and Progress, and implements the accessibility behaviours ADR-0007 requires on every route change.

**Scope (in):** Review and Progress as views with `mount`/`unmount`; document title; focus to main heading; scroll reset with back/forward restore; polite live-region announce; reduced-motion-safe short transition.

**Scope (out):** Admin (SPEC-004); restyle; sound kit.

---

## Understood (agent's read of the intent)

- [ ] und-1: Existing review and progress behaviour and test hooks stay.
- [ ] und-2: Listeners and timers are removed in `unmount`.

## Acceptance Criteria

- [ ] ac-1 (REQ-008): Review and Progress work inside the shell with no regression against their existing unit tests and applicable Chrome smokes.
- [ ] ac-2 (REQ-004): on each in-app route change, title updates, focus moves to the main heading, scroll resets (history restores), and a polite live region announces the screen.
- [ ] ac-3 (REQ-005): the route transition honours `prefers-reduced-motion`.

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Live region text language | Hebrew primary, matching current shell labels | resolved | yes — existing bilingual pattern |

> **Readiness** = 100.

---

## Deliverables

- [ ] Review and Progress view modules
- [ ] Router a11y behaviours
- [ ] Tests for title, focus, announce, reduced motion
- [ ] Completion summary

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the branch.

---

## Security

- **Sensitivity:** light. No new secret surface.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** Review and Progress are views; route changes meet REQ-004 and REQ-005.
- **Check + executor:** `mechanized` — `npm test`.
- **Pass condition:** exit 0; a11y assertions present.
- **Provenance tag:** `INIT-005/SPEC-003`
