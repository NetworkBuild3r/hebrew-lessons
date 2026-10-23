# SPEC-015 · frontend: My stuff — mistakes, verb tables, word bank

## Metadata

```yaml
spec_id: SPEC-015
initiative_id: INIT-016
title: "My stuff: mistakes, verb tables, word bank"
domain: frontend
status: planned
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [deploy]
reversibility_class: remote
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-014]
blocks: [SPEC-019]
```

## Description

**Context:** My stuff is the learner's own section. `/me` exists with account basics.

**Scope (in):**

- `GET /me/words`: word bank — every word the learner has met, filterable by state (learning, known, struggling), each in the text form with its root link.
- `GET /me/verbs`: one table per verb the learner knows (any form in learning or better), rows by tense, columns by person; forms the learner knows are marked, others shown muted.
- `GET /me/mistakes`: "Mistakes you keep making": wrong → right with the note, times, and the explainer link when the mistake has a pattern.
- On explainer pages (SPEC-009): a "Learners often say" block from published common mistakes for that pattern.
- Empty states for each page.

**Scope (out):** Editing anything.

## Acceptance Criteria

- [ ] ac-1: `/me/mistakes` shows only the signed-in learner's mistakes, ordered by times then recency. (REQ-008)
- [ ] ac-2: `/me/verbs` shows a table for each verb with a known form and marks exactly the known forms. (REQ-010)
- [ ] ac-3: The explainer page shows published common mistakes for its pattern and none of any learner's. (REQ-009)
- [ ] ac-4: All pages pass at 390 and 1440, light and dark; tables scroll inside their card, not the page.

## Deliverables

- [ ] `internal/web/me_*.go`, `views/me*.templ`, CSS
- [ ] Render tests (two-learner isolation), browser test with screenshots

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** deploy. **Rollback:** revert and redeploy.

## Security

- **Sensitivity:** standard — private pages; the learner id comes from the session.

## Verification Strategy

- **Claim:** the learner sees their own words, verbs and mistakes, and nobody else's.
- **Check + executor:** mechanized — `go test ./internal/web/...`; browser suite with `-tags browser`.
- **Pass condition:** ac-1 to ac-4 tests and checks pass.

## Integration Points

- SPEC-019 adds My stuff to navigation.
- **Provenance tag:** `INIT-016/SPEC-015`
