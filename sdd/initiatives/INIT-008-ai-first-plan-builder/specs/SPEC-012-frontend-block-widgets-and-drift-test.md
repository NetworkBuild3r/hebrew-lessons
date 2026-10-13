# SPEC-012 · Frontend: Block hints and widgets for every block type; drift test

## Metadata

```yaml
spec_id: SPEC-012
initiative_id: INIT-008
title: "Hints and widgets for all eleven block types and the lesson and plan fields; new-block form; a drift test so nothing is silently uneditable"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-011]
blocks: [SPEC-015, SPEC-016]
```

---

## Description

**Context:** SPEC-011's renderer builds a plain form for any node and a JSON fallback for the rest. This spec makes the forms good for each real block type, and adds the test that turns schema drift into a failing build. `RSCH-009` §3 found the hand-written builder covered six of eleven block types and none of the pattern, cloze, support, glosses, `newWords`, `buildsOn`, `recycles`, or `visibility` fields, and recorded that a test that fails when a schema block `type` has no registry entry or an explicit "generic" exemption turns drift into a failure (§1.3, product choice). The schema as read on 2026-10-12 has the block types `introduction`, `graded-story`, `listening`, `short-dialogue`, `fluency-pass`, `spaced-review-checkpoint`, `explainer`, `conjugation-table`, `personal-question`, `verse-reading`, `letter-intro`, a lesson with `recipe`, `visibility`, `buildsOn`, `newWords`, `recycles`, `legacyStorageKey`, and plan fields `title`, `subtitle`, `language`, `migration`. ADR-0011 Decision 8 is the contract.

**Scope (in):**

- `shared/admin/plan-hints.js` — per-schema-path hints: a bilingual label (Hebrew first, English under it), one line of help, the widget name, `lang` and `dir` for the control (`he`/`rtl` for Hebrew fields, `en`/`ltr` for English, Latin for transliteration), and ordering. The language of each field agrees with the server's `FIELD_LANGUAGE` registry (SPEC-008): a test compares the two so they cannot disagree. Hints cover:
  - **Plan:** `title`, `subtitle`, `language` (`target`, `gloss`); `migration` is shown as advanced and edited as JSON.
  - **Lesson:** `title`, `subtitle`, `footer`, `level`, `visibility` (a select: `hidden`, `selected`, `everyone`), `recipe` (a select of the six recipes plus none), `buildsOn` (a list of lesson ids), `newWords` (a list of word ids), `recycles` (a list of `{ word, fromLessonId }`), `legacyStorageKey` (advanced).
  - **Blocks:** `introduction` (heading, note, words, pattern with rules and items, cloze with bank and items), `graded-story` (notice, paragraphs with spans and highlights, support, glosses, questions), `listening` (passage, `audioPath`, questions), `short-dialogue` (source, count, intro, `introEn`, lines), `fluency-pass` (seconds, source, cues), `spaced-review-checkpoint` (word keys, from lessons), `explainer` (title, sections with paragraphs, examples, table, callouts), `conjugation-table` (lemma key, tense, mode, cells with a cell code and a form key, sentences, questions), `personal-question` (prompts, answer), `verse-reading` (reference with source, new words, verse, questions, personal question), `letter-intro` (letter, name, sounds, find-in).
  - **Items:** `wordEntry` (word, meaning, sound, translit, pos, trap, infinitive, sentence, `audioPath`, `sentenceAudioPath`, `fromLessonId`, image, group), `paragraph`, `line`, `question`, `patternItem`, `clozeItem`, `glossPair`, `hebrewLine`, `letterSound`, `letterFind`, `explainerExample`, `explainerCallout`.
  - Fields that hold a Hebrew pointed form use a widget with `dir="rtl"` `lang="he"` and a visible note of the character count against the schema limit. A plan `image` is shown as the media id and alt with a link to the existing picture panel; picture attach, upload, generate, and approve stay in `plan-builder.js` (SPEC-011), unchanged.
