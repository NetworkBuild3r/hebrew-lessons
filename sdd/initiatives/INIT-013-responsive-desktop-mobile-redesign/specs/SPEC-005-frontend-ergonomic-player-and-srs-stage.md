# SPEC-005 · frontend: Ergonomic study stage — Desktop split-context Lesson Player, Mobile thumb-docked exercise tray, and SRS Review

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-013
title: "Ergonomic study stage: Desktop split-context Lesson Player, multi-verb/multi-tense conjugation drill stage, Mobile thumb-docked exercise tray, and SRS Review"
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

**Context:** When a learner opens a lesson (`shared/ui/plan-render.js`, `shared/ui/blocks/exercise.js`, `shared/ui/blocks/assignment.js`, `shared/ui/blocks/conjugation-table.js`, `shared/ui/blocks/explainer.js`, or legacy worksheet activities in `shared/activities/*`) or enters the SRS Review queue (`shared/review-page.js`), the interface currently renders inside the same narrow single column with stacked option buttons. Furthermore, when a `verb-workshop` lesson includes multiple `conjugation-table` blocks in `mode: "drill"` across multiple verbs (`lemmaKey`) and tenses (`present`, `past`, `future`), `drillQueue()` in `shared/ui/blocks/conjugation-table.js` interleaves them into one shared `[data-drill-station]`, **but** `showItem()` only sets `prompt.textContent = item.label` (`אני` / `SM`) without showing which verb (`item.lemma`) or tense (`item.tense`) the prompt belongs to, and non-owner `[data-mode="drill"][data-drill-owner="0"]` sections render as empty headers.

This spec implements Pillar 4 & Pillar 5 of `INIT-013` (`32:2` Desktop Lesson Player, `33:74` Mobile One-Handed Lesson Player, and `34:2` Mobile Dark-Mode SRS Flashcard in `NJ8HfTPCQgrFty2VVQBMhY`, plus `R1`, `R2`, `R6` for multi-tense verb conjugation workshops): optimizing the active study surfaces for **widescreen desktop focus (`≥1024px`)** and **one-handed mobile thumb ergonomics (`<768px`)**.

**Scope (in):**

1. **Desktop Lesson Player & Exercise Stage (`@media (min-width: 1024px)`, Figma `32:2`):**
   - Widens the plan player and worksheet container on desktop (`max-width: 68rem` / `1088px`) with elevated card surfaces (`background: var(--color-surface); border-radius: 24px; padding: 2rem 2.5rem; border: 1px solid var(--color-border-subtle);`).
   - **2×2 Responsive Choice Grid:** Multiple-choice (`choose`) and `true-false` exercise option lists (`.exercise-choices`, `.choice-list`, `.quiz-options`) arrange in a 2-column grid (`grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.85rem;`) on desktop so learners can scan all options at eye level without vertical scrolling.
   - **Conjugation Table & Grammar Blocks (`shared/ui/blocks/conjugation-table.js`, `shared/ui/blocks/explainer.js`):**
     - Clean tabular styling for `.explainer-table` (3-binyan root comparisons), `.conj-table[data-tense="present"]` (4-column `SM` / `SF` / `PM` / `PF` grid), and `.conj-table[data-tense="past"]` / `.conj-table[data-tense="future"]` (9-row `אני`–`הם/הן` table) with generous pointed-Hebrew font sizing (`>= 1.35rem`, `line-height: 1.6`), Warm Terracotta root-letter highlighting (`.root-letter`), and distinct tense/mode kicker badges.
     - **Interleaved Multi-Verb & Multi-Tense Drill Cues (`REQ-009`, `R1`):** In `shared/ui/blocks/conjugation-table.js`, update `showItem()` so `[data-drill-prompt]` sets `data-drill-cell`, `data-drill-lemma`, and `data-drill-tense` on every active prompt, and when the drill queue spans multiple lemmas or tenses (or always in a clean bilingual format), displays the active verb lemma (`item.lemma`), tense label (`הווה / present`, `עבר / past`, `עתיד / future`), and person/gender label (`item.label`) while preserving `'סיימתם. / Done.'` when the queue completes.
     - **Collapse Empty Non-Owner Drill Sections (`REQ-009`):** Non-owner drill sections (`section[data-block-type="conjugation-table"][data-mode="drill"][data-drill-owner="0"]`) that contain no study table, sentences, questions, or extra cells are hidden via CSS/DOM (`hidden` or compact zero-chrome) so multi-verb/multi-tense drills present a single cohesive `[data-drill-station]` card.
   - **Competence Feedback Banner (`.exercise-why`, `.feedback-banner`, `[data-drill-feedback]`):** Styled with Galilee Sage (`--color-correct-soft` / `--color-correct-ink`) for correct answers and Warm Terracotta (`--color-miss-soft` / `--color-miss-ink`) for retries/reveals, matching `RSCH-006` and `ADR-0010`.
