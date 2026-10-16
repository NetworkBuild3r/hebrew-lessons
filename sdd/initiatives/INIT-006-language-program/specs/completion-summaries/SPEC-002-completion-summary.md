# SPEC-002 Completion Summary — FSRS columns on review rows and graded events

**Traceability:** INIT-006/SPEC-002
**Status:** Complete
**Domain:** data
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `server/db/migrations/004-fsrs-review-state.js` (apply and down). Number assigned by the orchestrator. Existing files were `001-initial.js`, `002-plan-store.js`, and `003-media-and-ai-planning.js`. `005-lexicon-library-mastery.js` belongs to SPEC-003 and was not edited.
- [x] `server/db/repos/review.js` and `server/db/repos/events.js` read and write the new columns with an explicit column list. `setSuspended(userId, word, bool)` updates both directions.
- [x] `test/fsrs-migration.test.js` on a temporary database.

## Acceptance Criteria

- [x] ac-1: stage counts from the pre-migration ladder match the ADR-0009 stage mapping after backfill, per direction.
- [x] ac-2: `due`, `step`, `streak`, `lapses`, `last_result`, `last_at`, and `sources` are unchanged.
- [x] ac-3: `down` restores the original columns and rows; `up` after `down` is clean.
- [x] ac-4: grade 0 or 5, and a kind other than `word` or `form`, fail CHECK.

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`) that did **not** author the code.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-002-eval-report.md`](SPEC-002-eval-report.md)

**Verification tier:** `full` · **pre-flight:** dispatch did not include an AC-verification-tier line, so the tier is full · gate_actions=`[db_migration]` `HALT` · security_review=`standard` `FAIL` · domain=`data` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1 and ac-2: stage counts match and ladder values stay byte-identical` passed. `node --test test/fsrs-migration.test.js` → 4 pass, 0 fail, exit 0. |
| ac-2 | PASS | Same test. `assertLadderUntouched` compares every ladder column. |
| ac-3 | PASS | `ac-3: down restores the original columns and up after down is clean` passed in that run. |
| ac-4 | PASS | `ac-4: grade outside 1–4 and kind other than word or form fail CHECK` passed in that run. |

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-002`
- **Title:** Add reversible FSRS columns and a backfill that leaves due dates and ladder stages alone.
- **Files changed:** 5 · migration and test are new · repository edits +84/−7 and +33/−5
- **Change status:** proposed
- **Reversibility:** one_way (schema migration). `down(db)` restores the original column sets. Not applied to `data/hebrew.sqlite`.

---

## Tests

| Test file | New tests | Total in file |
| --------- | --------- | ------------- |
| `test/fsrs-migration.test.js` | 4 | 4 |

**All tests passing:** Yes · `node --test test/fsrs-migration.test.js` → 4 pass, 0 fail, exit 0. `npm test` → 583 pass, 0 fail, exit 0.
**Coverage (new code):** Not measured. The configured test command does not collect coverage. The spec's pass condition is exit 0 on the migration tests, which was met.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ----------- | ------------- | --------- | --------------- | ----- |
| db_migration | one_way | yes — writing the migration and tests only | `node --test test/fsrs-migration.test.js` (4 pass); `npm test` (583 pass) | Operator gate from the dispatch. `down(db)` is the rollback. Not applied to `data/hebrew.sqlite`. Production apply waits for SPEC-020. |

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** implementer review of the changed files (`self_reviewed: true`). A separate security-review subagent was not dispatched.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | — |

- **Positive observations:** Backfill and repository writes use bound parameters. `ALTER` / `DROP COLUMN` statements are literals in this file. `setSuspended` is scoped by `user_id` and `word` and hits both directions. CHECK constraints reject illegal grades, kinds, `fsrs_state`, `last_grade`, and `suspended`. `down` keeps original row values. Tests stamp `data/hebrew.sqlite` and use a temp directory. No new dependency and no secrets.

---

## Files Changed

Provenance is `INIT-006/SPEC-002` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `server/db/migrations/004-fsrs-review-state.js` | Created — `up` / `down` / backfill (288 lines) | `INIT-006/SPEC-002` |
| `server/db/repos/review.js` | Modified — explicit column list, FSRS fields on upsert, `setSuspended` | `INIT-006/SPEC-002` |
| `server/db/repos/events.js` | Modified — explicit column list; `grade`, `kind`, `lemma` | `INIT-006/SPEC-002` |
| `test/fsrs-migration.test.js` | Created — ac-1 through ac-4 on a temp database (730 lines) | `INIT-006/SPEC-002` |
| `test/db.test.js` | Modified — drop the require cache for `004-fsrs-review-state.js` and assert version 4's filename. Version 5 and the lexicon tables in this file are SPEC-003's. | `INIT-006/SPEC-002` |

---

## Trace Emission Degradation

Dispatched sub-agent. The orchestrator owns `open_trace` / `close_trace` and the timer (execute-spec Phase 0 step 2 and Phase 5 step 1). This agent did not call them.

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 sentinel | `trace_open`, `trace_complete` | Orchestrator owns the sentinel for a dispatched spec | This agent wrote no `.sdd-trace/SPEC-002.jsonl` events | The orchestrator must open and close the trace or a Tier-5 check has nothing from this agent to read |

---

## Deviations from the Spec

- Persona `.claude/agents/database-architect/AGENT.md` is not in this repo. No persona file was invented. Implementation followed the spec, ADR-0009 §2, and `server/db/` (bound parameters, `schema_migrations`, `up` / `down`).
- Commit, push, and PR were skipped. The dispatch said to skip them.
- `db_migration` was approved for the migration file and tests only. `npm run db:migrate` was not run against `data/hebrew.sqlite`.
- ADR-0009 §2 sets `fsrs_state` to `learning` when a never-passed row has lapses, and `new` when it does not. The spec's one-line backfill sentence says `new` for every never-passed row. The migration follows the ADR paragraph, which the spec calls the contract. Journey stage stays `new` either way, so ac-1 counts still match. A step above 4 is given stability 30 so a passed row that `metrics()` already calls long-term stays long-term.
- `metrics()` is invoked after `up`. Before `up` the new columns do not exist, and the repository selects them. The test computes the pre-migration counts with the same ever-passed test `metrics()` uses, then asserts `metrics()` after `up` is unchanged.
- Timer was not started or completed here. The orchestrator owns it.
- `005-lexicon-library-mastery.js` and lexicon tables were not edited.

---

## Effort Tracking

**Agent session time:** not recorded here — the orchestrator owns the timer
**Spec effort estimate:** 4 hours
