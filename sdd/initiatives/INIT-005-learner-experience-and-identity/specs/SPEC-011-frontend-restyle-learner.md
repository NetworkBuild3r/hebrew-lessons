# SPEC-011 · Frontend: Restyle learner views

## Metadata

```yaml
spec_id: SPEC-011
initiative_id: INIT-005
title: "Restyle Today, Lessons, player, Review, Progress"
domain: frontend
status: ready
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-004, SPEC-009, SPEC-010]
blocks: [SPEC-013]
```

---

## Description

**Context:** Structure, tokens, feedback kit and icons are ready. Apply ADR-0008 to every learner view in `shared/worksheet.css` without renaming queried hooks.

**Scope (in):** Today, Lessons, lesson/player views, Review, Progress; token application; stray English label cleanup (REQ-016); feedback kit wired on correct/wrong and complete.

**Scope (out):** login and admin (SPEC-012); screenshot suite (SPEC-013).

---

## Understood

- [ ] und-1: Class and `data-*` hooks a test or smoke queries change only in the same change as that test.
- [ ] und-2: Default is to restyle the legacy player now (initiative Q2 default).

## Acceptance Criteria

- [ ] ac-1 (REQ-017): learner views use ADR-0008 tokens; contrast pairs still pass.
- [ ] ac-2 (REQ-016): no stray English-only chrome labels on Hebrew screens (glosses remain).
- [ ] ac-3 (REQ-008): existing learner tests and applicable smokes still pass.

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Restyle legacy player | Yes, per initiative default | resolved | yes — unless operator changes Q2 |

> **Readiness** = 100.

---

## Deliverables

- [ ] Updated `shared/worksheet.css` and view markup as needed
- [ ] Tests still green
- [ ] Completion summary

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the branch.

---

## Security

- **Sensitivity:** light.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** learner views match ADR-0008 without breaking hooks.
- **Check + executor:** `mechanized` — `npm test`.
- **Pass condition:** exit 0.
- **Provenance tag:** `INIT-005/SPEC-011`
