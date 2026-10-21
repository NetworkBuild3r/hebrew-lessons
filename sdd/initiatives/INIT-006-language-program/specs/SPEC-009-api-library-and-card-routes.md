# SPEC-009 · API: Learner library and flashcard routes

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-006
title: "GET/POST/PATCH /api/me/library, GET /api/me/cards, graded signals, lexicon reads"
domain: api
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-004, SPEC-005, SPEC-006]
blocks: [SPEC-010, SPEC-012, SPEC-013]
```

---

## Description

**Context:** routes live in `server/api/` and are mounted by `server/api/router.js`; progress routes are `server/api/progress-routes.js`. Sessions, CSRF (`originAllowed` in `server/http/security.js`), and JSON body limits (`server/http/body.js`) already exist. The first slice needs these routes and nothing from INIT-004.

**Scope (in):** `server/api/library-routes.js` and `server/learner/library.js`:

- `library.list(userId, { q, status, weak, pos, binyan, tense, source, suspended, limit, cursor }, now)`: union of the learner's `review_items` words and `user_words`, joined with SPEC-006 `wordStates` and lexicon tags. Each row: `word`, `gloss_en` (the lexicon gloss when an entry exists, else the learner's own note), `note` (the learner's note, only when a lexicon entry also exists, shown as "your note"), `translit`, `pos`, `root`, `binyan`, `root_class`, `stage` per direction, `strength_bucket`, `weak`, `suspended`, `sources`, `audio` (from the vocab index).
- `library.add(userId, words[], { source })`, `words[]` is `{ key, gloss_en? }`: the key is checked like a lexicon key (NFC, 1 to 60 characters, at least one Hebrew letter, no `<` or `>`) and stored as `exactKey`. **A learner never writes a lexicon row** (ADR-0009 §6). When the lexicon or the vocab index has the key, `gloss_en` is ignored. When neither has it, `gloss_en` (at most 100 characters, same character rule) is required and is stored on the learner's own `user_words` row, because a card needs a meaning to be answered. Creates `user_words` rows and `review_items` rows in both directions with `fsrs_state='new'`. Idempotent. Max 50 per call. `source` is `self` from the learner's route and `ai` from the MCP adapter, which (SPEC-010) accepts only keys already in the lexicon.
- `library.setSuspended(userId, word, bool)`.
- `cards.deck(userId, deck, now)`: decks `due` (default), `weak`, `new`, `lesson:<slug>`, `concept:<key>`, `all`. Returns cards `{ word, direction, prompt, answer, translit, audio, intervals }` where `intervals` is `fsrs.previewIntervals` for the four grades. Caps from `DEFAULT_CAPS`.
- Signals: `POST /api/progress/signals` (existing) accepts `grade` and `kind` per SPEC-004; this spec adds route-level tests for the new fields.
- Routes: `GET /api/me/library`, `POST /api/me/library`, `PATCH /api/me/library/:key` (`{ suspended }`), `GET /api/me/cards?deck=`, `GET /api/lexicon`, `GET /api/lexicon/:key`, `GET /api/grammar/concepts`. Session required on all; CSRF on writes; `:key` URL-decoded then `exactKey`.

**Scope (out):** MCP tools and admin writes (SPEC-010); UI (SPEC-012, SPEC-013).

---

## Understood (agent's read of the intent)

- [ ] und-1: Every route is scoped to the session user. There is no `userId` parameter on a learner route.
- [ ] und-2: Adding a word you have never seen is allowed (the operator wants a personal library); it enters as a new card under the new-card cap.

## Acceptance Criteria

- [ ] ac-1 (REQ-005): a fixture learner with 5 reviewed words and 2 added words gets 7 library rows with lexicon tags and correct stage per direction.
- [ ] ac-2 (REQ-005): `POST /api/me/library` without a valid origin returns 403; with one, it creates two review rows per word and is idempotent on repeat. A key with no lexicon or vocab entry and no `gloss_en` returns 400; with one, it stores the note on `user_words` and the `lexicon_entries` row count is unchanged. After an admin adds a lexicon entry for that key, the list row shows the lexicon gloss and the learner's note as `note`.
- [ ] ac-3 (REQ-003): `GET /api/me/cards?deck=weak` returns only weak or very weak words; each card has four `intervals`.
- [ ] ac-4: a suspended word appears in the library with `suspended: true` and in no deck.
- [ ] ac-5: a signed-out request to any route returns 401; another learner's words never appear (two-user fixture).

## Assumptions Ledger (disambiguation gate)

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Lexicon reads for learners | Allowed for signed-in users; lexicon is shared content. Unverified forms are returned with `verified: false`. | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- [ ] `server/learner/library.js`, `server/learner/cards.js`, `server/api/library-routes.js`
- [ ] Mount in `server/api/router.js`
- [ ] Tests: `test/library-routes.test.js`

---

## Technical Requirements (domain-specific)

- [ ] Pagination with an opaque cursor; `limit` ≤ 200.
- [ ] Error shape matches the existing routes (`{ error: '<code>' }`).

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/library-routes.test.js` and `npm test`.
- **Rollback plan:** unmount the router entry.

---

## Security

- **Sensitivity:** required. Triggers matched: new learner write routes.
- **Security acceptance criteria:**
  - [ ] Session and CSRF on every write; per-user scoping proven with two users.
  - [ ] Body size and batch limits enforced.
- **Review depth:** standard · **focus areas:** authz, CSRF, input limits.

---

## Verification Strategy

- **Claim:** library and card routes return the session learner's data with tags and FSRS intervals, and writes are protected.
- **Check + executor:** `mechanized` — `node --test test/library-routes.test.js` against the in-process server helper used by existing route tests.
- **Pass condition:** exit 0; ac-1 to ac-5 named tests.
- **Test levels:** integration over HTTP on a temporary database.

---

## Integration Points

- SPEC-012 and SPEC-013 call these routes through `shared/api.js`. SPEC-010 reuses `library.add` for `add_library_words`.
- Produces **Change:** learner library and card routes.
- **Provenance tag:** `INIT-006/SPEC-009`
