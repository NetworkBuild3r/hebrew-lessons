# SPEC-017 completion summary

**Spec:** SPEC-017 · Docs: operator guide, skill update, rollout runbook  
**Initiative:** INIT-008  
**Completed:** 2026-10-12  
**Status:** in_review (documentation; the operator's read of the loop and of the "Plan builder" section is the open human check)  
**Provenance:** `INIT-008/SPEC-017`

## What shipped

- **`.cursor/skills/hebrew-worksheet/SKILL.md`** (the skill the operator's AI uses).
  - The sentence "There is no tool that creates a plan id…" is replaced by **new lessons are plans: start one with `create_plan`**, and an ordered **AI-first loop**: `list_plans`, `create_plan` (or open with `read_part`), build with the six operations, `list_plan_issues`, fix with `set_field` or `replace_part` using `fix.suggested`, `publish_plan`.
  - The tool table now lists all fifteen plan tools, the eight new ones by their registered names (`list_plans`, `create_plan`, `list_plan_revisions`, `get_plan_revision`, `diff_plan`, `restore_plan_revision`, `list_plan_issues`, `assign_plan`), with their arguments and the refusals the descriptions teach (`already_exists`, `id_mismatch`, `assign_required`, `assignee_refused`, `no_snapshot`).
  - New short sections: **a new plan starts with a placeholder block** (insert a real block first, then remove it; pass `unitId` and `lessonId`), **Read before you edit** (`editState`, `lastHumanEdit`, `previousValue`, `previousWriter`; advisory, nothing is refused; `overwrote`, `humanRevision`, `restore_plan_revision`; prefer to keep a person's fix and say why), **Issues** (the five codes, one line each; warnings never block a write or a publish; `lexicon-form` is authoritative only for a `verified` entry and only a person verifies; the shape of a `fix`), and **Publishing** (the AI may publish when there is no `block` finding; `validation` and `pending_media` still refuse; **guidance, not enforcement:** keep a new lesson `hidden` unless asked, because `visibility` is a field the AI can set; say what was published and which warnings remain).
  - "Build the next lesson" step 4 names `create_plan` and `list_plan_issues`; step 6 ("Save as a draft. Do not call `publish_plan` unless the operator asks") becomes "Check and publish", with the same guidance. The human-only acts (`set_lexicon_verified` and `set_ai_planning` answer `forbidden`; picture approval is a person's), the audio inventory and verify commands, "do not edit a lesson's `index.html`", and the `1.1.0` `schemaVersion` rule are unchanged.
  - The legacy line is stated in the Default path and in "Legacy worksheets": `upsert_lesson` is for a worksheet that has not moved (today, the three existing ones); prefer `create_plan` for a new lesson; a recommendation, not a refusal. The audio line is stated once ("Audio already in a plan"): a person's text fix leaves the old clip, `audio-stale` says so, and no tool regenerates audio for a plan field (SPEC-008's finding), so the fix is to leave the stale path out or set an existing new clip.
  - `recipes.md`, `schema.md`, and `example.md` were not changed.
- **`docs/operations/accounts-and-admin.md`**: a new **Plan builder** section (the Plans list and New plan; the editor and what the form is built from; History, Compare, and Restore, including that a restore is a new revision and that history is never edited; the Issues panel and the five codes; the preview and that it never reaches a learner; Publish and assign; **what the AI can do without you** and what it still cannot; **who made a change** with the actor limitation; where to look when something looks wrong). The MCP tools list gained the plan tools. Provenance line updated.
- **`docs/operations/plan-builder-rollout.md`** (new), on the pattern of `language-program-rollout.md`. It states first that the no-human-in-the-loop direction applies to authoring and that the **`db_migration` gate was waived by the operator on 2026-10-12** (local-only project, no sign-offs; a backup first is still good practice). Steps: **0** `npm run audio:inventory` and `node scripts/backup-db.js` (does not migrate); **1** `node scripts/rollout-preflight.js counts`; **2** restart so `server/start.js` applies migration `006` (`docker compose restart hebrew` locally; roll the image on the cluster); **3** verify (`schema_migrations` 1 to 6, `plan_revisions` and `plan_field_marks` empty, `plans` count unchanged, `rollout-preflight check`, `npm run db:check`); **4** a smoke check (edit `beginner-unit-1`'s lesson title, see the baseline and the human row in History, restore the value); **5** `npm run audio:verify`. A rollback table (previous image; run `down` of migration 006 deliberately; last-resort restore of the backup) and a last-resort restore paragraph.

## Checks run (mechanized, 2026-10-12)

- Every one of the eight new tool names appears in the skill (24 lines mention one of them); all five issue codes appear.
- Neither replaced sentence appears: no "There is no tool that creates a plan id", and no "Do not call `publish_plan` unless the operator asks".
- The runbook names `node scripts/backup-db.js`, `node scripts/rollout-preflight.js`, `npm run audio:inventory`, and `npm run audio:verify`, and says the gate was waived on 2026-10-12 and that a backup first is still good practice.
- None of the three files contains `ELEVENLABS_API_KEY=`, `OPENAI_API_KEY=`, `MCP_AUTH_TOKEN=`, `ADMIN_PASSWORD=`, a real account name, or a bearer value. The only account names are `example-learner` and `example-admin`.
- Tool names, routes, and codes were copied from `mcp/tools.js`, `server/api/plan-routes.js`, `server/plans/hebrew-checks.js`, and the completion summaries, not from memory. The `read_part`, `overwrote`, and `fix` shapes in the skill match what `test/plan-ai-first-e2e.test.js` asserts against the running server.
- The two `node -e` commands in the runbook (the read-only verification in step 3 and `down` in the rollback) were run against a **temporary** database: the verification printed `migrations: 1,2,3,4,5,6` and zeros for the two new tables; `down` left `1,2,3,4,5`.

## What was not done, stated plainly

- **The runbook's real-database steps were not performed by this run.** The backup, the restart that applies migration `006`, the verification on `data/hebrew.sqlite`, and the smoke check are the operator's, who said he would run `node scripts/backup-db.js` and `docker compose restart hebrew` himself. Nothing here opened, backed up, or migrated the real database. Whether `rollout-preflight check` passes on the live database is therefore unknown; the runbook says to read any failing line.
- **The operator's read** of the skill's loop and of the "Plan builder" section (the spec's `human` check) has not happened.
- The runbook mirrors `language-program-rollout.md`'s `kubectl exec` forms for the cluster and does not add new cluster commands; none was run.

## Deviations

- **The smoke check edits `beginner-unit-1`**, not "a draft plan" in general, and restores the title in step 4, so the real plan content ends unchanged (its revision and status change). There is no way to delete a plan, so a throwaway plan would stay in the list; this avoids that.
- **The guide's section is longer than the spec's list** by a "Who made a change" and a "When something looks wrong" subsection, so the actor limitation and the undo path each have a heading.
- `recipes.md` needed no edit (the spec allowed one only if the skill grew too long).

## Files

New: `docs/operations/plan-builder-rollout.md`, this summary.  
Edited: `.cursor/skills/hebrew-worksheet/SKILL.md`, `docs/operations/accounts-and-admin.md`, `SPEC-017-docs-operator-guide-skill-runbook.md` (status `in_review`, boxes ticked).  
No commit. No code changed.

## Provenance

`INIT-008/SPEC-017`
