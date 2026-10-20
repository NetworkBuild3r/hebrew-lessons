# SPEC-004 Completion Summary

**Spec:** SPEC-004 · A teacher starts a plan, adds and reuses lessons, and keeps a word bank
**Initiative:** INIT-011
**Status:** complete
**Authored by:** INIT-011/SPEC-004

## AC verification

**Tier:** full · pre-flight: gate_actions [] PASS · security_review=standard FAIL · domain=frontend FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false

Independent verifier. `node --test` on the plan-screen, list-create, research, and one-language tests: 50 pass, 0 fail.

| AC | Verdict |
|----|---------|
| ac-1 | PASS. New plan is a title and who it is for. A learner posts assign as personal. No recipe field. |
| ac-2 | PASS. One column by default. Two columns at 900px. The open row is marked. |
| ac-3 | PASS. Add lesson posts the typed title. The row shows that title. |
| ac-4 | PASS. Reuse mints a free id and drops buildsOn ids that are not earlier in this plan. |
| ac-5 | PASS. Three pasted lines become three words. A line with no separator is reported. |

## Security

No new route. Pasted text uses textContent. Critical 0, High 0, Medium 0, Low 0.

## Deviations

The lesson activity form is still the old one. SPEC-005 replaces it. No commit. No run trace in this repo. The phone and wide layout were checked in CSS, not in a signed-in browser.
