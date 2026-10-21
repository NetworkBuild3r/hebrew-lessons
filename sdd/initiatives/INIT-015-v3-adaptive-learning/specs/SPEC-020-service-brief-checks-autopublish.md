# SPEC-020 · service: Brief checks and auto-publish safety

## Metadata

```yaml
spec_id: SPEC-020
initiative_id: INIT-015
title: "Brief checks and auto-publish safety"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 6 hours
blocked_by: [SPEC-004, SPEC-019]
blocks: [SPEC-021]
```

## Description

**Context:** `units.Validate(ctx, doc, lookup) []Issue` runs the v2 rules (`internal/units/rules.go`): lexemes known, audio present, budgets, and checkpoint rules. A learner-scoped `put_unit` with `publish: true` already publishes when there are no block issues (`publishLearnerDraft` in `tool_publish_unit.go`). D5 removes any human step, so the validator is the gate.

**Scope (in):** `internal/units` and the publish path.

- **`ValidateWithBrief(ctx, doc, lookup, brief) []Issue`** adds these block rules:
  - `brief.coverage`: in each reading and listening passage, the share of tokens that are known, solid, or in the brief is 0.95 or more. Function words and proper names are excluded through the lexicon's tags.
  - `brief.new_intro_before_test`: each new word has an intro or exposure step before its first checked step.
  - `brief.new_min_uses`: each new word appears at least 3 times.
  - `brief.struggle_reuse`: each struggle word appears its required count, and at least once in an activity of its failing skill.
  - `brief.gloss_out_of_brief`: any lexeme outside the brief and outside known or solid has a gloss.
  - `brief.audio_all`: every Hebrew text field resolves to an audio asset.
  - `unit.no_markup`: no `<`, `>`, or `javascript:` in text fields.
- **Auto-publish:**
  - `publishLearnerDraft` calls `ValidateWithBrief` whenever the unit names a `briefId`.
  - A learner-scoped unit with no `briefId` is saved as a draft and not auto-published.
  - Every auto-publish writes `publish_audit` (actor, client name, unit, revision, brief, and the issues list, including warnings).
- **Unpublish:** `Units.UnpublishLearner(ctx, principal, unitID)` works only on a learner-scoped unit in the caller's scope, and writes an audit row.
- **Report:** the SPEC-017 report handler also unpublishes a learner-scoped unit and writes an audit row (`report_hide`). A shared unit is hidden for the reporter only.
- **Brief use:** a brief marks `used_by_unit_id` on publish. A brief can't be used twice.

**Scope (out):** MCP tool wiring (SPEC-021).

## Understood (agent's read of the intent)

- [x] und-1: "No HITL." Safety comes from strict checks, an audit trail, and a one-tap undo, not from approval.
- [x] und-2: A lesson written without a brief can still be saved, but it can't skip the brief checks to reach Today.

## Acceptance Criteria

- [x] ac-1: Each new rule has a failing fixture in `internal/units/testdata/` that produces exactly that rule id and a JSON pointer. — covered: yes, see completion summary
- [x] ac-2: A fixture that passes every rule auto-publishes through `put_unit publish:true`, and one audit row is written. — covered: yes, see completion summary
- [x] ac-3: A learner-scoped unit with `publish:true` and no `briefId` is saved as a draft, with the reason in the result. — covered: yes, see completion summary
- [x] ac-4: `UnpublishLearner` on another learner's unit, or on a shared unit, is refused. — covered: yes, see completion summary
- [x] ac-5: Reusing a used brief id is a block issue. — covered: yes, see completion summary

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Tokenizing Hebrew passages | Use `internal/lexicon/prepare.go` to strip niqqud and prefixes (ו, ה, ב, ל, מ, ש, כ) before the lookup. Unmatched tokens count as unknown. | decided | yes |
| aud-2 | Shared units | Unchanged: admin `publish_unit` only, through the same validator. D5 applies to learner scope. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [x] `internal/units/brief_rules.go`, and changes to `validate.go` and `rules.go`
- [x] Changes to `internal/mcp/tool_publish_unit.go` (`publishLearnerDraft`) and the units store
- [x] Tests: one fixture per rule, plus `autopublish_test.go` and `unpublish_test.go`

## Technical Requirements (domain-specific)

- [x] Rule ids are stable strings, documented in `internal/units/doc.go`.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/units/... ./internal/mcp/...`.
- **Rollback plan:** revert the commit. Learner auto-publish falls back to the v2 rules.

## Security

- **Sensitivity:** required — triggers matched: AI-written content reaches the learner with no human step, and a new authorization path (unpublish).
- **Security acceptance criteria:**
  - [x] Markup and script are rejected in every text field. templ escaping stays as the second layer.
  - [x] Unpublish checks scope from the principal, never from input.
  - [x] Every publish and unpublish is in `publish_audit`.
- **Review depth:** deep · **focus areas:** authorization, content injection, audit completeness.

## Verification Strategy

- **Claim:** only units that meet the brief reach Today, every publish is audited, and undo is scoped.
- **Check + executor:** mechanized — `go test ./internal/units/... ./internal/mcp/... -run 'Brief|AutoPublish|Unpublish'`.
- **Pass condition:** ac-1 to ac-5 pass.

## Integration Points

- SPEC-021 exposes `validate_unit` with `briefId` and `unpublish_unit`.
- **Provenance tag:** `INIT-015/SPEC-020`
