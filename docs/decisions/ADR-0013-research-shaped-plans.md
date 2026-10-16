# ADR-0013: Research-shaped plans

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted |
| **Date** | 2026-10-14 |
| **Accepted** | 2026-10-14 by the operator, who approved INIT-010 (D1 to D8) the same day. |
| **Authored by** | `INIT-010/SPEC-001` |
| **Initiative** | INIT-010 · research-plan-authoring |
| **Research grounding** | `RSCH-003` (spacing and recall); `RSCH-004` (§3 design rules, §4 candidate lesson types, Family-shaped unit); `RSCH-005` (picture optional on first meeting; picture not the cue on Type it); operator decisions D1 to D8, 2026-10-14 |
| **Supersedes** | — |
| **Extends** | [ADR-0003](ADR-0003-course-and-lesson-plan-language.md) (plan as ordered lessons and blocks; research checks as warnings). [ADR-0011](ADR-0011-ai-first-plan-builder.md) (MCP and admin share one operations layer; warnings do not block publish). [ADR-0001](ADR-0001-learner-review-model.md) (the existing review scheduler is the later day for practice). |
| **Corrects** | [ADR-0012](ADR-0012-custom-lesson-stack.md)'s reading that teaching content leaves the lesson editor. The three published worksheets still open there. New courses are plans. |
| **Superseded by** | — |

## Summary

A lesson is only the activities the instructor adds. Words, story, match, spelling, reinforcement, and a pattern are allowed. None is required on every lesson. A plan is those lessons in order. Later lessons build on earlier words. Family 1 can have Words. Family 2 can leave Words out. Family 3 can add Pattern. The AI builds that plan through MCP. The admin plan builder builds the same plan through the same operations.

Research checks are warnings. They do not lock the instructor and they do not block publish. The named checks are later-context, N12, picture-on-recall, and closed-set. The three published worksheets (`my-morning`, `shai-date-indirect-speech`, `anal-sex-vocab`) stay on the lesson editor. New courses are plans.

A plan has no maximum lesson count. One is allowed. A hundred is allowed. The current schema cap of 12 lessons per unit is what a later spec removes. The plan keeps the running list of words its lessons introduce, in lesson order. A learner's place is the last lesson they finished. The next lesson is the next one in order. Practice uses only words from lessons up to that place. Verb practice is untimed conjugation and listening of forms already in that reached list. Recall, then a later day, both directions (Hebrew to English and English to Hebrew). Practice does not skip the learner ahead.

The later day is the existing review scheduler ([ADR-0001](ADR-0001-learner-review-model.md)), not the act of finishing Family 1, 2, and 3 in one sitting. `SRS_SCHEDULER` is not switched. A picture is optional on Words and is not the cue on Type it. Match and Type it are activities the plan player does not run yet; a later spec adds them.

This ADR corrects ADR-0012's reading that teaching content leaves the lesson editor. The three worksheets still open there. New courses are plans.

---

## Context

### Problem statement

ADR-0012 recorded a custom lesson stack and steered new authoring toward plans, but its follow-through read as if teaching content left the lesson editor entirely. On the evening of 2026-10-14 the operator fixed that reading and named how a research-shaped plan works: optional activities per lesson, any lesson count, a running word list, a learner place, practice on the existing schedule, and research checks that warn without locking or blocking publish. `RSCH-003`, `RSCH-004`, and `RSCH-005` supply the evidence grades behind those choices.

### Current state (read in code and decisions on 2026-10-14)

- A plan is units of ordered lessons with `buildsOn`, `recycles`, `newWords`, and ordered `blocks` (ADR-0003).
- MCP and the admin editor call the same plan operations. Warnings do not stop publish (ADR-0011).
- The plan player renders introduction, graded story, listening, short dialogue, fluency pass, and spaced-review checkpoint. Match and Type it are not among them yet.
- The validator already warns on later-context and N12. Picture-on-recall and closed-set are named here for later specs to surface.
- The three published worksheets remain on disk under their folder names. INIT-009 sent Admin → Lessons into the plan screen; this ADR restores the worksheets on that editor for those three only.

### Requirements (must)

| Id | Requirement |
|----|-------------|
| R1 | Record D1–D8 in plain sentences as the authoring and practice contract. |
| R2 | Show how Family 1, 2, and 3 map onto activities and research jobs. |
| R3 | Name the four research warnings and state that none locks or blocks publish. |
| R4 | Keep the three published worksheets on the lesson editor; new courses are plans. |
| R5 | State no maximum lesson count and that the schema's 12-lesson unit cap is removed later. |
| R6 | State the running word list, the learner's place, and practice of reached words and verb forms. |
| R7 | State that the later day is ADR-0001's scheduler, not one sitting of Family 1–3, and that `SRS_SCHEDULER` is not switched. |

### Constraints (standing)

