# INIT-039: One platform, with lessons that teach from your own classes

**Initiative ID:** INIT-039 · **Status:** in progress · **Created:** 2026-11-11 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main` (in place, as INIT-036 to INIT-038 did)
**Source:** [BRAIN-002](../../brainstorms/BRAIN-002-lesson-types-done-right/decision-document.md), its [spec skeleton](../../brainstorms/BRAIN-002-lesson-types-done-right/spec-skeleton.md) (spec detail is in section 4 of that file), and [Lesson types](../../../docs/design/lesson-architecture.md). This is Stage 2 of [BRAIN-001](../../brainstorms/BRAIN-001-is-the-hebrew-app-built-right/decision-document.md).
**Brian's go:** 2026-11-11. "We need a unified platform and a working Hebrew school that lets me learn Hebrew as fast as possible, based on the actual science of learning another language." He removed the standalone worksheet player (`Aharon's code/`), and chose to amend abandon line A2 so this initiative could open before the Stage 1 exit check finished ([ADR-0033](../../../docs/decisions/ADR-0033-stage-1-trust-and-measurement.md), A2).
**Figma:** File `NJ8HfTPCQgrFty2VVQBMhY`, page `255:71`, board `255:82` ([Lesson types — Nov 2026](https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY?node-id=255-71)).

## What we found

Stage 1 made the app honest: no unfair wrong grade, listening back on, Progress counted as "proved" out of the words you were taught. It did not make the app teach. Four things stand between you and learning Hebrew as fast as the research allows:

1. **Lessons are not built from your own classes.** The research on adult second-language learning (RSCH-003, RSCH-004) points hardest at reading and hearing text where you already know about 90 to 95 percent of the words, plus spaced retrieval and producing the word yourself. Today's lessons are hand-authored units, AI lesson writing is paused, and the picker does not use the tutor's sheets. The tutor audit passed: 286 usable lines, 10 of 10 classes loadable.
2. **Most class vocabulary is not in the lexicon.** The audit found 742 of 974 distinct class tokens missing. A word with no lexicon entry cannot be a card, a key, or a lesson word.
3. **Today does not always offer something new.** It offers review. You study days after a class and need a lesson waiting.
4. **Volume.** Time on task predicts speed more than any method. The success line is 120 study minutes a week, and the app offers one session of about 25 minutes.

## What you get

- **A word lesson built by code from your tutor's classes.** About 4 new words, a grid with Listen, a meaning check, a passage of the tutor's own lines, then typing the word with the Hebrew hidden. Every lesson is different because the words come from a ranked pool across every loaded class, not only the newest.
- **An easy lesson** with no new words on days when too much is still being learned, so a heavy-review day does not stall.
- **Today: review first, then always one lesson,** labelled with the class its words came from.
- **A flag on every graded item.** One tap voids a bad grade, flags the item and suspends it. It works on right answers too.
- **The class queue over MCP,** so your own AI loads a class sheet and fills the missing words, with the app checking each entry.
- **A page to copy into your AI before a speaking session,** with this week's words and the tutor's sentences. Speaking happens with your AI or your tutor. The app does not score speech.

## Decisions (open questions from BRAIN-002 section 9, settled)

These follow the order the repo rule sets: the ADRs and specs, the research reports, the brainstorms, then the choice that best helps one adult learn Hebrew and remember it. None of them deletes the learning record.

| # | Question | Decision | Why |
| --- | --- | --- | --- |
| A1 | Freeze about 15 backfilled words out of lessons until day 42 so they can be a day-28 baseline? | **No.** The day-28 check is dropped. Each lesson type is judged on use and on the held-out probe S3. | Holding words back slows learning to protect a measurement the probe already provides. |
| A2 | Join attempts to runs. | **Add a nullable `run_id` to `attempts`** in SPEC-002. | A time-window join breaks when a lesson is resumed. |
| A3 | A second word source for T1 keys. | **None in this initiative.** Every graded key is T0: the tutor's own sheet shows the word with its English beside it. | A wrong key is the failure Brian named. |
| A4 | The 75 sheets not yet audited. | **Each sheet passes its own check before it joins the queue.** An agent that did not build SPEC-003 runs the check. | The 286-line count came from 10 sheets and does not carry over. |
| A5 | Voice. | **Lessons use the clips that exist and run silent where none does.** The listening lesson and the 20-item listening probe stay deferred until Brian names a voice he accepts. | His refusal of the computer-read voice on 2026-11-09 stands. |
| A6 | Where sheet text comes from. | **The agent passes the exported text verbatim through MCP.** No Drive credential on the server. | Rule: a secret that is not on this machine is a stop. This avoids one. |
| B7a | A new void reason for the flag? | **No new reason.** A flag is its own table, `item_flags`, and writes an ordinary `answer_voids` row when it takes a grade back. | SQLite cannot change a `CHECK` without rebuilding `answer_voids`, and nothing needs it. |
| B7 | Does a column for full spelling get added? | **No.** INIT-038/SPEC-015 solved it with a fold that rejects collisions (`בית` against `בת`). | One fewer migration. |
| B8 | Are Security Review and Diagrams specs? | **Yes,** SPEC-016 and SPEC-017, as the skeleton lists them. | The handoff requires them. |
| B9 | Tuning values. | **Use the skeleton's proposals:** a floor of 2 words and 4 items, a passage of up to 3 tutor lines at 90 percent taught, a review cap of about 12 to 15 minutes. They are tuned on real sheets. | There is no better data yet. |
| B11 | Start-by date of 2026-12-07. | **Work starts 2026-11-11.** The first lesson on the minimum path lands around 2026-11-22. | The initiative opens earlier than the skeleton assumed. |

