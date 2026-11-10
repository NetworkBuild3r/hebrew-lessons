# INIT-038/SPEC-010 completion summary: Progress leads with the ladder

**Status:** in_review, 2026-11-09. Written by the orchestrator from [Progress ladder headline](15614523-2530-463c-99a2-929485fd00ce)'s return. Not independently verified.

## What changed

Progress leads with Understand by ear and Hebrew from English. Each row shows proved, recalled later, slipping, and not yet asked, out of the words the app has taught. Each count opens that list. Today's "Words you know" is the Hebrew-from-English proved count. For 14 days from 2026-11-09 the page shows What changed: the old rule-4 count with voided answers included, the count after the void, and the proved count. The panel is gone from 2026-11-23. The history line uses one rule version. `today.templ` was not edited.

No browser was available, so there is no screenshot. The counts wrap, and the comparison stacks in one column under 640px.

## Tests

Every `TestProgress*` test passed, including `TestHomeAndProgressShareKnownAnySkill`. `go test ./internal/web/` failed `TestLessonRecallKeepsANewWordClose`: a new card is due in 21 hours, and the test still wants it within 15 minutes. That expectation belongs to the same-day cutoff in SPEC-006. This spec did not edit the session scheduler.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no persona agent is installed | Full tools. Disclosed. |
