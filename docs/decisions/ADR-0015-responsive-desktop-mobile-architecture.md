# ADR-0015: Responsive desktop & mobile architecture (Jerusalem Stone & Tekhelet Indigo)

## Metadata

| Field | Value |
|-------|-------|
| **Status** | Accepted (pending operator read of this record) |
| **Date** | 2026-10-17 |
| **Authored by** | `INIT-013/SPEC-001` |
| **Initiative** | INIT-013 · responsive-desktop-mobile-redesign |
| **Research grounding** | `RSCH-001` (strand balance ~¼ deliberate / ~¾ meaning); `RSCH-003` (interleaving, spacing, self-repair feedback); `RSCH-004` (`N7`); `RSCH-005` (hidden-stem recall); `RSCH-006` (WCAG 2.2 AA/AAA, ≥44px targets, nikud line height, thumb-zone); `RSCH-007` (icon + visible label, competence without streaks); `RSCH-008` (informational motion, `prefers-reduced-motion`); `RSCH-009` (human-verified pointing, `N13`/`N14`); `RSCH-010` (Consistent Navigation 3.2.3, Location 2.4.8) |
| **Extends** | [ADR-0006](ADR-0006-visual-language-and-interface-rules.md) (70 `data-*` / 50 class hooks, WCAG floors, `--size-target*`). [ADR-0007](ADR-0007-spa-shell-and-router.md) (shell & router). [ADR-0008](ADR-0008-learner-look-and-feedback.md) (learner look & dark mode). [ADR-0009](ADR-0009-learner-model-and-language-program.md) (Decisions 4, 6 & 8; `N13`/`N14`). [ADR-0010](ADR-0010-player-visual-language-and-competence-feedback.md) (competence feedback). [ADR-0014](ADR-0014-lesson-type-coverage.md) (Type D `verb-workshop`, `exercise`, `assignment`, `tense: "future"`, D8, D12, D13). |
| **Amends** | Layout contract of ADR-0006 / ADR-0008 / ADR-0010: replaces the single `40rem` (`640px`) `.wrap` column and wrapped top-pill `.hebrew-shell-nav` with a 3-tier adaptive shell. Does **not** retire hooks, target sizes, reduced-motion reset, or competence-only feedback. |
| **Supersedes** | Fixed single-column learner layout (`--wrap-w: 40rem`) and multi-row top navigation wrap as the default chrome. |
| **Superseded by** | — |
| **Figma source** | File [`NJ8HfTPCQgrFty2VVQBMhY`](https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY) · variable collection `VariableCollectionId:30:2` (Light & Dark) |

## Summary

The learner app stops being a fixed `640px` page with a wrapping top nav. **ADR-0015** fixes three contracts before any CSS or JS lands in `SPEC-002`–`SPEC-005`:

1. **3-tier breakpoints** — Mobile (`<768px`, tested to `320px`/`360px`), Tablet (`768px–1023px`), Desktop (`≥1024px`, widescreen `≥1280px`) for the shell, Today workspace, catalog views, and Lesson/SRS player (`REQ-001`–`REQ-004`, `REQ-006`).
2. **Jerusalem Stone (Light) & Judean Night (Dark)** — exact hex tokens for every pair `test/restyle-gate.test.js` checks, plus hero accents (`REQ-005`, `REQ-007`).
3. **Research-backed verb-workshop rules (`R1`–`R6`)** — interleaved hidden-stem recall, one retry before reveal, multi-day `form` signals, ~¼ pattern / ~¾ contextual output, human-verified pointing, and tabular/pad ergonomics (`REQ-009`–`REQ-011`).

Production files stay unchanged in this spec. Downstream specs implement the contract; this record is the single source they cite.

---

## Context

### Problem statement

Across `INIT-005` (`ADR-0006`, `ADR-0008`) and `INIT-007` (`ADR-0010`), the app kept a single `40rem` (`640px`) centered column (`.wrap`) and a top header whose `.hebrew-shell-nav` links wrap into multiple rows at every viewport width. On desktop monitors (`1280px–1920px`), most of the screen is empty while cards stack vertically. On phones (`360px–430px`), the wrapped top bar consumes roughly `150px` in the hardest-to-reach zone and cards keep desktop padding. Three eras of CSS tokens also coexist in `shared/worksheet.css`, and dark mode was left pinned off.

Before `SPEC-002` through `SPEC-005` touch `shared/worksheet.css` and `shared/app-shell.js`, this ADR names the responsive layout, the Figma frame map, the Light/Dark token table, and the compatibility rules that keep `ADR-0006` and `ADR-0010` test gates green.

### Requirements this record satisfies

