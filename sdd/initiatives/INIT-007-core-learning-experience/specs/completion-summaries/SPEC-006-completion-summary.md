# SPEC-006 completion summary

**Spec:** SPEC-006 · Review flashcards and session completion mastery moment  
**Initiative:** INIT-007  
**Completed:** 2026-10-11  
**Status:** complete (orchestrator closed 2026-10-11 — 855 pass)  
**Provenance:** `INIT-007/SPEC-006`

## What shipped

- `shared/review-page.js`: calm active-recall cards (Card N of M progress), reveal transition, audio on Hebrew reveal, FSRS four-grade buttons kept and restyled (rose Again/Hard · emerald Good/Easy, `data-signal` miss/pass), caught-up empty state with next-due hint, mastery summary with Back to Today + Practice again.
- `shared/player/core.js`: round-complete mastery ceremony (words met, steps done, next-review hint); primary Back to Today (`/`); secondary Practice again. Path list still shown when the round is not finished.
- `shared/worksheet.css`: SPEC-006 block for review cards, grade contrast ≥48px, reveal motion, catch-up/summary, and `.mastery-ceremony` / `.complete-ceremony`.

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass | Answer node only after reveal; `#review-show-answer` / `#review-answer`; `data-signal` on grade buttons; four `data-grade` buttons retained. |
| ac-2 | Pass | `.review-grade` min-height/min-width 48px; rose vs emerald families. |
| ac-3 | Pass | `#review-finished` summary with `#review-count-reviewed`, grade counts, `#review-next-due`. |
| ac-4 | Pass | `#mastery-ceremony` when all steps complete; `#mastery-words-met`; `#mastery-back-today` → `/`. |
| ac-5 | Pass | No streak/points/xp/leaderboard/confetti in review or ceremony markup/source. |

## Understood

- und-1: Competence only — no gamification copy or confetti.
- und-2: Words practiced / met + next due schedule on both review and lesson complete.
- und-3: Preserved `#review-due-count` / `#review-new-count` (Today), `data-signal`, `data-grade`, `#review-card`, four-grade IDs.

## Tests

- `npm test` — pass (855 / 0 fail, exit 0)
- `node --test test/review-page.test.js test/progress-view.test.js` — pass

## Follow-ups

- SPEC-007 can add Chrome smokes for mastery ceremony DOM and 48px grade targets.
- Optional: wire review empty next-due from a dedicated API field if library `due_in_days` proves sparse.
