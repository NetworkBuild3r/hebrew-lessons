# SPEC-001 · service: Safe lesson visibility on a fresh database

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-014
title: "Safe lesson visibility on a fresh database"
domain: service
status: ready
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 3 hours
blocked_by: [None]
blocks: [None]
```

## Description

**Context:** `server/visibility.js` `seedVisibility` (around lines 76–102) runs once, when its meta flag is absent, and inserts `everyone` for every lesson folder that has a `worksheet.js`. ASMT-002 measured a fresh database and a learner could see a lesson titled "test" (`worksheets/8ee73fd9-c223-4e01-8dcb-5f0a784eed97/`) and a lesson whose card says it is hidden unless assigned (`worksheets/anal-sex-vocab/`). Lessons added after the first seed already default to hidden. REQ-001, REQ-002.

**Scope (in):**

- On a fresh database, insert `everyone` only for `my-morning` and `shai-date-indirect-speech`.
- Do not insert `everyone` for any other slug. Leave those lessons with no row, so the existing default (hidden) applies.
- If the seed meta flag is already set, change nothing: do not update or delete existing visibility rows.
- Delete the stray folder `worksheets/8ee73fd9-c223-4e01-8dcb-5f0a784eed97/`.
- Keep `worksheets/anal-sex-vocab/` on disk. It must not be seeded as `everyone`.

**Scope (out):** No admin UI change. No rewrite of a production database. No new API. The operator's look at production visibility is a note in the completion summary, not a code change.

## Understood (agent's read of the intent)

- [ ] und-1: A rebuilt or first-run database must not publish every folder it finds.
- [ ] und-2: The two real teaching lessons stay available to a new learner.
- [ ] und-3: An already-seeded database keeps whatever visibility an admin has set.

## Acceptance Criteria

- [ ] ac-1: On a temporary database with no seed flag, after startup, `my-morning` and `shai-date-indirect-speech` are visible to a learner, and `anal-sex-vocab` is not. — covered: no
- [ ] ac-2: On a temporary database that already has the seed meta flag and a row, startup does not change that row. — covered: no
- [ ] ac-3: `worksheets/8ee73fd9-c223-4e01-8dcb-5f0a784eed97/` is not in the tree, and no test or catalog entry still requires it. — covered: no
- [ ] ac-4: The allowlist is a fixed list in `server/visibility.js`, not a value taken from a request. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Which lessons stay public on a fresh database | Only `my-morning` and `shai-date-indirect-speech`. Chosen so a new install still has the two teaching lessons, and the private lesson stays hidden. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `server/visibility.js` — allowlist inside `seedVisibility`
- [ ] A node test that covers ac-1 and ac-2 on a temporary database
- [ ] Removal of `worksheets/8ee73fd9-c223-4e01-8dcb-5f0a784eed97/`
- [ ] Tests: the new test file plus any existing visibility test that asserted "every folder is everyone"

## Technical Requirements (domain-specific)

- [ ] Do not add a migration. The meta flag already stops a second seed.
- [ ] Do not log lesson titles or learner data from the seed.
- [ ] Provenance comment: `INIT-014/SPEC-001`.

## Gates & Controls

- **Autonomy level:** auto_tests. Edit and run tests. Do not commit.
- **Gate actions triggered:** none.
- **Blocking checks:** the new visibility test passes before the spec is marked complete.
- **Rollback plan:** revert the commit. Already-seeded databases were never rewritten, so rollback does not need a data fix.

## Security

- **Sensitivity:** required — triggers matched: authz (who may see a lesson), information disclosure.
- **Security acceptance criteria:**
  - [ ] A learner session on a fresh database receives no lesson body for `anal-sex-vocab`.
  - [ ] The allowlist cannot be extended by query string, POST body, or a lesson file's own text.
  - [ ] No secret is added or logged.
- **Review depth:** thorough · **focus areas:** access control, data disclosure.

## Verification Strategy

- **Claim:** a fresh database publishes only the two named lessons, and an already-seeded database is left as it was.
- **Check + executor:** mechanized — `node --test` on the new visibility test and the existing visibility tests.
- **Pass condition:** ac-1, ac-2, and ac-3 tests fail before the change and pass after, and the already-seeded case asserts the stored value is byte-identical after a second startup.
- **Declared unmechanized:** the operator's look at production visibility. Record the result in the completion summary if it was done. Absence of that look does not fail the code check; the summary must say the look was not done.

## Integration Points

- Learner lesson lists (`shared/lessons-page.js`) and Today read visibility through the existing server path. They do not change in this spec.
- Produces **Change:** fresh databases no longer publish every lesson folder.
- **Provenance tag:** `INIT-014/SPEC-001`