| Id | Requirement |
|----|-------------|
| REQ-001 | Desktop persistent `260px` sidebar rail on `.hebrew-shell` at `≥1024px`. |
| REQ-002 | Mobile thumb-zone fixed bottom dock for `.hebrew-shell-nav` at `<768px`. |
| REQ-003 | Fluid `1200px` max-width workspace + Today Bento / compact feed. |
| REQ-004 | Multi-column desktop grids for Lessons, Guides, Words, Progress. |
| REQ-005 | Jerusalem Stone & Tekhelet Indigo Light + Dark palette + theme toggle contract. |
| REQ-006 | Ergonomic Lesson Player & SRS stage (desktop split / mobile thumb tray). |
| REQ-007 | Zero hook or accessibility regressions (70 `data-*`, 50 classes, 19 contrast pairs, targets). |
| REQ-009 | Multi-verb / multi-tense drill cues + ergonomic verb stage (`R1`, `R6`). |
| REQ-010 | Schema `1.2.0` multi-binyan workshop seed shape (documented here; seeded in `SPEC-007`). |
| REQ-011 | Research-ratio & verification guardrails (`R4`, `R5`, `N7`, `N13`/`N14`). |

---

## 1. Three-tier breakpoint contract

Breakpoints are CSS viewport widths. Logical properties (`inset-inline`, `margin-inline`, `padding-inline`, `border-inline-end`) are mandatory so Hebrew (`dir="rtl"`) and English (`dir="ltr"`) mirror cleanly (`REQ-008`).

| Tier | Range | Stage width | Shell chrome |
|------|-------|-------------|--------------|
| **Mobile** | `<768px` (tested down to `320px` / `360px`) | Full bleed; `16px` inline padding on `#app-main.wrap` | Compact `52px` top context bar + fixed thumb-zone bottom nav dock |
| **Tablet** | `768px`–`1023px` | Adaptive cards, `max-width: 960px` | Compact top navigation (no bottom dock; no full desktop rail) |
| **Desktop** | `≥1024px` (widescreen design frames at `≥1280px` / `1440px`) | Main stage `max-width: 1200px` (`75rem`) beside persistent rail | Persistent `260px` vertical sidebar rail |

Primary interactive targets remain `≥44×44` CSS px (`--size-target-primary: 44px`); minimum targets `≥24×24` CSS px (`--size-target: 24px`) — unchanged from `ADR-0006` / `test/restyle-gate.test.js`.

### 1.1 Application shell (`.hebrew-shell`, `.hebrew-shell-nav`, `#app-main`)

**Owners:** `shared/app-shell.js` (markup), `shared/app.html` (`#app-shell`, `#app-main`), `shared/worksheet.css` (layout rules — `SPEC-003`).

| Tier | Layout rule |
|------|-------------|
| **Mobile `<768px`** | Top bar is a single `52px` row: brand (`.hebrew-brand` / `[data-brand]`), language/theme toggles, account menu. `.hebrew-shell-nav` docks to `position: fixed; bottom: 0; inset-inline: 0` with `min-height: 64px` (or `--shell-mobile-dock-h: 68px`) plus `env(safe-area-inset-bottom)`. Each `.hebrew-shell-link` keeps `min-height: 44px`, icon + visible label (`RSCH-007`). `#app-main` gains bottom padding `calc(5.5rem + env(safe-area-inset-bottom, 0px))` and `scroll-padding-bottom: 6rem` so content and focus are never obscured (WCAG 2.4.11). Zero horizontal overflow at `360px` (`documentElement.scrollWidth <= 360`). |
| **Tablet `768–1023px`** | Compact horizontal top nav (no wrap into 2–3 rows of pills). Main column centered at `max-width: 960px`. No fixed bottom dock; no persistent `260px` rail. |
| **Desktop `≥1024px`** | `.hebrew-shell` becomes a persistent vertical rail `--shell-rail-w: 260px` beside `#app-main` (flex/grid row using logical start). Rail holds brand lockup, grouped `[data-nav]` links, calm session/sync (`[data-shell-sync]`), theme toggle, and account footer. `#app-main.wrap` expands to `--wrap-w-desktop: 75rem` (`1200px`). Navigation does not wrap across top rows (`REQ-001`). |

### 1.2 Today workspace (`#today-continue-card`, `#today-mix`, `#today-review-card`, `.today-bento-grid`)

**Owners:** `shared/today-page.js`, styles in `shared/worksheet.css` (`SPEC-004`).

