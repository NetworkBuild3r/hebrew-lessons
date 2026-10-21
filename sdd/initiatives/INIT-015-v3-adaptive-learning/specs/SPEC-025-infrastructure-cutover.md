# SPEC-025 · infrastructure: Cutover to production

## Metadata

```yaml
spec_id: SPEC-025
initiative_id: INIT-015
title: "Cutover to production"
domain: infrastructure
status: in_progress
primary_prompt: .claude/agents/devops-engineer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: [deploy, db_migration, merge]
reversibility_class: one_way
security_review: required
estimated_effort: 3 hours
blocked_by: [SPEC-023, SPEC-024]
blocks: []
```

## Description

**Context:** `hebrew.ibhacked.us` and `hebrew-next.ibhacked.us` both answer `/placement` today. Which image each host runs, and which data volume it mounts, is set in home_k3, outside this repo. D7 assumes v3 keeps the SQLite file its host already uses and migrates it forward (006, 007, and the weekly migration).

**Scope (in):** a runbook, `docs/ops/v3-cutover.md`, and the checks it calls. Steps:

1. Confirm, from home_k3 and the running pods, which image and volume each host uses. If production runs `hebrew-lessons`, stop and record that D7's second branch applies (archive the v1 database, start fresh with a placement).
2. Back up the production SQLite file (`.backup` through `sqlite3`, with a checksum).
3. Dry run: copy the backup to staging, run v3 against it, and run `hebrew migrate --check` plus a smoke script (sign in, Today, one review, the placement start, `get_learner_model` over MCP).
4. Write the exact home_k3 change: the image pin or updater rule for the production app, set to the v3 `sha-` tag.
5. Rollback: the previous image tag, plus restoring the backup file. Both are written out as commands.
6. After cutover: turn on the SPEC-023 automation, and watch the first daily job.

**Scope (out):** Running any of these steps. This spec is `plan_only`. The operator, or an agent the operator names, runs the runbook.

## Understood (agent's read of the intent)

- [ ] und-1: One learner's whole history lives in one file. Nothing touches production without a verified backup and a tested rollback.

## Acceptance Criteria

- [x] ac-1: The runbook names every command, its expected output, and a stop condition for each step. — covered: no
- [ ] ac-2: The smoke script exists (`scripts/smoke-v3.sh`), and it passes against staging with a copy of production data. — covered: partial (script written, passes 7 of 7 against a local server; not run against staging, which needs the operator and a backup)
- [x] ac-3: The home_k3 change is written as a diff in the runbook and not applied. — covered: no
- [ ] ac-4: The rollback is rehearsed on staging: deploy the previous tag, restore the backup, and run the smoke script again. — covered: no (needs the operator; step 5.R of the runbook is the rehearsal)

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | D7, production data | Checked in step 1. The runbook has both branches. | assumed | no |

> **Readiness** = 85. aud-1 is resolved by step 1 at run time.

## Deliverables

- [ ] `docs/ops/v3-cutover.md`
- [ ] `scripts/smoke-v3.sh`
- [ ] `hebrew migrate --check` subcommand, if not present

## Technical Requirements (domain-specific)

- [ ] The smoke script reads its token from the environment and never echoes it.

## Gates & Controls

- **Autonomy level:** plan_only.
- **Gate actions triggered:** deploy, db_migration, and merge. Hebrew PR Reviewer merges the runbook PR. The operator runs the cutover.
- **Blocking checks:** SPEC-024 green, ac-2 and ac-4 done on staging.
- **Rollback plan:** in the runbook, rehearsed (ac-4).

## Security

- **Sensitivity:** required — triggers matched: production deploy, production data.
- **Security acceptance criteria:**
  - [ ] Backups are stored where only the operator can read them, not in the repo or the image.
  - [ ] The staging copy of production data is deleted after the dry run.
- **Review depth:** standard · **focus areas:** data handling, rollback.

## Verification Strategy

- **Claim:** the cutover can be run safely and undone.
- **Check + executor:** mechanized — the smoke script against staging, before and after the rehearsed rollback. Inspection — the runbook review.
- **Pass condition:** ac-1 to ac-4 hold, and the smoke script passes all three times (dry run, after rollback, and after re-deploy).

## Integration Points

- Reads `docs/ops/v3-hosts.md` (SPEC-002) and `docs/ops/weekly-coach.md` (SPEC-023).
- **Provenance tag:** `INIT-015/SPEC-025`
