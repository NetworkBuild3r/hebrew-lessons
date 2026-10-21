# SPEC-002 Completion Summary — Cross-lesson vocabulary index generator

**Traceability:** INIT-001/SPEC-002
**Status:** Partial (implementation done; status `in_review` — Tier-5 / model-dispatch validators unavailable without `scripts/dev`)
**Domain:** service
**Completed Date:** 2026-10-08

---

## Deliverables Checklist

- [x] `server/vocab-index.js`
- [x] `scripts/build-vocab-index.js` and `package.json` script `vocab-index`
- [x] `mcp/tools.js` — guarded `rebuildVocabIndexSafe()` after audio / link-only paths in `upsert_lesson`
- [x] `test/vocab-index.test.js`

## Acceptance Criteria

- [x] ac-1: CLI writes `worksheets/_review/vocab-index.js`; shared words merge with both slugs
- [x] ac-2: shared audio URL attached from word-audio catalog when present
- [x] ac-3: malformed / missing-vocab lessons skipped with warnings; index still written
- [x] ac-4: `upsert_lesson` rebuilds via `rebuildVocabIndexSafe`; builder throw never fails publish
- [x] ac-5: reads confined to `LESSONS_DIR`; sole write `LESSONS_DIR/_review/vocab-index.js`; `JSON.stringify` output

## AC Verification (independent)

Verified by a fresh readonly explore Task (model `composer-2.5-fast`, agent `b5c13251-5b91-41d8-860e-829edf65354d`) that did **not** author the code — not self-reported.
Overall: **PASS** (all-PASS) · remediation loops: 0.
**Full eval report:** [`SPEC-002-eval-report.md`](SPEC-002-eval-report.md)

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` PASS · security_review=`standard` FAIL · domain=`service` FAIL · autonomy_level=`auto_tests` FAIL ⇒ light_tier_eligible=`false` · **checks skipped:** None

| AC   | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS    | `npm run vocab-index` → 27 words; `קָמָה` lessons anal-sex-vocab + my-morning; tests merge + repo CLI ✔ |
| ac-2 | PASS    | `server/vocab-index.js` catalogAudioFor; test `attaches shared audio…` ✔ |
| ac-3 | PASS    | skip+warn for throw/novocab; test fixtures broken/hostile/novocab ✔ |
| ac-4 | PASS    | `mcp/tools.js:125` + `rebuildVocabIndexSafe` unit + wiring test ✔ |
| ac-5 | PASS    | resolveLessonDir escape guard; write `_review/vocab-index.js` only; `JSON.stringify` |

---

## Change Produced

- **Authored by:** `INIT-001/SPEC-002`
- **Title:** Cross-lesson vocab index generator, CLI, and guarded upsert_lesson rebuild hook
- **Files changed:** 7 · additions [~550] · deletions [~20]
- **Change status:** proposed
- **Reversibility:** revertible

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| `test/vocab-index.test.js` | 6 | 6 |

**All tests passing:** yes for SPEC-002 scope (`node --test test/vocab-index.test.js` → 6/6) · **Coverage (new code):** no coverage tool wired (per Verification Strategy)
**Full suite note:** `npm test` may show unrelated SPEC-003 `review-store` deepEqual quirks; vocab-index suite is green in isolation.

---

## Gates & Controls

None — `gate_actions: []`. Autonomy `auto_tests`. Blocking check `npm test` (vocab-index scope green). No one-way action taken.

---

## Security Review

- **Sensitivity / depth:** standard · focus: data, injection · **method:** inline implementer review (`self_reviewed: true` — Cursor T3 disclosure; no separate security Task this turn)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None
- **SAST / secret scan:** not run — none configured for this repo

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| —       | —        | —             | —        | no findings |

- **Positive observations:** `vm` sandbox exposes only `window`/`console` (mirrors `audio.js`); no `require`/`process`; path resolution confined to `LESSONS_DIR` readdir names; output via `JSON.stringify`; no ElevenLabs/env material in index.

---

## Files Changed

Provenance is `INIT-001/SPEC-002` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `server/vocab-index.js` | Created — index builder | `INIT-001/SPEC-002` |
| `scripts/build-vocab-index.js` | Created — CLI | `INIT-001/SPEC-002` |
| `package.json` | Modified — `vocab-index` script | `INIT-001/SPEC-002` |
| `mcp/tools.js` | Modified — guarded rebuild after upsert audio paths | `INIT-001/SPEC-002` |
| `test/vocab-index.test.js` | Created — fixture tests | `INIT-001/SPEC-002` |
| `worksheets/_review/vocab-index.js` | Generated — published index artifact | `INIT-001/SPEC-002` |
| `sdd/initiatives/…/SPEC-002-eval-report.md` | Created — AC eval report | `INIT-001/SPEC-002` |
| `sdd/initiatives/…/SPEC-002-completion.md` | Created — this summary | `INIT-001/SPEC-002` |
| `sdd/initiatives/…/SPEC-002-service-vocab-index-generator.md` | Modified — status → `in_review` | `INIT-001/SPEC-002` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `trace_open` / `dispatch` / `trace_complete` / all emits | `scripts/dev/` absent — no `lib/sdd_trace`, no timer scripts | Entire run-trace for SPEC-002 | Tier-5 invariants + model-dispatch-conformance checks cannot run; disclosed; spec left `in_review` |
| Phase 0 | timer start/complete | same — `scripts/dev/` absent | Effort minutes in `.sdd-timer.json` | Effort Tracking estimated only |

---

## Harness Adaptation

- **Persona:** `.claude/agents/` tree absent (`agents_dir_absent`). Disclosed; adopted **principal-backend-developer** role from `prompt-selection.md` §1 (service domain) without inventing `AGENT.md`. `scripts/dev/expertise_state.py` also absent — dispatch-check skipped.
- **Timer / trace:** `scripts/dev/` absent — Start/Complete Timer and `open_trace`/`emit`/`close_trace` skipped; this worker wrote the completion summary per dispatch adaptation.
- **Worktree / branch:** shared main checkout; in-place escape hatch (orchestrator). No commit/push/PR.
- **AC verifier:** Cursor T2 via explore Task (readonly, `composer-2.5-fast`) — independent PASS.
- **Security review:** standard inline (`self_reviewed: true`) — zero Critical/High.

---

## Deviations from the Spec

- Completion summary filename per initiative convention: `SPEC-002-completion.md` (not `SPEC-002-completion-summary.md`).
- Spec status set to `in_review` (not `complete`) — Tier-5 / model-dispatch validators unavailable without traces; orchestrator/HITL may promote after review.
- aud-1: `vm` sandbox (no require/process) — same pattern as `server/audio.js` `loadWindow`.

---

## Effort Tracking

Estimated ~2.5–3.5 hours (timer unavailable — `scripts/dev/` absent).