| Tier | Layout rule |
|------|-------------|
| **Mobile** | Single-column edge-to-edge feed: `#today-continue-card` (hero continue/next), then `#today-mix` (balanced practice queue), then `#today-review-card` (SRS status). Compact card padding; full-width primary CTAs (`≥44px` height). Optional 2-up metric strips only when they remain ≥44px targets. |
| **Tablet** | Two-column adaptive cards inside `max-width: 960px` when space allows; otherwise stack. |
| **Desktop** | Asymmetric 12-column Bento via `.today-bento-grid`: primary column `7fr` (Judean Midnight hero `#today-continue-card` + `#today-mix`), secondary intelligence rail `5fr` (`#today-review-card` + lexicon/competence widgets). Hero surface uses `--color-hero-bg` / `--color-hero-text` (`REQ-003`). |

### 1.3 Catalog views (Lessons, Guides, Words, Progress)

**Owners:** `shared/lessons-page.js`, `shared/guides-page.js`, `shared/words-page.js`, `shared/progress-page.js`, `shared/worksheet.css` (`SPEC-004`).

| Tier | Layout rule |
|------|-------------|
| **Mobile** | Single-column lists of `.lesson-card` / catalog rows; `16px` inline padding; full-width actions. |
| **Tablet** | Two-column card grids where density helps; prose still readable. |
| **Desktop** | Multi-column CSS Grid for catalog cards. Reading prose blocks (guide bodies, progress explanations) stay capped at a comfortable `60ch`–`70ch` measure (`--wrap-w-reading: 42rem` when a reading column is needed) (`REQ-004`). |

### 1.4 Lesson player & SRS stage (`REQ-006`)

**Owners:** `shared/ui/plan-render.js`, `shared/ui/blocks/*` (especially `conjugation-table.js`, `exercise.js`), `shared/player/core.js`, `shared/review-page.js`, `shared/worksheet.css` (`SPEC-005`).

| Tier | Lesson player | SRS review (`#review-card`) |
|------|---------------|------------------------------|
| **Mobile `<768px`** | Exercise choices (`choose`, `true-false`), inline `.exercise-why` / competence feedback, Continue CTAs, `[data-letter-pad]` (22-letter Hebrew pad), and drill actions dock into a thumb-friendly bottom tray (`≥44px` primary height). Zero layout jump; zero horizontal overflow at `360px`. | Four recall rating buttons (Again / Hard / Good / Easy from `shared/review-page.js`) sit in the lower thumb arc, full-width or equal-flex, `≥44px` height. |
| **Tablet** | Comfortable single stage; option grids may be 2-up when width allows. | Ratings remain reachable without overlapping chrome. |
| **Desktop `≥1024px`** | Split-context stage: exercise / drill on the primary column; grammar / conjugation reference (`conjugation-table` `mode: "study"`, explainer tables) on a side drawer or adjacent column. `choose` / `true-false` options use a responsive 2-column grid; keyboard shortcut badges may show. | Wide card with side-by-side prompt and rating row; ratings stay ≥44px. |

**Motion:** Informational only (`RSCH-008`). Global `@media (prefers-reduced-motion: reduce)` reset remains mandatory (`ADR-0006`). No streaks, points, or leaderboards (`RSCH-006`, `RSCH-007`).

---

## 2. Jerusalem Stone & Tekhelet Indigo token table

### 2.1 How both token tests stay green (`REQ-005`, `REQ-007`)

| Gate | What it reads | ADR-0015 rule |
|------|---------------|---------------|
| `test/design-tokens.test.js` | The comment-delimited block `/* === ADR-0006 design tokens === */` in `shared/worksheet.css`, checked against `docs/decisions/ADR-0006-assets/tokens.json` | **Leave that block byte-intact.** It is the historical baseline (`ADR-0006`). Active theme values live in later `:root` / `:root[data-theme="light"]` / `:root[data-theme="dark"]` rules authored by `SPEC-002`. |
| `test/restyle-gate.test.js` | Final resolved tokens from top-level `:root`, `:root:not([data-theme="dark"])`, `:root[data-theme="light"]`, and `:root[data-theme="dark"]` rules; 19 foreground/background pairs; plain `#rrggbb` only | **Every active pair below must meet its ratio.** Text pairs `≥4.5:1`; UI pairs (`--color-line` borders, focus-ring uses of `--color-primary`) `≥3.0:1`. |

Theme persistence (`SPEC-002`): `localStorage` key `hebrew-theme` allowlisted to `'light' | 'dark'`; applied as `document.documentElement.dataset.theme`. Default without a stored value: `'light'` (Jerusalem Stone).

### 2.2 Light theme — Jerusalem Stone

