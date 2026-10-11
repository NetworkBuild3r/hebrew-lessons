# SPEC-003 Completion Summary — Lexicon, concept registry, personal library, concept mastery

**Traceability:** INIT-006/SPEC-003
**Status:** Complete
**Domain:** data
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `server/db/migrations/005-lexicon-library-mastery.js` (apply and down). Number assigned by the orchestrator. `004-fsrs-review-state.js` was not edited.
- [x] `server/db/repos/lexicon.js`, `concepts.js`, `user-words.js`, `concept-mastery.js` — prepared statements, explicit columns, no file IO.
- [x] `test/lexicon-library-store.test.js` on a temporary database (`DATA_DIR` under the OS temp dir).

## Acceptance Criteria

- [x] ac-1: enum columns reject out-of-registry values; `gloss_en` and `accept_json` reject values past the ADR-0009 limits.
- [x] ac-2: deleting a user removes that user's `user_words` and `concept_mastery` rows and no lexicon row.
- [x] ac-3: seeded `grammar_concepts` keys equal the ADR-0009 §4 list (32 keys, copied once in the test).
- [x] ac-4: `down` drops the four new tables and leaves `users`, `review_items`, `practice_events`, and `lesson_progress` counts unchanged.

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`) that did not author the code.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 1.
**Full eval report:** [`SPEC-003-eval-report.md`](SPEC-003-eval-report.md)

**Verification tier:** `full` · **pre-flight:** not classified light by the orchestrator · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: enum columns and length limits reject out-of-registry values` passed after the accept-list trigger was added. `node --test test/lexicon-library-store.test.js` → 6 pass, 0 fail, exit 0. |
| ac-2 | PASS | `ac-2: deleting a user removes library and mastery rows and no lexicon row` passed in that run. |
| ac-3 | PASS | `ac-3: seeded grammar_concepts keys equal ADR-0009 §4` passed in that run. |
| ac-4 | PASS | `ac-4: down drops the four tables and leaves earlier row counts unchanged` passed in that run. |

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-003`
- **Title:** Add lexicon, grammar-concept, personal-library, and concept-mastery tables
- **Files changed:** migration, four repositories, the new test, and four existing tests whose version lists assumed the schema stopped at migration 003
- **Change status:** proposed
- **Reversibility:** one_way (migration file). Not applied to `data/hebrew.sqlite`. Rollback is `down`, which drops the four new tables and deletes schema version 5.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/lexicon-library-store.test.js` | 6 | 6 |

**All tests passing:** Yes · `node --test test/lexicon-library-store.test.js` → 6/6, exit 0 · `npm test` → 583/583, exit 0 · **Coverage (new code):** no coverage tool is configured; the spec's bar is this file plus the existing suite

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| db_migration | one_way | yes — operator gate on the dispatch, scoped to writing the migration and tests | `node --test test/lexicon-library-store.test.js`, `npm test` | Not applied to `data/hebrew.sqlite`. Production apply waits for the rollout spec. Rollback: `down` drops the four tables and deletes schema version 5. |

---

## Security Review

- **Sensitivity / depth:** required · standard focus (data minimization, constraints) · **method:** scoped review of this spec's files. SAST and secret scan: not run — none configured. Reviewer judgment: implementing agent, `self_reviewed: true`. The independent AC verifier also checked the two security acceptance criteria and returned PASS.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| — | — | — | — | — |

- **Positive observations:** Learner rows (`user_words`, `concept_mastery`) reference `users(id)` with `ON DELETE CASCADE`. Lexicon rows do not. `concept_mastery` stores attempts, correct, and the recent score, plus `updated_at`. It has no typed-answer column and no per-attempt timestamp. `user_words.added_at` is the library-membership time the spec requires, not a practice-event time. `user_words.gloss_en` is capped at 100 characters. Lexicon text columns reject angle brackets and ASCII controls. `accept_json` is a JSON array of at most 8 spellings, each 1 to 60 characters, with no Hebrew points (U+0591–U+05C7). Queries use bound parameters. Repos do not read the filesystem. Tests use a temporary database. Who may set `verified` is SPEC-005 and SPEC-010, not this table.

