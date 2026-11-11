# SPEC-004 · frontend: new plan and the plan screen

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-011
title: A teacher starts a plan, adds and reuses lessons, and keeps a word bank
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

**Context:** Figma `19:2304` is the plan: a wide list of lessons and a word bank on the side. Today New plan asks for a recipe slug, Add lesson makes `lesson-N`, and there is no way to bring back a topic the teacher already taught (`retaining: Living room`, `Intro to opinions` twice in the Drive folder).

**Scope (in):**
- **New plan** asks for a title and who it is for (one learner from the account list, or everyone). It calls `create_plan`, then `assign` when a learner was picked. No recipe field.
- **The plan screen.** At 900px and wider: the lesson list on the left, the word bank on the right. Below 900px the bank sits under the list. One row per lesson: title, the one sentence (`subtitle`), the words it introduces, move up and down, Open lesson. The open lesson has the selected treatment from the frame.
- **Add lesson** asks for a title and an optional sentence. The id is made from the title (`lesson-N` only when the title has no Latin letters). Calls `insert_lesson`.
- **Reuse lesson** picks a lesson from this plan or another plan (read with the admin parts route), and inserts a copy with `insert_lesson`. The copy gets a free id. `buildsOn` keeps only lessons earlier in this plan, and the copy builds on the original when the original is in this plan.
- **Word bank.** Every word the plan's lessons introduce, and the lesson that added it. Add a word, or paste lines of `Hebrew – English` (also tab, comma, or `=`). Pasted words go into the open lesson's `newWords` and its Words activity through `set_field`.
- **Header.** Who the plan is for, in a sentence ("For Brian", "For every learner"), Saved, and Publish.

**Scope (out):** The lesson screen (SPEC-005). Drag and drop. A new plan operation.

## Acceptance criteria

- [ ] ac-1 (REQ-009): New plan has a title field and a For field, and no recipe field. Creating a plan for a learner leaves it `personal` with that assignee.
- [ ] ac-2 (REQ-003, REQ-006): At 1100px the lesson list and the word bank are side by side. At 700px the bank is under the list. The open row carries the selected treatment.
- [ ] ac-3 (REQ-001, REQ-005, D4): Add lesson with the title `Sharing Opinions` inserts a lesson with that title through `POST /api/plans/:id/lessons`. No `lesson-N` title is shown.
- [ ] ac-4 (REQ-013, D10): Reusing a lesson from another plan inserts one lesson with a new id, and its `buildsOn` holds no id that is not earlier in this plan.
- [ ] ac-5 (REQ-012): Pasting three lines of `אמא – mother` style text adds three words to the open lesson. A line with no separator is reported, not dropped silently.

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | sample copy in the frame | Family 1, mother, father are examples. The page shows the open plan's lessons and words. | resolved | yes |
| aud-2 | units | A plan the teacher makes has one unit. The screen does not show units. Plans with several units list their lessons in unit order. | resolved | yes |

## Deliverables

- [ ] `shared/admin/plan-list.js` (New plan), `shared/admin/plan-builder.js` (plan screen, add, reuse, bank), `shared/worksheet.css`
- [ ] Node tests for ac-1, ac-3, ac-4, ac-5

## Verification Strategy

- **Claim:** a teacher starts a plan for a learner, adds a titled lesson, reuses an old one, and pastes a word list, all through existing operations.
- **Check + executor:** `mechanized` — `node --test` on the plan-builder and plan-list tests, plus the new ones. `human` — the operator at 1100px and 700px against frame `19:2304`.
- **Pass condition:** each new test fails on today's code (recipe field present, `lesson-N` title, no reuse, no paste). The operator can point at the selected row and the bank and say they match the frame's columns.

## Gates

No new endpoint. Reuse reads with the admin parts route. Pasted text is parsed on the client and sent as strings; it is rendered with `textContent`.
