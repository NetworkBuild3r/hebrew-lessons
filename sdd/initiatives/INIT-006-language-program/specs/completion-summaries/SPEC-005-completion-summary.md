# SPEC-005 Completion Summary — Lexicon service, validator, and seed

**Traceability:** INIT-006/SPEC-005
**Status:** Complete
**Domain:** service
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `server/lexicon/validate.js`, `server/lexicon/service.js`, `server/lexicon/seed.js`
- [x] One call in `server/start.js`, after `db.migrate()`, same place as `seedVisibility`
- [x] `test/lexicon-service.test.js` on a temporary database and a copy of the three lesson worksheets

## Acceptance Criteria

- [x] ac-1 (REQ-004): a batch with one invalid entry writes nothing and returns every error with `key`, `field`, `code`.
- [x] ac-2 (REQ-004): text containing `<script>` is rejected; NFD input is stored NFC.
- [x] ac-3 (REQ-013): `upsert` with a stale `expectedRev` returns `revision_conflict` and leaves the row unchanged.
- [x] ac-4 (REQ-011): editing a verified form clears `verified`; `setVerified` writes an audit row and refuses when the actor is `mcp`.
- [x] ac-5: running `seed` twice on the three current lessons yields the same row count; an `mcp`-sourced entry with the same key keeps its gloss.

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`) that did not author the code.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-005-eval-report.md`](SPEC-005-eval-report.md)

**Verification tier:** `full` · **pre-flight:** not classified light by the orchestrator · **checks skipped:** `None`

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: a batch with one invalid entry writes nothing and returns every error` passed. `node --test test/lexicon-service.test.js` → 7 pass, 0 fail, exit 0. |
| ac-2 | PASS | `ac-2: markup is rejected and NFD input is stored NFC` passed in that run. |
| ac-3 | PASS | `ac-3: a stale expectedRev returns revision_conflict and leaves the row` passed in that run. |
| ac-4 | PASS | `ac-4: editing a verified form clears verified; setVerified audits and refuses mcp` passed in that run. |
| ac-5 | PASS | `ac-5: seeding the three lessons twice keeps the row count and an mcp gloss` passed in that run. |

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-005`
- **Title:** Add the lexicon validator, write service, and idempotent vocab seed
- **Files changed:** three service modules, a startup call, two repository read helpers, and the new test
- **Change status:** proposed
- **Reversibility:** branch. Rollback is to remove the `server/start.js` call. Rows with `source='seed'` can be deleted. No migration was added.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/lexicon-service.test.js` | 7 | 7 |

**All tests passing:** Yes · `node --test test/lexicon-service.test.js` → 7/7, exit 0 · `npm test` → 590/590, exit 0 · `npm run audio:verify` → OK, 65 recordings · **Coverage (new code):** no coverage tool is configured; the spec's bar is this file plus the existing suite

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| none | branch | n/a | `node --test test/lexicon-service.test.js`, `npm test`, `npm run audio:verify` | All three exited 0. No `db_migration`. Seed runs only after migrations already applied by `server/start.js`. |

---

## Security Review

- **Sensitivity / depth:** required · focus: input validation and verification authz · **method:** [Security Review](90e06aa4-7adc-4623-82b7-8fcd0ae99771), scoped to this spec's files. SAST and secret scan: not run — none configured.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0 (three notes recorded below, not raised as findings)
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| — | — | — | — | — |

- **Positive observations:** `<>` and ASCII controls are rejected in the validator and again by the SPEC-003 checks. `accept[]` entries that still contain U+0591–U+05C7 are rejected. `upsert` ignores payload `verified` and `source`. `setVerified` returns `forbidden` for actor `mcp` and writes no audit row on that path. Audit details are `{ count, cleared }` and `{ count, value }` with a null target. `search` binds every value and escapes `LIKE` metacharacters. Inputs longer than 500 UTF-16 units are rejected before normalization, and error keys are clipped to 60 code points.
- **Notes (not findings):** `actor` is trusted from the caller; HTTP and MCP adapters (SPEC-009, SPEC-010) must set it from the authenticated identity. `setVerified` does not bump `rev`. The repository `upsert` has no auth of its own; adapters are not given a SQL path.

---

## Files Changed

Provenance is `INIT-006/SPEC-005` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `server/lexicon/validate.js` | Created — batch validation, ADR-0009 §6 | `INIT-006/SPEC-005` |
| `server/lexicon/service.js` | Created — list, get, upsert, setVerified, paradigm, applySeed | `INIT-006/SPEC-005` |
| `server/lexicon/seed.js` | Created — idempotent seed from `buildVocabIndex()` | `INIT-006/SPEC-005` |
| `server/start.js` | Modified — seed call after migrations, count logged | `INIT-006/SPEC-005` |
| `server/db/repos/lexicon.js` | Modified — `listLemmaKeys` and `search` | `INIT-006/SPEC-005` |
| `test/lexicon-service.test.js` | Created — temporary-database tests | `INIT-006/SPEC-005` |
| `sdd/initiatives/INIT-006-language-program/specs/SPEC-005-service-lexicon.md` | Status set to `complete`; criteria checked | `INIT-006/SPEC-005` |

`shared/player/fsrs.js` and `server/progress.js` were not edited.

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4 security review | `subagent_dispatch` | T2 (Cursor) | [Security Review](90e06aa4-7adc-4623-82b7-8fcd0ae99771), model `grok-4.7-high`, scoped to this spec's files |
| Phase 4.5 AC verifier | `subagent_dispatch` | T2 (Cursor) | [Verify SPEC-005 criteria](630e8312-4615-4d58-b298-55103ee3d13c), model `composer-2.5-fast`. The Task tool has no readonly flag; the prompt forbade edits |

---

## Trace Emission Degradation

Timer and trace scripts are not in this repo. This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the sentinel; `scripts/dev/lib/sdd_trace` is not in this repo | No opening sentinel for SPEC-005 | A Tier-5 reader has no live trace file. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions | Same |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No harness-fallback event | Disclosed here instead |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Orchestrator owns close; scripts absent | No reconciled `trace_complete` | Tier-5 and model-dispatch validators were not run (scripts absent) |

---

## Deviations from the Spec

- Git commit, push, and PR were not performed. The dispatch said to skip them.
- Persona `.claude/agents/principal-backend-developer/AGENT.md` is not in this repo. No persona file was invented. Implementation followed the spec, ADR-0009 §6, and `server/db/repos/lexicon.js` (bound parameters, explicit columns).
- `web.js` was not given a seed call. It does not migrate or call `seedVisibility` either. The one startup call is in `server/start.js`, which is the process that migrates and then listens.
- One `expectedRev` applies to every row in the batch. A missing row's current revision is 0. A batch that mixes a new row with a row whose `rev` is not 0 returns `revision_conflict` and writes nothing.
- Any successful `upsert` stores `verified` as 0, including a lemma that was previously verified. The audit `cleared` count includes those rows. The acceptance criterion is the verified form, which is covered.
- Seed glosses longer than 120 characters, and transliterations longer than 80, are clipped. A field that contains `<`, `>`, or a control character is skipped and counted, not inserted. The three current lessons skipped 0.
- `server/db/repos/lexicon.js` gained `listLemmaKeys` and `search` so the service does not write SQL. Adapters still have no SQL path.

---

## Pre-flight

```text
Spec: SPEC-005 validated · domain: service · prompt: principal-backend-developer (file absent; degradation disclosed)
Autonomy: auto_tests · gates expected: none
Dependencies: SPEC-003 complete · context loaded · timer owned by the orchestrator
```
