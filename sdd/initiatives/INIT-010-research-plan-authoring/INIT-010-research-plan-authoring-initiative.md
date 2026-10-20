# Initiative: Build a research-shaped plan from the admin builder and from MCP

**Initiative ID:** INIT-010
**Status:** on_track (SPEC-001, SPEC-002, and SPEC-003 complete 2026-10-14. SPEC-004, SPEC-005, and SPEC-006 are not started. `/editor/` is not live until the server restarts.)
**Slug:** INIT-010-research-plan-authoring
**Discipline Profile:** `software`
**Created:** 2026-10-14
**Owner:** Brian Nelson
**Ticket:** None (`branch_naming.ticket_system: none`)
**Branch:** `research-plan-authoring` (not created). Documents were written in this checkout. The tree already holds uncommitted work, so a worktree was not opened. No commit unless the operator asks.
**Target Completion:** not a promise. The spec table is an estimate that starts when the operator says to execute.
**Source:** operator, 2026-10-14. A lesson is the activities the instructor picks (words, story, match, spelling, reinforcement, verbs). A plan is any number of those lessons in order, from one to a hundred, with no maximum. The plan tracks words added by earlier lessons and where the learner stopped. Practice is a word list of what they have reached, including verb conjugation and verb listening, on the schedule the app already uses. The AI builds the plan through MCP, and the admin builder builds the same plan. Research: `RSCH-003`, `RSCH-004`, `RSCH-005`. Builds on ADR-0001, ADR-0003, ADR-0009, and ADR-0011. Corrects INIT-009. Does not continue INIT-009's remaining work.

---

## 0. What is already true

| Fact | Where |
|------|--------|
| A plan is units of ordered lessons. A lesson has `buildsOn`, `recycles`, `newWords`, and an ordered `blocks` list | ADR-0003 |
| MCP and the admin editor call the same plan operations. Warnings do not stop publish | ADR-0011 |
| `create_plan` makes one lesson and one placeholder block. `insert_block` adds a block to a lesson that already exists. There is no operation whose job is "add the next lesson" | `server/plans/create.js`, `mcp/tools.js` `insert_block` |
| The plan player renders `introduction`, `graded-story`, `listening`, `short-dialogue`, `fluency-pass`, and `spaced-review-checkpoint` | `shared/ui/plan-render.js` |
| The worksheet player renders a different list: words, match, story, fill, pattern, retell, ask-friend, speed-quiz, build-sentence, listen-pick, type-it | `shared/activities/catalog.js` |
| A multi-lesson plan already warns when a new word never appears again in a later lesson (`later-context`), and when a fluency block sits in a lesson that introduces new words (`N12`) | `server/plans/validate.js` |
| Pictures are allowed on introduction words. A person approves the picture bytes | ADR-0003, ADR-0004 |
| The three published worksheets are still the lesson editor, and their folder names stay | INIT-009 D4, still true |
| INIT-009/SPEC-003 sends Admin → Lessons into the plan screen, or into a "no plan yet" note | `shared/admin/admin-shell.js`, `shared/admin/lessons.js` |

---

## 1. Executive summary

One plan is the course. It holds as many lessons as the instructor wants, one or a hundred. Each lesson is only the activities that lesson needs: Words, Story, Match, Type it, Listening, Dialogue, Fluency, Review, Pattern. Family 1 can be words and a picture. Family 2 can be a story and recall, with no new word list. Family 3 can add verb forms. Nothing is inserted automatically.

The plan keeps two records for each learner. The word list is every word introduced by lessons up to the one they finished, in that order. The place is that last finished lesson. The next lesson is the next one in the plan. Practice is that word list, on the review schedule already in the app: the learner recalls the word before the answer is shown, in both directions, on a later day. Verb conjugation and verb listening use forms whose lemma is already on that list. They are untimed, and a miss shows the right form. Practice does not open a lesson the learner has not reached.

The admin plan builder and the MCP tools write the same plan. The builder is a page of its own, `/editor/`. Admin stays the page for users and for the three published worksheets. Warnings tell the author when a new word never returns, when a fluency lesson adds a word, when a picture is left on Type it, and when a lesson is only a closed set such as relatives. Warnings do not block publish. The three published worksheets stay on the lesson editor. The review scheduler is not replaced, and `SRS_SCHEDULER` is not switched.

---

## 2. Operator decisions (2026-10-14, evening)

