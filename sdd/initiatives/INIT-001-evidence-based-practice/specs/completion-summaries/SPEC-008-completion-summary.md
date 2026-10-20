# SPEC-008 Completion Summary — Validator nudges (recycling, coverage, glosses)

**Traceability:** INIT-001/SPEC-008  
**Status:** Partial (implementation done; status `in_review` — Tier-5 / model-dispatch validators unavailable without `scripts/dev`)  
**Domain:** service  
**Completed Date:** 2026-10-08

---

## Deliverables Checklist

- [x] `.cursor/skills/hebrew-worksheet/scripts/validate.js` — recycling / encounters / coverage warnings; glosses schema error + stray-key warning
- [x] `mcp/tools.js` — `formatUpsertHead` surfaces warnings ahead of `rebuildVocabIndexSafe`
- [x] `test/validate-nudges.test.js`

## Acceptance Criteria

- [x] ac-1: fixture with 0 recycled words warns; ≥3 does not
- [x] ac-2: new word with &lt;2 encounters warns
- [x] ac-3: low-coverage story warns with estimated `%`
- [x] ac-4: malformed `reading.glosses` → error; keys not in story → warning
- [x] ac-5: existing published lessons exit 0 (no new errors)
- [x] ac-6: `upsert_lesson` reply includes warnings via `formatUpsertHead`

## AC Verification (independent)

Verified by a fresh readonly explore Task (model `composer-2.5-fast`, agent `62a2e1a0-a792-4d37-97da-348f6ec1dd42`) that did **not** author the code — not self-reported.  
Overall: **PASS** (all-PASS) · remediation loops: 0.  
**Full eval report:** [`SPEC-008-eval-report.md`](SPEC-008-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` PASS · security_review=`standard` FAIL · domain=`service` FAIL · autonomy_level=`auto_tests` FAIL ⇒ light_tier_eligible=`false` · **checks skipped:** None

| AC   | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS    | recycle warn `<3`; test ac-1 ✔ |
| ac-2 | PASS    | encounters warn; test ac-2 ✔ |
| ac-3 | PASS    | coverage `%` warn; test ac-3 ✔ |
| ac-4 | PASS    | glosses error/warn; test ac-4 ✔ |
| ac-5 | PASS    | three published slugs exit 0; `npm test` 59/59 |
| ac-6 | PASS    | `formatUpsertHead` + store WARN lines; test ac-6 ✔ |

---

## Change Produced

- **Authored by:** `INIT-001/SPEC-008`
- **Title:** Validator nudges for recycling, encounters, coverage, and glosses; surface warnings in upsert reply
- **Files changed:** 5 · additions [~350] · deletions [~30]
- **Change status:** proposed
- **Reversibility:** revertible

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| `test/validate-nudges.test.js` | 8 | 8 |

**All tests passing:** yes (`npm test` → 59/59) · **Coverage (new code):** no coverage tool wired (per Verification Strategy)

---

## Gates & Controls

None — `gate_actions: []`. Autonomy `auto_tests`. Blocking check `npm test` green; validator over `worksheets/*` published lessons exit 0 (warnings only, no new errors). No one-way action taken.

---

## Security Review

- **Sensitivity / depth:** standard · focus: data · **method:** inline implementer review (`self_reviewed: true` — Cursor T3 disclosure for reviewer-agent judgment; independent AC verifier was T2 Task)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None
- **SAST / secret scan:** not run — none configured for this repo

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| —       | —        | —             | —        | no findings |

- **Positive observations:** validator still reads only under `LESSONS_DIR` / `HEBREW_ROOT` via existing `path.join` helpers; `vm` sandbox unchanged (window/console only); no new network or write paths; gloss values are validated as strings only (no execution).

---

## Files Changed

Provenance is `INIT-001/SPEC-008` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `.cursor/skills/hebrew-worksheet/scripts/validate.js` | Modified — nudge warnings + glosses schema | `INIT-001/SPEC-008` |
| `mcp/tools.js` | Modified — `formatUpsertHead` (warnings before vocab-index rebuild) | `INIT-001/SPEC-008` |
| `test/validate-nudges.test.js` | Created — fixture + published-lesson tests | `INIT-001/SPEC-008` |
| `sdd/initiatives/…/SPEC-008-eval-report.md` | Created — AC eval report | `INIT-001/SPEC-008` |
| `sdd/initiatives/…/SPEC-008-completion.md` | Created — this summary | `INIT-001/SPEC-008` |
| `sdd/initiatives/…/SPEC-008-service-validator-nudges.md` | Modified — status → `in_review`; AC checkboxes | `INIT-001/SPEC-008` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `trace_open` / `dispatch` / `trace_complete` / all emits | `scripts/dev/` absent — no `lib/sdd_trace`, no timer scripts | Entire run-trace for SPEC-008 | Tier-5 invariants + model-dispatch-conformance checks cannot run; disclosed; spec left `in_review` |
| Phase 0 | timer start/complete | same — `scripts/dev/` absent | Effort minutes in `.sdd-timer.json` | Effort Tracking estimated only |

---

## Harness Adaptation

- **Persona:** `.claude/agents/` tree absent (`agents_dir_absent`). Disclosed; adopted **principal-backend-developer** role from `prompt-selection.md` §1 (service domain) without inventing `AGENT.md`. `scripts/dev/expertise_state.py` also absent — dispatch-check skipped.
- **Timer / trace:** `scripts/dev/` absent — Start/Complete Timer and `open_trace`/`emit`/`close_trace` skipped; this worker wrote the completion summary per dispatch adaptation.
- **Worktree / branch / git:** Git & Issue-Tracker Operations Guard active for dispatched worker — no branch/commit/push/PR. Worktree path as provided by orchestrator.
- **AC verifier:** Cursor T2 via explore Task (readonly, `composer-2.5-fast`) — independent PASS.
- **Security review:** standard inline (`self_reviewed: true`) — zero Critical/High.

---

## Deviations from the Spec

- Completion summary filename per initiative convention: `SPEC-008-completion.md` (dispatch asked for this path).
- Spec status set to `in_review` (not `complete`) — Tier-5 / model-dispatch validators unavailable without traces; orchestrator/HITL may promote after review.
- SPEC-007 tokenizer not landed — local `tokenizeHebrew` with TODO to consolidate.
- Git operations skipped (guard / dispatched sub-agent).

---

## Effort Tracking

Estimated ~2–3 hours (timer unavailable — `scripts/dev/` absent).
