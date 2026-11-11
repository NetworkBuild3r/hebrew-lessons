# SPEC-004 · frontend: play Match and Type it inside a plan

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-010
title: The plan player runs Match and Type it
domain: frontend
status: ready
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-002]
blocks: None
```

## Description

**Context:** Family 2's recall and matching are worksheet activities. The plan player does not run them. `RSCH-005` says a picture comes off when the task is to remember the written form. `RSCH-003` says recall on a later day beats another look at the word.

**Scope (in):** Schema entries for `match` and `type-it`, rendered by `shared/ui/plan-render.js`. Match is Hebrew–English pairs from the lesson's known words (`newWords` plus `buildsOn`). Type it shows the English meaning and asks for the Hebrew. It does not show the picture. Vowel points may show on the word in Words. Type it ignores vowel points when checking, the same way the worksheet `type-it` activity does. A wrong answer can be tried again before the answer is shown, matching the existing retry-before-reveal rule.

**Scope (out):** The admin chrome (SPEC-003). New picture generation. The review scheduler.

## Acceptance criteria

- [ ] ac-1: A plan lesson with a `match` block renders pairs and no other activity — covered: no
- [ ] ac-2: A `type-it` block renders the English cue and a Hebrew field, and the picture markup is absent — covered: no
- [ ] ac-3: A typed answer that differs only by vowel points is accepted — covered: no
- [ ] ac-4: The block is escaped. A meaning that contains `<img` is text, not an element — covered: no

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | which words Match and Type it use | The lesson's new words plus words brought forward by `buildsOn` and `recycles` | resolved | yes |

## Deliverables

- [ ] Schema additions in `docs/decisions/ADR-0003-assets/lesson-plan.schema.json`
- [ ] `shared/ui/plan-render.js`
- [ ] Node tests for ac-1 through ac-4

## Verification

- **Claim:** Match renders pairs, and Type it never includes the picture.
- **Check + executor:** `mechanized` — `node --test` on the plan-render tests. `human` — the operator plays one Type it card in the browser and confirms no picture.
- **Pass condition:** the Type it HTML fixture contains the English meaning and does not contain an image element, and the nikud-only difference is scored correct.

## Gates

No dependency. No secret. Tests do not call a model or a speech API.