| Id | Decision |
|----|----------|
| D1 | A lesson is the activities the instructor adds. Words, story, match, spelling, reinforcement, and a pattern are all allowed. None is required on every lesson. |
| D2 | A plan is those lessons in order. Later lessons build on earlier words. Family 1 can have Words. Family 2 can leave Words out. Family 3 can add Pattern. |
| D3 | The AI builds that plan through MCP. The admin plan builder builds the same plan through the same operations. |
| D4 | Research checks are warnings. They do not lock the instructor, and they do not block publish. |
| D5 | The three published worksheets stay as they are. New courses are plans. |
| D6 | A plan holds as many lessons as the instructor wants. One lesson is allowed. A hundred is allowed. There is no maximum. |
| D7 | A plan keeps the running list of words introduced by its lessons. Practice uses only the words from lessons the learner has reached. The next lesson is the next one in order. |
| D8 | Verb practice is untimed conjugation and listening of forms already in that reached list. It works like the review cards: recall, then a later day, both directions. |
| D9 | The plan builder is its own page at `/editor/`. Admin stays a different page: users, lesson visibility, and the three worksheets. |

---

## 3. How a Family plan maps onto the research

| Lesson | Activities | Research job | Already enforced |
|--------|------------|--------------|------------------|
| Family 1 | Words, optional picture, Match | First meeting. English meaning is the reliable gloss. A picture is optional (`RSCH-005`) | Introduction words require a meaning. Images only on those words |
| Family 2 | Story, Match, Type it, Review. No new word list | The words return in a new sentence. Type it is recall with the picture hidden (`RSCH-003`, `RSCH-005`) | `later-context` warns if a new word never returns. The plan player cannot play Match or Type it yet |
| Family 3 | Pattern (one), Story that uses it, Fluency with no new word | One new pattern, also in the story. Fluency adds nothing (`RSCH-004` rules 3 and 10, both product choices) | `N12` warns when fluency sits on a lesson that introduces words. One-pattern is `N5` |
| A later day | Practice list of words already reached, both directions | Spacing is the strongest finding (`RSCH-003`). Playing Family 1, 2, and 3 in one sitting does not deliver it | The per-word schedule exists (ADR-0001). SPEC-006 limits it to words and verb forms from finished lessons |

A lesson whose new words are only kinship terms (mother, father, sister) gets a warning. A family theme that mixes people, places, and actions does not. The evidence for that warning is Low (`RSCH-004` rule 8). It stays a warning.

## 3.1 Reinforcement

The practice list is the words the learner has already met in this plan, brought back on a later day. That is the finding the research supports most strongly (`RSCH-003`: spacing is High). Recalling the word beats reading it again (Moderate). Each word is practiced in both directions, Hebrew to English and English to Hebrew (Moderate). A card session by itself fades: delayed tests in the flashcard research drop well below the score from the same day (Moderate), which is why the list is scheduled, not a page of words to reread.

The plan does two jobs the cards do not. It keeps the running list, in lesson order, so Family 3 still shows the words from Family 1. And it remembers the last lesson this learner finished, so the next lesson is the next one in the plan. Practice may add due cards. It does not skip the learner ahead (`RSCH-004` rule 14).

Verbs use the same pool. Once a finished lesson has introduced a verb, untimed conjugation and listening of those forms are practice, with the right form shown after a miss. Feedback on that kind of form work is supported (Moderate). Which feedback wording is best is not settled (Low). Timed drills of roots and verb types stay out, as ADR-0009 already decided. Hearing a known form is listening practice of a hard pattern. It is not a reason to open the next lesson early.

The scheduler stays the one already shipping. Anki-style cards are this queue. `SRS_SCHEDULER` is not switched to FSRS here.

---

## 4. Activity names

The builder and the MCP show the left column. The plan stores the right column.

| Name the author sees | Stored block | Player today |
|----------------------|--------------|--------------|
| Words | `introduction` | yes |
| Story | `graded-story` | yes |
| Listening | `listening` | yes |
| Dialogue | `short-dialogue` | yes |
| Fluency | `fluency-pass` | yes |
| Review | `spaced-review-checkpoint` | yes |
| Pattern | `conjugation-table` or the one pattern on Words | yes for the table |
| Match | `match` | no — SPEC-004 adds it |
| Type it | `type-it` | no — SPEC-004 adds it. The picture is not the cue |

---

## 5. Requirements

