# ADR-0012: Custom lesson stack

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-14 |
| **Accepted** | 2026-10-14 by the operator, who approved INIT-009 (D1 to D4) the same day. |
| **Authored by** | `INIT-009/SPEC-001` |
| **Initiative** | INIT-009 · custom-lesson-stack |
| **Research grounding** | `RSCH-004` (§3 design rules, §4 candidate lesson types, §1.4 lesson structure); operator decisions D1 to D4, 2026-10-14 |
| **Supersedes** | — |
| **Extends** | [ADR-0003](ADR-0003-course-and-lesson-plan-language.md) 1 (entities and declarations), 2 (block vocabulary as optional parts of a lesson, not a fixed ladder). [ADR-0011](ADR-0011-ai-first-plan-builder.md) 8 (the editor presents the stack and block menu described here; no change to the revision or audit model). |
| **Superseded by** | — |
| **Corrected by** | [ADR-0013](ADR-0013-research-shaped-plans.md), Accepted 2026-10-14: the three published worksheets still open on the lesson editor; new courses are plans. Metadata only; the decisions in this file are left as written. |

## Summary

The lesson editor and the player still behave as if every lesson must carry vocabulary, an activity path, fill, grammar, reading, and ask-a-friend. When a worksheet has no `activities` array, the player invents that same six-part ladder at play time. That contradicts `RSCH-004`, which treats a unit as a mix of strands the instructor chooses (introduction, graded story, listening, short dialogue, fluency pass, spaced-review checkpoint, and the support types in §4), not one fixed drill sequence.

ADR-0003 already defines ordered lessons, optional blocks, `buildsOn`, and `recycles`. This ADR records the operator's choice to **author and play a lesson as only the blocks stored on it**, in array order, and to **present a plan as one ordered stack of lessons** whose order is the teaching order. Vocabulary-as-introduction, fill on the introduction, grammar pattern, reading, and ask-a-friend dialogue remain available block and field shapes from ADR-0003 and schema `1.1.0`; none of them is required on a new lesson. **An empty block list is not back-filled with the legacy six-part ladder** in the editor or in the player.

A new plan holds its lessons in **one unit**; lesson array order inside that unit is the teaching order. Words in a later lesson may declare `buildsOn` on earlier lessons in the stack and `recycles` with `fromLessonId`. The three published worksheets (`my-morning`, `shai-date-indirect-speech`, `anal-sex-vocab`) are not rewritten; their folder names stay. Downstream specs change editor and player behavior; this record adds no dependency, migration, secret, or change to `SRS_SCHEDULER`.

---

## Context

### Problem statement

On 2026-10-14 the operator asked for lessons that are "more custom": not forced to include vocabulary, an activity path, fill, grammar, reading, or ask-a-friend, and for a plan that is "a stack of lessons in order" where words build on each other. `RSCH-004` §1.4 says a unit mixes four strands and that deliberate study is the minority; §4 lists candidate lesson types an instructor picks for the theme, not one mandatory ladder. ADR-0003 encoded those types as blocks and kept worksheet sections such as fill and grammar on the introduction or as mapped blocks, but the **authoring surfaces and the player fallback** still assume the old worksheet skeleton.

### Current state (read in code on 2026-10-14)

- The lesson editor always renders a vocabulary table, an activity path, and JSON sections named fill, grammar, reading, and askFriend (`shared/admin/lesson-editor.js`).
- When a worksheet has no `activities` array, the player invents words, fill, grammar, reading, and ask-a-friend (`shared/player/core.js`, `fallbackActivities`, `resolveSteps`).
- A plan is already units of lessons with ordered `blocks`, `buildsOn`, `recycles`, and `newWords` (ADR-0003; `docs/decisions/ADR-0003-assets/lesson-plan.schema.json`).
- A new plan is created with one lesson and a single fluency-pass placeholder because a lesson must contain at least one block (`server/plans/create.js`).

### Requirements (must and should)

| Id | Requirement | Priority |
|----|-------------|----------|
| REQ-001 | A new lesson does not contain vocabulary, an activity path, fill, grammar, reading, or ask-a-friend unless the instructor adds that block or field. | must |
| REQ-002 | The player plays the blocks that are present, in order. It does not invent the six-part ladder when the list is empty or short. | must |
| REQ-003 | The instructor can add, remove, and reorder blocks. The allowed set includes the `RSCH-004` §4 types that already have a plan block, plus existing plan block types. | must |
| REQ-004 | A plan shows its lessons as one ordered stack. Reordering a lesson changes the teaching order. | must |
| REQ-005 | `buildsOn` on a lesson is chosen from lessons earlier in that stack, or from a lesson id in another plan. | must |
| REQ-006 | A recycled word records the earlier lesson it came from (`recycles[].fromLessonId`). | must |
| REQ-007 | A fluency block cannot introduce a `newWord`. A warning, not a forced block, notes when a new word has no recall in a later lesson. | should |
| REQ-008 | The three published worksheets (`my-morning`, `shai-date-indirect-speech`, `anal-sex-vocab`) are not rewritten and their folder names stay. | must |

