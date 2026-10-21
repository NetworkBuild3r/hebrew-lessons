# SPEC-014 Completion Summary — Today, plan, and six block renderers

**Traceability:** INIT-004/SPEC-014
**Status:** Complete
**Domain:** frontend
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] `shared/ui/` component functions (`shared/ui/plan-render.js`)
- [x] Lesson and Today entry that fetches the published snapshot when signed in (`shared/plan/index.html`, opened at `/shared/plan/?id=<plan id>`)
- [x] Tests: `test/plan-render.test.js`

## Acceptance Criteria

- [x] ac-1: a fixture for `beginner-unit-1` renders Today, the unit overview, an introduction, recall, and the checkpoint, and the checkpoint lists each recycled word with `fromLessonId`
- [x] ac-2: each of the six block types has a renderer test; `graded-story`, `listening`, `short-dialogue`, `fluency-pass`, and the checkpoint emit no `img`
- [x] ac-3: a `<script>` plan string and an empty alt produce no extra script, no event-handler attribute, and no `img`; rendered text is escaped
- [x] ac-4: the new screen rules use the token variables; text pairs meet 4.5:1 in light and dark; primary controls are at least 44px; other pointer targets are at least 24px
- [x] ac-5: the sign-in page and the account menu still contain the admin-visibility sentence

## AC Verification (independent)

Verified by a fresh sub-agent (model `composer-2.5-fast`, agent `e136026e-a24b-42ef-9398-218510058ec9`) that did **not** author the code — not self-reported.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-014-eval-report.md`](SPEC-014-eval-report.md) (per-AC evidence + remediation log).

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` `pass` · security_review=`standard` `FAIL` · domain=`frontend` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

The dispatch prompt did not name a light-tier pre-flight, so the tier is `full`. Cursor Task has no native `readonly` flag; the prompt forbade edits.

