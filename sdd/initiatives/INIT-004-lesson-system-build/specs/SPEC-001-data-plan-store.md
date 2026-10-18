# SPEC-001 · Data: Plan store, working copy, revision, and snapshot

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-004
title: "Plan store: working copy, revision, and snapshot"
domain: data
status: complete
primary_prompt: .claude/agents/database-architect/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: one_way
security_review: standard
estimated_effort: 4 hours
blocked_by: []
blocks: [SPEC-003, SPEC-006]
```

---

## Description

**Context:** ADR-0005 §5 stores the plan working copy, revision, status, and published snapshot in new tables in `hebrew.sqlite`, through `server/db/`. Accounts and progress stay in the tables at `origin/main:server/db/migrations/001-initial.js`.

**Scope (in):**

- The next migration file after the highest number already in `server/db/migrations/`. Do not guess the number; read the directory at execution.
- Tables for one plan document: working-copy JSON, integer `revision` starting at 1, `status` (`draft` or `published`), published-snapshot JSON nullable until the first publish, `audience`, `assignee_id` nullable.
- Apply and down. Down drops only these new tables.
- A repository module `server/plans/store.js` that is the only plan SQL: load working copy, save working copy, load snapshot, save snapshot. No mutation rules in this spec (SPEC-003).

**Scope (out):** importing the sample plans (SPEC-007); media tables and the AI-planning column (SPEC-006); HTTP and MCP.

---

## Understood (agent's read of the intent)

- [x] und-1: The document is one JSON blob per plan, not a row per block. Part operations edit the blob. Stable ids live inside the JSON, as ADR-0003 §1.
- [x] und-2: Existing tables are not altered. A down migration does not drop `users`, `lesson_progress`, `review_items`, or `audit_log`.

## Acceptance Criteria

- [x] ac-1 (REQ-001): after migrate, a plan can be inserted as `draft` at revision 1 with a null snapshot, then read back with the same JSON bytes.
- [x] ac-2 (REQ-001): the down migration removes the new tables and leaves the INIT-002 tables and their rows in place. A test asserts row counts of `users` and `lesson_progress` before and after.
- [x] ac-3 (REQ-014): the migration does not write audio paths, and it does not open `LESSONS_DIR`.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Blob versus normalized block rows | One JSON document per plan, matching ADR-0003 §3 | resolved | yes — ADR-0003 |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/db/migrations/002-plan-store.js` (apply and down)
- [x] `server/plans/store.js`
- [x] Tests: `test/plan-store.test.js` on a temporary database

---

## Technical Requirements (domain-specific)

- [x] Follow `server/db/migrations/001-initial.js` style: `node:sqlite` through the existing wrapper, a `schema_migrations` row, timestamps in the same form that file uses.
- [x] `assignee_id` is the account username ADR-0003 describes (the users-table rule `username = lower(username)`, length 1–64) or null. It is not a display name. Migration 012 relaxed the original 8-character `[a-z0-9-]` check.
- [x] No lesson text fixtures that contain a learner name, a transcript, or a secret.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`. The agent writes the migration and tests. Applying it to a shared or production database waits for the operator.
- **Gate actions triggered:** `db_migration`.
- **Blocking checks:** `node --test test/plan-store.test.js` and `npm test` on the temporary-database suite. `npm run audio:verify` against the current inventory, because this spec shares the repo with the lessons tree even though it does not read it.
- **Rollback plan:** run the down migration. It drops only the new tables.

---

## Security

- **Sensitivity:** standard. Triggers matched: new persistent document that will later hold personal-plan assignee ids. The snapshot of learner progress is not stored here.
- **Security acceptance criteria:**
  - [x] SQL uses the repository's bound parameters.
  - [x] The down path cannot drop a pre-existing table.
- **Review depth:** standard · **focus areas:** migration safety, no secrets in fixtures.

---

## Verification Strategy

- **Claim:** the new tables round-trip a plan document, and the down migration leaves accounts and progress rows intact.
- **Check + executor:** `mechanized` — `node --test test/plan-store.test.js`, then `npm test`.
- **Pass condition:** both commands exit 0; the down-migration test's before and after counts for `users` and `lesson_progress` are equal.
- **Test levels:** integration against a temporary SQLite file. No property-based test: the schema is one document shape. No coverage tool is configured (`origin/main:package.json` test script is `node --test test/*.test.js`); the bar is these tests plus the existing suite.
- **Test data:** a minimal valid plan object built in the test, not a copied worksheet.

---

## Integration Points

- SPEC-003 is the only writer of revision and snapshot contents. This spec only stores bytes.
- Produces **Change:** additive plan tables and `server/plans/store.js`.
- **Provenance tag:** `INIT-004/SPEC-001`
