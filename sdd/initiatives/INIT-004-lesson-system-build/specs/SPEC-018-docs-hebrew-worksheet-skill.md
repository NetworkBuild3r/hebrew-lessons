# SPEC-018 · Docs: Hebrew-worksheet skill for the plan language

## Metadata

```yaml
spec_id: SPEC-018
initiative_id: INIT-004
title: "Rewrite the hebrew-worksheet skill for plans"
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
blocked_by: [SPEC-005, SPEC-012]
blocks: [SPEC-020]
```

---

## Description

**Context:** `.cursor/skills/hebrew-worksheet/` is how the operator's AI writes lessons today. It tells the agent to call `upsert_lesson` and to omit audio fields. After the plan tools exist, the skill has to teach the plan language, part patches, image requests, and the validator ids. Legacy worksheets stay editable until SPEC-020 retires one, so the skill keeps a short legacy section.

**Scope (in):**

- Rewrite `SKILL.md` and the schema notes it points at so the default path for a new lesson is the plan tools: `read_part`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `set_field`, `publish_plan`.
- Document id paths from ADR-0003 §4, `expectedRevision`, and `revision_conflict` (read again, do not merge).
- Document image requests: `plan_image`, then `generate_image` with `confirm: true`, then a human approves. The agent cannot approve. Alt text is required. Pictures only on introduction words.
- State that the agent never puts a username, learner data, or a transcript into a lesson file. The learner-profile tools are INIT-006's and are documented by INIT-006 SPEC-019 as an addition to this skill (amended 2026-10-09 by INIT-006/SPEC-001, which superseded `get_learner_snapshot`). This spec does not document them.
- Document validator ids N1–N12 by pointing at ADR-0003 §6. State that legacy plans warn on N1 and that the agent does not delete words from `my-morning`, `anal-sex-vocab`, or `shai-date-indirect-speech` to satisfy the cap.
- State that audio paths already in a plan are copied unchanged, and that `npm run audio:inventory` / `audio:verify` still apply before any regeneration. `upsert_lesson` remains the tool for a worksheet that has not moved.
- The skill must not contain a key, a learner's real name, or a pasted transcript.

**Scope (out):** application code; deleting the legacy section before cutover.

---

## Understood (agent's read of the intent)

- [x] und-1: The skill is instructions for the agent, not a second validator. The server still enforces the rules.
- [x] und-2: Creating a brand-new plan id is not documented as a tool, because that operation is open. The skill edits the imported plans and says to ask the operator before expecting a create tool.

## Acceptance Criteria

- [x] ac-1 (REQ-012): the skill names each plan MCP tool from the parity table and the three ADR-0004 media tools (`upload_media`, `plan_image`, `generate_image`), and it says approval is not an MCP success.
- [x] ac-2 (REQ-012): the skill tells the agent to send `expectedRevision`, to stop on `revision_conflict`, and to cite N1 as a warn on `migration.legacy` plans rather than deleting words.
- [x] ac-3 (REQ-014): the skill forbids putting account names, answer text, and secrets into plan files, and it forbids inventing `.mp3` paths.
- [x] ac-4 (REQ-004): `upsert_lesson` is described as the worksheet tool, not as the plan editor.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Replace or keep the folder | Rewrite the files in `.cursor/skills/hebrew-worksheet/`. Do not add a second skill that contradicts it. | resolved | yes — INIT-003 SPEC-010 item 6 |

> **Readiness** = 100.

---

## Deliverables

- [x] Updated `.cursor/skills/hebrew-worksheet/SKILL.md` and the schema doc it references, as far as the plan language requires
- [x] Tests: none (documentation). A checklist in the spec is the verification.

---

## Technical Requirements (domain-specific)

- [ ] Tool names match SPEC-005 and SPEC-012 exactly.
- [ ] No sample that uses a real username. Examples use `example-learner`.

---

## Gates & Controls

- **Autonomy level:** `plan_only`.
- **Gate actions triggered:** none.
- **Blocking checks:** the verification checklist below.
- **Rollback plan:** restore the previous skill files from git.

---

## Security

- **Sensitivity:** light. Triggers matched: the text tells the agent how to call sensitive tools. It must not include secrets or real learner data.
- **Review depth:** quick · **focus areas:** no secrets, no real usernames.

---

## Verification Strategy

- **Claim:** an agent following the skill can name the plan operations, the revision rule, and the image confirm rule, and is told not to write learner data into a plan.
- **Check + executor:** `human` — a reader who did not edit the skill marks each ac against the file. `mechanized` — a search that the seven plan tool names, `upload_media`, `plan_image`, `generate_image`, and `revision_conflict` each occur, and that `OPENAI_API_KEY=` and `ELEVENLABS_API_KEY=` do not.
- **Pass condition:** the search exits 0 with every name found and neither assignment found; the reader can point to the sentence for N1 legacy warn and the sentence that forbids inventing audio paths.

---

## Integration Points

- Depends on the tools being the names SPEC-005 and SPEC-012 registered. SPEC-020 expects this skill before the operator relies on the agent during cutover.
- Produces **Change:** authoring skill rewrite.
- **Provenance tag:** `INIT-004/SPEC-018`
