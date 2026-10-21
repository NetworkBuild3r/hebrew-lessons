# SPEC-007 · Testing: Player visual regression, a11y, touch-target, and Chrome smoke verification

## Metadata

```yaml
spec_id: SPEC-007
initiative_id: INIT-007
title: "Player visual regression, a11y, touch-target, and Chrome smoke verification"
domain: testing
status: complete
primary_prompt: .claude/agents/test-automator/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 4 hours
blocked_by: [SPEC-004, SPEC-005, SPEC-006]
blocks: []
```

---

## Description

**Context:** Overhauling the player, drills, audio indicators, and review experience changes CSS and HTML generation across multiple files. To ensure no regressions occur, this spec verifies all accessibility baselines, pointer target minimums, contrast floors, reduced motion compliance, and end-to-end interactive flows across mobile and desktop.

This spec executes comprehensive mechanized checks:
1. **Touch Target Dimensions:**
   - Primary interactive controls (Play, Next, Check, Knew, Missed, Continue, Back) must measure ≥44x44 CSS pixels (WCAG 2.5.5 AAA product choice).
   - Secondary / quiet controls (Pills, Vowel toggle, filters) must measure ≥24x24 CSS pixels (WCAG 2.5.8 AA).
2. **Contrast Baseline:**
   - Pointed text and vowel marks (nikud) must clear ≥4.5:1 against card and page backgrounds across both light and dark themes.
   - UI controls, focus rings, and border lines must clear ≥3:1 against adjacent colors.
3. **Reduced Motion Audit:**
   - Under `prefers-reduced-motion: reduce`, all step animations, pulse indicators, and transitions must evaluate to `none` or 0s.
4. **End-to-End Chrome Smokes:**
   - Run complete lesson walkthrough in Chrome (`my-morning`), verifying step advance, audio play trigger, match pairing, story read-aloud highlighting, and round completion ceremony.
   - Verify `/review/` active recall, reveal answer, pass/miss recording, and completion state.
5. **Hook Contract Audit:**
   - Mechanized verification that all 121 `data-*` and class name hooks required by `ADR-0006` remain intact and unrenamed.

**Scope (in):**
- New automated test script `test/player-ui-a11y.test.js`.
- Updates to existing smoke tests (`test/chrome-navigation-smoke.js`, `test/chrome-review-smoke.js`, `test/chrome-learner-support-smoke.js`).
- Automated contrast and target size verification.

**Scope (out):**
- Production code changes (any failures discovered are remediated in the originating spec or via targeted fixes).

---

## Understood

- [x] und-1: Zero regressions allowed across existing 318 unit and integration tests.
- [x] und-2: All primary interactive targets must measure ≥44x44px.
- [x] und-3: Contrast ratios must be verified mathematically for both light and dark modes.

---

## Acceptance Criteria

- [x] ac-1: Automated test `test/player-ui-a11y.test.js` verifies all primary buttons and interactive cards have computed dimensions of at least 44x44px.
- [x] ac-2: Pointed Hebrew text across all player components has measured contrast ≥4.5:1 against its background in light and dark schemes.
- [x] ac-3: CSS audit confirms all animations and transitions are disabled under `@media (prefers-reduced-motion: reduce)`.
- [x] ac-4: End-to-end Chrome smoke suite runs without error against a live server instance, completing a full lesson and a review session.
- [x] ac-5: Hook contract test confirms all 70 `data-*` attributes and 50 required class names remain present in generated player markup.

---

## Assumptions Ledger

| id | item | interpretation | tone | resolved |
|---|---|---|---|---|
| aud-1 | Test harness | Uses Node's built-in test runner `node --test` with Chrome DevTools Protocol or Puppeteer | resolved | yes |
| aud-2 | Contrast script | Reuses the relative luminance formula from `spikes/look/check.js` | resolved | yes |

> **Readiness** = 100.

---

## Deliverables

- `test/player-ui-a11y.test.js`
- Updates to `test/chrome-navigation-smoke.js` and `test/chrome-review-smoke.js`
- Test report summary at `sdd/initiatives/INIT-007-core-learning-experience/specs/completion-summaries/SPEC-007-completion-summary.md`

---

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Blocking checks:** Full test suite passes: `npm test`.
- **Rollback plan:** Revert newly added test files.

---

## Verification Strategy

- **Claim:** All player screens meet touch target ≥44px, contrast ≥4.5:1, reduced-motion compliance, zero hook regressions, and passing Chrome smokes.
- **Check + executor:** `mechanized` — `node --test test/player-ui-a11y.test.js` and full `npm test`.
- **Pass condition:** ac-1 through ac-5 verified.
- **Provenance tag:** `INIT-007/SPEC-007`
