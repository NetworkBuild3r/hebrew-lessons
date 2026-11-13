# INIT-039: One platform, with lessons that teach from your own classes

**Initiative ID:** INIT-039 · **Status:** complete 2026-11-12 · **Created:** 2026-11-11 · **Owner:** Brian Nelson
**Discipline Profile:** `software` · **Ticket:** none · **Branch:** `main` (in place, as INIT-036 to INIT-038 did)
**Source:** [BRAIN-002](../../brainstorms/BRAIN-002-lesson-types-done-right/decision-document.md), its [spec skeleton](../../brainstorms/BRAIN-002-lesson-types-done-right/spec-skeleton.md) (spec detail is in section 4 of that file), and [Lesson types](../../../docs/design/lesson-architecture.md). This is Stage 2 of [BRAIN-001](../../brainstorms/BRAIN-001-is-the-hebrew-app-built-right/decision-document.md).
**Brian's go:** 2026-11-11. "We need a unified platform and a working Hebrew school that lets me learn Hebrew as fast as possible, based on the actual science of learning another language." He removed the standalone worksheet player (`Aharon's code/`), and chose to amend abandon line A2 so this initiative could open before the Stage 1 exit check finished ([ADR-0033](../../../docs/decisions/ADR-0033-stage-1-trust-and-measurement.md), A2).
**Figma:** File `NJ8HfTPCQgrFty2VVQBMhY`, page `255:71`, board `255:82` ([Lesson types — Nov 2026](https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY?node-id=255-71)).

## What we found

Stage 1 made the app honest: no unfair wrong grade, listening back on, Progress counted as "proved" out of the words you were taught. It did not make the app teach. Four things stand between you and learning Hebrew as fast as the research allows:

1. **Lessons are not built from your own classes.** The research on adult second-language learning (RSCH-003, RSCH-004) points hardest at reading and hearing text where you already know about 90 to 95 percent of the words, plus spaced retrieval and producing the word yourself. Today's lessons are hand-authored units, AI lesson writing is paused, and the picker does not use the tutor's sheets. The tutor audit passed: 286 usable lines, 10 of 10 classes loadable.
2. **Most class vocabulary is not in the lexicon.** The audit found 742 of 974 distinct class tokens missing. A word with no lexicon entry cannot be a card, a key, or a lesson word.
3. **Today does not always offer something new.** It offers review. You study days after a class and need a lesson waiting.
4. **Volume.** Time on task predicts speed more than any method. The success line is 120 study minutes a week, and the app offers one review sitting of about 20 minutes.

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
| SPEC-003 | service | Class sheet import with per-sheet checks | SPEC-002 | 5h | complete 2026-11-11. `lessons.ImportSheet`, `db.SaveClassSheet`. Local tests. |
| SPEC-004 | service | Matcher, T0 test, and lexicon entry checks | SPEC-003 | 6h | complete 2026-11-11. Exact match, one prefix, two hits is a miss, entry checks, ranked pool. Forms are passed in on the word. |
| SPEC-005 | api | MCP tools for the class queue | SPEC-004 | 3h | complete 2026-11-11. `import_class_sheet`, `get_class_queue`. `record_taught_words` stores `sheetRef` when it names a passed sheet and writes nothing when it does not. A word with no sheet still records. |
| SPEC-006 | service | Lesson engine: suspension, taught-first buttons, second ask | SPEC-002 | 5h | complete 2026-11-12. Suspension, taught-first, `WithUnrecorded`, and the flag writes are in. The second ask belonged to the class-sheet player. ADR-0034 D1 and ADR-0035 D1 study in the review player and do not build a second one. |
| SPEC-007 | service | Word lesson picker, assembler, and the pre-flight | SPEC-004, SPEC-006 | 6h | complete 2026-11-11. `lessons.Assemble` and `Preflight`. A lesson under 2 words or 4 graded items becomes the easy lesson. A missed word left out of the passage is added. |
| SPEC-008 | service | Easy lesson assembler and the Today plan | SPEC-007 | 5h | complete 2026-11-12. `TodayOffer` and the easy-lesson assembler stay in the code. Today does not call them: ADR-0034 D1 says the class-sheet assembler is not what Today offers. SPEC-026 is the offer. |
| SPEC-009 | service | Minimal measurement and the weekly readout | SPEC-002, SPEC-007, SPEC-008 | 4h | complete 2026-11-12. The weekly payload carries the five lines, and Progress shows them on the weekly card. Under 3 starts it says too early. |
| SPEC-010 | api | One story tool, and one skill that must call it (cuttable) | SPEC-002, SPEC-007 | 4h | complete 2026-11-11. `put_story` returns every failure in one reply. Migration 025 stores one story. The skill is `.cursor/skills/hebrew-story`. |
| SPEC-011 | frontend | Thin lesson player: word lesson and easy lesson | SPEC-006, SPEC-007, SPEC-008 | 6h | complete 2026-11-12. Not built. ADR-0035 D1: a lesson is studied in the review player, and there is no second player. ADR-0034 D1: the class-sheet assembler is not what Today offers. |
| SPEC-012 | frontend | Flag control | SPEC-006, SPEC-011 | 4h | complete 2026-11-12. A miss can be taken back. A right answer can be flagged as a bad question. Undo puts the grade and the word back. One request does the void, the flag, and the suspension. |
| SPEC-013 | frontend | Today: review first, reserved lesson slot, source label | SPEC-008, SPEC-011 | 4h | complete 2026-11-12. Superseded by ADR-0034 D5 and SPEC-026. Today offers a review sitting, and a sitting of new common words when nothing is due. There is no class-date lesson slot. |
| SPEC-014 | frontend | Speaking-prep page (cuttable) | SPEC-004 | 2h | complete 2026-11-11. `GET /me/speaking` is this learner's tutor lines only. |
| SPEC-015 | service | Release-bar tests | SPEC-007, SPEC-008, SPEC-011, SPEC-012 | 5h | complete 2026-11-12. `TestReleaseBarSixLessons` is the robot. The typed accept-set table stays the INIT-038 test. The class-sheet player those lessons would have opened is not what Today offers (ADR-0034 D1). |
| SPEC-016 | docs | Security Review | SPEC-002 to SPEC-015, SPEC-024, SPEC-026 | 3h | complete 2026-11-12. An agent that did not write this code found no Critical or High issue. Two Medium notes stay open: a class sheet has no total size cap, and studying a tutor class can write a shared lexicon gloss from the sheet. Undo of a void batch is checked for this learner in the web handler. |
| SPEC-017 | docs | Architecture and Design Diagrams | SPEC-016 | 2h | complete 2026-11-12. [diagrams.md](diagrams.md) shows the class queue, the assembler that Today does not call, Today's sitting, the five sections, the flag and Undo, and retirement. |
| SPEC-018 | service | [One review sitting of about 20 minutes](specs/SPEC-018-service-twenty-minute-sitting.md) | none | 2h | complete 2026-11-11. A sitting is about 20 minutes and at most 40 study questions. A faster tap does not lengthen the list. Local tests passed. |
| SPEC-019 | service | [Lessons from the words that cover speech](specs/SPEC-019-service-words-that-cover-speech.md) | none | 4h | complete 2026-11-11. Pinto 2018, 5,000 subtitle lemmas, about 91 percent of that corpus. Today links to the first lesson when nothing is due. Local tests passed. |
| SPEC-020 | service | [Walk the most common words](specs/SPEC-020-service-walk-the-common-words.md) | SPEC-019 | 3h | complete 2026-11-11. The next six after the first six. A sentence stays only at about 95 percent known. Local tests passed. |
| SPEC-021 | docs | [Where a graded meaning comes from](specs/SPEC-021-docs-course-key-source.md) | none | 1h | complete 2026-11-12. ADR-0034 D1, D3, D5 amended. |
| SPEC-022 | data | [The course list](specs/SPEC-022-data-course-list.md) | SPEC-021 | 6h | complete 2026-11-12. `cmd/coursegen`, `course.tsv`, unsigned fixes. Local tests passed. |
| SPEC-023 | data | [Check the first 500 course words](specs/SPEC-023-data-review-first-500.md) | SPEC-022 | 4h | complete 2026-11-12. grok-4.7 signed ranks 1–500: 475 ready, 28 dropped, 4 unsigned. Local tests passed. |
| SPEC-024 | service | [Course words into the lexicon and into review](specs/SPEC-024-service-course-into-review.md) | SPEC-022 | 5h | complete 2026-11-12. Migration 026. Course words are the first source of new review words. Local tests passed. |
| SPEC-025 | service | [Never the same kind of question twice in a row](specs/SPEC-025-service-no-same-kind-twice.md) | none | 2h | complete 2026-11-12. `MaxKindRun` is 1. Local tests passed. |
| SPEC-026 | frontend | [The course page, and Today when words are waiting](specs/SPEC-026-frontend-course-page-and-today.md) | SPEC-024 | 5h | complete 2026-11-12. `/speech` is the course page. Today offers a sitting when words are waiting. Local tests passed. |
| SPEC-027 | service | [The weekly report counts the course](specs/SPEC-027-service-weekly-course-lines.md) | SPEC-024, SPEC-026 | 3h | complete 2026-11-12. The weekly payload has a course block. The course page shows last week. Local tests passed. |
| SPEC-028 | service | [Numbered lessons from the common words](specs/SPEC-028-service-numbered-course-lessons.md) | SPEC-024 | 4h | complete 2026-11-12. 468 words in 94 lessons of five. Local tests passed. |
| SPEC-029 | frontend | [The Lessons page is the course](specs/SPEC-029-frontend-lessons-page-course.md) | SPEC-028 | 5h | complete 2026-11-12. `/lessons` lists the course; `/lessons/{n}` studies its words. Local tests passed. |
| SPEC-030 | frontend | [The Tutor section](specs/SPEC-030-frontend-tutor-section.md) | SPEC-024 | 4h | complete 2026-11-12. `/tutor` lists classes and studies one. Local tests passed. |
| SPEC-031 | service | [Review says where each word came from](specs/SPEC-031-service-review-source-line.md) | SPEC-024 | 2h | complete 2026-11-12. A card front names Common words, tutor, or earlier. Local tests passed. |
| SPEC-032 | frontend | [Learn gets the alphabet and the vowels](specs/SPEC-032-frontend-learn-alphabet-vowels.md) | none | 4h | complete 2026-11-12. `/learn/alefbet` and `/learn/vowels`. Local tests passed. |
| SPEC-033 | frontend | [Progress shows the mix](specs/SPEC-033-frontend-progress-mix.md) | SPEC-026, SPEC-028 | 3h | complete 2026-11-12. Three lines and three links on the brief view; the detail view unchanged. Local tests passed. |
| SPEC-034 | docs | [The five sections, recorded](specs/SPEC-034-docs-five-sections-recorded.md) | SPEC-029 to SPEC-033 | 1h | complete 2026-11-12. ADR-0035. |

**Total:** about 116h, of which about 92h is code. Cut SPEC-010 and SPEC-014 first if time is short.
**Full course (SPEC-021 to SPEC-027):** plan [full-course-plan.md](full-course-plan.md), approval [full-course-approval.md](full-course-approval.md). Build order: 021, 022, then 023 by a different agent while 024 and 025 are built, then 026, then 027. SPEC-016's security review now also covers SPEC-024 and SPEC-026, which add a startup write to the lexicon and a POST route.
**Five sections (SPEC-028 to SPEC-034):** plan [five-sections-plan.md](five-sections-plan.md), approval [five-sections-approval.md](five-sections-approval.md). Build order: 028, then 029 and 031, then 030, then 032 and 033, then 034. SPEC-016's security review also covers SPEC-029, SPEC-030, and SPEC-031, which add three POST routes and two pages that write words.
**Minimum path to a first lesson you can study:** SPEC-001, 002, 003, 004, 005, 006, 007, 011, 013 (41h).

## Done when

The release bar from the skeleton (section 4A), all run by `go test`:

1. A perfect-student run passes on at least 6 fixed sample lessons, including 2 from sheets outside the newest 10 classes and 1 with a held line. A wrong-student run fails every graded step. A lesson with zero graded keys fails.
2. The typed accept-set table (about 150 rows, shipped in INIT-038) passes, including the lesson's write step.
3. No graded key is below T0, and no word on screen is untaught.

And, run locally on a copy of the data: a word lesson and an easy lesson play end to end on a class sheet, Today shows them after review, and the flag voids a grade with a working Undo. The layout audit (`internal/web/testdata/layout-audit.mjs`) reports no issues on the new screens.

## Constraints

- **Nothing is deleted.** The old player `lesson.go`, the old validator, units 1 to 34, and the unit tools stay. They retire in a later group after the new lessons have been on Today for three weeks (BRAIN-002 section 6, Retirement group).
- **No rule is added to the old validator.** Built lessons are checked by the pre-flight in SPEC-007. The story, if an agent writes one, is checked by the one tool in SPEC-010. A sentence in a skill is not a check.
- **One initiative in flight.** Conjugation (slice 2) and retirement are separate later initiatives.
- **Migrations are additive.** No learning record is overwritten. Tests run on a temp database and on a copy of `data\local-dev`.
- **No production deploy.** Every spec closes on a local check.

## Deferred

The conjugation lesson (slice 2), the retirement group (archive units 12 to 34, retire `put_unit`, `publish_unit`, `get_lesson_brief`, `internal/brief`, the paused author skills), the listening lesson (needs a voice), the spelling lesson (cut: typed Write does that work), T1 keys, words from outside the class sheets. Building a stored sentence is in the word lesson (ADR-0034, amended 2026-11-11), not deferred.

## How a lesson is checked

An agent may write the optional story. It does not write the questions, the keys, or the steps for words the passage left out. Those are code. The skill can be large. The tool is one call. A lesson the tool rejects is not saved.

**SPEC-007, the pre-flight, runs in code before a lesson opens.** It is deterministic. Today can call it as a dry run. It refuses or drops a step, and it writes the drop into the frozen lesson with the check id. It does not call the old validator.

| Check | Pass | If it fails |
| --- | --- | --- |
| New words | A word lesson has 2 to 4 T0 words, no two sharing a gloss. An easy lesson has none. | Offer the easy lesson, or review only. |
| Passage | Up to 3 tutor lines, at least 90 percent of the tokens already taught. | Drop the passage. Never fill it with an agent sentence that failed its check. |
| Order of steps | See, then the meaning check, then the passage, then Write with the Hebrew hidden, then building the stored sentence. | The assembler does not emit another order. |
| Sentence key | The key is the stored line, plus any other order stored with it. | A fill-in-the-blank the code cannot prove is not graded. |
| Sentence sound | One clip of the whole line, or no clip. | Word clips are not played in a row as the sentence. |
| Due words left out | A word that ranked into this lesson, or a word due back from an earlier class (two or more classes, a held line, or a miss), and that is not in the passage or the new-word list. | Code adds a step for that word: a meaning check, or the word inside the stored sentence. The agent does not add the step. |
| Graded keys | At least one graded key, and every key is T0. | The lesson does not open. |

**SPEC-010 is one tool, not a set of small ones.** `put_story` takes the story text and returns pass or fail with every broken check in that same response: unpointed text, a word with no lexicon entry, coverage under 90 percent, too long, markup, a repeat of an earlier story, and any due word the story was given and did not use. The agent retries against that list. There is at most one open story slot. The story is checked again when the lesson starts. It is never graded. If it fails, the step is dropped and the lesson still runs.

**One large skill calls that tool.** The paused author skill stays paused for writing whole units. The story skill, when it is written, holds the lesson shape and the research in one place so the agent is not sent through five small skills. The only write it makes is the story. It must call `put_story` and stop when the tool still rejects the text. `put_unit` is not how a story is saved.

**What this does not do.** It does not add rules to the old validator, and it does not let the skill decide the questions. The old validator stays for units 1 to 34 until those units are archived.
