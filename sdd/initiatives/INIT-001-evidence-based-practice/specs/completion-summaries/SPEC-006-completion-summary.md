# SPEC-006 Completion Summary — Retry-before-reveal feedback

**Traceability:** INIT-001/SPEC-006  
**Status:** `in_review`  
**Domain:** frontend  
**Completed Date:** 2026-10-08  
**Persona:** principal-frontend-developer (described — `.claude/agents/` absent; no AGENT.md invented)

---

## Deliverables Checklist

- [x] `shared/activities/pattern.js` — retry-before-reveal + miss on first wrong + `attempts` in collect
- [x] `shared/activities/fill.js` — same
- [x] `shared/activities/listen-pick.js` — same
- [x] `test/retry-before-reveal-smoke.test.js` — Chrome smoke wrong-once / wrong-twice / miss / reload

## Acceptance Criteria

- [x] ac-1: first wrong → Try again, no reveal
- [x] ac-2: second wrong → reveal + audio affordance
- [x] ac-3: first wrong with vocab word → `miss` in review store
- [x] ac-4: reload restores revealed / attempt state

## AC Verification (independent)

Verified by fresh readonly `composer-2.5-fast` subagents (not the implementer).  
Overall: **PASS** (all-PASS after 1 remediation loop on ac-4).  
**Full eval report:** [`SPEC-006-eval-report.md`](SPEC-006-eval-report.md).

**Verification tier:** `full` · **pre-flight:** gate_actions `[]` PASS · security_review=`light` PASS · domain=`frontend` FAIL · autonomy_level=`auto_tests` PASS => light_tier_eligible=`false` · **checks skipped:** None

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | retry paint paths + smoke wrong-once |
| ac-2 | PASS | reveal + speakButton / Speech.play + smoke |
| ac-3 | PASS | record miss he_en / en_he + smoke |
| ac-4 | PASS | reload (pattern, listen-pick) + fill remount restore |

---

## Change Produced

- **Authored by:** `INIT-001/SPEC-006`
- **Title:** Retry-before-reveal on pattern/fill/listen-pick; record first-wrong miss; persist attempts in activityState
- **Files changed:** 3 activity files + 1 smoke test + 2 sdd artifacts (+ spec status)
- **Change status:** proposed
- **Reversibility:** revertible (revert the three activity files + smoke test)

---

## Tests

| Test file | New tests | Notes |
| --------- | --------- | ----- |
| `test/retry-before-reveal-smoke.test.js` | 4 | Headless Chrome: wrong-once/twice, miss, reload restore |

**All tests passing:** yes · `npm test` → exit 0 · **Coverage (new code):** exercised by smoke suite (AAA; real player + review store)

---

## Gates & Controls

None — `gate_actions: []`. Autonomy `auto_tests`. Blocking check `npm test` green. Chrome smoke included in `npm test`.

---

## Security Review

- **Sensitivity / depth:** light · quick · **method:** inline T3 reviewer judgment (`self_reviewed: true` — Cursor harness; no Critical/High)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | no findings |

- **Positive observations:** Review writes fail-soft; no new network/fetch; attempt state is local lesson progress only; HTML via existing `esc` / `speakButton` helpers.

---

## Files Changed

Provenance is `INIT-001/SPEC-006` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `shared/activities/pattern.js` | Retry-before-reveal; miss/`pass` first attempt; attempts in activityState | `INIT-001/SPEC-006` |
| `shared/activities/fill.js` | Same for fill blanks | `INIT-001/SPEC-006` |
| `shared/activities/listen-pick.js` | Same for listen-pick; persist queue/options for restore | `INIT-001/SPEC-006` |
| `test/retry-before-reveal-smoke.test.js` | Created — Chrome smoke | `INIT-001/SPEC-006` |
| `sdd/.../SPEC-006-eval-report.md` | Created | `INIT-001/SPEC-006` |
| `sdd/.../SPEC-006-completion.md` | Created | `INIT-001/SPEC-006` |
| `sdd/.../SPEC-006-frontend-retry-before-reveal.md` | status → `in_review`; ACs covered | `INIT-001/SPEC-006` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `trace_open` / `dispatch` / `trace_complete` | `scripts/dev/` / `lib.sdd_trace` absent in this repo; orchestrator owns timer/trace for this dispatched run | Live `.sdd-trace/SPEC-006.jsonl` not written by this worker | Tier-5 / model-dispatch checks deferred to orchestrator; disclosed |

---

## Effort Tracking

| Field | Value |
| ----- | ----- |
| Estimated | 3.5 hours |
| Actual (this worker) | ~1 session (orchestrator owns timer) |
| Timer ownership | Orchestrator (dispatched sub-agent) |

---

## Deviations

- **Persona:** `.claude/agents/` absent (`agents_dir_absent`) — adopted principal-frontend-developer role from `prompt-selection.md` §1; did not invent AGENT.md.
- **Timer / worktree / branch / git:** skipped — orchestrator-dispatched; Git & Issue-Tracker ops deferred to orchestrator.
- **ADR §5 vs SPEC-006:** ADR prose says “final miss after retries emits miss”; resolved SPEC-006 / aud-1 / dispatch: **record miss on first wrong** — implemented as specified.
- **Fill reload:** remount-from-`collect()` (my-morning has fill data but no fill activity step); pattern and listen-pick use full Chrome reload.
- **Signal directions:** listen-pick `he_en` (ADR table); fill/pattern `en_he` (English prompt → Hebrew response, ADR “as prompted”).
- Spec status set to `in_review` (not `complete`) — Tier-5 / model-dispatch validators unavailable without worker-owned traces; orchestrator/HITL may promote after review.

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4.5 AC verifier | `subagent_dispatch` | T2 (Cursor) | `Task` + readonly + `composer-2.5-fast` (2 loops) |
| Phase 4 security review | reviewer judgment | T3 | Inline light review; `self_reviewed: true` |

## Research grounding

Per `RSCH-003` (corrective feedback / retry prompts) cited in the spec; behavior follows resolved aud-1 (one retry then reveal).
