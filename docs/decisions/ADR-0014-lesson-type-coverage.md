# ADR-0014: Lesson-type coverage (Exercise, Assignment, future tense)

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted (pending operator read of this record) |
| **Date** | 2026-10-15 |
| **Authored by** | `INIT-012/SPEC-001` |
| **Initiative** | INIT-012 · lesson-type-coverage |
| **Research grounding** | `RSCH-001` (no morphology-drill speed claim); `RSCH-003` (retrieval, feedback, deliberate study); `RSCH-004` (four strands, ~¼ deliberate study, lesson-type shape); `RSCH-005` (recall vs recognition); `RSCH-009` (AI pointing needs a person); operator decisions D2–D13, 2026-10-15 |
| **Extends** | [ADR-0003](ADR-0003-course-and-lesson-plan-language.md) (serialization §3, block list). [ADR-0009](ADR-0009-learner-model-and-language-program.md) (§7 plan language 1.1.0, rules N13–N19). [ADR-0013](ADR-0013-research-shaped-plans.md) (warnings do not block publish). |
| **Supersedes** | — |
| **Superseded by** | — |

## Summary

Four of the teacher's five lesson types already have a recipe. What they lack is the exercise and writing formats used inside them, plus future tense on the verb table. This ADR names those formats before any schema edit.

It adds two block types (`exercise`, `assignment`), one tense value (`future` on `conjugation-table`), schema version `1.2.0` with the same guard-and-auto-raise pattern as 1.1.0, validator rules N20–N23, and a lesson-type-to-block table the rest of INIT-012 implements. Match and Type it stay INIT-010's family for now. Exercise answers live in the published plan JSON and are not secret.

Example blocks live in [ADR-0014-assets/exercise-examples.json](ADR-0014-assets/exercise-examples.json).

---

## Context

### Problem statement

The teacher runs topic conversations, review worksheets, Bible-verse lessons, verb workshops, and short stories with true-or-false checks. The plan schema and player already cover introductions, stories, dialogues, verses, and present/past conjugation tables. They do not cover choose-the-sentence, true/false, unscramble, translate, dictation, a multi-sentence writing task with optional homework, or future-tense tables. INIT-012 adds those once so every type is buildable on `/editor/` and over MCP.

### Requirements this record satisfies

| Id | Requirement |
|----|-------------|
| R1 | Map each of the five lesson types to recipes, block order, and what is new in 1.2.0. |
| R2 | Fix the `exercise` and `assignment` block shapes, text limits, and item caps. |
| R3 | Fix future tense cell codes (same nine as past; imperative out). |
| R4 | Name N20–N23 and say how existing rules apply to the new constructs. |
| R5 | Record D2–D13 with research citations. |
| R6 | Record the 1.2.0 version decision (O3), the two exercise families (O1), and the answer-disclosure limit (D6). |

---

## 1. Lesson types to blocks

Teaching order is guidance for the author and the AI. The validator does not enforce order except through the named rules.

| Type | Recipe | Blocks, in teaching order | Block availability |
|------|--------|---------------------------|-------------------|
| A. Topic conversation | `topic-conversation` | `spaced-review-checkpoint`, `introduction` (three groups of five, sentence frames, fill-in), `short-dialogue`, `personal-question`, `assignment` | checkpoint, introduction, short-dialogue: schema today (1.0.0). personal-question: schema today (1.1.0). assignment: **new 1.2.0**. |
| B. Review worksheet | `review-worksheet` | `introduction` (with fill-in), `match`, `exercise` (`kind: choose`), `exercise` (`kind: unscramble`), `graded-story` (two readings), `personal-question` or `assignment` | introduction, graded-story: schema today. match: **INIT-010** (schema entry in INIT-010 SPEC-004; not in the shipping player until that spec lands). exercise: **new 1.2.0**. personal-question: 1.1.0. assignment: **new 1.2.0**. |
| C. Bible verse | `verse-reading` | `verse-reading`, `explainer` (grammar pivot), `exercise` (`kind: translate`) for numbers, `personal-question` | verse-reading, explainer, personal-question: schema today (1.1.0). The type was already buildable without translate; translate is **new 1.2.0** for number drills. |
| D. Verb workshop | `verb-workshop` | `explainer`, `conjugation-table` (`tense: present`), `conjugation-table` (`tense: past`), `conjugation-table` (`tense: future`), `conjugation-table` (`mode: drill`) | explainer, conjugation-table present/past: schema today (1.1.0). `tense: future`: **new 1.2.0**. |
| E. Short story and true or false | `review-worksheet` | `graded-story`, `exercise` (`kind: true-false`), `exercise` (`kind: choose`) | graded-story: schema today. exercise: **new 1.2.0**. (Same recipe value as B; D2.) |
| Any lesson | any | `listening`, then `exercise` (`kind: dictation`) | listening: schema today. exercise dictation: **new 1.2.0**. |

