# SPEC-004 Completion Summary — Pure FSRS scheduler and signal mapping

**Traceability:** INIT-006/SPEC-004
**Status:** Complete
**Domain:** service
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `shared/player/fsrs.js`: a UMD IIFE in the same style as `review.js`, with no dependency. It exports `initCard`, `nextCard`, `retrievability`, `previewIntervals`, a frozen `PARAMS` (FSRS-6, 21 weights, retention 0.90, intervals 1–365, version and URL in the comment), and `GRADES`. The header tells a future editor: pure functions only, the server's `now` is authoritative, and nothing from the DOM.
- [x] `server/progress.js` changes:
  - `applySignals` accepts an optional integer `grade` 1–4. Without one it maps `pass`→3, `miss`→1, `not_yet`→no schedule change.
  - `kind: 'form'` signals are logged with `lemma` and create no review row.
  - Suspended rows are not scheduled.
  - `SRS_SCHEDULER` (`ladder` default, or `fsrs`) is read once when the module loads.
  - `stageFor` reads stability under fsrs.
- [x] Tests: `test/fsrs.test.js` and `test/progress-grades.test.js`.
- [x] Golden fixture `test/fixtures/fsrs/fsrs6-golden.json`, with its generators `generate-golden.py` (py-fsrs, primary) and `crosscheck-ts-fsrs.cjs` (ts-fsrs, secondary). Both ran outside the repository in a throwaway environment. npm test does not run them.

## Acceptance Criteria

- [x] ac-1 (REQ-003): golden tests: for a fixed `now` and a sequence of grades, `nextCard` produces the stability, difficulty, and interval values listed in a fixture computed from the pinned FSRS formulas (the fixture records the formula source).
- [x] ac-2 (REQ-003): a signal with `result: 'pass'` and no grade schedules as Good; `miss` as Again and increments lapses; `not_yet` leaves `due`, `stability`, `difficulty` unchanged.
- [x] ac-3 (REQ-003): two `pass` signals on the same local day advance once.
- [x] ac-4 (REQ-011): a `kind: 'form'` signal inserts a `practice_events` row with `kind='form'` and `lemma`, and creates no `review_items` row.
- [x] ac-5 (REQ-016): with `SRS_SCHEDULER=ladder`, every existing `test/*review*` and progress test passes unchanged.

## AC Verification (independent)

Verified by fresh sub-agents that did **not** author the code. The first verifier is [Verify SPEC-004 FSRS criteria](b626cff7-4242-4fe9-bbf8-72eb3b9ec22c). After two non-blocking findings were fixed, a second fresh verifier, [Re-verify SPEC-004 after fixes](00acdccb-a471-4608-84ba-68f5d78ad904), checked the final code.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0 (no AC failed; the findings were fixed and re-verified).
**Full eval report:** [`SPEC-004-eval-report.md`](SPEC-004-eval-report.md) (per-AC evidence and the follow-up log).

