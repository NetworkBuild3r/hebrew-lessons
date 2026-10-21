# SPEC-011 Completion Summary — Media, generation, and approval routes

**Traceability:** INIT-004/SPEC-011
**Status:** Complete
**Domain:** api
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] Routes on the existing API router (`server/api/media-routes.js`, mounted from `server/web.js`)
- [x] Tests: `test/media-routes.test.js`

## Acceptance Criteria

- [x] ac-1 (REQ-006): the matrix cells for upload, fetch, and approval match the table for signed-out, user, and admin. A user fetch of an unreferenced id is 404.
- [x] ac-2 (REQ-007): `image-plan` returns the dollar estimate and the fake transport records zero calls. `POST /api/admin/images` without confirm records zero calls.
- [x] ac-3 (REQ-006): publishing a plan to `everyone` while an introduction word references a `pending` media id is refused. The same reference on a `hidden` lesson is allowed.

## AC Verification (independent)

Verified by a fresh sub-agent ([Verify SPEC-011 criteria](a1053405-e20a-4fff-a77a-76201b2045c4), model `composer-2.5-fast`) that did **not** author the code — not self-reported.
Overall: **PASS** · remediation loops: 0.
**Full eval report:** [`SPEC-011-eval-report.md`](SPEC-011-eval-report.md)

**Verification tier:** `full` · **pre-flight:** not present on the dispatch, so `full` (fail-closed) · gate_actions=`[]` `pass` · security_review=`required` `FAIL` · domain=`api` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | Matrix tests in `test/media-routes.test.js`. GET checks lesson visibility before `sendStoredImage`. Verifier re-ran `node --test test/media-routes.test.js` → 7 pass, 0 fail, exit 0. |
| ac-2 | PASS | Dry-run returns `0.011` / `0.022` with zero transport calls. Generate without `confirm: true` records zero calls. A confirmed generate records one fake call and does not call `https.request`. |
| ac-3 | PASS | `everyone` and `selected` publish return `pending_media` and leave the snapshot unset. A hidden lesson with the same pending id publishes. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-011`
- **Title:** Browser routes for media upload, fetch, approval, delete, and image generation
- **Files changed:** 6 spec-owned paths (see table). `server/web.js` and the signed-in test helper already had other uncommitted work; this spec added the media mount, the multipart CSRF exception, and the media module cache entries.
- **Change status:** proposed
- **Reversibility:** branch

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| `test/media-routes.test.js` | 7 | 7 |

**All tests passing:** Yes · `node --test test/media-routes.test.js` → 7 pass, 0 fail. `npm test` → 546 pass, 0 fail (run before the delete CSRF assertion; that assertion then passed in the scoped file).
**Coverage (new code):** No coverage tool is configured. The bar is this file plus `npm test`, per the spec.

---

## Gates & Controls

None — branch/reversible only. `gate_actions` is empty. Commit, push, and PR were not requested and were not performed.

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ----------- | ------------- | --------- | --------------- | ----- |
| — | branch | — | `node --test test/media-routes.test.js`, `npm test` | Rollback is unmounting the routes. Archived files stay archived. |

---

## Security Review

- **Sensitivity / depth:** required · thorough · **method:** security-review subagent ([Security Review](317d5b91-51c7-45be-bfc4-34668efef9c1), model `grok-4.7-high`)
- **Harness:** cursor T2 (`Task`, independent agent). `self_reviewed: false`
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 2 (informational)
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| MCP bearer plus an admin cookie on approval is blocked in code and not separately asserted | Low | API1 / authz | `server/api/media-routes.js` `refuseMcpApproval` | accepted — bearer-without-cookie is tested; the handler returns 403 before the session handler |
| A personal plan's assignee can fetch a pending image on a hidden lesson | Low | API1 / authz | `userMayFetch` | accepted — matches the publish rule (pending blocks `everyone` and `selected` only) and the personal-plan read rule |

- **Positive observations:** Upload bytes go through ingest. The client filename is not stored. JSON and multipart bodies reject `codec`, `now`, `actor`, and `transport`. Audit `detail` is null. Responses are scrubbed for `OPENAI_API_KEY`. Fetch checks lesson visibility before streaming. Approval with an MCP bearer is 403.

---

## Files Changed

Provenance is `INIT-004/SPEC-011` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `server/api/media-routes.js` | Created — upload, fetch, approval, delete, image-plan, and generate | `INIT-004/SPEC-011` |
| `test/media-routes.test.js` | Created — matrix, dry-run, publish refusal, CSRF, oversize, secret scrub | `INIT-004/SPEC-011` |
| `server/plans/publish.js` | Modified — refuse publish when an everyone or selected lesson references pending media | `INIT-004/SPEC-011` |
| `server/plans/store.js` | Modified — `listSnapshots` for fetch visibility | `INIT-004/SPEC-011` |
| `server/web.js` | Modified — mount the media routes; `POST /api/media` accepts multipart and still requires Origin | `INIT-004/SPEC-011` |
| `test/helpers/signed-in-server.js` | Modified — drop cached media modules between harness starts | `INIT-004/SPEC-011` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-011-api-media-generation-approval.md` | Status set to `complete`; criteria checked | `INIT-004/SPEC-011` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 | `trace_open` | Orchestrator owns the sentinel. `scripts/dev/lib/sdd_trace` is not in this repo | No opening sentinel for SPEC-011 | A Tier-5 reader has no live trace file. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions | Same |
| Phase 5 | `dispatch`, `trace_complete`, `spec_lifecycle` | Same, and this dispatch skipped commit | No closing sentinel | The invariants check and the model-dispatch check were not run here. The orchestrator owns them on return. |

---

## Deviations from the Spec

- `PATCH /api/admin/users/:id/ai-planning` was not added. INIT-006 owns it.
- `mcp/tools.js` was not edited. SPEC-012 owns the MCP tools. The HTTP approval route still returns 403 when the caller presents the MCP bearer, which is this spec's matrix cell.
- Admin fetch of a deleted id is 404. The row is kept and the live file is gone. `sendStoredImage` returns false, per the SPEC-008 adapter note.
- The persona file `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo. Work followed `server/api/plan-routes.js` and this spec. No replacement persona was manufactured.
- Commit, push, and PR were skipped. The dispatch said not to commit.
- `server/web.js` and `test/helpers/signed-in-server.js` already contained other uncommitted edits (SPA shell, plan-route cache entries). This spec did not revert them.

---

## Effort Tracking

**Agent session time:** not recorded here — the orchestrator owns the timer
**Spec effort estimate:** 4 hours
