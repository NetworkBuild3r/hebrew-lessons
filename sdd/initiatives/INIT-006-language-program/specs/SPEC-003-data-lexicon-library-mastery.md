# SPEC-003 · Data: Lexicon, concept registry, personal library, concept mastery

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-006
title: "Tables for lexicon entries and forms, grammar concepts, user library, concept mastery"
domain: data
status: complete
primary_prompt: .claude/agents/database-architect/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: one_way
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-001]
blocks: [SPEC-005]
```

---

## Description

**Context:** there is no lexicon. Words exist inside lesson files and are merged into `_review/vocab-index.js` (`server/vocab-index.js:78`–`:118`). Grammar tags exist nowhere. ADR-0009 §4 and §6 define the registry and the lexicon. Content (lexicon) and learner data (library, mastery) are separate tables so the profile query can never join content writes to a learner by accident.

**Scope (in):** next free migration, `<n>-lexicon-library-mastery.js`:

- `lexicon_entries`: `key TEXT PRIMARY KEY` (exactKey), `kind TEXT CHECK (kind IN ('lemma','form','phrase'))`, `lemma_key TEXT` (null for a lemma), `gloss_en TEXT`, `pos TEXT` (enum from ADR-0009 §6), `root TEXT` (letters joined by `.`, e.g. `ח.ש.ב`), `binyan TEXT`, `root_class TEXT`, `tense TEXT`, `cell TEXT`, `translit TEXT`, `accept_json TEXT NOT NULL DEFAULT '[]'` (unpointed accepted spellings), `verified INTEGER NOT NULL DEFAULT 0`, `verified_by TEXT`, `source TEXT` (`seed`, `mcp`, `admin`), `rev INTEGER NOT NULL DEFAULT 0`, `updated_at TEXT NOT NULL`. CHECKs for every enum column; `length()` CHECKs from ADR-0009 §6.
- `grammar_concepts`: `key TEXT PRIMARY KEY`, `family TEXT` (`binyan`, `tense`, `cell`, `root-class`), `label_en TEXT`, `label_he TEXT`, `colour TEXT NULL` (`green`/`yellow`/`red` for root classes), `sort INTEGER`. Seeded by the migration from ADR-0009 §4.
- `user_words`: `user_id` FK cascade, `word TEXT` (exactKey), `source TEXT CHECK (source IN ('self','ai','lesson'))`, `added_at TEXT`, `gloss_en TEXT NULL CHECK (gloss_en IS NULL OR length(gloss_en) <= 100)` (the learner's own note, ADR-0009 Decision 6), `PRIMARY KEY (user_id, word)`.
- `concept_mastery`: `user_id` FK cascade, `concept_key` FK to `grammar_concepts`, `attempts INTEGER`, `correct INTEGER`, `ewma REAL`, `updated_at TEXT`, `PRIMARY KEY (user_id, concept_key)`.
- Indexes: `lexicon_entries(lemma_key)`, `lexicon_entries(binyan)`, `concept_mastery(user_id)`.
- Repos in `server/db/repos/`: `lexicon.js`, `concepts.js`, `user-words.js`, `concept-mastery.js`, in the existing repo style (`server/db/repos/review.js`). No file IO.
- Down: drop the four tables.

**Scope (out):** validation rules beyond CHECKs (SPEC-005); seeding entries from lessons (SPEC-005); mastery math (SPEC-006).

---

## Understood (agent's read of the intent)

- [x] und-1: A form row's `lemma_key` must name a lemma row; enforced by the service (SPEC-005), not by a FK, so a batch can insert lemma and forms in one call in any order inside a transaction.
- [x] und-2: `user_words` holds the fact that a word is in the library, why, and the learner's own meaning note for a word that has no lexicon entry. Cards are `review_items` rows; this table does not duplicate schedule state. The suspended flag stays on `review_items` (SPEC-002).

## Acceptance Criteria

- [x] ac-1 (REQ-004): every enum column rejects an out-of-registry value; `gloss_en` (on `lexicon_entries` and on `user_words`) and `accept_json` reject values above the ADR-0009 length limits.
- [x] ac-2 (REQ-006): deleting a user removes their `user_words` and `concept_mastery` rows and no lexicon row.
- [x] ac-3: the seeded `grammar_concepts` rows equal ADR-0009 §4 exactly (a test copies the key list once from the ADR).
- [x] ac-4: `down` drops the four tables and leaves `users`, `review_items`, `practice_events`, `lesson_progress` row counts unchanged.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Lexicon in SQLite or a file under `LESSONS_DIR` | SQLite. INIT-004 moves plans into SQLite (ADR-0005 §5 "Where state lives"); the lexicon is edited by the same tools and benefits from the same revision rule. | resolved | yes |
| aud-2 | One lexicon for everyone | Yes. Content is shared and learners never write it. A learner's own meaning note for a word with no lexicon entry lives on their `user_words` row (amended by SPEC-001, ADR-0009 Decision 6). | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/db/migrations/005-lexicon-library-mastery.js`
- [x] `server/db/repos/lexicon.js`, `concepts.js`, `user-words.js`, `concept-mastery.js`
- [x] Tests: `test/lexicon-library-store.test.js`

---

## Technical Requirements (domain-specific)

- [x] Bound parameters; explicit column lists.
- [x] Keys stored NFC. A test inserts an NFD string through the repo and reads back NFC.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** `db_migration`.
- **Blocking checks:** `node --test test/lexicon-library-store.test.js` and `npm test`.
- **Rollback plan:** `down(db)`; SPEC-020 backup.

---

## Security

- **Sensitivity:** required. Triggers matched: new learner-data tables (`user_words`, `concept_mastery`); content tables later rendered to every learner.
- **Security acceptance criteria:**
  - [x] Learner tables cascade on user delete.
  - [x] No column stores typed answer text or per-event timestamps beyond `updated_at`.
- **Review depth:** standard · **focus areas:** data minimization, constraints.

---

## Verification Strategy

- **Claim:** the four tables enforce the ADR-0009 enums and limits and keep content and learner data apart.
- **Check + executor:** `mechanized` — `node --test test/lexicon-library-store.test.js`.
- **Pass condition:** exit 0 with ac-1 to ac-4 each a named test.
- **Test levels:** integration on a temporary database.

---

## Integration Points

- SPEC-005 owns writes to lexicon tables. SPEC-006 writes `concept_mastery`. SPEC-009 and SPEC-010 write `user_words` through `library.add`.
- Produces **Change:** lexicon, concepts, library, mastery tables.
- **Provenance tag:** `INIT-006/SPEC-003`
