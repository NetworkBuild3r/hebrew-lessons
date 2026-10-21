# SPEC-001 · docs: ADR-0015, responsive desktop & mobile architecture and Jerusalem Stone & Indigo tokens

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-013
title: "ADR-0015: Responsive desktop & mobile architecture, Bento grid rules, Jerusalem Stone & Indigo tokens, and multi-tense verb workshop rules"
domain: docs
status: complete
primary_prompt: .claude/agents/docs-agent/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: plan_only
gate_actions: []
reversibility_class: branch
security_review: light
estimated_effort: 3 hours
blocked_by: None
blocks: [SPEC-002, SPEC-007]
```

## Description

**Context:** Across `INIT-005` (`ADR-0006`, `ADR-0008`) and `INIT-007` (`ADR-0010`), the app kept a single `40rem` (`640px`) centered column (`.wrap`) and a top header bar whose navigation links (`.hebrew-shell-nav`) wrap into multiple rows at every viewport width. On desktop monitors (`1280px–1920px`), ~65% of the screen is empty while cards stack vertically; on phones (`360px–430px`), the wrapped top navigation bar consumes ~150px at the top of the screen and cards use desktop padding. Additionally, three eras of CSS tokens accumulated in `shared/worksheet.css` and dark mode was pinned off.

Before `SPEC-002` through `SPEC-005` touch `shared/worksheet.css` and `shared/app-shell.js`, one Architecture Decision Record — **`ADR-0015`** (the highest existing ADR in `docs/decisions/` is `ADR-0014`) — defines the responsive layout contract, the Figma frame mapping (`NJ8HfTPCQgrFty2VVQBMhY`), the Light/Dark color token table, and the compatibility rules that keep `ADR-0006` and `ADR-0010` test gates green.

**Scope (in):** `docs/decisions/ADR-0015-responsive-desktop-mobile-architecture.md` recording:

1. **The 3-Tier Breakpoint Contract:**
   - **Mobile (`< 768px`, tested down to `320px`/`360px`):** Compact `52px` top context bar + fixed thumb-zone bottom navigation bar + single-column edge-to-edge feed (`16px` inline padding) + thumb-docked exercise/SRS action tray.
   - **Tablet (`768px – 1023px`):** Two-column adaptive cards (`max-width: 960px`) with compact top navigation.
   - **Desktop (`≥ 1024px`, widescreen `≥ 1280px`):** Persistent `260px` vertical sidebar navigation rail + `1200px` max-width main stage with asymmetric 12-column Bento grids (`7fr / 5fr` on Today; multi-column card grids on Lessons, Guides, Words, Progress; split-context stage on Lesson Player).
2. **The Jerusalem Stone & Tekhelet Indigo Token Table (Light & Dark Modes):**
   - Exact hex values and pre-computed WCAG 2.2 AA contrast ratios for all 19 token pairs enforced by `test/restyle-gate.test.js` (`--paper`, `--color-surface`, `--color-surface-soft`, `--color-text`, `--color-muted`, `--muted`, `--color-primary`, `--color-primary-hover`, `--color-primary-soft`, `--color-primary-ink`, `--color-correct-ink`, `--color-correct-soft`, `--color-miss-ink`, `--color-miss-soft`, `--color-line`), plus the Hero card tokens (`--color-hero-bg: #141B2D`, `--color-hero-text: #FFFFFF`, `--color-accent-terracotta`, `--color-accent-sage`).
3. **Preservation of `ADR-0006` & `ADR-0010` Invariants:**
   - All 70 `data-*` attributes and 50 CSS classes remain intact.
   - Primary interactive targets remain `≥ 44×44 CSS px` (`--size-target-primary: 44px`), minimum targets `≥ 24×24 CSS px` (`--size-target: 24px`).
   - `prefers-reduced-motion: reduce` global reset and competence-only feedback (no streaks, points, or leaderboards per `RSCH-006` and `RSCH-007`) remain mandatory.
4. **Figma Traceability Table:** Maps Figma nodes `30:20`, `31:2`, `32:2`, `33:2`, `33:74`, and `34:2` in file `NJ8HfTPCQgrFty2VVQBMhY` to their production files (`shared/app-shell.js`, `shared/today-page.js`, `shared/ui/plan-render.js`, `shared/review-page.js`, `shared/worksheet.css`).
5. **Research-Backed Multi-Tense Verb Workshop & Interleaved Drill Architecture (`R1`–`R6`):**
   - Documents why verb conjugation practice separates pointed study tables (`conjugation-table` `mode: "study"` across `present`, `past`, and `future`) from interleaved, hidden-stem recall (`conjugation-table` `mode: "drill"`, `RSCH-003` §1.1 [12][21], `RSCH-005` §1.3 [10][11]) rather than inline visible-table cell inputs.
   - Specifies the `[data-drill-prompt]` contract (`data-drill-lemma`, `data-drill-tense`, `data-drill-cell`) when `drillQueue` interleaves multiple verbs (`lemmaKey`) and tenses (`present`, `past`, `future`), the collapsing of empty non-owner drill sections (`[data-drill-owner="0"]`), the ~¼ deliberate pattern study to ~¾ contextual sentence (`exercise` `choose`/`translate`) + output (`assignment`, `personal-question`) balance (`RSCH-001`, `RSCH-004` `N7`), and the human verification checklist gate (`RSCH-009`, `ADR-0014` D13, `N13`/`N14`).

