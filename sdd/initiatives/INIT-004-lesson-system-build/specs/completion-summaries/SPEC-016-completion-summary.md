# SPEC-016 Completion Summary — Builder media library, preview, and validation

**Traceability:** INIT-004/SPEC-016
**Status:** Complete
**Domain:** frontend
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] Media, preview, and validation regions on the builder view (`shared/admin/plan-builder.js`)
- [x] Tests: `test/plan-builder-media.test.js`

## Acceptance Criteria

- [x] ac-1 (REQ-010): choosing a picture on a graded-story block is not offered. On an introduction word, save sends `set_field` for `mediaId` and `alt`
- [x] ac-2 (REQ-007): the generate button stays disabled until a dry-run response is shown and the admin checks confirm. The fake server records no generate call before that
- [x] ac-3 (REQ-003): a publish response that includes an N1 warning shows the id `N1`. A block failure shows that the snapshot was not published
- [x] ac-4 (REQ-006): the approval button is disabled when the alt field is empty

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`, agent `f2d999a3-f3f9-4257-a884-03dd5a6658de`) that did **not** author the code — not self-reported.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-016-eval-report.md`](SPEC-016-eval-report.md) (per-AC evidence + remediation log).

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` PASS · security_review=`standard` FAIL · domain=`frontend` FAIL · autonomy_level=`auto_tests` FAIL ⇒ light_tier_eligible=`false` · **checks skipped:** None

The dispatch prompt did not name a light-tier pre-flight, so the tier is `full`. Cursor Task has no native `readonly` flag; the prompt forbade edits.

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `ac-1: a graded-story block offers no picture; an introduction word saves mediaId and alt`. Story block has no attach control. Introduction save PATCH fields are `mediaId` then `alt`. `node --test test/plan-builder-media.test.js` → 7 pass, exit 0. |
| ac-2 | PASS | `ac-2: generate stays disabled until the dry-run figure is shown and confirmed`. No `POST /api/admin/images` before the dry-run is shown and confirm is checked. Estimate text is the fake response `7.25`. The POST sends `confirm: true` and that same number. |
| ac-3 | PASS | `ac-3: an N1 warning is shown, and a block failure says the snapshot was not published`. Finding text contains `N1`. Refusal text contains `snapshot was not published` and the route error `validation`. Status stays `draft`. |
| ac-4 | PASS | `ac-4: the approval button is disabled while alt text is empty`. Whitespace leaves it disabled and records no approval call. A non-empty alt sends `POST /api/admin/media/coffee-cup/approval`. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-016`
- **Title:** Admin builder media library, confirmed image generation, validation panel, and preview after save
- **Files changed:** 4 · builder media regions; multipart admin fetch; layout rules; fake-fetch tests
- **Change status:** proposed
- **Reversibility:** branch

---

## Decomposition Telemetry (`frontend` domain only — `INIT-064/SPEC-006`)

- **`axis_chosen`:** not captured — `design-corpus/frontend-design.md` is not in this repo, so no §3.1 rule number was applied. The spec's own scope is three regions on the existing plan-builder screen.
- **`predicted_vs_actual_sizing`:** reconstructed from the spec Scope (no recorded band): screens: 1→1 · regions: media library, picture attach, validation, preview 4→4 · states: generate locked until dry-run plus confirm, approval locked while alt is empty, publish refusal.
- **`integration_gate_result`:** folded — ac-1 through ac-4 are this spec's integration checks. 0 — all-PASS on the verifier's first pass. Remediation loops: 0.

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| `test/plan-builder-media.test.js` | 7 | 7 |

**All tests passing:** Yes · `node --test test/plan-builder-media.test.js` 7/7 exit 0 · `node --test test/plan-builder.test.js` 7/7 exit 0 · `npm test` 553 pass, 0 fail, exit 0 · **Coverage (new code):** no coverage tool is configured; the bar is this file plus `npm test`

---

## Gates & Controls

None — branch/reversible only. `gate_actions: []`. Autonomy `auto_tests`. No one-way action taken. No commit, push, or pull request (dispatch standing constraint).

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** independent `security-review` Task (`composer-2.5-fast`, agent `1fb18ed7-69e7-4f67-8b65-f4e114d932cc`), scoped to the SPEC-016 builder, API client, and tests. Cursor Task has no native `readonly` flag. Not self-reviewed. `shared/worksheet.css` is layout tokens only; the reviewer did not include it, and it has no request or secret behavior.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | — |

- **Positive observations:** Upload and generate use `HebrewApi.request` with `credentials: 'same-origin'`. There is no `<form action>`. Generate stays off until the dry-run `estimatedDollars` is on screen and the confirm box is checked; the POST sends `confirm: true` and that number. The client source has no API key. Validation rows use `textContent`. Thumbnail `src` is `/api/media/` plus an id that matches the media slug. Preview HTML is only `HebrewPlanRender.renderBlock`. After the review noted `accept` included GIF, the file input was limited to PNG, JPEG, and WebP, which is what ingest decodes. The server still sniffs the bytes.

---

## Files Changed

Provenance is `INIT-004/SPEC-016` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `shared/admin/plan-builder.js` | Modified — media library, upload, dry-run, confirmed generate, approval, introduction-only picture attach, validation panel, preview after save | `INIT-004/SPEC-016` |
| `shared/api.js` | Modified — a FormData body is posted as multipart with same-origin credentials and no JSON content type | `INIT-004/SPEC-016` |
| `shared/worksheet.css` | Modified — appended layout rules for the media list, findings, checkbox, and estimate | `INIT-004/SPEC-016` |
| `test/plan-builder-media.test.js` | Created — fake fetch for attach, generate gate, N1, approval, upload, and preview | `INIT-004/SPEC-016` |

---

## Harness Adaptation

Cursor Task dispatch (T2) for the AC verifier and the security review. No native `readonly` flag; both prompts forbade edits. Not a degraded tier.

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `trace_open`, `task_step`, `spec_lifecycle`, `dispatch`, `trace_complete` | Dispatched sub-agent. Timer and trace sentinels are owned by the orchestrator (`execute-spec` Phase 0 step 2). This agent did not call `open_trace` / `emit` / `close_trace`. | This process wrote no `.sdd-trace/SPEC-016.jsonl` events. | The orchestrator must emit on return. A Tier-5 check against a missing trace is `FAIL`/`INCONCLUSIVE`, not a pass. |

---

## Deviations from the Spec

- **Persona.** `.claude/agents/` is absent in this repo (`agents_dir_absent`). `principal-frontend-developer` was not loaded and no `AGENT.md` was invented. The page follows the existing builder, ADR-0006 tokens, and `HebrewPlanRender`.
- **Commit / push / PR.** Skipped. The dispatch said not to commit, push, or open a PR.
- **Media list.** SPEC-011 has no list route, and this spec does not add one. The library shows images returned by upload, generate, and approval in this session (pending or approved) plus introduction pictures already on the lesson. Thumbnails use `GET /api/media/:id`.
- **New picture.** `set_field` of `mediaId` and `alt` lands on `image` only when that object already exists. A word without one first sends `set_field` `image`, then `mediaId` and `alt`. The ac-1 word already has an image, so that save is only the two fields.
- **Browser.** No browser tools were used. Behavior was checked with fake-fetch tests and `npm test`, not by clicking the admin page.
- **File hint.** The upload `accept` list is PNG, JPEG, and WebP. Ingest still decides what bytes are stored.

---

## Effort Tracking

**Agent session time:** not recorded here — the orchestrator owns the timer
**Spec effort estimate:** 5 hours
