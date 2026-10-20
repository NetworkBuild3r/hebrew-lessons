# SPEC-007 Completion Summary — Story glosses and coverage estimate

**Traceability:** INIT-001/SPEC-007  
**Status:** `in_review`  
**Domain:** frontend  
**Completed Date:** 2026-10-08  
**Persona:** principal-frontend-developer (described — `.claude/agents/` **absent** / `agents_dir_absent`; no AGENT.md invented; executed under described frontend voice)

---

## Deliverables Checklist

- [x] `shared/activities/story.js` — tap-to-gloss popover, `not_yet` signal, coverage badge
- [x] `shared/player/utils.js` — `tokenizeHebrew(text)` (maqaf/geresh stay with word)
- [x] `shared/worksheet.css` — SPEC-007 block appended after SPEC-004 end marker only
- [x] Tests: `test/tokenize-hebrew.test.js`, `test/story-gloss.test.js`, `test/chrome-story-gloss-smoke.js`

## Acceptance Criteria

- [x] ac-1 (REQ-006): tap word with gloss/vocab → English meaning; tap elsewhere closes
- [x] ac-2 (REQ-006): each tap records `not_yet` (at most once per word per day; no lapse / no higher-step reset)
- [x] ac-3 (REQ-007): coverage estimate in story header; empty store → unknown (not 0%)
- [x] ac-4 (REQ-009): lessons without `reading.glosses` still gloss vocab words
- [x] ac-5 (security): gloss text escaped via `U.escapeHtml`

## AC Verification (independent)

Verified by fresh readonly `composer-2.5-fast` subagents (not the implementer).  
Overall: **PASS** (all-PASS after 1 remediation loop).  
**Full eval report:** [`SPEC-007-eval-report.md`](SPEC-007-eval-report.md).

**Verification tier:** `full` · **pre-flight:** gate_actions `[]` PASS · security_review=`standard` PASS · domain=`frontend` FAIL · autonomy_level=`auto_tests` PASS => light_tier_eligible=`false` · **checks skipped:** None

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | gloss/vocab popover + tap-elsewhere unit + smoke |
| ac-2 | PASS | `recordNotYet` once/day; higher step preserved; smoke `not_yet` |
| ac-3 | PASS | unknown on empty store; % after seeded pass |
| ac-4 | PASS | my-morning vocab fallback without gloss map |
| ac-5 | PASS | `escapeHtml` fixture + smoke |

---

## Change Produced

- **Authored by:** `INIT-001/SPEC-007`
- **Title:** Add story tap-to-gloss, not_yet review signal, coverage estimate badge, and shared tokenizeHebrew
- **Files changed:** 6 implementation/test + 2 sdd artifacts (+ spec status)
- **Change status:** proposed
- **Reversibility:** revertible (revert `story.js`, `utils.js`, CSS append, delete SPEC-007 tests)

---

## Tests

| Test file | New tests | Notes |
| --------- | --------- | ----- |
| `test/tokenize-hebrew.test.js` | 8 | whitespace, punctuation, maqaf, geresh, finals, blank, NFC |
| `test/story-gloss.test.js` | 7 | elsewhere dismiss, not_yet once/day, coverage, XSS, CSS marker |
| `test/chrome-story-gloss-smoke.js` | 1 script | my-morning story tap → meaning, not_yet, Escape + elsewhere |

**All tests passing:** yes · `npm test` → **94/94** pass · Chrome smoke: **ok** on PORT 8080  
**Coverage (new code):** no coverage tool wired; tokenizer + gloss/coverage paths exercised by unit + smoke (per initiative)

---

## Gates & Controls

None — `gate_actions: []`. Autonomy `auto_tests`. Blocking checks: `npm test` green; story gloss smoke green.

---

## Security Review

- **Sensitivity / depth:** standard · XSS focus · **method:** inline T3 reviewer judgment (`self_reviewed: true` — Cursor harness; no Critical/High)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | no findings |

- **Positive observations:** Gloss/meaning/token attributes rendered via `U.escapeHtml`; no `innerHTML` of raw author gloss; no lesson file edits; review writes use existing `HebrewReview.record` fail-soft path.

---

## Files Changed

Provenance is `INIT-001/SPEC-007` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `shared/player/utils.js` | Added `tokenizeHebrew` | `INIT-001/SPEC-007` |
| `shared/activities/story.js` | Tap-to-gloss, coverage badge, not_yet | `INIT-001/SPEC-007` |
| `shared/worksheet.css` | Appended SPEC-007 block after SPEC-004 end marker | `INIT-001/SPEC-007` |
| `test/tokenize-hebrew.test.js` | Created | `INIT-001/SPEC-007` |
| `test/story-gloss.test.js` | Created | `INIT-001/SPEC-007` |
| `test/chrome-story-gloss-smoke.js` | Created | `INIT-001/SPEC-007` |
| `sdd/.../SPEC-007-eval-report.md` | Created | `INIT-001/SPEC-007` |
| `sdd/.../SPEC-007-completion-summary.md` | Created | `INIT-001/SPEC-007` |
| `sdd/.../SPEC-007-frontend-story-glosses-coverage.md` | status → `in_review` | `INIT-001/SPEC-007` |

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4 security review | subagent_dispatch | T3 | Inline self-reviewed XSS review (`self_reviewed: true`) |
| Phase 4.5 AC verifier | subagent_dispatch | T2 | Fresh `composer-2.5-fast` Task (readonly prompt); remediation loop 1 |

---

## Trace Emission Degradation

Trace library `scripts/dev/lib/sdd_trace` **absent** in this repo — `open_trace`/`emit`/`close_trace` skipped. Disclosed; spec work not blocked. Timer owned by orchestrator (dispatched sub-agent) — skipped.

---

## Deviations

- **Git & Issue-Tracker Operations Guard:** git branch/stage/commit/push skipped (orchestrator-dispatched / guard active).
- **Persona:** `.claude/agents/` absent — disclosed; executed as principal-frontend-developer voice without inventing AGENT.md.
- **SPEC-008:** left local tokenizer + TODO in `validate.js` (prefer not to edit validate.js); shared `tokenizeHebrew` landed in `utils.js` for future consolidate.
- Research grounding: `RSCH-003` (glosses / coverage) via initiative + ADR-0001.

---

## Understood (from spec)

- [x] und-1: Glosses are opt-in per tap; story never shows English underneath by default.
- [x] und-2: Coverage is a guide for lesson choice, not a grade.
