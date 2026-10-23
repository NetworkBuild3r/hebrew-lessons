# SPEC-005 completion summary

**Initiative:** INIT-016 · **Spec:** SPEC-005 (service) · **Status:** complete · **Date:** 2026-10-23

## Change

`learner.Model.CanSay` returns the words and phrases whose recall state is known or solid, phrases first and then the most recently learned, with a weekly count and up to five learning words that are closest. It reads one learner.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestCanSay`: known and solid words are listed, a learning word is in Next, and a struggling word is in neither. |
| ac-2 | PASS | `TestCanSayWeekBoundary`: a word learned seven days ago counts, one learned eight days ago does not. A word that has slipped out of known is not counted. |
| ac-3 | PASS | `TestCanSay` builds a second learner and checks each list holds only that learner's words. |

## Deviations and disclosures

- No personas and no trace, as in SPEC-001.
- "Moved into known" is taken as the earliest recall review of the word. The review log does not record the resulting card state, so the first review is the closest record, and a word no longer known or solid is excluded because it is not in the list.
