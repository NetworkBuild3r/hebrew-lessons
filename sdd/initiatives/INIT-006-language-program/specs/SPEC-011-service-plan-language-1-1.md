# SPEC-011 · Service: Plan language 1.1 — five blocks, recipes, rules

## Metadata

```yaml
spec_id: SPEC-011
initiative_id: INIT-006
title: "Schema 1.1.0 with explainer, conjugation-table, personal-question, verse-reading, letter-intro; recipe rules"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: opus
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-005, INIT-004/SPEC-002, INIT-004/SPEC-003]
blocks: [SPEC-014, SPEC-018, SPEC-019]
```

---

## Description

**Context:** ADR-0003 defines six blocks and schema `1.0.0` (`docs/decisions/ADR-0003-assets/lesson-plan.schema.json`); INIT-004 SPEC-002 implements the validator with registry N1–N12 and SPEC-003 the operations. ADR-0009 §7 adds five blocks, a lesson `recipe`, and rules N13 onward. The teacher's formats (INIT-006 §2) are the recipes.

**Scope (in):**

- `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` → `schemaVersion` `1.0.0` or `1.1.0`. Every new construct is allowed only in a `1.1.0` document; a `1.0.0` document containing one is rejected, not guessed at (ADR-0009 §7, "Schema"). Block shapes and limits are ADR-0009 §7 "Fields". Every list item has an `id`. In short:
  - `explainer`: `title`, `sections[]` of `{ id, heading, paragraphs[], examples[] {id, he, en, translit?, root?}, table? {columns[], rows[][]}, callouts[] {id, tone: green|yellow|red|info, title, text} }`. Plain text only.
  - `conjugation-table`: `lemmaKey`, `tense` (`present` | `past`), `mode` (`study` | `drill`), `cells[]` of `{ cell, formKey }`, optional `sentences[]` (at most 3) `{id, he, en?}` and `questions[]` (at most 4) `{id, he, en?}`.
  - `personal-question`: `prompts[]` `{ id, he, en? }`, `answer` (`speak` | `type` | `either`). Never scored. The block has no field that holds an answer, and answers are not sent to the server.
  - `verse-reading`: `ref` `{ book, chapter, from, to, source }`, `newWords[]` (exactKey), `verse` (pointed text), `questions[]` `{id, he, en?}`, `personalQuestion` `{he, en?}`. No translation of the verse is stored.
  - `letter-intro`: `letter`, `name`, `sounds[]` `{ vowel, pointed, sound }`, `findIn[]` `{ id, word, contains: bool }`. **No picture field** (ADR-0009 §7, "Pictures": ADR-0003 §5 limits images to introduction words). `findIn` words create no review rows and are exempt from N1 and N6.
