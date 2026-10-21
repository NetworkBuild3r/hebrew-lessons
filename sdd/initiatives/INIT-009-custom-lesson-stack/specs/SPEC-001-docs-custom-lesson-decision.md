# SPEC-001 · docs: ADR-0012 custom lesson stack

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-009
title: Record the custom-lesson and ordered-plan decision
domain: docs
status: complete
primary_prompt: .claude/agents/docs-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 2 hours
blocked_by: None
blocks: [SPEC-002, SPEC-003]
```

---

## Description

**Context:** The operator wants a lesson built from the blocks that make sense, and a plan that is an ordered stack whose words build on each other. ADR-0003 already has blocks, `buildsOn`, and `recycles`. The lesson editor and the player still assume a fixed ladder. The decision needs a record before the code specs change that behavior.

**Scope (in):** `docs/decisions/ADR-0012-custom-lesson-stack.md`. A short extension note on ADR-0003's metadata, the same way ADR-0011 extended it.

**Scope (out):** Code, schema changes, and edits to the three published worksheets.

---

## Understood

- [ ] und-1: The six parts (vocabulary, activity path, fill, grammar, reading, ask-a-friend) stay available and stop being required.
- [ ] und-2: Lesson order on the plan is the teaching order. `buildsOn` and `recycles` are how words chain.
- [ ] und-3: `RSCH-004` rules that are product choices stay labelled as product choices. This ADR does not promote them to evidence-backed constraints.

## Acceptance Criteria

- [ ] ac-1: ADR-0012 states D1–D4 from the initiative and cites `RSCH-004` §3 and §4 — covered: no
- [ ] ac-2: ADR-0012 says an empty activity list is not filled with the six-part ladder — covered: no
- [ ] ac-3: ADR-0003's extended-by line names ADR-0012 — covered: no

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| ---- | ---- | ---- | ---- | ---- |
| aud-1 | "a stack of lessons" | Lesson order inside the plan. The schema unit remains the container; a new plan has one unit. | resolved | yes |

---

## Deliverables

- [ ] `docs/decisions/ADR-0012-custom-lesson-stack.md`
- [ ] Metadata extension row on `docs/decisions/ADR-0003-course-and-lesson-plan-language.md`

---

## Technical Requirements

- [ ] Number is ADR-0012. ADR-0011 is the highest file on disk as of 2026-10-14. Do not leave a placeholder number.
- [ ] No secret, no learner data, no change to `SRS_SCHEDULER`.

---

## Verification Strategy

- **Claim:** ADR-0012's decisions match D1–D4 and do not require the six-part ladder.
- **Check + executor:** `human` — the operator reads ADR-0012 against the initiative §2 table.
- **Pass condition:** every D-row appears in the ADR, the empty-ladder sentence is present, and ADR-0003's extended-by line names ADR-0012. A reader who did not write the ADR can check those three facts from the files.

---

## Gates & Controls

No gate action. `plan_only` until the operator asks to execute.