---

## 2. Decisions (D2–D13)

| Id | Decision | Rationale |
|----|----------|-----------|
| D2 | Do not add a recipe for the short story. It is a `review-worksheet` with a story and true/false (and choose) exercises. | One recipe value keeps the validator and editor simpler. The story shape is already `graded-story`. |
| D3 | One `exercise` block with `kind`, not one block per format. | One schema definition, one player module, one editor form (`RSCH-004` lesson types are a small set of activities, not dozens of block types). |
| D4 | One `assignment` block covers the in-class writing task and optional homework lines. | Matches the teacher's end-of-lesson prompt plus take-home lines; same privacy model as `personal-question`. |
| D5 | Future tense reuses the nine past person codes. Imperative stays out of `conjugation-table` and lexicon `CELLS`. | `cellCode` already lists `1s` … `3p`. Imperative is a different paradigm and was explicitly out of scope. |
| D6 | Exercise answers sit in the plan and are checked in the browser. They are not sent to the server and they are not secret. | Same honest limit as cloze answers today. This is a study worksheet, not a proctored exam. See §9. |
| D7 | New product rules warn unless the item cannot be played at all. Warnings never block publish (ADR-0013 D4). | N20–N22 **block** broken items. N23 **warns** only. |
| D8 | Content is original. Class docs inform structure only. Pointed forms come from the dictionary, not from class docs. | Class docs contain learner errors and unreliable pointing (`RSCH-009`, Moderate: AI pointing needs verification). |
| D9 | Number, date, and time drills use `translate` items the author writes. Generated drills are later work. | Avoids a second generator before the block exists. |
| D10 (O3) | `exercise`, `assignment`, and `tense: future` require `schemaVersion` `1.2.0`. A part operation that adds one raises the plan in the same revision. A plan never goes down. | ADR-0003 §3 and ADR-0009 Decision 7 precedent. See §7. |
| D11 (O1) | Two exercise families for now: `exercise` (this initiative) and `match` + `type-it` (INIT-010). They share one compare helper. | Recall vs recognition evidence conflicts (`RSCH-003` §1.1; `RSCH-005`). Folding is tidiness only after both ship. See §8. |
| D12 (O2) | No `transform` exercise kind. Person-form practice stays in the `conjugation-table` drill. | No published comparison shows morphology drills speed acquisition (`RSCH-001`, Low). Free-typed transforms need verified Hebrew (`RSCH-009`). Untimed drill with feedback is supported (`RSCH-003` §1.3, Moderate). |
| D13 (O4) | A person marks future lexicon forms `verified`, after a Hebrew-literate checker reads the list. No tool sets `verified`. | `RSCH-009`; N14 warns and does not score until then. |

---

## 3. Block: `exercise` (`type: "exercise"`)

### 3.1 Block-level fields

| Field | Type | Required | Limit |
|-------|------|----------|-------|
| `id` | `entryId` | yes | slug pattern `^[a-z0-9][a-z0-9-]*$`, max 32 |
| `type` | string | yes | `"exercise"` |
| `kind` | string | yes | `"choose"` \| `"true-false"` \| `"unscramble"` \| `"translate"` \| `"dictation"` |
| `heading` | `text120` | no | max 120 chars, no HTML |
| `items` | array | yes | 1–12 items; each item shape depends on `kind` |

