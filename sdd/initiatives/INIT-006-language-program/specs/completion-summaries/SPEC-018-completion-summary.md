# SPEC-018 Completion Summary — Draft verb guide and verb workshop

**Traceability:** INIT-006/SPEC-018
**Status:** Complete
**Domain:** data
**Completed Date:** 2026-10-10

---

## Deliverables Checklist

- [x] `scripts/seed-starter-content.js`
- [x] `sdd/initiatives/INIT-006-language-program/specs/completion-summaries/SPEC-018-forms-checklist.md`
- [x] `test/starter-content.test.js`

## Acceptance Criteria

- [x] ac-1 (REQ-014): both drafts validate under schema 1.1.0 with no block-level errors; N14 unverified-form warnings are expected.
- [x] ac-2 (REQ-014): 4 lemmas × 13 forms = 52 forms, each tagged with tense, cell, binyan, and root class.
- [x] ac-3 (REQ-014): the checklist lists all 52 forms; a later run verifies only ticked forms.
- [x] ac-4: no teacher name, learner name, or copied class line in the content.

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`) that did not author the code.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-018-eval-report.md`](SPEC-018-eval-report.md)

**Verification tier:** `full` · **pre-flight:** the dispatch did not classify this spec as light-tier AC verification · **checks skipped:** `None`

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | `ac-1 and ac-2: drafts validate and 52 forms stay unverified` passed. Both plans are schema 1.1.0 with an empty `blocks` list. The verb workshop warns N14. `node --test test/starter-content.test.js` → 5 pass, 0 fail, exit 0. |
| ac-2 | PASS | Same test: 52 forms, 4 lemmas, tense, cell, binyan, and root class stored, `verified` 0. |
| ac-3 | PASS | `ac-3: a second run verifies only ticked forms` passed. Two ticks verify two forms; clearing one tick leaves one verified. |
| ac-4 | PASS | `ac-4: stored content has no name and no copied class line` passed. |

---

## Change Produced

- **Authored by:** `INIT-006/SPEC-018`
- **Title:** Add draft Decoding Hebrew Verbs guide, one verb workshop, and an unverified 52-form lexicon seed
- **Files changed:** seed script, forms checklist, temporary-database test, this spec's status
- **Change status:** proposed
- **Reversibility:** revertible. Delete plans `guides` and `verbs`, and lexicon rows whose keys are this script's lemmas and forms. No migration. Nothing is published.

The script was not run against the project database. It writes only when invoked, and only through `lexicon.upsert`, `setVerified`, and plan ops. A placeholder plan row is inserted when the plan id is missing, because ops has no create-plan operation; `replacePart` immediately replaces that row with the validated draft.

---

## Tests

| Test file | New tests | Total |
| ------------- | --------- | ----- |
| `test/starter-content.test.js` | 5 | 5 |

**All tests passing:** Spec file yes · `node --test test/starter-content.test.js` → 5/5, exit 0 · `npm run audio:verify` → OK, 65 recordings · `npm test` → 793 pass, 1 fail, exit 1 · **Coverage:** no coverage tool is configured; the spec's bar is the temporary-database test

The one suite failure is `test/plan-render.test.js` ("the page script does not fetch a worksheet or write plan text except through renderScreens"). `shared/ui/plan-render.js` now calls `renderScreens(plan, { forms, audio })`, so the test's exact-string removal no longer strips `innerHTML`. This spec does not edit that file.

---

## Gates & Controls

| Gate action | Reversibility | Approved? | Blocking checks | Notes |
| ------------------- | ------------- | --------- | ---------------- | ------------------- |
| Operator content check before `setVerified` or publish | revertible | not requested | checklist ticks | No form is verified in the committed checklist. `setVerified` runs only for `[x]` lines. Publish is not called. |
| none configured as `gate_actions` | revertible | n/a | spec test, audio verify | `npm test` is red in a file this spec does not own. See Tests. |

---

## Security Review

- **Sensitivity / depth:** light · focus: no names or transcripts · **method:** [Light security review SPEC-018](7de2a07c-3d4c-4f83-9f5d-8e87c2f1f6cc), scoped to this spec's files. SAST and secret scan: not run — none configured.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | ----------------- | ------------- | ----------- | ----------------------- |
| — | — | — | — | — |

