# SPEC-021 · api: MCP tools for the learner model and briefs

## Metadata

```yaml
spec_id: SPEC-021
initiative_id: INIT-015
title: "MCP tools for the learner model and briefs"
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 6 hours
blocked_by: [SPEC-014, SPEC-019, SPEC-020, SPEC-026]
blocks: [SPEC-022, SPEC-023]
```

## Description

**Context:** `internal/mcp` serves these tools over OAuth (`oauth.go`) and a static admin token (`auth.go`), with the role gate in `policy.go`:

- `get_context` and `get_learner_words`
- the coach tools (`coach.go`)
- `lookup` and `upsert_lexemes`
- `generate_audio`
- `validate_unit`, `put_unit`, and `publish_unit`

`authz_test.go` is a role table test. North-star section 8 lists what to add, so any client (Muse, Grok, Claude, Cursor) can build lessons the same way.

**Scope (in):**

| Tool | Input | Returns |
| --- | --- | --- |
| `get_learner_model` | `state?`, `skill?`, `band?`, `cursor?`, `limit ≤ 200` | Per word and skill: state, attempts, accuracy, last seen, stability, and cause |
| `get_progress` | `from`, `to` (≤ 366 days) | Snapshots, regression events, gained and lost, retention against target, placement history, and weekly reports |
| `get_lesson_brief` | `theme?` | The SPEC-019 brief with `briefId` |
| `validate_unit` | add `briefId?` | Adds the brief rule issues |
| `add_coach_note` | `body ≤ 500` | The stored note id |
| `unpublish_unit` | `unitId` | Status. Learner scope only |

- `get_context` adds the last 5 coach notes and the model summary.
- `get_learner_words` keeps its shape and reads from `learner.Model`.
- Register the resource `hebrew://authoring`, served from an embedded markdown file that SPEC-022 fills. This spec ships a stub.
- **Scope rules:**
  - A learner principal sees only its own data.
  - The admin token must pass `learnerId` for the learner tools, and it may read any learner.
  - Both rules are added to `authz_test.go` as rows.
- Each tool description follows the house style in `tool_publish_unit.go`: plain sentences, the workflow, and the error cases.

**Scope (out):** The authoring text (SPEC-022).

## Understood (agent's read of the intent)

- [ ] und-1: "All this data is presented to my AI to build lessons." The AI and the app read the same functions.

## Acceptance Criteria

- [x] ac-1: `authz_test.go` has a row per new tool for learner, other learner, admin, and no token, and every row passes. — covered: yes, see completion summary
- [x] ac-2: `get_learner_model` pages correctly, and `limit` over 200 is clamped. — covered: yes, see completion summary
- [x] ac-3: An end-to-end test plays the client side: `get_lesson_brief`, then `validate_unit` with `briefId`, then `generate_audio`, then `put_unit publish:true`, then `add_coach_note`. The unit appears in the learner's Today query. — covered: yes, see completion summary
- [x] ac-4: `unpublish_unit` by a learner on its own auto-published unit removes it from Today and writes an audit row. — covered: yes, see completion summary
- [x] ac-5: `resources/list` includes `hebrew://authoring`. — covered: yes, see completion summary
## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Rate limits | Use the existing per-token limiter from `internal/mcp`, if present. If it is absent, add 60 calls a minute per token for the new read tools. | decided | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `internal/mcp/tool_learner_model.go`, `tool_progress.go`, `tool_lesson_brief.go`, `tool_coach_note.go`, and `tool_unpublish_unit.go`
- [ ] Changes to `tools.go`, `policy.go`, `schema.go`, `store.go`, `memory.go`, and `internal/mcpstore`
- [ ] `internal/mcp/resources/authoring.md` (stub) and its registration
- [ ] Tests: `authz_test.go` rows, one test per tool, and `e2e_brief_test.go`

## Technical Requirements (domain-specific)

- [ ] The in-memory store (`memory.go`) implements every new store method, so the MCP tests don't need SQLite.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none.
- **Blocking checks:** `go test ./internal/mcp/... ./internal/mcpstore/...`.
- **Rollback plan:** revert the commit. The tools vanish, and the data stays.

## Security

- **Sensitivity:** required — triggers matched: new reads of personal data over a network API, and a new write path.
- **Security acceptance criteria:**
  - [ ] Scope comes from the principal. A learner principal ignores any `learnerId` input.
  - [ ] Errors do not reveal whether another learner's brief or unit exists.
  - [ ] Inputs are bounded (limit, date range, body length).
- **Review depth:** deep · **focus areas:** authorization (BOLA), input bounds, error leakage.

## Verification Strategy

- **Claim:** the new tools work end to end for a learner, and no principal reads another learner's data.
- **Check + executor:** mechanized — `go test ./internal/mcp/... -run 'Authz|LearnerModel|Progress|Brief|CoachNote|Unpublish|E2E'`.
- **Pass condition:** ac-1 to ac-5 pass.

## Integration Points

- SPEC-022 writes the authoring resource text. SPEC-023 uses `get_progress` and `add_coach_note`.
- **Provenance tag:** `INIT-015/SPEC-021`
