# SPEC-004 · frontend: Desktop Bento workspace & Mobile compact feed for Today, Lessons, Words, Guides, and Progress

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-013
title: "Desktop Bento workspace and Mobile compact feed for Today, Lessons, Words, Guides, and Progress"
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
blocked_by: [SPEC-002, SPEC-003]
blocks: [SPEC-006]
```

## Description

**Context:** Even with a responsive shell (`SPEC-003`), the learner views (`Today`, `Lessons`, `Guides`, `Words`, `Progress`) currently stack every card in a single vertical list (`display: flex; flex-direction: column; gap: 0.75rem`) designed for a `640px` column. On desktop (`≥1024px`), this wastes horizontal real estate and forces long scrolling; on mobile (`<768px`), cards have oversized padding and lack mobile-native density.

This spec implements the **Desktop Bento Workspace** (Figma `31:2`) and **Mobile Compact Feed** (Figma `33:2`) across `shared/today-page.js`, `shared/lessons-page.js`, `shared/words-page.js`, `shared/guides-page.js`, `shared/progress-page.js`, and `shared/worksheet.css`.

**Scope (in):**

1. **Today Workspace (`shared/today-page.js` + `shared/worksheet.css`, Figma `31:2` & `33:2`):**
   - **Hero Continue / Next Lesson Card (`#today-continue-card`, `#today-next-card`):**
     - Styled as the high-contrast **Judean Midnight** focal card (`background-color: var(--color-surface-hero); color: #FFFFFF; border-radius: 22px;`) with clear eyebrow metadata, prominent pointed Hebrew title (`Frank Ruhl Libre`), and a high-contrast Tekhelet Indigo primary CTA button (`#today-continue-link`, `#today-next-link`).
   - **Desktop Asymmetric 2-Column Bento Grid (`@media (min-width: 1024px)`):**
     - `.today-list[data-today-state="ready"]` becomes a 12-column CSS Grid (`display: grid; grid-template-columns: minmax(0, 7fr) minmax(0, 5fr); gap: 1.5rem; align-items: start;`).
     - Primary column (`grid-column: 1`): Hero Continue/Next card (`#today-continue-card` / `#today-next-card` / `#today-welcome-card`) and Today's Balanced Practice Queue (`#today-mix`).
     - Secondary intelligence column (`grid-column: 2`): SRS Review card (`#today-review-card`) and secondary lesson/competence cards.
   - **Mobile Compact Feed (`@media (max-width: 767px)`):**
     - Tight `0.85rem` vertical card spacing, `1rem` (`16px`) internal card padding, and full-width primary action buttons (`width: 100%; justify-content: center; min-height: 46px;`).
   - **Today Practice Mix Rows (`.today-mix-row`):**
     - Styled as tactile step rows with step indicators, clear Hebrew/English typography, and pill action buttons.
2. **Lessons & Guides Catalog Views (`shared/lessons-page.js`, `shared/guides-page.js`):**
   - On Desktop (`≥1024px`), lesson cards and guide lists arrange in a responsive multi-column Bento grid (`grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 1.25rem;`) while keeping opened guide reading prose capped at `65ch` for optimal reading comfort.
   - On Mobile (`<768px`), lesson cards stack cleanly in a single column with full-width tap areas.
3. **Lexicon Words & Competence Progress Views (`shared/words-page.js`, `shared/progress-page.js`):**
   - On Desktop (`≥1024px`), summary stat cards arrange in a 4-up metric row and word/root cards render in a 2- or 3-column responsive grid.
   - On Mobile (`<768px`), summary stats arrange in a compact 2-up strip (`grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.65rem;`) matching Figma `33:2`.

**Scope (out):** Lesson Player interactive blocks and SRS Flashcard session screen (`SPEC-005`).

## Understood

