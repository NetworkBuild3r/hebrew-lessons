# SPEC-002 · Service: Cross-lesson vocabulary index generator

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-001
title: Cross-lesson vocabulary index generator
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 3.5 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004]
```

---

## Description

**Context:** the "Review today" page (SPEC-004) needs every published word with its meaning, sound, audio clip, and source lessons. Lessons load as `window.WORKSHEET` via script tag and must not be fetched, so the server generates one index script, the same way `server/audio.js` writes `worksheets/_audio/words/catalog.js`.

**Scope (in):**
- `server/vocab-index.js`: read each lesson folder under `LESSONS_DIR` (skip `_`-prefixed entries and non-directories), evaluate its `worksheet.js` in a sandbox (`vm`, no `require`, no globals beyond a `window` stub) to read `window.WORKSHEET.vocab.items`, and merge words by the ADR-0001 identity key.
- Write `LESSONS_DIR/_review/vocab-index.js` as `window.HEBREW_VOCAB_INDEX = <JSON>;` with entries `{ key, word, meaning, sound, audio, sentence, sentenceAudio, lessons: [slug…] }`.
- CLI `npm run vocab-index` (`scripts/build-vocab-index.js`).
- Call the builder from `upsert_lesson` after audio generation, wrapped so a builder failure never fails the publish (log and continue).

**Scope (out):** client code (SPEC-003/004); new MCP tools; lesson schema changes.

---

## Understood (agent's read of the intent)

- [x] und-1: Rebuilding is cheap and idempotent; it runs on every publish and on demand.
- [x] und-2: A malformed lesson is skipped with a warning, never aborts the whole index.

## Acceptance Criteria

- [x] ac-1 (REQ-001, REQ-009): running the CLI on the repo's `worksheets/` writes `worksheets/_review/vocab-index.js` containing every vocab word from every lesson, with a word that appears in two lessons (e.g. קָמָה in `my-morning` and `anal-sex-vocab`) present once with both slugs — covered: yes
- [x] ac-2: entries carry the shared audio URL when the word-audio catalog has one — covered: yes
- [x] ac-3: a lesson whose `worksheet.js` throws or lacks `vocab` is skipped and reported; the index is still written — covered: yes
- [x] ac-4: `upsert_lesson` rebuilds the index; a thrown builder error is logged and the tool still returns success — covered: yes
- [x] ac-5 (security): the builder never reads outside `LESSONS_DIR` and never writes anywhere but `LESSONS_DIR/_review/vocab-index.js`; output is produced with `JSON.stringify` — covered: yes

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Evaluating `worksheet.js` with `vm` | acceptable because lesson files are authored by the operator's own pipeline; sandbox has no `require`/`process` (same pattern as `server/audio.js` `loadWindow`) | resolved | yes — orchestrator 2026-10-08 (mode-e run) |

---

## Deliverables

- [x] `server/vocab-index.js`
- [x] `scripts/build-vocab-index.js` and `package.json` script `vocab-index`
- [x] `mcp/tools.js` — one guarded call after audio generation in `upsert_lesson`
- [x] Tests: `test/vocab-index.test.js`

---

## Technical Requirements (domain-specific)

- [x] CommonJS, no new dependencies, Node built-ins only (`fs`, `path`, `vm`).
- [x] Uses `server/paths.js` for `LESSONS_DIR`.
- [x] Deterministic output order (sorted by key) so diffs stay small.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the listed files; delete `worksheets/_review/`.

---

## Security

- **Sensitivity:** standard — triggers matched: filesystem reads/writes, evaluation of lesson scripts.
- **Security acceptance criteria:**
  - [x] `vm` context exposes only a `window` object; a lesson script that references `require` or `process` fails and is skipped.
  - [x] Path handling uses `path.join(LESSONS_DIR, name)` on `readdirSync` names only; no user-supplied paths.
  - [x] The ElevenLabs key and environment are never written into the index.
- **Review depth:** standard · **focus areas:** data, injection.

---

## Verification Strategy

- **Claim:** the index contains exactly the merged vocabulary of all readable lessons, and building it can neither break a publish nor escape `LESSONS_DIR`.
- **Check + executor:** `mechanized` — `npm test` runs `test/vocab-index.test.js` against a temp `LESSONS_DIR` with fixture lessons (two sharing a word, one malformed, one `_`-prefixed folder, one script that touches `require`).
- **Pass condition:** the generated file parses, has the expected entry count and merged slugs, skips the malformed and hostile fixtures with warnings, writes no other file in the temp tree, and `npm test` exits 0. No coverage tool is wired up in this repo; every exported function has a normal-path and an edge-case test.

---

## Integration Points

- Consumed by SPEC-004 (`<script src="worksheets/_review/vocab-index.js">`) and optionally by SPEC-007.
- Produces **Change:** new generator, CLI, and a guarded publish hook.
- **Provenance tag:** `INIT-001/SPEC-002`
