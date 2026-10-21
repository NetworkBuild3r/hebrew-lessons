# SPEC-010 · Service: Personal-plan assignee checks

## Metadata

```yaml
spec_id: SPEC-010
initiative_id: INIT-004
title: "Personal-plan assignee checks"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 2 hours
blocked_by: [SPEC-003]
blocks: [SPEC-020]
```

---

## Amended 2026-10-09 by INIT-006/SPEC-001

The learner snapshot this spec used to build is superseded by INIT-006 (ADR-0009 §1, Accepted 2026-10-09). The service for it is INIT-006 SPEC-007, and its MCP tool and switch route are INIT-006 SPEC-010. What this spec used to specify is ADR-0004 §1, whose text is unchanged. Only the personal-plan assignee checks stay here. Effort went from 4h to 2h. The model went from `opus` to `sonnet`, because the snapshot contract was the reason for `opus`. The old dependency on SPEC-006 is replaced by SPEC-003, because the checks call `ops.js` and read `users`, not the switch.

---

## Description

**Context:** ADR-0003 §1 defines a personal plan as `audience: "personal"` with an `assigneeId`, and a shared plan as `audience: "shared"` with `assigneeId: null`. ADR-0005 §5 gives `server/plans/personal.js` one job: check the account, then call `ops.js`. It does not reimplement an operation.

**Scope (in):**

- `server/plans/personal.js` sets `audience` and `assigneeId` on a plan by calling `ops.setField` or `replacePart`. It writes no SQL to a plan table.
- An `assigneeId` that names no active account is refused, and the plan is unchanged.
- Setting `audience` to `shared` sets `assigneeId` to `null` in the same operation.
- The module has no learner progress in it. Reading what a learner knows is INIT-006 SPEC-007.

**Scope (out):** any learner data, the MCP tool, the disclosure sentence, and the AI-planning switch (INIT-006); plan HTTP routes (SPEC-004).

---

## Understood (agent's read of the intent)

- [x] und-1: This module does not read `review_items`, `practice_events`, or `lesson_progress`.
- [x] und-2: This module does not call an AI provider.

## Acceptance Criteria

- [x] ac-1 (REQ-002): setting `audience` and `assigneeId` on a plan goes through `ops.setField` or `replacePart`. A spy shows `personal.js` did not write SQL to the plan table itself.
- [x] ac-2 (REQ-002): an `assigneeId` for a disabled or unknown account is refused with a named error, and the stored revision and document are unchanged.
- [x] ac-3 (REQ-002): setting `audience` to `shared` leaves `assigneeId` null, and the plan validates against the ADR-0003 schema.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Where the snapshot went | INIT-006 SPEC-007 builds the learner profile that supersedes ADR-0004 §1's snapshot. This spec keeps only the assignee checks. | resolved | yes — INIT-006/SPEC-001 |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/plans/personal.js`
- [x] Tests: `test/personal-plan.test.js`

---

## Technical Requirements (domain-specific)

- [x] Read accounts through the existing user repository. Do not select `password_hash` or `display_name`.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/personal-plan.test.js` and `npm test`.
- **Rollback plan:** delete the module. Tests use a temporary database.

---

## Security

- **Sensitivity:** required. Triggers matched: authorization of who may be assigned a personal plan.
- **Security acceptance criteria:**
  - [x] A plan cannot be assigned to a disabled or missing account.
  - [x] The module does not write plan SQL itself, so revision checks are never bypassed.
- **Review depth:** standard · **focus areas:** authz of assignment, revision checks.

---

## Verification Strategy

- **Claim:** assignment goes through the shared operations and refuses a bad assignee.
- **Check + executor:** `mechanized` — `node --test test/personal-plan.test.js`.
- **Pass condition:** exit 0; the spy records zero direct plan writes; the refused case leaves the revision unchanged.
- **Test levels:** integration, temporary database. No network. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- Plan HTTP routes (SPEC-004) and plan MCP tools (SPEC-005) reach a personal plan through `ops.js`. This module is the assignment helper, and SPEC-020's rollout checks it.
- Produces **Change:** assignment helper.
- **Provenance tag:** `INIT-004/SPEC-010`