**Verification tier:** `full` · **pre-flight:** dispatch did not include an AC-verification-tier line, so the tier is full · gate_actions=`[]` `pass` · security_review=`standard` `FAIL` · domain=`service` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `test/fsrs.test.js` replays 10 golden cases (52 steps), checking stability, difficulty, retrievability (1e-9 relative), interval, and the four previews at every step. The verifier regenerated the fixture with py-fsrs 6.3.2 outside the repo and got bit-identical values; with the ts-fsrs cross-check added, the file has the same SHA-256 as the committed one. Seven deliberate math mutations were all caught. `PARAMS.w` equals ADR-0009 §2 exactly. |
| ac-2 | PASS | Three ac-2 tests in `test/progress-grades.test.js` pass. The verifier's probe under `SRS_SCHEDULER=fsrs` matched py-fsrs bit for bit: a miss adds one lapse and is due the next day, and not_yet leaves due, stability, and difficulty unchanged. |
| ac-3 | PASS | The ac-3 tests pass in both test files, under fsrs and ladder, including midnight and the caller's local zone. Probe: passes at 09:00, 18:00, and 23:00 advanced once; the next day advanced again. |
| ac-4 | PASS | The ac-4 test passes under both schedulers. The event has kind `form`, an NFC lemma, and default direction `en_he`, and no review row is created or changed. |
| ac-5 | PASS | `SRS_SCHEDULER=ladder npm test` → 641/641, exit 0. The 8 existing review and progress files plus the 2 new ones → 119/119. The existing files were last written on or before 2026-10-09, before this spec started. |

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-004`
- **Title:** Add a pure FSRS-6 scheduler shared by browser and server, map lesson signals to grades in `applySignals`, and switch schedulers with `SRS_SCHEDULER` (ladder by default).
- **Files changed:** 7 · `server/progress.js` +221 / −41 · new: `fsrs.js` 353, `fsrs.test.js` 363, `progress-grades.test.js` 504, the fixture (1,343 lines of JSON), `generate-golden.py` 222, `crosscheck-ts-fsrs.cjs` 107 · plus this spec's status and checkboxes
- **Change status:** proposed (not committed; the dispatch said no commit)
- **Reversibility:** branch. Rollback is `SRS_SCHEDULER=ladder` (the default), or reverting the module and the `progress.js` edit. No schema change and no data rewrite.

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| `test/fsrs.test.js` | 31 | 31 |
| `test/progress-grades.test.js` | 20 | 20 |

**All tests passing:** Yes.
- `SRS_SCHEDULER=fsrs node --test test/fsrs.test.js test/progress-grades.test.js` → 51/51, exit 0.
- The same two files under ladder → 51/51.
- `SRS_SCHEDULER=ladder npm test` → 641/641, exit 0 (the second verifier's run on the final code).

**Coverage (new code):** not measured; this repo has no coverage tool. The spec's bar is the pass condition above. The fixture covers every formula branch: initial state, recall with each grade, the lapse and its cap, same-day reviews with each grade, the interval cap, and the ordering rule. The verifier's seven mutations each failed the suite.

**Full suite under fsrs (advisory, not a pass condition):** 639/641. The two failures are existing ladder-equivalence tests at `test/progress-service.test.js:92` and `:253`, which compare due dates with the ladder's `nextState`. ac-5 requires those tests unchanged. See the SPEC-020 handoff below.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ----------- | ------------- | --------- | --------------- | ----- |
| — | branch | n/a | `node --test test/fsrs.test.js test/progress-grades.test.js`; `npm test` | None — branch/reversible only. No `gate_actions`, no migration, no new npm dependency. Rollback: `SRS_SCHEDULER=ladder`; revert the module. |

---

## Security Review

- **Sensitivity / depth:** standard · standard (focus: signal validation) · **method:** scoped review of this spec's changed files by the implementing agent (`self_reviewed: true`). Both independent AC verifiers also probed validation (45 malformed signals in a batch and 42 one at a time, under both schedulers). SAST and secret scan: not run, because none is configured in this repo.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0 (one Low found and fixed during the spec; see the table)
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| `kind` and `lemma` were coerced with `String()`, so `kind: ['form']` was accepted and object lemmas were stored as `"[object Object]"` | Low | API8 (input validation) | `server/progress.js` `parseSignal` | fixed: a non-string `kind` or `lemma` is rejected; three tests added |
| `word`, `result`, and `direction` are still coerced with `String()` (behavior from before this spec). Only equal values pass (e.g. `['pass']`→`'pass'`), and length is bounded by the 64 KB body limit and 200 signals per request | Info | API8 | `server/progress.js` `parseSignal` | accepted (pre-existing, out of scope) |

- **Positive observations:**
  - `grade` must be an integer that agrees with its result: 1 with `miss`, 2–4 with `pass`, none with `not_yet`. Anything else is rejected and counted, and the DB CHECK (1–4) remains a second line of defense.
  - `parseSignal` builds a fresh object from listed fields only, so a client cannot inject `user_id`, `stability`, or `due`. The server computes every schedule with server time.
  - The new `practice_events` query uses bound parameters and is scoped to the session user. `SRS_SCHEDULER` is matched against a fixed list, and an unknown value is logged JSON-escaped.
  - `fsrs.js` has no network, storage, or DOM access and never reads the clock (a test stubs `Date.now` to throw).
  - Suspended rows are not scheduled.
  - No route changed: the signals route still requires a session and a visible lesson slug for every signal.

---

## Files Changed

Provenance is `INIT-006/SPEC-004` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `shared/player/fsrs.js` | Created — pure FSRS-6 scheduler: `initCard`, `nextCard`, `retrievability`, `previewIntervals`, frozen `PARAMS`, the ADR-0009 §2 same-day and interval rules | `INIT-006/SPEC-004` |
| `server/progress.js` | Modified — `parseSignal` (grade, kind, lemma), `SRS_SCHEDULER`, the fsrs path (`fsrsRow`, `cardForRow`, lazy backfill), the grade still written under ladder, suspended skip, form signals without review rows, `stageFor` reads stability under fsrs, `kind = 'word'` filters in `metrics` and `troubleWords`; exports `SCHEDULER` and `cardForRow` | `INIT-006/SPEC-004` |
| `test/fsrs.test.js` | Created — golden replay, fixture provenance, same-day rule, interval rules, bookkeeping, retrievability, purity, browser load | `INIT-006/SPEC-004` |
| `test/progress-grades.test.js` | Created — temporary database; `SRS_SCHEDULER` resolution; grade, kind, and lemma validation; ac-2, ac-3, and ac-4 under both schedulers; suspended rows; ladder fallback | `INIT-006/SPEC-004` |
| `test/fixtures/fsrs/fsrs6-golden.json` | Created — 10 cases and 52 steps from py-fsrs 6.3.2, with the ts-fsrs 5.4.2 cross-check block | `INIT-006/SPEC-004` |
| `test/fixtures/fsrs/generate-golden.py` | Created — fixture generator, run outside the repo with `fsrs==6.3.2` | `INIT-006/SPEC-004` |
| `test/fixtures/fsrs/crosscheck-ts-fsrs.cjs` | Created — second-reference check, run outside the repo with `ts-fsrs@5.4.2` | `INIT-006/SPEC-004` |
| `sdd/initiatives/INIT-006-language-program/specs/SPEC-004-service-fsrs-scheduler.md` | Status set to `complete`; criteria checked | `INIT-006/SPEC-004` |

---

## Trace Emission Degradation

The timer and trace scripts are not in this repo (`scripts/dev/lib/sdd_trace` and the validators are absent, and there is no `.sdd-timer.json`). This worker was dispatched by the orchestrator, which owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 | `trace_open` | The orchestrator owns the sentinel, and the trace library is not in this repo | No opening sentinel for SPEC-004 | A Tier-5 reader has no live trace file. Disclosed, not invented. |
| Phase 1 | `task_step` | Same | Step transitions | Same |
| Phase 4.5 | `harness_fallback` | Trace library absent | No event for the Cursor (T2) verifier dispatch | Disclosed here instead |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | The orchestrator owns the close, and the scripts are absent | No reconciled `trace_complete` | `run_trace_invariants_validate` and `model_dispatch_conformance_validate` were not run (scripts absent) |

---

## Deviations from the Spec

**Process**
- Commit, push, and PR were skipped, as the dispatch said.
- The persona `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo, as the spec notes. No persona was invented. The work followed the spec, ADR-0009 §2, and the conventions in `review.js` and `progress.js`.
- The timer, trace, Tier-5, and model-conformance checks were not run: the orchestrator owns them, and the scripts are absent.
- **Harness:** cursor, T2. The Task tool has no readonly flag, so both verifier prompts forbade edits. The config names `composer-2.5-fast` for the verifier, but this harness allows a different model only when the user names one, so both ran on the inherited model.
- **Research index:** no new research decision. FSRS is a product choice recorded in ADR-0009 §2.

