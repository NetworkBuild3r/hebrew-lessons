# SPEC-005 · docs: the AI builds the same Family plan

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-010
title: MCP skill walks through Family 1, Family 2, and Family 3
domain: docs
status: ready
primary_prompt: .claude/agents/docs-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-002]
blocks: None
```

## Description

**Context:** `.cursor/skills/hebrew-worksheet/SKILL.md` already says new lessons are plans and `upsert_lesson` is only for a worksheet that has not moved. It does not say how to stack Family 1, Family 2, and Family 3, or which activity name inserts which block.

**Scope (in):** The skill's plan section gains the activity-name table from initiative section 4, the three lesson operations from SPEC-002, and a short Family walkthrough: `create_plan`, `insert_lesson` twice, Words on the first lesson only, Story and Type it on the second, Pattern on the third, `buildsOn` pointing at the first, then `list_plan_issues`. It states that a plan has no lesson maximum, that the plan's word list is the words introduced so far, and that practice and verb drills use only lessons the learner has finished. The tool descriptions for `insert_lesson`, `move_lesson`, and `remove_lesson` match the functions. `upsert_lesson` stays documented for the three published worksheets only.

**Scope (out):** New tools beyond SPEC-002. Changes to validator behavior.

## Acceptance criteria

- [ ] ac-1: The skill names `insert_lesson` and the activity map, and the Family walkthrough uses only tools that exist after SPEC-002 — covered: no
- [ ] ac-2: The skill says a warning from `list_plan_issues` is fixed or left, and is not a reason to stop. It also says practice uses the reached word list and does not skip a learner ahead — covered: no
- [ ] ac-3: The skill still says the three published worksheets are `upsert_lesson`, not a new plan — covered: no

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | skill path | `.cursor/skills/hebrew-worksheet/SKILL.md` is the AI's authoring guide | resolved | yes |

## Deliverables

- [ ] `.cursor/skills/hebrew-worksheet/SKILL.md`
- [ ] Matching sentences in the `insert_lesson` tool description if SPEC-002 left it generic

## Verification

- **Claim:** an agent that reads only the skill can name the calls for Family 1, Family 2, and Family 3.
- **Check + executor:** `human` — the operator reads the walkthrough against SPEC-002's tool names.
- **Pass condition:** every tool in the walkthrough is registered in `mcp/tools.js`, the activity names match the map module, and the three worksheet names are still listed as legacy.

## Gates

No code path. No secret in the skill.