| Id | Requirement | Priority |
|----|-------------|----------|
| REQ-001 | An author can add, reorder, and remove lessons in one plan. Family 1, Family 2, Family 3 is three lessons in that order. | must |
| REQ-002 | Each lesson starts with no required activity. The author adds the ones that lesson needs. | must |
| REQ-003 | The activity menu uses the names in §4. MCP and the admin builder offer the same names and write the same block types. | must |
| REQ-004 | `buildsOn` on Family 2 and Family 3 can name Family 1. A recycled word records `fromLessonId`. | must |
| REQ-005 | `list_plan_issues` and the admin issues panel show the existing research warnings, plus a picture-on-Type-it warning and a closed-set word-list warning. None of them blocks publish. | must |
| REQ-006 | The learner can play Match and Type it inside a published plan. Type it asks for the Hebrew with the picture hidden. | must |
| REQ-007 | The MCP skill tells the AI to build a plan this way, and `/editor/` builds the same Family plan without a second document. | must |
| REQ-012 | The plan builder is served at `/editor/` and is not a tab of `/admin/`. `/admin/` keeps users and the worksheet lessons. Both pages are admin-only on the server. | must |
| REQ-008 | The three published worksheets still open in the lesson editor. | must |
| REQ-009 | A plan accepts one lesson and accepts 100 lessons. The schema's 12-lesson cap per unit is removed. `buildsOn` can name more than 12 earlier lessons. | must |
| REQ-010 | The plan shows the words introduced so far, in lesson order. A learner's place in the plan is the last lesson they finished. Practice draws only from lessons up to that place. | must |
| REQ-011 | Practice is the existing review queue: both directions, due on a later day, recall before the answer. Verb conjugation and verb listening are practice of forms already in that queue, untimed, with feedback. | must |

---

## 6. Specs

| Spec | Domain | Delivers | Effort | Blocked by |
|------|--------|----------|--------|------------|
| SPEC-001 | docs | ADR-0013. Records D1–D8, the Family table, the running word list, the learner's place, and practice | 2h | none |
| SPEC-002 | service | Add, reorder, and remove a lesson. No maximum lesson count. One activity-name map. The two new warnings. MCP tools and admin routes call the same functions | 5h | SPEC-001 |
| SPEC-003 | frontend | `/editor/` page: the stack, the activity menu, the running word list. `/admin/` keeps users and the worksheet lessons | 6h | SPEC-002 |
| SPEC-004 | frontend | Plan player runs Match and Type it. Type it hides the picture | 6h | SPEC-002 |
| SPEC-005 | docs | MCP skill and tool text walk through Family 1, 2, 3 with the activity names and `list_plan_issues` | 3h | SPEC-002 |
| SPEC-006 | service | The plan's running word list, the learner's place in it, and practice drawn from words and verb forms already reached | 6h | SPEC-002 |

SPEC-003, SPEC-004, and SPEC-006 wait on SPEC-002. SPEC-003 and SPEC-004 do not edit the same files. SPEC-005 waits on the tool names from SPEC-002 and describes the word list and the place from SPEC-006. Critical path: SPEC-001, SPEC-002, then SPEC-003, SPEC-004, and SPEC-006. SPEC-005 is last.

---

## 7. Out of scope

- Rebuilding the review scheduler, or pretending that finishing the plan today is the later-day recall.
- A Hebrew frequency list (`RSCH-004` rule 9).
- Rewriting `my-morning`, `shai-date-indirect-speech`, or `anal-sex-vocab`.
- Refusing a publish because a warning fired.
- A new dependency, a build step, or a change to `SRS_SCHEDULER`.

---

## 8. Threat model

No new account, secret, or public write. New MCP tools and admin routes stay on the existing admin and bearer-token checks. Lesson text is content. Picture bytes stay behind the approval ADR-0004 already requires. Security review on the service and player specs is `standard`.

---

## 9. Success

An author, in the admin plan builder or through MCP, can create a plan, add Family 1, Family 2, and Family 3, put Words only on Family 1, put Story and Type it on Family 2, put Pattern on Family 3, point Family 2 and Family 3 at Family 1, and publish. `list_plan_issues` warns if a Family 1 word never returns. The learner plays those activities. After Family 1, practice offers those words and, once Family 3 is reached, the verb forms from it. Family 4 stays locked until Family 3 is finished. The three existing worksheets still open from Lessons.