| Token | Hex | Role |
|-------|-----|------|
| `--paper` / `--color-bg` | `#F7F5F0` | Page canvas |
| `--color-surface` | `#FFFFFF` | Card / elevated surface |
| `--color-surface-soft` | `#EFECE6` | Soft panel |
| `--color-text` / `--ink` | `#121621` | Body text |
| `--color-muted` / `--muted` | `#4B5565` | Secondary text |
| `--color-primary` | `#254EDB` | Tekhelet Indigo — links, chips, focus |
| `--color-primary-hover` | `#1D3FB8` | Primary hover |
| `--color-primary-soft` | `#EBF0FF` | Selected-chip tint |
| `--color-primary-ink` | `#FFFFFF` | Label on primary fill |
| `--color-correct-ink` | `#065F46` | Correct feedback text |
| `--color-correct-soft` | `#D1FAE5` | Correct feedback panel |
| `--color-miss-ink` | `#92400E` | Miss feedback text |
| `--color-miss-soft` | `#FEF3C7` | Miss feedback panel |
| `--color-line` | `#757C8A` | Control borders (UI contrast) |
| `--color-border-subtle` | `#E4E0D8` | Decorative hairlines (not in the 19-pair gate) |
| `--color-hero-bg` | `#141B2D` | Judean Midnight hero card |
| `--color-hero-text` | `#FFFFFF` | Hero foreground |
| `--color-accent-terracotta` | `#D97706` | SRS / root-letter accent |
| `--color-accent-sage` | `#0D9488` | Galilee Sage competence accent |
| `--color-surface-hero` | `#141B2D` | Alias for hero surface in theme engine |

### 2.3 Dark theme — Judean Night (`:root[data-theme="dark"]`)

| Token | Hex | Role |
|-------|-----|------|
| `--paper` / `--color-bg` | `#0E121B` | Page canvas |
| `--color-surface` | `#161C2A` | Card surface |
| `--color-surface-soft` | `#1E2638` | Soft panel |
| `--color-surface-hero` | `#1D2842` | Hero surface in dark |
| `--color-text` / `--ink` | `#F3F4F6` | Body text |
| `--color-muted` / `--muted` | `#A3B1C6` | Secondary text |
| `--color-primary` | `#6E93FF` | Primary (AA on canvas & surface) |
| `--color-primary-hover` | `#8AA8FF` | Primary hover |
| `--color-primary-soft` | `#172347` | Selected-chip tint |
| `--color-primary-ink` | `#0B0F19` | Label on primary fill |
| `--color-correct-ink` | `#A7F3D0` | Correct feedback text |
| `--color-correct-soft` | `#0F2E2B` | Correct feedback panel |
| `--color-miss-ink` | `#FDE68A` | Miss feedback text |
| `--color-miss-soft` | `#3B280C` | Miss feedback panel |
| `--color-line` | `#687792` | Control borders |
| `--color-hero-bg` | `#141B2D` | Hero card (explicit solid; not gradient-only) |
| `--color-hero-text` | `#FFFFFF` | Hero foreground |
| `--color-accent-terracotta` | `#D97706` | Root / SRS accent |
| `--color-accent-sage` | `#0D9488` | Competence accent |

### 2.4 The 19 `restyle-gate` pairs — measured contrast

Contrast uses the same relative-luminance formula as `test/restyle-gate.test.js` (`lin` / `luminance` / `contrast`). Ratios below were verified with that formula on 2026-10-17 (`INIT-013/SPEC-001`).

**Light (Jerusalem Stone)**

