# SPEC-005 Completion Summary

**Spec:** SPEC-005 · A teacher builds a lesson from named activities and sees what the learner sees
**Initiative:** INIT-011
**Status:** complete
**Authored by:** INIT-011/SPEC-005

## AC verification

**Tier:** full · pre-flight: gate_actions [] PASS · security_review=standard FAIL · domain=frontend FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false

The orchestrator re-ran the tests (not the author). 32 pass, 0 fail:

`node --test test/plan-editor-lesson-screen.test.js test/plan-editor-one-language.test.js test/plan-editor-plan-screen.test.js test/plan-editor-research.test.js test/activity-menu-schema.test.js`

| AC | Verdict |
|----|---------|
| ac-1 | PASS in CSS. Three columns from 1100px. Stacked below 900px. Not checked in a signed-in browser. |
| ac-2 | PASS. Adding Story posts `/blocks`. No Id or Index field. |
| ac-3 | PASS. A new lesson's activity list is empty. The placeholder is removed after the first save. Removing the only activity is not sent. |
| ac-4 | PASS. Words with two fill-in-the-blank items saves and previews. |
| ac-5 | PASS. Names and hints are one language. Match stays off the menu. |

## Security

No new route. Critical 0, High 0, Medium 0, Low 0.

## Deviations

Match is omitted until INIT-010 SPEC-004 adds a match block. No commit. No run trace in this repo. The wide and phone layouts were checked in CSS, not in the browser.
