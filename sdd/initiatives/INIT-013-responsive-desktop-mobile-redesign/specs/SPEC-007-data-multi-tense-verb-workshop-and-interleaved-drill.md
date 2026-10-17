# SPEC-007 · data: Schema 1.2.0 multi-binyan, multi-tense (`present`, `past`, `future`) verb workshop, 88-form lexicon seed, and verification checklist

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-013
title: "Schema 1.2.0 multi-binyan, multi-tense (present, past, future) verb workshop, 88-form lexicon seed, and verification checklist"
domain: data
status: complete
primary_prompt: .claude/agents/data-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: ["Operator content check before setVerified or publish"]
reversibility_class: branch
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-001]
blocks: [SPEC-006]
```

## Description

**Context:** The operator shared a multi-binyan verb conjugation worksheet (`1kl-0sfsiuVJ67sYKg59GOwD3nhZcHxGi7lRzlnM0VLE`) covering `לְהַחְלִיט` (to decide — `הִפְעִיל`, `ח.ל.ט`), `לְהַאֲמִין` (to believe — `הִפְעִיל`, `א.מ.נ`), `לְנַצֵּחַ` (to win — `פִּיעֵל`, `נ.צ.ח`), and `לִבְחוֹר` (to choose — `פָּעַל`, `ב.ח.ר`) and asked how to build this kind of verb practice in our app across **present, past, and future tenses** in the way our research corpus (`RSCH-001`, `RSCH-003`, `RSCH-004`, `RSCH-005`, `RSCH-009`) shows will help the learner acquire verbs fastest.

Meanwhile, the existing starter verb workshop (`scripts/seed-starter-content.js` from `INIT-006/SPEC-018`) was authored in `schemaVersion: "1.1.0"` before `INIT-012` (`ADR-0014`) added `tense: "future"`, `exercise`, and `assignment` in `schemaVersion: "1.2.0"`. It only holds `present` and `past` tables, a single-verb `present` drill, and no contextual sentence `exercise` or `assignment` blocks.

Following `INIT-013` §1b (`R1`–`R5`), this spec builds `scripts/seed-verb-tense-workshop.js`, an idempotent `schemaVersion: "1.2.0"` seed and human-verification checklist (`SPEC-007-forms-checklist.md`) without altering `scripts/seed-starter-content.js` (so `test/starter-content.test.js` stays 100% green).

**Scope (in):**

1. **88-Form + 4-Lemma Multi-Binyan Lexicon Seed (`scripts/seed-verb-tense-workshop.js`):**
   - Seeds the 4 core verbs across three binyanim:
     - `לְהַחְלִיט` (`to decide`, `pos: "verb"`, `binyan: "hifil"`, `root: "ח.ל.ט"`, `root_class: "regular"`, `accept: ["להחליט"]`)
     - `לְהַאֲמִין` (`to believe`, `pos: "verb"`, `binyan: "hifil"`, `root: "א.מ.נ"`, `root_class: "guttural"`, `accept: ["להאמין"]`)
     - `לְנַצֵּחַ` (`to win`, `pos: "verb"`, `binyan: "piel"`, `root: "נ.צ.ח"`, `root_class: "guttural"`, `accept: ["לנצח"]`)
     - `לִבְחוֹר` (`to choose`, `pos: "verb"`, `binyan: "paal"`, `root: "ב.ח.ר"`, `root_class: "guttural"`, `accept: ["לבחור", "לבחר"]`)
   - For each of the 4 verbs, seeds **22 inflected forms** (`4 present` [`ms`, `fs`, `mp`, `fp`] + `9 past` [`1s`–`3p`] + `9 future` [`1s`–`3p`] = **88 forms total**):
     - Uses standard Modern Hebrew dictionary pointing, explicitly correcting the class Google Doc's pointing typos (`הֶחְלַטְתִּי` instead of `הֶחְלַטֶתִי`, `בָּחַרְתִּי` instead of `בַּחָרתִי`, `מְנַצֵּחַ` instead of `מְנָּצֵחַ`).
     - Populates `accept[]` with unpointed and plene/defective spellings (`R2`).
     - Stores all rows with `verified = 0` (`source: "admin"`) until ticked on `SPEC-007-forms-checklist.md` (`R5`, `ADR-0014` D13, `RSCH-009`).
2. **Research-Shaped `schemaVersion: "1.2.0"` Verb Workshop Plan (`planId: "verb-tense-workshop"`, lesson `decide-choose-believe-win`):**
   - **Stage 1 — Short Explicit Pattern (`explainer`, ~¼ deliberate study, `R4`):**
     - 3-binyan comparison table (`פָּעַל`, `פִּיעֵל`, `הִפְעִיל`) showing the root (`שׁוֹרֶשׁ`), infinitive, present prefix (`מ-` in `פִּיעֵל`/`הִפְעִיל`), past stem vowel change (`הֶחְלַטְתִּי` vs. `הֶחְלִיט`), and future prefix (`א/ת/י/נ`), with `highlightRoot` examples and traffic-light callouts (`green` regular `הִפְעִיל`, `yellow` guttural/furtive patach in `לְנַצֵּחַ` and `לְהַאֲמִין`, `info` unverified-form notice).
   - **Stage 2 — Pointed Reference Tables (`conjugation-table`, `mode: "study"`, `present`, `past`, `future`, `R1`):**
     - Study tables for all 4 verbs across `present` (4 cells), `past` (9 cells), and `future` (9 cells), with original pointed example `sentences` and `questions` on the present tables.
   - **Stage 3 — Interleaved Multi-Verb, Multi-Tense Hidden-Stem Recall (`conjugation-table`, `mode: "drill"`, `R1`–`R3`):**
     - `drill` blocks across the 4 verbs and 3 tenses (`present`, `past`, `future`) so `drillQueue()` shuffles cells and interleaves across lemmas and tenses with hidden-stem recall, one retry before reveal, and `kind: "form"` signals for Daily Mix spacing.
   - **Stage 4 — Contextualized Sentence Completion (`exercise`, `kind: "choose"` & `kind: "translate"`, `R4`):**
     - Replaces the Google Doc's `אני _____(לְהַחְלִיט)_____` pronoun + infinitive cloze lines with original contextualized `choose` items (with `why` explanations of person/tense agreement) and `translate` items (English sentence cue -> typed Hebrew sentence with unpointed/plene tolerance and one retry).
   - **Stage 5 — Communicative Sentence & Question Production (`assignment` & `personal-question`, `R4`):**
     - An `assignment` block (`minSentences: 6`, `useWords: ["לְהַחְלִיט", "לִבְחוֹר", "לְהַאֲמִין", "לְנַצֵּחַ"]`, `homework` lines for writing present, past, and future sentences and questions) plus a closing `personal-question` block.
3. **Side-by-Side Verification Checklist (`SPEC-007-forms-checklist.md`) & Temp-DB Test Suite (`test/verb-tense-workshop.test.js`):**
   - `SPEC-007-forms-checklist.md` lists all 88 forms grouped by lemma and tense (`present`, `past`, `future`), unticked (`- [ ]`) by default. Ticking `[x]` and re-running `scripts/seed-verb-tense-workshop.js` calls `lexicon.setVerified` only for the ticked forms.

**Scope (out):** Modifying `scripts/seed-starter-content.js` (`INIT-006/SPEC-018` remains untouched); automatically setting `verified = 1` in any test or seed run; copying any sentence or personal name from the class Google Doc (`ADR-0014` D8).

## Understood

- [ ] und-1: `validatePlan(plan, { lexicon })` requires `schemaVersion: "1.2.0"` because the lesson uses `tense: "future"`, `exercise`, and `assignment` (`ADR-0014` D10).
- [ ] und-2: When `present` and `future` share an identical pointed string for `2ms` (`אַתָּה`) and `3fs` (`הִיא`) in `future` (e.g., `תַּחְלִיט`, `תִּבְחַר`, `תַּאֲמִין`, `תְּנַצֵּחַ`), `server/lexicon/validate.js` and `lexicon_entries` require each `key` in the lexicon to be unique. Check how `test/fixtures/lesson-types/lexicon-ledaber.json` or `server/lexicon/validate.js` handles `2ms` vs `3fs` in `future` (or distinct vocalization/keys) so `validatePlan` (`N13`) passes with zero blocking errors.
- [ ] und-3: No test and no default run may leave `verified = 1` in the committed checklist (`ADR-0014` D13, `RSCH-009`).

## Acceptance Criteria

- [ ] ac-1: `node --test test/verb-tense-workshop.test.js` passes on a temporary database: the `verb-tense-workshop` plan validates under `schemaVersion: "1.2.0"` with `validation.blocks.length === 0` (`N14` unverified warnings expected when unticked) (`REQ-010`, `REQ-011`).
- [ ] ac-2: `scripts/seed-verb-tense-workshop.js` seeds all 4 lemmas (`לְהַחְלִיט`, `לִבְחוֹר`, `לְהַאֲמִין`, `לְנַצֵּחַ`) and all `4 × 22 = 88` inflected forms across `present` (4), `past` (9), and `future` (9), each tagged with `tense`, `cell`, `binyan`, `root`, `root_class`, `accept[]`, and `verified = 0` (`REQ-010`).
- [ ] ac-3: `SPEC-007-forms-checklist.md` lists all 88 forms across `present`, `past`, and `future` unticked (`- [ ]`); a test confirms that ticking specific `present`, `past`, and `future` lines and re-running verifies only those forms (`verified = 1`) and unticking clears them (`REQ-010`, `REQ-011`).
- [ ] ac-4: The `decide-choose-believe-win` lesson contains all 5 research-recommended stages (`explainer` with 3-binyan table, `conjugation-table` `study` in `present`/`past`/`future`, `conjugation-table` `drill` across verbs and tenses, `exercise` `choose` & `translate`, and `assignment` + `personal-question`), and when forms are verified in a test, `drillQueue()` interleaves across verbs and tenses (`REQ-009`, `REQ-010`, `REQ-011`).
- [ ] ac-5: Neither `scripts/seed-verb-tense-workshop.js` nor `SPEC-007-forms-checklist.md` contains any personal names (`Brian`, `Nelson`, `Aaron`, `Shai`, `Shay`), copied class-doc sentences, or the class doc's pointing errors (`הֶחְלַטֶתִי`, `בַּחָרתִי`, `מְנָּצֵחַ`) (`REQ-011`).

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | Future `2ms` (`אַתָּה`) vs `3fs` (`הִיא`) homographs in Hebrew | In Hebrew, future `2ms` and `3fs` are homographs (`תַּחְלִיט`, `תִּבְחַר`, `תַּאֲמִין`, `תְּנַצֵּחַ`). If a single lexicon row has `cell: "2ms"`, check `server/plans/validate.js` (`N13`): `N13` checks `entry.tense === block.tense && entry.cell === cell.cell` or whether `2ms`/`3fs` need distinct keys or if `N13` checks `cell`. Match `server/plans/validate.js` and `server/lexicon/validate.js` rules so `validation.blocks` is empty | resolved | yes |

## Deliverables

- [ ] `scripts/seed-verb-tense-workshop.js`
- [ ] `sdd/initiatives/INIT-013-responsive-desktop-mobile-redesign/specs/completion-summaries/SPEC-007-forms-checklist.md`
- [ ] `test/verb-tense-workshop.test.js`

## Technical Requirements

- [ ] Follow the exact idempotent architecture of `scripts/seed-starter-content.js`: write through `lexicon.upsert`, `lexicon.setVerified`, and plan `store`/`ops`; refuse with `error: 'foreign_source'` if any target key exists with `source !== 'admin'`.
- [ ] Do not open or modify `data/hebrew.sqlite` during tests (`test/verb-tense-workshop.test.js` uses `fs.mkdtempSync`).

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** Operator content check before ticking `SPEC-007-forms-checklist.md` or publishing against the live database.
- **Blocking checks:** `node --test test/verb-tense-workshop.test.js test/starter-content.test.js`.
- **Rollback plan:** Delete `scripts/seed-verb-tense-workshop.js`, `test/verb-tense-workshop.test.js`, and `SPEC-007-forms-checklist.md`.

## Security

- **Sensitivity:** `light`.
- **Security acceptance criteria:**
  - [ ] All seeded content is original, contains no HTML markup, and contains no personal names or class transcripts.
  - [ ] Plans are seeded as `status: "draft"`, `visibility: "hidden"`, `snapshot: null`, and all forms default to `verified: 0`.

## Verification Strategy

- **Claim:** The multi-binyan, multi-tense (`present`, `past`, `future`) verb workshop and its 88-form lexicon paradigm validate under `schemaVersion: "1.2.0"` with zero blocking findings, support selective human verification via `SPEC-007-forms-checklist.md`, and interleave across verbs and tenses in `drillQueue()`.
- **Check + executor:** `mechanized` — `node --test test/verb-tense-workshop.test.js test/starter-content.test.js`.
- **Pass condition:** All tests pass (`exit 0`) with zero regressions.

## Integration Points

- Feeds into `SPEC-006`'s verification suite and pairs with `SPEC-005`'s multi-verb/multi-tense conjugation drill UI.
- **Provenance tag:** `INIT-013/SPEC-007`.
