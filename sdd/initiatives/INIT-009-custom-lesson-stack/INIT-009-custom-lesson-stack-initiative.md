# Initiative: Custom lessons in an ordered plan

**Initiative ID:** INIT-009
**Status:** paused 2026-10-14. The operator corrected the model the same day: a lesson is the activities (words, story, match, spelling, reinforcement), and a plan is those lessons stacked. INIT-010 plans that. Do not execute the rest of INIT-009. SPEC-003's "no plan yet" gate is in the way and INIT-010/SPEC-004 removes it.
**Slug:** INIT-009-custom-lesson-stack
**Discipline Profile:** `software` (config §7.1 `active_discipline_profiles` was not present in the skill config read for this session; the procedure fails closed to `software`, and that is the profile this repo already uses)
**Created:** 2026-10-14
**Owner:** Brian Nelson
**Ticket:** None (`branch_naming.ticket_system: none`; sourced from the operator request of 2026-10-14)
**Branch:** `custom-lesson-stack` (not created). Orchestration on 2026-10-14: no worktree holds that branch (Outcome A). The operator chose to write SPEC-001 in this checkout. A worktree was not created. No commit unless the operator asks.
**Target Completion:** not a promise. Effort in the spec table is a planning estimate that starts only after the operator says to execute.
**Source:** operator request 2026-10-14: "the lesson should be more custom. not forcing me to have vocabulary, activity path, fill, grammer, reading, askfriend. Based on the research. WE should allow the instrcuture to build a lesson to what makes sense. A plan should be a stack of lessons in order. Where words build on each other." Research: `RSCH-004` (course shape, four strands, candidate lesson types, design rules). Decisions it builds on: ADR-0003 (units of ordered lessons, `buildsOn`, `recycles`, optional blocks, recipes), ADR-0011 (the plan editor is the authoring surface). Depends on INIT-003, INIT-004, and INIT-008, all complete.

---

## 0. Source tree used for current-behavior claims

`git rev-parse HEAD` on 2026-10-14: `a2725eb5143665aaf1eb4ea560363f597c236515` on branch `main`. `git fetch` was not run. The working tree holds uncommitted work. Claims below cite the working tree as read on 2026-10-14.

| Fact | Citation |
|------|----------|
| The lesson editor always renders a vocabulary table, an activity path, and JSON sections named fill, grammar, reading, and askFriend | `shared/admin/lesson-editor.js` `KNOWN_JSON_SECTIONS`, `renderVocabTable`, `renderActivityPath` |
| When a worksheet has no `activities` array, the player invents words, fill, grammar, reading, and ask-a-friend | `shared/player/core.js` `fallbackActivities`, `resolveSteps` |
| A plan is already units of lessons. Each lesson has an ordered `blocks` list, `buildsOn` (earlier lesson ids), `recycles` (`fromLessonId`), and `newWords` | ADR-0003; `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` |
| A new plan is one lesson with a single fluency-pass placeholder, because a lesson must contain at least one block | `server/plans/create.js` |
| Recipes name a lesson type. They set `schemaVersion` and `recipe`. They do not write a full activity skeleton | `server/plans/recipes.js` |
| `RSCH-004` §1.4: a unit mixes four strands (input, output, deliberate study, fluency). Deliberate study is the minority. No source requires one fixed drill ladder | `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md` §1.4, §4 |
| `RSCH-004` §3 rules 5 and 6: a new word returns on a later day, in a later lesson, in a different sentence. Rule 3: a fluency block adds no new word | same report §3 |
| Highest ADR on disk is ADR-0011. This initiative's decision is ADR-0012 | `docs/decisions/` |

---

## 1. Executive Summary

The screen the operator uses to edit a lesson always offers the same six parts: vocabulary, an activity path, fill, grammar, reading, and ask-a-friend. An empty activity list is filled in with that same ladder when the learner plays it. That fights the research. `RSCH-004` says an instructor picks the strands a lesson needs (a story, listening, a short dialogue, a fluency pass, a later recall), and that most of a unit is meaning, not drills. It also says new words come back in a later lesson, not only inside the lesson that introduced them.

The plan language already has the pieces: ordered lessons, optional blocks, `buildsOn`, and `recycles`. INIT-009 makes that the way a lesson is built.