| # | Foreground | Background | Need | Measured | Role |
|---|------------|------------|------|----------|------|
| 1 | `--color-text` `#121621` | `--paper` `#F7F5F0` | ≥4.5 | **16.58:1** | Body text on page |
| 2 | `--color-text` `#121621` | `--color-surface` `#FFFFFF` | ≥4.5 | **18.07:1** | Body text on card |
| 3 | `--color-text` `#121621` | `--color-surface-soft` `#EFECE6` | ≥4.5 | **15.32:1** | Body text on soft panel |
| 4 | `--color-muted` `#4B5565` | `--paper` `#F7F5F0` | ≥4.5 | **6.92:1** | Muted on page |
| 5 | `--color-muted` `#4B5565` | `--color-surface` `#FFFFFF` | ≥4.5 | **7.54:1** | Muted on card |
| 6 | `--color-muted` `#4B5565` | `--color-surface-soft` `#EFECE6` | ≥4.5 | **6.39:1** | Muted on soft panel |
| 7 | `--muted` `#4B5565` | `--color-surface` `#FFFFFF` | ≥4.5 | **7.54:1** | Legacy `--muted` on card |
| 8 | `--muted` `#4B5565` | `--paper` `#F7F5F0` | ≥4.5 | **6.92:1** | Legacy `--muted` on page |
| 9 | `--color-primary-ink` `#FFFFFF` | `--color-primary` `#254EDB` | ≥4.5 | **6.58:1** | Primary button label |
| 10 | `--color-primary-ink` `#FFFFFF` | `--color-primary-hover` `#1D3FB8` | ≥4.5 | **8.55:1** | Primary label, hover |
| 11 | `--color-primary` `#254EDB` | `--color-surface` `#FFFFFF` | ≥4.5 | **6.58:1** | Link / chip text on card |
| 12 | `--color-primary` `#254EDB` | `--paper` `#F7F5F0` | ≥4.5 | **6.04:1** | Link text on page |
| 13 | `--color-primary` `#254EDB` | `--color-primary-soft` `#EBF0FF` | ≥4.5 | **5.78:1** | Chip text on tint |
| 14 | `--color-correct-ink` `#065F46` | `--color-correct-soft` `#D1FAE5` | ≥4.5 | **6.78:1** | Correct feedback |
| 15 | `--color-miss-ink` `#92400E` | `--color-miss-soft` `#FEF3C7` | ≥4.5 | **6.37:1** | Miss feedback |
| 16 | `--color-line` `#757C8A` | `--color-surface` `#FFFFFF` | ≥3.0 | **4.20:1** | Control border on card |
| 17 | `--color-line` `#757C8A` | `--paper` `#F7F5F0` | ≥3.0 | **3.85:1** | Control border on page |
| 18 | `--color-primary` `#254EDB` | `--color-surface` `#FFFFFF` | ≥3.0 | **6.58:1** | Focus ring on card |
| 19 | `--color-primary` `#254EDB` | `--paper` `#F7F5F0` | ≥3.0 | **6.04:1** | Focus ring on page |

**Dark (Judean Night)**

| # | Foreground | Background | Need | Measured | Role |
|---|------------|------------|------|----------|------|
| 1 | `--color-text` `#F3F4F6` | `--paper` `#0E121B` | ≥4.5 | **17.02:1** | Body text on page |
| 2 | `--color-text` `#F3F4F6` | `--color-surface` `#161C2A` | ≥4.5 | **15.47:1** | Body text on card |
| 3 | `--color-text` `#F3F4F6` | `--color-surface-soft` `#1E2638` | ≥4.5 | **13.73:1** | Body text on soft panel |
| 4 | `--color-muted` `#A3B1C6` | `--paper` `#0E121B` | ≥4.5 | **8.62:1** | Muted on page |
| 5 | `--color-muted` `#A3B1C6` | `--color-surface` `#161C2A` | ≥4.5 | **7.83:1** | Muted on card |
| 6 | `--color-muted` `#A3B1C6` | `--color-surface-soft` `#1E2638` | ≥4.5 | **6.95:1** | Muted on soft panel |
| 7 | `--muted` `#A3B1C6` | `--color-surface` `#161C2A` | ≥4.5 | **7.83:1** | Legacy `--muted` on card |
| 8 | `--muted` `#A3B1C6` | `--paper` `#0E121B` | ≥4.5 | **8.62:1** | Legacy `--muted` on page |
| 9 | `--color-primary-ink` `#0B0F19` | `--color-primary` `#6E93FF` | ≥4.5 | **6.64:1** | Primary button label |
| 10 | `--color-primary-ink` `#0B0F19` | `--color-primary-hover` `#8AA8FF` | ≥4.5 | **8.32:1** | Primary label, hover |
| 11 | `--color-primary` `#6E93FF` | `--color-surface` `#161C2A` | ≥4.5 | **5.90:1** | Link / chip text on card |
| 12 | `--color-primary` `#6E93FF` | `--paper` `#0E121B` | ≥4.5 | **6.50:1** | Link text on page |
| 13 | `--color-primary` `#6E93FF` | `--color-primary-soft` `#172347` | ≥4.5 | **5.32:1** | Chip text on tint |
| 14 | `--color-correct-ink` `#A7F3D0` | `--color-correct-soft` `#0F2E2B` | ≥4.5 | **11.32:1** | Correct feedback |
| 15 | `--color-miss-ink` `#FDE68A` | `--color-miss-soft` `#3B280C` | ≥4.5 | **11.28:1** | Miss feedback |
| 16 | `--color-line` `#687792` | `--color-surface` `#161C2A` | ≥3.0 | **3.76:1** | Control border on card |
| 17 | `--color-line` `#687792` | `--paper` `#0E121B` | ≥3.0 | **4.14:1** | Control border on page |
| 18 | `--color-primary` `#6E93FF` | `--color-surface` `#161C2A` | ≥3.0 | **5.90:1** | Focus ring on card |
| 19 | `--color-primary` `#6E93FF` | `--paper` `#0E121B` | ≥3.0 | **6.50:1** | Focus ring on page |

