# SPEC-015 Completion Summary — Builder outline and block editors

**Traceability:** INIT-004/SPEC-015
**Status:** Complete
**Domain:** frontend
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] Admin builder view for outline and fields (`shared/admin/plan-builder.js`, opened from Admin → Plans, `#plan/<slug>`)
- [x] Tests: `test/plan-builder.test.js`

## Acceptance Criteria

- [x] ac-1 (REQ-010): insert, move, remove, field save, and publish call the SPEC-004 routes and do not patch the plan document in the browser
- [x] ac-2 (REQ-002): a 409 leaves the textarea value in place and shows `revision_conflict` plus the server revision
- [x] ac-3 (REQ-010): a non-admin session does not receive the builder markup
- [x] ac-4 (REQ-014): `data-lesson-save` is still on the legacy lesson editor

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`, agent `22e87cbe-29d2-464a-ae69-44178c7d6c90`) that did **not** author the code — not self-reported.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-015-eval-report.md`](SPEC-015-eval-report.md) (per-AC evidence + remediation log).

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` PASS · security_review=`standard` FAIL · domain=`frontend` FAIL · autonomy_level=`auto_tests` FAIL ⇒ light_tier_eligible=`false` · **checks skipped:** None

The dispatch prompt did not name a light-tier pre-flight, so the tier is `full`. Cursor Task has no native `readonly` flag; the prompt forbade edits.

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: insert, move, remove, field save, and publish call the plan routes`. Five write URLs match SPEC-004. Bodies omit `plan` and `units`. `node --test test/plan-builder.test.js` → 7 pass, exit 0. |
| ac-2 | PASS | `ac-2: a 409 keeps the textarea and names revision_conflict plus the stored revision`. Draft stays `ADMIN DRAFT`. Conflict text contains `revision_conflict` and `9`. |
| ac-3 | PASS | `ac-3: a non-admin session does not get the builder from the admin shell`. Role `user` on `/admin/` gets `No access` and the shell does not mount. `plan-builder.js` is only on the admin route. |
| ac-4 | PASS | `keeps the legacy save hook and does not post to upsert_lesson`. `data-lesson-save` remains in `shared/admin/lesson-editor.js`. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-015`
- **Title:** Admin outline that inserts, moves, removes, edits, and publishes a lesson plan through the plan HTTP routes
- **Files changed:** 5 · new builder module and tests; admin shell hash and tab; admin route scripts; builder layout rules
- **Change status:** proposed
- **Reversibility:** branch

---

## Decomposition Telemetry (`frontend` domain only — `INIT-064/SPEC-006`)

- **`axis_chosen`:** not captured — `design-corpus/frontend-design.md` was not opened this run. The spec's aud-1 already fixed the page as a new admin view beside the lesson editor. Execution followed that: one outline inside the existing `/admin/` shell.
- **`predicted_vs_actual_sizing`:** screens: 1→1 (plan outline, plus a plan-id picker on `#plans`) · components: outline rows, field textareas, one preview region · states: loading, conflict, invalid id. Media and the N1–N12 panel stay on SPEC-016.
- **`integration_gate_result`:** folded — ac-1 through ac-4 are this spec's integration checks. 0 — all-PASS on first pass. Remediation loops: 0.

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| `test/plan-builder.test.js` | 7 | 7 |

**All tests passing:** Yes · `node --test test/plan-builder.test.js` 7/7 exit 0 · `npm test` 495 pass, 0 fail, 2 todo, exit 0 · **Coverage (new code):** no coverage tool is configured; the bar is this file plus `npm test`

---

## Gates & Controls

None — branch/reversible only. `gate_actions: []`. Autonomy `auto_tests`. No one-way action taken. No commit, push, or pull request (dispatch standing constraint).

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** independent `security-review` Task (`composer-2.5-fast`, agent `af1dfc09-58d4-44ea-ab86-49cb008b146a`), scoped to the SPEC-015 files. Cursor Task has no native `readonly` flag. Not self-reviewed.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | — |

- **Positive observations:** Writes go through `HebrewApi` with `credentials: 'same-origin'` and `Content-Type: application/json`, the same CSRF gate as other admin posts. `expectedRevision` is the integer from the last successful response, not the toolbar text. Field values are `textarea.value` / `textContent`. Preview HTML is only `HebrewPlanRender.renderBlock`. Plan ids and part paths are checked against the slug and id-path patterns before a request. `/admin/` still returns 403 for role `user`, and the plan routes stay admin-only.

---

## Files Changed

Provenance is `INIT-004/SPEC-015` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `shared/admin/plan-builder.js` | Created — outline, field editors, publish, conflict alert, preview | `INIT-004/SPEC-015` |
| `shared/admin/admin-shell.js` | Plans tab and `#plan/<slug>` / `#plans` routes beside Lessons | `INIT-004/SPEC-015` |
| `shared/app-routes.js` | Admin route loads `plan-render.js` and `plan-builder.js` before the shell | `INIT-004/SPEC-015` |
| `shared/worksheet.css` | Appended layout rules for `[data-plan-builder]`, using existing tokens | `INIT-004/SPEC-015` |
| `test/plan-builder.test.js` | Created — route parity, 409 draft, escape, non-admin guard, legacy hook | `INIT-004/SPEC-015` |

---

## Harness Adaptation

Cursor Task dispatch (T2) for the AC verifier and the security review. No native `readonly` flag; both prompts forbade edits. Not a degraded tier.

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `trace_open`, `task_step`, `spec_lifecycle`, `dispatch`, `trace_complete` | Dispatched sub-agent. Timer and trace sentinels are owned by the orchestrator (`execute-spec` Phase 0 step 2). This agent did not call `open_trace` / `emit` / `close_trace`. | This process wrote no `.sdd-trace/SPEC-015.jsonl` events. | The orchestrator must emit on return. A Tier-5 check against a missing trace is `FAIL`/`INCONCLUSIVE`, not a pass. |

---

## Deviations from the Spec

- **Persona.** `.claude/agents/` is absent in this repo (`agents_dir_absent`). `principal-frontend-developer` was not loaded and no `AGENT.md` was invented. The page follows the existing admin shell, ADR-0006 tokens, and `HebrewPlanRender`.
- **Commit / push / PR.** Skipped. The dispatch said not to commit, push, or open a PR.
- **Where the page lives.** A hash view on the existing admin route (`#plans`, `#plan/<slug>`), not a new top-level HTML file. The lesson editor is unchanged.
- **Preview timing.** The preview is the block from the last `GET /api/plans/:id/parts` read (and again after a successful write, which reloads that part). Unsaved textarea text is not previewed. SPEC-016 owns the media library, the N1–N12 panel, and live preview after save.
- **Browser.** No browser tools were available in this session. Behavior was checked with `node --test test/plan-builder.test.js` and `npm test`, not by clicking the admin page.
- **Static script.** `/shared/admin/plan-builder.js` is served to any signed-in user, the same way `lesson-editor.js` is. The page markup is not sent for `/admin/` unless the role is admin, and the plan APIs reject non-admins.

---

## Effort Tracking

**Agent session time:** not recorded here — the orchestrator owns the timer
**Spec effort estimate:** 5 hours
