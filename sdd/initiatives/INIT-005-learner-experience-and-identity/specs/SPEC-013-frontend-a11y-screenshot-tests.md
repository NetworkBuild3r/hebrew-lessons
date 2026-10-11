# SPEC-013 · Frontend: Accessibility, screenshot and hook-contract tests

## Metadata

```yaml
spec_id: SPEC-013
initiative_id: INIT-005
title: "Accessibility, screenshot and hook-contract tests"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-011, SPEC-012]
blocks: []
```

---

## Description

**Context:** Close INIT-005 with durable checks: contrast and targets from ADR-0008, reduced motion per route, screenshots at 360 and 1280, and the class/`data-*` hook contract.

**Scope (in):** automated contrast/target checks; reduced-motion tests; screenshot capture for shell routes; hook-contract test that fails if a queried selector was renamed without updating the test; update Chrome smokes only if a hook intentionally changed.

**Scope (out):** new features; further look changes.

---

## Understood

- [ ] und-1: Screenshot paths live under a checked-in directory or a test artefact path consistent with existing smoke scripts.
- [ ] und-2: This is the last INIT-005 build spec. Completion report waits for `complete-initiative`.

## Acceptance Criteria

- [ ] ac-1 (REQ-017, REQ-019): contrast and target-size checks pass for ADR-0008 tokens.
- [ ] ac-2 (REQ-005, REQ-019): reduced-motion path is asserted per route transition and feedback kit.
- [ ] ac-3 (REQ-019): screenshots at 360 and 1280 exist for Today, Lessons, Review, Progress, Admin, and login.
- [ ] ac-4: hook-contract test covers the selectors the unit tests and Chrome smokes query.

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Screenshot tool | Reuse the approach already used in `spikes/look` / Chrome smokes | resolved | yes — existing pattern |

> **Readiness** = 100.

---

## Deliverables

- [ ] Test files and screenshots
- [ ] Completion summary
- [ ] Initiative status ready for `complete-initiative` (mode e) once this passes

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test`; screenshot artefacts present.
- **Rollback plan:** revert the branch.

---

## Security

- **Sensitivity:** light.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** the restyle is locked by tests and screenshots.
- **Check + executor:** `mechanized` — `npm test`; `manual` — spot-check screenshots.
- **Pass condition:** exit 0; ac-1 through ac-4.
- **Provenance tag:** `INIT-005/SPEC-013`
