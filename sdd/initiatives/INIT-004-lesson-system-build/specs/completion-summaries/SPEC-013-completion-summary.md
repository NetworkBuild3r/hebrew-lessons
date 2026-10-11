# SPEC-013 Completion Summary — ADR-0006 tokens in the shared stylesheet

**Traceability:** INIT-004/SPEC-013
**Status:** Complete
**Domain:** frontend
**Completed Date:** 2026-10-09

---

## Deliverables Checklist

- [x] Token block in `shared/worksheet.css`
- [x] Tests: `test/design-tokens.test.js`

## Acceptance Criteria

- [x] ac-1: every color token has light and dark custom properties, and contrast stays at 4.5:1 (text) and 3:1 (UI) at the ADR-0006 ratios
- [x] ac-2: `--size-target` is 24px, `--size-target-primary` is 44px, `--radius` is 2px, and the new rules add no box-shadow
- [x] ac-3: class selectors queried by `test/app-shell.test.js` and the chrome smokes are not renamed

## AC Verification (independent)

Verified by a fresh readonly sub-agent (model `composer-2.5-fast`, agent `64e9bbac-8c71-4ab0-ae2a-b5544acbc175`) that did **not** author the code — not self-reported.
Overall: **PASS** (all-PASS required to complete) · remediation loops: 0.
**Full eval report:** [`SPEC-013-eval-report.md`](SPEC-013-eval-report.md) (per-AC evidence + remediation log).

**Verification tier:** `full` · **pre-flight:** gate_actions=`[]` `pass` · security_review=`light` `PASS` · domain=`frontend` `FAIL` · autonomy_level=`auto_tests` `FAIL` ⇒ light_tier_eligible=`false` · **checks skipped:** `None`

| AC   | Verdict | Evidence (file:line · test · command output) |
| ---- | ------- | -------------------------------------------- |
| ac-1 | PASS    | `node --test test/design-tokens.test.js` 9/9 exit 0. Light/dark/data-theme color variables in `shared/worksheet.css` 1679–1774. Contrast test matches all 13 ADR pairs. |
| ac-2 | PASS    | `--size-target: 24px`, `--size-target-primary: 44px`, `--radius: 2px` at `shared/worksheet.css` 1713–1715. No `box-shadow` declaration in the token block. |
| ac-3 | PASS    | Snapshot test of 38 queried class selectors passed. Diff removes no `.class` selector. |

---

## Change Produced

- **Authored by:** `INIT-004/SPEC-013`
- **Title:** Add the ADR-0006 token block to the shared stylesheet without restyling legacy corners
- **Files changed:** 2 · additions +349 in the stylesheet, plus a new test file · deletions −18 in the stylesheet
- **Change status:** proposed
- **Reversibility:** branch

---

## Decomposition Telemetry (`frontend` domain only — `INIT-064/SPEC-006`)

- **`axis_chosen`:** Rule 1 — shared/design-system element (one token block on the shared stylesheet, ahead of the screens that will consume it) → component-axis spec. The ordered table stops there. ADR-0006 is already the design record, so this spec is the build of that handoff.
- **`predicted_vs_actual_sizing`:** screens: 0→0 · components: 0→0 (tokens, not a component) · states: light, dark, data-theme light, data-theme dark, and reduced motion were named in the spec and shipped. The spec did not record a frontend sizing band; this reconstruction is from its Scope text.
- **`integration_gate_result`:** folded — this spec's own ac-1 is the component-level contrast check. Flow-level accessibility stays with later specs (this spec `blocks` SPEC-014). 0 — all-PASS on first pass. Remediation loops: 0.

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| `test/design-tokens.test.js` | 9 | 9 |

**All tests passing:** Yes · `node --test test/design-tokens.test.js` 9/9 exit 0 · `npm test` 336/336 exit 0 · **Coverage (new code):** no coverage tool is configured; the bar is this file plus `npm test`

---

## Gates & Controls

None — branch/reversible only. `gate_actions: []`. Autonomy `auto_tests`. No one-way action taken. No commit, push, or pull request (dispatch standing constraint).

---

## Security Review

- **Sensitivity / depth:** light · quick · **method:** independent readonly generalPurpose Task (`composer-2.5-fast`, agent `a8b1a7eb-1aa3-4ba5-817f-c0a5fae877ff`), prompt-enforced read-only. Cursor Task has no native `readonly` flag. Not self-reviewed.
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| — | — | — | — | — |