- **Positive observations:** Plans stay `draft` with lesson `visibility` `hidden` and a null snapshot. `upsert` stores `verified` as 0. Only checklist lines that match a known form, lemma, tense, and cell reach `setVerified`. A key whose `source` is not `admin` refuses the whole run before any write. Lesson prose is original and contains no HTML. The seed script does not use SQL.
- **Notes (not findings):** The operator who can run the script can also tick the checklist. That is the content gate, not a cross-user boundary. `checklistPath` writes wherever the caller points it.

---

## Files Changed

Provenance is `INIT-006/SPEC-018` for every row below.

| File | Change | Authored by |
| --------------------- | ---------------------------- | ------------------- |
| `scripts/seed-starter-content.js` | Created — idempotent draft seed | `INIT-006/SPEC-018` |
| `test/starter-content.test.js` | Created — temporary-database tests | `INIT-006/SPEC-018` |
| `sdd/initiatives/INIT-006-language-program/specs/completion-summaries/SPEC-018-forms-checklist.md` | Created — 52 unticked forms | `INIT-006/SPEC-018` |
| `sdd/initiatives/INIT-006-language-program/specs/SPEC-018-data-starter-content-verbs.md` | Status set to `complete`; criteria checked | `INIT-006/SPEC-018` |
| `sdd/initiatives/INIT-006-language-program/specs/completion-summaries/SPEC-018-completion-summary.md` | Created | `INIT-006/SPEC-018` |
| `sdd/initiatives/INIT-006-language-program/specs/completion-summaries/SPEC-018-eval-report.md` | Created | `INIT-006/SPEC-018` |

No `worksheets/*/index.html` file was edited. No secret was written.

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4 security review | `subagent_dispatch` | T2 (Cursor) | [Light security review SPEC-018](7de2a07c-3d4c-4f83-9f5d-8e87c2f1f6cc), model `composer-2.5-fast` |
| Phase 4.5 AC verifier | `subagent_dispatch` | T2 (Cursor) | [Verify SPEC-018 criteria](ddd4943b-1855-49bc-a892-59ae01a7885e), model `composer-2.5-fast`. The Task tool has no readonly flag; the prompt forbade edits |

---

## Trace Emission Degradation

This worker was orchestrator-dispatched, so the orchestrator owns `open_trace` / `close_trace` and the timer. `scripts/dev/lib/sdd_trace` is not in this repo. Nothing was invented in their place.

| Site | Event type | Reason | What was lost | Impact |
| -------------------------- | ---------------- | ------------------------------------------------------------ | ------------------------------------ | -------------------------------------------------------------- |
| Phase 0 | `trace_open` | Orchestrator owns the sentinel; trace library is not in this repo | No opening sentinel for SPEC-018 | A Tier-5 reader has no live trace file. Disclosed; not invented. |
| Phase 1 | `task_step` | Same | Step transitions | Same |
| Phase 4 / 4.5 | `harness_fallback` | Trace library absent | No harness-fallback event | Disclosed here instead |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Orchestrator owns close; scripts absent | No reconciled `trace_complete` | Tier-5 and model-dispatch validators were not run here |

---

## Deviations from the Spec

- Git commit, push, and PR were not performed. The dispatch said to skip them.
- Persona `.claude/agents/implementation-planner/AGENT.md` is not in this repo. No persona file was invented. Implementation followed the spec, ADR-0009 §7 block shapes, and the lexicon service.
- Plan ops cannot create a plan. The script inserts a one-block placeholder with `saveWorkingCopy`, then `replacePart` replaces it with the validated draft. That placeholder is not the content.
- The real database was not seeded. Running `node scripts/seed-starter-content.js` with `DATA_DIR` pointed at the project data directory is the operator step. The committed checklist is entirely unticked, so that run leaves every form unverified.
- Pointing is a standard draft, not a checked paradigm. Class notes were not copied. `verified` stays 0 until a person ticks a line.
- `npm test` is not green. The failure is in `test/plan-render.test.js` against `shared/ui/plan-render.js`, which this spec does not change.

---

## Pre-flight

```text
Spec: SPEC-018 validated · domain: data · prompt: implementation-planner (file absent; degradation disclosed)
Autonomy: auto_tests · gates expected: operator content check before setVerified or publish (not taken; nothing verified or published)
Dependencies: SPEC-005 and SPEC-011 complete · context loaded · timer owned by the orchestrator
```
