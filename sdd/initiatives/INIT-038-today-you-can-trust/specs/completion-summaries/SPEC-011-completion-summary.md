# INIT-038/SPEC-011 completion summary: Today offers one session

**Status:** complete, 2026-11-10. Written by the orchestrator from [Today one session](c5a2aaf0-977e-4043-87dc-1b0a71235c6f)'s return. Phone capture `shots/today-390.png` does not scroll sideways. Figma frame `257:123`.

## What changed

Today's main action is "Today's session, about N minutes," sized from cards that can be asked today. With nothing due, Today says so and offers no button. `SHOW_ROOTS`, `SHOW_ISAIAH`, `SHOW_PLACEMENT`, and `SHOW_CHECKIN` default off. A route behind an off switch returns 404. No side-feature code was deleted.

`internal/web/shell.go` still registers the also-right handler from SPEC-009 (`postAlsoRight` in `web.go`) and the new switches (`showRoots` and the rest). Both edits are present.

Lesson word-card root links are still set in `lesson.go`, which this spec was told not to edit. `/roots` returns 404 while the switch is off. No browser was available, so there is no screenshot.

## Tests

The Today, Home, Shell, Roots, Isaiah, and Placement web tests passed. The rest of `./internal/web/` passed except `TestLessonRecallKeepsANewWordClose`, which still expects a new card within 15 minutes. `./cmd/hebrew/` passed, including the four switches defaulting off. `TestNumbersAgreeAcrossSurfaces` fails on the proved "Words you know" count from SPEC-010. Progress files were not edited here.

## Harness adaptation

| Site | Capability | Tier | What was done | Impact |
| --- | --- | --- | --- | --- |
| orchestration dispatch | subagent_dispatch | T2 cursor | generalPurpose, because no persona agent is installed | Full tools. Disclosed. |
