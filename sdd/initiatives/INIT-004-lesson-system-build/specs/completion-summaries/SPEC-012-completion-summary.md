# SPEC-012 Completion Summary — MCP tools for media and image generation

**Traceability:** INIT-004/SPEC-012
**Status:** Complete
**Domain:** api
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] Tools added in `mcp/tools.js` — `upload_media`, `plan_image`, and `generate_image`. Each calls `ingest.ingestImage` or `images.planImage` / `images.generateImage`. There is no second ingest and no second image client.
- [x] Tests: `test/media-generation-mcp.test.js` — fake transport, temporary database, no live OpenAI call.

## Acceptance Criteria

- [x] ac-1: `upload_media`, `plan_image`, and `generate_image` are registered. The seven SPEC-005 plan tools and the eight legacy tools stay registered.
- [x] ac-2: `plan_image` does not call the transport. `generate_image` without `confirm: true` does not call the transport. Both refuse when `MCP_AUTH_TOKEN` is unset.
- [x] ac-3: no registered tool name approves media. `POST /api/admin/media/:id/approval` with only the MCP token returns 403, and the row stays `pending`.
- [x] Security: a bad or missing token returns no image body. The tool list has no approve tool. Errors do not contain the API key.

## AC Verification (independent)

Verified by a fresh sub-agent ([Verify SPEC-012 criteria](f5441a64-f220-4bcc-bede-fb6ec506e726), model `composer-2.5-fast`) that did **not** author the code.
Overall: **PASS** · remediation loops: 0.
**Full eval report:** [`SPEC-012-eval-report.md`](SPEC-012-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` `pass` · security_review=`required` `FAIL` · domain=`api` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: media tools and the SPEC-005 plan tools are registered`. Handlers in `mcp/tools.js`. Verifier: `node --test test/media-generation-mcp.test.js test/plan-mcp.test.js` → 18 pass, 0 fail, exit 0. |
| ac-2 | PASS | Dry-run and confirm-less generate keep the transport count at 0. Unset token refuses all three tools with no image body. |
| ac-3 | PASS | No approve tool. HTTP approval with only the MCP bearer is 403 and the media row stays `pending`. |
| security ×3 | PASS | Wrong bearer is 401 with no image. Key-bearing ingest errors are scrubbed. `get_learner_snapshot` is not registered. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-012`
- **Title:** Register MCP media tools that call the existing ingest and image services
- **Files changed:** `mcp/tools.js`, `test/media-generation-mcp.test.js`, and a pre-handler on the approval route in `server/api/media-routes.js`, plus this summary, the eval report, and the spec status
- **Change status:** proposed — not committed (dispatch said no commit)
- **Reversibility:** branch. Rollback: unregister the three tools. Plan tools from SPEC-005 stay. Remove `refuseMcpApproval` from the approval route if that pre-handler is reverted with them.

---

## Tests

| Test file | New tests | Total in file |
| --------- | --------- | ------------- |
| `test/media-generation-mcp.test.js` | 12 | 12 |
| `test/plan-mcp.test.js` | 0 | 6 (unchanged, re-run) |
| `test/media-routes.test.js` | 0 | 7 (unchanged, re-run after the approval pre-handler) |

**All tests passing:** Yes · `node --test test/media-generation-mcp.test.js test/plan-mcp.test.js` → 18 pass, 0 fail, exit 0. `npm test` → 546 pass, 0 fail, exit 0.
**Coverage (new code):** not measurable. This repository has no coverage tool. The bar is this file plus `npm test`.

---

## Gates & Controls

None — branch/reversible only. `gate_actions: []`. Autonomy `auto_tests`. No database migration, no dependency add, no commit, no push, no pull request.

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ----------- | ------------- | --------- | --------------- | ----- |
| — | branch | — | `node --test test/media-generation-mcp.test.js test/plan-mcp.test.js` and `npm test` both exit 0 | Rollback is unregistering the three tools |

---

## Security Review

- **Sensitivity / depth:** required · thorough · **method:** security-review subagent (Cursor T2), model `composer-2.5-fast`
- **Reviewer:** [Security Review](3d778827-a441-46fc-93ef-21c890f4aae6)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 2 (informational)
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| HTTP JSON scrubbing redacts `OPENAI_API_KEY` and does not also redact `MCP_AUTH_TOKEN`. The approval refusal is a generic 403 and does not echo the token. | Low | secrets in responses | `server/api/media-routes.js` response helper | Noted. Not remediated here; the MCP tool replies scrub both. |
| Tool handlers check that `MCP_AUTH_TOKEN` is configured. The bearer compare stays in `mcp/http.js`, which runs before the handler. | Low | authn | `mcp/tools.js` `requireMcpAuthToken` | Accepted. Same split as the plan tools. Direct handler calls are the test seam. |

- **Positive observations:** The actor stored on upload and generate is always `mcp`. A caller-supplied actor, model, filename, or filesystem path is ignored. `confirm` must be boolean `true`. Replies omit image bytes and the prompt. Successful upload and generate write an audit row with `detail` null. A valid MCP credential on the approval route is 403 even if an admin cookie is also present.

---

## Files Changed

Provenance is `INIT-004/SPEC-012` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `mcp/tools.js` | Added `upload_media`, `plan_image`, `generate_image` | `INIT-004/SPEC-012` |
| `test/media-generation-mcp.test.js` | Created — 12 tests | `INIT-004/SPEC-012` |
| `server/api/media-routes.js` | Approval route refuses a presented MCP credential with 403 before the session guard | `INIT-004/SPEC-012` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-012-api-media-generation-mcp.md` | Status set to `complete`; criteria checked | `INIT-004/SPEC-012` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/completion-summaries/SPEC-012-eval-report.md` | Created — verifier's per-AC verdicts | `INIT-004/SPEC-012` |

---

## Trace Emission Degradation

Timer and trace scripts are not in this repo. This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the trace; `scripts/dev/lib/sdd_trace` is not in this repo | No opening sentinel for SPEC-012 | A Tier-5 reader has no live trace file from this worker. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions were not emitted | Same |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No event for the security review or the AC verifier | Disclosed here and under Deviations |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Scripts absent; orchestrator owns close | No reconciled `trace_complete` from this worker | `run_trace_invariants_validate` and `model_dispatch_conformance_validate` were not run here |

---

## Deviations from the Spec

- **Approval pre-handler.** The dispatch said not to edit HTTP media routes. The session guard on `POST /api/admin/media/:id/approval` returned 401 for a request that presented only the MCP token, and ac-3 requires 403 (ADR-0004 §4, mcp column). `refuseMcpApproval` was added in front of that guard. Upload, fetch, generate, delete, and the admin approval success path were not rewritten.
- **Audit rows.** Successful MCP upload and generate append `media_upload` / `media_generate` with actor `mcp` and `detail` null, matching ADR-0004 §10. The prompt is not copied into the audit row.
- **Git:** no commit, push, or PR (the dispatch said not to).
- **Persona:** `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo. No persona file was invented. The code follows the spec and `mcp/tools.js`. `scripts/dev/expertise_state.py` is also absent, so the dispatch-check was not run.
- **Harness (cursor, T2):** the AC verifier was a fresh `generalPurpose` subagent on `composer-2.5-fast`. The security review was a `security-review` subagent on `composer-2.5-fast`. The Task tool has no readonly flag; both prompts forbade edits. The compensating control remains the initiative-wide Security Review at `complete-initiative`.
- **Learner snapshot:** not built. INIT-006 owns `get_learner_profile`. This spec did not add `get_learner_snapshot`.
- **Quality gates:** no lint, type-check, or coverage tool is configured. `node --check` is clean on the edited JavaScript, and `npm test` is the gate. No suppression comment was added.
- **Token check:** handlers use `requireMcpAuthToken`, the same env check the plan tools use. `list_lessons` itself does not call it; the HTTP bearer gate in `mcp/http.js` still refuses a missing or wrong token before any tool runs.

---

## Effort Tracking

**Agent session time:** not recorded — the timer script is absent and the orchestrator owns it. This dispatch ran on 2026-10-09.
**Spec effort estimate:** 4 hours
