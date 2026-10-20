# SPEC-003 · frontend: Adaptive application shell — Desktop persistent sidebar rail & Mobile thumb-zone bottom tab dock

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-013
title: "Adaptive application shell: Desktop persistent sidebar rail and Mobile thumb-zone bottom tab dock"
domain: frontend
status: complete
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 6 hours
blocked_by: [SPEC-002]
blocks: [SPEC-004, SPEC-005]
```

## Description

**Context:** Today, `.hebrew-shell` (`shared/app-shell.js` and `shared/worksheet.css` lines 4345–4517) renders a single horizontal top bar where `.hebrew-shell-nav` has `order: 3; flex: 1 1 100%; flex-wrap: wrap;`. On mobile (`<768px`), up to 8 navigation pills wrap into 2–3 rows at the top of the screen, wasting ~150px of vertical height and forcing thumb reach to the top edge. On desktop (`≥1024px`), navigation still wraps inside a top bar instead of providing a persistent application sidebar rail.

This spec implements Pillar 1 of the Figma redesign (`30:20`, `31:2`, `33:2` in `NJ8HfTPCQgrFty2VVQBMhY`): transforming `.hebrew-shell` into a **persistent vertical Navigation Rail (`260px`) on Desktop (`≥1024px`)** and a **Compact Single-Row Top Header + Fixed Thumb-Zone Bottom Tab Dock on Mobile (`<768px`)**.

**Scope (in):**

1. **Desktop Persistent Navigation Rail (`@media (min-width: 1024px)`):**
   - `body.has-hebrew-shell` becomes a CSS Grid (`grid-template-columns: 260px minmax(0, 1fr); min-height: 100dvh; align-items: start;`).
   - `#app-shell` / `.hebrew-shell` sits in the `260px` column (`position: sticky; top: 0; height: 100dvh; overflow-y: auto; border-inline-end: 1px solid var(--color-border-subtle); background: var(--color-surface);`).
   - Inside `.hebrew-shell-inner`, the brand lockup (`.hebrew-brand`) sits at the top, `.hebrew-shell-nav` stacks vertically (`flex-direction: column; align-items: stretch; gap: 0.35rem;`) with full-width pill links (`min-height: 44px`, icon + label aligned cleanly), and the utility/account/sync block (`.hebrew-shell-account`, `.hebrew-shell-sync`, language switcher, theme switcher) anchors at the bottom of the rail (`margin-top: auto`).
   - `#app-main.wrap` occupies the second grid column (`width: 100%; max-width: var(--wrap-w-desktop, 75rem); margin: 0 auto; padding: 2rem 2.5rem 4rem;`).
2. **Mobile Compact Top Bar + Thumb-Zone Bottom Tab Dock (`@media (max-width: 767px)`):**
   - The top bar (`.hebrew-shell`) remains at the top of the page as a compact single-row header (`padding: 0.5rem 0.85rem;`) containing `.hebrew-brand`, `.hebrew-shell-sync`, and `.hebrew-shell-account` (language toggle, theme toggle, account menu button).
   - `.hebrew-shell-nav` docks to the bottom of the viewport (`position: fixed; bottom: 0; inset-inline: 0; z-index: 40; background: var(--color-surface); border-top: 1px solid var(--color-line); padding: 0.35rem 0.5rem calc(0.35rem + env(safe-area-inset-bottom, 0px));`).
   - Nav items inside the mobile bottom dock arrange in a clean, non-overflowing grid/bar (`display: grid; grid-auto-flow: column; grid-auto-columns: minmax(0, 1fr);` for the 6 learner links, or a compact 44px-tall responsive grid when admin links are present) where every `.hebrew-shell-link` stacks its icon above its compact label (`flex-direction: column; justify-content: center; min-height: 44px; padding: 0.25rem 0.2rem; font-size: 0.75rem;`) so that at `360px` width:
     - Every `.hebrew-shell-link` has `height >= 44px` (`test/chrome-restyle-a11y-smoke.js` line 122).
     - `document.documentElement.scrollWidth <= window.innerWidth` (`360px` overflow check in `test/chrome-restyle-a11y-smoke.js` line 283).
     - All 6 learner links (or 7 admin links) remain visible in the DOM (`out.nav.links` check in `test/chrome-restyle-a11y-smoke.js` lines 307 & 318).
   - `#app-main.wrap` gets `padding-bottom: calc(6rem + env(safe-area-inset-bottom, 0px))` and `html` gets `scroll-padding-bottom: 6.5rem` so focused controls are never obscured by the fixed bottom bar (`WCAG 2.4.11`).
