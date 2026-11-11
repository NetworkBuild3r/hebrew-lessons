# SPEC-003 · frontend: one language, a readable search, a visible failure

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-011
title: English shows English, and every refused write says why
domain: frontend
status: complete
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 3 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004, SPEC-005]
```

## Description

**Context:** With EN selected, the plan library still shows `4 תוכניות (4 plans)`, and Issues still shows `מעבר לשדה (Go to the field)` as one string. The search field paints an icon on top of the placeholder. Add lesson can return without a new row and without a sentence.

**Scope (in):** Every chrome string on `/editor/` is either a `.ui-pair` or one language chosen from `data-ui-lang`. The plan count, the issue jump, loading and error lines, the preview heading, and the assignment line are in that set. The search field is a normal input. When any write is refused, `data-plan-error` shows a sentence in the teacher's language. A schema refusal is turned into a plain sentence (which activity, which field is missing), not the raw message. An early return (no plan, no unit) does not stay silent.

**Scope (out):** The two screen layouts (SPEC-004, SPEC-005). MCP tool text.

## Acceptance criteria

- [ ] ac-1 (REQ-002): With `data-ui-lang` not `he`, the plan library count and the issue jump do not contain Hebrew. With `data-ui-lang="he"`, those nodes do not contain the English gloss.
- [ ] ac-2 (REQ-007): `.plan-filter-search` has no background image. Its placeholder is not covered.
- [ ] ac-3 (REQ-005): A refused Add lesson shows the error line with a reason. A refused activity names the activity and the missing field. The node test that adds two lessons still ends with three rows.

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | lesson titles | A Hebrew title stays Hebrew in English mode. It is content. | resolved | yes |

## Deliverables

- [ ] `shared/admin/plan-list.js`, `plan-issues.js`, `plan-history.js`, `plan-builder.js`
- [ ] `shared/worksheet.css` for the search field
- [ ] A node test for ac-1 and ac-3

## Verification Strategy

- **Claim:** English chrome has no Hebrew gloss, and a refused write is visible and readable.
- **Check + executor:** `mechanized` — `node --test` on the plan-list and plan-builder tests, plus the new assertions. `human` — the operator, in the browser, with EN selected.
- **Pass condition:** the new assertions fail if the count string still contains both `תוכניות` and `plans`. A forced refusal shows a sentence that does not contain a JSON path.

## Gates

No new route. Admin-only stays as it is. Text from the server goes through `textContent`.
