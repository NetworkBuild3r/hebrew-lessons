# SPEC-023 · scheduled: Weekly report and the coach automation

## Metadata

```yaml
spec_id: SPEC-023
initiative_id: INIT-015
title: "Weekly report and the coach automation"
domain: scheduled
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-014, SPEC-021, SPEC-022]
blocks: [SPEC-025]
```

## Description

**Context:** North-star phase 5 calls for a weekly report that names what was gained, what was lost, and why. The server can compute the report from snapshots. Writing the next lesson needs an AI client, and the server has no model key. The rest of phase 5 is already placed: re-check due dates in SPEC-007, band-slip events in SPEC-014, and both on Today in SPEC-017. Coach notes are in SPEC-021, and the report card is in SPEC-018.

**Scope (in):**

- **`Weekly.Run(ctx, weekStart)`**, run by the SPEC-014 scheduler on Monday at 04:00. For each learner it writes one `weekly_reports` row. The payload holds:
  - words gained and words lost, with causes;
  - struggle words that cleared;
  - retention against target;
  - minutes studied;
  - band-slip events.

  The `summary` is three plain sentences built from templates. It is idempotent per week.
- **CLI:** `hebrew jobs weekly --week YYYY-MM-DD`.
- **Coach automation:** a Cursor Automation definition in `docs/ops/weekly-coach.md`. Every Monday at 07:00 it runs an agent with the Hebrew MCP server connected, using an OAuth learner token. The prompt:
  1. Read `get_progress` for the last 7 days.
  2. Follow `hebrew://authoring` to build the next lesson, with a theme drawn from the week's struggle words.
  3. Leave one coach note.

  This spec writes the definition. The operator turns it on (SPEC-025, step 6).

**Scope (out):** Running a model inside the server. Screens.

## Understood (agent's read of the intent)

- [ ] und-1: "Each week I get a report naming what I gained, what I lost, and why."
- [ ] und-2: With D5, the weekly lesson reaches Today with no human step once the automation is on.

## Acceptance Criteria

- [x] ac-1: With a fixture week of snapshots and events, the report lists the gained and lost words those rows imply, each lost word with its cause. — covered: no
- [x] ac-2: Running the job twice for one week leaves one row. — covered: no
- [x] ac-3: `docs/ops/weekly-coach.md` holds a complete automation definition, and every tool and resource it names exists in `tools.go`. A test checks the names. — covered: no

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| --- | --- | --- | --- | --- |
| aud-1 | Which client runs the weekly lesson | A Cursor Automation, because it is already in the operator's toolset. Any MCP client can follow the same text. | assumed | yes |

> **Readiness** = 100.

## Deliverables

- [ ] `internal/jobs/weekly.go`, and the `jobs weekly` subcommand
- [ ] `docs/ops/weekly-coach.md`
- [ ] Tests: `weekly_test.go`, and `internal/mcp/coach_doc_test.go` for ac-3

## Technical Requirements (domain-specific)

- [ ] Uses the SPEC-014 scheduler, lock, and fake clock.

## Gates & Controls

- **Autonomy level:** auto_tests.
- **Gate actions triggered:** none. Turning on the automation is the operator's step.
- **Blocking checks:** `go test ./internal/jobs/... ./internal/mcp -run CoachDoc`.
- **Rollback plan:** `HEBREW_JOBS=off`, then revert.

## Security

- **Sensitivity:** standard — triggers matched: background writes, and an external automation with MCP access.
- **Security acceptance criteria:**
  - [ ] The automation uses an OAuth learner token, never the static admin token.
- **Review depth:** quick · **focus areas:** token scope.

## Verification Strategy

- **Claim:** a weekly report is written once a week, and the coach automation is ready to turn on.
- **Check + executor:** mechanized — `go test ./internal/jobs/... -run Weekly` and the doc-name test.
- **Pass condition:** ac-1 to ac-3 pass.

## Integration Points

- SPEC-018 shows the latest report. SPEC-021's `get_progress` returns it. SPEC-025 turns on the automation.
- **Provenance tag:** `INIT-015/SPEC-023`