3. **Tablet Intermediate Layout (`@media (min-width: 768px) and (max-width: 1023px)`):**
   - Clean 2-row or single-row header with `max-width: 60rem` (`960px`) container padding.

**Scope (out):** Page-internal Bento grids (`SPEC-004`) and Lesson Player / SRS tray internals (`SPEC-005`).

## Understood

- [ ] und-1: `test/chrome-restyle-a11y-smoke.js` asserts `a.nav.links === 6` for a learner and `a.nav.links === 7` for an admin, and checks that every visible `.hebrew-shell-link` has `height >= 43.5px`. None of the `a[data-nav]` links may be `display: none` at `360px` or `1280px`.
- [ ] und-2: `test/restyle-gate.test.js` line 248 checks that the `/* === INIT-005/SPEC-011 restyle learner === */` section still contains `.hebrew-shell-link { ... min-height: 44px }`.
- [ ] und-3: All existing IDs (`#hebrew-shell-account-btn`, `#hebrew-shell-account-menu`, `#hebrew-shell-pw-dialog`, etc.) and classes (`.hebrew-shell`, `.hebrew-shell-inner`, `.hebrew-shell-nav`, `.hebrew-shell-link`, `.hebrew-brand`) must be preserved.

## Acceptance Criteria

- [ ] ac-1: At `1280×900` and `1440×960` (Desktop), `.hebrew-shell` renders as a vertical sidebar rail (`width: 260px`) beside `#app-main`, and `.hebrew-shell-nav` links stack vertically with `flex-direction: column` (`REQ-001`).
- [ ] ac-2: At `360×800` and `390×844` (Mobile), `.hebrew-shell-nav` is fixed to the bottom of the viewport (`bottom: 0`), every `.hebrew-shell-link` has rendered height `>= 44px`, and `document.documentElement.scrollWidth <= window.innerWidth` (`REQ-002`, `REQ-007`).
- [ ] ac-3: Switching between Hebrew (`dir="rtl"`) and English (`dir="ltr"`) mirrors the Desktop sidebar rail and Mobile bottom dock cleanly via CSS logical properties (`REQ-008`).
- [ ] ac-4: `test/app-shell.test.js`, `test/restyle-gate.test.js`, and `test/chrome-restyle-a11y-smoke.js` pass with zero errors (`REQ-007`).

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | Admin link count on mobile | All 7 admin `a[data-nav]` links stay visible in `.hebrew-shell-nav` at `360px` (using compact vertical icon+label cells so all 7 fit within `360px` without horizontal page overflow or `display: none`) | resolved | yes |

## Deliverables

- [ ] `shared/app-shell.js` (structural grouping hooks for rail header/footer if needed, preserving all existing DOM hooks)
- [ ] `shared/layout.js` (workspace topbar adjustments for desktop vs. mobile)
- [ ] `shared/worksheet.css` (responsive shell rules for `<768px`, `768px–1023px`, and `≥1024px`)
- [ ] `test/app-shell.test.js` (assertions for shell markup and responsive classes)

## Technical Requirements

- [ ] Use CSS logical properties (`border-inline-end`, `inset-inline`, `padding-inline`, `margin-inline`) throughout the shell rules.
- [ ] Preserve `min-height: 44px` on `.hebrew-shell-link` in the `INIT-005/SPEC-011` section of `shared/worksheet.css` to satisfy `test/restyle-gate.test.js` line 248.

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/app-shell.test.js test/restyle-gate.test.js`.
- **Rollback plan:** Revert shell CSS grid/fixed-dock rules in `shared/worksheet.css`.

## Security

- **Sensitivity:** `standard`.
- **Security acceptance criteria:**
  - [ ] Account display name and all nav attributes remain escaped via `escapeHtml()`.
  - [ ] Password dialog (`#hebrew-shell-pw-dialog`) and nav-lock (`hebrew-shell-nav-locked`) continue to block navigation on both desktop rail and mobile dock when a password change is required.

## Verification Strategy

- **Claim:** The application shell adapts into a `260px` vertical sidebar rail on desktop (`≥1024px`) and a compact header + fixed `≥44px` bottom tab dock on mobile (`<768px`) without breaking any accessibility or hook test.
- **Check + executor:** `mechanized` — `node --test test/app-shell.test.js test/restyle-gate.test.js` and `node test/chrome-restyle-a11y-smoke.js`.
- **Pass condition:** All unit tests pass, and at `360px` and `1280px` viewports `scrollWidth <= innerWidth` with all nav links visible and `>= 44px` tall.

## Integration Points

- Provides the outer responsive frame for `SPEC-004` (Today & Catalog Bento grids) and `SPEC-005` (Lesson Player & SRS stage).
- **Provenance tag:** `INIT-013/SPEC-003`.
