# SPEC-003 completion summary

**Spec:** INIT-010/SPEC-003
**Status:** complete
**Date:** 2026-10-14

## Change

`/editor/` is the plan builder. `/admin/` keeps users and the worksheet lesson editor. A plan on the editor page is an ordered stack of lessons, with the activity names and the words earlier lessons added.

## Acceptance criteria

| AC | Verdict | Where |
|----|---------|--------|
| ac-1 editor lists plans and reorders lessons | pass | `test/plan-editor-research.test.js`, re-run 2026-10-14 |
| ac-2 activity menu and running word list | pass | same file |
| ac-3 buildsOn is earlier lessons only | pass | same file |
| ac-4 admin has no plan builder; a worksheet opens in the lesson editor; non-admin gets 403 on `/editor/` | pass | same file |

The Family walkthrough in the browser was not done. No browser sign-in was available. The server process that was already running does not load this route until it is restarted. Restarting also applies migration 007.

## Harness

Cursor. No frontend persona is installed.
