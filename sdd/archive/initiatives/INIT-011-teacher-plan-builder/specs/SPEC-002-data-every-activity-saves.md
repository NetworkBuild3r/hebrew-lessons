# SPEC-002 · data: every activity in the menu is a block the schema accepts

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-011
title: Put Verse back, and keep the menu and the schema in step
domain: data
status: complete
primary_prompt: .claude/agents/data-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004, SPEC-005]
```

## Description

**Context:** The uncommitted Type it change replaced `verseReadingBlock` in `block.oneOf` instead of adding beside it, so a verse lesson is refused. Match is in `activity-map.js` but the schema has no `match` block, so a teacher who adds Match always gets a refusal. Writing (`personal-question`), Notes (`explainer`), and Verse are in the schema and absent from the menu.

**Scope (in):**
- `verseReadingBlock` is back in `block.oneOf`, beside `typeItBlock`.
- `activity-map.js` gains Writing → `personal-question`, Notes → `explainer`, Verse → `verse-reading`, with Hebrew names in `plan-builder.js`.
- Match: if INIT-010 SPEC-004 has landed the `match` block, keep it. If not, Match leaves the menu until it does.
- A test: every name in `ACTIVITY_NAMES` maps to a type that some branch of `block.oneOf` accepts, and every `*Block` definition is referenced from `block.oneOf` (or listed as deliberately unused).

**Scope (out):** A choice block (O1). The Match player (INIT-010 SPEC-004).

## Acceptance criteria

- [ ] ac-1 (REQ-010): `docs/decisions/ADR-0003-assets/samples/` verse plans validate again.
- [ ] ac-2 (REQ-010, D8): The new test fails if `match` is in the menu without a schema block, and fails if `verseReadingBlock` is dropped from `block.oneOf`.
- [ ] ac-3 (REQ-011): The menu lists Writing, Notes, and Verse.

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | the Type it change | It is the operator's work in progress. This spec keeps it and only restores the entry it displaced. | resolved | yes |

## Deliverables

- [ ] `docs/decisions/ADR-0003-assets/lesson-plan.schema.json`
- [ ] `shared/admin/activity-map.js`, the Hebrew names in `shared/admin/plan-builder.js`
- [ ] A node test for ac-1 and ac-2

## Verification Strategy

- **Claim:** nothing in the menu is refused for its type, and Verse validates.
- **Check + executor:** `mechanized` — `node --test` on the new test and the existing schema tests.
- **Pass condition:** the new test is red on the current tree (Match, Verse) and green after the change.

## Gates

Schema change is additive (an entry restored, none removed). No migration. No published plan changes.
