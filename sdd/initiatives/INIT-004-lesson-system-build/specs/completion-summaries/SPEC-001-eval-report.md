# SPEC-001 AC Verification Eval Report — Plan store: working copy, revision, and snapshot

**Traceability:** INIT-004/SPEC-001 \
**Verifier:** independent readonly sub-agent · model `composer-2.5-fast` (`ac_eval.verifier_model` on cursor) · temperature 0 \
**Evaluated at:** 2026-10-09T16:20:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | After migrate, a plan inserts as `draft` at revision 1 with a null snapshot and reads back the same JSON bytes | PASS | Test `ac-1: inserts a draft at revision 1 with a null snapshot and reads the same JSON bytes` passed. `test/plan-store.test.js` asserts `saved.document === document`, `loadSnapshot` null, and the raw `working_copy` column. `server/plans/store.js` INSERT writes `snapshot` NULL. `node --test test/plan-store.test.js`: 9 pass, 0 fail, exit 0. |
| ac-2 | Down removes the new tables and leaves INIT-002 rows; `users` and `lesson_progress` counts match before and after | PASS | Test `ac-2: down drops only plans and keeps users and lesson_progress row counts` passed. Asserts equal counts, `removed` deep-equals `['plans']`, and `users` / `lesson_progress` / `review_items` / `audit_log` remain. `down` is `DROP TABLE IF EXISTS plans` plus a bound delete of schema version 2. Same command: 9 pass, 0 fail. |
| ac-3 | Migration does not write audio paths and does not open `LESSONS_DIR` | PASS | Test `ac-3: the migration writes no audio paths and does not open LESSONS_DIR` passed. Source scan of `002-plan-store.js` finds no `LESSONS_DIR`, `.mp3`, `worksheets`, `audio`, or `require(`. A temp lessons directory's marker file is unchanged across `db.migrate()`. Stored plan text contains no `.mp3`. Same command: 9 pass, 0 fail. |
| security | SQL uses bound parameters | PASS | `server/plans/store.js` queries use `?` and pass values to `db.get` / `db.run`. Test `stores a quoted document through bound parameters` and `rejects an id that is not a slug, including a SQL fragment` passed in the same 9/9 run. |
| security | Down cannot drop a pre-existing table | PASS | The only `DROP` is `plans`. ac-2's table-name diff is exactly `['plans']`. Same 9/9 run. |

## Definition of done

- [x] Working copy, revision, status, nullable snapshot, audience, and assignee round-trip on a temporary database — ac-1 test.
- [x] Apply and down exist; down leaves `users` and `lesson_progress` counts unchanged — ac-2 test, then `db.migrate()` recreates `plans`.
- [x] Migration does not read the lessons tree — ac-3 test.
- [x] `npm test` exits 0 — implementer run, 327 pass / 0 fail (verifier ran the spec file only, per its prompt).

## Tests run by the verifier

```text
node --test test/plan-store.test.js
→ 9 pass / 0 fail ; exit 0
```

The verifier did not run `npm test` or `npm run db:migrate`, and did not open `data/hebrew.sqlite`.

---

## Remediation log (feedback loop)

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-004/SPEC-001 · files) | Re-verify result |
| ---- | ------------- | -------------------- | ------------------------------- | ---------------- |
| — | — | First pass was all-PASS | — | — |

---

## Notes

- The verifier was a fresh general-purpose subagent (`composer-2.5-fast`) that did not author the code. This report was written from that subagent's returned table because the verifier was instructed not to edit files.
- No AC was passed by weakening a test or the criterion.
- Adversarial notes that did not change the verdict: the insert path is `saveWorkingCopy` after migrate, not a raw SQL insert; ac-3's runtime check is a temp `LESSONS_DIR` plus a source scan.
