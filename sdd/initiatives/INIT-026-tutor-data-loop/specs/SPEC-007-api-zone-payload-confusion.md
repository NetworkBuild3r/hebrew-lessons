# SPEC-007 · api: `set_time_zone`, zone and weekly payload in `get_progress`, confusion pair in `get_learner_model`

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-026
title: "set_time_zone, zone and weekly payload in get_progress, confusion pair in get_learner_model"
domain: api
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: required
estimated_effort: 3 hours
blocked_by: [SPEC-003]
blocks: [SPEC-008]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- `get_progress` documents and computes its days in UTC (`internal/mcp/tool_progress.go:16`, `:29`, `:30`, `:83`).
- Weekly reports come back as `ReportView{WeekStart, Summary, CreatedAt}` (`internal/mcp/learner_data.go:179`). The payload is dropped at `internal/mcpstore/learner_model.go:163`, and a test pins that (`internal/mcp/learner_tools_sql_test.go:311`, `:369`).
- `ModelWord` has `cause` but no pair (`internal/mcp/learner_data.go:88`). `learner.DominantPair` (`internal/learner/diagnose.go:120`) returns the key and count.

**Scope (in):**
- `set_time_zone {learnerId?, zone}`:
  - Listed in `learnerTools`, `learnerWriteTools`, and `learnerOwnDataTools`. `MCP_READONLY_TOKEN` is refused. An admin may name any learner.
  - Validates with `studyday.ParseZone` and writes with `SetUserTimeZone`.
  - Returns `{zone, today}`, where today is the learner's day in that zone.
  - The description gives one example (`America/Denver`) and says the zone decides every day the app reports.
- `get_progress`:
  - Adds `zone` and `zoneSource` to the output.
  - `from` and `to` are days in that zone. `to` defaults to the learner's today.
  - The description and the schema text drop "UTC".
- `ReportView` gains `payload`, decoded from `payload_json`: `gained`, `lost` (with `cause` and `recovered`), `cleared`, `retention`, `minutesStudied`, `bandSlips`, and `zone`. A row whose JSON does not decode leaves `payload` out and is not an error.
- `ModelWord` gains `confusedWith {kind, key, count}` when `DominantPair` returns ok. Otherwise it is omitted.
- The schema and tool-list tests include the new tool and fields.

**Scope (out):** The digest (SPEC-009). Tutor writes (SPEC-008). Pages (SPEC-006).

## Acceptance Criteria

- [ ] ac-1: A learner calling `set_time_zone {zone: "America/Denver"}` stores it, and the next `get_progress` says `zone: America/Denver`, `zoneSource: learner`. `Not/AZone` is a tool error, and nothing is written.
- [ ] ac-2: Security:
  - A learner naming another `learnerId` gets `learnerId must be your own id.`
  - `MCP_READONLY_TOKEN` calling `set_time_zone` gets `forbidden: admin only`.
  - An admin may set any learner's zone.
- [ ] ac-3: `get_progress` with no `to`, for a Denver learner at 2026-11-01T05:30:00Z, ends at `2026-10-31`. `TestProgressRange` is updated to the learner zone, not deleted.
- [ ] ac-4: Learner A's `get_progress` returns A's weekly payload with `gained`, `lost`, and `minutesStudied`. The cross-learner check at `learner_tools_sql_test.go:369` still finds none of B's strings, including B's payload. A's fixture becomes a real payload.
- [ ] ac-5: A word with three `ב/כ` letter-pair confusions in the window returns `confusedWith: {kind: letter_pair, key: "ב/כ", count: 3}`. A word without one has no `confusedWith`.
- [ ] ac-6: `TestToolsResourcesAndIdempotency` lists `set_time_zone`, and `TestResourcesAuthoringNamesEveryTool` passes.

## Deliverables

- [ ] `internal/mcp/tool_time_zone.go` and test
- [ ] `internal/mcp/tool_progress.go`, `learner_data.go`, `policy.go`, `tools.go`
- [ ] `internal/mcpstore/learner_model.go` (payload decode, confusion pair, zone)
- [ ] `internal/mcp/learner_tools_test.go`, `learner_tools_sql_test.go`
- [ ] `internal/mcp/resources` authoring text, where it names every tool

## Verification Strategy

- **Claim:** an LLM can set the learner's zone, sees which zone every day is in, gets the full weekly payload for its own learner only, and gets the confusion pair.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 -run 'TestProgress|TestLearnerModel|TestSetTimeZone|TestToolsResources|TestResourcesAuthoringNamesEveryTool|TestCoachNotesOverSQLite' ./internal/mcp ./internal/mcpstore`. In-memory and SQLite-backed MCP tests, both of which exist today. Contract seam: the JSON schema snapshot in the tool-list test.
- **Pass condition:** ac-1 to ac-6 pass. Removing `set_time_zone` from `learnerOwnDataTools` fails ac-2.

## Integration Points

- **Provenance tag:** `INIT-026/SPEC-007`
- SPEC-009 reuses the payload view and `confusedWith`.
