# SPEC-006 Completion Summary — Three look mockups

**Traceability:** INIT-003/SPEC-006
**Status:** Complete (three mockups, comparison, check script, not-served test, and spec `status: complete` written in this session)
**Domain:** frontend
**Completed Date:** 2026-10-08

---

## Deliverables Checklist

- [x] `spikes/look/a-calm-type/`, `spikes/look/b-warm-illustrated/`, `spikes/look/c-dense-tool/` — each with `today.html`, `lesson-step.html`, `look.css`, and `README.md`
- [x] `spikes/look/COMPARE.md` and `spikes/look/shots/` (12 light-scheme screenshots: both screens, 360 and 1280)
- [x] `spikes/look/check.js`
- [x] `.dockerignore` includes `spikes/`
- [x] `test/spikes-not-served.test.js`
- [x] SPEC-006 yaml `status: complete`

## Acceptance Criteria

- [x] ac-1: Three folders under `spikes/look/`. Each page is plain HTML and CSS and opens from the file system. Rubik loads from `../../../shared/fonts/` (confirmed in headless Chrome: `document.fonts.check` for Rubik was true without a special file-access flag).
- [x] ac-2: Phone query `max-width: 599px` and desktop query `min-width: 1280px`. `dir="rtl"` and `lang="he"` on `<html>`. English glosses are inline `lang="en" dir="ltr"`. Light and dark come from `color-scheme: light dark` plus `light-dark()`, and from the scheme radios.
- [x] ac-3: Each README has rows 1–18. Together they cover the RSCH-006 table. Each names at least one visible trade-off.
- [x] ac-4: `node spikes/look/check.js` exited 0 (contrast, target size, reduced motion).
- [x] ac-5: `spikes/look/COMPARE.md` embeds the twelve screenshots. No learner names, transcripts, or secrets.
- [x] ac-6: `test/spikes-not-served.test.js` passed. `publicFile` / `resolveFile` return null for `/spikes/`. Served roots do not contain `spikes/`. `.dockerignore` lists `spikes/`.

## AC Verification (independent)

Mechanized checks ran in this session: `node spikes/look/check.js` and `npm test` (294 passed, 0 failed), including the new not-served test and the self-hosted asset test (the new pages add no remote styles or fonts).

The operator has not yet named a pick in writing. That pick is SPEC-008's input. This summary recommends `a-calm-type`.

Overall: **mechanized checks passed** · operator pick not yet written · remediation loops: 1 (English glosses were full-width `dir="ltr"` paragraphs and sat on the left; they are now inline spans).

**Verification tier:** `partial` (light security; mechanized checks run; operator view not done) · **pre-flight:** gate_actions `[]` · security_review=`light` · domain=`frontend` · autonomy_level=`auto_branch`

| AC   | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS | Three folders, two HTML pages each, own CSS. Chrome loaded Rubik from the relative font URL. |
| ac-2 | PASS | Screenshots at 360 and 1280. Scheme radios and `light-dark()` cover both schemes. Gloss spans are `dir="ltr"`. |
| ac-3 | PASS | `check.js` requires README rows 1–18 plus "Follows" and "Trades off". |
| ac-4 | PASS | `node spikes/look/check.js` exit 0. |
| ac-5 | PASS | Twelve PNGs linked from `COMPARE.md`. |
| ac-6 | PASS | `npm test` — "spikes are not served" 3/3. |

---

## Change Produced

- **Authored by:** `INIT-003/SPEC-006`
- **Title:** Three static looks for Today and an introduction step, for the operator to choose among
- **Change status:** written on the shared checkout. No commit.
- **Reversibility:** revertible (delete `spikes/look/`, the `.dockerignore` line, the test, and this summary; set the spec status back to `draft`)

---

## Tests

| Test file | New tests | Total in that file |
| --------- | --------- | ------------------ |
| `test/spikes-not-served.test.js` | 3 | 3 |
| `spikes/look/check.js` (blocking check, not under `npm test`) | contrast, targets, reduced motion | exit 0 |

**All tests passing:** yes (`npm test`, 294 passed, 0 failed) · **Coverage (new code):** the not-served test and `check.js`. The pages are static HTML.

---

## Gates & Controls

`gate_actions: []`. aud-1, aud-2, and aud-3 were already resolved. No commit, push, or pull request.

---

## Security Review

- **Sensitivity / depth:** light · quick · **method:** inline review (`self_reviewed: true`)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

The pages have no scripts, no inline event handlers, and no remote URLs. They are not on a served root and are excluded from the Docker build context. Sample words only: שָׁלוֹם, קָפֶה, בֹּקֶר. The audio control does not point at a clip.

- **Positive observations:** Placeholders are labeled מציין מקום. No learner images.

---

## Files Changed

Provenance is `INIT-003/SPEC-006` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `spikes/look/a-calm-type/` | Created — calm type mockup | `INIT-003/SPEC-006` |
| `spikes/look/b-warm-illustrated/` | Created — warm illustrated mockup | `INIT-003/SPEC-006` |
| `spikes/look/c-dense-tool/` | Created — dense tool mockup | `INIT-003/SPEC-006` |
| `spikes/look/COMPARE.md` | Created — comparison and recommendation | `INIT-003/SPEC-006` |
| `spikes/look/check.js` | Created — contrast, target, reduced-motion check | `INIT-003/SPEC-006` |
| `spikes/look/shots/*.png` | Created — 12 screenshots | `INIT-003/SPEC-006` |
| `.dockerignore` | Modified — `spikes/` | `INIT-003/SPEC-006` |
| `test/spikes-not-served.test.js` | Created | `INIT-003/SPEC-006` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/SPEC-006-frontend-three-look-mockups.md` | `status: complete`; checkboxes marked | `INIT-003/SPEC-006` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/completion-summaries/SPEC-006-completion-summary.md` | This file | `INIT-003/SPEC-006` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | trace and timer scripts | This session was an orchestrator-dispatched worker; timer and `open_trace` stay with the orchestrator | Run-trace and timed effort | Effort is an estimate only |

---

## Harness Adaptation

- **Persona:** `.claude/agents/principal-frontend-developer/AGENT.md` is not in this repo. Disclosed in the spec. The pages follow the spec, RSCH-006, and ADR-0003's `introduction` block.
- **Harness:** `cursor`.
- **Worktree / branch:** shared checkout. No commit.
- **AC verifier:** mechanized checks in this session. Not a second person. The operator has not picked yet.

---

## Deviations from the Spec

- Colour schemes use `color-scheme` and `light-dark()`, with radios that force light or dark, rather than a `data-theme` attribute. `light-dark()` follows `prefers-color-scheme` when the scheme is `light dark`.
- Screenshots are the light scheme only. Both schemes are in the CSS and are measured by `check.js`. The acceptance text asks for both screens at both widths, which the twelve shots cover.
- The lesson file stacks `#meet` and `#miss` so both states can be compared without a server. Each state is still one screen. a and b keep one word on the meet screen.

---

## Recommendation for the operator

Pick **a-calm-type** unless a warmer picture or a denser workbench is wanted on purpose. It is the only look that keeps one next action, one word, and a phone action in the lower reachable area, and it adds no banner or index.