- **Positive observations:** The token block is custom properties and media queries only. It has no `url()`, `@import`, or remote host. The test rejects `url(http`, protocol-relative URLs, and `@import`. Rubik stays the existing self-hosted `shared/fonts/` files.

---

## Files Changed

Provenance is `INIT-004/SPEC-013` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `shared/worksheet.css` | Modified — ADR-0006 token block appended; legacy `--radius: 16px` kept as `--legacy-radius` | `INIT-004/SPEC-013` |
| `test/design-tokens.test.js` | Created — token, contrast, and class-hook checks | `INIT-004/SPEC-013` |
| `sdd/initiatives/INIT-004-lesson-system-build/specs/SPEC-013-frontend-design-tokens.md` | Status set to `complete`; acceptance criteria checked | `INIT-004/SPEC-013` |

---

## Trace Emission Degradation

Timer and trace were absent on this dispatch (orchestrator-owned; this worker was told not to emit). No `open_trace` / `emit` / `close_trace` call was made.

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 | `trace_open` | Dispatch said timer/trace absent; orchestrator owns the sentinel | Opening sentinel for SPEC-013 | A Tier-5 reader of this worker's session sees no trace file from this agent |
| Phase 1 | `task_step` | Same | Step transitions | Granular step history was not written |
| Phase 4 / 4.5 | `harness_fallback` | Same | Cursor T2 disclosure event | Disclosed in this summary instead of the trace |
| Phase 5 | `dispatch`, `spec_lifecycle`, `trace_complete` | Same | Closing sentinel and model-dispatch evidence | Tier-5 and model-dispatch checks were not run here; they belong to the orchestrator if it owns the trace |

---

## Deviations from the Spec

- **Persona.** `.claude/agents/` is absent (`agents_dir_absent`). `scripts/dev/expertise_state.py` is not in this repo, so `dispatch-check` could not run. The whole persona layer is unprovisioned. Work followed the spec, ADR-0006, `tokens.json`, and the existing stylesheet. No `AGENT.md` was written.
- **`--legacy-radius`.** `tokens.json` names `radius.default` as `--radius: 2px`. The stylesheet already used `--radius: 16px` in legacy rules, including `calc(var(--radius) - 6px)`. Setting `--radius: 2px` on `:root` would restyle those rules and can make the calc negative. Existing rules now read `--legacy-radius: 16px`. The token block sets `--radius: 2px` for later specs. Class names and `data-*` hooks were not renamed. Login and the look spikes keep their own `--radius`.
- **`color-scheme`.** `:root` in the token block sets `color-scheme: light dark`, the dark media query sets `color-scheme: dark`, and `data-theme` sets a single scheme. Legacy hex colors are unchanged. Native form controls can follow the system scheme.
- **No lint or typecheck command** is configured in `package.json`. Phase 4 ran `node --test test/design-tokens.test.js` and `npm test` only. No `# type: ignore` or `# noqa` was added. No ratchet baseline was moved.
- **Git.** No commit, push, or pull request, per the dispatch. The git identity guard is inert (`guard_identities` empty). `scripts/dev/repo_context.py` is not in this repo; the directory is a git checkout (`git status` at session start).
- **Discipline.** Initiative field `Discipline Profile: software`. Software verification mechanics applied (scoped `node --test`, plus the spec's own `npm test` bar). Coverage percentage is not measurable here.
- **Research.** Token values are taken from ADR-0006 / `tokens.json`, which cite `RSCH-006`. They were not re-derived from the research report.

---

## Harness Adaptation

| Site | Capability | Tier | What was done |
| ---- | ---------- | ---- | ------------- |
| Phase 4 security review | `subagent_dispatch` | T2 (Cursor) | Fresh `composer-2.5-fast` Task. Prompt forbade edits. Agent `a8b1a7eb-1aa3-4ba5-817f-c0a5fae877ff`. |
| Phase 4.5 AC verifier | `subagent_dispatch` | T2 (Cursor) | Fresh `composer-2.5-fast` Task. Prompt forbade edits. Agent `64e9bbac-8c71-4ab0-ae2a-b5544acbc175`. All-PASS, 0 remediation loops. |

---

## Effort Tracking

**Agent session time:** not recorded — timer absent on this dispatch (orchestrator-owned)
**Spec effort estimate:** 3 hours