No new npm dependency. No build step. No database migration in this record. No secret and no learner data in a plan. The three published `worksheet.js` files and their folder names are not rewritten here. `SRS_SCHEDULER` is not changed. Hard validation that already blocks a write or publish is unchanged; the research checks named here stay warnings.

---

## Options considered

| Option | What it is | Outcome |
|--------|------------|---------|
| A. Keep every lesson on the six-part worksheet skeleton | Editor always opens vocabulary, fill, grammar, reading, ask-a-friend | **Rejected** by D1 and D5. New courses are plans with chosen activities only. |
| B. Move all teaching off the lesson editor, including the three published worksheets | Lessons tab is plans only | **Rejected** by D5. This ADR corrects that reading of ADR-0012. |
| C. Block publish when a research check fires | later-context, N12, picture-on-recall, or closed-set refuse publish | **Rejected** by D4. Warnings do not lock the instructor. |
| D. Treat finishing Family 1–3 today as the spaced later day | One sitting replaces the review queue | **Rejected.** Spacing is the High finding (`RSCH-003`). The later day is ADR-0001. |
| E. **Research-shaped plan: optional activities, any lesson count, running word list, place, practice on the existing scheduler, warnings only** | D1–D8 as written | **Chosen.** |

---

## Decision

### Chosen options (summary)

| # | Decision |
|---|----------|
| 1 | **D1 — Lesson composition.** A lesson is only the activities the instructor adds. Words, story, match, spelling, reinforcement, and a pattern are allowed. None is required on every lesson. |
| 2 | **D2 — Plan order.** A plan is those lessons in order. Later lessons build on earlier words. Family 1 can have Words. Family 2 can leave Words out. Family 3 can add Pattern. |
| 3 | **D3 — One operations path.** The AI builds that plan through MCP. The admin plan builder builds the same plan through the same operations. |
| 4 | **D4 — Research checks are warnings.** They do not lock the instructor and they do not block publish. The named checks are later-context, N12, picture-on-recall, and closed-set. |
| 5 | **D5 — Worksheets stay; new courses are plans.** The three published worksheets (`my-morning`, `shai-date-indirect-speech`, `anal-sex-vocab`) stay on the lesson editor. New courses are plans. |
| 6 | **D6 — No maximum lesson count.** A plan has no maximum lesson count. One is allowed. A hundred is allowed. The current schema cap of 12 lessons per unit is what a later spec removes. |
| 7 | **D7 — Running word list and place.** The plan keeps the running list of words its lessons introduce, in lesson order. A learner's place is the last lesson they finished. The next lesson is the next one in order. Practice uses only words from lessons up to that place. |
| 8 | **D8 — Verb practice and directions.** Verb practice is untimed conjugation and listening of forms already in that reached list. Recall, then a later day, both directions (Hebrew to English and English to Hebrew). Practice does not skip the learner ahead. |
| 9 | **Later day and scheduler.** The later day is the existing review scheduler (ADR-0001), not the act of finishing Family 1, 2, and 3 in one sitting. `SRS_SCHEDULER` is not switched. |
| 10 | **Picture, Match, Type it.** A picture is optional on Words and is not the cue on Type it. Match and Type it are activities the plan player does not run yet; a later spec adds them. |
| 11 | **Correction of ADR-0012.** This ADR corrects ADR-0012's reading that teaching content leaves the lesson editor. The three worksheets still open there. New courses are plans. |

### 1. Lesson and plan shape (D1, D2, D6)

A lesson is only the activities the instructor adds. Words, story, match, spelling, reinforcement, and a pattern are allowed. None is required on every lesson. A plan is those lessons in order. Later lessons build on earlier words. Family 1 can have Words. Family 2 can leave Words out. Family 3 can add Pattern. A plan has no maximum lesson count. One is allowed. A hundred is allowed. The current schema cap of 12 lessons per unit is what a later spec removes.

### 2. How a Family plan maps onto the research

| Lesson | Activities | Research job | Already enforced |
|--------|------------|--------------|------------------|
| Family 1 | Words, optional picture, Match | First meeting. English meaning is the reliable gloss. A picture is optional (`RSCH-005`) | Introduction words require a meaning. Images only on those words |
| Family 2 | Story, Match, Type it, Review. No new word list | The words return in a new sentence. Type it is recall with the picture hidden (`RSCH-003`, `RSCH-005`) | later-context warns if a new word never returns. The plan player cannot play Match or Type it yet |
| Family 3 | Pattern (one), Story that uses it, Fluency with no new word | One new pattern, also in the story. Fluency adds nothing (`RSCH-004` rules 3 and 10, both product choices) | N12 warns when fluency sits on a lesson that introduces words. One-pattern is N5 |
| A later day | Practice list of words already reached, both directions | Spacing is the strongest finding (`RSCH-003`). Playing Family 1, 2, and 3 in one sitting does not deliver it | The per-word schedule exists (ADR-0001). A later service spec limits practice to words and verb forms from finished lessons |

