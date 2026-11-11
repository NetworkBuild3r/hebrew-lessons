# SPEC-012 · Frontend: Restyle login and Admin

## Metadata

```yaml
spec_id: SPEC-012
initiative_id: INIT-005
title: "Restyle login and Admin"
domain: frontend
status: complete
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

- [x] und-1: Login stays outside the SPA shell. _(Closure: `login/index.html` is still a self-contained full page; styles and script stay inline because `/shared/worksheet.css` is gated while signed out.)_
- [x] und-2: Admin remains server-gated. _(Closure: no server or route change. `test/web-gating.test.js` and `test/spa-shell.test.js` pass.)_

## Acceptance Criteria

- [x] ac-1 (REQ-017): login and Admin use ADR-0008 tokens. _(Closure: ADR-0010 is the adopted look. Admin uses the shared `var(--color-*)` tokens; login inlines the same token names and values with a `data-theme="dark"` set. Controls set their own background and colour.)_
- [x] ac-2: sign-in, sign-out, forced password change and admin tests still pass. _(Closure: `npm test` 1040 pass; `chrome-signin-smoke`, `chrome-admin-smoke`, `chrome-admin-lessons-smoke` and `chrome-ai-profile-smoke` exit 0.)_
- [x] ac-3: no auth or CSRF rule is weakened. _(Closure: the diff touches no server file, cookie flag, CSRF check or rate limit. The sign-in script change only builds the two privacy notes with a gloss span instead of one text string; the request, `next` handling and error messages are as before.)_

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Login brand | Same logo pick as the shell | resolved | yes — SPEC-006 pick |

> **Readiness** = 100.

---

## Deliverables

- [x] Login and Admin restyle
- [x] Tests green
- [x] Completion summary _(Closure: `completion-summaries/SPEC-012-completion-summary.md`.)_

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the branch.

---

## Security

- **Sensitivity:** standard. Auth surface.
- **Security acceptance criteria:**
  - [x] No change to session cookie flags, CSRF, or rate limits in this spec.
- **Review depth:** standard.

---

## Verification Strategy

- **Claim:** login and Admin match the new look without auth regressions.
- **Check + executor:** `mechanized` — `npm test`.
- **Pass condition:** exit 0.
- **Provenance tag:** `INIT-005/SPEC-012`
