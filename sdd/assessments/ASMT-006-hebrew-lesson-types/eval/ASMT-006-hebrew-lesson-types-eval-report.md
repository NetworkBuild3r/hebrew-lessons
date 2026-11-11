# ASMT-006 · Independent check

**Checker:** a fresh read-only subagent that wrote none of the report. It was given the report, the design, and 12 claims to try to disprove.
**Date:** 2026-11-10
**Verdict:** MITIGATE. Both fixes were applied to the report the same day.

| # | Claim | Result | Evidence the checker read |
| --- | --- | --- | --- |
| 1 | At most 3 new words per lesson, and more blocks | Confirmed | `internal/units/rules.go` 18, `validate_budget.go` 34–36 |
| 2 | The drill cap counts one-token shadow, listen_pick, recall, and produce at 40%, and blocks with a brief. 8 of 12 is right. | Confirmed | `shape_rules.go` 13, 31, 90–120; `validate.go` 80–81 |
| 3 | Every lesson needs a story when `minStoryTokens` is set | Confirmed | `shape_rules.go` 39–56 |
| 4 | Lesson `produce` is Hebrew buttons, not typing | Confirmed | `lesson.go` 1244–1245, 1428–1430; `player.templ` 152–157 |
| 5 | The skill name for `produce` disagrees: spell in the validator, recall in the player | Confirmed | `brief_rules.go` 123–124; `lesson.go` 658–659 |
| 6 | A shared publish runs no brief rules | Confirmed | `tool_publish_unit.go` 80; `tools.go` 553. A learner publish re-checks against the brief while auto-publish is on (188). |
| 7 | The brief has no lesson-type field | Confirmed | `brief/build.go` 252–291, and a search of `internal/` |
| 8 | The authoring guide teaches story-first, and line 5 conflicts with D11 | Confirmed | `authoring.md` 5, 100–106; ADR-0033 120–122 |
| 9 | A single-word lesson recall offers English buttons | Confirmed | `lesson.go` 1239–1243, 1428–1434 |
| 10 | The ADR-0033 lines (A2, D9, D12, D13) | Confirmed | ADR-0033 209, 111, 129–132, 137 |
| 11 | The letters check is not recorded | Confirmed | `brief/build.go` 332–334 |
| 12 | A lesson recall or listen_pick schedules the card at once | Partly true | `lesson.go` 602–616; `evidence/recorder.go` 57–79. A first `listen_pick` miss with no listen card is not scheduled. |

## Fixes applied

1. A5 now names the `listen_pick` first-miss exception.
2. The D12 lines no longer say the conjugation lesson is frozen. D12 freezes word-page noun and verb forms and the brief's pace and tier gates. Whether the forms screen counts is left to Brian.
3. D13 is cited at line 137, not 136.
4. The `put_unit` sentence now notes that a learner publish re-checks against the brief while auto-publish is on.

## Recommendation

The checker found it supported. A server-side recipe rule that runs with and without a brief, a brief that names `lessonType` and `steps`, and word cards the server fills in all follow from the evidence. Retiring four rules meets D13's rule budget.
