# Initiative: A teacher can build the plan the Figma shows

**Initiative ID:** INIT-011
**Status:** on_track. SPEC-001 through SPEC-005 are `complete`. SPEC-006 is still `ready`. It is a signed-in browser pass and is not done. The initiative is not ready to close until that pass is recorded.
**Slug:** INIT-011-teacher-plan-builder
**Discipline Profile:** `software`
**Created:** 2026-10-15. Revised the same day after a check against the teacher's Drive folder (section 0b).
**Owner:** Brian Nelson
**Ticket:** None (`branch_naming.ticket_system: none`)
**Branch:** `teacher-plan-builder` (not created). Orchestration on 2026-10-15: no worktree holds that branch. The operator chose to continue in this checkout, because INIT-011 and the unfinished editor are uncommitted here. A worktree was not created. No commit unless the operator asks.
**Target Completion:** not a promise. The spec table is an estimate that starts when the operator says to execute.
**Source:** operator, 2026-10-15. The Figma screens `19:2304` (Family plan builder) and `19:2592` (Family 2 lesson builder) in file `NJ8HfTPCQgrFty2VVQBMhY` are the look. MCP stays the way a plan is written. A teacher who is not a programmer has to be able to build the same plan on `/editor/`. The teacher's real material is the Drive folder `1eKpZisxfS6DwdNf5jcOgEJY6JRqeu71_`. Research: `RSCH-004`. Builds on ADR-0003, ADR-0011, and INIT-010.

---

## 0. What is already true

| Fact | Where |
|------|--------|
| MCP and `/editor/` are supposed to call the same plan operations | ADR-0011, INIT-010 |
| Adding a lesson is `POST /api/plans/:id/lessons`. The node test adds two lessons and sees three rows | `server/plans/ops.js` `insertLesson`, `test/plan-editor-research.test.js` |
| On the live page, Add lesson can fail with no sentence the teacher can read. The list does not change | `shared/admin/plan-builder.js` `addLesson`, `write` |
| English mode still shows mixed strings such as `4 תוכניות (4 plans)` and `מעבר לשדה (Go to the field)` | `shared/admin/plan-list.js`, `shared/admin/plan-issues.js` |
| The open plan is a schema form: Id, Activity, Index, Up, Down, Remove, and a tall empty preview | `/editor/` today |
| New plan asks for a title and a recipe slug (`topic-conversation`, `verb-workshop`, …). It makes `lesson-1` with an empty Fluency activity named `placeholder` that the teacher did not add | `shared/admin/plan-list.js`, `server/plans/create.js` |
| Add lesson names the lesson `lesson-N` and gives it the same empty Fluency activity | `newLessonShell` in `shared/admin/plan-builder.js` |
| **Match is in the activity menu, but the schema has no `match` block.** Every Match insert is refused | `shared/admin/activity-map.js`, `lesson-plan.schema.json` `block.oneOf` |
| **The uncommitted Type it change dropped `verseReadingBlock` from `block.oneOf`.** A verse lesson no longer validates | `git diff` on `lesson-plan.schema.json` |
| The schema has Writing (`personal-question`), Notes (`explainer`), and Verse (`verse-reading`) blocks, but the activity menu does not list them | `activity-map.js` |
| INIT-010 SPEC-004 (the player runs Match and Type it) is still `ready`, not done | INIT-010 |

## 0b. What the teacher's Drive folder shows

The folder is two years of real lessons: about 75 dated lesson folders (`2025/`, `2026/`), nine numbered worksheets (`Worksheets/01`–`09`), a 7-day drill sheet, a weekly training program, and a 59-card production deck (CSV).

| What the teacher makes | Example | What it needs in the builder |
|------------------------|---------|-------------------------------|
| A plan is a dated run of lessons for one learner | `2026/01.14 … 10.06`, "Brian — Hebrew Personal Training Program" | A plan is for a person or for everyone. That choice is on the plan, in plain words |
| A lesson is a topic with a title in Hebrew or English | `04.15 Sharing Opinions`, `03.25 המשפחה שלי` | Add lesson asks for the title. It never shows `lesson-3` |
| A topic comes back weeks later | `04.01 retaining: Living room`, `04.29 retaining kitchen`, "Intro to opinions" on 04.28 and 05.05 | Reuse a lesson from this or another plan, and mark it as building on the first |
| Word bank, then complete the sentences | `02 אוהב לא אוהב` Ex. 1, `03 המשפחה שלי` Ex. 1 | Words with a fill-in-the-blank (the `cloze` already in `introduction`) |
| Match column A to column B | `03` Ex. 2 | Match |
| Choose the correct sentence | `02` Ex. 2 | **Nothing in the schema holds this.** Open decision O1 |
| A short reading and questions | `02` Ex. 3–4, `03` Ex. 3 | Story with questions |
| Write about yourself / discussion prompts | `02` Ex. 5, `03` Ex. 5, `Sharing Opinions` | Writing (`personal-question`) |
| A verse with glosses | `10.06 ישעיהו מ 1-8`, the Genesis series | Verse (`verse-reading`) |
| Verb rows to chant | Drill sheet Day 2, `Decoding Hebrew Verbs` | Pattern (`conjugation-table`) |
| English cue, say the Hebrew | Drill sheet Days 1, 3, 4, 6; the CSV deck | Type it, and Fluency for the timed round |
| A teacher note (`פעל - נפעל`, "ban English *have*") | `Sharing Opinions`, drill sheet rules | Notes (`explainer`) |
| A word list the teacher already has | The CSV deck, the `אוצר מילים` tables | Paste a list of Hebrew–English lines into the word bank |

