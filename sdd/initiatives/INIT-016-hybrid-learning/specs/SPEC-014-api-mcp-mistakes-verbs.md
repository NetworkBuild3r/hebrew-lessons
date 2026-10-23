# SPEC-014 · api: MCP mistakes and verb forms; the brief and learner model read personal mistakes

## Metadata

```yaml
spec_id: SPEC-014
initiative_id: INIT-016
title: "MCP: mistakes and verb forms; brief and learner model read personal mistakes"
domain: api
status: planned
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: [.claude/agents/security-specialist/AGENT.md]
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 6 hours
blocked_by: [SPEC-003, SPEC-013]
blocks: [SPEC-015]
```

## Description

**Context:** A tutor's agent records a learner's mistakes; an admin maintains the general list and verb forms. The tuned path uses the learner's own mistakes (D5).

**Scope (in):**

- `record_mistakes`: a learner token writes its own learner's mistakes; an admin token names the learner explicitly. At most 50 per call. Each has `wrong`, `right`, optional `note`, `pattern`, `lexeme`.
- `upsert_common_mistakes` (admin) and `upsert_verb_forms` (admin; the lemma must exist in the lexicon).
- `get_learner_model` adds the learner's top 10 mistakes by `times` and recency.
- The brief adds up to 3 personal mistakes as `fixMistakes`, each with its contrast, and the authoring resource tells the author to practise the right form and show the contrast once. A validator rule checks a brief's `fixMistakes` right forms appear in the unit.
- Authoring resource and the lesson-author skill updated.

**Scope (out):** Pages (SPEC-015). Mining app attempts into personal mistakes automatically (later).

## Acceptance Criteria

- [ ] ac-1: A learner token can record and read only its own mistakes; a call naming another learner is refused. (REQ-008)
- [ ] ac-2: An admin `record_mistakes` without a learner id is refused. (REQ-008)
- [ ] ac-3: A brief for a learner with recorded mistakes carries up to 3 in `fixMistakes`, and a unit built from it without the right forms fails validation. (REQ-008)
- [ ] ac-4: `upsert_verb_forms` refuses a lemma not in the lexicon and writes nothing for the batch. (REQ-010)

## Deliverables

- [ ] `internal/mcp/tool_mistakes.go`, `tool_verb_forms.go`, learner model and brief changes, validator rule
- [ ] Tests for each tool, the scope rules, and the brief and validator change

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none. **Rollback:** revert.

## Security

- **Sensitivity:** required — private learner data over MCP.
- **Security acceptance criteria:**
  - [ ] The learner id for a learner token comes from the token, never from input.
  - [ ] Personal mistakes never appear in any shared tool output (`list_patterns`, common mistakes, explainers).
  - [ ] Input lengths are capped before storage.
- **Review depth:** standard · **focus areas:** authorization, scope, data leakage between learners.

## Verification Strategy

- **Claim:** mistakes flow from a tutor's agent into one learner's lessons without leaking.
- **Check + executor:** mechanized — `go test ./internal/mcp/... ./internal/brief/... ./internal/units/...`.
- **Pass condition:** ac-1 to ac-4 tests and the two-learner leakage test pass.

## Integration Points

- SPEC-004 runbook adds the steps. SPEC-015 shows the lists.
- **Provenance tag:** `INIT-016/SPEC-014`
