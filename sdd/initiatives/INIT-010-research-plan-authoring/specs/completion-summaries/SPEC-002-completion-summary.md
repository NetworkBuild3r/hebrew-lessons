# SPEC-002 completion summary

**Spec:** INIT-010/SPEC-002
**Status:** complete
**Date:** 2026-10-14

## Change

A plan can gain, reorder, and drop lessons, and it no longer stops at 12. `buildsOn` is not capped at 12. Words, Story, Match, and Type it map to stored block types. `picture-on-recall` and `closed-set` are warnings and do not stop publish. `later-context` still warns and still allows publish.

## Acceptance criteria

| AC | Verdict | Where |
|----|---------|--------|
| ac-1 three lessons, reorder, 100 lessons | pass | `test/plan-lesson-stack.test.js`, re-run 2026-10-14 |
| ac-2 activity map | pass | same file |
| ac-3 later-context warns and publish succeeds | pass | same file |
| ac-4 new warnings are not publish blocks | pass | same file |
| ac-5 admin routes are on the admin write list; MCP tools require the bearer token | pass | same file |

## Database

Editing migration 006 does not change a database that already applied it. Migration 007 rebuilds `plan_revisions` when the old check is still in place, and copies the rows. It has not been applied to `data/hebrew.sqlite`. The next server start applies it. Tests use a temporary database and did not open the live file.

## Harness

Cursor. No service persona is installed. Trace and timer scripts are not in this repository.
