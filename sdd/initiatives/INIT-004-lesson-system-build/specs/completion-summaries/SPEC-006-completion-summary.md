# SPEC-006 Completion Summary — Media asset rows and the AI-planning switch

**Traceability:** INIT-004/SPEC-006
**Status:** Complete
**Domain:** data
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `server/db/migrations/003-media-and-ai-planning.js` (apply and down). Next number confirmed by listing `server/db/migrations/` (`001-initial.js` and `002-plan-store.js` only).
- [x] `server/db/repos/media.js` and `getAiPlanning` / `setAiPlanning` on `server/db/repos/users.js`. No file IO.
- [x] `test/media-switch-store.test.js` on a temporary database (`DATA_DIR` under the OS temp dir).

## Acceptance Criteria

- [x] ac-1: pre-existing users and inserts that omit the column are off. The default is `DEFAULT 0` on the column.
- [x] ac-2: a media row is stored. The column list has no `filename` and no `original_name`.
- [x] ac-3: down removes `users.ai_planning` and drops `media`. `plans` remains. `lesson_progress` counts are unchanged.

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`) that did **not** author the code.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-006-eval-report.md`](SPEC-006-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[db_migration]` `HALT` · security_review=`required` `FAIL` · domain=`data` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: existing users and inserts that omit the column stay off` passed. `003-media-and-ai-planning.js` adds `ai_planning INTEGER NOT NULL DEFAULT 0` and does not `UPDATE users`. `node --test test/media-switch-store.test.js` → 5 pass, 0 fail, exit 0. |
| ac-2 | PASS | `ac-2: media can be stored and the column list has no client filename` passed in that same run. `PRAGMA table_info(media)` has `stored_name` and neither `filename` nor `original_name`. |
| ac-3 | PASS | `ac-3: down drops media and ai_planning and keeps plans and lesson progress` passed in that same run. Counts of `lesson_progress` and `plans` match before and after `down`. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-006`
- **Title:** Add media rows and a default-off AI-planning switch on users
- **Files changed:** 6 code files · additions +721 · deletions −6 (plus this summary, the eval report, and the spec status)
- **Change status:** proposed
- **Reversibility:** one_way (migration file). Not applied to `data/hebrew.sqlite`. Rollback is `down`, which drops `media` and removes `users.ai_planning`.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/media-switch-store.test.js` | 5 | 5 |
| `test/db.test.js` | 0 (assertion extended for version 3 and the `media` table) | existing suite |
| `test/plan-store.test.js` | 0 (version list after `002` down now keeps version 3) | existing suite |

**All tests passing:** Yes · `node --test test/media-switch-store.test.js` → 5/5, exit 0 · `npm test` → 341/341, exit 0 · `npm run audio:verify` → 65 recordings, exit 0 · **Coverage (new code):** no coverage tool is configured; the spec's bar is this file plus `npm test`

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| db_migration | one_way | yes — operator "OK finish init-004", scoped to writing the migration and tests | `node --test test/media-switch-store.test.js`, `npm test`, `npm run audio:verify` | Not applied to a shared or production `data/hebrew.sqlite`. Production apply waits for SPEC-020. Rollback: `down` drops `media`, drops `users.ai_planning`, and deletes schema version 3. |

---

## Security Review

- **Sensitivity / depth:** required · thorough · **method:** `security-review` subagent (model `claude-opus-5-5-max`), scoped to this spec's files. SAST and secret scan: not run — none configured.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| — | — | — | — | — |

- **Positive observations:** `ai_planning` is `INTEGER NOT NULL DEFAULT 0` with `CHECK (ai_planning IN (0, 1))`. `users.create` does not name the column. Media SQL uses bound parameters. `rejectClientFilename` refuses upload-name keys, and the table has no `filename` or `original_name` column. `stored_name` is a hex stem plus `.png`. Prompt text is stored on `media` and is not inserted into `audit_log`. `setAiPlanning` returns `{ previous, next }` and does not write the audit row. Fixtures use `alice`, `bob`, `carol`, and the placeholder hash `hash`. Tests set `DATA_DIR` to a temp directory and do not call a provider. `publicUser` still omits `ai_planning`, so this column is not added to `GET /api/me`.

---

## Files Changed

Provenance is `INIT-004/SPEC-006` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `server/db/migrations/003-media-and-ai-planning.js` | Created — `media` table, `users.ai_planning`, `up` and `down` (81 lines) | `INIT-004/SPEC-006` |
| `server/db/repos/media.js` | Created — insert, update, get (172 lines) | `INIT-004/SPEC-006` |
| `server/db/repos/users.js` | Modified — `getAiPlanning` and `setAiPlanning` (+54 / −1) | `INIT-004/SPEC-006` |
| `test/media-switch-store.test.js` | Created — temporary-database tests (393 lines) | `INIT-004/SPEC-006` |
| `test/db.test.js` | Modified — expect migration version 3 and the `media` table (+17 / −3) | `INIT-004/SPEC-006` |
| `test/plan-store.test.js` | Modified — after `002` down, version 3 stays; remigrate expects three versions | `INIT-004/SPEC-006` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-006-data-media-and-ai-planning.md` | Status set to `complete`; criteria checked | `INIT-004/SPEC-006` |

---

## Trace Emission Degradation

Timer and trace scripts are not in this repo. This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the sentinel; `scripts/dev/lib/sdd_trace` is not in this repo | No opening sentinel for SPEC-006 | A Tier-5 reader has no live trace file. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions | Same |
| Phase 1.5 | `gate`, `one_way_action` | Orchestrator owns gate emission; approval was already on the dispatch ("OK finish init-004") | No gate event in a trace file | The approval is recorded in this summary, not in `.sdd-trace` |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No harness-fallback event | Disclosed here instead |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Orchestrator owns close; scripts absent | No reconciled `trace_complete` | Tier-5 and model-dispatch validators were not run (scripts absent) |

---

## Deviations from the Spec

- Git commit, push, and PR were not performed. The dispatch said to skip them, and the operator did not ask to commit.
- Persona `.claude/agents/database-architect/AGENT.md` is not in this repo. No persona file was invented. Implementation followed the spec and `server/db/` (bound parameters, ISO timestamps, `schema_migrations`).
- `test/db.test.js` and `test/plan-store.test.js` were updated. `db.migrate()` applies every numbered file, so assertions that stop at version 2 fail once `003-media-and-ai-planning.js` exists. `002`'s down still deletes only version 2, so version 3 remains until this migration's own down.
- `stored_name` must be a hex stem plus `.png` (length 5–68). That is the stored name from ADR-0004 §5, not the client filename. `prompt_text` is limited to 4000 characters. The spec said the prompt is nullable and did not set a maximum.
- `npm test` is this repo's quality gate. No lint, type-checker, or coverage command is configured. `# type: ignore` / `# noqa` scan: not applicable (JavaScript, no such comments added).
- Timer start/complete and `run_trace_invariants_validate` / `model_dispatch_conformance_validate` were not run: those scripts are not in this repo, and the orchestrator owns the timer and trace on a dispatched spec.
- Harness: cursor. AC verification used a fresh sub-agent (`composer-2.5-fast`). Security review used a fresh `security-review` sub-agent (`claude-opus-5-5-max`). The Task tool has no readonly flag; both prompts forbade edits.
- Discipline profile is `software` (initiative field). Domain patterns followed the repo's SQLite migration style.
- Research index: ADR-0004 already applies `RSCH-005` for image approval. This spec stores the row that later approval will use. No new research decision.
- The test process was Node v26.11.0, which is the `node` on this machine. The project standard is Node 22. The code uses `node:sqlite` only.

---

## Effort Tracking

**Agent session time:** not recorded — timer script absent and owned by the orchestrator
**Spec effort estimate:** 4 hours
