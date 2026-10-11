# SPEC-004 Completion Summary — Plan HTTP routes for admin and learner

**Traceability:** INIT-004/SPEC-004
**Status:** Complete
**Domain:** api
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `server/api/plan-routes.js` — mounted from `createRouter()` in `server/web.js`. Writes and the working-copy read require an admin session. `GET /api/plans/:id/published` requires a session and returns the snapshot only when ADR-0003 §7 allows it.
- [x] `test/plan-routes.test.js` — HTTP integration on the existing signed-in server. Temporary database, two extra users, one personal plan, one `selected` plan. No ElevenLabs or OpenAI calls.

## Acceptance Criteria

- [x] ac-1: each write route calls one export (`replacePart`, `insertBlock`, `moveBlock`, `removeBlock`, `setField`, `publishPlan`). The spy sees that name and no other.
- [x] ac-2: `expectedRevision` 99 on revision 1 returns HTTP 409 `{ "error": "revision_conflict", "revision": 1 }`. The stored document, revision, and `updatedAt` stay put.
- [x] ac-3: a stranger and an admin who are not `assigneeId` receive 404 for a personal snapshot. A `selected` plan 404s until every lesson id is granted. The assignee receives the snapshot. Signed-out is 401. Draft, hidden, and a missing id are 404. After a later edit, the learner still receives the previous snapshot.
- [x] ac-4: a learner receives 403 `forbidden` on all six write routes, and the exports are not called. `GET /api/admin/users` is unchanged. `npm test` exits 0.

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`, [Verify SPEC-004 criteria](391df52d-43c1-450b-a55d-4451159515d8)) that did **not** author the code — not self-reported.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-004-eval-report.md`](SPEC-004-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` `pass` · security_review=`required` `FAIL` · domain=`api` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | Property calls in `server/api/plan-routes.js`. Test `ac-1: each write route calls only its function`. Verifier: `node --test test/plan-routes.test.js` → 9 pass, 0 fail, exit 0. |
| ac-2 | PASS | 409 body is exactly `revision_conflict` plus the stored revision. Test `ac-2: a stale revision is 409 revision_conflict and the document is unchanged`. |
| ac-3 | PASS | `mayReadPublished` plus the two ac-3 tests. 404 text has no `newWords` or `assigneeId`. |
| ac-4 | PASS | `requireAdmin` on every write. Test `ac-4`. Isolated `npm test` → 474 pass, 0 fail, 2 todo. |
| sec ×3 | PASS | Admin gate, generic 404, and the existing `handleApi` CSRF/Origin check. Test `security: a write without Origin is the existing CSRF 403`. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-004`
- **Title:** Plan HTTP API for admin edits and the learner snapshot
- **Files changed:** 4 code files (`server/api/plan-routes.js`, `server/web.js` mount, `test/plan-routes.test.js`, test-harness module reload) plus this summary and the eval report
- **Change status:** proposed
- **Reversibility:** branch. Rollback: unmount `planRoutes.register` and delete `server/api/plan-routes.js` and `test/plan-routes.test.js`. No snapshot has to be served. No migration.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/plan-routes.test.js` | 9 | 9 |

**All tests passing:** Yes.

- `node --check` on `server/api/plan-routes.js`, `server/web.js`, and `test/plan-routes.test.js` → exit 0.
- `node --test test/plan-routes.test.js` → 9 pass, 0 fail, exit 0. The verifier ran the same command and got the same result.
- `npm test` → 476 tests: 474 pass, 0 fail, 2 `todo`. The two `todo` tests are in `test/media-ingest.test.js` (SPEC-008). `web-gating.test.js` and `http-security.test.js` passed in that run.
- **Coverage (new code):** no coverage tool is configured. The spec's bar is this file plus `npm test`.

---

## Gates & Controls

None — branch/reversible only. `gate_actions: []`. No dependency was added. No commit, push, or PR. No blocker.

---

## Security Review

- **Sensitivity / depth:** required · thorough · **method:** inline review by the implementing agent (`self_reviewed: true`) of `server/api/plan-routes.js`, the `server/web.js` mount, and `test/plan-routes.test.js`. Focus: authz, personal data, mass assignment. The Cursor Task tool was used for the AC verifier, not for a second security-review model. SAST and secret scan: not run — none configured. Registry steps were not run: `scripts/dev/security_scan_indexer.py` and `sdd/security-reviews/` are not in this repo.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| None in scope | — | — | — | — |

- **Positive observations:** Writes and `GET .../parts` go through `requireSession` then `requireAdmin` before the handler. The published route never calls `readPart`. Access uses the snapshot's `audience`, `assigneeId`, `status`, and lesson `visibility`, so a draft edit of the working copy does not publish itself and does not change who can read. Personal access compares `assigneeId` to `user.username` only, which is what `server/plans/personal.js` stores; a display name that copies the username does not match. A shared plan with any `hidden` lesson, any unknown visibility, or a `selected` lesson missing a grant is 404 for the whole snapshot. Denied and missing ids share one JSON error and do not include plan text. Unknown JSON keys are rejected before any export is called, and the object passed in is built from the allowed keys. Body size is capped at 1 MiB. CSRF and Origin stay in `server/web.js` `handleApi`. Responses use `Cache-Control: no-store`. SQL stays inside `server/plans/store.js`.
- **Noted, not counted:** `PUT` and `PATCH` still call `replacePart` and `setField`, so an admin can change `audience` or `assigneeId` without `personal.assign`. SPEC-004 requires those routes to call those functions and no other edit implementation. Account checks on assign stay in SPEC-010.

---

## Files Changed

Provenance is `INIT-004/SPEC-004` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `server/api/plan-routes.js` | Created — plan HTTP adapters | `INIT-004/SPEC-004` |
| `server/web.js` | Mount `planRoutes.register` on the existing router | `INIT-004/SPEC-004` |
| `test/plan-routes.test.js` | Created — 9 HTTP tests | `INIT-004/SPEC-004` |
| `test/helpers/signed-in-server.js` | Drop plan and router modules from the require cache on start/stop | `INIT-004/SPEC-004` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-004-api-plan-routes.md` | Status set to `complete`; criteria checked | `INIT-004/SPEC-004` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/completion-summaries/SPEC-004-eval-report.md` | Created — verifier's per-AC verdicts | `INIT-004/SPEC-004` |

---

## Trace Emission Degradation

Timer and trace scripts are not in this repo. This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the trace; `scripts/dev/lib/sdd_trace` is not in this repo | No opening sentinel for SPEC-004 | A Tier-5 reader has no live trace file from this worker. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions were not emitted | Same |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No event for the inline security review or the AC verifier | Disclosed here and under Deviations |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Scripts absent; orchestrator owns close | No reconciled `trace_complete` from this worker | `run_trace_invariants_validate` and `model_dispatch_conformance_validate` were not run here |

---

## Deviations from the Spec

- **404 body.** Denied reads use the existing `sendJsonError` envelope `{ "error": "Not found", "code": "not_found" }`, not a zero-byte body. The verification pass condition is that the body does not contain `newWords` or `assigneeId`. A missing id and a hidden plan return the same text.
- **`assigneeId` is the username.** The users table has an integer primary key. `isAssignee` compares the snapshot's `assigneeId` to `user.username`, matching SPEC-010's `assign`.
- **Learner read after a later edit.** A working copy that has returned to `draft` still serves the last snapshot when that snapshot is `published` and the visibility rules allow it. ADR-0003 §7 says learners keep reading the snapshot until the next publish. A plan with no snapshot is 404.
- **Git:** no commit, push, or PR (the dispatch said not to).
- **Persona:** `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo (`agents_dir_absent`). No persona file was invented. The handlers follow `server/api/admin-routes.js`, `server/api/router.js`, and `server/http/security.js`. `scripts/dev/expertise_state.py` is also absent, so the dispatch-check was not run.
- **Harness:** Cursor. The AC verifier was a Task sub-agent (T2). Security review was inline (`self_reviewed: true`) because this dispatch did not start a second review model.

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4.5 AC verifier | `subagent_dispatch` | T2 | Fresh Task agent, model `composer-2.5-fast`, read-only by prompt. All AC PASS. |
| Phase 4 security review | `subagent_dispatch` | T3 | Inline review, `self_reviewed: true`. Critical 0, High 0. |
