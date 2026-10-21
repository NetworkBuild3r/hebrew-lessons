# SPEC-004 Completion Summary — Review today page and home queue

**Traceability:** INIT-001/SPEC-004  
**Status:** `in_review`  
**Domain:** frontend  
**Completed Date:** 2026-10-08  
**Persona:** principal-frontend-developer (described — `.claude/agents/` **absent** / `agents_dir_absent`; no AGENT.md invented; executed under described frontend voice)

---

## Deliverables Checklist

- [x] `review/index.html`, `shared/review-page.js`, styles appended to `shared/worksheet.css` (SPEC-004 marker block)
- [x] `index.html` — review card and status logic (aud-1-filtered due/new counts)
- [x] `server/web.js` — `/review/` static allowlist (required so site-root page is reachable)
- [x] Tests: `test/review-page.test.js`, `test/chrome-review-smoke.js`

## Acceptance Criteria

- [x] ac-1: interleaved due items from ≥2 lessons (`my-morning`, `anal-sex-vocab`)
- [x] ac-2: answer hidden until Show answer; Knew/Missed → `HebrewReview.record`; due moves per ADR-0001
- [x] ac-3: both directions after first `he→en` pass; typed `en→he` without nikud
- [x] ac-4: home card due count matches session filter; disappears when nothing is due
- [x] ac-5: script-tag load only; bilingual labels; nikud on by default + prefs toggle
- [x] ac-6: missing vocab index → bilingual “no review data yet”
- [x] XSS: index text escaped (`escapeIndexText` / `esc`)

## AC Verification (independent)

Verified by fresh readonly `composer-2.5-fast` subagents (not the implementer).  
Overall: **PASS** (all-PASS after 1 remediation loop).  
**Full eval report:** [`SPEC-004-eval-report.md`](SPEC-004-eval-report.md).

**Verification tier:** `full` · **pre-flight:** gate_actions `[]` PASS · security_review=`standard` PASS · domain=`frontend` FAIL · autonomy_level=`auto_tests` PASS => light_tier_eligible=`false` · **checks skipped:** None

| AC | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | interleave test + `dueList`/`filterQueue` |
| ac-2 | PASS | hidden answer + Knew/Missed record tests |
| ac-3 | PASS | aud-1 filter + `typedMatches` |
| ac-4 | PASS | home filter parity + hide-when-zero |
| ac-5 | PASS | script tags / bilingual / vowels default |
| ac-6 | PASS | missing-index bilingual empty state |
| XSS | PASS | `escapeIndexText` img onerror fixture |

---

## Change Produced

- **Authored by:** `INIT-001/SPEC-004`
- **Title:** Add Review today page, home due card, and review smoke/unit tests
- **Files changed:** 7 implementation/test + 2 sdd artifacts (+ spec status)
- **Change status:** proposed
- **Reversibility:** revertible (delete `review/`, `shared/review-page.js`; revert `index.html`, `worksheet.css`, `server/web.js`, tests)

---

## Tests

| Test file | New tests | Notes |
| --------- | --------- | ----- |
| `test/review-page.test.js` | 12 | script tags, XSS escape, interleave, aud-1, Knew/Missed, typed en→he, home card, bilingual/vowels, missing index, CSS marker, `/review/` allowlist |
| `test/chrome-review-smoke.js` | 1 script | headless reveal → Knew it → store due advances |

**All tests passing:** yes · `npm test` → **78/78** pass · Chrome smoke: **ok** on PORT 8081  
**Coverage (new code):** exercised by review-page suite + Chrome smoke (AAA; seeded store)

---

## Gates & Controls

None — `gate_actions: []`. Autonomy `auto_tests`. Blocking checks: `npm test` green; review smoke green.

---

## Security Review

- **Sensitivity / depth:** standard · XSS focus · **method:** inline T3 reviewer judgment (`self_reviewed: true` — Cursor harness; no Critical/High)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | no findings |

- **Positive observations:** All vocab/index strings rendered via `esc`/`U.escapeHtml`; no `fetch` of lesson data; `/review/` path containment rejects `..` traversal; localStorage prefs/store only.

---

## Files Changed

Provenance is `INIT-001/SPEC-004` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `review/index.html` | Created — Review today shell + script-tag loads | `INIT-001/SPEC-004` |
| `shared/review-page.js` | Created — session UI, caps via `dueList`, aud-1 filter | `INIT-001/SPEC-004` |
| `shared/worksheet.css` | Appended SPEC-004 review-page section (marker for SPEC-007) | `INIT-001/SPEC-004` |
| `index.html` | Home “חזרה היום” card + due/new counts | `INIT-001/SPEC-004` |
| `server/web.js` | Allowlist `/review/` for static serve | `INIT-001/SPEC-004` |
| `test/review-page.test.js` | Created | `INIT-001/SPEC-004` |
| `test/chrome-review-smoke.js` | Created | `INIT-001/SPEC-004` |
| `sdd/.../SPEC-004-eval-report.md` | Created | `INIT-001/SPEC-004` |
| `sdd/.../SPEC-004-completion.md` | Created | `INIT-001/SPEC-004` |
| `sdd/.../SPEC-004-frontend-review-today-page.md` | status → `in_review` | `INIT-001/SPEC-004` |

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4 security review | subagent_dispatch | T3 | Inline self-reviewed XSS review (`self_reviewed: true`) |
| Phase 4.5 AC verifier | subagent_dispatch | T2 | Fresh `composer-2.5-fast` Task (readonly prompt); remediation loop 1 |

---

## Trace Emission Degradation

Trace library `scripts/dev/lib/sdd_trace` **absent** in this repo — `open_trace`/`emit`/`close_trace` skipped (ModuleNotFoundError). Disclosed; spec work not blocked. Timer owned by orchestrator (dispatched sub-agent) — skipped.

---

## Deviations

- **Git & Issue-Tracker Operations Guard:** skipped branch/stage/commit/push (orchestrator / guarded dispatch).
- **Persona:** `.claude/agents/` absent (`agents_dir_absent`) — disclosed; adopted described `principal-frontend-developer` voice without inventing AGENT.md.
- **Owned-file expansion:** `server/web.js` updated so `/review/index.html` is reachable (static allowlist previously blocked it). Required for aud-2 page path and smoke.
- **Research grounding:** caps/interleave per ADR-0001 / `RSCH-003` (advisory).

---

## Understood (intent)

- und-1: Answer never visible before reveal — implemented (`review-answer` starts `hidden`; grade gated on `revealed`).
- und-2: Words met only in lesson round 1 still appear once due — via store/`dueList` (no special exclusion).