2. **Mobile One-Handed Lesson Player & Hebrew Letter Pad (`@media (max-width: 767px)`, Figma `33:74`):**
   - Keeps the Hebrew prompt/sentence card elevated in the upper eye-level zone (`font-size: 1.5rem`, `line-height: 1.65`) while anchoring the interactive answer options, `[data-drill-input]`, the 22-letter Hebrew on-screen pad (`[data-letter-pad]` arranged in a responsive grid that never overflows `360px` with `>= 44px` primary check/next buttons and `>= 24px` letter keys), inline `why` / `[data-drill-feedback]` toast, and full-width `Continue` / `Check` buttons into a cohesive bottom thumb-zone tray (`border-radius: 22px; padding: 1rem; background: var(--color-surface);`).
   - Ensures `.conj-table` (`present` 4-column and `past`/`future` 9-row) and `.explainer-table` fit within `360px` with zero horizontal page overflow (`scrollWidth <= innerWidth`).
3. **SRS Flashcard Review Stage (`shared/review-page.js` + `shared/worksheet.css`, Figma `34:2`):**
   - **Tactile Flashcard Surface:** Elevated card (`border-radius: 24px; padding: 2rem 1.5rem;`) with memory-stage pill badge, pointed Hebrew lemma at `2.5rem–3rem` (`40px–48px`, `Frank Ruhl Libre`), audio trigger, and inset example sentence box.
   - **Thumb-Docked 4-Button Rating Bar (`Again`, `Hard`, `Good`, `Easy`):**
     - On mobile (`<768px`), the 4 rating buttons sit in a 4-up or 2×2 thumb-zone dock at the bottom of the review card (`min-height: 52px` per button) with semantic border/text accents (Again, Hard, Good, Easy) that pass WCAG 2.2 AA contrast (`>= 4.5:1`) in both Light and Dark modes.

**Scope (out):** Schema changes, FSRS scheduling math, or Exercise/Assignment scoring logic (already completed in `INIT-012`); seeding the 88-form multi-binyan verb workshop (`SPEC-007`).

## Understood

- [ ] und-1: `test/plan-render-exercise.test.js`, `test/plan-render-exercise-typed.test.js`, `test/plan-render-assignment.test.js`, `test/render-conjugation.test.js`, `test/chrome-exercise-smoke.js`, `test/chrome-verb-workshop-smoke.js`, and `test/chrome-lesson-types-smoke.js` assert on the DOM structure and classes of `shared/ui/blocks/*`. Styling and layout enhancements must preserve all existing classes, `data-*` attributes, button elements, and live regions.
- [ ] und-2: `test/chrome-verb-workshop-smoke.js` checks `prompt.getAttribute('data-drill-cell')` and `prompt.textContent.indexOf('Done') >= 0`. Enhancing `[data-drill-prompt]` in `shared/ui/blocks/conjugation-table.js` to also set `data-drill-lemma` and `data-drill-tense` and show the active lemma/tense cue must preserve `data-drill-cell` and `'סיימתם. / Done.'`.
- [ ] und-3: At `360px` wide (`test/plan-render-exercise.test.js` `ac-8` and `test/chrome-exercise-smoke.js`), nothing inside the player, conjugation tables, letter pad, or exercise blocks may scroll the page sideways (`scrollWidth <= innerWidth`).

## Acceptance Criteria

