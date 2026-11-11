# SPEC-001 · docs: the teacher workflow and the two screens

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-011
title: Record the teacher workflow, the two screens, and the worksheet recipe
domain: docs
status: complete
primary_prompt: .claude/agents/docs-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: None
blocks: [SPEC-002, SPEC-003]
```

## Description

**Context:** INIT-010 already says the admin builder and MCP write the same plan. The page that shipped is a schema form. The operator has two Figma frames that are the teacher view, and a Drive folder of real worksheets that show what a lesson holds. This spec writes the workflow down so the later specs build it, and so the AI knows how to turn a worksheet into a lesson.

**Scope (in):**
- A section in the operator guide: the four steps (New plan, the plan, the lesson, Publish), the two Figma nodes, the one-language rule, and that every change is an existing plan operation.
- The worksheet-to-activity table from the initiative §0b, as the teacher's reference.
- A recipe in `.cursor/skills/hebrew-worksheet/recipes.md`: given a worksheet (for example a Google Doc), build one lesson with `create_plan` or `insert_lesson`, then `insert_block` per exercise using the table. It lists what the AI leaves for the teacher (choose-the-correct-sentence, drawing tasks).

**Scope (out):** No new ADR. No code. No change to MCP tool names.

## Acceptance criteria

- [ ] ac-1 (REQ-001, D1): The guide says the teacher page and MCP call the same operations, and names `insert_lesson` as the operation behind Add lesson and Reuse lesson.
- [ ] ac-2 (REQ-003, REQ-004, D2): The guide names the plan screen as Figma `19:2304` and the lesson screen as Figma `19:2592`, in file `NJ8HfTPCQgrFty2VVQBMhY`.
- [ ] ac-3 (REQ-002, D3, D4): The guide says chrome is one language, and that a teacher does not type a path, a block id, an index, or a recipe slug.
- [ ] ac-4 (REQ-011, REQ-014): The recipe maps each exercise in worksheet `03 המשפחה שלי` to an activity name, and names the one it cannot hold.

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | a new ADR | Not needed. ADR-0011 already binds the write path. This spec only records the screens and the recipe. | resolved | yes |
| aud-2 | reading Google Docs | The AI reads the Doc with its own Drive tool. The app does not get a Drive integration. | resolved | yes |

## Deliverables

- [ ] A section in the operator guide for the workflow and the two screens
- [ ] A worksheet-to-lesson recipe in `.cursor/skills/hebrew-worksheet/recipes.md`

## Verification Strategy

- **Claim:** a teacher can tell which Figma frame is which screen, and the AI can map a real worksheet to activities.
- **Check + executor:** `human` — the operator reads the section against the two frame URLs, and reads the recipe's table against worksheet `03`.
- **Pass condition:** the section names both node ids. Every exercise in `03` is either mapped to a menu activity or listed as not held.
