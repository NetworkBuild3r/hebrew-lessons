# SPEC-005 · frontend: the lesson screen

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-011
title: A teacher builds a lesson from named activities and sees what the learner sees
domain: frontend
status: complete
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 8 hours
blocked_by: [SPEC-002, SPEC-003]
blocks: [SPEC-006]
```

## Description

**Context:** Figma `19:2592` is one lesson: tabs across the top, a word bank on the left, activities in the middle, the learner preview on the right. The live page opens a lesson into Id, Activity, and Index, and every new lesson already holds an empty Fluency activity the teacher did not add.

**Scope (in):**
- **Layout.** At 1100px: bank, activities, preview, in the frame's order. Below 900px they stack in that order. Lesson tabs and a breadcrumb back to the plan.
- **Lesson details** at the top in plain words: title, the one sentence, level (Beginner, Intermediate, Advanced), and who can see it (Hidden, Everyone, Selected learners). Builds on lists only earlier lessons, by title.
- **The activity menu** is the SPEC-002 list: Words, Match (if present), Story, Listening, Dialogue, Writing, Verse, Pattern, Type it, Fluency, Review, Notes, each with a one-line hint ("Word bank, then fill in the blank").
- **Adding an activity** opens its form with the fields to fill and a Save button. It is inserted with `insert_block` at the end once it is valid, using the staged draft the builder already has. The block id is made by the page. Reorder uses the block move operation.
- **Words** includes the fill-in-the-blank (`cloze`) in the same form: the sentence with a blank, the answer, an optional bank. Picking a word in the bank adds it to the open Words activity.
- **The placeholder.** The empty `placeholder` Fluency activity is not listed. When the first real activity is saved, the page removes the placeholder with the remove operation.
- **The preview** is the selected activity, drawn by the learner renderer.

**Scope (out):** Drag and drop. New block types. The Match player (INIT-010 SPEC-004). Choose-the-correct-sentence (O1).

## Acceptance criteria

- [ ] ac-1 (REQ-004, REQ-006): At 1100px the lesson is three columns in the frame's order. At 700px they stack in that order.
- [ ] ac-2 (REQ-001, REQ-010, D4): Adding Story with one paragraph and one question saves through `POST /api/plans/:id/blocks`. There is no field whose visible name is `Id` or `Index`.
- [ ] ac-3 (D9): A new lesson's activity list is empty. After the first activity is saved, the lesson has no block with id `placeholder`.
- [ ] ac-4 (REQ-011): A Words activity with two fill-in-the-blank items, built from worksheet `02` Ex. 1, saves and previews.
- [ ] ac-5 (REQ-002): The activity names and hints are one language.

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | drag onto an activity | Out. Reorder and add stay buttons that call the operations. | resolved | yes |
| aud-2 | the last activity | Removing the only activity is refused by the schema (`minItems: 1`). The page says "A lesson needs at least one activity" instead of sending it. | resolved | yes |

## Deliverables

- [ ] The lesson layout in `shared/admin/plan-builder.js`, the activity forms in `shared/admin/plan-form.js` / `plan-widgets.js`, and `shared/worksheet.css`
- [ ] Node tests for ac-2 through ac-5

## Verification Strategy

- **Claim:** a teacher adds activities by name, fills them in a form, never sees the placeholder, and the preview is the selected activity.
- **Check + executor:** `mechanized` — `node --test` on the plan-builder activity tests and the new ones. `human` — the operator, at both widths, against frame `19:2592`.
- **Pass condition:** the ac-3 test fails on today's code (the placeholder is listed). The operator does not see an Id or Index field.

## Gates

Saves stay `insert_block`, `move_block`, `remove_block`, and `set_field` through the existing transport. No new public route.
