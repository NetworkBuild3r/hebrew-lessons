# SPEC-006 · Data: Media assets and the AI-planning switch

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-004
title: "Media asset rows and the AI-planning switch"
domain: data
status: complete
primary_prompt: .claude/agents/database-architect/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: one_way
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: [SPEC-008]   # INIT-006 SPEC-007 and SPEC-010 also need the `users.ai_planning` column (amended 2026-10-09)
```

---

## Description

**Context:** ADR-0004 stores image records in the account database and a per-learner switch that defaults off. Bytes themselves are files (SPEC-008). This spec is the rows.

**Scope (in):**

- Next migration after SPEC-001's file. Apply and down.
- A media table: id (the slug the plan stores), stored filename (random, not the upload name), status `pending` or `approved`, alt text, prompt text nullable, provider name nullable, model id nullable, actor, timestamps. The original client filename is not a column.
- `users.ai_planning` integer or boolean, default off, for every existing row and every later insert. The column addition is the only change to `users`. Down removes that column and drops the media table.
- Repository functions in `server/db/repos/` following the existing repo style: get and set the switch, insert and update a media row. No file IO.

**Scope (out):** sniff and re-encode (SPEC-008); the learner profile query (INIT-006 SPEC-007); HTTP for media (SPEC-011) and for the switch (INIT-006 SPEC-010).

---

## Understood (agent's read of the intent)

- [x] und-1: Default off means a user created after this migration also starts off. The default lives in the schema, not only in one update statement.
- [x] und-2: Prompt text is on the media row, as ADR-0004 §7, and is not copied into `audit_log` by this spec.

## Acceptance Criteria

- [x] ac-1 (REQ-008): after migrate, every pre-existing user has the switch off. Inserting a user without naming the column yields off.
- [x] ac-2 (REQ-006): a media row can be stored without an original filename column. A test that selects the column list fails if `filename` or `original_name` is present.
- [x] ac-3 (REQ-001): the down migration restores `users` without `ai_planning` and drops the media table. Plan tables from SPEC-001 remain. Counts of `lesson_progress` are unchanged.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Switch storage | A column on `users`, not a second table. One account, one flag. | resolved | yes — ADR-0004 §3 "one account at a time" |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/db/migrations/003-media-and-ai-planning.js`
- [x] Repo helpers next to `server/db/repos/users.js`
- [x] Tests: `test/media-switch-store.test.js`

---

## Technical Requirements (domain-specific)

- [x] Bound parameters. The switch write records previous and new value only when INIT-006 SPEC-010 writes the audit row; this spec exposes both values to that caller.
- [x] No secret and no learner display name in fixtures. Usernames in tests are the synthetic names the existing auth tests already use.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`. Applying to a database outside the test waits on the operator.
- **Gate actions triggered:** `db_migration`.
- **Blocking checks:** `node --test test/media-switch-store.test.js` and `npm test`. `npm run audio:verify`.
- **Rollback plan:** down migration. Media files are not created by this spec, so there is nothing to archive.

---

## Security

- **Sensitivity:** required. Triggers matched: learner consent flag, prompt text that will later be sent to a provider.
- **Security acceptance criteria:**
  - [x] Existing accounts are off. A test inserts no "on" default.
  - [x] The client-supplied filename cannot be persisted because the column does not exist.
- **Review depth:** thorough · **focus areas:** PII, schema default, migration down.

---

## Verification Strategy

- **Claim:** the switch defaults off for old and new accounts, and media rows cannot store the upload filename.
- **Check + executor:** `mechanized` — `node --test test/media-switch-store.test.js`.
- **Pass condition:** exit 0; the column-list assertion and the default-off assertion both run.
- **Test levels:** integration, temporary database copied from the initial migration plus SPEC-001. No coverage tool is configured; the bar is this file plus `npm test`.

---

## Integration Points

- SPEC-008 writes media rows after ingest. INIT-006 SPEC-007 reads the switch, and INIT-006 SPEC-010 is the only HTTP writer of it.
- Produces **Change:** media table and `users.ai_planning`.
- **Provenance tag:** `INIT-004/SPEC-006`