Every item has an `id` (`entryId`, required). Unknown properties fail validation (ADR-0003 §3).

### 3.2 Item shapes by `kind`

#### `choose`

| Field | Type | Required | Limit |
|-------|------|----------|-------|
| `id` | `entryId` | yes | |
| `prompt` | `text200` | yes | |
| `choices` | string[] | yes | 2–5 entries, each `text120` |
| `answer` | string | yes | must equal one of `choices` (N20) |
| `why` | `text200` | no | shown after the learner answers |

**Example (pointed Hebrew, English gloss in `why`):**

- Prompt: `אֵיזו מִשְׁפָּט נָכוֹן? — "Which sentence is correct?"`
- Choices include `אֲנִי שׁוֹתֶה מַיִם.` (I drink water) and two distractors on a neutral topic.
- `answer`: the water sentence. `why`: one line in English.

#### `true-false`

| Field | Type | Required | Limit |
|-------|------|----------|-------|
| `id` | `entryId` | yes | |
| `statement` | `text200` | yes | pointed Hebrew or English |
| `answer` | boolean | yes | `true` or `false` |
| `why` | `text200` | no | |

**Example:** `statement`: `הַיָּם גָּדוֹל.` — "The sea is big." `answer`: `true`. `why`: `יָם` means sea.

#### `unscramble`

| Field | Type | Required | Limit |
|-------|------|----------|-------|
| `id` | `entryId` | yes | |
| `words` | string[] | yes | 3–10 tokens, each `text80`, correct order |
| `alt` | string[][] | no | 0–3 alternate orders; each order length 3–10; every order is a permutation of the same multiset as `words`, and no order is identical to `words` (N21) |
| `english` | `text120` | no | gloss for the target sentence |

**Example:** `english`: "The cat sleeps on the chair." `words`: [`הַחֲתוּלָה`, `יְשֵׁנָה`, `עַל`, `הַכִּסֵּא`].

#### `translate`

| Field | Type | Required | Limit |
|-------|------|----------|-------|
| `id` | `entryId` | yes | |
| `english` | `text120` | yes | cue |
| `answer` | string | yes | canonical Hebrew; compared after stripping U+0591–U+05C7 |
| `accept` | string[] | no | 0–8 alternate unpointed spellings; each 1–60 chars, at least one Hebrew letter (U+05D0–U+05EA), no characters in U+0591–U+05C7 |

**Example:** `english`: "book" · `answer`: `ספר` · `accept`: [`ספר`] (defective/plene variants as needed).

#### `dictation`

| Field | Type | Required | Limit |
|-------|------|----------|-------|
| `id` | `entryId` | yes | |
| `audioPath` | `audioPath` | yes | plan `audioPath` pattern; N22 |
| `answer` | string | yes | same scoring as `translate` |
| `accept` | string[] | no | same rules as `translate` |

**Example:** `audioPath`: `audio/sentences/vocab-1.mp3` · `answer`: `מַיִם` — "water" (word heard in the clip) · `accept`: [`מיים`] (common plene spelling; scoring strips vowel points like `translate`).

### 3.3 Player behavior (downstream)

Wrong answers get one retry, then the correct answer shows (matches Type it / conjugation drill). Typed kinds use vowel-point-insensitive compare and optional `accept[]` (`RSCH-003` §1.3, Moderate, for feedback).

---

## 4. Block: `assignment` (`type: "assignment"`)

| Field | Type | Required | Limit |
|-------|------|----------|-------|
| `id` | `entryId` | yes | |
| `type` | string | yes | `"assignment"` |
| `prompt` | `text300` or `plainText` | yes | task description |
| `minSentences` | integer | yes | 1–12 |
| `useWords` | string[] | no | 1–12 entries, each `text80` (words the learner should use) |
| `homework` | string[] | no | 1–6 lines, each `text200` |

**Rules:** Never scored. Stores no answer. Nothing the learner writes is sent to the server (same contract as `personal-question`).

**Example:** Prompt asks for six sentences about weekend plans; `useWords` lists three recycled lemmas; `homework` holds two take-home lines in English.

---

## 5. Future tense on `conjugation-table`

