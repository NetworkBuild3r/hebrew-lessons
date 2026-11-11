# SPEC-013 · Frontend: Accessibility, screenshot and hook-contract tests

## Metadata

```yaml
spec_id: SPEC-013
initiative_id: INIT-005
title: "Accessibility, screenshot and hook-contract tests"
domain: frontend
status: complete
primary_prompt: .claude/agents/principal-frontend-developer/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-011, SPEC-012]
blocks: []
```

---

## Description

**Context:** Close INIT-005 with durable checks: contrast and targets from ADR-0008, reduced motion per route, screenshots at 360 and 1280, and the class/`data-*` hook contract.

**Scope (in):** automated contrast/target checks; reduced-motion tests; screenshot capture for shell routes; hook-contract test that fails if a queried selector was renamed without updating the test; update Chrome smokes only if a hook intentionally changed.

**Scope (out):** new features; further look changes.

---

## Understood

- [x] und-1: Screenshot paths live under a checked-in directory or a test artefact path consistent with existing smoke scripts. _(Closure: PNGs go to `spikes/restyle-shots/`, the untracked, never-served folder the earlier restyle screenshots used; `test/chrome-restyle-screenshots-smoke.js` follows the `chrome-*-smoke.js` pattern. They are for human review, not golden images.)_
- [x] und-2: This is the last INIT-005 build spec. Completion report waits for `complete-initiative`. _(Closure: acknowledged; the completion summary is written, the initiative completion is not.)_

## Acceptance Criteria

- [x] ac-1 (REQ-017, REQ-019): contrast and target-size checks pass for ADR-0008 tokens. _(Closure: ADR-0010 is the adopted look, so the checks run on its colours. `test/restyle-gate.test.js` checks 19 token pairs in light and dark; `test/chrome-restyle-a11y-smoke.js` measures computed text contrast, 24 px targets (with the WCAG spacing exception) and 44 px primary controls on every route and login at 360 and 1280 px. Four real defects found and fixed, see the summary.)_
- [x] ac-2 (REQ-005, REQ-019): reduced-motion path is asserted per route transition and feedback kit. _(Closure: the smoke emulates `prefers-reduced-motion: reduce` over CDP, asserts no route transition and no transition or animation on any element of all seven routes and login, and asserts the feedback kit's loading, playing, reveal, correct and miss states are still, after first proving they animate under normal motion.)_
- [x] ac-3 (REQ-019): screenshots at 360 and 1280 exist for Today, Lessons, Review, Progress, Admin, and login. _(Closure: `test/chrome-restyle-screenshots-smoke.js` writes 18 PNGs (also Guides, Words and the player Match step) and asserts size and non-blank. They are untracked artefacts; run the script to regenerate.)_
- [x] ac-4: hook-contract test covers the selectors the unit tests and Chrome smokes query. _(Closure: `test/restyle-gate.test.js` re-checks the 70 + 50 hooks against production code only, checks the SPA shell files, and fails if a data-* attribute or id a smoke or shell test queries is gone. Three ADR hooks are no longer emitted: `data-key`, `data-slug`, `lessons-grid`; listed in the summary.)_

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
| -- | ---- | -------------- | ---- | -------- |
| aud-1 | Screenshot tool | Reuse the approach already used in `spikes/look` / Chrome smokes | resolved | yes — existing pattern |

> **Readiness** = 100.

---

## Deliverables

- [x] Test files and screenshots _(Closure: `test/restyle-gate.test.js`, `test/chrome-restyle-a11y-smoke.js`, `test/chrome-restyle-screenshots-smoke.js`, `test/helpers/chrome-cdp.js`.)_
- [x] Completion summary _(Closure: `completion-summaries/SPEC-013-completion-summary.md`.)_
- [ ] Initiative status ready for `complete-initiative` (mode e) once this passes

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** `npm test`; screenshot artefacts present.
- **Rollback plan:** revert the branch.

---

## Security

- **Sensitivity:** light.
- **Review depth:** light.

---

## Verification Strategy

- **Claim:** the restyle is locked by tests and screenshots.
- **Check + executor:** `mechanized` — `npm test`; `manual` — spot-check screenshots.
- **Pass condition:** exit 0; ac-1 through ac-4.
- **Provenance tag:** `INIT-005/SPEC-013`
