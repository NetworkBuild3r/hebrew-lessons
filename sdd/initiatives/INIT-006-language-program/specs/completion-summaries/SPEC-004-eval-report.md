# SPEC-004 AC Verification Eval Report — Pure FSRS scheduler and signal mapping

**Traceability:** INIT-006/SPEC-004 \
**Verifier:** two fresh `generalPurpose` sub-agents that did not author the code, run one after the other: [Verify SPEC-004 FSRS criteria](b626cff7-4242-4fe9-bbf8-72eb3b9ec22c), then [Re-verify SPEC-004 after fixes](00acdccb-a471-4608-84ba-68f5d78ad904). Both used the inherited session model and temperature 0 (prompt). Both were read-only by prompt, because the Task tool has no readonly flag. \
**Evaluated at:** 2026-10-11T02:30:00Z \
**Verdict:** PASS · **Remediation loops:** 0 (the first pass was all-PASS; two of its non-blocking findings were fixed, then a fresh verifier re-ran every check on the final code)

---

## Acceptance-criteria results

Overall **PASS** requires every AC to PASS with concrete evidence. This table is the second verifier's verdict on the final code. The first verifier gave the same verdicts on the earlier code.

| AC | Criterion | Verdict | Evidence |
| ---- | --------- | ------- | -------- |
| ac-1 | Golden tests: for a fixed `now` and a sequence of grades, `nextCard` produces the stability, difficulty, and interval values in a fixture computed from the pinned FSRS formulas; the fixture records the formula source | PASS | **Provenance:** the fixture records the formula page, its URL, the retrieval date 2026-10-09, and ADR-0009 §2 (`test/fixtures/fsrs/fsrs6-golden.json:3-8`). It names the implementation as py-fsrs 6.3.2, FSRS-6, run outside the repo (`:9-24`), and has a ts-fsrs 5.4.2 `crossCheck` block: 52 steps, largest relative difference 4.8e-8, limit 1e-6.<br>**Tests:** each step compares retrievability, stability, and difficulty with a relative tolerance of 1e-9, the interval exactly, and the four preview intervals (`test/fsrs.test.js:23,26-33,74-96`).<br>**Parameters:** `PARAMS.w` equals ADR-0009 §2's 21 numbers, retention is 0.9, intervals are 1–365 days, and the constant is frozen.<br>**Regeneration:** the verifier rebuilt the fixture with py-fsrs 6.3.2 outside the repo. All 52 steps and 156 float values were bit-identical, and with the cross-check added the file had the same SHA-256 as the committed one.<br>**Lapse cap:** the `lapse-cap` case's step 2 stability equals 0.212 / e^(w17·w18), so the cap binds.<br>**Mutation test:** the verifier made 7 deliberate math changes: removing the lapse cap, the same-day floor, the ordering rule, the Again interval, mean reversion, and the Hard penalty. Every one failed the golden test. |
| ac-2 | Under fsrs: `pass` with no grade schedules as Good; `miss` as Again and increments lapses; `not_yet` leaves `due`, `stability`, `difficulty` unchanged | PASS | Code: `server/progress.js:22,307-309`; the not_yet branch is at `:241-247`, and the repository keeps FSRS fields that are not passed in (`server/db/repos/review.js:49-54`). The three ac-2 tests in `test/progress-grades.test.js` pass.<br>The verifier's own probe started a process with `SRS_SCHEDULER=fsrs` and took expected values from py-fsrs, not `fsrs.js`. A first pass gives stability 2.3065, due in 2 days. A miss 3 days after Good gives stability 0.6368506992409603 and difficulty 7.394502741279718, bit-equal to py-fsrs; lapses go 0→1, due the next day, state `relearning`. A not_yet leaves due, stability, difficulty, reps, lapses, last grade, and last time identical. |
| ac-3 | Two `pass` signals on the same local day advance once | PASS | Code: `shared/player/fsrs.js:288-294`, with `fromLesson` set at `server/progress.js:248`. The ac-3 tests pass in `test/progress-grades.test.js` (fsrs and ladder) and `test/fsrs.test.js`, including the local-calendar-day cases for midnight and Tokyo.<br>Probe at UTC-6: passes at 09:00, 18:00, and 23:00 left reps at 1 and due unchanged. The next day reps went to 2, matching py-fsrs. Two passes in one batch: applied 2, advanced once. |
| ac-4 | A `kind: 'form'` signal inserts a `practice_events` row with `kind='form'` and `lemma`, and creates no `review_items` row | PASS | Code: `server/progress.js:311-326`. The ac-4 tests pass under both schedulers.<br>Probe: the event has kind `form`, an NFC lemma, direction `en_he` by default, and grade 3 (1 for a miss). The review-row count was unchanged, and an existing review row for the same word was left alone. |
| ac-5 | With `SRS_SCHEDULER=ladder`, every existing `test/*review*` and progress test passes unchanged | PASS | **Ladder set:** 8 existing review and progress files plus the 2 new ones, 119/119 passing.<br>**Full suite:** `npm test` under ladder, 641/641 passing.<br>**Unchanged:** the existing files were last written on or before 2026-10-09 09:01; this spec's files date from 2026-10-10 19:47 onward. The git diffs on `review-page.test.js` and `progress-view.test.js` come from the earlier single-page-app move and contain no scheduler lines. |
| Security | `grade` must be an integer 1–4, or the signal is rejected and counted in `rejected` | PASS | Code: `server/progress.js:124-129,146,153,157-160`. Under both schedulers the verifier sent 45 malformed signals in one batch and 42 one at a time. All were rejected and counted, nothing was written for them, and the 8 valid controls applied.<br>The malformed set covered grades `0`, `5`, `-1`, `2.5`, `'3'`, booleans, `NaN`, `Infinity`, `[3]`, `{}`, and `1e300`; a grade that doesn't fit the result; a grade on not_yet; non-string or unknown kinds; and missing, empty, or non-string lemmas. |
| Technical requirements | No `Date.now()` in pure functions; floats compared with a tolerance; static allowlist (conditional) | PASS | `Date.now` appears only in comments (`fsrs.js:2,6`), and a test that replaces `Date.now` with a throwing stub passes. Tolerances are 1e-9 and 1e-12 relative. `server/web.js:130-135,154-163` serves all of `/shared/` by prefix, so there was no explicit allowlist to edit. |
| Constraints | `shared/activities/` unchanged; no new npm dependency; migration 005 not touched | PASS | Nothing under `shared/activities/` changed (newest file is Oct 8). Neither `package.json` nor `package-lock.json` mentions fsrs. Migration 005 is tagged `INIT-006/SPEC-003` and does not touch the review or event tables. |