### Constraints (standing)

No new npm dependency. No build step. No database migration. No secret and no learner data in a plan. `worksheets/*/index.html` and the three published `worksheet.js` files are not edited by this ADR. `SRS_SCHEDULER` is not changed. Validation rules N1–N19 and the labels in ADR-0003 Decision 6 stay as written; product-choice rules are not promoted to evidence-backed constraints (`RSCH-004` §3, §7 recommended next steps).

---

## Options considered

| Option | What it is | Outcome |
|--------|------------|---------|
| A. Keep the six-part skeleton as the default for every new lesson and empty play path | Editor opens with all sections; player back-fills missing activities | **Rejected** by D1 and D4. Fights `RSCH-004` §1.4 and §4. |
| B. Retire fill, grammar, reading, and ask-a-friend from the schema | Remove legacy shapes | **Rejected.** ADR-0003 maps them to introduction and dialogue blocks; legacy worksheets and migrated plans still use them. |
| C. **Custom blocks in author order; plan as ordered lesson stack; no empty-list ladder** | Lesson is stored blocks only; menu from `RSCH-004` §4 plus existing types; one unit holds the stack for new plans | **Chosen** (D1–D4). |

---

## Decision

### Chosen options (summary)

| # | Decision |
|---|----------|
| 1 | **D1 — Custom lesson composition.** Do not force vocabulary, an activity path, fill, grammar, reading, or ask-a-friend. The instructor adds the blocks the lesson needs, in the order they need them. |
| 2 | **D2 — Block menu.** The menu of blocks follows `RSCH-004` §4: introduction, graded story, listening, short dialogue, fluency pass, spaced-review checkpoint, script and sound support (fields on introduction words), and optional literal comprehension check. Existing plan block types from ADR-0003 and ADR-0009 stay in the allowed set. A block type the research does not require is still allowed; it is not inserted automatically. |
| 3 | **D3 — Ordered plan stack and word chaining.** A plan is an ordered stack of lessons. For a **new plan**, those lessons live in **one unit**; the order of lessons in that unit is the **teaching order**. Words in a later lesson may build on words from an earlier lesson via `buildsOn` (earlier lesson ids in the stack or ids named in another plan) and `recycles` (`fromLessonId`). A fluency block adds no new word (`RSCH-004` §3 rule 3). Recall of a new word on a later day is expected (`RSCH-004` §3 rules 5 and 6); the validator may warn; the editor does not auto-insert a checkpoint block (product choice, not a template). |
| 4 | **D4 — Legacy published lessons unchanged.** The three published worksheets are not rewritten and their folder names stay. New authoring does not start from the six-part skeleton. Migrated and legacy plans keep playing from their stored blocks and migration metadata. |
| 5 | **Empty list behavior.** **An empty activity list or an empty `blocks` array is not filled with the six-part ladder** (vocabulary, fill, grammar, reading, ask-a-friend, and the old activity path). The player runs only stored blocks. Downstream code implements that rule; this ADR states it. |
| 6 | **Carried constraints.** No migration, no new secret, no change to `SRS_SCHEDULER`. ADR-0011 operations, revision log, and editor generation from schema remain in force. |

### 1. Lesson is stored blocks (D1, REQ-001, REQ-002, REQ-003)

**Options considered:** (1) optional blocks only, author order — **chosen**; (2) require at least one introduction block — rejected, the operator may author a listening-only or checkpoint-only lesson; (3) keep player fallback — rejected by REQ-002.

A **lesson** is the ordered `blocks` array the author stored on that lesson, plus the declarations ADR-0003 requires (`newWords`, `buildsOn`, `recycles`, and the other lesson fields the schema allows). The legacy worksheet notion of a fixed "activity path" is not a required part of a plan lesson. Fill and grammar content that ADR-0003 places on `introduction` (cloze, pattern) appear only when the author adds or edits an introduction block with those fields.

The block menu is grounded in `RSCH-004` §4. Types already in the plan schema (`introduction`, `graded-story`, `listening`, `short-dialogue`, `fluency-pass`, `spaced-review-checkpoint`, and the ADR-0009 extensions) are the primary menu. Script and sound support remain fields on introduction words, not a separate required block (`RSCH-004` §4 row "Script and sound support"). Comprehension questions stay optional on graded story or listening (`RSCH-004` §3 rule 13, product choice).

### 2. Plan as ordered stack (D3, REQ-004, REQ-005, REQ-006)

