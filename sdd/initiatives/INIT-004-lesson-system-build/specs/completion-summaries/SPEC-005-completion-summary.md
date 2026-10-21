# SPEC-005 Completion Summary — Plan MCP tools

**Traceability:** INIT-004/SPEC-005
**Status:** Complete
**Domain:** api
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] Plan tools in `mcp/tools.js` — `read_part`, `replace_part`, `insert_block`, `move_block`, `remove_block`, `set_field`, `publish_plan`. Each calls the matching function in `server/plans/ops.js` or `server/plans/publish.js`.
- [x] Tests: `test/plan-mcp.test.js` — spy byte-match, `revision_conflict`, unset token, tool-name list, wrong bearer, no learner-progress fields.

## Acceptance Criteria

- [x] ac-1: each new tool calls the matching function once. The stored document bytes match the same call made through the function directly.
- [x] ac-2: a stale `expectedRevision` returns `revision_conflict` and the stored revision. The stored plan is unchanged.
- [x] ac-3: with `MCP_AUTH_TOKEN` unset, `read_part` returns no plan JSON. `test/mcp-auth.test.js` still passes for the legacy HTTP token gate.
- [x] ac-4: `list_lessons`, `get_lesson`, `upsert_lesson`, `set_lesson_visibility`, and `generate_lesson_audio` still register, along with the other three legacy tools. A test lists the names.
- [x] Security: no plan tool returns `review_items`, `practice_events`, or `lesson_progress`. A missing or wrong token yields no working-copy JSON.

## AC Verification (independent)

