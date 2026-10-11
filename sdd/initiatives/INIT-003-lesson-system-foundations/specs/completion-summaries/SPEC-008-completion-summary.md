# SPEC-008 Completion Summary — ADR-0006 visual language and interface rules

**Traceability:** INIT-003/SPEC-008
**Status:** Complete (ADR written; spec `status: complete`)
**Domain:** docs
**Completed Date:** 2026-10-08

---

## Deliverables Checklist

- [x] `docs/decisions/ADR-0006-visual-language-and-interface-rules.md`
- [x] `docs/decisions/ADR-0006-assets/tokens.json`
- [x] SPEC-008 yaml `status: complete`
- [x] ADR-0005 status set to Accepted (operator confirmed Option A)

## Acceptance Criteria

- [x] ac-1: Decision §§1–8 cover the pick, tokens, components, rules, accessibility, right-to-left, exclusions, and the class/`data-*` contract.
- [x] ac-2: 48 tokens, each with a value and a purpose. 13 foreground/background pairs, each with a light and a dark ratio.
- [x] ac-3: All 18 `RSCH-006` rule rows match in text, grade, source, and label. The only application difference is rule 11's introduction example, explained as a product choice. Dark mode does not edit a row.
- [x] ac-4: The baseline gives ratios (4.5:1, 3:1), sizes (24px, 44px, 200%, 320px), spacing (1.5, 2, 0.12em, 0.16em), 5 seconds, and 20 dB.
- [x] ac-5: 70 `data-*` attributes and 50 classes, each `kept`, plus `data-action="set-dir"` as `retire`. Named unit tests, the ten Chrome smokes, and the three other `test/*.test.js` files that assert hooks are in the tables. `chrome-audio-smoke.js` asserts none.
- [x] ac-6: No secret values, learner names, or transcript text.

## AC Verification (independent)

The spec assigns rule tracing to a human who did not write the ADR (five sampled rules) and the ratios and hooks to a script. The script was run in this session: every `tokens.json` contrast ratio matches a fresh WCAG computation to one decimal place, and every `data-*` hook and styling class asserted in `test/*.test.js` and `test/chrome-*-smoke.js` appears in the ADR. The five-rule human sample was not run by a second person. The 18 rule rows were compared mechanically to `RSCH-006` and match.

Overall: **not independently verified** · remediation loops: 0.

**Verification tier:** `light` · **pre-flight:** gate_actions `[]` PASS · security_review=`light` PASS · domain=`docs` PASS · autonomy_level=`plan_only` PASS ⇒ light_tier_eligible=`true` · **checks skipped:** test-suite re-run — the spec names no test file.

| AC   | Verdict | Evidence |
| ---- | ------- | -------- |
| ac-1 | PASS (author) | ADR-0006 Decision §§1–8 |
| ac-2 | PASS (author) | Token tables and `tokens.json` `contrast[]`; recomputed ratios match to 1 decimal |
| ac-3 | PASS (author) | 18/18 rule rows identical to `RSCH-006`; rule 11 exception explained |
| ac-4 | PASS (author) | Decision 5 criterion table |
| ac-5 | PASS (author) | Decision 8; scan of `test/` found no omitted hook |
| ac-6 | PASS (author) | Author scan; no Hebrew excerpt, no credential, no learner name |

---

## Change Produced

- **Authored by:** `INIT-003/SPEC-008`
- **Title:** Record a-calm-type as the visual language, with dark mode and stable test hooks
- **Files changed:** 5 · ADR-0006, tokens.json, this summary, SPEC-008 status, ADR-0005 status
- **Change status:** written on branch `lesson-system-foundations`
- **Reversibility:** revertible (delete ADR-0006, its assets folder, and this summary; set spec status back to `ready`; set ADR-0005 status back to Proposed)

---

## Tests

| Test file | New tests | Total |
| --------- | --------- | ----- |
| —         | 0         | 0     |

**All tests passing:** N/A (docs) · **Coverage (new code):** N/A

---

## Gates & Controls

