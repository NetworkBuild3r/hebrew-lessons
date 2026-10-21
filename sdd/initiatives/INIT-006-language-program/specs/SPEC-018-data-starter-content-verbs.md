# SPEC-018 · Data: Starter content — verbs guide and verb workshop (drafts)

## Metadata

```yaml
spec_id: SPEC-018
initiative_id: INIT-006
title: "Draft 'Decoding Hebrew Verbs' guide and one verb workshop; lexicon forms for four verbs, unverified"
domain: data
status: complete
primary_prompt: .claude/agents/implementation-planner/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-005, SPEC-011]
blocks: [SPEC-020]
```

---

## Description

**Context:** the operator supplied an example verb workshop (לַחְשֹׁב, לְהַרְגִּישׁ, לִשְׁאֹל, לַעֲנוֹת with present and past tables, sentences, questions) and a "Decoding Hebrew Verbs" outline (shoresh, binyan, regular vs non-פָּעַל verbs, infinitive, present, past, traffic-light method with yellow ל״ה verbs like רָאָה / קָנָה and red verbs like הָלַךְ / יָרַד / יָשַׁב). The class notes contain pointing errors (INIT-006 §9), so no form is trusted until checked. ADR-0001 #7: no teacher text is copied; content is written fresh.

**Scope (in):**

- Lexicon via `lexicon.upsert` (actor `admin`, `verified=0`): the four lemmas with gloss, root (`ח.ש.ב`, `ר.ג.ש`, `ש.א.ל`, `ע.נ.ה`), binyan (`paal` for three, `hifil` for לְהַרְגִּישׁ), root class (`lamed-he` for ע.נ.ה); their 4 present forms and 9 past forms each, pointed, with `accept[]` unpointed spellings. Also lemmas for the guide's examples (רָאָה, קָנָה, הָלַךְ, יָרַד, יָשַׁב) without full paradigms.
- Plan "Guides" (shared, draft) with lesson `decoding-hebrew-verbs` (recipe `guide`): explainer sections for root, binyan, "regular vs special" (פָּעַל vs not), the infinitive in פָּעַל (ל + … + וֹ before the last letter), present (four forms), past (person endings), traffic-light callouts (green regular, yellow ל״ה, red irregular).
- Plan "Verbs" (shared, draft) with lesson `verbs-think-feel-ask-answer` (recipe `verb-workshop`): explainer (one short rule), four conjugation tables per tense in study mode, one drill block, one sentence and two questions per verb, a closing personal question.
- A checklist file `sdd/initiatives/INIT-006-language-program/specs/completion-summaries/SPEC-018-forms-checklist.md` listing every form with its cell for the operator (or teacher) to tick; then `setVerified` for ticked forms only (admin action, after the gate).
- Script `scripts/seed-starter-content.js` that writes the above through the services (idempotent; refuses to run against a database whose lexicon already has the same keys with `source` ≠ `admin`).

**Scope (out):** publishing (only after the gate); audio generation (separate, through `generate_lesson_audio` with inventory and verify).

---

## Understood (agent's read of the intent)

- [x] und-1: Drafts stay `hidden`. Nothing is published by this spec's code.
- [x] und-2: Example sentences are new, simple, and contain no names of real people from class.

## Acceptance Criteria

- [x] ac-1 (REQ-014): both drafts validate under schema 1.1.0 with no block-level errors; unverified-form warnings are expected.
- [x] ac-2 (REQ-014): the lexicon has 4 lemmas × 13 forms = 52 forms tagged with tense, cell, binyan, root class.
- [x] ac-3 (REQ-014): the forms checklist lists all 52 forms; after the operator ticks them, a second run marks only ticked forms verified.
- [x] ac-4: a search finds no teacher name, learner name, or class transcript text in the content.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Who verifies forms | The operator, or the teacher at the operator's request. Not the AI. | resolved | yes — `verified` is a person's act (INIT-006 §12) |
| aud-2 | Spelling of infinitives | Pointed standard forms (לַחְשֹׁב, לִשְׁאֹל) with plene variants (לחשוב, לשאול) in `accept[]`. | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [x] `scripts/seed-starter-content.js`
- [x] The forms checklist file
- [x] Tests: `test/starter-content.test.js` (runs the script on a temporary database and validates)

---

## Technical Requirements (domain-specific)

- [x] Writes go through `lexicon.upsert` and INIT-004 `ops` only.
- [x] NFC everywhere; a test asserts every stored key is NFC.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** operator content check before any `setVerified` or publish.
- **Blocking checks:** `node --test test/starter-content.test.js`, `npm test`, `npm run audio:verify`.
- **Rollback plan:** delete the two draft plans and lexicon rows with this spec's keys.

---

## Security

- **Sensitivity:** light. No learner data; content only.
- **Review depth:** quick · **focus areas:** no names or transcripts.

---

## Verification Strategy

- **Claim:** the starter content exists as valid drafts, fully tagged, and nothing is marked verified without a person's tick.
- **Check + executor:** `mechanized` — `node --test test/starter-content.test.js`. `human` — operator ticks the checklist.
- **Pass condition:** exit 0; verified count equals ticked count.
- **Test levels:** integration on a temporary database.

---

## Integration Points

- SPEC-014 and SPEC-016 render these lessons. SPEC-008's drill draws from verified forms.
- Produces **Change:** starter content.
- **Provenance tag:** `INIT-006/SPEC-018`
