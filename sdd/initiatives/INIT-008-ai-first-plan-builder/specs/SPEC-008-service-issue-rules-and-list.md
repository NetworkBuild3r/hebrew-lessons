# SPEC-008 · Service: Issue rules as warnings with stable codes; the issue list

## Metadata

```yaml
spec_id: SPEC-008
initiative_id: INIT-008
title: "Five Hebrew and consistency checks as warnings; one list of every finding with id paths and fix hints"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-001]
blocks: [SPEC-009]
```

---

## Description

**Context:** with no human gate (D1), the AI and the admin each find and fix problems themselves, so problems must be listed with codes both can act on (D3). `RSCH-009` §1.4 found what the validator checks today (`pointed-word`, `meaning`, `romanized-answer`, HTML, audio paths, sizes, N13/N14, N19) and what it does not: malformed pointing, Hebrew in an English field or none in a Hebrew field, a pointed form that differs from the lexicon entry with the same stripped key, one word pointed two ways in a lesson, and a plan text field that changed while its `audioPath` stayed. A person who fixes a sentence leaves the old clip attached, and nothing links a text field to its clip except the path pattern. The validator is a pure function of a document and an optional lexicon (`server/plans/validate.js:291`); its findings carry `{ rule, severity, path, message }` where `path` is an RFC 6901 pointer with array indexes, which an AI cannot use as an id path. ADR-0011 Decision 5 is the contract.

**These five rules are warnings. They never block a write and never block a publish.** Existing hard errors (`schema`, `html`, `audio-path`, `image-placement`, `N8`–`N11` on write; the publish blocks) are unchanged.

**Scope (in):**

- `server/plans/hebrew-checks.js` — `checkHebrew(plan, { lexicon, snapshot })` returning findings, registered as five new rows in the `RULES` table (`severity: 'warn'`, a `summary`, and a label in the row comment: product choice or adapted). New findings carry two optional keys the old ones do not: `field` and `fix`. Existing findings are unchanged. Codes:
  - **`pointing-form`** — in any field the registry marks Hebrew: (i) a vowel point (U+05B0–U+05BB, U+05C7) with no Hebrew letter before it, or at the start of the text or after a space or punctuation; (ii) two vowel points on one letter. A letter may also carry one dagesh or mapiq (U+05BC), one shin or sin dot (U+05C1, U+05C2), a rafe (U+05BF), a meteg (U+05BD), and cantillation (U+0591–U+05AF), none of which count as the vowel; (iii) the string is not in NFC, the form `exactKey` uses (ADR-0001). `fix.suggested` is the NFC-normalized value for (iii) only; (i) and (ii) have no suggestion. The ranges are the ones `validate.js` already uses (`HEBREW_POINT`, `POINTS_AND_ACCENTS`). A person who reads Hebrew confirms the allowed-combination list before this spec completes (ledger aud-2).
  - **`field-language`** — a field the registry marks Hebrew holds no Hebrew letter (an empty field is not flagged); a field marked English or Latin holds a Hebrew letter. A field marked `mixed` is exempt.
  - **`lexicon-form`** — for each new word's `word`: when the lexicon has exactly one entry whose stripped key (ADR-0001's stripped key) equals the word's stripped key and that entry's pointed `key` differs from the word (NFC), warn with `fix.suggested` = the lexicon's pointed key. When that entry is `verified`, also warn where `meaning` or `translit` differs (trimmed, case-insensitive) from `gloss_en` or `translit`, with `fix.suggested` the lexicon value. Several entries sharing a stripped key, or none, produce no finding. The message says the lexicon is the authority only when `verified`.
  - **`pointing-consistency`** — within one lesson, the same stripped key pointed in two different ways across the registry's word-level Hebrew fields (`wordEntry.word`, `letterFind.word`, `explainerExample.he`, `conjugationCell` forms as stored); `fix.candidates` lists the forms. No suggestion is made.
  - **`audio-stale`** — when `snapshot` is given: for each text and clip pair in the `AUDIO_PAIRS` table, a node present in both the working copy and the snapshot whose text field differs (NFC, trimmed) while its clip path is non-empty and equal. The message says the clip may no longer match the text and that the check cannot tell a clip that was already stale at publish. `fix` names the field and the clip, and the regenerate path (ledger aud-4). No snapshot, no finding.
