# SPEC-006 · Service: Lesson visibility and per-user content filtering

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-002
title: Lesson visibility and per-user content filtering
domain: service
status: complete
primary_prompt: .claude/agents/principal-backend-developer/AGENT.md   # not present in this repo; degradation path, disclosed
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-003]
blocks: [SPEC-007, SPEC-008]
```

---

## Description

**Context:** REQ-010 lets admins hide a lesson or share it with selected users, for example the adult lesson. Four generated scripts are shared by every page and would leak a restricted lesson's title and words if served whole:
- `worksheets/catalog.js` (`HEBREW_CATALOG`)
- `worksheets/_review/vocab-index.js` (`HEBREW_VOCAB_INDEX`; entries carry `lessons`)
- `worksheets/_audio/words/catalog.js` (`HEBREW_WORD_AUDIO`; maps word text to file and carries **no** lesson ids)
- `shared/word-audio-catalog.js`

**Scope (in):**
- `server/visibility.js`:
  - `visibleSlugs(user)` and `canSeeLesson(user, slug)`. Admins see every lesson. Otherwise `everyone` lessons are visible, `selected` lessons only with a grant, and `hidden` lessons never. A lesson with no row takes ADR-0002's default (aud-2).
  - `setVisibility(actor, slug, mode)` and `setGrants(actor, slug, userIds)`, both admin-only, audited, and validated with `assertSlug`.
- `server/content-filter.js`:
  - `catalogScriptFor(user)`, `vocabIndexScriptFor(user)`, and `wordAudioScriptFor(user, which)`.
  - Each reads the source file with the literal-only `server/lesson-data.js` parser, filters it, and re-emits it as `window.NAME = <JSON>;` through `JSON.stringify`. Output for the same visible set is cached and invalidated by file mtime.
  - Word-audio filtering keeps an entry only if its exact key, or its unique stripped key (ADR-0001 identity), appears among the vocab words or the tokenized text of a visible lesson.
- **One-time seed.** `seedVisibility()` runs at startup after migration. For every lesson folder that has no visibility row, it inserts `everyone` only on the very first run (recorded in a `meta` flag), so the three existing lessons, including `anal-sex-vocab`, stay visible after the upgrade. Lessons that appear later with no row take the default from ADR-0002 (proposed `hidden`).
- `actor` may be a user or the system actor `mcp`, so SPEC-007's MCP tools can call `setVisibility` and `setGrants` with audit rows naming `mcp`.

**Scope (out):** HTTP wiring and the 404 for lesson paths (SPEC-008); admin UI (SPEC-017). Word mp3 files are not filtered per lesson; they are hash-named. This is the accepted residual, recorded in ADR-0002.

---

## Understood (agent's read of the intent)

- [ ] und-1: Filtering must fail closed. If a source file can't be parsed, the filtered output is an empty dataset plus a server log line, never the unfiltered file.
- [ ] und-2: Admins get the full data so the editor and admin pages work.

## Acceptance Criteria

- [ ] ac-1 (REQ-010): for a user without a grant, none of the four filtered scripts contains a `hidden` or `selected` lesson's slug, title, or any vocab word unique to that lesson. A test asserts this for each script against a fixture with one restricted lesson.
- [ ] ac-2 (REQ-010): the same user, once granted, sees that lesson in all four scripts. An admin always does.
- [ ] ac-3: a word shared between a visible and a restricted lesson stays in the filtered vocab index and word-audio catalog, with only the visible lesson in its `lessons` list.
- [ ] ac-4: filtered output parses back through `server/lesson-data.js` (literal-only), and a value containing `</script>` or a quote is emitted safely through `JSON.stringify`.
- [ ] ac-5: a malformed source file yields an empty filtered dataset (fail closed), not the raw file.
- [ ] ac-6 (REQ-008): `setVisibility` and `setGrants` refuse non-admin user actors, reject invalid slugs, and write an audit row. The `mcp` actor is accepted and recorded as `mcp`.
- [ ] ac-7 (REQ-010): with the three current lessons on disk and an empty database, the first startup makes all three `everyone` (including `anal-sex-vocab`). A fourth lesson added afterward gets the ADR-0002 default, and a second startup changes nothing.

## Assumptions Ledger (disambiguation gate)

| id    | item | interpretation | tone | resolved |
| ----- | ---- | -------------- | ---- | -------- |
| aud-1 | Tokenizing lesson text for word-audio filtering | reuse the existing Hebrew tokenizer used by `test/tokenize-hebrew.test.js` / story glosses | resolved | yes — plan default |

---

## Deliverables

- [ ] `server/visibility.js` (including `seedVisibility`), `server/content-filter.js`
- [ ] Tests: `test/visibility.test.js`, `test/content-filter.test.js`

---

## Technical Requirements (domain-specific)

- [ ] Parse with `server/lesson-data.js` only. Never execute a data file (INIT-001 HIGH-1).
- [ ] No change to the files on disk. Filtering happens at serve time.

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `npm test`.
- **Rollback plan:** revert the listed files.

---

## Security

- **Sensitivity:** required. Triggers matched: authorization (per-lesson access), information disclosure.
- **Review depth:** full. Security acceptance: ac-1, ac-4, ac-5, ac-6, and ac-7.

---

## Verification Strategy

- **Claim:** a user who can't see a lesson can't learn its title or its unique words from any shared data script, and filtering fails closed.
- **Check + executor:** `mechanized`. `node --test test/visibility.test.js test/content-filter.test.js` runs with a fixture lessons dir holding one `everyone` lesson, one `selected` lesson, and one `hidden` lesson.
- **Pass condition:** for each of the four scripts, the restricted-lesson leak test passes for a non-granted user. The grant and admin tests show the lesson, and the malformed-source test returns an empty dataset.

---

## Integration Points

- Consumed by SPEC-008 (serving filtered scripts and the lesson-path 404), SPEC-009 (lesson-progress access), SPEC-007 and SPEC-010 (admin visibility), and SPEC-017 (UI).
- **Provenance tag:** `INIT-002/SPEC-006`
