# Pull Request: INIT-013 — Responsive Desktop & Mobile Redesign, Jerusalem Stone & Judean Night Themes, and Multi-Tense Verb Workshop

## Summary

Transforms **דֶּרֶךְ עִבְרִית (Hebrew Way)** from a single-column `640px` layout with wrapped top navigation pills into an adaptive desktop (`≥1024px`) and one-handed mobile (`<768px`) web application implementing the **Jerusalem Stone (`Light`) & Judean Night (`Dark`)** design system (`ADR-0015`, Figma `NJ8HfTPCQgrFty2VVQBMhY`), and adds the research-backed (`R1`–`R6`) multi-binyan, multi-tense (`present`, `past`, `future`) verb conjugation workshop (`scripts/seed-verb-tense-workshop.js`, 88 inflected forms + 4 lemmas) and interleaved hidden-stem recall cues (`shared/ui/blocks/conjugation-table.js`).

## Work Performed (by Spec)

- **`INIT-013/SPEC-001` (docs):** Authored `docs/decisions/ADR-0015-responsive-desktop-mobile-architecture.md` specifying the 3-tier breakpoints (`<768px`, `768px–1023px`, `≥1024px`), RTL/LTR adaptive shell, Bento grid rules, 38 Light/Dark token contrast pairs (`19 Light + 19 Dark`), and research-backed (`R1`–`R6`) multi-tense verb workshop architecture.
- **`INIT-013/SPEC-002` (frontend):** Implemented the Jerusalem Stone (`Light`) and Judean Night (`Dark`) `:root` / `:root[data-theme="dark"]` token system in `shared/worksheet.css` and added `normalizeTheme`, `getStoredTheme`, `applyTheme`, `toggleTheme`, and the bilingual `[data-theme-toggle]` button in `shared/app-shell.js`.
- **`INIT-013/SPEC-007` (data):** Added `scripts/seed-verb-tense-workshop.js`, `SPEC-007-forms-checklist.md`, and `test/verb-tense-workshop.test.js`, seeding `לְהַחְלִיט` (`hifil`), `לִבְחוֹר` (`paal`), `לְהַאֲמִין` (`hifil`), and `לְנַצֵּחַ` (`piel`) across `present` (16), `past` (36), and `future` (36) = 88 forms (`verified = 0` by default) and the 5-stage `schemaVersion: "1.2.0"` `verb-workshop` lesson (`decide-choose-believe-win`).
- **`INIT-013/SPEC-003` (frontend):** Implemented the Desktop (`≥1024px`) persistent `260px` sidebar navigation rail (`body.has-hebrew-shell` CSS Grid with RTL/LTR mirroring) and Mobile (`<768px`) compact top header + fixed bottom tab dock (`.hebrew-shell-nav`, `min-height: 44px` links, `scroll-padding-bottom: 6.5rem`) across `shared/worksheet.css`, `shared/app-shell.js`, `shared/layout.js`, and `login/index.html`.
- **`INIT-013/SPEC-004` (frontend):** Implemented the Desktop `7fr / 5fr` 2-column Bento workspace for `Today` (`.today-list[data-today-state="ready"]`), Judean Midnight Hero cards (`#today-continue-card`, `#today-next-card`), multi-column grids for `Lessons`, `Words`, `Guides`, and `Progress`, and Mobile compact single-column feed with full-width CTAs (`min-height: 46px`).
- **`INIT-013/SPEC-005` (frontend):** Implemented the Desktop `68rem` Lesson Player stage with 2×2 choice grids, enhanced `shared/ui/blocks/conjugation-table.js` so `showItem()` exposes `data-drill-lemma`, `data-drill-tense`, and `data-drill-cell` and displays `lemma · tense · label` during interleaved multi-verb/multi-tense drills while collapsing empty `[data-drill-owner="0"]` sections, and styled the Mobile 22-letter pad and SRS Flashcard Review (`>= 36px` pointed Hebrew lemma + `>= 48px` rating buttons).
- **`INIT-013/SPEC-006` (testing):** Added `test/responsive-layout.test.js`, extended `test/chrome-restyle-a11y-smoke.js` with `1440×960` Desktop rail/Bento and `390×844` Mobile dock bounding-box assertions, and refreshed 18 screenshots under `spikes/restyle-shots/`.

## Tests & Smoke Verification

- **Full Node unit test suite (`npm test`):** **All tests pass (`0` failures)**, including `test/responsive-layout.test.js`, `test/verb-tense-workshop.test.js`, `test/render-conjugation.test.js`, `test/restyle-gate.test.js`, `test/design-tokens.test.js`, `test/player-ui-a11y.test.js`, and `test/app-shell.test.js`.
- **Real headless Chrome smoke suites executed:**
  - `node test/chrome-restyle-a11y-smoke.js` -> `PASS` (`360×800`, `390×844`, `1280×900`, `1440×960` in Light & Dark themes)
  - `node test/chrome-exercise-smoke.js` -> `ok`
  - `node test/chrome-verb-workshop-smoke.js` -> `PASS`
  - `node test/chrome-lesson-types-smoke.js` -> `PASS`
  - `node test/chrome-restyle-screenshots-smoke.js` -> `PASS`

## Eval & Security Reports

- Per-spec independent AC verification eval reports (`SPEC-001-eval-report.md` through `SPEC-007-eval-report.md`): **all-PASS (7/7)**.
- Initiative-wide Security Review (`INIT-013-security-review.md`, `SECSCAN-013`, `INIT-013/SECURITY-REVIEW`): **PASS** (0 Critical, 0 High, 0 Medium).
- Architecture Diagrams: `sdd/initiatives/INIT-013-responsive-desktop-mobile-redesign/design/INIT-013-responsive-desktop-mobile-redesign-architecture.md` (`INIT-013/DESIGN-DIAGRAMS`).