The teacher writes these in Google Docs today. The MCP-first path is: the AI reads a worksheet and builds the lesson with the plan operations, then the teacher opens it on `/editor/` and fixes what is wrong. The builder has to make that second step easy, and it has to let the teacher do the whole lesson by hand when there is no AI in the loop.

---

## 1. Executive summary

The AI still builds a plan through MCP, with the operations that already exist. The teacher page is a second way to call those same operations. It is not a second kind of plan.

One workflow, start to end, with no id, path, index, or recipe slug on screen:

1. **New plan.** Title, and who it is for: one learner, or everyone.
2. **The plan** (Figma `19:2304`). Lessons in order, one wide row each. Add lesson asks for a title and one sentence. Reuse lesson copies one from this or another plan. Move and Open lesson sit on the row. The word bank on the side lists each word and the lesson that added it. Words can be pasted as a list.
3. **The lesson** (Figma `19:2592`). Tabs for the lessons in this plan. Word bank on the left, activities in the middle, the learner preview on the right. The activity menu covers what the worksheets already do: Words (with fill-in-the-blank), Match, Story (with questions), Writing, Verse, Pattern, Type it, Fluency, Review, Notes. An activity opens with the fields to fill. It is saved when it is valid.
4. **Publish.** Warnings do not stop it.

English shows English. Hebrew shows Hebrew. A Hebrew lesson title stays Hebrew in both, because it is the lesson, not the chrome.

A phone stacks the columns. A wide screen uses the Figma split.

---

## 2. Operator decisions (2026-10-15)

| Id | Decision |
|----|----------|
| D1 | MCP stays first. The teacher page calls the same operations. No second write path and no second document. |
| D2 | Two screens. The plan screen is Figma `19:2304`. The lesson screen is Figma `19:2592`. The schema form is not the teacher view. |
| D3 | One interface language. A label is never `Hebrew (English)` as one string. |
| D4 | A teacher does not type a path, a block id, an index, a recipe slug, or JSON. Ids are made from the title. |
| D5 | Add lesson either shows the new row or a plain sentence that says why it did not. Silence is a bug. |
| D6 | A phone stacks. A computer uses the column split in the two frames. |
| D7 | Warnings still do not block publish. INIT-010 D4 stands. |
| D8 | Every activity name in the menu is a block the schema accepts. A test enforces it. |
| D9 | The empty placeholder activity is never shown as a teacher's activity. It is removed by the same remove operation when the first real activity is saved. |
| D10 | Reusing a lesson is `insert_lesson` with a copy. There is no new copy operation, so MCP can do the same thing. |
| D11 | The acceptance test is real material: worksheets `02 אוהב לא אוהב` and `03 המשפחה שלי` from the Drive folder, rebuilt by hand and by the AI. |

## 2b. Open decisions

| Id | Question | Default if not answered |
|----|----------|-------------------------|
| O1 | "Choose the correct sentence" (worksheet `02` Ex. 2) has no block. Add a choice block to the schema, or teach it as a Story question? | Out of INIT-011. Story question until a later ADR-0003 change |
| O2 | The weekly program (teacher days and solo days) is a schedule, not a lesson. Should a plan hold it? | Out of INIT-011 |

---

## 3. Requirements