Verified by a fresh readonly sub-agent (model `composer-2.5-fast`, agent `f0f60dda-8451-4733-adbd-55b9093f814d`) that did **not** author the code.
Overall: **PASS** · remediation loops: 0.
**Full eval report:** [`SPEC-005-eval-report.md`](SPEC-005-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` `pass` · security_review=`standard` `FAIL` · domain=`api` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: each plan tool calls its function once and stores the same bytes` (`test/plan-mcp.test.js:232-329`). Handlers at `mcp/tools.js:439-527`. Verifier: `node --test test/plan-mcp.test.js test/mcp-auth.test.js` → 8 pass, 0 fail. |
| ac-2 | PASS | `ac-2: a stale expectedRevision returns revision_conflict and does not write` (`test/plan-mcp.test.js:332-370`). `planToolResult` at `mcp/tools.js:153-161`. |
| ac-3 | PASS | `ac-3: with the token unset, read_part returns no plan JSON` (`test/plan-mcp.test.js:372-400`). `MCP requires the bearer token` (`test/mcp-auth.test.js:27-46`). |
| ac-4 | PASS | `ac-4: legacy tools and the seven plan tools are registered` (`test/plan-mcp.test.js:208-230`). |
| security ×2 | PASS | Wrong bearer (`test/plan-mcp.test.js:402-445`). No progress fields (`test/plan-mcp.test.js:447-457`). |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-005`
- **Title:** Register seven plan MCP tools that call the existing plan operations
- **Files changed:** 2 code files (`mcp/tools.js` +158, `test/plan-mcp.test.js` created), plus this summary, the eval report, and the spec status
- **Change status:** proposed — not committed (dispatch said no commit)
- **Reversibility:** branch. Rollback: unregister the seven tools. Legacy tools remain.

---

## Tests

| Test file | New tests | Total in file |
| --------- | --------- | ------------- |
| `test/plan-mcp.test.js` | 6 | 6 |
| `test/mcp-auth.test.js` | 0 | 2 (unchanged, re-run) |

**All tests passing:** Yes · `node --test test/plan-mcp.test.js test/mcp-auth.test.js` → 8 pass, 0 fail. `npm test` → 459 pass, 0 fail, 2 todo, exit 0.
**Coverage (new code):** not measurable. This repository has no coverage tool. The bar is the spec file plus `npm test`.

---

## Gates & Controls

None — branch/reversible only. `gate_actions: []`. Autonomy `auto_tests`. No database migration, no dependency add, no commit, no push, no pull request.

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ----------- | ------------- | --------- | --------------- | ----- |
| — | branch | — | `node --test test/plan-mcp.test.js test/mcp-auth.test.js` and `npm test` both exit 0 | Rollback is unregistering the seven tools |

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** security-review subagent (Cursor T2)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | No medium-or-higher issue in `mcp/tools.js` or `test/plan-mcp.test.js` |

- **Positive observations:** `callPlanTool` checks `MCP_AUTH_TOKEN` before any plan read. HTTP still rejects a missing or wrong bearer in `mcp/http.js` before a handler runs. `planToolResult` omits `plan`, `part`, and `document` on errors and names `revision_conflict` with the stored revision. `planInput` forwards only `path`, `expectedRevision`, `index`, `field`, `value`, and `block`. The token and the plan body are not logged. The tools do not query learner progress tables.
- **Reviewer:** fresh `composer-2.5-fast` Task, agent `c03d945c-39eb-4bc4-8edc-c5c80db2ca79`. Prompt forbade edits and scoped the review to this spec's two files.

---

## Files Changed

Provenance is `INIT-004/SPEC-005` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `mcp/tools.js` | Modified — seven plan tools plus `planToolResult`, `callPlanTool`, `planInput`. Legacy eight tools unchanged. | `INIT-004/SPEC-005` |
| `test/plan-mcp.test.js` | Created — temporary database, spies, token and conflict cases | `INIT-004/SPEC-005` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-005-api-plan-mcp-tools.md` | Status set to `complete`; acceptance criteria checked | `INIT-004/SPEC-005` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/completion-summaries/SPEC-005-completion-summary.md` | Created | `INIT-004/SPEC-005` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/completion-summaries/SPEC-005-eval-report.md` | Created from the verifier's table | `INIT-004/SPEC-005` |

---

## Trace Emission Degradation

Timer and trace were absent on this dispatch (orchestrator-owned; this worker was told not to emit). `scripts/dev/lib/sdd_trace` is not in this repository. No `open_trace` / `emit` / `close_trace` call was made.

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 | `trace_open` | Orchestrator owns the sentinel; trace library is not in this repo | Opening sentinel for SPEC-005 | A Tier-5 reader of this worker's session sees no trace file from this agent |
| Phase 1 | `task_step` | Same | Step transitions | Granular step history was not written |
| Phase 4 / 4.5 | `harness_fallback` | Same | Cursor T2 disclosure event | Disclosed in Harness Adaptation below instead of the trace |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Same | Closing sentinel and model-dispatch evidence | Tier-5 and model-dispatch checks were not run here; they belong to the orchestrator if it owns the trace |

---

## Deviations from the Spec

- **Persona.** `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo (the spec already names that degradation). `scripts/dev/expertise_state.py` is not here, so `dispatch-check` could not run. Work followed the spec, ADR-0005 §5, and the existing `mcp/tools.js` style.
- **Git.** No commit, push, or pull request, per the dispatch. `git config user.email` is `brian.nelson@ahead.com`. The skill guard list ships empty, so the guard did not fire; the skip is the dispatch instruction.
- **No lint or typecheck command** is configured in `package.json`. Phase 4 ran the spec's `node --test` command and `npm test`. No `# type: ignore` or `# noqa` was added. No ratchet baseline was moved.
- **Discipline.** Initiative field `Discipline Profile: software`. Software verification mechanics applied (scoped `node --test`, plus the spec's own `npm test` bar). Coverage percentage is not measurable here.
- **HTTP routes.** Not touched. SPEC-004 owns that column.
- **Wrong token.** The handler refuses when `MCP_AUTH_TOKEN` is missing or empty, the same helper the legacy visibility tools use. A wrong bearer is refused by `mcp/http.js` before the handler runs. That file was not changed.

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4 security review | `subagent_dispatch` | T2 (Cursor) | Fresh `composer-2.5-fast` security-review Task. Prompt forbade edits. Agent `c03d945c-39eb-4bc4-8edc-c5c80db2ca79`. |
| Phase 4.5 AC verifier | `subagent_dispatch` | T2 (Cursor) | Fresh `composer-2.5-fast` generalPurpose Task. Prompt forbade edits. Agent `f0f60dda-8451-4733-adbd-55b9093f814d`. All-PASS, 0 remediation loops. |

---

## Effort Tracking

**Agent session time:** not recorded — timer absent on this dispatch (orchestrator-owned)
**Spec effort estimate:** 4 hours
