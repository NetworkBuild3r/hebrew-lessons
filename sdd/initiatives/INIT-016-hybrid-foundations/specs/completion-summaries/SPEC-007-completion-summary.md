# SPEC-007 completion summary

**Initiative:** INIT-016 · **Spec:** SPEC-007 (service) · **Status:** complete · **Date:** 2026-10-23

## Change

The brief reads recent accuracy over the newest 40 checked attempts and sets a load: ease below 70%, stretch above 90%, steady between, with a 3-point margin before it flips and steady whenever there are fewer than 15 attempts. Ease takes the fewest new words and caps struggle words at 3. Stretch takes the most new words and fewer comfortable words. The payload records `load` and `recentAccuracy`, and its version is 2. A version 1 brief still reads. `get_lesson_brief` explains `load`.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestLoadForThresholds`: both edges, the margin that holds and releases, and the 15-attempt floor. |
| ac-2 | PASS | `TestLoadChangesTheBrief`: ease takes 3 new words and stretch takes 4 for a beginner, both inside the range. `TestTunedNewWordsStayInRange` pins 5 and 8 for an established learner. |
| ac-3 | PASS | `TestPreviousVersionStillReads`: a version 1 payload reads, a version 3 payload is refused. `TestPayloadSchemaV1` pins version 2 with `load` and `recentAccuracy`. |
| ac-4 | PASS | `TestLoadChangesTheBrief` builds the same learner twice and gets the same words. |

## Deviations and disclosures

- No personas and no trace, as in SPEC-001.
- The selection tests pin a steady brief, so their fixtures end with a seen word answered 80% right. That keeps the newest attempts in the steady band, and the accuracy tuning is tested on its own in `load_test.go`.
- "Comfortable at the high end" is the existing cap of 150. Stretch uses 30.