- Introduction words gain an optional `group` (1 to 3), `1.1.0` only.
- Lesson field `recipe` (`topic-conversation`, `review-worksheet`, `verse-reading`, `verb-workshop`, `letters-and-vowels`, `guide`), optional, **`1.1.0` documents only**.
- Validator rules (INIT-004's registry module, extended; ids and severities are ADR-0009 §7): N13 conjugation-table `formKey` names a stored form of `lemmaKey` with the table's `tense` and the entry's cell, each cell at most once (block); N14 unverified form in a table (warn); N15 review-first for a lesson that has a `recipe` other than `guide` and a non-empty `buildsOn` (warn); N16 a `topic-conversation`, `verb-workshop`, or `verse-reading` lesson without a `personal-question` / `personalQuestion` (inform); N17 three consecutive lessons with one recipe in a unit (inform); N18 `topic-conversation` more than 8 new words or a group over 5 (warn), more than 15 words (block); N19 `verse-reading` without a non-empty `ref.source` (block). Changes to ADR-0003 rules: N1 uses 15 and 8 for `topic-conversation` only; N7 counts `introduction`, `explainer`, `conjugation-table`, and `letter-intro` as language-focused and leaves `guide` and `letters-and-vowels` lessons out of the count; N2 does not apply to `guide`; N6 applies to `verse-reading.newWords`. The `topic-conversation` numbers (warn from 9 words or a group over 5, block above 15) were accepted by the operator with ADR-0009 on 2026-10-09 (SPEC-001 aud-1).
- Operations: no new operation. The six INIT-004 operations already address blocks by id; tests insert each new block through `insertBlock` and `replacePart`. One change to `ops.js`: when a part operation leaves a `1.0.0` plan holding a `1.1.0` construct, the same revision sets `schemaVersion` to `1.1.0`. It is the only field an operation changes that it was not given, and a plan never returns to `1.0.0`.

**Scope (out):** renderers (SPEC-014, SPEC-016); content (SPEC-018).

---

## Understood (agent's read of the intent)

- [x] und-1: A `1.0.0` plan never fails a rule introduced here. N13 to N19 reach only `1.1.0` constructs, and the N1, N2, and N7 changes give a `1.0.0` plan the same result as before.
- [x] und-2: The validator checks that a table cell points at a form; it cannot check that the Hebrew is right. That is `verified`.

## Acceptance Criteria

- [x] ac-1 (REQ-010): the four ADR-0003 sample plans validate unchanged under `1.1.0` rules with the same results as before.
- [x] ac-2 (REQ-010): one fixture per new block validates; one hostile fixture per block (markup, oversize, unknown property) fails with the rule id.
- [x] ac-3 (REQ-011): a conjugation table naming a form of another lemma blocks; naming an unverified form warns.
- [x] ac-4 (REQ-010): each inform/warn rule from ADR-0009 §7 has a positive and a negative fixture.
- [x] ac-5: `insertBlock` of each new type through INIT-004 `ops.js` succeeds and bumps the revision. On a `1.0.0` plan the same revision sets `schemaVersion` to `1.1.0`; on a plan already at `1.1.0` it changes nothing else.
- [x] ac-6: a `1.0.0` document that contains a new block, a `recipe`, or a `group` is rejected by the validator; a `letter-intro` with a picture field fails; a `guide` lesson and a `letters-and-vowels` lesson do not trigger N2 or N7. (Met under ADR-0009 §7, which exempts only `guide` from N2; see the completion summary.)
- [x] ac-7: a `topic-conversation` lesson with 9 new words warns, one with 16 new words blocks, and one with an introduction group of 6 words warns. A lesson of another recipe keeps the ADR-0003 N1 thresholds.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Bible text licensing | Use public-domain pointed Masoretic text; record the source edition in `ref.source`. No modern copyrighted translation in `en`. | resolved | yes |
| aud-2 | Exercise 1–7 review worksheet needs a new block | No. It is a recipe over existing blocks (`spaced-review-checkpoint`, `graded-story`, `short-dialogue`, `personal-question`). | resolved | yes — ADR-0003 §2 rejects new types without a research row |

> **Readiness** = 100 once INIT-004 SPEC-002 and SPEC-003 are complete.

---

## Deliverables

- [x] `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` (1.1.0)
- [x] Validator rule additions in INIT-004's `server/plans/validate.js` (or the registry module it names), and the `schemaVersion` raise in `server/plans/ops.js`
- [x] Fixtures under `test/fixtures/plans-1.1/`
- [x] Tests: `test/plan-language-1-1.test.js`

---

## Technical Requirements (domain-specific)

- [x] JSON Schema draft-07, `additionalProperties: false` on every new object.
- [x] Rule ids and severities in one table in code, matching ADR-0009 §7 one for one.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-language-1-1.test.js` and `npm test`.
- **Rollback plan:** revert the schema and rule additions; `1.0.0` documents are unaffected.

---

## Security

- **Sensitivity:** required. Triggers matched: untrusted plan text rendered for learners.
- **Security acceptance criteria:**
  - [x] Every new string field is length-limited and markup-free at validation.
  - [x] `personal-question` has no field that stores an answer.
- **Review depth:** standard · **focus areas:** schema closure, size limits.

---

## Verification Strategy

- **Claim:** schema 1.1.0 is additive, closed, and its rules match ADR-0009 §7.
- **Check + executor:** `mechanized` — `node --test test/plan-language-1-1.test.js`.
- **Pass condition:** exit 0; old samples unchanged results; every rule has fixtures.
- **Test levels:** unit (validator), integration (ops on a temporary database).

---

## Integration Points

- SPEC-014 and SPEC-016 render the five blocks. SPEC-018 writes them. SPEC-019 teaches them.
- Produces **Change:** plan language 1.1.
- **Provenance tag:** `INIT-006/SPEC-011`