| Change | Value |
|--------|--------|
| `tense` enum | `["present", "past", "future"]` |
| `futureCell` | same nine codes as `pastCell`: `1s`, `2ms`, `2fs`, `3ms`, `3fs`, `1p`, `2mp`, `2fp`, `3p` |
| Imperative | not a `conjugation-table` tense; no `CELLS.imperative` entries |

When `tense` is `future`, each `cells[].cell` must be a `futureCell` code. N13 requires each `formKey` to name a lexicon form with that lemma, `tense: future`, and cell. N14 warns on unverified forms and the drill does not score them until a person sets `verified` (D13).

---

## 6. Validator rules (after N19)

| Id | Level | Trigger (one sentence) |
|----|-------|-------------------------|
| **N20** | **block** | In a `choose` item, `answer` is not equal to any string in that item's `choices`. |
| **N21** | **block** | In an `unscramble` item, `words` has fewer than 3 or more than 10 entries, or an `alt` order is not a permutation of the same multiset as `words`, or an `alt` order equals `words`. |
| **N22** | **block** | In a `dictation` item, `audioPath` is missing or does not match the plan `audioPath` pattern. |
| **N23** | **warn** | A lesson with `recipe: topic-conversation` contains more than two `exercise` blocks. |

**N23 product choice:** Topic lessons are mostly meaning-focused conversation and output (`RSCH-004` §1.2 four strands: about three-quarters meaning-focused work and about one-quarter deliberate study; the quarter is a planning rule, Low for the exact share). More than two deliberate `exercise` blocks in one topic lesson warns that exercises are crowding out conversation. N23 never blocks publish (D7).

### How existing rules apply

| Rule | Application to new constructs |
|------|----------------------------------|
| **N3** | Coverage (95% known running words) applies to `graded-story` and `listening` only. Exercise prompts, choices, and assignment text do not enter the N3 passage. |
| **N7** | `exercise` and `assignment` are not language-focused blocks. Only `introduction`, `explainer`, `conjugation-table`, and `letter-intro` count toward the language-focused share. |
| **N9** | UTF-8 size caps apply to whole plans, lessons, and blocks; a large `items` list or long `homework` array can block on byte size. |
| **N10** | Schema list caps bind: 12 blocks per lesson, 12 exercise items, 5 choices, 10 unscramble words, 3 `alt` orders, 8 `accept` strings, 12 `useWords`, 6 `homework` lines. |
| **N11** | Every authored string uses a typed limit (`plainText`, `text80`, `text120`, `text200`, `text300`) or the schema `audioPath` / `entryId` patterns. |
| **N13** | Each `formKey` in a `conjugation-table` with `tense: future` must name a stored future form for the table's `lemmaKey` and the row's cell. |
| **N14** | A future form listed in the table that is not `verified` warns; drill mode does not score that cell. |

---

## 7. Schema version 1.2.0 (O3 resolved)

ADR-0003 §3 (serialization):

> A later minor version may add optional fields only by publishing a schema that names that version. A `1.0.0` reader rejects fields it does not know.

ADR-0009 Decision 7 (schema, lines 400–407):

> `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` therefore takes `schemaVersion` `1.0.0` or `1.1.0`. A `1.0.0` document cannot contain a new block, a `recipe`, or a `group`. The validator rejects it, and does not guess. A part operation that adds a `1.1.0` construct to a `1.0.0` plan raises `schemaVersion` to `1.1.0` in the same revision. That is the only field an operation changes that it was not given. A plan never returns to `1.0.0`.

**1.2.0 adds:**

| Construct | Guard |
|-----------|--------|
| Blocks `exercise`, `assignment` | `v120BlockType`: a `1.0.0` or `1.1.0` document cannot contain these `type` values (same pattern as `v110BlockType` for the five 1.1.0 blocks). |
| `conjugation-table` with `tense: "future"` | `v120Only` (or equivalent field guard): future tense is invalid below 1.2.0. |
| Auto-raise | `usesV120` in `server/plans/ops.js` raises `schemaVersion` to `1.2.0` in the same revision when a part operation introduces any of the above. The plan never lowers. |

