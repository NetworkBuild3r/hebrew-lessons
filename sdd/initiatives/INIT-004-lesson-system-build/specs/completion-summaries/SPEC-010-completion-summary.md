# SPEC-010 Completion Summary — Personal-plan assignee checks

**Traceability:** INIT-004/SPEC-010
**Status:** Complete
**Domain:** service
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `server/plans/personal.js` — `assign` checks the account, then calls `ops.replacePart`. It writes no plan SQL.
- [x] `test/personal-plan.test.js` — temporary database; spy on `replacePart` and `db.run`.
- [x] `server/db/repos/users.js` — `findAccount` selects `id`, `username`, and `status` only. The existing `findByUsername` still selects every column; this module does not call it.

## Acceptance Criteria

- [x] ac-1: audience and assigneeId go through `ops.replacePart`. A spy that does not write shows `personal.js` issued no plan SQL.
- [x] ac-2: a disabled or unknown account returns `assignee_refused`. Revision, document, and `updatedAt` stay as stored.
- [x] ac-3: `audience: "shared"` sets `assigneeId` to null in that same `replacePart`, and `validatePlan` accepts the plan.
- [x] Security: a disabled or missing account cannot be assigned. Revision checks stay inside `ops.replacePart`.

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`) that did **not** author the code — not self-reported.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-010-eval-report.md`](SPEC-010-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` `pass` · security_review=`required` `FAIL` · domain=`service` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `assign` → `ops.replacePart` (`server/plans/personal.js:74-78`). Test `ac-1: audience and assigneeId go through replacePart, and personal.js writes no plan SQL`. Verifier: `node --test test/personal-plan.test.js` → 4 pass, 0 fail, exit 0. |
| ac-2 | PASS | `assignee_refused` before `replacePart` (`server/plans/personal.js:68-69`). Test `ac-2: a disabled or unknown assignee is refused and the plan is unchanged`. |
| ac-3 | PASS | Shared sets both fields in one `replacePart` (`server/plans/personal.js:64-66`). Test `ac-3: shared sets assigneeId null in one operation and validates`; `validatePlan` `ok`. |
| sec ×2 | PASS | Same ac-2 refusal; stale `expectedRevision` returns `revision_conflict` and does not write. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-010`
- **Title:** Assign a personal plan only to an active account, through `ops.replacePart`
- **Files changed:** 3 code files · `personal.js` 81 lines, test 287 lines, `findAccount` added on the users repository
- **Change status:** proposed
- **Reversibility:** branch. Rollback: delete `server/plans/personal.js` and `test/personal-plan.test.js`, and remove `findAccount`. Tests use a temporary database.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/personal-plan.test.js` | 4 | 4 |

**All tests passing:** Yes.

- `node --test test/personal-plan.test.js` → 4 pass, 0 fail, exit 0. The verifier ran the same command and got the same result.
- `npm test` → 461 tests: 459 pass, 0 fail, 2 `todo`. The two `todo` tests are in `test/media-ingest.test.js` (SPEC-008). This spec did not touch that file.
- `node --check` on `server/plans/personal.js`, `server/db/repos/users.js`, and `test/personal-plan.test.js` → exit 0.
- **Coverage (new code):** no coverage tool is configured. The spec's bar is this file plus `npm test`.

`assigneeId` stored by `assign` is the account username. `Learner01` is stored as `learner01`. It is not the integer primary key. SPEC-004 should compare the caller to that username, and should call `assign` rather than `ops.setField` on `assigneeId`.

---

## Gates & Controls

None — branch/reversible only. `gate_actions: []`. No dependency was added. No commit, push, or PR.

---

## Security Review

- **Sensitivity / depth:** required · standard (the spec's Security section says standard; metadata says required) · **method:** `security-review` subagent, model `grok-4.7-high`, scoped to `server/plans/personal.js`, `findAccount`, and `test/personal-plan.test.js`. SAST and secret scan: not run — none configured. Registry steps were not run: `scripts/dev/security_scan_indexer.py` and `sdd/security-reviews/` are not in this repo.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| None in scope | — | — | — | — |

- **Positive observations:** Disabled and unknown accounts share one error, `assignee_refused`, so the caller is not told which case it was. The stored id is the username from the row, not the caller's original spelling. `shared` forces `assigneeId` to null even if the caller sent one. The lookup is parameterized and capped at 64 characters. `personal.js` does not import the database and does not read learner progress.
- **Noted, not counted:** `ops.setField` can still write `audience` and `assigneeId` without this check. SPEC-003 defined those as ordinary fields. Callers that must enforce the check use `assign`. An active username shorter than 8 characters, or outside `[a-z0-9-]`, passes the account check and then fails schema validation inside `ops`, so the plan is still not written.

---

## Files Changed

Provenance is `INIT-004/SPEC-010` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `server/plans/personal.js` | Created — `assign` (81 lines) | `INIT-004/SPEC-010` |
| `test/personal-plan.test.js` | Created — 4 tests (287 lines) | `INIT-004/SPEC-010` |
| `server/db/repos/users.js` | Added `findAccount` (`id`, `username`, `status` only) and its export | `INIT-004/SPEC-010` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-010-service-personal-plan-checks.md` | Status set to `complete`; criteria checked | `INIT-004/SPEC-010` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/completion-summaries/SPEC-010-eval-report.md` | Created — verifier's per-AC verdicts | `INIT-004/SPEC-010` |

---

## Trace Emission Degradation

Timer and trace scripts are not in this repo. This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the trace; `scripts/dev/lib/sdd_trace` is not in this repo | No opening sentinel for SPEC-010 | A Tier-5 reader has no live trace file from this worker. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions were not emitted | Same |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No event for the security review or the AC verifier | Disclosed here and under Deviations |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Scripts absent; orchestrator owns close | No reconciled `trace_complete` from this worker | `run_trace_invariants_validate` and `model_dispatch_conformance_validate` were not run here |

---

## Deviations from the Spec

- **`findAccount` on the users repository.** The deliverable list names `personal.js` and the test. `findByUsername` selects every column, including the password hash. A narrower query was added so this module can meet the technical requirement.
- **`assigneeId` is the username.** The users table has an integer primary key. ADR-0003 requires a string of 8–64 characters from `[a-z0-9-]`. `assign` stores the account username. A short username that is a real active account is refused by schema validation inside `ops`, not by `assignee_refused`.
- **Git:** no commit, push, or PR (the dispatch said not to).
- **Persona:** `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo (`agents_dir_absent`). No persona file was invented. The code follows the spec and `server/plans/` (`ops.js`, `store.js`). `scripts/dev/expertise_state.py` is also absent, so the dispatch-check was not run.
- **Harness (cursor, T2):** the AC verifier was a fresh `generalPurpose` subagent on `composer-2.5-fast`. The Task tool has no readonly flag; the prompt forbade edits. The security review was a `security-review` subagent on `grok-4.7-high`, not an inline self-review. The compensating control remains the initiative-wide Security Review at `complete-initiative`.
- **Learner snapshot:** not built. INIT-006 owns it. This spec did not add `get_learner_snapshot` or an AI-planning route.
- **Quality gates:** no lint, type-check, or coverage tool is configured. `node --check` is clean, and `npm test` is the gate. No suppression comment was added.
- **Research index:** nothing in `sdd/research/README.md` changes the assignee rule already in ADR-0003 and ADR-0005.
- **Side effects of `npm test`:** the same catalog rewrites earlier plan specs recorded. This spec's tests set `LESSONS_DIR` to an empty temp directory.

---

## Effort Tracking

**Agent session time:** not recorded — the timer script is absent and the orchestrator owns it. This dispatch ran on 2026-10-09.
**Spec effort estimate:** 2 hours
