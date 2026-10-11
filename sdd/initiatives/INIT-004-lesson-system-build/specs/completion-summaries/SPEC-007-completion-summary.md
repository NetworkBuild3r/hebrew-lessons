# SPEC-007 Completion Summary — Import the three legacy lessons and the café unit

**Traceability:** INIT-004/SPEC-007
**Status:** Complete
**Domain:** data
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `scripts/import-lesson-plans.js` — reads the four ADR-0003 samples, inventories audio, inserts through `server/plans/store.js`, then verifies. Header documents inventory, import, verify, and the four-row delete.
- [x] `test/import-lesson-plans.test.js` — temporary database and a fixture lessons tree.

## Acceptance Criteria

- [x] ac-1: three legacy plans `published` with a snapshot equal to the working copy; `beginner-unit-1` `draft` with a null snapshot. N1 warns on `anal-sex-vocab` and `shai-date-indirect-speech` and does not refuse the import.
- [x] ac-2: every stored `audioPath` and `sentenceAudioPath` equals the sample. The test does not write worksheets.
- [x] ac-3: `audio:verify` exits 0 against the fixture inventory taken before the write. That tree is not `worksheets/`.
- [x] ac-4: the three legacy `index.html` files are not in this spec's diff.

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`) that did **not** author the code.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-007-eval-report.md`](SPEC-007-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` `PASS` · security_review=`standard` `FAIL` · domain=`data` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

The dispatch did not classify this spec as light tier. Verification ran the spec's test file.

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: stores the three legacy plans published and the café unit as draft` passed. `node --test test/import-lesson-plans.test.js` → 6 pass, 0 fail, exit 0. |
| ac-2 | PASS | `ac-2: stored audio strings match the samples and worksheets are untouched` passed in that same run. |
| ac-3 | PASS | `ac-3: audio:verify exits 0 against the fixture inventory, not the live tree` passed. Verifier saw `audio:verify OK`. |
| ac-4 | PASS | `git diff` on the three `worksheets/*/index.html` paths was empty. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-007`
- **Title:** Import the four ADR-0003 sample plans without touching recordings or worksheet HTML
- **Files changed:** 2 code files · additions +876 · deletions −0 (plus this summary, the eval report, and the spec status)
- **Change status:** proposed
- **Reversibility:** revertible. Not applied to `data/hebrew.sqlite`. Rollback is `DELETE` of the four plan ids. Recordings are not part of that delete.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/import-lesson-plans.test.js` | 6 | 6 |

**All tests passing:** Yes · `node --test test/import-lesson-plans.test.js` → 6/6 · `npm test` → 474 pass, 0 fail, 2 todo, exit 0 · `node scripts/audio-inventory.js verify --against data/audio-inventory/20261009T115356.json` → 65 recordings, exit 0 · **Coverage (new code):** no coverage tool is configured; the spec's bar is this file plus `npm test`

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| — | revertible | — | `node --test test/import-lesson-plans.test.js`, `npm test`, `audio:verify` | No `gate_action`. No schema change. The real database was not loaded. |

None — reversible only. Inventory ran before any file this spec wrote. Verify after implementation matched that inventory (65 recordings).

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** scoped review of this spec's files (sample-path containment, no worksheet writes, no plan-body logs). SAST and secret scan: not run — none configured. Reviewer judgment: implementing agent, `self_reviewed: true` (see Harness note under Deviations). The independent AC verifier also checked the two security acceptance criteria and returned PASS.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| — | — | — | — | — |

- **Positive observations:** Sample reads go through a four-name allowlist and a containment check. The CLI rejects path arguments. Plan text is stored through `server/plans/store.js` bound parameters. Store errors are not printed, so a SQLite message cannot echo the document. `printResult` logs id, status, rule, and path. Tests use `DATA_DIR` and `LESSONS_DIR` under the OS temp directory. `IMPORT_FIXTURE_ID` is limited to `[a-z0-9-]`. Fixture mode refuses to inventory the live `worksheets/` tree. The script does not delete or overwrite clips.

---

## Files Changed

Provenance is `INIT-004/SPEC-007` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `scripts/import-lesson-plans.js` | Created — import the four samples after an audio inventory (453 lines) | `INIT-004/SPEC-007` |
| `test/import-lesson-plans.test.js` | Created — temporary-database and fixture-audio tests (423 lines) | `INIT-004/SPEC-007` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-007-data-import-legacy-lessons.md` | Status set to `complete`; criteria checked | `INIT-004/SPEC-007` |

---

## Trace Emission Degradation

Timer and trace scripts are not in this repo. This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the sentinel; `scripts/dev/lib/sdd_trace` is not in this repo | No opening sentinel for SPEC-007 | A Tier-5 reader has no live trace file. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions | Same |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No harness-fallback event | Disclosed here instead |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Orchestrator owns close; scripts absent | No reconciled `trace_complete` | Tier-5 and model-dispatch validators were not run (scripts absent) |

---

## Deviations from the Spec

- Git commit, push, and PR were not performed. The dispatch said to skip them.
- Persona `.claude/agents/database-architect/AGENT.md` is not in this repo, and `.claude/agents/` is absent. `scripts/dev/expertise_state.py` is not in this repo, so the dispatch-check script was not run. No persona file was invented. Implementation followed the spec, `server/plans/store.js`, and `server/plans/validate.js`.
- `npm test` is this repo's quality gate. No lint, type-checker, or coverage command is configured. `# type: ignore` / `# noqa` scan: not applicable (JavaScript, no such comments added).
- Timer start/complete and `run_trace_invariants_validate` / `model_dispatch_conformance_validate` were not run: those scripts are not in this repo, and the orchestrator owns the timer and trace on a dispatched spec.
- Harness: cursor. AC verification used a fresh sub-agent (`composer-2.5-fast`). The Task tool has no readonly flag; the prompt forbade edits. Security review of these files was done by the implementing agent (`self_reviewed: true`).
- `git diff -- worksheets` was not empty at setup because `worksheets/catalog.js` was already modified. This spec does not change that file. The test records the worksheets tree hash and the worksheets diff before import and asserts both are unchanged. The three legacy `index.html` diffs are empty.
- The real plan database was not loaded. The operator runs `node scripts/import-lesson-plans.js` after reading inventory output. That command inventories the real `LESSONS_DIR`, inserts, then verifies. Tests set `IMPORT_AUDIO_FIXTURE=1` and a temp `LESSONS_DIR`.
- Research index: `RSCH-004` is already applied in ADR-0003, which this spec follows. No new research decision.

---

## Effort Tracking

**Agent session time:** not recorded — timer script absent and owned by the orchestrator
**Spec effort estimate:** 4 hours