| AC | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS | `renders Today, the overview, an introduction, recall, and the checkpoint`. Checkpoint lists 13 recycles with `fromLessonId`. `node --test test/plan-render.test.js` → 14 pass, 0 fail, exit 0. |
| ac-2 | PASS | `renders each type, and only an introduction with alt may emit an img`. Six `data-block-type` values. Non-introduction blocks emit no `<img>`. |
| ac-3 | PASS | `escapes a script payload, refuses an empty alt, and keeps one script src`. One `<script src="/shared/ui/plan-render.js">`. Empty alt and a URL media id produce no `<img>`. |
| ac-4 | PASS | `uses token variables, meets 4.5:1, and sizes the controls`. SPEC-014 block uses `var(--color-*)`, `--size-target` (24px), `--size-target-primary` (44px). No CSS `direction`. |
| ac-5 | PASS | `leaves the sign-in page and the account menu sentence in place`. Sentence still in `shared/app-shell.js`, `shared/login.js`, and `login/index.html`. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-014`
- **Title:** Render Today, the plan overview, and the six block types from a published snapshot
- **Files changed:** 5 · new renderer, entry page, and tests; plan-screen rules appended to the stylesheet; token snapshot grew by two styling class names
- **Change status:** proposed
- **Reversibility:** branch

---

## Decomposition Telemetry (`frontend` domain only — `INIT-064/SPEC-006`)

- **`axis_chosen`:** not captured from `design-corpus/frontend-design.md` (that file was not opened this run). The spec's own aud-1 already fixed the page as one column of escaped HTML strings. Execution followed that: Today, the overview, and the lesson step call six block renderers.
- **`predicted_vs_actual_sizing`:** screens: 3→3 named screens (Today, overview, lesson step), with recall as a state of the lesson step and the other four block types as further sections in the same document · components: 6→6 block renderers · states: meet, recall, empty attempt, miss, and hit, as the spec named them. The spec recorded no frontend sizing band; this reconstruction is from its Scope text.
- **`integration_gate_result`:** folded — ac-1 through ac-5 are this spec's integration and accessibility checks. 0 — all-PASS on first pass. Remediation loops: 0.

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| `test/plan-render.test.js` | 14 | 14 |

**All tests passing:** Yes · `node --test test/plan-render.test.js` 14/14 exit 0 · `npm test` 488 pass, 0 fail, 2 todo, exit 0 · **Coverage (new code):** no coverage tool is configured; the bar is this file plus `npm test`

---

## Gates & Controls

None — branch/reversible only. `gate_actions: []`. Autonomy `auto_tests`. No one-way action taken. No commit, push, or pull request (dispatch standing constraint).

---

## Security Review

- **Sensitivity / depth:** standard · standard · **method:** independent `security-review` Task (`composer-2.5-fast`, agent `84b2397a-50ea-49c9-ac30-91b409aae9de`), scoped to the SPEC-014 files. Cursor Task has no native `readonly` flag. Not self-reviewed.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | — |

- **Positive observations:** Plan strings go through `HebrewUtils.escapeHtml` before the one `innerHTML` assignment. Picture `src` is only `/api/media/` plus a slug; an empty alt or a non-id emits no `<img>`. The learner fetch is `GET /api/plans/:id/published` with `credentials: 'same-origin'`, and a non-slug id is not requested.

---

## Files Changed

Provenance is `INIT-004/SPEC-014` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `shared/ui/plan-render.js` | Created — six block renderers, Today, overview, lesson step, recall, published fetch | `INIT-004/SPEC-014` |
| `shared/plan/index.html` | Created — signed-in entry, one script, `/shared/plan/?id=` | `INIT-004/SPEC-014` |
| `shared/worksheet.css` | Appended the plan-screen block after the SPEC-013 token marker. Uses token variables. Scoped to `.plan-page` | `INIT-004/SPEC-014` |
| `test/plan-render.test.js` | Created — fixture, six types, hostile text, audio, recall, fetch, tokens, admin sentence | `INIT-004/SPEC-014` |
| `test/design-tokens.test.js` | Added `audio` and `word` to the queried-class snapshot. Those names are styling classes; the chrome smokes already mention `.audio` and `.word` inside script strings. No hook was renamed | `INIT-004/SPEC-014` |

---

## Harness Adaptation

Cursor Task dispatch (T2) for the AC verifier and the security review. No native `readonly` flag; both prompts forbade edits. Not a degraded tier.

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | `trace_open`, `task_step`, `spec_lifecycle`, `dispatch`, `trace_complete` | Dispatched sub-agent. Timer and trace sentinels are owned by the orchestrator (`execute-spec` Phase 0 step 2). This agent did not call `open_trace` / `emit` / `close_trace`. | This process wrote no `.sdd-trace/SPEC-014.jsonl` events. | The orchestrator must emit on return. A Tier-5 check against a missing trace is `FAIL`/`INCONCLUSIVE`, not a pass. |

---

## Deviations from the Spec

- **Persona.** `.claude/agents/` is absent in this repo (`agents_dir_absent`). `principal-frontend-developer` was not loaded and no `AGENT.md` was invented. The screens follow ADR-0006, ADR-0005's `(props) => string` contract, and `HebrewUtils.escapeHtml`.
- **Commit / push / PR.** Skipped. The dispatch said not to commit, push, or open a PR.
- **Audio with an `audioPath`.** The control is marked `data-plan-audio="ready"` and the path is not copied into a URL. This spec's learner load is only the published snapshot, and those paths are worksheet-relative. A missing path stays `data-plan-audio="none"` and does not request a clip.
- **Chrome click.** Not run. The spec allows that when a browser driver is not used. Recall is covered by `decideRecall` and by the hidden correction markup.
- **`/api/media/:id`.** The renderer points pictures at that URL. This spec does not add the handler. A same-origin image request sends the session cookie when the route exists.
- **Live Today link.** Unchanged. Open a plan at `/shared/plan/?id=<plan id>`. Cutover stays with SPEC-020.

---

## Effort Tracking

**Agent session time:** not recorded here — the orchestrator owns the timer
**Spec effort estimate:** 6 hours
