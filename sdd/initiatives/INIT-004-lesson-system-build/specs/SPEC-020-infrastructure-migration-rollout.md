# SPEC-020 · Infrastructure: Migration and rollout

## Metadata

```yaml
spec_id: SPEC-020
initiative_id: INIT-004
title: "Migration and rollout"
domain: infrastructure
status: complete   # 2026-10-09; no learner link changed; the deploy gate stays open (GATE REQUEST in the completion summary)
primary_prompt: .claude/agents/principal-devops-engineer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: one_way
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-007, SPEC-010, SPEC-014, SPEC-016, SPEC-017, SPEC-018, SPEC-019]   # SPEC-010 added 2026-10-09 (INIT-006/SPEC-001)
blocks: []
```

---

## Description

**Context:** the initiative §14 table is the plan. This spec runs the checks and the cutover gate. It does not re-implement the modules. ADR-0005 §8: one screen at a time, same process, same image.

**Scope (in):**

- A rollout checklist committed at `docs/operations/lesson-plan-rollout.md` that copies the initiative §14 steps, each with its rollback.
- At the start: `npm run audio:inventory` writing a new file under `data/audio-inventory/`. Record the path in the checklist run log (the log stays out of git if `data/` is gitignored).
- Confirm the four plan rows from SPEC-007 exist and the three worksheet `index.html` files are unmodified.
- Confirm `users`, `lesson_progress`, and `review_items` are non-empty if they were non-empty before the migration (compare counts; do not print row contents).
- Confirm the eight legacy MCP tools still answer a tokened `list_lessons`.
- Cutover of one learner link happens only after the operator writes approval at the `deploy` gate, naming the lesson id. Until then the link stays on the worksheet.
- After any change this spec makes: `npm run audio:verify` against the inventory from the start. It must report the same 65 paths.
- No step deletes an `.mp3`. No step copies recordings into the image. `replicas` stays 1.

**Scope (out):** writing plan code; flipping the AI-planning switch for a learner (INIT-006 builds that route and the admin toggle); retiring every worksheet in one change.

---

## Understood (agent's read of the intent)

- [x] und-1: Default cutover is none. The checklist can finish with every learner still on worksheets.
- [x] und-2: Retiring a worksheet file is a later operator act, after that lesson's snapshot is what learners open and the Chrome script for that screen is green. This spec does not delete the file.

## Acceptance Criteria

- [x] ac-1 (REQ-013): the checklist has a rollback sentence for each initiative §14 step, and none of those sentences say to delete a recording without an archive. The steps this initiative actually runs do not overwrite recordings at all.
- [x] ac-2 (REQ-013): `npm run audio:verify` against the pre-rollout inventory exits 0. The recording count is 65 and the relative paths match the inventory.
- [x] ac-3 (REQ-013): a dry run of the checklist (no link change) leaves `worksheets/*/index.html` unmodified and leaves `lesson_progress` row count unchanged.
- [x] ac-4 (REQ-014): the test disposition in the checklist matches initiative §14: current `npm test` files kept, new tests added beside them, no file retired under Option A.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Who approves a link change | Brian Nelson, in writing, per lesson. | resolved | yes — initiative §5 and §14 |

> **Readiness** = 100.

---

## Deliverables

- [x] `docs/operations/lesson-plan-rollout.md`
- [x] A small check script `scripts/rollout-preflight.js` that exits non-zero if the inventory verify fails, if a worksheet `index.html` hash changed against git, or if plan rows for the three legacy ids are missing
- [x] Tests: `test/rollout-preflight.test.js` on a temporary fixture, not on the live database

---

## Technical Requirements (domain-specific)

- [x] The preflight script prints counts and paths. It does not print usernames, progress JSON, or keys.
- [x] It does not open a socket.

---

## Gates & Controls

- **Autonomy level:** `auto_tests` for the checklist and the preflight script. Changing a learner-facing link is the gate.
- **Gate actions triggered:** `deploy`.
- **Blocking checks:** `node --test test/rollout-preflight.test.js`, `npm test`, `npm run audio:inventory`, `npm run audio:verify`.
- **Rollback plan:** for a link that was switched, point it back at the worksheet. The worksheet file is still there. For a checklist-only run, there is nothing to undo.

---

## Security

- **Sensitivity:** standard. Triggers matched: production cutover of who can see a lesson. Personal data is not dumped by the preflight.
- **Security acceptance criteria:**
  - [x] Preflight output has no snapshot body and no key.
  - [x] A lesson link does not change without the gate.
- **Review depth:** standard · **focus areas:** rollback, data loss, auth still on after cutover.

---

## Verification Strategy

- **Claim:** a rollout dry run keeps the 65 recordings, the worksheet files, and the progress rows, and every step has a rollback.
- **Check + executor:** `mechanized` — `node --test test/rollout-preflight.test.js` and `npm run audio:verify`. `human` — the operator reads the checklist and signs the deploy gate before any link changes.
- **Pass condition:** verify exits 0 on the 65-file inventory; the preflight test exits 0; the checklist's rollback column is non-empty for every step; no learner link changed unless the gate record names that lesson.

---

## Integration Points

- Last spec. It consumes the import, the renderers, the builder, the volume, the skill, and the operator guide.
- Produces **Change:** rollout checklist and preflight script.
- **Provenance tag:** `INIT-004/SPEC-020`