| Id | Requirement | Priority |
|----|-------------|----------|
| REQ-001 | Every teacher action that changes a plan is one of the existing plan operations. MCP can do the same change. | must |
| REQ-002 | With English selected, chrome is English only. With Hebrew selected, chrome is Hebrew only. Lesson titles and word text stay as stored. | must |
| REQ-003 | The plan screen matches `19:2304`: ordered lesson rows, the open row marked, move, open, add, reuse, and a word bank beside the list. | must |
| REQ-004 | The lesson screen matches `19:2592`: lesson tabs, a word bank, activity rows the teacher can reorder, one open activity, and a learner preview. | must |
| REQ-005 | Add lesson inserts a lesson the teacher can see, or the page says why it did not. | must |
| REQ-006 | Below 900px the columns stack. At 900px and wider they sit side by side as in the frames. | must |
| REQ-007 | The search field is a normal field. The placeholder is readable. | must |
| REQ-008 | Publish still publishes. A warning still does not stop it. | must |
| REQ-009 | New plan asks for a title and who the plan is for. It does not ask for a recipe slug. | must |
| REQ-010 | Every activity in the menu can be added and saved. Verse lessons validate again. | must |
| REQ-011 | The activity menu covers the worksheet exercises in section 0b, except O1. | must |
| REQ-012 | A teacher can paste a list of Hebrew–English lines into the word bank. | should |
| REQ-013 | A teacher can reuse a lesson from this plan or another plan. | should |
| REQ-014 | After the AI builds a lesson, the teacher can open it, change a sentence, and publish, without leaving the two screens. | must |

---

## 4. Specs

| Spec | Domain | What it does | Hours | Blocked by |
|------|--------|----------------|-------|------------|
| SPEC-001 | docs | The teacher workflow, the two screens, the worksheet-to-activity table, and the MCP recipe for turning a worksheet into a lesson | 3 | — |
| SPEC-002 | data | Put Verse back in the schema. Every menu activity is a block the schema accepts. A test that keeps it that way | 3 | SPEC-001, INIT-010 SPEC-004 for the `match` block |
| SPEC-003 | frontend | One language, a readable search field, and a visible reason when a write fails | 3 | SPEC-001 |
| SPEC-004 | frontend | New plan, the plan screen (`19:2304`), Add lesson with a title, reuse a lesson, the word bank, paste a word list | 8 | SPEC-002, SPEC-003 |
| SPEC-005 | frontend | The lesson screen (`19:2592`), the full activity menu, activities that open with fields to fill, the placeholder removed | 8 | SPEC-002, SPEC-003 |
| SPEC-006 | testing | Rebuild worksheets `02` and `03` as a two-lesson plan for one learner, by hand and by the AI, in English, at a phone width and a wide width | 4 | SPEC-004, SPEC-005 |

SPEC-004 and SPEC-005 can be built in either order after SPEC-002 and SPEC-003. SPEC-006 waits for both.

**Outside dependency:** the `match` block comes from INIT-010 SPEC-004. If that spec is not done when SPEC-002 starts, SPEC-002 hides Match from the menu instead of adding the block, and the test in D8 still passes.

**Expertise check:** `scripts/dev/expertise_state.py` is not in this repo, so it was not run. The prompts are the ones INIT-010 already uses: `docs-agent`, `frontend-agent`, and `data-agent` for the schema. Nothing new was generated.

---

## 5. What this does not do

- It does not replace MCP, and it does not add a tool the page cannot also call.
- It does not import Google Docs from the page. The AI reads the worksheet over MCP; the page edits the result.
- It does not change the review scheduler.
- It does not move the three published worksheets.
- It does not invent a drag-and-drop that the operations cannot save. Reorder stays the move operation.
- It does not add a choice block (O1) or a weekly schedule (O2).
- It does not restyle Today, Lessons, or Sign in.

---

## 6. Threat model

The page stays admin-only, the same gate as `/editor/` today. A teacher session is an admin session. Writes still send `expectedRevision`. Reuse reads another plan through the admin parts route that already exists. Pasted word lists are text and go through `textContent`. No new secret, no new public route, no learner access to the builder.

---

## 7. Risks

| Risk | What we do |
|------|------------|
| The Figma frames are examples (Family 1–5, mother, father). Copying that text into the app would lie about the open plan. | The layout and the color come from the frames. The words on the rows come from the plan that is open. |
| An activity is refused because its shell is empty (Words needs one word, Story one paragraph). | SPEC-005 opens the activity as a form and saves it once it is valid, the same staged draft the builder already uses. |
| A reused lesson carries a `buildsOn` or a word that does not exist in the new plan. | SPEC-004 drops `buildsOn` ids that are not earlier in the new plan, and gives the copy a new id if the old one is taken. |
| A partial CSS restyle is already in the tree and fights the new layout. | SPEC-004 and SPEC-005 replace that layout. They do not layer another skin on the schema form. |
| The schema regression (Verse) ships with the Type it change. | SPEC-002 is first in line and its test fails on any `*Block` definition missing from `block.oneOf`. |
