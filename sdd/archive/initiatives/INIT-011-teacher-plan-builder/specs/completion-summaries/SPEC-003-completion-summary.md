# SPEC-003 Completion Summary

**Spec:** SPEC-003 · English shows English, and every refused write says why
**Initiative:** INIT-011
**Status:** complete
**Authored by:** INIT-011/SPEC-003

## AC verification

**Tier:** full · pre-flight: gate_actions [] PASS · security_review=standard FAIL · domain=frontend FAIL · autonomy_level=auto_tests FAIL => light_tier_eligible=false

Independent check, then a fix for labels that were still one bilingual string, then `node --test test/plan-editor-one-language.test.js test/plan-editor-research.test.js` — 14 pass, 0 fail.

| AC | Verdict |
|----|---------|
| ac-1 | PASS. Count and issue jump are separate Hebrew and English spans. The English span has no Hebrew. |
| ac-2 | PASS. `.plan-filter-search` has `background-image: none`. |
| ac-3 | PASS. A refused Add lesson shows a sentence. A schema refusal names the activity and the missing field, not a JSON path. Two adds still leave three rows. |

## Also changed

Chrome that was still one `Hebrew (English)` string now splits, including the history cut marker, revision title, new-plan error, aria-labels, and the schema-form buttons and notes. Server text still goes through `textContent`.

## Security

No new route. Critical 0, High 0, Medium 0, Low 0.

## Deviations

One pre-existing failure remains in `plan-editor-issues-preview.test.js` (`type-it has a preview`). It was not caused by this spec and was not redesigned. No run trace in this repo. No commit.
