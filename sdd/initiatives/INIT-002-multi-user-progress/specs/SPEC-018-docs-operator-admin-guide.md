# SPEC-018 · Docs: Operator and admin guide, authoring-skill updates

## Metadata

```yaml
spec_id: SPEC-018
initiative_id: INIT-002
title: Operator and admin guide, authoring-skill updates
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-011, SPEC-017]
blocks: []
```

---

## Description

**Context:** after this initiative the site needs an admin to exist, accounts to be created, lessons to be shared, the database to be backed up, and audio to be checked before any regeneration. Lesson authors (the operator, and agents using the `hebrew-worksheet` skill and MCP) need to know that new lessons start hidden unless assigned (ADR-0002 aud-2) and how to keep recordings safe.

**Scope (in):**
- `docs/operations/accounts-and-admin.md`:
  - **First run:** set `DATA_DIR`, then `npm run user -- create <name> --admin`.
  - **Day-to-day:** add users, reset passwords, share lessons.
  - **Backup and restore:** `npm run db:backup`, and how to restore a backup file.
  - **Audio safety:** `npm run audio:inventory` / `audio:verify`, and where archived clips live.
  - **Environment variables:** `DATA_DIR`, `COOKIE_SECURE`, and `PUBLIC_ORIGIN`, and their k8s values (linking SPEC-011's deploy doc).
  - **What admins can see:** everything a learner does, including the complete answer history. Learners are told on the sign-in page. The AI/MCP cannot read any of it.
  - **Working with the AI:** the AI creates lessons and assigns them (`list_users`, `upsert_lesson` with `visibility`, `set_lesson_visibility`); this needs `MCP_AUTH_TOKEN` set; a lesson created without an assignment stays hidden until assigned.
  - **Known limits:** single replica; `file://` no longer works; word mp3 files are gated by sign-in, not per lesson.
  - **OAuth later:** the `auth_provider` / `provider_subject` hook.
- `.cursor/skills/hebrew-worksheet/SKILL.md` (and its references, if they cover publishing):
  - New lessons are hidden until assigned. Pass `visibility` to `upsert_lesson` (or call `set_lesson_visibility`) to assign them to everyone or to named users; use `list_users` to find usernames.
  - Lessons can also be edited in the browser editor.
  - Run the audio inventory before regenerating audio.
  - The existing rules stay: no learner names or transcripts in lesson files; never edit a lesson `index.html`.
- `README.md` run section (if present): sign-in now required; first-admin command.

**Scope (out):** code.

---

## Understood (agent's read of the intent)

- [ ] und-1: The guide quotes no secret values. It names env vars and k8s secret names only.

## Acceptance Criteria

- [ ] ac-1 (REQ-001, REQ-013): following the guide's first-run steps on a clean checkout produces a signed-in admin. The steps are checked by running them against a temp `DATA_DIR`.
- [ ] ac-2 (REQ-012): the guide's audio section names both commands, the archive location, and the pre-deploy inventory on the live volume.
- [ ] ac-3 (REQ-014): the skill update keeps the existing constraints verbatim in force, and adds the visibility default and the audio check.
- [ ] ac-4: no file in this spec's diff contains a secret value, a learner's name, or transcript text.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Guide location | `docs/operations/` (new folder) | resolved | yes — plan default |

---

## Deliverables

- [ ] `docs/operations/accounts-and-admin.md`
- [ ] `.cursor/skills/hebrew-worksheet/SKILL.md` (+ references as needed)
- [ ] `README.md` (if present)

---

## Technical Requirements (domain-specific)

- [ ] Commands are copy-pasteable for PowerShell (local) and for `kubectl exec` (k8s).

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** operator approval; the ac-1 run-through.
- **Rollback plan:** revert the listed files.

---

## Security

- **Sensitivity:** light. Triggers matched: none (docs only); ac-4 is the guard.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** an operator who has not seen this initiative can bring up the site with an admin, share a lesson, back up the database, and check audio safety, using only the guide.
- **Check + executor:**
  - `mechanized`: run the first-run and backup commands from the guide against a temp `DATA_DIR`.
  - `human`: the operator reads the guide and the skill diff.
- **Pass condition:** the commands run as written and produce an admin and a backup file. The operator confirms each ac-2 and ac-3 item is present, and a secret scan of the diff finds nothing.

---

## Integration Points

- Final spec. Feeds `complete-initiative`.
- **Provenance tag:** `INIT-002/SPEC-018`