## Definition of done

- [x] `shared/player/fsrs.js` exports `initCard`, `nextCard`, `retrievability`, and `previewIntervals`. Both verifiers read the code and ran the tests.
- [x] `applySignals` accepts an optional grade, maps pass→3, miss→1, and not_yet→none, logs form signals without a review row, and skips suspended rows. Checked by the tests and the verifiers' probes.
- [x] `SRS_SCHEDULER` is read once at module load and defaults to ladder; `stageFor` reads stability under fsrs. Checked by tests.
- [x] Pass condition met: `SRS_SCHEDULER=ladder npm test` exits 0 (641/641), and `SRS_SCHEDULER=fsrs node --test test/fsrs.test.js test/progress-grades.test.js` exits 0 (51/51).

## Tests run by the verifier (second pass, final code)

```text
$env:SRS_SCHEDULER='fsrs';   node --test test/fsrs.test.js test/progress-grades.test.js        → exit 0 ; 51/51
$env:SRS_SCHEDULER='ladder'; node --test <8 existing review/progress files + 2 new files>      → exit 0 ; 119/119
$env:SRS_SCHEDULER='ladder'; npm test                                                          → exit 0 ; 641/641
$env:SRS_SCHEDULER='ladder'; node --test <all test/*.test.js except plan-validate.test.js>     → exit 0 ; 571/571
py-fsrs 6.3.2 regeneration (outside the repo) vs committed fixture                            → 0 differences
ts-fsrs 5.4.2 cross-check on a scratch copy                                                    → same maxima as committed
mutation script (7 mutations)                                                                  → all 7 caught
signal probes on a temp database (fsrs 37 checks, ladder 18 checks)                            → all pass
```

The first pass ran the same commands on the earlier code and got 50/50, 118/118, and 640/640.

---

## Remediation log (feedback loop)

| Loop | Failing AC(s) | Gap (from verifier) | Fix (INIT-006/SPEC-004 · files) | Re-verify result |
| ---- | ------------- | ------------------- | ------------------------------- | ---------------- |
| — | none | First pass was all-PASS. Two of its non-blocking findings were fixed anyway (rows below). | — | — |
| follow-up 1 | none (finding) | `kind` and `lemma` were coerced with `String()`: `kind: ['form']` was accepted, and `lemma: {}` was stored as `"[object Object]"` | `parseSignal` rejects a non-string `kind` or `lemma` · `server/progress.js`; three more bad signals in the validation test · `test/progress-grades.test.js` | PASS (fresh verifier) |
| follow-up 2 | none (finding) | No fixture step reached FSRS-6's cap on stability after a lapse, so removing the cap still passed | New golden case `lapse-cap`; fixture regenerated with py-fsrs 6.3.2 and the ts-fsrs cross-check re-run · `test/fixtures/fsrs/*` | PASS (fresh verifier; removing the cap now fails the test) |

---

## Notes

- **Model:** the SDD config names `composer-2.5-fast` for the Cursor verifier. This harness lets a sub-agent use a different model only when the user names one, so both verifiers ran on the inherited session model.
- **Full suite under fsrs (advisory):** 639 of 641 pass. The two failures are existing tests in `test/progress-service.test.js` at lines 92 and 253, which compare directly against the ladder's `nextState`. ac-5 requires those tests unchanged, and the spec's pass condition only asks for ladder `npm test` plus the two new files under fsrs. The Verification Strategy's wider wording ("the full suite under both scheduler values") cannot hold until those two tests are pinned to ladder. That is a handoff to SPEC-020.
- **External failure during the implementer's run:** while the implementer was running `npm test`, a parallel session edited `docs/decisions/ADR-0003-assets/lesson-plan.schema.json` (20:16) and `server/plans/validate.js`. `test/plan-validate.test.js` failed during that window. That test loads only `server/plans/validate`, which loads neither `progress.js` nor `fsrs.js`. Once the parallel edit finished, the verifier's full run passed 641/641.
- **Same-day floor:** `fsrs.js` floors the same-day stability increase at 1 for Hard as well as Good and Easy. ADR-0009 §2 does not spell out the floor; py-fsrs 6.3.2 (`scheduler.py:702-708`) and ts-fsrs 5.4.2 both apply it, and the golden cases catch its removal.
- **Side effect of `npm test`:** the suite rewrites `shared/word-audio-catalog.js` and two lesson index files in place. Their content stays byte-identical, and this predates this spec.
- No AC was passed by weakening a test or the criterion.
