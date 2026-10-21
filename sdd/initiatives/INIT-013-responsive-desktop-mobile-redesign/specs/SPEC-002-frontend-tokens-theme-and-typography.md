# SPEC-002 · frontend: Design tokens, Light/Dark theme engine, and typography hierarchy in `shared/worksheet.css`

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-013
title: "Design tokens, Light/Dark theme engine, and typography hierarchy in shared/worksheet.css"
domain: frontend
status: complete
primary_prompt: .claude/agents/frontend-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 5 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003, SPEC-004, SPEC-005]
```

## Description

**Context:** `shared/worksheet.css` currently has three overlapping token blocks: the `ADR-0006` block (lines 1–215), a legacy worksheet `:root` block, and the `INIT-005/SPEC-011` restyle block (line 4251+), which pinned dark mode off via `:root:not([data-theme="dark"])` and left dark mode incomplete.

This spec implements the **Jerusalem Stone & Tekhelet Indigo** token foundation from `ADR-0015` (`SPEC-001`) and Figma board `30:20` (`Derekh Ivrit Tokens`), restores a working Light/Dark theme engine (`:root[data-theme="light"]`, `:root[data-theme="dark"]`, and `@media (prefers-color-scheme: dark)`), and establishes the responsive spacing, elevation, radius, and bilingual typography scale that `SPEC-003`, `SPEC-004`, and `SPEC-005` consume.

**Scope (in):**

1. **Active Light & Dark Theme Tokens in `shared/worksheet.css`:**
   - **Light Theme (`Jerusalem Stone`):**
     - `--paper` / `--color-bg`: `#F7F5F0`
     - `--color-surface`: `#FFFFFF`
     - `--color-surface-soft`: `#EFECE6`
     - `--color-surface-hero`: `#141B2D`
     - `--color-text` / `--ink`: `#121621`
     - `--color-muted` / `--muted`: `#4B5565`
     - `--color-primary`: `#254EDB`
     - `--color-primary-hover`: `#1D3FB8`
     - `--color-primary-soft`: `#EBF0FF`
     - `--color-primary-ink`: `#FFFFFF`
     - `--color-correct-ink`: `#065F46`, `--color-correct-soft`: `#D1FAE5`, `--color-accent-sage`: `#0D9488`
     - `--color-miss-ink`: `#92400E`, `--color-miss-soft`: `#FEF3C7`, `--color-accent-terracotta`: `#D97706`
     - `--color-line`: `#757C8A` (ensuring `≥ 3.0:1` UI contrast against both `#FFFFFF` and `#F7F5F0` per `test/restyle-gate.test.js`) and `--color-border-subtle`: `#E4E0D8` for decorative hairlines.
   - **Dark Theme (`Judean Night`, `:root[data-theme="dark"]`):**
     - `--paper` / `--color-bg`: `#0E121B`
     - `--color-surface`: `#161C2A`
     - `--color-surface-soft`: `#1E2638`
     - `--color-surface-hero`: `#1D2842`
     - `--color-text` / `--ink`: `#F3F4F6`
     - `--color-muted` / `--muted`: `#A3B1C6`
     - `--color-primary`: `#6E93FF` (ensuring `≥ 4.5:1` against `#0E121B`, `#161C2A`, and `--color-primary-soft: #172347`, with `--color-primary-ink: #0B0F19` `≥ 4.5:1`)
     - `--color-primary-hover`: `#8AA8FF`
     - `--color-correct-ink`: `#A7F3D0`, `--color-correct-soft`: `#0F2E2B`
     - `--color-miss-ink`: `#FDE68A`, `--color-miss-soft`: `#3B280C`
     - `--color-line`: `#687792` (`≥ 3.0:1` against `#0E121B` and `#161C2A`).
2. **Responsive Layout & Typography Custom Properties:**
   - Define `--shell-rail-w: 260px`, `--shell-mobile-dock-h: 68px`, `--wrap-w-desktop: 75rem` (`1200px`), `--wrap-w-reading: 42rem` (`672px`), `--radius-card: 20px`, `--radius-control: 12px`, `--radius-pill: 999px`.
   - Define the bilingual font stacks (`--font-hebrew-display`, `--font-hebrew-sans`, `--font-sans`, `--font-mono`) using local/system font fallbacks with zero external font network requests (`REQ-018`).
3. **Theme Persistence Helper in `shared/app-shell.js`:**
   - Read/write `localStorage.getItem('hebrew-theme')` (`'light'` | `'dark'`), apply `data-theme` on `document.documentElement`, and expose a theme toggle control in the shell header/rail with bilingual accessible labels (`מצב כהה` / `Dark mode`).