**Known gap (INIT-010, not fixed here):** `type-it` is already in `lesson-plan.schema.json` with no `v110BlockType` / version guard. INIT-010 SPEC-004 should guard `match` and `type-it` alongside the 1.1.0 blocks when those types ship.

---

## 8. Two exercise families (O1 resolved)

| Family | Blocks | Initiative |
|--------|--------|------------|
| Deliberate formats | `exercise` (`choose`, `true-false`, `unscramble`, `translate`, `dictation`) | INIT-012 |
| Recall / pairing | `match`, `type-it` | INIT-010 SPEC-004 |

**Shared compare:** `shared/ui/blocks/compare.js` holds vowel-point-insensitive Hebrew compare, plene/defective acceptance via `accept[]`, and final-letter normalization. Both families call it.

**Fold together later only if:** both families have shipped, operators still want one block type per interaction, and the maintenance cost of two registries outweighs a migration. The research does not require one family (`RSCH-003` §1.1; `RSCH-005`).

---

## 9. Answer disclosure (D6)

Exercise items store `answer`, `choices`, and boolean truth in the published plan JSON. The player checks answers in the browser. No exercise response is posted to the server.

A learner who opens devtools or reads the plan file can see answers, the same way cloze `answer` fields are visible today. That is acceptable for a homework worksheet. It is not a proctored exam.

---

## Consequences

### Positive

- Every INIT-012 spec can cite one ADR for block shapes, version guards, and rule ids.
- Topic and review lessons can hold the teacher's checks and writing tasks without JSON hacks.
- Future workshops use the same table and drill as past, with lexicon checks unchanged in spirit.

### Negative / trade-offs

- Two exercise families until INIT-010 and INIT-012 both ship.
- Future forms written by the AI stay unverified until a person checks them (D13).
- N23 is a soft guard; an author can ignore it and still publish.

### Follow-ups (other specs)

| Spec | Consumes this ADR |
|------|-------------------|
| INIT-012 SPEC-002 | Schema 1.2.0, `exerciseBlock`, N20–N23 |
| INIT-012 SPEC-003 | `assignmentBlock`, `tense: future`, lexicon cells |
| INIT-012 SPEC-004–006 | Player and compare helper |
| INIT-010 SPEC-004 | Guard `match` and `type-it`; reuse `compare.js` |

---

## Security considerations

No new route or secret. Attack surface is the same as today: admin/MCP writes plan JSON; learners read published plans. Authored strings must pass HTML and size rules (REQ-014 in the initiative). Answers in JSON are intentional disclosure (D6), not a vulnerability.

**Security review (this spec, light tier):** Critical 0 · High 0 · Medium 0 · Low 0 (documentation only; no executable change).

---

## References

- `sdd/initiatives/INIT-012-lesson-type-coverage/INIT-012-lesson-type-coverage-initiative.md` — D2–D13, lesson-type table
- `sdd/research/RSCH-001-english-to-hebrew-learning/RSCH-001-english-to-hebrew-learning-report.md`
- `sdd/research/RSCH-003-language-learning-science/RSCH-003-language-learning-science-report.md`
- `sdd/research/RSCH-004-course-design-and-lesson-types/RSCH-004-course-design-and-lesson-types-report.md`
- `sdd/research/RSCH-005-pictures-typing-spelling/RSCH-005-pictures-typing-spelling-report.md`
- `sdd/research/RSCH-009-worksheet-builder-design/RSCH-009-worksheet-builder-design-report.md` (AI pointing verification)
- [ADR-0003: Course and lesson-plan language](ADR-0003-course-and-lesson-plan-language.md)
- [ADR-0009: Learner model and language program](ADR-0009-learner-model-and-language-program.md)
- [ADR-0013: Research-shaped plans](ADR-0013-research-shaped-plans.md)
- `docs/decisions/ADR-0003-assets/lesson-plan.schema.json`
- `docs/decisions/ADR-0014-assets/exercise-examples.json`

## Changelog

| Date | Change |
|------|--------|
| 2026-10-15 | Initial record — `INIT-012/SPEC-001`. |
