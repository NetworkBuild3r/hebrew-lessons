# SPEC-009 · Docs: Authoring guide — evidence-based lesson rules

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-001
title: "Authoring guide: evidence-based lesson rules"
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
blocked_by: [SPEC-007, SPEC-008]
blocks: None
```

---

## Description

**Context:** the agent that writes lessons follows `.cursor/skills/hebrew-worksheet/SKILL.md` and `schema.md`. After SPEC-007/008 the skill must tell it what the evidence favors and what the validator checks.

**Scope (in):**
- `SKILL.md`: a short "What the research says" section citing `RSCH-003` — recycle known words, repeat each new word in context at least twice, keep stories mostly known words (about 95%+ estimate), supply `reading.glosses` for words outside the vocab list, keep grammar short with meaningful practice, never add learning-style variants, no root/binyan speed drills, no forced vowel fading.
- `schema.md`: document `reading.glosses` and the new validator warnings.
- `README.md`: one paragraph on the "Review today" page and that progress stays in the browser.

**Scope (out):** rewriting existing lessons; code.

---

## Understood (agent's read of the intent)

- [x] und-1: Guidance is short and imperative so an authoring agent follows it.

## Acceptance Criteria

- [x] ac-1 (REQ-008): `SKILL.md` lists each rule with its `RSCH-003` evidence level — covered: yes
- [x] ac-2 (REQ-006): `schema.md` documents `reading.glosses` with an example — covered: yes
- [x] ac-3: `README.md` describes the review page and local-only progress — covered: yes
- [x] ac-4 (REQ-009): no learner name or transcript text appears in any updated file — covered: yes

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| — | none | — | — | — |

---

## Deliverables

- [x] `.cursor/skills/hebrew-worksheet/SKILL.md`
- [x] `.cursor/skills/hebrew-worksheet/schema.md`
- [x] `README.md`

---

## Technical Requirements (domain-specific)

- [x] Keep the skill file's existing structure; add, don't rewrite.

---

## Gates & Controls

- **Autonomy level:** `plan_only` — draft for operator approval.
- **Gate actions triggered:** none.
- **Blocking checks:** operator approval.
- **Rollback plan:** revert the three files.

---

## Security

- **Sensitivity:** light — triggers matched: none.
- **Review depth:** quick.

---

## Verification Strategy

- **Claim:** an authoring agent reading only the skill files would produce a lesson that passes SPEC-008's validator without warnings.
- **Check + executor:** `human` — the operator asks a fresh agent to draft one test lesson using the updated skill, then runs the validator on it (`mechanized`).
- **Pass condition:** the validator reports no recycling, encounters, coverage, or gloss warnings on that lesson, and each rule in `SKILL.md` names its `RSCH-003` evidence level.

---

## Integration Points

- Documents SPEC-007 (glosses) and SPEC-008 (validator rules).
- Produces **Change:** skill, schema, and README updates.
- **Provenance tag:** `INIT-001/SPEC-009`