**ADR readings made explicit.** Each is tested and documented in the code.
- *Again is due the next day.* ADR-0009 §2 says "an Again card is due the next day". `nextCard` puts Again at 1 day even when FSRS stability after the lapse is longer; stability still follows FSRS. Where this differs from the reference, the fixture stores both `interval` and `referenceInterval` and labels the step.
- *Button ordering applies to the stored schedule.* Each interval is first bounded to 1–365 days; then Good ≥ Hard + 1 and Easy ≥ Good + 1; then the cap wins. `nextCard` uses the same numbers as `previewIntervals`, so a button label never differs from the schedule it sets.
- *Same-day rule.* A lesson success on a card graded earlier that local day records `lastGrade` and `lastAt` and changes nothing else. A miss always counts. An explicit (flashcard) grade on the same day uses the FSRS same-day formula. As in both reference implementations, that formula never lowers stability for Hard, Good, or Easy.
- *`not_yet` under fsrs* leaves `due`, `last_at`, and the FSRS fields alone, because `last_at` is the elapsed-time input for FSRS. Under ladder, `not_yet` behaves exactly as before.
- *Grade and result must agree:* 1 with `miss`, 2–4 with `pass`, none with `not_yet`. Anything else is rejected and counted.
- *A form signal with no direction* is stored as `en_he`, because the column is required and the learner produces the Hebrew form. Form events are kept out of word stages, accuracy, and trouble words (`kind = 'word'` filters), because forms feed SPEC-006 only.

