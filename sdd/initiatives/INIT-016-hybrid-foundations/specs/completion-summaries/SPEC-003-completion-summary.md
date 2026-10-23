# SPEC-003 completion summary

**Initiative:** INIT-016 · **Spec:** SPEC-003 (api) · **Status:** complete · **Date:** 2026-10-23

## Change

`upsert_lexemes` takes `root` and `binyan`, and its description says how to write a root and lists the binyan values. `lookup` returns both. `hebrew://authoring` and the `hebrew-lesson-author` skill explain them.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestUpsertThenLookupReturnsRootAndBinyan`: an admin upsert stores `שׁ־מ־ע` and `paal`, and lookup returns them. |
| ac-2 | PASS | `TestLearnerAndAnonymousCannotUpsertLexemes` still refuses a learner token. |
| ac-3 | PASS | `TestUpsertLexemesSchema` pins `root`, `binyan`, the maqaf description, and `hitpael`. |

## Deviations and disclosures

- No personas and no trace, as in SPEC-001.
- The authoring sentence says "a search returns both" rather than naming the lookup tool, so the guide's tool-order check still sees each tool once.
