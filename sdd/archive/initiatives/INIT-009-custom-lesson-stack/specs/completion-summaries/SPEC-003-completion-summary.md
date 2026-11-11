# SPEC-003 completion summary

**Spec:** INIT-009/SPEC-003
**Status:** in_review
**Date:** 2026-10-14

## Change

The plan editor shows lessons as one ordered stack, with reorder, a block menu, and a builds-on picker limited to earlier lessons. Admin → Lessons creates a plan instead of a six-section worksheet. Opening a lesson whose id is not a plan id shows a note and does not open the six-section editor. Visibility saving is unchanged.

## Acceptance criteria

| AC | Verdict | Where |
|----|---------|--------|
| ac-1 new lesson form has no six sections | pass | `test/plan-editor-stack.test.js`, re-run 2026-10-14 |
| ac-2 reorder changes saved order | pass | same file |
| ac-3 buildsOn lists earlier lessons | pass | same file |
| ac-4 visibility options stay one language | pass | `test/admin-lesson-editor.test.js` still covers the Everyone option |

The operator browser pass in the spec (add a second lesson, add one block, point it at the first) was not done. No browser tool is available in this session. Login at `http://127.0.0.1:8080/login/` answered 200.

## Limit

The three published lessons use folder names. Their plans use UUIDs. Opening one of those lessons from Admin → Lessons shows “no plan yet” and a link to Plans. Edit them from Plans.

## Harness

Cursor. No frontend persona is installed, so the write ran as a general-purpose worker (`cursor-grok-4.5-high-fast`). Trace and timer scripts are not in this repository.
