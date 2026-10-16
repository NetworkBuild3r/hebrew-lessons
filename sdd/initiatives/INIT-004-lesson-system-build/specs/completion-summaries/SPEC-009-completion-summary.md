# SPEC-009 Completion Summary — OpenAI image client: dry-run, caps, and confirm

**Traceability:** INIT-004/SPEC-009
**Status:** complete
**Domain:** service
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `server/media/openai-images.js` — `planImage` dry-run, `generateImage` with confirm, caps, pinned model, and ingest.
- [x] `test/openai-images.test.js` — injected transport. `fetch` and `https.request` are tripwired.

## Acceptance Criteria

- [x] ac-1: a one-image dry-run returns `0.011` and the fake records zero calls. Generate without boolean `confirm: true` records zero calls.
- [x] ac-2: with the key unset, plan and generate throw `missing_key` and the fake records zero calls. The error message is fixed and has no key-shaped string.
- [x] ac-3: the 16th generate in a clock hour, and a generate that would take the UTC day past $1, are refused before the provider. A provider error after send still counts.
- [x] ac-4: the outbound body model is `gpt-image-1`. The request builder source has no other model string.

## AC Verification (independent)

Verified by a fresh sub-agent ([Verify SPEC-009 criteria](30a9e4a7-b41f-43d3-b019-b29821f9892a), model `composer-2.5-fast`) that did **not** author the code — not self-reported.
Overall: **PASS** · remediation loops: 0.
**Full eval report:** [`SPEC-009-eval-report.md`](SPEC-009-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` `pass` · security_review=`required` `FAIL` · domain=`service` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `server/media/openai-images.js:127-129`, `401-407`, `412-413`. Tests at `test/openai-images.test.js:173-223`. Verifier re-ran `node --test test/openai-images.test.js` → 17 pass, 0 fail. |
| ac-2 | PASS | `readApiKey` at `server/media/openai-images.js:157-161`. In-process and child-process tests at `test/openai-images.test.js:225-280`. Refusals use the fixed message. |
| ac-3 | PASS | `reserveGeneration` before `await transport` at `server/media/openai-images.js:456-460`. Tests at `test/openai-images.test.js:282-388`. Day spend after 90 calls is 990 mills; the next call does not contact the provider. |
| ac-4 | PASS | `buildGenerationBody` at `server/media/openai-images.js:142-149`. Test at `test/openai-images.test.js:390-421` checks the body and the function source. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-009`
- **Title:** OpenAI image client with a dry-run, confirm, spend caps, and ingest of the returned bytes
- **Files changed:** 2 new code and test files, plus this spec's status and the two completion artifacts
- **Change status:** proposed
- **Reversibility:** branch. Deleting the module and its test removes the client. No image is stored if ingest is not called.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/openai-images.test.js` | 17 | 17 |

**All tests passing:** Yes · `node --test test/openai-images.test.js` → 17 pass, 0 fail, exit 0 · `npm test` → 526 pass, 0 fail, exit 0 · **Coverage (new code):** no coverage tool is configured. The spec's bar is this file plus `npm test`.

---

## Gates & Controls

None — branch/reversible only. `gate_actions` is empty. The OpenAI key is SPEC-017. This spec reads `OPENAI_API_KEY` at call time and refuses when it is absent. No package was added.

---

## Security Review

- **Sensitivity / depth:** required · thorough · **method:** independent `security-review` Task (`grok-4.7-high`, [Security Review](e3e2bf26-160c-4450-a976-a05ca6d85cb2)), scoped to the two SPEC-009 files. Cursor Task has no native `readonly` flag. The prompt forbade edits. Not self-reviewed.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | no findings in scope |

- **Positive observations:** the key is read at call time, refusals use fixed strings, the host is `api.openai.com`, response URLs are not fetched, the model id cannot be substituted, and the cap is reserved before the provider is contacted. Caps are in memory for one replica, the same shape as the upload cap in ingest.

---

## Files Changed

Provenance is `INIT-004/SPEC-009` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `server/media/openai-images.js` | Created — plan, generate, caps, pinned model, ingest (517 lines) | `INIT-004/SPEC-009` |
| `test/openai-images.test.js` | Created — 17 tests, injected transport (609 lines) | `INIT-004/SPEC-009` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-009-service-image-generation.md` | Status set to `complete`; criteria checked | `INIT-004/SPEC-009` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/completion-summaries/SPEC-009-eval-report.md` | Created — verifier's per-AC verdicts | `INIT-004/SPEC-009` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/completion-summaries/SPEC-009-completion-summary.md` | Created — this summary | `INIT-004/SPEC-009` |

---

## Trace Emission Degradation

This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. `scripts/dev/lib/sdd_trace` is not in this repo. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the trace; the trace library is not in this repo | No opening sentinel for SPEC-009 | A Tier-5 reader has no live trace file from this worker. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions were not emitted | Same |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No event for the security review or the AC verifier | Disclosed here and under Deviations |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Scripts absent; orchestrator owns close | No reconciled `trace_complete` from this worker | `run_trace_invariants_validate` and `model_dispatch_conformance_validate` were not run here |

---

## Deviations from the Spec

- **Git:** no commit, push, or PR. The dispatch said not to.
- **Persona:** `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo. No persona file was invented. The code follows the spec, ADR-0004 §7, and `server/media/ingest.js`. `scripts/dev/expertise_state.py` is also absent, so the dispatch-check was not run.
- **Prompt list:** ADR-0004 names the categories sexual act, minor, and likeness, and does not list individual words. The client uses a fixed word list for those three categories. A refusal is not sent.
- **Harness (cursor, T2):** the AC verifier was a fresh `generalPurpose` subagent on `composer-2.5-fast`. The security review was a `security-review` subagent on `grok-4.7-high`. The Task tool has no readonly flag; both prompts forbade edits.
- **Quality gates:** no lint, type-check, or coverage tool is configured. `node --check` is clean, and `npm test` is the gate. No suppression comment was added.
- **SPEC-017, plan routes, and `worksheets/*/index.html`:** not touched.

---

## Effort Tracking

**Agent session time:** not recorded — the timer script is absent and the orchestrator owns it. This dispatch ran on 2026-10-09.
**Spec effort estimate:** 4 hours
