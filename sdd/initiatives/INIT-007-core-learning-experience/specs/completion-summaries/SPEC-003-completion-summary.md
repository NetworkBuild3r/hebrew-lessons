# SPEC-003 completion summary

**Spec:** SPEC-003 · Player shell, step transitions, and thumb-zone mobile ergonomics  
**Initiative:** INIT-007  
**Completed:** 2026-10-11  
**Status:** complete (orchestrator closed 2026-10-11 — npm test + chrome-navigation-smoke PASS)  
**Provenance:** `INIT-007/SPEC-003`

## What shipped

- `shared/player/core.js`: cleaner player chrome (exit/path control, `שלב X מתוך Y` indicator, global `.btn-vowels`), sticky progress without points/streaks, step enter transition hooks, scroll-to-top on step advance, polite `aria-live` step announcements, quiet re-render path for vowel toggles.
- `shared/worksheet.css`: thumb-zone `.action-bar` dock (glassmorphic surface, `env(safe-area-inset-bottom)`, primary ≥48px on ≤599px), redesigned `.player-top` / `.progress-track`, `--motion-step` (120ms) with `prefers-reduced-motion` disable, focus `scroll-margin` so the dock does not obscure focused controls. Preserved `.path-btn` / button ink (`color: inherit` / `var(--ink)`).
- Home remains in the app shell above the player (ADR-0007); no `.player-top .back-link`.

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass | Fixed `.action-bar` with safe-area padding; `.action-bar-inner .btn` min-height `var(--target-primary, 48px)` especially under `@media (max-width: 599px)`. |
| ac-2 | Pass | Progress fill width from step/completed position only; no points/streaks UI in chrome. |
| ac-3 | Pass | Shell `.btn-vowels` toggles `vowelsOn`, quiet-re-renders the current step so `[data-vowels]` / story nikud stay in sync. |
| ac-4 | Pass | `scrollPlayerToTop()` + `#player-step-announce` on step/path changes (skipped on quiet re-renders). |
| ac-5 | Pass | `.step-screen.step-enter` uses `--motion-step: 0.12s ease-out`; reduced to `0s` / `animation: none` under `prefers-reduced-motion`. |

## Understood

- und-1: Dock padding + `scroll-margin-bottom` / `scroll-padding-bottom` on `.player.has-action`.
- und-2: Global vowels pill always in `.player-top`; activity toggles share `vowelsOn` via core handler.
- und-3: Primary controls ≥44px; dock primary ≥48px on mobile.

## Tests

- `npm test` — pass (exit 0)
- `node test/chrome-navigation-smoke.js` — pass

## Conflicts with SPEC-002

- SPEC-002 (feedback kit / tactile / audio-pulse / correction) is still `in_progress` and did not own `.player-top`, `.action-bar`, `.progress-track`, or step-enter classes.
- SPEC-003 touched only those shell regions plus `--motion-step` / `--target-primary` on `:root`. Existing brand-polish tactile `.btn:active` rules were left intact.
- If SPEC-002 later adds player dock styles, merge by keeping SPEC-003 dock geometry/safe-area and SPEC-002 feedback/audio selectors.

## Follow-ups

- SPEC-004 / SPEC-005 restyle activity interiors; may remove duplicate in-activity vowels controls once the shell pill is universal.
- SPEC-007 can add targeted a11y/smokes for dock height and reduced-motion step enter.

## Mobile notes

- ≤599px: dock padding increases; English parentheticals on exit/vowels hide to save width; primary action stays ≥48px in thumb zone.
- Safe-area inset bottom prevents home-indicator clipping.
- App-shell Home/Lessons nav unchanged above the player.
