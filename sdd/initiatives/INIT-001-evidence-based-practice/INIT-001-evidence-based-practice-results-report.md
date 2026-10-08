# Implementation Results Report: Evidence-Based Practice

**Report ID:** INIT-001-evidence-based-practice · **Status:** Complete \
**Initiative:** INIT-001-evidence-based-practice (SDD `create-initiative` → `execute-spec` → `complete-initiative`) · **Repository:** Hebrew (local working tree, uncommitted; no PR opened) \
**Completed:** 2026-10-08 · **Prepared by:** Cursor agent (Spec-Driven Development) \
**Outcome:** Delivered. All 9 specs pass independent AC verification. One High is open and risk-accepted until 2026-10-15 (API key rotation). · **Confidence:** 0.85

---

## TL;DR

Nine specs turned the RSCH-003 research into a spaced-review system, production recall, retry feedback, tap-to-gloss, and authoring checks, with 114 passing tests. The process caught a High-severity code-execution path in the lesson loaders during the security review, and that path is now fixed. Agent time was not recorded, because the timer tooling is absent from this repo.

---

## Executive Summary

**What was built.** A "Review today" page at `/review/` with a home due card, backed by a browser-side store and an ADR-0001 scheduler (1, 3, 7, 14, 30 day steps). Word rounds now include production recall (English to Hebrew). Pattern, fill, and listen-pick each allow one retry before revealing the answer. Stories support tap-to-gloss, an estimated coverage badge, and a `not_yet` signal. A vocabulary index (`worksheets/_review/vocab-index.js`) merges words across lessons. The authoring validator adds nudges for recycling, encounters, coverage, and glosses. The learner-review model is recorded in `docs/decisions/ADR-0001-learner-review-model.md`. Specs: SPEC-001 to SPEC-009 (see the completion report's Spec Summary).

**What it cost.** 9 specs. Session and agent time: **not recorded** (no `.sdd-timer.json`; `scripts/dev/` absent). Tests: 114 passing, up from the 94 counted at SPEC-007 close, plus 20 added in the security remediation. Quality gate: `npm test` 114/114. Remediation loops: 3 (one each on SPEC-004, SPEC-005, SPEC-007).

**What the process caught.** The security review found that lesson loaders evaluated `worksheet.js` in Node `vm`, which allowed code execution in the server process and access to the ElevenLabs key. It was fixed before completion, with a literal-only loader and regression tests. The earlier `hostile` test passed only because it checked for `require`, not for the constructor escape. The new tests close that gap. Independent verifiers also caught and fixed defects in SPEC-004, SPEC-005, and SPEC-007 before they completed.

**Security and compliance posture.** Final severity: Critical 0 · High 2 (1 fixed, 1 operator action open and risk-accepted to 2026-10-15) · Medium 1 · Low 2. Positive observations: the static-file allowlist rejects traversal and dotfiles, every DOM sink reviewed escapes its input, and the API key is sent only in a request header and never logged.

**Bottom line.** Nine specs and one security fix shipped with independent verification on every spec. Cost is not measured, so efficiency cannot yet be stated. Restoring the timer tooling is the first thing to fix before the next initiative.

---

## 1. What the Process Caught

- **Security review (Phase 3.5), HIGH-1:** lesson loaders executed `worksheet.js` in Node `vm`, which allowed escape to `process.env` and the ElevenLabs key. Caught before completion. The fix is a literal-only parser (`server/lesson-data.js`) with 20 regression tests.
- **Independent verification (SPEC-004, SPEC-005, SPEC-007):** each needed one remediation loop before its acceptance criteria all passed.

---

## 2. Methodology — How the Code Was Produced

### 2.1 Design-first and contract-first

Frontend work followed the existing shared player structure, with activity modules under `shared/activities/` and the review store as a shared module. Service work (the index generator, validator nudges) was contract-first: the ADR-0001 identity key (NFC plus whitespace collapse, with a unique vowel-stripped fallback) was fixed before the code. Frontend specs ran as `principal-frontend-developer`; the `.claude/agents/` personas were absent, so the role descriptions were used without inventing AGENT.md.

### 2.2 Standards-conformant implementation

ADR-0001 governed the learner-review model. Research grounding came from RSCH-003 (Moderate confidence). Reuse-first: the existing `words`, `type-it`, `pattern`, `fill`, and `listen-pick` activities were extended, not rebuilt. The lesson-data loader was consolidated into one module, and the existing `vm` path was kept only for trusted repo code.

### 2.3 The quality gate every spec had to clear

| Gate | What it checks | Result across the initiative |
| ---- | -------------- | ---------------------------- |
| `npm test` | Unit and headless Chrome smoke tests | PASS: 114 tests, 0 failures |
| Suppression scan | Lint-disable and type-ignore escape hatches | N/A (JavaScript; no such config) |
| Stale-reference sweep | Renamed or removed targets updated | Clean; loaders delegate to `loadDataWindow` |
| Independent AC verification | Fresh sub-agent re-derives evidence for each AC | 9/9 specs PASS |
| Security review | Initiative-wide, targeted | 0 Critical open; 2 High (1 fixed, 1 risk-accepted); 1 Medium; 2 Low |

**No exceptions were taken to reach these results.** No bypass flags, no weakened acceptance criteria, no test changes to force a pass. The remediation fix corrects the actual behavior; it did not weaken a check. The one risk acceptance (SEC-OPS-1) is a recorded decision by the operator, not a bypass of a test.

### 2.4 Efficiency evidence

| Metric | Value |
| ------ | ----- |
| Total specs | 9 (docs 2, service 2, frontend 5) |
| Total sessions | not recorded |
| Total agent time | not recorded |
| Average / median session | not recorded |
| Remediation loops | 3 (SPEC-004, SPEC-005, SPEC-007) plus the Phase 3.5 remediation |
| Wall-clock span | not recorded |

---

## 3. Results — Spec-by-Spec Delivery

| Spec | Domain | Title | Status | Agent time | New tests |
| ---- | ------ | ----- | ------ | ---------- | --------- |
| SPEC-001 | docs | Accept ADR-0001 learner review model | complete | not recorded | 0 |
| SPEC-002 | service | Vocab index generator and guarded rebuild | complete | not recorded | 6 |
| SPEC-003 | frontend | Review store and scheduler; words self-marks | complete | not recorded | 19 |
| SPEC-004 | frontend | Review today page and home card | complete | not recorded | 12 + smoke |
| SPEC-005 | frontend | Production recall; type-it intermediate | complete | not recorded | not itemized + smoke |
| SPEC-006 | frontend | Retry before reveal | complete | not recorded | 4 (smoke) |
| SPEC-007 | frontend | Story glosses and coverage | complete | not recorded | 16 + smoke |
| SPEC-008 | service | Validator nudges | complete | not recorded | 8 |
| SPEC-009 | docs | Authoring guide | complete | not recorded | 0 |
| **Total** | | | **9/9 complete** | **not recorded** | **114 suite pass (incl. 20 remediation)** |

---

## 4. Design References

No design file was used. The frontend specs were built from the existing shared styles (`shared/worksheet.css`) and the player's own components, so this section has no design-reference table.

---

## 5. Quality & Verification Evidence

- **Independent AC verification (9/9 specs PASS):** SPEC-004, 005, and 007 each recorded one remediation loop before passing. The other six specs passed with no remediation loop recorded in their eval reports. Eval reports are in `specs/completion-summaries/`.
- **Security review — Verdict: MITIGATE (one operator action open):** Critical 0 · High 2 · Medium 1 · Low 2. The High code-execution path is fixed and tested. The open High (exposed API key) is risk-accepted to 2026-10-15 and requires rotation. Positive observations: traversal and dotfile rejection in the static server; escaping in every DOM sink reviewed; the key is sent only in a header and never logged.
- **Test evidence:** 114 tests, 0 failures, for `npm test`. Coverage was not measured (no coverage tool is configured).
- **Quality sweep:** `npm test` is the repo's gate. `prek` and `nox` are not configured in this repo. Syntax check passes on all changed loader files.

---

## 6. Risks & Follow-Ups

| Item | Severity | Status |
| ---- | -------- | ------ |
| SEC-OPS-1: ElevenLabs API key exposed in tool output; rotate `ELEVENLABS_API_KEY` | High | open; risk-accepted to 2026-10-15 |
| MED-1: MCP write tool runs without a bearer token in compose (`MCP_ALLOW_NO_AUTH=1`) | Medium | open, follow-up |
| LOW-1: container runs as root (pre-existing) | Low | open, follow-up |
| LOW-2: static web server has no authentication (pre-existing) | Low | open, follow-up |
| HIGH-1 independent re-review of `server/lesson-data.js` | — | recommended before merge |
| Timer, trace, dehydration, and security-registry tooling absent (`scripts/dev/`) | — | open; restore before the next initiative |
| Working tree contains unrelated uncommitted work (ElevenLabs audio, Compose, personal lesson) | — | split into separate PRs |
| Research citations [27] and [38] edited after the last verifier pass | — | disclosed in RSCH-003 |

---

## 7. Conclusion & Recommendation

The initiative delivered what RSCH-003 recommended: spaced recall, production recall, retry feedback, glosses, and authoring checks. Every spec passed independent verification, and the security review found and fixed a real code-execution path before completion. Two things need action before merge: rotate the exposed ElevenLabs key (the risk acceptance expires 2026-10-15), and get an independent re-review of the new loader. Recommended next steps: restore the timer tooling so future efficiency can be measured, and tighten the MCP auth setting in compose.

---

**Rendering:** the branded PDF was not produced. `pandoc` is not on PATH in this environment, so `render_report_pdf.sh` was skipped ("Results report PDF skipped: pandoc not found"). The markdown source is complete.
