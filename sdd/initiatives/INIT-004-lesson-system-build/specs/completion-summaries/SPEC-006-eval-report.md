# SPEC-006 AC Verification Eval Report — Media asset rows and the AI-planning switch

**Traceability:** INIT-004/SPEC-006 \
**Verifier:** independent sub-agent · model `composer-2.5-fast` (cursor `ac_eval.verifier_model`) · temperature 0 \
**Evaluated at:** 2026-10-09T16:45:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | After migrate, every pre-existing user is off. An insert that omits the column is off. | PASS | Test `ac-1: existing users and inserts that omit the column stay off` passed. `alice` was inserted before migration 003, while `users` had no `ai_planning` column (`test/media-switch-store.test.js` setup + assertion). After `db.migrate()`, `alice.ai_planning` is 0. Raw insert of `bob` and `users.create` of `carol` omit the column and read 0. `PRAGMA table_info` reports `notnull` 1 and default `0`. Migration source matches `ADD COLUMN ai_planning INTEGER NOT NULL DEFAULT 0` and has no `UPDATE users` (`server/db/migrations/003-media-and-ai-planning.js`). `users.create` INSERT does not name `ai_planning` (`server/db/repos/users.js`). Command: `node --test test/media-switch-store.test.js` → 5 pass, 0 fail, exit 0. |
| ac-2 | A media row can be stored. The column list fails if `filename` or `original_name` is present. | PASS | Test `ac-2: media can be stored and the column list has no client filename` passed. `PRAGMA table_info(media)` includes `stored_name` and excludes `filename` and `original_name`. `SELECT filename` and `SELECT original_name` throw. An INSERT that names `filename` throws `no column named filename` and writes no row. `media.insert` with a `filename` field throws `client filename is not stored`. A row for `coffee-cup` round-trips with status `pending`. Same command: 5 pass, 0 fail. |
| ac-3 | Down restores `users` without `ai_planning`, drops `media`, leaves plan tables, and does not change `lesson_progress` counts. | PASS | Test `ac-3: down drops media and ai_planning and keeps plans and lesson progress` passed. `down` drops `media`, drops `ai_planning` when present, and deletes schema version 3 (`003-media-and-ai-planning.js`). Counts of `users`, `lesson_progress`, and `plans` match. `SELECT ai_planning` throws. `plans` and `lesson_progress` remain and their rows are readable. `db.migrate()` applies version 3 again and the switch is 0. Same command: 5 pass, 0 fail. |

## Definition of done

- [x] The switch defaults off for old and new accounts — ac-1 test, exit 0.
- [x] Media rows cannot store the upload filename — ac-2 column-list assertions ran and passed.
- [x] `node --test test/media-switch-store.test.js` exits 0.
- [x] Security: no insert uses an on default. The explicit `setAiPlanning(..., 1)` in a later test is a write, then ac-3 shows down removes it and re-migrate is 0 again.
- [x] Security: the client filename cannot be persisted because the column does not exist — ac-2.
- [x] `setAiPlanning` returns `{ previous, next }` and does not write `audit_log`. Prompt text is not copied into `audit_log`. Test `set returns previous and next and does not copy the prompt into audit_log` passed.

## Tests run by the verifier

```text
node --test test/media-switch-store.test.js
→ tests 5, pass 5, fail 0, exit 0
```

The implementer also ran `npm test` (341 pass, 0 fail, exit 0) and `npm run audio:verify` (65 recordings, exit 0). The verifier's own command was the spec test file.

---

## Remediation log (feedback loop)

No remediation loop. The first independent pass was all-PASS.

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-004/SPEC-006 · files) | Re-verify result |
| ---- | ------------- | -------------------- | ------------------------------- | ---------------- |
| — | — | — | — | — |

---

## Notes

- The verifier did not author the migration, the repositories, or the tests.
- No AC was passed by weakening a test or the criterion.
- Tests use a temporary `DATA_DIR`. They do not call ElevenLabs or OpenAI.
- `data/hebrew.sqlite` was not migrated.