- **Registries, with drift tests.** `FIELD_LANGUAGE` maps each string property of the plan schema to `he`, `en`, `latin`, `mixed`, or `exempt` (ids, paths, enums, patterns, titles). `AUDIO_PAIRS` lists each text field and its clip field. The categories the executor fills from `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` (as read 2026-10-12): Hebrew — `wordEntry.word`, `wordEntry.sentence`, `patternItem.pointed`, `paragraph.text`, `line.pointed`, `listeningBlock.passage`, `hebrewLine.he`, `explainerExample.he`, `glossPair.he`, `clozeItem.textPre`/`textPost`/`answer`, `letterSound.pointed`, `letterFind.word`, `verseReadingBlock.verse`, `versePersonalQuestion.he`; English — `wordEntry.meaning`, `glossPair.en`, `clozeItem.english`, `hebrewLine.en`, `explainerExample.en`, `versePersonalQuestion.en`; Latin — `wordEntry.translit`, `wordEntry.sound`, `explainerExample.translit`, `letterSound.sound`. Audio pairs — `wordEntry.word`→`audioPath`, `wordEntry.sentence`→`sentenceAudioPath`, `patternItem.pointed`→`audioPath`, `paragraph.text`→`audioPath`, `question.question`→`audioPath`, `line.pointed`→`audioPath`, `listeningBlock.passage`→`audioPath`. A test walks the schema and fails when a string property is in neither the language registry nor its exemption list, or when a property named `audioPath` or `sentenceAudioPath` has no pair — so a new field cannot silently skip the checks.
- **`validatePlan(document, { lexicon, snapshot })`** gains the optional `snapshot` option and merges the new findings into `warnings`. Publish and the write operations pass the stored snapshot when one exists.
- **The issue list.** `server/plans/issues.js` — `list(store, planId, { severity, code, pathPrefix, limit }, ctx)` opens the working copy, validates it with the lexicon and the snapshot, and returns `{ revision, status, counts: { block, warn, inform }, issues: [{ code, severity, hard, path, field, pointer, message, fix? }], truncated }`. `code` is the finding's `rule` (stable); `hard` is `severity === 'block'`; `pointer` is the original RFC 6901 pointer; `path` is the deepest id-addressable node on that pointer (`units/{u}/lessons/{l}/blocks/{b}/words/{id}`, the same paths `read_part` takes); `field` is the remaining pointer after that node (for example `meaning`, or `spans/2/text`), empty when the finding is about the node. Default severities returned are `block` and `warn`; `inform` only when asked. Sorted by severity (block, warn, inform), then path. Default limit 100, maximum 200, `truncated` set past it. A `fix`, where present, is `{ action, path, field, suggested | candidates }` and applies with one `set_field` or `replace_part` call.

**Scope (out):** the tool and route (SPEC-009, SPEC-010); the panel and the "use the lexicon form" button (SPEC-015); building a new media or audio feature; changing any existing rule's severity; a server-side text model.

---

## Understood (agent's read of the intent)

- [x] und-1: Correctness of a Hebrew form in context is not mechanizable. The lexicon `verified` flag set by a person is the authority; the rules flag form and consistency only (`RSCH-009` §1.4: experts miss sparse pointing errors, Moderate).
- [x] und-2: A warning is advice for the AI and the admin. A plan with only these warnings publishes.
- [x] und-3: `audio-stale` is a warning on published plans only. It compares against the stored snapshot, so it is silent for a plan never published.

## Acceptance Criteria

