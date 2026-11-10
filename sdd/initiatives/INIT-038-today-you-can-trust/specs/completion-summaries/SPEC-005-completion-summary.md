# INIT-038/SPEC-005 completion summary: the proved ladder

**Status:** in_review, 2026-11-09. Written by the orchestrator from [Proved ladder](8be708d8-89f3-48b5-97ef-ad67a9e87524)'s return. Not independently verified.

## What changed

Each taught word, in each skill, is proved, recalled later, slipping, or not yet asked. The count leaves out cards that only placement created. A voided answer counts for nothing. Daily snapshots are stamped rule version 5. `KnownRule4` still returns the old count. Digest and `get_context` report the four rungs. `knownLexemes` is recalled later or proved. The rule lives in `internal/learner/rule` because package `db` cannot import package `learner`.

The Progress headline was not changed. `KnownAnySkill` still uses rule 4, so the page does not move until SPEC-010. `cmd/hebrew` help text still says rule version 4. The rebuild stamp uses `jobs.RuleVersion`, which is 5.

## Tests

`internal/db` passed. Ladder, known-lexeme, and rule-version tests passed. `internal/learner` failed `TestListFirstPageOver50kAttempts` (300ms against 200ms). It passes with `LEARNER_PERF_SLACK=2`. That bench is pre-existing. `TestStandingDueIsCardsDueNow`, one snapshot fixture, and two jobs listen-replay tests failed. The worker attributes those to the clip lookup and same-day due work, which this spec did not edit.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no persona agent is installed | Full tools. Disclosed. |