**Additions beyond the literal scope.** Each is small and cites the ADR.
- `step` and `streak` keep following the ladder under fsrs, so `SRS_SCHEDULER=ladder` resumes where the ladder would be (REQ-016).
- A passed row with no stability gets the ADR-0009 §2 migration values when it is read. These are rows the ladder wrote after migration 004.
- `cardForRow` and `SCHEDULER` are exported for SPEC-006 and SPEC-009.
- The returned card includes `lastAt` alongside the listed fields, so it can be passed back into the next call.
- `previewIntervals` returns `{1,2,3,4}` keyed by grade.
- `retrievability` returns `null` for a never-graded card.

**Other notes**
- **Static allowlist:** not applicable. `server/web.js` serves all of `shared/` by prefix, and the Dockerfile copies the whole tree.
- **Verification Strategy wording:** the strategy says "the full suite under both scheduler values". The pass condition (met) is ladder `npm test` plus the two files under fsrs. Under fsrs, two existing ladder tests fail and cannot change under ac-5.
- **Parallel work in the same checkout:**
  - `005-lexicon-library-mastery.js` was written at 19:33 by the SPEC-003 session, not by this spec.
  - At 20:16 another session edited `lesson-plan.schema.json` and `server/plans/validate.js`. `test/plan-validate.test.js` failed during that window; it loads neither changed module. The verifier's later full run passed 641/641.
- **No new npm dependency:** the reference packages were installed only in `%TEMP%\fsrs-ref-INIT-006-SPEC-004`.

**Handoffs**
- **SPEC-006:** call `HebrewFsrs.retrievability(card, now)` with a card from `progress.cardForRow`. It returns `null` when the card was never graded and counts elapsed days as whole local calendar days. Read `progress.SCHEDULER` for the ladder approximation. `recordConceptEvent` fits beside the `kind === 'form'` branch in `applySignals`.
- **SPEC-009:** build each card's `intervals` with `HebrewFsrs.previewIntervals(progress.cardForRow(userId, row, now), now)`.
- **SPEC-012:** send `grade` 1–4 with `result` `miss` for 1 and `pass` for 2–4.
- **SPEC-014:** form signals need `kind: 'form'`, `lemma`, and a visible `slug`, which the route requires for every signal.
- **SPEC-020:** before running the suite with fsrs, pin `test/progress-service.test.js:92` and `:253` to ladder. Flip to fsrs soon after migration, as the runbook does: rows the ladder advances in between keep migration-time stability. NULL rows are handled by the lazy backfill. `.env.example` and the compose and k8s docs are SPEC-020's.

---

## Effort Tracking

**Agent session time:** not recorded — the timer script is absent, and the orchestrator owns the timer
**Spec effort estimate:** 5 hours
