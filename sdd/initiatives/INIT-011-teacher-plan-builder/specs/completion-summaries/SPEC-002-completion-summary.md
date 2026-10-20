# SPEC-002 Completion Summary

**Spec:** SPEC-002 · Put Verse back, and keep the menu and the schema in step
**Initiative:** INIT-011
**Domain:** data · **Autonomy:** auto_tests
**Status:** complete
**Authored by:** INIT-011/SPEC-002

## Change produced

| File | Change |
|------|--------|
| `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` | `verseReadingBlock` is back in `block.oneOf`, beside `typeItBlock` |
| `shared/admin/activity-map.js` | Writing, Notes, and Verse added. Match removed |
| `shared/admin/plan-builder.js` | Same names, with Hebrew labels כתיבה, הערות, פסוק |
| `test/activity-menu-schema.test.js` | Menu and schema stay in step |
| `test/plan-editor-research.test.js` | Match is expected to be absent |
| `test/plan-lesson-stack.test.js` | Same Match expectation. The first edit left this assertion behind; it was updated before this spec was marked complete |

## AC verification

**Tier:** full · pre-flight: gate_actions [] PASS · security_review=light PASS · domain=data FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false

Independent verifier, then a re-run after the leftover Match assertion was updated.

| AC | Verdict | Evidence |
|----|---------|----------|
| ac-1 | PASS | `verseReadingBlock` at schema line 223, `typeItBlock` at 225. Verse fixture validates |
| ac-2 | PASS | `test/activity-menu-schema.test.js` 2 pass. It failed before the schema and menu edits |
| ac-3 | PASS | `activity-map.js` lists Writing, Notes, Verse |

Tests after the leftover fix: `node --test test/activity-menu-schema.test.js test/plan-editor-research.test.js test/plan-lesson-stack.test.js` — 14 pass, 0 fail.

## Security

Schema restore and a menu change. No new route. Critical 0, High 0, Medium 0, Low 0.

## Gates

None. No commit. Match stays off the menu until INIT-010 SPEC-004 adds a match block.

## Deviations

- No data-agent persona file in this repo.
- No run trace. This repo has no trace script.
- Work stayed in this checkout. Branch `teacher-plan-builder` was not created.
