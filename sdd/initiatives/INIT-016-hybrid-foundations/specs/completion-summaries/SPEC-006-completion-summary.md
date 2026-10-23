# SPEC-006 completion summary

**Initiative:** INIT-016 · **Spec:** SPEC-006 (frontend) · **Status:** complete · **Date:** 2026-10-23

## Change

Progress gains a "You can now say" section: the count, the weekly change, the list with a play orb where a clip exists, and "Almost there". Today gains one line linking to it. Before any word is known, the section says how the list fills in and Today shows no count.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestCanSaySectionRendersTheList` and `TestTodayCanSayLine`: the list, both counts, and the Today line that links to Progress. |
| ac-2 | PASS | Hebrew renders in `bdi lang="he" dir="rtl"`. `TestProgressInBrowser` passes at 390 and 1440, light and dark, with no sideways scroll, 44px targets, and contrast held on the page that now contains the section. |
| ac-3 | PASS | `TestCanSaySectionEmpty`: the empty line and no count. `TestTodayCanSayLine`: Today shows nothing when the count is zero. |

## Deviations and disclosures

- No personas and no trace, as in SPEC-001.
- The browser run covers the whole Progress page, whose seeded learner has no recall-known words, so it exercises the empty section and the layout. The populated list is locked by the render test.
