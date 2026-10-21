# SPEC-022 · docs: One authoring skill for every AI

## Metadata

```yaml
spec_id: SPEC-022
initiative_id: INIT-015
title: "One authoring skill for every AI"
domain: docs
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-021]
blocks: [SPEC-023]
```

## Description

**Context:** `.cursor/skills/hebrew-worksheet/SKILL.md` on `main` teaches one client to build plans through the old plan tools. v3 needs one procedure that Muse, Grok, Claude, and Cursor all follow over MCP (north-star section 8).

**Scope (in):**

- Fill `internal/mcp/resources/authoring.md`, served as `hebrew://authoring`. The steps:
  1. `get_context`.
  2. `get_lesson_brief` (with an optional theme).
  3. Write a learner-scoped unit within the brief: a short story or dialogue, activities for the skill focus, and glosses for words outside the brief.
  4. `lookup` any uncertain form, and `upsert_lexemes` only for verified forms.
  5. `generate_audio` for every Hebrew text.
  6. `validate_unit` with `briefId`, and fix every issue by rule id.
  7. `put_unit` with `scope = learner:<id>`, `publish: true`, and the `briefId`.
  8. `add_coach_note` with one sentence about what the lesson targets.
- Include the unit JSON shape, the rule ids and how to fix each one, and two worked examples (a struggle-heavy brief and a new-words brief).
- Mirror the same text into the repo as `.cursor/skills/hebrew-lesson-author/SKILL.md` on `v3`. A test checks the two files are identical.

**Scope (out):** Changing the old `hebrew-worksheet` skill on `main`.

## Understood (agent's read of the intent)

- [ ] und-1: Any bot builds the same kind of lesson, because they all read the same steps.

## Acceptance Criteria

- [x] ac-1: The resource text names every tool in the order above, and every rule id from SPEC-020. A test greps for each one. — covered: no
- [x] ac-2: The repo skill file is identical to the resource. — covered: no
- [x] ac-3: A dry run, with one MCP client following only this text against a test server, publishes a unit that passes validation. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Dry-run client | The executing agent in Cursor, connected to a local server, with the static token scoped to a test learner. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `internal/mcp/resources/authoring.md`
- [ ] `.cursor/skills/hebrew-lesson-author/SKILL.md`
- [ ] Tests: `internal/mcp/resources_test.go`

## Technical Requirements (domain-specific)

- [ ] Plain sentences, as in `docs/north-star.md`. No model-specific instructions.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/mcp -run Resources`.
- **Rollback plan:** revert to the stub.

## Security

- **Sensitivity:** light — triggers matched: none.
- **Security acceptance criteria:**
  - [ ] The text contains no token, host secret, or learner data.
- **Review depth:** quick · **focus areas:** secrets.

## Verification Strategy

- **Claim:** one text tells any AI how to build a valid lesson from a brief.
- **Check + executor:** mechanized — the resource test. Inspection — the dry-run transcript attached to the completion summary.
- **Pass condition:** ac-1 and ac-2 pass, and the dry run publishes a unit.

## Integration Points

- SPEC-023's weekly coach prompt points at `hebrew://authoring`.
- **Provenance tag:** `INIT-015/SPEC-022`
