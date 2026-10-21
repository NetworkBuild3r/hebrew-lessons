# SPEC-005 Completion Summary — Production recall in lessons

**Traceability:** INIT-001/SPEC-005  
**Status:** `in_review`  
**Domain:** frontend  
**Completed Date:** 2026-10-08  
**Persona:** principal-frontend-developer (described — `.claude/agents/` absent / `agents_dir_absent`; no AGENT.md invented)

---

## Deliverables Checklist

- [x] `shared/activities/words.js` — recall rounds alternate / force he↔en; English-first cards; `en_he` marks
- [x] `shared/activities/type-it.js` — intermediate+; show-letters hint; `en_he` pass/miss
- [x] `shared/activities/catalog.js` — type-it levels + `typeAllowed` for intermediate
- [x] `test/validate-activities.test.js`, `test/production-recall.test.js`, `test/chrome-production-recall-smoke.js`

## Acceptance Criteria

- [x] ac-1: English-first recall card; Hebrew hidden until reveal; grades `en→he`
- [x] ac-2: type-it at intermediate; nikud-insensitive; records `en→he`
- [x] ac-3: round 1 unchanged (Hebrew + meaning both shown)
- [x] ac-4: no lesson `index.html` / worksheet edits required for new words behavior

## AC Verification (independent)

Verified by fresh readonly `composer-2.5-fast` Task subagents (not the implementer).  
Overall: **PASS** (all-PASS after 1 remediation loop on ac-2/ac-3 evidence).  
**Full eval report:** [`SPEC-005-eval-report.md`](SPEC-005-eval-report.md).

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` PASS · security_review=`light` PASS · domain=`frontend` FAIL · autonomy_level=`auto_tests` PASS ⇒ light_tier_eligible=`false` · **checks skipped:** None

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | words en_he UI + record; production-recall tests; Chrome smoke ok |
| ac-2 | PASS | catalog/available intermediate; normalizeHebrew + en_he record tests |
| ac-3 | PASS | round-1 dual-display regression test |
| ac-4 | PASS | shared activities only; my-morning smoke without lesson edits |

---

## Change Produced

- **Authored by:** `INIT-001/SPEC-005`
- **Title:** Production (English→Hebrew) recall in words rounds; widen type-it to intermediate with en_he signals
- **Files changed:** 6 implementation/test + 2 sdd artifacts (+ spec status)
- **Change status:** proposed
- **Reversibility:** revertible (revert the three activity files + tests)

---

## Decomposition Telemetry (`frontend` domain only — `INIT-064/SPEC-006`)

- **`axis_chosen`:** Rule — shared activity components (words card + type-it) → component-axis spec (0 new screens).
- **`predicted_vs_actual_sizing`:** screens: 0→0 · components: 2→2 (`words`, `type-it`) · states: recognition / receptive / productive / revealed / marked → delivered (retroactive from Scope sizing band).
- **`integration_gate_result`:** folded — this spec's ACs carry integration with `ctx.review` (`en_he`); 0 integration/a11y AC failures after loop 1 · remediation loops: 1.

---

## Tests

| Test file | New / updated | Notes |
| --------- | ------------- | ----- |
| `test/production-recall.test.js` | Created | direction, type-it gate/match/record, round-1 HTML |
| `test/validate-activities.test.js` | Updated | type-it intermediate levels |
| `test/chrome-production-recall-smoke.js` | Created | headless en_he card + review store |

**All tests passing:** yes · `npm test` → **78/78** pass · Chrome smoke → ok  
**Coverage (new code):** exercised by production-recall suite + smoke (no coverage tool wired)

---

## Gates & Controls

None — `gate_actions: []`. Autonomy `auto_tests`. Blocking checks: `npm test` green; Chrome smoke green.

---

## Security Review

- **Sensitivity / depth:** light · quick · **method:** inline T3 reviewer judgment (`self_reviewed: true` — Cursor harness; no Critical/High)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | — |

- **Positive observations:** HTML via `ctx.esc`; review `record` fail-soft try/catch; no fetch; no secrets; direction labels bilingual via `ctx.bi`.

---

## Files Changed

| File | Change | Authored by |
| --------------------- | ----------------------- | ------------------- |
| `shared/activities/words.js` | Modified — production recall + direction switch | `INIT-001/SPEC-005` |
| `shared/activities/type-it.js` | Modified — intermediate gate, hint, en_he record | `INIT-001/SPEC-005` |
| `shared/activities/catalog.js` | Modified — type-it levels / typeAllowed | `INIT-001/SPEC-005` |
| `test/validate-activities.test.js` | Modified — intermediate type-it assertion | `INIT-001/SPEC-005` |
| `test/production-recall.test.js` | Created — unit coverage | `INIT-001/SPEC-005` |
| `test/chrome-production-recall-smoke.js` | Created — headless en_he smoke | `INIT-001/SPEC-005` |
| `…/SPEC-005-eval-report.md` | Created — AC eval report | `INIT-001/SPEC-005` |
| `…/SPEC-005-completion.md` | Created — this summary | `INIT-001/SPEC-005` |
| `…/SPEC-005-frontend-production-recall.md` | Modified — status → `in_review` | `INIT-001/SPEC-005` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `trace_open` / `dispatch` / `trace_complete` / emits | Orchestrator-owned (dispatched sub-agent); `scripts/dev/lib/sdd_trace` not invoked here | Entire run-trace for SPEC-005 from this agent | Tier-5 / model-dispatch checks deferred to orchestrator; spec left `in_review` |

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4.5 AC verifier | `subagent_dispatch` | T2 (Cursor) | Fresh `Task` + `composer-2.5-fast`; prompt-enforced read-only (no native `readonly` flag on Cursor Task schema) |
| Phase 4 security review | reviewer judgment | T3 | Inline light review · `self_reviewed: true` |

---

## Deviations from the Spec

- **Persona:** `.claude/agents/` absent (`agents_dir_absent`) — executed as described `principal-frontend-developer`; no AGENT.md fabricated.
- **Git / worktree / branch / timer:** skipped — Git & Issue-Tracker guard + orchestrator-owned timer/trace (dispatched sub-agent).
- **Completion summary filename:** `SPEC-005-completion.md` (initiative convention, matching SPEC-002/003).
- **Spec status:** `in_review` (not `complete`) — Tier-5 / model-dispatch validators not run locally without orchestrator trace ownership.
- Research grounding (advisory): direction independence per `RSCH-003` / ADR-0001 signal table.

---

## Effort Tracking

Estimated ~3–4 hours (timer unavailable — orchestrator-owned / `scripts/dev` timer not started here).  
**Spec effort estimate:** 4 hours