Hero pair `--color-hero-text` `#FFFFFF` on `--color-hero-bg` `#141B2D` is outside the 19-pair gate but is required for Continue/Next hero cards; it is solid `background-color` (not gradient-only) so live DOM contrast checks remain honest.

### 2.5 Layout & typography custom properties (consumed by later specs)

| Token | Value | Purpose |
|-------|-------|---------|
| `--shell-rail-w` | `260px` | Desktop sidebar rail |
| `--shell-mobile-dock-h` | `68px` | Mobile bottom dock height budget |
| `--wrap-w-desktop` | `75rem` (`1200px`) | Desktop main stage |
| `--wrap-w-reading` | `42rem` (`672px`) | Long-form measure |
| `--radius-card` | `20px` | Card radius |
| `--radius-control` | `12px` | Control radius |
| `--radius-pill` | `999px` | Pill radius |
| `--font-hebrew-display` / `--font-hebrew-sans` / `--font-sans` / `--font-mono` | Local/system stacks only | Bilingual type; **no** external `@import` / `url(https://…)` fonts (`REQ-018`) |

Pointed Hebrew study text: `line-height: 1.55`–`1.7`; target study words `≥1.5rem` (`24px`) so nikud does not clip (`RSCH-006`, `REQ-008`).

---

## 3. Figma traceability

File: [`NJ8HfTPCQgrFty2VVQBMhY`](https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY). Variable collection: `VariableCollectionId:30:2` (Light & Dark modes → §2 token table).

| Figma node | Frame name | Production files | DOM / CSS selectors |
|------------|------------|------------------|---------------------|
| `30:20` | `00 — Design System & Responsive Architecture` + Derekh Ivrit Tokens | `shared/worksheet.css` (active `:root` / theme rules — `SPEC-002`); token contract in this ADR | `:root`, `:root[data-theme="light"]`, `:root[data-theme="dark"]`; custom properties in §2 |
| `31:2` | `01 — Desktop App: Today Workspace (1440×960)` | `shared/today-page.js`, `shared/app-shell.js`, `shared/worksheet.css` (`SPEC-003`, `SPEC-004`) | `.hebrew-shell` (rail), `#app-main`, `.today-bento-grid`, `#today-continue-card`, `#today-mix`, `#today-review-card`, `.today-card` |
| `32:2` | `02 — Desktop App: Lesson Player & Context Drawer (1440×960)` | `shared/ui/plan-render.js`, `shared/ui/blocks/*`, `shared/player/core.js`, `shared/worksheet.css` (`SPEC-005`) | `.plan-page`, `[data-step-type]`, `.conj-table`, `[data-drill-station]`, `[data-drill-prompt]`, `.exercise-why`, `.player-top` |
| `33:2` | `03 — Mobile App: Today Feed & Bottom Dock (390×844)` | `shared/app-shell.js`, `shared/today-page.js`, `shared/worksheet.css` (`SPEC-003`, `SPEC-004`) | `.hebrew-shell-nav` (fixed bottom dock), `.hebrew-shell-link`, `#app-main` (bottom padding), `#today-continue-card`, `#today-mix`, `#today-review-card` |
| `33:74` | `04 — Mobile App: One-Handed Lesson Player (390×844)` | `shared/ui/plan-render.js`, `shared/ui/blocks/exercise.js`, `shared/ui/blocks/conjugation-table.js`, `shared/worksheet.css` (`SPEC-005`) | Thumb tray for choices / Continue; `[data-letter-pad]`; `[data-drill-station]`; `[data-drill-prompt]` with `data-drill-lemma`, `data-drill-tense`, `data-drill-cell` |
| `34:2` | `05 — Mobile App: SRS Flashcard & Dark Mode (390×844)` | `shared/review-page.js`, `shared/app-shell.js` (theme toggle), `shared/worksheet.css` (`SPEC-002`, `SPEC-005`) | `#review-card`, `.review-card`, rating buttons (Again/Hard/Good/Easy); `document.documentElement[data-theme="dark"]` |

Catalog frames are not separate Figma nodes in this file; Lessons / Guides / Words / Progress follow the same shell + multi-column grid rules as §1.3 and consume tokens from `30:20`.

---

## 4. Research-backed multi-tense verb workshop (`R1`–`R6`)

This section distinguishes **evidence-backed constraints** (must not regress) from **product design choices** (INIT-013 visual system). It builds on `ADR-0009` Decisions 4, 6 & 8 and `ADR-0014` (Type D, D8, D12, D13) without adding a `transform` exercise kind or allowing tools to set `verified = 1`.

### 4.1 Evidence-backed constraints

