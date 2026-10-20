# SPEC-003 · Service: Plan operations, store adapter, and publish

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-004
title: "Plan operations, store, and publish"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-001, SPEC-002]
blocks: [SPEC-004, SPEC-005, SPEC-007]
```

---

## Description

**Context:** ADR-0005 §5 names one module both callers import. The spike's `apply` / `applyBody` is the behavior to port. The public surface is the six functions plus `publishPlan`, not a second editor inside the HTTP or MCP layers.

**Scope (in):**

- `readPart`, `replacePart`, `insertBlock`, `moveBlock`, `removeBlock`, `setField` in `server/plans/ops.js`.
- `publishPlan` in `server/plans/publish.js`: copy the working document onto the snapshot, set `published`, increment `revision`.
- Address by the id paths in ADR-0003 §4, not by a raw array index from the client, except `insert_block` and `move_block`, which take the destination index the ADR names.
- Every mutation loads through `store.js`, checks `expectedRevision` against the working copy, runs `validate.js`, and saves. A mismatch returns `{ error: "revision_conflict", revision: <stored> }` and does not write.
- A successful mutation increments `revision` by 1. A part edit after publish sets `status` back to `draft` and leaves the previous snapshot in place for learners (ADR-0003 §7).
- Publish refuses when validation returns any block failure.

**Scope (out):** HTTP status codes and sessions (SPEC-004); MCP tool registration (SPEC-005); creating a plan id (open; not this spec).

---

## Understood (agent's read of the intent)

- [ ] und-1: Adapters must not parse a path and edit JSON themselves. They call these functions.
- [ ] und-2: There is no merge. The caller reads again.

## Acceptance Criteria

- [x] ac-1 (REQ-002): each of the five mutating operations (`replacePart`, `insertBlock`, `moveBlock`, `removeBlock`, `setField`) updates only the addressed part, increments revision by 1, and a second call with the old revision returns `revision_conflict` and leaves the stored JSON byte-identical to the first result. `readPart` returns the addressed part and does not write or increment (ADR-0003 §4: read has no mutation; "Every successful mutation increments"). Amended 2026-10-09 after verifier vs ADR conflict — do not make a read increment.
- [x] ac-2 (REQ-002): `publishPlan` copies the working document to the snapshot and increments revision. A following `setField` changes the working copy, sets `draft`, and leaves the snapshot bytes unchanged.
- [x] ac-3 (REQ-003): a publish of a document with a block-severity failure does not write a snapshot.
- [x] ac-4 (REQ-015): the functions behave as the spike's conflict rule: the caller's plan object is unchanged on conflict. A test ports that case.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Internal shape | One internal apply used by the six exports is still one implementation. Two copies of the edit logic are not. | resolved | yes — ADR-0005 "One implementation" |

> **Readiness** = 100.

---

## Deliverables

- [ ] `server/plans/ops.js`
- [ ] `server/plans/publish.js`
- [ ] Tests: `test/plan-ops.test.js`

---

## Technical Requirements (domain-specific)

- [ ] Stateless apart from the store passed in. No HTTP types in this module.
- [ ] `setField` accepts one field name under the path (`title`, `meaning`, `alt`, `newWords`, and the other fields the schema allows). Unknown field names fail validation rather than being stored (additional properties fail).
- [ ] Personal `audience` and `assigneeId` are ordinary fields on the document. This module does not read learner progress.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none. The migration already landed in SPEC-001; these tests use a temporary database.
- **Blocking checks:** `node --test test/plan-ops.test.js` and `npm test`.
- **Rollback plan:** revert the module. Stored rows in a temporary test database are discarded with the test.

---

## Security

- **Sensitivity:** standard. Triggers matched: untrusted JSON merged into a document other learners will later render. Authorization is the adapters' job.
- **Security acceptance criteria:**
  - [ ] A conflict path performs no `UPDATE`.
  - [ ] Block-severity HTML is not persisted by publish.
- **Review depth:** standard · **focus areas:** lost updates, validation before write.

---

## Verification Strategy

- **Claim:** one revision sequence rejects a stale writer, and publish freezes the snapshot the learner will read.
- **Check + executor:** `mechanized` — `node --test test/plan-ops.test.js`.
- **Pass condition:** exit 0, including the byte-identity assertion on conflict and the snapshot assertion after a later edit.
- **Test levels:** integration on a temporary database. Contract test is SPEC-004 and SPEC-005 (both call these functions). No property-based test. No coverage tool is configured; the bar is this file plus `npm test`.
- **Test data:** `beginner-unit-1.plan.json` loaded into the temporary store.

---

## Integration Points

- SPEC-004 and SPEC-005 are adapters. SPEC-007 calls the store to insert documents, then these functions if it amends them.
- Produces **Change:** the only plan mutation implementation.
- **Provenance tag:** `INIT-004/SPEC-003`