1. **A lesson is the blocks the instructor adds, in the order they add them.** Vocabulary, fill, grammar, reading, and ask-a-friend are available block types. None of them is required. An empty lesson is not back-filled with that ladder.
2. **A plan is a stack of lessons in order.** The order on the plan is the teaching order. A unit stays in the document as the theme group the schema already has; the editor presents one ordered stack, and a new plan starts as one unit holding that stack.
3. **Words build on earlier lessons.** `buildsOn` names lessons above this one in the stack (or a lesson in another plan). `recycles` names the earlier lesson a returning word came from. A fluency block adds no new word (`RSCH-004` rule 3). Recall of a new word belongs in a later lesson (`RSCH-004` rules 5 and 6), as a warning, not as a template the editor inserts.

The three published worksheets keep their current files and keep playing. This initiative does not rewrite them and does not rename their folders.

---

## 2. Operator decisions (2026-10-14)

| Id | Decision |
|----|----------|
| D1 | Do not force vocabulary, an activity path, fill, grammar, reading, or ask-a-friend. The instructor adds the blocks the lesson needs. |
| D2 | The menu of blocks follows `RSCH-004` §4 (introduction, graded story, listening, short dialogue, fluency pass, spaced-review checkpoint, script and sound support, optional comprehension check). Existing plan block types stay. A block type the research does not require is still allowed; it is just not inserted automatically. |
| D3 | A plan is an ordered stack of lessons. Words in a later lesson may build on words from an earlier lesson. |
| D4 | Existing published lessons stay as they are. New authoring does not start from the six-part skeleton. |

---

## 3. Requirements

| Id | Requirement | Priority |
|----|-------------|----------|
| REQ-001 | A new lesson does not contain a vocabulary table, an activity path, fill, grammar, reading, or ask-a-friend unless the instructor adds that block. | must |
| REQ-002 | The player plays the blocks that are present, in order. It does not invent the six-part ladder when the list is empty or short. | must |
| REQ-003 | The instructor can add, remove, and reorder blocks on a lesson. The allowed set includes the `RSCH-004` §4 lesson types that already have a plan block, plus the existing plan block types. | must |
| REQ-004 | A plan shows its lessons as one ordered stack. Reordering a lesson changes the teaching order. | must |
| REQ-005 | `buildsOn` on a lesson is chosen from lessons earlier in that stack, or from a lesson id the instructor names in another plan. | must |
| REQ-006 | A recycled word records the earlier lesson it came from (`recycles[].fromLessonId`). | must |
| REQ-007 | A fluency block cannot introduce a `newWord`. A warning, not a forced block, notes when a new word has no recall in a later lesson. | should |
| REQ-008 | The three published worksheets (`my-morning`, `shai-date-indirect-speech`, `anal-sex-vocab`) are not rewritten and their folder names stay. | must |

---

## 4. Spec breakdown

| Spec | Domain | What it delivers | Effort | Blocked by |
|------|--------|------------------|--------|------------|
| SPEC-001 | docs | ADR-0012: a lesson is chosen blocks; a plan is an ordered stack; words chain with `buildsOn` and `recycles`; the six-part ladder is not a template | 2h | none |
| SPEC-002 | service | Create, validate, and play a lesson that has only the blocks the author stored. Stop the empty-activity fallback. Keep the three published worksheets byte-for-byte | 4h | SPEC-001 |
| SPEC-003 | frontend | Plan editor presents the stack, the block menu, and a `buildsOn` / recycle picker. The lesson editor no longer opens with the six required sections | 6h | SPEC-001 |

SPEC-002 and SPEC-003 can run in parallel after SPEC-001. Critical path is SPEC-001, then SPEC-003 (the screen the operator asked about).

---

## 5. Threat model

No new trust boundary. Authoring stays admin-only, enforced on the server, as ADR-0011 already requires. Lesson documents are content, not credentials. This initiative does not add an account, a secret, a migration, or a public write. Security review class on the specs is `standard`.

---

## 6. Out of scope

- Rewriting the three published worksheets, renaming their folders, or changing progress keys.
- Flipping `SRS_SCHEDULER`.
- A Hebrew frequency list (`RSCH-004` rule 9 stays unenforced).
- Reordering unseen lessons from a learner's known words (`RSCH-004` rule 14).
- New block types that do not already exist in the plan schema. SPEC-003 uses the types the schema already allows.
- A commit or a push. The operator has not asked for one.

---

## 7. Expertise check

`scripts/dev/expertise_state.py` is not in this repository. `.claude/agents/` is not the dispatch path this app uses. No persona was manufactured. Each spec names a `primary_prompt` role for the skill's template. Execution degrades to the session agent, disclosed, until a persona exists.

---

## 8. Success

The operator can open a plan, add a second lesson under the first, put only the blocks that lesson needs on it, point `buildsOn` at the earlier lesson, and publish. The learner plays those blocks and no others. A brand-new lesson does not arrive with vocabulary, fill, grammar, reading, and ask-a-friend already on it.