| Id | Constraint | Research | Implementation note |
|----|------------|----------|---------------------|
| — | WCAG 2.2 AA/AAA contrast floors; text ≥4.5:1; UI ≥3.0:1 | `RSCH-006` | §2 token table; `test/restyle-gate.test.js` |
| — | Primary touch targets ≥44×44 CSS px; minimum ≥24×24 | `RSCH-006` | `--size-target-primary` / `--size-target`; shell links & trays |
| — | Spatial contiguity of prompt + input + feedback | `RSCH-006` | Drill station and exercise why stay adjacent; no distant answer keys |
| — | Nikud-safe line height `1.55`–`1.7`; study words ≥1.5rem | `RSCH-006` | `.conj-table`, pointed prompts |
| — | Icon + visible label on primary nav; competence feedback without streaks/points/leaderboards | `RSCH-007` | `.hebrew-shell-link` label span; player feedback tokens only |
| — | Informational motion; `prefers-reduced-motion: reduce` global reset | `RSCH-008` | Existing global reset must remain; no scroll-jacking |
| — | Consistent Navigation (3.2.3) and Location (2.4.8) across list vs opened lesson vs editor | `RSCH-010` | Same nav landmarks; shell adapts by breakpoint, not by inventing a second IA |
| **R1** | Interleaved, **hidden-stem** recall — separate `conjugation-table` `mode: "study"` from `mode: "drill"`; do not type into a visible table while the stem stays on screen | `RSCH-003` §1.1 [12][21]; `RSCH-005` §1.3 [10][11] | `drillQueue` interleaves lemmas/tenses; `[data-drill-prompt]` carries `data-drill-lemma`, `data-drill-tense`, `data-drill-cell`; collapse empty `[data-drill-owner="0"]` sections |
| **R2** | One retry before reveal (`"נסו שוב. / try again"`); unpointed / plene tolerance via strip nikud + `accept[]` | `RSCH-003` §1.3 [6][26]; `RSCH-006` | Drill/exercise feedback states; compare helper |
| **R3** | Multi-day spaced `kind: "form"` signals into Daily Mix — do not cram 36–88 cells in one sitting as the only practice | `RSCH-003` §1.1 [1][2][12]; `ADR-0009` Decisions 4 & 8 | `#today-mix` consumes mastery; workshop drills emit `formSignal` |
| **R4** | ~¼ deliberate pattern study to ~¾ contextual sentence/output (`N7`) | `RSCH-001` §1 [1][8][9][11][12]; `RSCH-004` §1.4 [1] `N7`; `ADR-0014` D12 | Study tables + interleaved drill ≈ quarter; `exercise` (`choose`/`translate`) + `assignment` / `personal-question` ≈ three quarters; **no** `transform` kind |
| **R5** | Human-verified pointing only; tools never set `verified = 1` (`N13`/`N14`) | `RSCH-009` §1.1 [17] & §1.4 [55]–[57]; `ADR-0014` D8 & D13 | Unverified forms show “not yet checked”; `drillQueue` skips scoring until checklist tick (`SPEC-007`) |
| **R6** | Tabular clarity on desktop; 22-letter pad & tables fit `360px` with zero horizontal overflow | `RSCH-006` | `.explainer-table`, `.conj-table[data-tense="present"]` (4-col), `.conj-table[data-tense="past"|"future"]` (9-row), `.root-letter` (terracotta), `[data-letter-pad]` |

### 4.2 Product design choices (not research mandates)

These are INIT-013 / Figma product decisions. Research does not require them; they must still meet the constraints in §4.1.

| Choice | Spec owner |
|--------|------------|
| Asymmetric **Bento** grid (`7fr` / `5fr`) on Today desktop | `SPEC-004` |
| Persistent **260px** sidebar rail width | `SPEC-003` |
| **Jerusalem Stone & Tekhelet Indigo** (and Judean Night) palette hex values | `SPEC-002` (values fixed in §2) |
| Card radii `20px` / control `12px` / hero Judean Midnight treatment | `SPEC-002` / `SPEC-004` |

### 4.3 `[data-drill-prompt]` contract (multi-verb / multi-tense)

When `drillQueue` interleaves multiple verbs (`lemmaKey`) and/or tenses (`present`, `past`, `future`):

1. Mount one shared `[data-drill-station]` on the owner drill block (`isOwner()`).
2. On each item, update `[data-drill-prompt]` attributes: `data-drill-lemma`, `data-drill-tense`, `data-drill-cell`, and show the active verb + tense alongside the person/gender cue (`R1`, `REQ-009`).
3. Collapse / do not paint empty visual cards for `[data-mode="drill"][data-drill-owner="0"]` sections with no study content.
4. Preserve existing finish copy (`'סיימתם. / Done.'`) and keep `data-drill-cell` for current tests (`SPEC-005`).

---

