# SPEC-002 AC Verification Eval Report — FSRS columns on review rows and graded events

**Traceability:** INIT-006/SPEC-002
**Verifier:** independent sub-agent · model `composer-2.5-fast` · did not author the code
**Evaluated at:** 2026-10-10
**Verdict:** PASS · **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires every AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence |
| ---- | ----------- | ------- | -------- |
| ac-1 | Fixture rows at steps 0–4 and some never passed: `new` / `learning` / `longTerm` from `metrics()` before migration equal the ADR-0009 stage mapping after migration, per direction | PASS | Test `ac-1 and ac-2: stage counts match and ladder values stay byte-identical` passed. Pre-migration counts use the same ever-passed test as `server/progress.js:264–268` and the same ladder stage rules as `stageFor` (`test/fsrs-migration.test.js` `everPassed`, `ladderStage`, `fx.before`). After `up`, `progress.metrics().stages` equals those counts, and per-row ADR stages (never passed → `new`; passed and stability ≥ 14 → `longTerm`; passed and stability below 14 → `learning`) equal `metrics().stages`. Pinned totals: user 1 `he_en` `{new:5, learning:2, longTerm:3}`, `en_he` `{new:0, learning:3, longTerm:1}`. Backfill is `server/db/migrations/004-fsrs-review-state.js` `backfill`. Command: `node --test test/fsrs-migration.test.js` → 4 pass, 0 fail, exit 0. |
| ac-2 | Pre-existing `due`, `step`, `streak`, `lapses`, `last_result`, `last_at`, `sources` are byte-identical after migration | PASS | Same test. `assertLadderUntouched` deep-compares those columns for every row before and after `up`. `backfill` assigns only `stability`, `difficulty`, `reps`, and `fsrs_state`. |
| ac-3 | `down` restores the original column sets with every row intact; `up` after `down` is clean | PASS | Test `ac-3: down restores the original columns and up after down is clean` passed. After `down`, `review_items` and `practice_events` column lists match `001-initial.js`, ladder and event values match the pre-migration snapshot, and schema versions are 1–3. `db.migrate()` applies `004-fsrs-review-state.js` again; backfill and stage counts match; indexes `idx_review_items_user_due` and `idx_practice_events_user_at` remain; version 4 is recorded once. A second `down` then `migrate()` repeats cleanly. Same command: 4 pass, 0 fail. |
| ac-4 | Inserting `grade` 0 or 5, or a `kind` other than `word` / `form`, fails the CHECK | PASS | Test `ac-4: grade outside 1–4 and kind other than word or form fail CHECK` passed. Inserts of grade 0, grade 5, kind `sentence`, and kind `''` throw `CHECK constraint failed`. Grade null, 1, and 4, and kind `word` and `form`, succeed. `events.insert` with `grade: 0` also throws. Same command: 4 pass, 0 fail. |

## Definition of done

- [x] Additive FSRS columns and backfill on a temporary database, without rewriting ladder columns or `due`.
- [x] `down` then `up` round-trips.
- [x] CHECK constraints reject illegal grades and kinds.
- [x] `npm test` exits 0 — implementer run, 583 pass / 0 fail. The verifier ran `test/fsrs-migration.test.js` only.

## Tests run by the verifier

```text
node --test test/fsrs-migration.test.js
→ 4 pass / 0 fail ; exit 0
```

The verifier did not run `npm test` or `npm run db:migrate`, and did not open `data/hebrew.sqlite`.

---

## Remediation log

| Loop | Failing AC(s) | Gap | Fix | Re-verify result |
| ---- | ------------- | --- | --- | ---------------- |
| — | — | First pass was all-PASS | — | — |

---

## Notes

- The verifier was a fresh general-purpose subagent (`composer-2.5-fast`) that did not author the code. This report was written from that subagent's returned table because the verifier was instructed not to edit files.
- Pre-migration stage counts are computed with the same predicate as `metrics()`, then `metrics()` is called after `up`. `metrics()` cannot run before `up` once `review.js` selects the new columns. The test also asserts `metrics()` after `up` still equals those counts, which holds because the migration does not change the columns `metrics()` reads.
- A never-passed row with lapses is `fsrs_state` `learning` and journey stage `new`. That matches ADR-0009 §2 and does not change the stage counts.