- [ ] ac-1: At `1280×900` and `1440×960` (Desktop), `choose` and `true-false` exercise choices render in a 2-column CSS Grid, and every choice button has `min-height: 44px` and a visible `:focus-visible` ring (`REQ-006`, `REQ-007`).
- [ ] ac-2: At `360×800` and `390×844` (Mobile), exercise prompt cards, option buttons, unscramble word bank/slots, translate/dictation inputs, conjugation study tables (`present`, `past`, `future`), the 22-letter drill pad (`[data-letter-pad]`), and assignment blocks fit within the viewport with zero horizontal overflow (`scrollWidth <= innerWidth`) and `>= 44px` primary button heights (`REQ-006`, `REQ-007`, `REQ-009`).
- [ ] ac-3: The SRS Review card (`shared/review-page.js`) renders the pointed Hebrew lemma at `>= 36px` (`2.25rem`), and its rating buttons (`Again`, `Hard`, `Good`, `Easy`) have `height >= 44px` and pass `>= 4.5:1` text contrast in both Light and Dark themes (`REQ-005`, `REQ-006`, `REQ-007`).
- [ ] ac-4: When `conjugation-table` (`mode: "drill"`) interleaves cells across multiple verbs (`lemmaKey`) and/or tenses (`present`, `past`, `future`), `[data-drill-prompt]` exposes `data-drill-lemma`, `data-drill-tense`, and `data-drill-cell` and displays the active verb lemma and tense alongside the person/gender prompt (`REQ-009`).
- [ ] ac-5: All existing player, conjugation, and exercise tests (`test/plan-render-exercise.test.js`, `test/plan-render-exercise-typed.test.js`, `test/plan-render-assignment.test.js`, `test/render-conjugation.test.js`, `test/player-ui-a11y.test.js`) pass (`REQ-007`, `REQ-009`).

## Assumptions

| id | item | interpretation | tone | resolved |
|----|------|----------------|------|----------|
| aud-1 | Long option text in 2×2 grid | On mobile (`<600px`), `choose` options with long Hebrew sentences stack 1-per-row (`grid-template-columns: 1fr`) inside the thumb tray so pointed Hebrew never wraps into cramped 2-word lines; short options or `true-false` use 2 columns | resolved | yes |

## Deliverables

- [ ] `shared/worksheet.css` (responsive player stage, 2×2 desktop choice grid, mobile thumb-docked exercise tray, conjugation study/drill table & 22-letter pad polish, and SRS flashcard + 4-button rating bar styles in Light & Dark mode)
- [ ] `shared/ui/blocks/conjugation-table.js` (active `data-drill-lemma` and `data-drill-tense` prompt cues during multi-verb/multi-tense interleaved drills, and collapsing empty non-owner `[data-drill-owner="0"]` sections)
- [ ] `shared/ui/blocks/exercise.js` / `shared/review-page.js` (only non-breaking CSS class hooks if needed, preserving all existing hooks and tests)
- [ ] `test/render-conjugation.test.js` (unit test assertions for multi-verb/multi-tense drill prompt cues)

## Technical Requirements

- [ ] Do not alter any answer-checking, retry-before-reveal, or live-region announcement logic in `shared/ui/blocks/exercise.js`.
- [ ] Ensure all animations/transitions respect `@media (prefers-reduced-motion: reduce)` (`test/restyle-gate.test.js` line 274).

## Gates & Controls

- **Autonomy level:** `auto_tests`.
- **Gate actions triggered:** none.
- **Blocking checks:** `node --test test/plan-render-exercise.test.js test/plan-render-exercise-typed.test.js test/plan-render-assignment.test.js test/render-conjugation.test.js test/player-ui-a11y.test.js`.
- **Rollback plan:** Revert player and review CSS rules in `shared/worksheet.css`.

## Security

- **Sensitivity:** `standard`.
- **Security acceptance criteria:**
  - [ ] All authored text in exercises, conjugation tables, assignments, and review cards remains escaped via `escapeHtml()` (`REQ-014` from `INIT-012` preserved).
  - [ ] Zero network calls made when answering plan exercises.

## Verification Strategy

- **Claim:** The Lesson Player and SRS Review surfaces adapt cleanly between a wide multi-column desktop stage and a one-handed mobile thumb tray while passing all exercise, conjugation, and accessibility tests.
- **Check + executor:** `mechanized` — `node --test test/plan-render-exercise.test.js test/plan-render-exercise-typed.test.js test/plan-render-assignment.test.js test/render-conjugation.test.js test/player-ui-a11y.test.js`.
- **Pass condition:** 100% pass rate across all five test files and zero horizontal overflow at `360px`.

## Integration Points

- Combines with `SPEC-003` and `SPEC-004` for end-to-end verification in `SPEC-006`.
- **Provenance tag:** `INIT-013/SPEC-005`.
