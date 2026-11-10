# INIT-038/SPEC-009 completion summary: "My answer was also right"

**Status:** in_review, 2026-11-09. Written by the orchestrator from [Also-right tap](fd4307a4-f59b-4915-b633-ede07b557046)'s return. Not independently verified.

## What changed

After a wrong graded answer, "My answer was also right" voids that attempt and restores the card to its state before the answer. The attempt stays wrong. The handler refuses another learner's attempt and a missing CSRF token. Report a problem still hides the unit, and now voids that learner's wrong answers in it in one batch with reason `report`. No MCP tool can void.

Letter and pattern drills write `drill_answers`, not `attempts`, so those screens have no tap. On a match, only the asked word's wrong attempt is voided.

## Tests

`TestAlsoRight`, `TestReportVoids`, and `TestReportForm` passed. `TestReportHidesTheUnitInTheSameResponse` fails before a report is posted: it now expects Today not to link a lesson, and Today still does. That assertion comes from SPEC-011, which is still editing the shell. This spec also edited `internal/web/shell.go`, which SPEC-011 owns. Check that file when SPEC-011 returns so one change does not wipe the other.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no persona agent is installed | Full tools. Disclosed. |
