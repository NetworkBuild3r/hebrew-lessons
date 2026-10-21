# SPEC-006 · testing: Responsive viewport matrix (`360px`, `390px`, `768px`, `1280px`, `1440px`), Light/Dark contrast, and Chrome smoke suite

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-013
title: "Responsive viewport matrix, Light/Dark WCAG 2.2 contrast, multi-tense verb workshop, and Chrome smoke verification"
domain: testing
status: complete
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-004, SPEC-005, SPEC-007]
blocks: []
```

## Description

**Context:** `SPEC-002` through `SPEC-005` overhaul the token system, application shell, Today/Catalog Bento workspaces, and Lesson Player / Multi-Tense Verb Drill / SRS Review surfaces for both phones and desktops, while `SPEC-007` adds the `schemaVersion: "1.2.0"` multi-binyan, multi-tense (`present`, `past`, `future`) verb workshop and 88-form lexicon seed. Before `INIT-013` can close, automated tests and real headless Chrome smoke checks must prove that the responsive geometry actually switches at the breakpoints (rather than remaining a single column or wrapping top nav pills), that the multi-tense verb workshop and interleaved hidden-stem drill operate cleanly, and that every route passes WCAG 2.2 AA contrast, target size (`>= 44px` primary, `>= 24px` minimum), focus visibility, reduced-motion, and zero-overflow checks in both Light and Dark themes.

**Scope (in):**

1. **Responsive Layout & Multi-Tense Verb Drill Matrix Test (`test/responsive-layout.test.js`):**
   - Static and DOM-computed assertions verifying that `shared/worksheet.css` and `shared/ui/blocks/conjugation-table.js` define:
     - Desktop (`@media (min-width: 1024px)`) persistent sidebar rail rules for `.hebrew-shell` (`260px` column) and 2-column Bento grid rules for `.today-list[data-today-state="ready"]`.
     - Mobile (`@media (max-width: 767px)`) fixed bottom dock rules (`position: fixed; bottom: 0`) for `.hebrew-shell-nav` and bottom clearance padding on `#app-main.wrap`.
     - Desktop 2-column choice grid rules for `.exercise-choices`, conjugation table (`present`, `past`, `future`) & 22-letter pad (`[data-letter-pad]`) responsiveness, and mobile thumb-tray rules.
     - Active `data-drill-lemma`, `data-drill-tense`, and `data-drill-cell` cues on `[data-drill-prompt]` during interleaved multi-verb/multi-tense drills (`REQ-009`).
     - Complete Light (`Jerusalem Stone`) and Dark (`Judean Night`) token definitions and theme toggle persistence in `shared/app-shell.js`.
2. **Real Chrome Responsive, Verb Workshop & Accessibility Smokes:**
   - Run `node test/chrome-restyle-a11y-smoke.js`, `node test/chrome-restyle-screenshots-smoke.js`, `node test/chrome-exercise-smoke.js`, `node test/chrome-verb-workshop-smoke.js`, and `node test/chrome-lesson-types-smoke.js`.
   - Verify bounding-box geometry in headless Chrome at Mobile (`360×800` and `390×844`) vs. Desktop (`1280×900` and `1440×960`):
     - On Desktop (`1280px` / `1440px`): `.hebrew-shell` sits beside `#app-main` (`shellRect.width <= 280` and `mainRect.left >= shellRect.right - 2` in LTR or `mainRect.right <= shellRect.left + 2` in RTL), and on `Today` when both `#today-continue-card` (or `#today-next-card`) and `#today-review-card` are present, they sit in two side-by-side columns rather than a single narrow column.
     - On Mobile (`360px` / `390px`): `.hebrew-shell-nav` sits docked at the bottom of the viewport (`Math.abs(navRect.bottom - window.innerHeight) <= 2`), all nav links are `>= 44px` tall, and `document.documentElement.scrollWidth <= window.innerWidth` across `Today`, `Lessons`, `Review`, and the `verb-workshop` player.

**Scope (out):** New product features outside `SPEC-002`–`SPEC-005` and `SPEC-007`.

## Understood

- [x] und-1: `test/restyle-gate.test.js` line 452 requires that screenshot scripts write only under `spikes/restyle-shots/`, which the server never serves.
- [x] und-2: All existing unit tests (`npm test` / `node --test`) and Chrome smokes must pass with zero regressions.

## Acceptance Criteria

- [x] ac-1: `node --test test/responsive-layout.test.js test/verb-tense-workshop.test.js test/render-conjugation.test.js test/restyle-gate.test.js test/design-tokens.test.js test/player-ui-a11y.test.js test/app-shell.test.js` passes 100% (`REQ-001`–`REQ-011`).
- [x] ac-2: `node test/chrome-restyle-a11y-smoke.js` exits `0` (`PASS`), confirming zero contrast failures across all routes at `360px` and `1280px` in both Light and Dark themes, zero horizontal overflow, `>= 44px` primary targets, visible keyboard focus across Tab stops, and working `prefers-reduced-motion: reduce` (`REQ-005`, `REQ-007`).
- [x] ac-3: Headless Chrome bounding-box checks confirm that at `1440×960` the navigation rail and main stage sit side-by-side and `.today-list` renders a 2-column Bento layout, while at `390×844` `.hebrew-shell-nav` is docked at `bottom: 0` (`REQ-001`, `REQ-002`, `REQ-003`).
- [x] ac-4: `node test/chrome-exercise-smoke.js`, `node test/chrome-verb-workshop-smoke.js`, and `node test/chrome-lesson-types-smoke.js` exit `0` (`PASS`) (`REQ-006`, `REQ-007`, `REQ-009`, `REQ-011`).

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | Screenshot output path | Any captured verification screenshots are written to `spikes/restyle-shots/` per `test/restyle-gate.test.js` line 452 | resolved | yes |

## Deliverables

- [x] `test/responsive-layout.test.js`
- [x] Updated screenshot captures in `spikes/restyle-shots/` via `node test/chrome-restyle-screenshots-smoke.js`
- [x] Completion summary in `sdd/initiatives/INIT-013-responsive-desktop-mobile-redesign/specs/completion-summaries/SPEC-006-completion-summary.md`

## Verification Strategy

- **Claim:** Every desktop and mobile layout requirement (`REQ-001` through `REQ-008`) is verified by both fast Node tests and real headless Chrome geometry/a11y checks.
- **Check + executor:** `mechanized` — `node --test test/responsive-layout.test.js test/restyle-gate.test.js test/design-tokens.test.js test/player-ui-a11y.test.js test/app-shell.test.js` and `node test/chrome-restyle-a11y-smoke.js && node test/chrome-exercise-smoke.js && node test/chrome-lesson-types-smoke.js`.
- **Pass condition:** All commands exit `0` with zero failures.

## Gates

No production code changes unless fixing a defect uncovered by the smoke suite.

## Integration Points

- Final verification gate for `INIT-013` before `complete-initiative` Phases 3.5 and 3.6.
- **Provenance tag:** `INIT-013/SPEC-006`.