- **New-block form.** `newBlock(type, id)` is generated from the schema's `required` list for that block type: each required string is empty or the schema's minimal valid value, each required array is empty, with a new unique block id; no Hebrew is generated. It replaces the hand-written `newBlock` and covers all eleven types. Insert sends `POST /api/plans/:id/blocks`. Where the server refuses an empty required field at write time, the form says which rule (the server's code) and the admin fills it in.
- **Delete the hand-written lists.** `BLOCK_TYPES`, `BLOCK_FIELDS`, `ITEM_FIELDS`, and `FIELD_OK` in `plan-builder.js` are removed; nothing reads them. The block-type list comes from the schema.
- **The drift test.** `test/plan-editor-drift.test.js` loads the real schema and calls a pure checker `findUncoveredSchemaNodes(schema, { widgets, hints, exemptions })` exported from the editor files. It fails when: a block `type` const in the schema has no hint entry and no exemption; a property of any block definition or of `lesson` or the plan has no hint and no exemption; a hint names a path the schema does not have; the widget registry has a name no hint or default uses. An **exemption** is an explicit entry with a written reason (for example `migration` advanced-JSON, `legacyStorageKey`) and the count of exemptions is asserted so one cannot be added without a test change. The test also runs the checker against **a mutated copy of the schema** — one added block `type`, one added lesson property, one added property on `wordEntry` — and asserts it reports exactly those three, so the test cannot pass by being blind. The named `1.1.0` constructs (`explainer`, `conjugation-table`, `personal-question`, `verse-reading`, `letter-intro`), the lesson `recipe`, and the lesson `newWords`, `buildsOn`, `recycles`, and `visibility` are asserted by name to have hints.
- **Smoke additions** to `test/chrome-plan-editor-smoke.js`: open a lesson of a fixture plan that holds one block of each of the eleven types; assert each block's form renders with its hint labels and a `lang`/`dir` on every Hebrew control; edit one field in each of an `explainer`, a `conjugation-table`, a `personal-question`, a `verse-reading`, and a `letter-intro` block, reload, and see the saved values; set the lesson `recipe` and `visibility` and see them saved.

**Scope (out):** the plan list and create flow (SPEC-013); history (SPEC-014); issues and preview (SPEC-015); changing a schema definition or a validator rule.

---

## Understood (agent's read of the intent)

- [x] und-1: Hints are presentation only. The schema and the server stay the authority for what is valid.
- [x] und-2: The drift test is the guarantee. It is a failing test, not documentation, and it runs in `npm test`.
- [x] und-3: An exemption is a visible, counted, reasoned list. It is not a way to hide a gap.

## Acceptance Criteria

- [x] ac-1 (REQ-010): `node --test test/plan-editor-drift.test.js` passes on the real schema and fails on the three-defect mutated schema, reporting exactly the added block type, the added lesson property, and the added `wordEntry` property.
- [x] ac-2 (REQ-010, REQ-011): the test asserts by name that hints exist for the five `1.1.0` blocks, `recipe`, `newWords`, `buildsOn`, `recycles`, and `visibility`, and for each of the eleven block types; the exemption list is asserted to have the expected length and a non-empty reason on each entry.
- [x] ac-3 (REQ-011): `newBlock(type, id)` returns for each of the eleven types an object that passes the schema's `required` check for that block (validated with the same schema, in the test), has a unique id, and contains no Hebrew text. _(Met for the equired check: every required key is present, the id is unique, the type matches, and no Hebrew is generated. It is not a full schema validation: an empty required string breaks minLength, which is why the form asks for the required fields before it inserts.)_
- [x] ac-4 (REQ-011): a form renders for every one of the eleven block types, the lesson, the plan, and every item type listed above, with a widget or the JSON fallback for every property; every Hebrew-language control has `lang="he"` `dir="rtl"`, every English control `lang="en"` `dir="ltr"`; the language per field equals the server `FIELD_LANGUAGE` for the same schema path (a comparison test). _(Met. Besides the node forms, a test renders every schema definition that has properties as a draft form and counts one row and one registered widget per property; language attributes are checked on every text control of every block form, nested ones included. A field that mixes English with Hebrew words (mixed) gets dir="auto" and no lang, so the lang=en rule applies to English fields only.)_
- [x] ac-5 (REQ-011): the hand-written `BLOCK_TYPES`, `BLOCK_FIELDS`, `ITEM_FIELDS`, and `FIELD_OK` no longer appear in `shared/admin/` (a source search), and the existing outline actions (insert, move, remove, publish) still call their routes.
- [x] ac-6 (REQ-011): `node test/chrome-plan-editor-smoke.js` exits 0 for the added checks: all eleven block types render and keep an edited value after reload; the lesson `recipe` and `visibility` save; the five `1.1.0` blocks are editable. _(Met. The smoke edits a word, the explainer, conjugation-table, personal-question, verse-reading, and letter-intro blocks, reloads, and checks the form and the stored plan; recipe and visibility save; a new block is inserted through the new-block form.)_
- [x] ac-7 (REQ-017): contrast, target-size, and reduced-motion checks of `test/restyle-gate.test.js` and `test/chrome-restyle-a11y-smoke.js` still pass; each new control has an accessible name; labels are bilingual per ADR-0001 #6; no points, streak, or joke wording in any new string.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Which fields are exempt | `migration` and `legacyStorageKey` (advanced, JSON). The executor may add an exemption only with a reason, and the test counts them. | resolved | yes |
| aud-2 | Where hints live | One file, `shared/admin/plan-hints.js`, so a new field is one entry. | resolved | yes |
| aud-3 | Language data shared with the server | Not shared at runtime. The two registries are compared by a test, so the browser does not fetch the server's registry. | resolved | yes — no new route |

> **Readiness** = 100.

---

## Deliverables

- [x] `shared/admin/plan-hints.js`; changes to `shared/admin/plan-widgets.js`, `shared/admin/plan-form.js`, `shared/admin/plan-builder.js` (lists removed, new-block form)
- [x] Tests: `test/plan-editor-drift.test.js`, `test/plan-editor-widgets.test.js`; added checks in `test/chrome-plan-editor-smoke.js`

---

## Technical Requirements (domain-specific)

- [x] No dependency, no build step. IIFE and `var` style. Server text set with `textContent`.
- [x] Hebrew in test fixtures is hand-written; no teacher text, no learner text.
- [x] The checker is a pure function of the schema and the registries, importable by a Node test without a browser.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-editor-drift.test.js test/plan-editor-widgets.test.js`, `node test/chrome-plan-editor-smoke.js`, and `npm test`.
- **Rollback plan:** revert; SPEC-011's generic form still edits every node.

---

## Security

- **Sensitivity:** standard. Triggers matched: admin UI rendering and posting untrusted plan text.
- **Review depth:** standard · **focus areas:** escaped rendering in every new widget; no `innerHTML` with server data.

---

## Verification Strategy

- **Claim:** every block type and lesson field in the schema is editable with a purpose-built or fallback widget, and the build fails when a new one is added without coverage.
- **Check + executor:** `mechanized` — the commands above.
- **Pass condition:** exit 0; the mutated-schema case in ac-1 reports exactly the three planted gaps (so the test is not blind); the named-constructs assertions pass; the smoke reloads show saved values.
- **Test levels:** unit for the checker, `newBlock`, and the hints; Chrome smoke for rendering and persistence. A property-style test (every block type in the schema) earns its keep because the claim is "all block types".

---

## Integration Points

- SPEC-015 mounts the issues panel and preview beside these forms. SPEC-016 edits one block of every type through the editor and the MCP.
- Produces **Change:** hints, widgets, new-block form, drift test.
- **Provenance tag:** `INIT-008/SPEC-012`
