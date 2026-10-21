# SPEC-019 · Docs: Skill rewrite for recipes and the profile loop; operator guide

## Metadata

```yaml
spec_id: SPEC-019
initiative_id: INIT-006
title: "Teach the AI the profile → recipe → write → coverage → draft loop; document the switch for the operator"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-010, SPEC-011]
blocks: [SPEC-020]
```

---

## Description

**Context:** `.cursor/skills/hebrew-worksheet/SKILL.md` is how the operator's AI authors lessons. INIT-004 SPEC-018 rewrites it for the plan language; this spec adds INIT-006's tools and recipes on top (SPEC-001 pointed INIT-004 SPEC-018 here for the profile wording). `docs/operations/accounts-and-admin.md` is the operator guide.

**Scope (in):**

- Skill: a "Build the next lesson" loop:
  1. `get_learner_profile` (stop and tell the operator on `planning_off`).
  2. Choose a recipe: rotate among `topic-conversation`, `review-worksheet`, `verse-reading`, `verb-workshop` (and `letters-and-vowels` when the journey stage is `script`); avoid three in a row; open with review of due or weak words; end most blocks with a personal question.
  3. Pick words: recycle weak and very weak words and recently learned words; introduce new words within the recipe's cap; target the weakest grammar concept with a verb workshop when one is `weak` or worse.
  4. Write with INIT-004 plan tools and the five new blocks; tag every new word with `upsert_lexicon_entries` (pos, root, binyan, tense, cell, `accept[]`).
  5. `check_text_coverage` on every reading; revise until ≥ 0.95 (RSCH-004 rule 1, Low).
  6. Save as draft; never publish without the operator; never mark forms verified.
- Recipe cards: block sequence for each recipe, mirroring the teacher's formats in INIT-006 §2, with the research note for each step.
- Tagging rules: binyan and root conventions, cell codes, traffic-light root classes, plene vs defective spellings in `accept[]`.
- Hard rules: never copy the profile, a username, typed answers, or a transcript into a plan or lexicon entry; reduce English and vowel points only as the journey stage rises and only as authored text (the learner's toggle stays theirs, ADR-0001 #5).
- Operator guide: what the switch sends (the ADR-0009 §1 list), how to view it on Progress, how to turn it on for yourself, how to tick the forms checklist, how to fall back to the ladder scheduler.
- Operator guide, one replacement: `docs/operations/accounts-and-admin.md` line 138, "The AI and MCP tools cannot read progress or answer history", becomes the in-force rule from ADR-0009 §9 (the MCP reads only the §1 profile, only when that learner's switch is on, and never answer history, typed text, timestamps, passwords, sessions, or admin views). This is the one place that sentence changes, so the old and new rules are never both in the file. INIT-004 SPEC-019 leaves it alone (amended 2026-10-09 by INIT-006/SPEC-001).

**Scope (out):** code.

---

## Understood (agent's read of the intent)

- [x] und-1: The skill is guidance; the server enforces rules regardless.

## Acceptance Criteria

- [x] ac-1 (REQ-015): the skill names every INIT-006 MCP tool exactly as registered and the six-step loop.
- [x] ac-2 (REQ-015): the skill has one recipe card per recipe in ADR-0009 §7.
- [x] ac-3 (REQ-015): the operator guide lists the profile fields and the never-returned list by pointing to ADR-0009 §1 and quotes the disclosure sentence. The sentence "The AI and MCP tools cannot read progress or answer history" no longer appears unqualified.
- [x] ac-4: no key assignment, no real username, no teacher text in either file.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | INIT-004 SPEC-018 not yet done | Write INIT-006's section as an addition that does not contradict the legacy section; INIT-004 SPEC-018 merges around it. | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [x] Updated `.cursor/skills/hebrew-worksheet/SKILL.md` (and a `recipes.md` beside it if the skill gets long)
- [x] Updated `docs/operations/accounts-and-admin.md`
- [x] Tests: none (documentation). Verification checklist below.

---

## Technical Requirements (domain-specific)

- [ ] Examples use `example-learner`.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the verification search.
- **Rollback plan:** revert the files.

---

## Security

- **Sensitivity:** light. Instructions for sensitive tools; no secrets or real data.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** an agent following the skill builds a recipe-based draft from the profile without leaking learner data.
- **Check + executor:** `mechanized` — search that every INIT-006 tool name and every recipe id appears; that `_API_KEY=`, `MCP_AUTH_TOKEN=`, `ADMIN_PASSWORD=` do not. `human` — operator reads the loop.
- **Pass condition:** search passes; operator accepts.

---

## Integration Points

- Tool names from SPEC-010; blocks and recipes from SPEC-011.
- Produces **Change:** skill and guide.
- **Provenance tag:** `INIT-006/SPEC-019`
