# SPEC-005 · Service: Lexicon service, validator, and seed

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-006
title: "Lexicon read/write service with validation; seed from the vocab index"
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-003]
blocks: [SPEC-006, SPEC-009, SPEC-010, SPEC-011, SPEC-018]
```

---

## Description

**Context:** SPEC-003 created the tables. This spec is the only code that writes them (ADR-0005 §5 "one implementation"). Seed data comes from `buildVocabIndex()` (`server/vocab-index.js:159`), which already merges `word`, `meaning`, `sound`, and source lessons by `exactKey`.

**Scope (in):** `server/lexicon/` with:

- `validate.js`: per-entry checks from ADR-0009 §6 — key is NFC and equals `exactKey(key)`; Hebrew letters required in `key`; enum fields in the registry; `root` matches 2–4 Hebrew letters joined by `.`; a `form` names a `lemma_key` present in the batch or the table; `cell` belongs to its `tense`; `accept[]` entries are unpointed (no U+0591–U+05C7) and at most 8; no `<` or `>` in any text field; size limits. Returns `{ ok, errors: [{ key, field, code }] }`.
- `service.js`: `list({ q, pos, binyan, tense, rootClass, verified, limit, cursor })`, `get(key)` (lemma plus its forms grouped as a paradigm by tense and cell), `upsert(entries, { actor, expectedRev })` (validate whole batch, one transaction, `rev` bump per entry, reject a stale `expectedRev` with `revision_conflict`; a write to a `verified` form clears `verified` and records it), `setVerified(keys, value, { actor })` (admin only; the caller checks the role), `paradigm(lemmaKey)`.
- Audit rows for `lexicon_upsert` and `lexicon_verify` with actor, keys count, and no text.
- `seed.js`: idempotent seed from `buildVocabIndex()`: one `lemma` or `phrase` entry per vocab key (phrase when the key contains a space), `gloss_en` from `meaning`, `translit` from `sound`, `source='seed'`, `verified=0`. Never overwrites an entry whose `source` is not `seed`. Run at startup after migrations, logged as a count.
- A short header comment in `service.js`: all lexicon writes go through `upsert`; adapters do not write SQL.

**Scope (out):** HTTP and MCP adapters (SPEC-009, SPEC-010); conjugation forms for specific verbs (SPEC-018).

---

## Understood (agent's read of the intent)

- [x] und-1: The seed does not guess part of speech, root, or binyan. Tags come from the AI or an admin.
- [x] und-2: `accept[]` is for typed answers; display always uses the pointed `key`.

## Acceptance Criteria

- [x] ac-1 (REQ-004): a batch with one invalid entry writes nothing and returns every error with `key`, `field`, `code`.
- [x] ac-2 (REQ-004): text containing `<script>` is rejected; NFD input is stored NFC.
- [x] ac-3 (REQ-013): `upsert` with a stale `expectedRev` returns `revision_conflict` and leaves the row unchanged.
- [x] ac-4 (REQ-011): editing a verified form clears `verified`; `setVerified` writes an audit row and refuses when the actor is `mcp`.
- [x] ac-5: running `seed` twice on the three current lessons yields the same row count; an `mcp`-sourced entry with the same key keeps its gloss.

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Seed on startup vs a command | Startup, idempotent, count logged. Same pattern as `seedVisibility` in `server/start.js`. | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [x] `server/lexicon/validate.js`, `server/lexicon/service.js`, `server/lexicon/seed.js`
- [x] One call in `server/start.js`
- [x] Tests: `test/lexicon-service.test.js`

---

## Technical Requirements (domain-specific)

- [x] `exactKey` imported from `server/vocab-index.js`; vowel stripping uses the ADR-0001 range U+0591–U+05C7.
- [x] Batch limit 200 entries; text field limits from ADR-0009 §6.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/lexicon-service.test.js`, `npm test`, `npm run audio:verify` (seed reads lesson files).
- **Rollback plan:** remove the startup call; seeded rows are deleted with `source='seed'`.

---

## Security

- **Sensitivity:** required. Triggers matched: untrusted text written by the MCP and rendered to every learner.
- **Security acceptance criteria:**
  - [x] Markup characters rejected server-side, independent of the renderer's escaping.
  - [x] `setVerified` is unreachable for actor `mcp`.
- **Review depth:** standard · **focus areas:** input validation, authz of verification.

---

## Verification Strategy

- **Claim:** lexicon writes are validated, atomic, revisioned, and audited; the seed is idempotent and non-destructive.
- **Check + executor:** `mechanized` — `node --test test/lexicon-service.test.js`.
- **Pass condition:** exit 0 with ac-1 to ac-5 as named tests.
- **Test levels:** unit (validator), integration (temporary database, a copy of `worksheets/` in a temp `LESSONS_DIR`).

---

## Integration Points

- SPEC-006 reads tags for attribution. SPEC-009 and SPEC-010 adapt `list`, `get`, `upsert`, `setVerified`. SPEC-011 checks conjugation-table references with `get`. SPEC-018 writes the four starter verbs with `upsert`.
- Produces **Change:** lexicon service.
- **Provenance tag:** `INIT-006/SPEC-005`