A lesson whose new words are only kinship terms (mother, father, sister) gets a closed-set warning. A family theme that mixes people, places, and actions does not. The evidence for that warning is Low (`RSCH-004` rule 8). It stays a warning.

### 3. Authoring path (D3)

The AI builds that plan through MCP. The admin plan builder builds the same plan through the same operations. There is no second document and no separate write path for the Family shape.

### 4. Research checks (D4)

Research checks are warnings. They do not lock the instructor and they do not block publish. The checks are later-context, N12, picture-on-recall, and closed-set. Existing hard errors that already refuse a write or publish are unchanged.

### 5. Published worksheets and new courses (D5, correction of ADR-0012)

The three published worksheets (`my-morning`, `shai-date-indirect-speech`, `anal-sex-vocab`) stay on the lesson editor. New courses are plans. This ADR corrects ADR-0012's reading that teaching content leaves the lesson editor. The three worksheets still open there. New courses are plans.

### 6. Running word list, place, and practice (D7, D8)

The plan keeps the running list of words its lessons introduce, in lesson order. A learner's place is the last lesson they finished. The next lesson is the next one in order. Practice uses only words from lessons up to that place. Verb practice is untimed conjugation and listening of forms already in that reached list. Recall, then a later day, both directions (Hebrew to English and English to Hebrew). Practice does not skip the learner ahead.

The later day is the existing review scheduler (ADR-0001), not the act of finishing Family 1, 2, and 3 in one sitting. `SRS_SCHEDULER` is not switched. A picture is optional on Words and is not the cue on Type it. Match and Type it are activities the plan player does not run yet; a later spec adds them.

### 7. Activity names the author sees

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
| Match | `match` | no — a later spec adds it |
| Type it | `type-it` | no — a later spec adds it. The picture is not the cue |

---

## Consequences

### Positive

- An author can rebuild a Family plan from this record alone: optional Words on Family 1, no new word list on Family 2, Pattern on Family 3, warnings that never refuse publish.
- Practice stays on the shipping scheduler; finishing three lessons today does not pretend to be the later day.
- The three published worksheets remain reachable from the lesson editor while new courses use plans.

### Negative / trade-offs

- Match and Type it exist in the authoring contract before the plan player can run them.
- The schema still caps lessons per unit at 12 until a later service spec removes that cap.
- Instructors can ignore research warnings; publish still goes through.

### Follow-ups (out of scope here)

Service: add, reorder, and remove lessons; remove the 12-lesson unit cap; surface picture-on-recall and closed-set; running word list and learner place for practice. Frontend: stack editor with activity menu; Lessons tab opens the three worksheets again; plan player runs Match and Type it. Docs: MCP skill walkthrough of Family 1, 2, and 3.

### Rollback

Remove or revert this ADR and the metadata lines on ADR-0012 and ADR-0003. No migration to roll back. Downstream code specs revert with their own changes.

---

## Implementation plan (downstream)

| Spec | Consumes from this ADR |
|------|-------------------------|
| SPEC-002 | D1–D4, D6; activity-name map; later-context, N12, picture-on-recall, closed-set as warnings; remove 12-lesson unit cap |
| SPEC-003 | D1–D3, D5, D7; stack, activity menu, running word list; three worksheets on the lesson editor |
| SPEC-004 | Match and Type it in the plan player; picture not the cue on Type it |
| SPEC-005 | MCP skill text for Family 1–3 and `list_plan_issues` |
| SPEC-006 | D7–D8; running word list, learner place, practice of reached words and verb forms on ADR-0001 |

---

## Security considerations

No new trust boundary. Authoring stays admin-only and MCP-only on the server, as ADR-0011 requires. This ADR adds no secret, no learner field, and no change to who can publish. Lesson documents remain untrusted content governed by ADR-0003 HTML and size rules. Practice continues to use the existing review store; no new schedule identity is introduced.

---

## References

- `sdd/research/RSCH-003-spaced-retrieval-and-flashcards/RSCH-003-spaced-retrieval-and-flashcards-report.md` — spacing and recall
- `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md` — §3 design rules, §4 candidate lesson types
- `sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md` — optional picture; Type it without picture cue
- [ADR-0001: Learner review model](ADR-0001-learner-review-model.md)
- [ADR-0003: Course and lesson-plan language](ADR-0003-course-and-lesson-plan-language.md)
- [ADR-0011: AI-first plan builder](ADR-0011-ai-first-plan-builder.md)
- [ADR-0012: Custom lesson stack](ADR-0012-custom-lesson-stack.md)
- `sdd/initiatives/INIT-010-research-plan-authoring/INIT-010-research-plan-authoring-initiative.md`

## Changelog

| Date | Change |
|------|--------|
| 2026-10-14 | Initial record and acceptance — `INIT-010/SPEC-001`. The operator approved INIT-010 (D1 to D8) the same day. Corrects ADR-0012's reading that teaching content leaves the lesson editor. |
