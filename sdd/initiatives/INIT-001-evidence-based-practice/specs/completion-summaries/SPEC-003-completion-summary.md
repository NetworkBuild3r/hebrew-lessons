# SPEC-003 Completion Summary — Review store and scheduler

**Traceability:** INIT-001/SPEC-003  
**Status:** `in_review`  
**Domain:** frontend  
**Completed Date:** 2026-10-08  
**Persona:** principal-frontend-developer (described — `.claude/agents/` absent; no AGENT.md invented)

---

## Deliverables Checklist

- [x] `shared/player/review.js` (`window.HebrewReview` + `module.exports`)
- [x] `shared/worksheet.js` load order (review after utils, before activities)
- [x] `shared/player/core.js` (`ctx.review`, load + migrate on boot)
- [x] `shared/activities/words.js` (knew/missed → `record` `he_en`)
- [x] `test/review-scheduler.test.js`, `test/review-store.test.js`

## Acceptance Criteria

- [x] ac-1: ADR ladder / same-day / miss→step 0
- [x] ac-2: dueList oldest-first, caps, interleave ≤2
- [x] ac-3: words self-marks → `he_en` record
- [x] ac-4: wordMarks migrate once; re-run no-op
- [x] ac-5: setItem quota fail-soft; no uncaught; clean console on review path
- [x] ac-6: no lesson `index.html` edits; no network fetch

## AC Verification (independent)

Verified by fresh readonly `composer-2.5-fast` subagents (not the implementer).  
Overall: **PASS** (all-PASS after 1 remediation loop on ac-5).  
**Full eval report:** [`SPEC-003-eval-report.md`](SPEC-003-eval-report.md).

**Verification tier:** `full` · **pre-flight:** gate_actions `[]` PASS · security_review=`light` PASS · domain=`frontend` FAIL · autonomy_level=`auto_tests` PASS => light_tier_eligible=`false` · **checks skipped:** None

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | `review.js` nextState + scheduler tests |
| ac-2 | PASS | `dueList` + interleave/cap tests |
| ac-3 | PASS | `words.js` record hook; `ctx.review` |
| ac-4 | PASS | migrate + no-op tests |
| ac-5 | PASS | quota + clean-console store tests (loop 1) |
| ac-6 | PASS | loader order; no index.html / fetch |

---

## Change Produced

- **Authored by:** `INIT-001/SPEC-003`
- **Title:** Add browser review store + ADR-0001 scheduler; wire words self-marks and wordMarks migration
- **Files changed:** 6 implementation/test + 2 sdd artifacts (+ spec status)
- **Change status:** proposed
- **Reversibility:** revertible (revert listed files; `hebrew-review:v1` ignored by older code)

---

## Tests

| Test file | New tests | Notes |
| --------- | --------- | ----- |
| `test/review-scheduler.test.js` | 11 | fake clock; ladder, same-day, caps, interleave |
| `test/review-store.test.js` | 8 | fake localStorage, quota, migration, clean console |

**All tests passing:** yes · `npm test` → **47/47** pass · **Coverage (new code):** exercised by the two new suites (AAA; fake clock / fake storage)

---

## Gates & Controls

None — `gate_actions: []`. Autonomy `auto_tests`. Blocking check `npm test` green. Chrome smoke not extended (unit tests cover ACs; optional per dispatch).

---

## Security Review

- **Sensitivity / depth:** light · quick · **method:** inline T3 reviewer judgment (`self_reviewed: true` — Cursor harness; no Critical/High)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | no findings |

- **Positive observations:** Single-user localStorage only; fail-soft on quota/parse; no fetch; no secrets/PII in review blob; NFC key matches speech identity.

---

## Files Changed

Provenance is `INIT-001/SPEC-003` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `shared/player/review.js` | Created — scheduler + store | `INIT-001/SPEC-003` |
| `shared/worksheet.js` | Load `player/review.js` before activities | `INIT-001/SPEC-003` |
| `shared/player/core.js` | `ctx.review`, load + migrate | `INIT-001/SPEC-003` |
| `shared/activities/words.js` | Self-marks → `record(he_en)` | `INIT-001/SPEC-003` |
| `test/review-scheduler.test.js` | Created | `INIT-001/SPEC-003` |
| `test/review-store.test.js` | Created | `INIT-001/SPEC-003` |
| `sdd/.../SPEC-003-eval-report.md` | Created | `INIT-001/SPEC-003` |
| `sdd/.../SPEC-003-completion.md` | Created | `INIT-001/SPEC-003` |
| `sdd/.../SPEC-003-frontend-review-store-scheduler.md` | status → `in_review` | `INIT-001/SPEC-003` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `trace_open` / `dispatch` / `trace_complete` | `scripts/dev/` absent — no `lib/sdd_trace`; orchestrator owns timer/trace for this dispatched run | Live `.sdd-trace/SPEC-003.jsonl` not written by this worker | Tier-5 / model-dispatch checks deferred to orchestrator; disclosed |

---

## Effort Tracking

| Field | Value |
| ----- | ----- |
| Estimated | 4.5 hours |
| Actual (this worker) | ~1 session (orchestrator owns timer) |
| Timer ownership | Orchestrator (dispatched sub-agent) |

---

## Deviations

- **Persona:** `.claude/agents/` absent (`agents_dir_absent`) — adopted principal-frontend-developer role from `prompt-selection.md` §1; did not invent AGENT.md.
- **Timer / worktree / branch:** skipped — orchestrator-dispatched; worktree already engaged in-place on main.
- **Chrome smoke:** not extended; unit coverage for ACs sufficient per dispatch option.
- **Workspace bleed:** sibling SPEC-002 paths (`package.json`, vocab-index, mcp) may appear dirty in the shared checkout — not modified by this worker.
- Migration of `knew` follows **ADR-0001 §6** (step 0, due +1 day), not the informal “step 1” phrase in the spec description.

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4.5 AC verifier | `subagent_dispatch` | T2 (Cursor) | `Task` + `readonly: true` + `composer-2.5-fast` |
| Phase 4 security review | reviewer judgment | T3 | Inline light review; `self_reviewed: true` |
