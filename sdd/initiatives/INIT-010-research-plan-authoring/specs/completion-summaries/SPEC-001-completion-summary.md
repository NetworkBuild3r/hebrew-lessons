# SPEC-001 completion summary

**Spec:** INIT-010/SPEC-001
**Status:** complete
**Date:** 2026-10-14

## Change

ADR-0013 records the plan: optional activities, no maximum lesson count, a running word list, the learner's place, and practice of reached words and verb forms on the existing review scheduler. ADR-0012 and ADR-0003 point at it. No code.

## Acceptance criteria

| AC | Verdict | Where |
|----|---------|--------|
| ac-1 D1–D8, Family, word list, place, practice | pass | `docs/decisions/ADR-0013-research-shaped-plans.md` decision rows 1–8 and §2 |
| ac-2 warnings named | pass | same file, decision row 4 |
| ac-3 later day is the review scheduler | pass | same file, decision row 9 |
| ac-4 ADR-0012 points at ADR-0013 | pass | ADR-0012 Corrected by row |

Checked by reading the files after they were written. One leaked instruction word ("Name later-context") was rewritten into a sentence.

## Harness

Cursor. No docs persona is installed, so the write ran as a general-purpose worker. Trace and timer scripts are not in this repository. The operator chose this checkout.
