# SPEC-017 · Docs: Operator guide, skill update, rollout runbook

## Metadata

```yaml
spec_id: SPEC-017
initiative_id: INIT-008
title: "Teach the AI the AI-first loop and the new tools; tell the operator what history, restore, and issues do and what the AI can now do alone; write the migration runbook"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-016]
blocks: []
```

---

## Description

**Context:** `.cursor/skills/hebrew-worksheet/SKILL.md` is how the operator's AI builds lessons. Today it says "There is no tool that creates a plan id. Edit a plan that is already imported. Ask the operator before expecting a create tool.", lists seven plan tools, and says "Do not call `publish_plan` unless the operator asks in this conversation" (the sixth of the six steps in "Build the next lesson", and a convention, not a rule the server enforces). INIT-008 changes all three. `docs/operations/accounts-and-admin.md` is the operator guide; `docs/operations/language-program-rollout.md` is the pattern for a rollout runbook (backup, apply migrations at `npm start`, verify, roll back). ADR-0011 and SPEC-009's tool names (as registered) are the inputs.

**Scope (in):**

- **Skill** (`.cursor/skills/hebrew-worksheet/SKILL.md`, and `recipes.md` if the skill grows too long):
  - Replace the "no create tool" paragraph with the AI-first loop: `list_plans` → `create_plan` (or open an existing plan) → build with the six operations → `list_plan_issues` → fix with `set_field` or `replace_part`, using `fix.suggested` where given → `publish_plan`.
  - Add the eight new tools to the tool table with the exact registered names: `list_plans`, `create_plan`, `list_plan_revisions`, `get_plan_revision`, `diff_plan`, `restore_plan_revision`, `list_plan_issues`, `assign_plan`.
  - Add "Read before you edit": call `read_part` first; its `editState` lists fields a person last changed and the value they replaced. It is advice, not a refusal. If you overwrite one, your result lists it under `overwrote`; read it and, if the person's value was right, call `restore_plan_revision` for that field. Prefer to keep a person's fix unless you have a reason, and say what the reason is.
  - Add the issue codes, one line each: `pointing-form`, `field-language`, `lexicon-form`, `pointing-consistency`, `audio-stale`, and note that they are warnings that never block publish, and that `lexicon-form` is authoritative only when the lexicon entry is `verified` (and only a person verifies).
  - Replace the publish sentence: the AI may publish when `list_plan_issues` shows no hard error. **Guidance, not enforcement:** keep a new lesson `hidden` unless the operator asked you to expose it, because `visibility` is a field you can set and exposing a lesson reaches learners. Say what you published and what warnings remain, in the reply.
  - Keep, unchanged: the human-only acts (`set_lexicon_verified` and `set_ai_planning` return `forbidden`; picture approval is a person's); no learner data in a plan; the audio inventory and verify commands; no edit to a lesson's `index.html`; the `1.1.0` rule that a block or `recipe` raises `schemaVersion`.
  - State the legacy line once: `upsert_lesson` is for a worksheet that has not moved onto a plan; prefer `create_plan` for a new lesson (initiative §16 Q3: a recommendation, not a refusal).
  - State the audio line once: a person's text fix leaves the old clip attached; `audio-stale` says so; the regenerate path is the one SPEC-008 verified.
- **Operator guide** (`docs/operations/accounts-and-admin.md`): a new "Plan builder" section: the Plans list and New plan; the editor and what the form is built from; History, Compare, and Restore, and that restoring is itself a new revision; the Issues panel and what warnings mean; the live preview and that it never reaches a learner; what the AI can now do **without you** (create, edit, publish, set `visibility`, restore) and what it still cannot (verify a form, approve a picture, turn on a learner's AI-planning switch); the actor limitation in plain words (every AI client shows as `mcp`; your own edits show your account name; the AI is not told your name); where to look when something looks wrong (History, then Restore); and the rule that history is never edited.
- **Rollout runbook** (`docs/operations/plan-builder-rollout.md`, new, on the pattern of `language-program-rollout.md`): rules on every step (secret names only, never a value; PowerShell locally and `kubectl exec` on the cluster); step 0 back up with `node scripts/backup-db.js` (or `npm run db:backup` for the media bundle) and take `npm run audio:inventory`; step 1 save row counts with `node scripts/rollout-preflight.js counts --out <file>`; step 2 deploy the new image, whose start applies migration `006` (the operator applies it, outside any spec; the `db_migration` gate was waived by the operator on 2026-10-12, so there is no sign-off, and the step 0 backup is still good practice); step 3 verify `schema_migrations` lists 6 and the two tables exist and are empty, `plans` rows unchanged, and `rollout-preflight check` passes; step 4 smoke: open the Plans list, make one small edit to a draft plan and see one History row (the baseline plus the edit); step 5 `npm run audio:verify`; rollback: roll the previous image (the new tables stay unused) or run `down` from migration 006 deliberately; last resort, restore the backup. State plainly that the operator's no-human-in-the-loop direction applies to authoring and that the `db_migration` gate was waived by the operator on 2026-10-12 (local-only project, no sign-offs; a backup first is still good practice).
- **Initiative follow-ups:** none edited here; the completion report (`complete-initiative`) records them.

**Scope (out):** code; any change to INIT-004 to INIT-007 documents; the ADR (SPEC-001).

---

## Understood (agent's read of the intent)

- [x] und-1: The skill is guidance. The server enforces the hard rules whatever the skill says.
- [x] und-2: The operator guide is honest about the new power: the AI can publish and expose a lesson alone. It says how to see and undo it.
- [x] und-3: Examples use `example-learner` and `example-admin`. No key assignment appears anywhere.

## Acceptance Criteria

- [x] ac-1 (REQ-015): the skill names every new tool exactly as registered (`list_plans`, `create_plan`, `list_plan_revisions`, `get_plan_revision`, `diff_plan`, `restore_plan_revision`, `list_plan_issues`, `assign_plan`), the loop in order, and the five issue codes. The sentence "There is no tool that creates a plan id" and the instruction not to publish unless asked no longer appear unqualified. _(Met. The eight names are checked against the list the end-to-end test and `test/plan-builder-mcp.test.js` read from a real client. A search finds neither old sentence. The skill's "create_plan first" rule is stated in the Default path and again in the legacy section, as a recommendation.)_
- [x] ac-2 (REQ-015, REQ-006): the skill tells the AI to read `editState` before editing a part, states that it is advisory and that nothing is refused, and names `overwrote` and `restore_plan_revision`. _(Met. "Read before you edit" names `editState`, `lastHumanEdit`, `previousValue`, `previousWriter`, `overwrote`, `humanRevision`, and `restore_plan_revision`.)_
- [x] ac-3 (REQ-015, REQ-019): the skill and the operator guide both state that the AI may publish and set `visibility` without a person, that the skill's advice to keep a lesson `hidden` is guidance and not enforced, and that `verified`, picture approval, and the AI-planning switch remain a person's. _(Met in the skill's "Publishing" and "Build the next lesson" step 6, and in the guide's "What the AI can do without you".)_
- [x] ac-4 (REQ-015): the operator guide's "Plan builder" section covers every item listed above, states the actor limitation in the words above, and states that history is never edited. _(Met. Sections: Plans list and New plan; The editor; History, Compare, and Restore; The Issues panel; Preview, publish, and assign; What the AI can do without you; Who made a change; When something looks wrong.)_
- [x] ac-5 (REQ-015, REQ-016): `docs/operations/plan-builder-rollout.md` has steps 0 to 5 and the rollback paths, names `node scripts/backup-db.js`, `node scripts/rollout-preflight.js`, `npm run audio:inventory`, and `npm run audio:verify`, says that the `db_migration` gate was waived by the operator on 2026-10-12 (local-only project, no sign-offs) and that a backup first is still good practice, and contains no secret value. _(Met. The two one-line `node -e` commands in steps 3 and the rollback were run by the executor against a temporary database (not the real one). The real-database steps (the backup, `docker compose restart hebrew`, the verification) were **not** run by the executor; the operator runs them.)_
- [x] ac-6 (REQ-017): none of the three files contains `ELEVENLABS_API_KEY=`, `OPENAI_API_KEY=`, `MCP_AUTH_TOKEN=`, or `ADMIN_PASSWORD=`, a real username, learner data, or teacher text. _(Met by search. The only account names are `example-learner` and `example-admin`.)_

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | The skill's publish advice | The AI may publish; advice to keep new lessons hidden is guidance (D1). | resolved | yes — operator D1 |
| aud-2 | Legacy worksheets | Recommend `create_plan` for new lessons; do not remove or refuse `upsert_lesson` (initiative §16 Q3). | ambiguous | no — planning default applied |
| aud-3 | Cluster commands | The runbook mirrors `language-program-rollout.md`'s PowerShell and `kubectl exec` forms; the executor copies the exact commands from that file and does not invent new ones. | resolved | yes |

> **Readiness** = 100 against the stated defaults.

---

## Deliverables

- [x] Updated `.cursor/skills/hebrew-worksheet/SKILL.md` (and `recipes.md` if needed) _(SKILL.md only; `recipes.md` needed no change)_
- [x] Updated `docs/operations/accounts-and-admin.md`
- [x] New `docs/operations/plan-builder-rollout.md`
- [x] Tests: none (documentation). The verification checklist below. _(The searches in the spec's verification strategy were run; the operator's read of the loop and the "Plan builder" section is still open.)_

---

## Technical Requirements (domain-specific)

- [x] Tool names, route names, and issue codes are copied from the registered code, not retyped from memory.
- [x] Examples use `example-learner` and `example-admin`.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the verification searches.
- **Rollback plan:** revert the three files.

---

## Security

- **Sensitivity:** light. Instructions for sensitive tools; no secrets or real data.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** an AI following the skill builds, checks, fixes, and publishes a plan with the new tools and reads a person's edits before overwriting them, and the operator can find, understand, and undo what it did.
- **Check + executor:** `mechanized` — searches that every new tool name and every issue code appears in the skill; that `list_plan_issues` and `restore_plan_revision` appear in the operator guide; that the two replaced sentences are gone; that the runbook names the four commands; that no `_API_KEY=`, `MCP_AUTH_TOKEN=`, or `ADMIN_PASSWORD=` appears. `human` — the operator reads the loop and the "Plan builder" section.
- **Pass condition:** the searches pass; the operator accepts.

---

## Integration Points

- Tool names from SPEC-009; routes and screens from SPEC-010 to SPEC-015; the loop from SPEC-016; the issue codes from SPEC-008.
- Produces **Change:** the skill, the guide section, the rollout runbook.
- **Provenance tag:** `INIT-008/SPEC-017`
