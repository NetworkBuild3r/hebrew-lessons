# SPEC-007 AC Verification Eval Report — Import the three legacy lessons and the café unit

**Traceability:** INIT-004/SPEC-007 \
**Verifier:** independent sub-agent · model `composer-2.5-fast` (`ac_eval.verifier_model` on cursor) · temperature 0 \
**Evaluated at:** 2026-10-09T18:30:00Z \
**Verdict:** PASS  ·  **Remediation loops:** 0

---

## Acceptance-criteria results

Overall **PASS** requires **every** AC to PASS with concrete evidence.

| AC | Criterion | Verdict | Evidence (test name+result · `file:line` · command output) / gap |
| ---- | ----------- | ------- | ---------------------------------------------------------------- |
| ac-1 | The three legacy plans and `beginner-unit-1` are stored with the ADR-0003 §10 statuses. N1 warnings on the two lessons over 8 words are recorded and do not refuse the import. | PASS | Test `ac-1: stores the three legacy plans published and the café unit as draft` passed. Legacy rows are `published` with snapshot text equal to the sample; `beginner-unit-1` is `draft` with a null snapshot. N1 warnings are only `anal-sex-vocab` and `shai-date-indirect-speech`, severity `warn`, and `result.ok` is true. `scripts/import-lesson-plans.js` inserts through `store.saveWorkingCopy` / `store.saveSnapshot` and returns before insert when `validatePlan` reports a block. `node --test test/import-lesson-plans.test.js`: 6 pass, 0 fail. |
| ac-2 | Every stored `audioPath` and `sentenceAudioPath` equals the sample. The test does not write worksheets. | PASS | Test `ac-2: stored audio strings match the samples and worksheets are untouched` passed. A walk of both fields `deepEqual`s store and sample. `shai-date-indirect-speech` and `beginner-unit-1` have zero audio fields before and after. The café unit's first word still has neither key. Worksheet tree hashes and the three `index.html` hashes match the setup snapshot. |
| ac-3 | `audio:verify` exits 0 against the inventory taken at the start. The fixture tree is not the live tree. | PASS | Test `ac-3: audio:verify exits 0 against the fixture inventory, not the live tree` passed. Inventory `lessonsDir` resolves to the temp fixture, not `worksheets/`. `node scripts/audio-inventory.js verify --against <that file>` exited 0 and printed `audio:verify OK`. Fixture clip bytes were unchanged. |
| ac-4 | No `index.html` under the three legacy lesson folders is in this spec's diff. | PASS | `git diff -- worksheets/my-morning/index.html worksheets/anal-sex-vocab/index.html worksheets/shai-date-indirect-speech/index.html` produced no output. Those paths are untracked-status clean. `worksheets/catalog.js` is a pre-existing diff and is not one of these files. |
| und-1 | A missing audio path is not invented. | PASS | Covered by ac-2: plans that omit the fields still omit them, and the stored document text equals the sample text. |
| und-2 | Re-import does not write a second copy or touch worksheets. | PASS | Test `und-2: a second import fails without a second copy and without writing worksheets` passed. Error `already_imported`, still 4 rows, `updated_at` unchanged, worksheet stamp unchanged. |
| security | The importer reads only the four sample paths. | PASS | `SAMPLE_NAMES` is the four ADR-0003 files. `sampleFile` allowlists the basename and rejects a path that escapes the samples directory. CLI argv is refused. Test `prints N1 warnings and no plan body, and rejects a path argument` passed. |
| security | The importer does not log plan bodies. | PASS | `printResult` prints ids, statuses, rule, and path. The CLI test asserts the sample sentence and gloss are absent from stdout. |

## Definition of done

- [x] Four plans load through `server/plans/store.js` on a temporary database — ac-1.
- [x] A validator block writes nothing — test `refuses a block failure and an inventory failure without inserting rows` passed in the same 6/6 run.
- [x] Audio inventory runs before the write and verify runs after, on a fixture tree in tests — ac-3.
- [x] Rollback is the four-row delete in the script header. Recordings are not deleted.
- [x] `npm test` exits 0 — implementer run, 474 pass / 0 fail / 2 todo. The verifier ran the spec file only.

## Tests run by the verifier

```text
node --test test/import-lesson-plans.test.js
→ 6 pass / 0 fail ; exit 0
```

The verifier did not run `npm test`. The implementer ran `npm test` (474 pass, 0 fail, 2 todo, exit 0) and `node scripts/audio-inventory.js verify --against data/audio-inventory/20261009T115356.json` (65 recordings, exit 0). The verifier did not open `data/hebrew.sqlite`.

---

## Remediation log (feedback loop)

No remediation loop. The first independent pass was all-PASS.

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-004/SPEC-007 · files) | Re-verify result |
| ---- | ------------- | ------------------- | ------------------------------- | ---------------- |
| — | — | — | — | — |

---

## Notes

- No AC was passed by weakening a test or the criterion.
- The Task tool in this session has no readonly flag. The verifier prompt forbade file edits. The verifier reported that it changed nothing.
- `worksheets/catalog.js` was already modified before this spec. It is outside ac-4. The import test compares the worksheets tree to its own setup snapshot and does not write it.