## 5. Decisions

| Id | Decision | Rationale |
|----|----------|-----------|
| D1 | Adopt the 3-tier breakpoint table in §1 as the layout contract for `SPEC-003`–`SPEC-005`. | Resolves the “neither phone nor desktop” trap without forking the SPA into two apps (`RSCH-010`). |
| D2 | Active theme tokens are Jerusalem Stone / Judean Night (§2); the `ADR-0006` comment block and `tokens.json` remain the historical baseline for `test/design-tokens.test.js`. | Keeps both gates green without rewriting INIT-005 history (`aud-2` in SPEC-001). |
| D3 | Figma file `NJ8HfTPCQgrFty2VVQBMhY` + collection `VariableCollectionId:30:2` is the visual source of truth for tokens and the six frames in §3. | Operator design input for INIT-013. |
| D4 | Verb workshops follow `R1`–`R6` (§4); no `transform` kind; humans only set `verified`. | Continues `ADR-0014` D12 / D13; grounded in `RSCH-001`/`003`/`004`/`005`/`009`. |
| D5 | Competence-only feedback; no streaks/points/leaderboards; reduced-motion reset stays. | `RSCH-006`, `RSCH-007`, `RSCH-008`; `ADR-0010`. |

---

## Consequences

### Positive

- One citeable contract for shell, Bento, tokens, Figma map, and verb-workshop pedagogy before CSS/JS land.
- `SPEC-002` can implement tokens knowing every restyle-gate pair already clears AA.
- `SPEC-007` can seed the multi-tense workshop against explicit `R1`–`R6` / `N7` / `N13`/`N14` rules.

### Negative / trade-offs

- Desktop Bento and 260px rail are product choices; if research later favors a different chrome density, adjust layout specs without reopening token AA math unless hex values change.
- Two token “eras” remain on disk until a future initiative retires the `ADR-0006` baseline block (out of scope here).

### Follow-ups (other specs)

| Spec | Consumes this ADR |
|------|-------------------|
| INIT-013 SPEC-002 | §2 tokens, theme engine, typography vars |
| INIT-013 SPEC-003 | §1.1 shell rail / bottom dock |
| INIT-013 SPEC-004 | §1.2–§1.3 Today Bento & catalog grids |
| INIT-013 SPEC-005 | §1.4 player/SRS stage; §4 drill contract |
| INIT-013 SPEC-007 | §4 workshop / lexicon verification |
| INIT-013 SPEC-006 | Viewport matrix verifying §1 at `360`/`390`/`768`/`1280`/`1440` |

---

## Security considerations

Documentation only — no executable change in `INIT-013/SPEC-001`. Downstream specs must keep:

- Theme values allowlisted to `'light' | 'dark'` before writing `dataset.theme`.
- `escapeHtml()` / `bi()` / `textContent` for shell and page markup (no raw `innerHTML` interpolation of user or plan strings).
- Zero new external network / font URLs (`REQ-018`).
- Learner typed drill/exercise answers never leave the browser in `formSignal` payloads.

**Security review (this spec, light tier):** Critical 0 · High 0 · Medium 0 · Low 0 (markdown ADR only; no runtime surface).

---

## References

- `sdd/initiatives/INIT-013-responsive-desktop-mobile-redesign/INIT-013-responsive-desktop-mobile-redesign-initiative.md`
- `sdd/initiatives/INIT-013-responsive-desktop-mobile-redesign/specs/SPEC-001-docs-adr-0015-responsive-architecture.md`
- `sdd/initiatives/INIT-013-responsive-desktop-mobile-redesign/specs/SPEC-002-frontend-tokens-theme-and-typography.md` (hex table source)
- Figma [`NJ8HfTPCQgrFty2VVQBMhY`](https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY) · `VariableCollectionId:30:2`
- `test/restyle-gate.test.js` (19-pair `PAIRS` + luminance formula)
- `test/design-tokens.test.js` (`ADR-0006` block vs `docs/decisions/ADR-0006-assets/tokens.json`)
- [ADR-0006](ADR-0006-visual-language-and-interface-rules.md) · [ADR-0008](ADR-0008-learner-look-and-feedback.md) · [ADR-0010](ADR-0010-player-visual-language-and-competence-feedback.md) · [ADR-0014](ADR-0014-lesson-type-coverage.md)
- `sdd/research/RSCH-001-*` · `RSCH-003-*` · `RSCH-004-*` · `RSCH-005-*` · `RSCH-006-*` · `RSCH-007-*` · `RSCH-008-*` · `RSCH-009-*` · `RSCH-010-*`

## Changelog

| Date | Change |
|------|--------|
| 2026-10-17 | Initial record — `INIT-013/SPEC-001`. |
