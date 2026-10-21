# SPEC-019 · Docs: Operator guide for media and backup

## Metadata

```yaml
spec_id: SPEC-019
initiative_id: INIT-004
title: "Operator guide: media and backup"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 3 hours
blocked_by: [SPEC-011]
blocks: [SPEC-020]
```

---

## Description

**Context:** the guide needs the media sentences and the media backup sentence ADR-0004 §9 left open. ADR-0004 §11 also told this initiative to replace the guide's "AI cannot read progress" sentence. That work moved to INIT-006 SPEC-019 on 2026-10-09 (INIT-006/SPEC-001), because the profile, the switch route, and the disclosure sentence are INIT-006's (ADR-0009 §1, Accepted 2026-10-09). Until INIT-006 ships, the MCP in this initiative reads no learner progress, so the guide's current sentence stays true and this spec leaves it alone.

**Scope (in):**

- Document upload, approval (admin only, alt required), dry-run, confirm, the 15-per-hour cap, and the $1 daily cap, without quoting a key.
- Document backup: `DATA_DIR/media` and `_archive` are in the backup set after SPEC-017, and restore stops writers first.
- Document that deleting an image archives first.

**Scope (out):** the skill the authoring agent reads (SPEC-018); cluster commands that would print a secret; the AI-planning switch, the learner profile, and the disclosure sentence (INIT-006 SPEC-019).

---

## Understood (agent's read of the intent)

- [x] und-1: This spec does not touch the "AI cannot read progress" sentence. INIT-006 SPEC-019 replaces it, once, so the old and new rules are never both in the file.
- [x] und-2: The guide names the variable `OPENAI_API_KEY` and does not contain a value.

## Acceptance Criteria

- [x] ac-1 (REQ-012): `docs/operations/accounts-and-admin.md` documents upload, approval, dry-run, confirm, the caps, and delete-archives-first, and its "AI cannot read progress" sentence is unchanged by this spec.
- [x] ac-2 (REQ-011): the guide states that media trees are backed up with the database and that restore checks approved images after both trees are restored.
- [x] ac-3 (REQ-014): a search of the edited file finds no key assignment and no learner transcript.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Which file | `docs/operations/accounts-and-admin.md`, the file ADR-0004 §11 names. | resolved | yes — ADR-0004 |

> **Readiness** = 100.

---

## Deliverables

- [x] Updated `docs/operations/accounts-and-admin.md`
- [x] Tests: none (documentation)

---

## Technical Requirements (domain-specific)

- [x] Keep the existing account, sign-in, and audio sections. Edit the sentences this spec names. Do not rewrite the whole guide.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the verification search.
- **Rollback plan:** restore the guide from git.

---

## Security

- **Sensitivity:** standard. Triggers matched: the guide describes media approval and the key's location.
- **Security acceptance criteria:**
  - [x] No secret value.
- **Review depth:** standard · **focus areas:** accurate authz description, no secrets.

---

## Verification Strategy

- **Claim:** the guide matches ADR-0004's media rules and names the media backup.
- **Check + executor:** `human` — a reader traces the media sentences to ADR-0004 §5 to §9. `mechanized` — search the file for the absence of `sk-` and `ELEVENLABS_API_KEY=`.
- **Pass condition:** every media sentence traces to the ADR; the search finds no secret assignment.

---

## Integration Points

- SPEC-020 expects operators to have this page before they upload or generate an image in production.
- Produces **Change:** operator guide update.
- **Provenance tag:** `INIT-004/SPEC-019`
