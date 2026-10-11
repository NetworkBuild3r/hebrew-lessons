# SPEC-004 · Frontend: Port Admin; legacy lessons; route tests

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-005
title: "Port Admin; legacy lessons per SPEC-002; route and 401 tests"
domain: frontend
status: in_progress
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-003]
blocks: [SPEC-011, SPEC-012]
```

---

## Description

**Context:** Finish the SPA port. Admin becomes a view. Legacy lessons follow the SPEC-002 decision. Direct URLs, unknown routes, and 401 handling are tested.

**Scope (in):** Admin view; legacy lesson mount or full-page fallback; 401 → `/login/` with safe `next`; open-in-new-tab and middle-click still load the target URL; route and 401 tests.

**Scope (out):** Restyle; builder screens (INIT-004).

---

## Understood

- [ ] und-1: Admin-only routes stay admin-gated server-side; the client hides the link for non-admins as today.
- [ ] und-2: SPEC-002's spike note is the source of the legacy-lesson rule; this spec implements it, it does not reopen it.

## Acceptance Criteria

- [ ] ac-1 (REQ-008): Admin works inside the shell with existing admin tests and smokes still passing.
- [ ] ac-2 (REQ-009): legacy lessons follow the SPEC-002 decision (in-shell or full page with the same header).
- [ ] ac-3 (REQ-007): a 401 from an API call during a view sends the learner to `/login/` with a safe `next`.
- [ ] ac-4 (REQ-004, REQ-006, REQ-019): tests cover direct URL, unknown route, history back/forward, and that plain `<a href>` still works for open-in-new-tab.

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Legacy fate | Taken from SPEC-002 spike note | resolved | yes — at SPEC-002 gate |

> **Readiness** = 100.

---

## Deliverables

- [ ] Admin view; legacy handling
- [ ] Route and 401 tests
- [ ] Completion summary

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the branch.

---

## Security

- **Sensitivity:** standard. Triggers matched: admin surface, 401 redirect.
- **Security acceptance criteria:**
  - [ ] Admin routes remain server-gated.
  - [ ] `next` uses `safeNextPath`.
- **Review depth:** standard.

---

## Verification Strategy

- **Claim:** all signed-in screens are in the shell (or legacy fallback); 401 and direct URL work.
- **Check + executor:** `mechanized` — `npm test`.
- **Pass condition:** exit 0.
- **Provenance tag:** `INIT-005/SPEC-004`