`gate_actions: []`. aud-1 was already the operator's pick, a-calm-type. aud-2 is resolved in this run: ship dark mode, yes, operator 2026-10-08. ADR-0005 was still Proposed; status is now Accepted, with one changelog line, because the operator had confirmed Option A. No commit, push, or pull request.

---

## Security Review

- **Sensitivity / depth:** light · quick · **method:** inline review of the ADR against ac-6 (`self_reviewed: true` — documentation only; no executable code)
- **Findings:** Critical: 0 · High: 0 · Medium: 0 · Low: 0
- **Unresolved Critical/High:** None

| Finding | Severity | OWASP/control | Location | Status |
| ------- | -------- | ------------- | -------- | ------ |
| —       | —        | —             | —        | no findings |

Admin hooks that hide the admin link stay `kept`. The token file has no key and no learner record.

---

## Files Changed

Provenance is `INIT-003/SPEC-008` for every row below.

| File | Change | Authored by |
| ---- | ------ | ----------- |
| `docs/decisions/ADR-0006-visual-language-and-interface-rules.md` | Created — ADR-0006 | `INIT-003/SPEC-008` |
| `docs/decisions/ADR-0006-assets/tokens.json` | Created — 48 tokens, 13 contrast pairs | `INIT-003/SPEC-008` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/SPEC-008-docs-adr-0006-visual-language.md` | Modified — status `complete`; aud-2 resolved; checkboxes marked | `INIT-003/SPEC-008` |
| `sdd/initiatives/INIT-003-lesson-system-foundations/specs/completion-summaries/SPEC-008-completion-summary.md` | Created — this summary | `INIT-003/SPEC-008` |
| `docs/decisions/ADR-0005-stack-and-architecture.md` | Modified — Status Accepted; one changelog line; summary tense updated | `INIT-003/SPEC-008` |

---

## Trace Emission Degradation

| Site | Event type | Reason | What was lost | Impact |
| ---- | ---------- | ------ | ------------- | ------ |
| Phase 0 / Phase 5 | trace and timer scripts | This session was an orchestrator-dispatched worker; timer and `open_trace` stay with the orchestrator | Run-trace and timed effort | Effort is an estimate only |

---

## Harness Adaptation

- **Persona:** `.claude/agents/implementation-planner/AGENT.md` is not in this repo. Disclosed in the spec. The ADR follows the section order of ADR-0001 and ADR-0002.
- **Harness:** `cursor`.
- **Worktree / branch:** shared checkout on `lesson-system-foundations`. No commit. Worktree engagement stays with the orchestrator.
- **AC verifier:** not a second person. Contrast recomputation, hook scan, and rule-row comparison only.

---

## Deviations from the Spec

- The architecture-decision-record template path named in the spec is not present in this repo. Section order follows ADR-0001 and ADR-0002.
- aud-2 was still `ambiguous` in the spec file at the start of the run. The operator's dispatch resolved it: ship dark mode, yes. The ledger is updated.
- Token values and contrast ratios are in both the ADR tables and `tokens.json`. ac-2 requires the table to list them, and the verification strategy compares the two. Prose outside Decision 2 cites token ids. A script checked that the ratios match.
- The hook scan includes `retry-before-reveal-smoke.test.js`, `production-recall.test.js`, and `story-gloss.test.js`, which the acceptance criterion does not name, because those files assert hooks a search of `test/` would otherwise call omitted.
- ADR-0005's summary sentence was updated from future to past tense along with the status field, so an Accepted record does not still say the operator has yet to confirm.

---

## Effort Tracking

**Agent session time:** not timed · one documentation session
**Spec effort estimate:** 3 hours

## Facts re-checked

Operator pick: a-calm-type. Dark mode: yes. Tokens: 48, from `spikes/look/a-calm-type/look.css`. Contrast: 13 pairs, recomputed with the `spikes/look/check.js` formula, all matching to one decimal place. Lowest text pair 9.5:1 (muted on background, light). Lowest UI pair 3.1:1 (line on surface, dark). Rules: 18/18 identical to `RSCH-006`. Hooks: 121 contract rows (70 `data-*` kept, 50 classes kept, 1 retired value). ADR-0005 status was Proposed and is now Accepted.