---

## Files Changed

Provenance is `INIT-006/SPEC-003` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `server/db/migrations/005-lexicon-library-mastery.js` | Created — four tables, concept seed, accept-list triggers, `up` and `down` | `INIT-006/SPEC-003` |
| `server/db/repos/lexicon.js` | Created — upsert/find, NFC keys | `INIT-006/SPEC-003` |
| `server/db/repos/concepts.js` | Created — list/find | `INIT-006/SPEC-003` |
| `server/db/repos/user-words.js` | Created — add/find, NFC word | `INIT-006/SPEC-003` |
| `server/db/repos/concept-mastery.js` | Created — upsert/find | `INIT-006/SPEC-003` |
| `test/lexicon-library-store.test.js` | Created — temporary-database tests | `INIT-006/SPEC-003` |
| `test/db.test.js` | Modified — expect migration 5 and the four tables, without assuming 004 is absent | `INIT-006/SPEC-003` |
| `test/plan-store.test.js` | Modified — version assertions allow migrations after 003 | `INIT-006/SPEC-003` |
| `test/media-switch-store.test.js` | Modified — version assertions allow migrations after 003 | `INIT-006/SPEC-003` |
| `test/rollout-preflight.test.js` | Modified — roll the fixture back to version 3 before the INIT-004 checklist | `INIT-006/SPEC-003` |
| `sdd/initiatives/INIT-006-language-program/specs/SPEC-003-data-lexicon-library-mastery.md` | Status set to `complete`; criteria checked | `INIT-006/SPEC-003` |

---

## Trace Emission Degradation

Timer and trace scripts are not in this repo. This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the sentinel; `scripts/dev/lib/sdd_trace` is not in this repo | No opening sentinel for SPEC-003 | A Tier-5 reader has no live trace file. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions | Same |
| Phase 1.5 | `gate`, `one_way_action` | Orchestrator owns gate emission; approval was already on the dispatch | No gate event in a trace file | The approval is recorded in this summary, not in `.sdd-trace` |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No harness-fallback event | Disclosed here instead |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Orchestrator owns close; scripts absent | No reconciled `trace_complete` | Tier-5 and model-dispatch validators were not run (scripts absent) |

---

## Deviations from the Spec

- Git commit, push, and PR were not performed. The dispatch said to skip them.
- Persona `.claude/agents/database-architect/AGENT.md` is not in this repo. No persona file was invented. Implementation followed the spec and `server/db/repos/review.js` (bound parameters, explicit columns, ISO timestamps).
- `lemma_key` is not a foreign key, as und-1 requires. A form must still have a non-null `lemma_key`, and a lemma or phrase must not.
- `accept_json` per-spelling rules live in BEFORE INSERT and BEFORE UPDATE triggers. A CHECK cannot use `json_each`, and a `'$[n]'` path inside a CHECK is parsed as a parameter.
- `test/db.test.js`, `test/plan-store.test.js`, `test/media-switch-store.test.js`, and `test/rollout-preflight.test.js` were updated so a schema past version 3 still passes. They do not assume migration 004 is absent.
- `server/db/migrations/004-fsrs-review-state.js` and `review_items` FSRS columns were not edited.
- `npm test` is this repo's quality gate. No lint, type-checker, or coverage command is configured. `# type: ignore` / `# noqa` scan: not applicable (JavaScript, no such comments added).
- Timer start/complete and `run_trace_invariants_validate` / `model_dispatch_conformance_validate` were not run: those scripts are not in this repo, and the orchestrator owns the timer and trace on a dispatched spec.
- Harness: cursor. AC verification used a fresh sub-agent (`composer-2.5-fast`). The Task tool has no readonly flag; the prompt forbade edits. One remediation loop closed the accept-list gap, then a second fresh verifier returned all-PASS. Security review of these files was done by the implementing agent (`self_reviewed: true`).
- Research index: no new research decision. The concept list and limits are ADR-0009 §4 and §6.

---

## Effort Tracking

**Agent session time:** not recorded — timer script absent and owned by the orchestrator
**Spec effort estimate:** 5 hours