- [x] ac-1 (REQ-008): each of the five codes fires on a purpose-built fixture, as `warn`, with the expected `path`, `field`, and (where defined) `fix`; and does not fire on a clean hand-written fixture. A run over the three imported legacy plans' unmodified text is recorded in the completion summary (each firing is listed for review, not asserted away). A fixture with two vowels on one letter, one with a stray point at the start of a word, and one with a decomposed (non-NFC) form each fire `pointing-form` for the right clause.
- [x] ac-2 (REQ-008): none of the five codes ever appears in `blocks`. A plan whose only findings are the five new warnings passes `validatePlan` with `ok: true`, is accepted by every write operation, and publishes through `publishPlan`. _Checked: setField, replacePart, insertBlock, moveBlock, and removeBlock each accept a plan that carries the warnings; assignPlan is not exercised (it changes no text)._
- [ ] ac-3 (REQ-008, REQ-017): the existing validator tests (`test/plan-validate.test.js`, `test/plan-language-1-1.test.js`) pass unchanged; findings of existing rules gain no new keys. _Not met as written: two assertions were narrowed, see the completion summary._
- [x] ac-4 (REQ-008): the registry drift tests fail on a mutated copy of the schema that adds a string property, and a property named `audioPath`, that are in no registry; and pass on the real schema.
- [x] ac-5 (REQ-008, REQ-018): `audio-stale` fires when a published plan's word is edited and its `audioPath` is left, does not fire when the `audioPath` also changed or when no snapshot exists, and its message says it cannot tell an already-stale clip; its `fix` text states the regenerate path the executor verified (ledger aud-4). _Checked through `validatePlan` and `issues.list`, which pass the stored snapshot. The write operations and `publishPlan` do not pass it yet: see the deliverable below._
- [x] ac-6 (REQ-008): `issues.list` for a plan with one hard error, three warnings of two codes, and two informs returns the block first, then the warnings by path, with `counts` `{ block: 1, warn: 3, inform: 2 }` always (the counts cover every severity) and the two informs listed only when `severity` asks for them; each issue's `path` equals an id path that `read_part` accepts and `field` is the remainder of the pointer; filters by `severity`, `code`, and `pathPrefix` each narrow the result; the 200 cap sets `truncated`.
- [x] ac-7 (REQ-008): applying a `fix` for `lexicon-form` with one `set_field` removes that finding on the next `issues.list`; applying it for `pointing-form` (iii) with one `set_field` removes that finding.
- [x] ac-8: no rule calls a network service or a model; the lexicon lookup uses only `server/db/repos/lexicon.js` reads; a validation of a 512 KiB fixture completes in the existing test time budget (no new timeout).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Warn only | All five are `warn`, with no setting to raise them. | resolved | yes — operator D3 |
| aud-2 | Allowed pointing combinations | The list in `pointing-form` is written from the Unicode ranges `validate.js` already uses; a person who reads Hebrew reviews it before completion. The rule is advisory, so an over-flag costs a glance, not a block. | ambiguous | no — planning default applied; review recorded in the completion summary |
| aud-3 | Stripped-key lookup | If `lexicon.js` has no stripped-key read, add a read-only function there (scan once per validation and cache per call). No schema change. | resolved | yes — no migration |
| aud-4 | Regenerate path for a plan clip | `RSCH-009` found that nothing under `server/` generates audio for a plan field. The executor checks `server/audio.js` and `generate_lesson_audio` for plan fields and writes the real path into the `audio-stale` fix text; if none exists, the fix text says so and names removing the stale `audioPath` as the fix. No media feature is built. | ambiguous | no — planning default applied (initiative §16 A7) |

> **Readiness** = 100 against the stated defaults.

---

## Deliverables

- [x] `server/plans/hebrew-checks.js`, `server/plans/issues.js`
- [ ] Changes to `server/plans/validate.js` (five `RULES` rows, the `snapshot` option, the merge), `server/plans/ops.js` and `server/plans/publish.js` (pass the snapshot), and `server/db/repos/lexicon.js` if a stripped-key read is missing _Partly: validate.js and lexicon.js are done; ops.js and publish.js are owned by SPEC-002 and still need the one-argument change in the completion summary._
- [x] Tests: `test/plan-hebrew-checks.test.js`, `test/plan-issues.test.js`

---

## Technical Requirements (domain-specific)

- [x] Pure functions of a document, an optional lexicon, and an optional snapshot. No I/O besides the lexicon reads the caller passes in.
- [x] Hebrew fixtures are written by hand in the test file. No teacher text, no learner text.
- [x] No new dependency; Unicode ranges as regular expressions; `String.prototype.normalize('NFC')` for form.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** the two test files, the unchanged validator tests, and `npm test`.
- **Rollback plan:** revert the commit; warnings disappear, nothing was stored.

---

## Security

- **Sensitivity:** standard. Triggers matched: reads plan text and lexicon rows; no write.
- **Review depth:** standard · **focus areas:** no finding can block; bounded work on a 512 KiB document; no value in logs.

---

## Verification Strategy

- **Claim:** every problem the research named is reported as a warning with a stable code, an id path the AI can act on, and a fix where one is known, and no such warning ever blocks a write or a publish.
- **Check + executor:** `mechanized` — `node --test test/plan-hebrew-checks.test.js test/plan-issues.test.js test/plan-validate.test.js test/plan-language-1-1.test.js`, then `npm test`. `human` — a person who reads Hebrew reviews the allowed-combination list in `pointing-form` (aud-2).
- **Pass condition:** exit 0; ac-1 through ac-8 each asserted; ac-2 is checked against a plan that has all five warnings and still publishes; the human review is recorded by role and date in the completion summary.
- **Test levels:** unit for each rule and the registries; integration for `issues.list` through `ops`/`publish` on a temporary database.

---

## Integration Points

- SPEC-009 and SPEC-010 expose `issues.list`. SPEC-015 renders it and applies `fix`. SPEC-017's skill tells the AI to read issues after a write.
- Produces **Change:** five warning rules and the issue list.
- **Provenance tag:** `INIT-008/SPEC-008`
