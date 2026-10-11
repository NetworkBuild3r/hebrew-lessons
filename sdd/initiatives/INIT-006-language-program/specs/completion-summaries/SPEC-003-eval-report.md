# SPEC-003 AC Verification Eval Report — Lexicon, concept registry, personal library, concept mastery

**Traceability:** INIT-006/SPEC-003 \
**Verifier:** independent sub-agent · model `composer-2.5-fast` (`ac_eval.verifier_model` on cursor) · temperature 0 \
**Evaluated at:** 2026-10-10 \
**Verdict:** PASS  ·  **Remediation loops:** 1

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | Enum columns reject out-of-registry values. `gloss_en` and `accept_json` reject values above the ADR-0009 length limits. | PASS | Test `ac-1: enum columns and length limits reject out-of-registry values` passed. Lexicon `gloss_en` ≤ 120 and `user_words.gloss_en` ≤ 100 are CHECKs. `accept_json` is a JSON array of at most 8 text spellings, each 1–60 characters, with no points U+0591–U+05C7, enforced by BEFORE INSERT and BEFORE UPDATE triggers in `server/db/migrations/005-lexicon-library-mastery.js`. The test rejects a 61-character spelling, nine spellings, and a pointed spelling, and accepts eight spellings of 60 characters. `node --test test/lexicon-library-store.test.js`: 6 pass, 0 fail, exit 0. The second verifier also rejected those payloads on a temporary database, including an UPDATE. |
| ac-2 | Deleting a user removes their `user_words` and `concept_mastery` rows and no lexicon row. | PASS | Test `ac-2: deleting a user removes library and mastery rows and no lexicon row` passed. `user_words.user_id` and `concept_mastery.user_id` use `ON DELETE CASCADE`. The lexicon count is unchanged. Another user's rows remain. Same command: 6 pass, 0 fail. |
| ac-3 | Seeded `grammar_concepts` rows equal ADR-0009 §4. | PASS | Test `ac-3: seeded grammar_concepts keys equal ADR-0009 §4` passed. The test copies the 32 keys once and compares them to `concepts.list()`. Spot checks cover Hebrew labels and the root-class colours. Same command: 6 pass, 0 fail. |
| ac-4 | `down` drops the four tables and leaves earlier row counts unchanged. | PASS | Test `ac-4: down drops the four tables and leaves earlier row counts unchanged` passed. `down` names only `concept_mastery`, `user_words`, `lexicon_entries`, and `grammar_concepts`. Counts of `users`, `review_items`, `practice_events`, and `lesson_progress` match. Same command: 6 pass, 0 fail. |
| NFC | A key written as NFD is stored and read back as NFC. | PASS | Test `stores an NFD key as NFC through the repository` passed. `lexicon.js` and `user-words.js` normalize with `normalize('NFC')`. Same command: 6 pass, 0 fail. |
| security | Learner tables cascade on user delete. | PASS | Same evidence as ac-2. |
| security | No column stores typed answer text or per-event timestamps beyond `updated_at`. | PASS | `concept_mastery` columns are `user_id`, `concept_key`, `attempts`, `correct`, `ewma`, `updated_at`. `user_words` has `added_at` because the spec requires the library-add time, and `gloss_en` as the learner's own note. Neither table stores a typed drill answer or a per-attempt timestamp. |

## Definition of done

- [x] The four tables enforce the ADR-0009 enums and length limits and keep content and learner data apart — ac-1 through ac-4.
- [x] Keys stored through the repositories are NFC — NFC test.
- [x] `npm test` exits 0 — implementer run, 583 pass / 0 fail (the verifier ran the spec file only).

## Tests run by the verifier

```text
node --test test/lexicon-library-store.test.js
→ 6 pass / 0 fail ; exit 0
```

The verifier did not run `npm test` and did not open `data/hebrew.sqlite`.

---

## Remediation log (feedback loop)

| Loop | Failing AC | Gap | Fix | Re-verify |
| ---- | ---------- | --- | --- | --------- |
| 1 | ac-1 | A total length of 505 characters still allowed one spelling longer than 60 characters, or more than eight spellings. | BEFORE INSERT and BEFORE UPDATE triggers reject a non-array, more than 8 items, a non-text item, an item outside 1–60 characters, and an item containing U+0591–U+05C7. Tests cover 61 characters, nine items, and a pointed spelling. | Fresh verifier: all-PASS. `node --test test/lexicon-library-store.test.js` → 6 pass, 0 fail, exit 0. |