**Scope (out):** Production CSS, JS, or lexicon seed changes (covered by `SPEC-002` through `SPEC-005` and `SPEC-007`).

## Understood

- [x] und-1: `ADR-0015` amends the layout and palette rules of `ADR-0006`, `ADR-0008`, and `ADR-0010` while keeping their 70 `data-*` hooks, 50 CSS classes, and WCAG 2.2 AA/AAA accessibility floors intact.
- [x] und-2: `test/design-tokens.test.js` tests the `/* === ADR-0006 design tokens === */` block against `docs/decisions/ADR-0006-assets/tokens.json`, whereas `test/restyle-gate.test.js` tests the final resolved `:root` Light and Dark tokens. `ADR-0015` documents how both tests stay green.
- [x] und-3: `ADR-0015` builds on `ADR-0009` (Decisions 4, 6 & 8) and `ADR-0014` (Type D `verb-workshop`, `exercise`, `assignment`, `tense: "future"`, D8, D12, D13) without adding a `transform` exercise kind or allowing tools to set `verified = 1`.

## Acceptance Criteria

- [x] ac-1: `docs/decisions/ADR-0015-responsive-desktop-mobile-architecture.md` exists and defines the Mobile (`<768px`), Tablet (`768px–1023px`), and Desktop (`≥1024px`) layout rules for the shell, Today workspace, catalog views, and Lesson/SRS player (`REQ-001`, `REQ-002`, `REQ-003`, `REQ-004`, `REQ-006`).
- [x] ac-2: `ADR-0015` includes a complete Light and Dark hex token table where every one of the 19 pairs tested by `test/restyle-gate.test.js` meets its required contrast ratio (`≥4.5:1` for text pairs, `≥3.0:1` for UI pairs) (`REQ-005`, `REQ-007`).
- [x] ac-3: `ADR-0015` maps each of the 6 Figma frames (`30:20`, `31:2`, `32:2`, `33:2`, `33:74`, `34:2` in `NJ8HfTPCQgrFty2VVQBMhY`) to the exact files and DOM selectors in `shared/`.
- [x] ac-4: `ADR-0015` explicitly cites `RSCH-001`, `RSCH-003`, `RSCH-004`, `RSCH-005`, `RSCH-006`, `RSCH-007`, `RSCH-008`, `RSCH-009`, and `RSCH-010`, distinguishing evidence-backed constraints (WCAG 2.2 AA/AAA contrast, ≥44px touch targets, spatial contiguity, nikud line height, interleaved hidden-stem recall, one retry before reveal, multi-day spaced `form` signals, human-verified pointing) from product design choices (Bento grid, sidebar rail width, Jerusalem Stone & Indigo palette) (`REQ-009`, `REQ-010`, `REQ-011`).

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | ADR number | `ADR-0015` (highest existing ADR in `docs/decisions/` is `ADR-0014`) | resolved | yes |
| aud-2 | `ADR-0006` token block | Keep `docs/decisions/ADR-0006-assets/tokens.json` and the `ADR-0006` comment block in `shared/worksheet.css` intact so `test/design-tokens.test.js` continues to verify the historical baseline, while `ADR-0015` governs the active `:root` theme rules | resolved | yes |

## Deliverables

- [x] `docs/decisions/ADR-0015-responsive-desktop-mobile-architecture.md`

## Verification Strategy

- **Claim:** `ADR-0015` defines a complete, conflict-free responsive architecture and Light/Dark token table whose contrast ratios mathematically satisfy `test/restyle-gate.test.js`.
- **Check + executor:** `mechanized` — run a Node one-liner using `test/restyle-gate.test.js`'s relative-luminance formula against every Light and Dark hex pair listed in `ADR-0015`. `human` — operator review of `ADR-0015` against the Figma boards in `NJ8HfTPCQgrFty2VVQBMhY`.
- **Pass condition:** All 19 Light and 19 Dark token pairs in `ADR-0015` compute to `≥4.5:1` (text) and `≥3.0:1` (UI), and all four acceptance criteria (`ac-1` through `ac-4`) are present in the document.

## Gates

No production code, no dependencies, no secrets.

## Integration Points

- Unblocks `SPEC-002`, `SPEC-003`, `SPEC-004`, and `SPEC-005`.
- **Provenance tag:** `INIT-013/SPEC-001`.
