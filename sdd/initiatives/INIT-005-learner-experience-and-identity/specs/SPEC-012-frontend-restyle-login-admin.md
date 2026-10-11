# SPEC-012 · Frontend: Restyle login and Admin

## Metadata

```yaml
spec_id: SPEC-012
initiative_id: INIT-005
title: "Restyle login and Admin"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-004, SPEC-009, SPEC-010]
blocks: [SPEC-013]
```

---

## Description

**Context:** Apply ADR-0008 to login (still a full page) and Admin (a shell view).

**Scope (in):** login page styles and brand; Admin view styles; hooks preserved; auth behaviour unchanged.

**Scope (out):** learner views (SPEC-011); screenshot suite (SPEC-013).

---

## Understood

- [ ] und-1: Login stays outside the SPA shell.
- [ ] und-2: Admin remains server-gated.

## Acceptance Criteria

- [ ] ac-1 (REQ-017): login and Admin use ADR-0008 tokens.
- [ ] ac-2: sign-in, sign-out, forced password change and admin tests still pass.
- [ ] ac-3: no auth or CSRF rule is weakened.

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Login brand | Same logo pick as the shell | resolved | yes — SPEC-006 pick |

> **Readiness** = 100.

---

## Deliverables

- [ ] Login and Admin restyle
- [ ] Tests green
- [ ] Completion summary

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the branch.

---

## Security

- **Sensitivity:** standard. Auth surface.
- **Security acceptance criteria:**
  - [ ] No change to session cookie flags, CSRF, or rate limits in this spec.
- **Review depth:** standard.

---

## Verification Strategy

- **Claim:** login and Admin match the new look without auth regressions.
- **Check + executor:** `mechanized` — `npm test`.
- **Pass condition:** exit 0.
- **Provenance tag:** `INIT-005/SPEC-012`