## Specs

Detail for each spec is section 4 of the [BRAIN-002 spec skeleton](../../brainstorms/BRAIN-002-lesson-types-done-right/spec-skeleton.md). Each spec file is written when that spec starts, from that section, and linked here.

| Spec | Domain | Title | Blocked by | Effort | Status |
| --- | --- | --- | --- | --- | --- |
| SPEC-001 | docs | [ADR-0034](../../../docs/decisions/ADR-0034-stage-2-lessons-built-from-your-classes.md): the Stage 2 lesson decision | none | 2h | complete 2026-11-11 |
| SPEC-002 | data | Migration 024: lesson type, item flags, class sheets and lines, sheet reference, `run_id` ([`migrations/024_class_sheets_and_flags.sql`](../../../migrations/024_class_sheets_and_flags.sql)) | SPEC-001 | 4h | complete 2026-11-11 |
| SPEC-003 | service | Class sheet import with per-sheet checks | SPEC-002 | 5h | not started |
| SPEC-004 | service | Matcher, T0 test, and lexicon entry checks | SPEC-003 | 6h | not started |
| SPEC-005 | api | MCP tools for the class queue | SPEC-004 | 3h | not started |
| SPEC-006 | service | Lesson engine: suspension, taught-first buttons, second ask | SPEC-002 | 5h | not started |
| SPEC-007 | service | Word lesson picker and assembler | SPEC-004, SPEC-006 | 6h | not started |
| SPEC-008 | service | Easy lesson assembler and the Today plan | SPEC-007 | 5h | not started |
| SPEC-009 | service | Minimal measurement and the weekly readout | SPEC-002, SPEC-007, SPEC-008 | 4h | not started |
| SPEC-010 | api | Optional: the story slot (cuttable) | SPEC-002, SPEC-007 | 3h | not started |
| SPEC-011 | frontend | Thin lesson player: word lesson and easy lesson | SPEC-006, SPEC-007, SPEC-008 | 6h | not started |
| SPEC-012 | frontend | Flag control | SPEC-006, SPEC-011 | 4h | not started |
| SPEC-013 | frontend | Today: review first, reserved lesson slot, source label | SPEC-008, SPEC-011 | 4h | not started |
| SPEC-014 | frontend | Speaking-prep page (cuttable) | SPEC-004 | 2h | not started |
| SPEC-015 | service | Release-bar tests | SPEC-007, SPEC-008, SPEC-011, SPEC-012 | 5h | not started |
| SPEC-016 | docs | Security Review | SPEC-002 to SPEC-015 | 3h | not started |
| SPEC-017 | docs | Architecture and Design Diagrams | SPEC-016 | 2h | not started |

**Total:** about 69h, of which about 52h is code. Cut SPEC-010 and SPEC-014 first if time is short.
**Minimum path to a first lesson you can study:** SPEC-001, 002, 003, 004, 005, 006, 007, 011, 013 (41h).

## Done when

The release bar from the skeleton (section 4A), all run by `go test`:

1. A perfect-student run passes on at least 6 fixed sample lessons, including 2 from sheets outside the newest 10 classes and 1 with a held line. A wrong-student run fails every graded step. A lesson with zero graded keys fails.
2. The typed accept-set table (about 150 rows, shipped in INIT-038) passes, including the lesson's write step.
3. No graded key is below T0, and no word on screen is untaught.

And, run locally on a copy of the data: a word lesson and an easy lesson play end to end on a class sheet, Today shows them after review, and the flag voids a grade with a working Undo. The layout audit (`internal/web/testdata/layout-audit.mjs`) reports no issues on the new screens.

## Constraints

- **Nothing is deleted.** The old player `lesson.go`, the old validator, units 1 to 34, and the unit tools stay. They retire in a later group after the new lessons have been on Today for three weeks (BRAIN-002 section 6, Retirement group).
- **No validator rule is added.** Built lessons are checked by the pre-flight and by tests.
- **One initiative in flight.** Conjugation (slice 2) and retirement are separate later initiatives.
- **Migrations are additive.** No learning record is overwritten. Tests run on a temp database and on a copy of `data\local-dev`.
- **No production deploy.** Every spec closes on a local check.

## Deferred

The conjugation lesson (slice 2), the retirement group (archive units 12 to 34, retire `put_unit`, `publish_unit`, `get_lesson_brief`, `internal/brief`, the paused author skills), the listening lesson (needs a voice), the spelling lesson (cut: typed Write does that work), graded word order, T1 keys, words from outside the class sheets.