**Scope (out):** Structural layout changes to the navigation rail/bottom dock (`SPEC-003`), Today/Catalog Bento grids (`SPEC-004`), or Lesson Player/SRS trays (`SPEC-005`).

## Understood

- [ ] und-1: `test/restyle-gate.test.js` parses top-level `:root`, `:root:not([data-theme="dark"])`, `:root[data-theme="light"]`, and `:root[data-theme="dark"]` rules and checks 19 foreground/background token pairs for plain `#hex` format and WCAG 2.2 AA contrast. Every single pair must pass in both `LIGHT` and `DARK`.
- [ ] und-2: `test/design-tokens.test.js` parses the `/* === ADR-0006 design tokens === */` region at the top of `shared/worksheet.css` — that block must remain untouched.
- [ ] und-3: No external `@import` or `url(https://...)` font requests may be added (`test/design-tokens.test.js` and `test/chrome-restyle-a11y-smoke.js` enforce same-origin only).

## Acceptance Criteria

- [ ] ac-1: All 19 `LIGHT` and 19 `DARK` token contrast pairs in `test/restyle-gate.test.js` pass (`≥ 4.5:1` for text pairs, `≥ 3.0:1` for UI pairs), and `test/design-tokens.test.js` passes (`REQ-005`, `REQ-007`).
- [ ] ac-2: Setting `document.documentElement.setAttribute('data-theme', 'dark')` switches the page canvas, card surfaces, text, borders, and primary buttons to the Dark (`Judean Night`) palette without any hardcoded light backgrounds bleeding through (`REQ-005`).
- [ ] ac-3: A user can toggle between Light and Dark themes via a control in `.hebrew-shell` (`min-height: 44px`, bilingual label via `bi()`), and the choice persists in `localStorage` under `'hebrew-theme'` (`REQ-005`).
- [ ] ac-4: Pointed Hebrew text tokens maintain `line-height: 1.55` to `1.7` and `>= 24px` (`1.5rem`) on target study words so nikud vowel marks never clip (`REQ-008`, `RSCH-006`).

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | Default theme when no `localStorage` key is set | Default to `'light'` (`Jerusalem Stone`) unless the user toggles dark mode or `data-theme="dark"` is set, preserving existing screenshot smoke expectations | resolved | yes |

## Deliverables

- [ ] `shared/worksheet.css` (updated theme token blocks and typography/layout variables)
- [ ] `shared/app-shell.js` (Light/Dark theme state & toggle button)
- [ ] `test/app-shell.test.js` and `test/restyle-gate.test.js` (verification of theme toggle and token pairs)

## Technical Requirements

- [ ] Every token in `test/restyle-gate.test.js`'s `PAIRS` array must resolve to a literal `#rrggbb` hex value in top-level `:root` / `:root[data-theme="light"]` / `:root[data-theme="dark"]` rules (the test regex requires `/^#[0-9a-fA-F]{3,6}$/`).
- [ ] Theme values read from `localStorage` must be validated against `val === 'dark' ? 'dark' : 'light'` before setting `data-theme`.

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/design-tokens.test.js test/restyle-gate.test.js test/player-ui-a11y.test.js test/app-shell.test.js`.
- **Rollback plan:** Revert token block in `shared/worksheet.css` and theme toggle in `shared/app-shell.js`.

## Security

- **Sensitivity:** `standard`.
- **Security acceptance criteria:**
  - [ ] `localStorage` theme value is strictly allowlisted to `'light'` or `'dark'`; arbitrary strings are ignored.
  - [ ] Theme toggle button uses `bi()` and carries no English-only `aria-label` or `title` (`REQ-016` check in `test/restyle-gate.test.js`).
  - [ ] Zero external font or asset URLs (`REQ-018`).

## Verification Strategy

- **Claim:** The Jerusalem Stone (Light) and Judean Night (Dark) token sets satisfy all automated WCAG 2.2 AA contrast assertions and switch cleanly at runtime.
- **Check + executor:** `mechanized` — `node --test test/design-tokens.test.js test/restyle-gate.test.js test/player-ui-a11y.test.js test/app-shell.test.js`.
- **Pass condition:** Zero test failures across all four test suites.

## Integration Points

- Provides CSS custom properties consumed by `SPEC-003`, `SPEC-004`, and `SPEC-005`.
- **Provenance tag:** `INIT-013/SPEC-002`.
