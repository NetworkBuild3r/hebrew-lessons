# SPEC-001 Completion Summary — Plan store: working copy, revision, and snapshot

**Traceability:** INIT-004/SPEC-001
**Status:** Complete
**Domain:** data
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `server/db/migrations/002-plan-store.js` (apply and down). Next number confirmed by listing `server/db/migrations/` (only `001-initial.js` existed).
- [x] `server/plans/store.js` — load/save working copy, load/save snapshot. The only plan SQL.
- [x] `test/plan-store.test.js` on a temporary database (`DATA_DIR` under the OS temp dir).

## Acceptance Criteria

- [x] ac-1: draft at revision 1, null snapshot, same JSON bytes on read.
- [x] ac-2: down drops `plans` only; `users` and `lesson_progress` counts match; `review_items` and `audit_log` remain.
- [x] ac-3: migration source has no lesson path and does not open `LESSONS_DIR`.

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`) that did **not** author the code.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-001-eval-report.md`](SPEC-001-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[db_migration]` `HALT` · security_review=`standard` `FAIL` · domain=`data` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: inserts a draft…` passed. `node --test test/plan-store.test.js` → 9 pass, 0 fail, exit 0. |
| ac-2 | PASS | `ac-2: down drops only plans…` passed in that same run. Counts of `users` and `lesson_progress` equal before and after. |
| ac-3 | PASS | `ac-3: the migration writes no audio paths…` passed in that same run. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-001`
- **Title:** Add the plan document table and the only module that reads and writes it
- **Files changed:** 4 code files · additions +614 · deletions −3 (plus this summary, the eval report, and the spec status)
- **Change status:** proposed
- **Reversibility:** one_way (migration file). Not applied to `data/hebrew.sqlite`. Rollback is `down`, which drops `plans` only.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/plan-store.test.js` | 9 | 9 |
| `test/db.test.js` | 0 (assertion extended for version 2 and the `plans` table) | existing suite |

**All tests passing:** Yes · `node --test test/plan-store.test.js` → 9/9 · `npm test` → 327/327, exit 0 · `npm run audio:verify` → 65 recordings, exit 0 · **Coverage (new code):** no coverage tool is configured; the spec's bar is this file plus the existing suite

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| db_migration | one_way | yes — operator "OK finish init-004", scoped to writing the migration and tests | `node --test test/plan-store.test.js`, `npm test`, `npm run audio:verify` | Not applied to a shared or production `data/hebrew.sqlite`. Production apply waits for SPEC-020. Rollback: `down` drops `plans` and deletes schema version 2. |

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** scoped review of this spec's files (migration safety, bound parameters, fixtures). SAST and secret scan: not run — none configured. Reviewer judgment: implementing agent, `self_reviewed: true` (see Harness note under Deviations). The independent AC verifier also checked the two security acceptance criteria and returned PASS.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| — | — | — | — | — |

- **Positive observations:** Plan SQL uses bound parameters. `assignee_id` is an opaque `[a-z0-9-]` id of length 8–64, not a display name and not a foreign key to `users.id`. `down` names only `plans`. Fixtures are a minimal JSON object and the placeholder hash already used by `test/db.test.js`. The store returns JSON text; it does not parse or evaluate it. Tests set `DATA_DIR` to a temp directory.

---

## Files Changed

Provenance is `INIT-004/SPEC-001` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `server/db/migrations/002-plan-store.js` | Created — `plans` table, `up` and `down` (73 lines) | `INIT-004/SPEC-001` |
| `server/plans/store.js` | Created — load/save working copy and snapshot (188 lines) | `INIT-004/SPEC-001` |
| `test/plan-store.test.js` | Created — temporary-database tests (342 lines) | `INIT-004/SPEC-001` |
| `test/db.test.js` | Modified — expect migration version 2 and the `plans` table (+11 / −3) | `INIT-004/SPEC-001` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-001-data-plan-store.md` | Status set to `complete`; criteria checked | `INIT-004/SPEC-001` |

---

## Trace Emission Degradation

Timer and trace scripts are not in this repo. This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the sentinel; `scripts/dev/lib/sdd_trace` is not in this repo | No opening sentinel for SPEC-001 | A Tier-5 reader has no live trace file. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions | Same |
| Phase 1.5 | `gate`, `one_way_action` | Orchestrator owns gate emission; approval was already on the dispatch ("OK finish init-004") | No gate event in a trace file | The approval is recorded in this summary, not in `.sdd-trace` |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No harness-fallback event | Disclosed here instead |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Orchestrator owns close; scripts absent | No reconciled `trace_complete` | Tier-5 and model-dispatch validators were not run (scripts absent) |

---

## Deviations from the Spec

- Git commit, push, and PR were not performed. The dispatch said the Git & Issue-Tracker guard is active and the operator did not ask to commit.
- Persona `.claude/agents/database-architect/AGENT.md` is not in this repo. No persona file was invented. Implementation followed the spec and `server/db/` (bound parameters, ISO timestamps, `schema_migrations`).
- `test/db.test.js` was updated in this change. `db.migrate()` applies every numbered file, so the old assertion of exactly one `schema_migrations` row would fail once `002-plan-store.js` exists.
- One table, `plans`, holds the working-copy text and the nullable snapshot. The acceptance criteria are about that document and a down path that drops only the new table.
- `npm test` is this repo's quality gate. No lint, type-checker, or coverage command is configured. `# type: ignore` / `# noqa` scan: not applicable (JavaScript, no such comments added).
- Timer start/complete and `run_trace_invariants_validate` / `model_dispatch_conformance_validate` were not run: those scripts are not in this repo, and the orchestrator owns the timer and trace on a dispatched spec.
- Harness: cursor. AC verification used a fresh sub-agent (`composer-2.5-fast`), which is the cursor tier for an independent verifier. The Task tool has no readonly flag; the prompt forbade edits. Security review of these files was done by the implementing agent (`self_reviewed: true`) so the review stayed on this spec's files rather than the rest of the dirty checkout.
- Research index: `RSCH-004` is already applied in ADR-0003, which this spec follows. No new research decision.

---

## Effort Tracking

**Agent session time:** not recorded — timer script absent and owned by the orchestrator
**Spec effort estimate:** 4 hours
