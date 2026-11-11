# SPEC-001 completion summary

**Spec:** INIT-009/SPEC-001
**Status:** complete
**Date:** 2026-10-14

## Change

ADR-0012 records that a lesson is the blocks the instructor stored, that an empty activity list is not filled with vocabulary, fill, grammar, reading, and ask-a-friend, and that a plan is an ordered stack of lessons. ADR-0003's extended-by line and changelog name that record. No code, no migration, no commit.

## Acceptance criteria

| AC | Verdict | Where |
|----|---------|--------|
| ac-1 D1–D4 and RSCH-004 §3 and §4 | pass | `docs/decisions/ADR-0012-custom-lesson-stack.md` decision rows 1–4 and the research-grounding line |
| ac-2 empty list is not the six-part ladder | pass | same file, decision row 5 |
| ac-3 ADR-0003 names ADR-0012 | pass | `docs/decisions/ADR-0003-course-and-lesson-plan-language.md` Extended by row and the 2026-10-14 changelog row |

Checked by reading those files after the writer returned, not from the writer's report alone.

## Harness

Cursor session. No `docs-agent` persona is installed here, so the write ran as a general-purpose worker. Trace and timer scripts are not in this repository; nothing was emitted. The operator chose to stay in this checkout.

## Gates

None.