**Options considered:** (1) one unit per new plan, lesson order equals teaching order — **chosen** (aud-1 in SPEC-001); (2) flatten units in the schema — rejected, ADR-0003 keeps units as the theme container; (3) auto-wire `buildsOn` on reorder — rejected, the instructor or AI sets declarations explicitly.

The **editor presents the plan's lessons as one ordered stack.** Reordering lessons in that stack changes the teaching order. The schema still has **units**; for authoring clarity, a **new plan** is created with **one unit** holding the full stack until the author adds another unit for a separate theme. `buildsOn` lists lesson ids whose vocabulary the lesson treats as already known; for lessons in the same plan, those ids are ordinarily **earlier in the stack**. `recycles` entries carry `fromLessonId` for words brought back from an earlier lesson (REQ-006), matching ADR-0003 Decision 1 and `RSCH-004` §3 rules 5 and 6 (recall on a later day and in a different sentence; spacing grade stays High via RSCH-003 as cited in `RSCH-004`).

REQ-007 aligns with ADR-0003 N12 and `RSCH-004` §3 rule 3: a fluency pass has no new words. A later-lesson recall warning follows existing warn rules; it does not force a spaced-review checkpoint block into every plan.

### 3. Published worksheets and new authoring (D4, REQ-008)

The files under `worksheets/my-morning/`, `worksheets/shai-date-indirect-speech/`, and `worksheets/anal-sex-vocab/` are **not rewritten** by this initiative. **Folder names stay.** Learners keep playing those lessons through the current worksheet or published snapshot path until a separate cutover spec says otherwise. **New plans and new lessons** do not open with vocabulary, fill, grammar, reading, and ask-a-friend pre-populated.

### 4. Relationship to ADR-0003 validation (`RSCH-004` §3)

ADR-0003 Decision 6 already encodes `RSCH-004` §3 rules with severities and labels (block, warn, inform; product choice vs evidence-backed). This ADR does **not** change those figures or promote product choices to evidence-backed constraints. It changes **authoring and playback expectations**: the instructor composes strands from §4; the validator still reports N1–N19 and the unnumbered warn rows when publish or issue listing runs.

---

## Consequences

### Positive

- Authoring matches `RSCH-004`: meaning-focused strands can dominate; deliberate study is added when needed, not as a fixed first screen.
- Plan order, `buildsOn`, and `recycles` express "words build on each other" without a separate activity-path concept.
- Legacy worksheets and migrated samples remain valid artifacts; INIT-009 code specs can remove fallback behavior without touching the three published folders.

### Negative / trade-offs

- An empty lesson or a lesson with only fluency is possible; the learner may see a very short session until the author adds blocks.
- The editor must stop implying required sections; that is SPEC-003 scope.
- Instructors must set `buildsOn` and `recycles` explicitly; the stack order alone does not write those arrays.

### Follow-ups (out of scope here)

SPEC-002: create, validate, and play stored blocks only; remove empty-activity fallback; keep published worksheets byte-for-byte. SPEC-003: stack UI, block menu, `buildsOn` / recycle picker; lesson editor without the six required sections. Optional: allow a truly empty new lesson when the schema minimum block count is relaxed in a later spec.

### Rollback

Remove or revert this ADR and the ADR-0003 metadata extension. Editor and player behavior reverts with the code specs that implemented them. No migration to roll back.

---

## Implementation plan (downstream)

| Spec | Consumes from this ADR |
|------|-------------------------|
| SPEC-002 | REQ-002, Decision 5 (no ladder fallback); REQ-008 (worksheets untouched) |
| SPEC-003 | D1–D3, REQ-001, REQ-003, REQ-004, REQ-005, REQ-006; block menu from §4 |

---

## Security considerations

No new trust boundary. Authoring stays admin-only and MCP-only on the server, as ADR-0011 requires. This ADR adds no secret, no learner field, and no change to who can publish. Lesson documents remain untrusted content governed by ADR-0003 HTML and size rules.

---

## References

- `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md` — §3 design rules, §4 candidate lesson types, §1.4 lesson structure
- [ADR-0003: Course and lesson-plan language](ADR-0003-course-and-lesson-plan-language.md)
- [ADR-0011: AI-first plan builder](ADR-0011-ai-first-plan-builder.md)
- `sdd/initiatives/INIT-009-custom-lesson-stack/INIT-009-custom-lesson-stack-initiative.md`

## Changelog

| Date | Change |
|------|--------|
| 2026-10-14 | Initial record and acceptance — `INIT-009/SPEC-001`. The operator approved INIT-009 (D1 to D4) the same day. |
| 2026-10-14 | ADR-0013 (`INIT-010/SPEC-001`, Accepted) corrects the reading that teaching content leaves the lesson editor: the three published worksheets still open there; new courses are plans. Metadata row and this row only. |
