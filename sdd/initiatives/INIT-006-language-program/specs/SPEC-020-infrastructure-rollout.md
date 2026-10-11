# SPEC-020 · Infrastructure: Migration and rollout

## Metadata

```yaml
spec_id: SPEC-020
initiative_id: INIT-006
title: "Backup, migrate, switch scheduler to FSRS, verify stage counts and audio; k8s and compose notes"
domain: infrastructure
status: complete
primary_prompt: .claude/agents/principal-devops-engineer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: one_way
security_review: standard
estimated_effort: 3 hours
blocked_by: [SPEC-012, SPEC-013, SPEC-015, SPEC-016, SPEC-017, SPEC-018, SPEC-019]
blocks: []
```

---

## Description

**Context:** INIT-006 §14 is the step table. The site runs in Docker locally (`docker-compose.yml`) and in k8s (`deploy/k8s-database.md`, `deploy/k8s-lessons-volume.md`) with one replica and `Recreate`. `DATA_DIR/hebrew.sqlite` runs in WAL mode.

**Scope (in):**

- `scripts/backup-db.js`: checkpoint WAL, copy `hebrew.sqlite` to `DATA_DIR/backups/hebrew-<ISO date>.sqlite`, print the path and size (never contents).
- `scripts/stage-counts.js`: print per-user `new` / `learning` / `longTerm` counts per direction (usernames only), for before/after comparison.
- Runbook in `docs/operations/language-program-rollout.md`: backup → `audio:inventory` → stage counts (before) → start new image (migrations run) → stage counts (after, must match) → `SRS_SCHEDULER=fsrs` → restart → flashcard smoke → `audio:verify`. Rollback lines for each step.
- `SRS_SCHEDULER` added to `.env.example` (with a comment), `docker-compose.yml` environment pass-through, and both k8s docs (plain env, not a secret).
- Confirm no new secret is needed; `MCP_AUTH_TOKEN` remains required.

**Scope (out):** INIT-004's rollout (its SPEC-020).

---

## Understood (agent's read of the intent)

- [x] und-1: The scheduler switch is the only behavior change at deploy; everything else is additive.

## Acceptance Criteria

- [x] ac-1 (REQ-016): on a copy of a real-shaped database, the runbook's before and after stage counts are equal.
- [x] ac-2 (REQ-016): `audio:verify` matches the inventory; 65 recordings at the same paths.
- [x] ac-3 (REQ-016): rollback by `SRS_SCHEDULER=ladder` restores ladder scheduling (smoke grades a card and checks `step` changed).
- [x] ac-4: backup script output contains no row data; `.env.example` has no value for any secret.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Backup location in k8s | Same data volume, `backups/` subfolder, plus the cluster's existing volume backup. | resolved | yes — ADR-0005 §8 |

> **Readiness** = 100.

---

## Deliverables

- [x] `scripts/backup-db.js`, `scripts/stage-counts.js`
- [x] `docs/operations/language-program-rollout.md`
- [x] Edits to `.env.example`, `docker-compose.yml`, `deploy/k8s-database.md`, `deploy/k8s-lessons-volume.md`
- [x] Tests: `test/rollout-scripts.test.js`

---

## Technical Requirements (domain-specific)

- [x] Scripts use `node:sqlite`; no dependency.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** `deploy` — operator runs or approves each production step.
- **Blocking checks:** `node --test test/rollout-scripts.test.js`, `npm test`, all Chrome smokes, `npm run audio:verify`.
- **Rollback plan:** the runbook's per-step rollback; restore the dated backup as the last resort.

---

## Security

- **Sensitivity:** standard. No secret changes; backups stay on the data volume, outside served roots.
- **Review depth:** standard · **focus areas:** backup location not served, no secrets in docs.

---

## Verification Strategy

- **Claim:** the rollout preserves learners' stages, recordings, and lessons, and can fall back.
- **Check + executor:** `mechanized` — tests and smokes; `human` — operator confirms production before/after counts.
- **Pass condition:** counts equal; audio verify passes; smokes exit 0.
- **Test levels:** integration, browser smoke, production check.

---

## Integration Points

- Last spec. Feeds `complete-initiative`.
- Produces **Change:** rollout.
- **Provenance tag:** `INIT-006/SPEC-020`