- [ ] und-1: All existing DOM IDs (`#today-review-card`, `#today-continue-card`, `#today-next-card`, `#today-welcome-card`, `#today-mix`, `#today-mix-list`, `#today-review-start`, `#today-continue-link`, `#today-next-link`) and `data-today-card` / `data-today-state` attributes in `shared/today-page.js` are asserted by tests and must remain intact.
- [ ] und-2: `test/chrome-restyle-a11y-smoke.js` checks computed text contrast against ancestor `backgroundColor` on every route in both Light and Dark mode. The Hero card (`#today-continue-card` / `#today-next-card`) must set an explicit solid `background-color` and explicit light text colors for all child headings, paragraphs, and links so `pageAudit()` measures `>= 4.5:1` contrast on every text node inside the Hero card.
- [ ] und-3: No view script may render its own `<h1>` (`test/restyle-gate.test.js` line 192: only `shared/layout.js` renders the route `<h1>`).

## Acceptance Criteria

- [ ] ac-1: At `1280×900` and `1440×960` (Desktop), `.today-list[data-today-state="ready"]` uses CSS Grid with two columns, placing the Hero lesson card and `#today-mix` alongside `#today-review-card` (`REQ-003`).
- [ ] ac-2: `#today-continue-card` and `#today-next-card` use the Judean Midnight hero surface (`#141B2D` in Light, `#1D2842` in Dark) with `>= 4.5:1` contrast on all child text and CTA buttons (`REQ-003`, `REQ-005`, `REQ-007`).
- [ ] ac-3: At `1280×900` (Desktop), `Lessons`, `Words`, and `Progress` render multi-column card/metric grids instead of a `640px` single column (`REQ-004`).
- [ ] ac-4: At `360×800` and `390×844` (Mobile), all cards in `Today`, `Lessons`, `Guides`, `Words`, and `Progress` stack without horizontal overflow (`scrollWidth <= innerWidth`) and primary CTA buttons expand to full card width with `height >= 44px` (`REQ-002`, `REQ-003`, `REQ-007`).

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | Today page when only 1 card is present (e.g., new user welcome state) | When `.today-list` has only one child (`:only-child`), it spans across the grid gracefully so it never looks lopsided | resolved | yes |

## Deliverables

- [ ] `shared/worksheet.css` (Bento grid rules for `.today-list`, Hero card styles, responsive multi-column grids for `.lessons-list`, `.words-list`, `.progress-*`, and mobile compact card density)
- [ ] `shared/today-page.js` (optional non-breaking class/layout hooks if needed, keeping all existing IDs and `data-*` attributes)
- [ ] Tests: existing `test/today-*.test.js` and `test/restyle-gate.test.js`

## Technical Requirements

- [ ] Use pure CSS Grid (`grid-template-columns`, `grid-column`) for the Desktop Bento arrangement so DOM order remains logical for keyboard Tab navigation and screen readers.
- [ ] Ensure every text element inside dark Hero cards (`#today-continue-card`, `#today-next-card`) inherits or sets a solid foreground color with `>= 4.5:1` contrast against `#141B2D` / `#1D2842`.

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/restyle-gate.test.js` and Today/Learner view tests.
- **Rollback plan:** Revert Bento grid CSS rules in `shared/worksheet.css`.

## Security

- **Sensitivity:** `standard`.
- **Security acceptance criteria:**
  - [ ] All dynamic strings in `shared/today-page.js` continue using `esc()` / `bi()`.
  - [ ] No new `<h1>` tags introduced in view scripts.

## Verification Strategy

- **Claim:** `Today`, `Lessons`, `Guides`, `Words`, and `Progress` render as multi-column Bento/grid workspaces on Desktop (`≥1024px`) and compact, thumb-friendly feeds on Mobile (`<768px`) with zero contrast or overflow failures.
- **Check + executor:** `mechanized` — `node --test test/restyle-gate.test.js` and `node test/chrome-restyle-a11y-smoke.js`.
- **Pass condition:** All routes pass `chrome-restyle-a11y-smoke.js` at `360px` and `1280px` in both Light and Dark themes.

## Integration Points

- Works inside `SPEC-003`'s adaptive shell and feeds into `SPEC-006`'s verification matrix.
- **Provenance tag:** `INIT-013/SPEC-004`.
