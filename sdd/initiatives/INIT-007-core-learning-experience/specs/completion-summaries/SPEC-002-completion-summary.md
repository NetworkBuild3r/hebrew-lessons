# SPEC-002 completion summary

**Spec:** SPEC-002 · Feedback kit, tactile interactions, and audio playback states  
**Initiative:** INIT-007  
**Completed:** 2026-10-11  
**Status:** complete (orchestrator closed 2026-10-11 — 855 pass; chrome-audio-smoke ok)  
**Provenance:** `INIT-007/SPEC-002`

## What shipped

- `shared/player/feedback.js` — `HebrewFeedback` competence helpers (`showCorrect` / `showMiss` / `clear`), optional Web Audio tactile chimes (OFF by default, `localStorage` key `hebrew-sfx-enabled`), new chimes suppressed while Hebrew speech is active (not ducked), Space/`r` replay binding, `[data-sfx-toggle]` persistent toggle hooks.
- `shared/player/speech.js` — playback state (`loading` / `playing` / `idle` / `error`), `is-loading` / `is-playing` classes on the associated control, `playClip` alias, `replayLast`, `hebrew-speech-playback` events; existing `play` / `speak` callers unchanged.
- `shared/player/core.js` — `playModel` passes the speak button into `playClip`.
- `shared/worksheet.css` — additive tactile press (`scale(0.98)`), focus rings, audio pulse, `.correction` / feedback panels, reduced-motion zeros. No `.player-top` / `.action-bar` / mobile-dock redesign (SPEC-003).
- Load path: `shared/worksheet.js` and `shared/app-routes.js` load `feedback.js` after `speech.js`.
- `test/player-feedback.test.js` — AC coverage for playback classes, miss correction, SFX defaults, CSS contracts, script wiring.

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass | `playClip` toggles `is-playing`; cleared on `ended` / `error` (`test/player-feedback.test.js`) |
| ac-2 | Pass | `showMiss` renders `.correction` + pointed Hebrew + `data-action` retry |
| ac-3 | Pass | CSS active `scale(0.98)` + 3px focus rings on `.btn`, `.choice-card`, `.word-match-card`, `.bank-word` |
| ac-4 | Pass | SFX default off; chimes refuse while `setSpeechActive(true)` |
| ac-5 | Pass | `prefers-reduced-motion: reduce` zeros `--motion-fast`, transforms, animations |

## Tests

- `npm test` — 855 pass, 0 fail, exit 0
- `node test/chrome-audio-smoke.js` — ok (clips play, `ttsCount: 0`)
- `node --test test/player-feedback.test.js test/player-speech-clips.test.js` — 8 pass

## Conflicts with SPEC-003

None intentional. SPEC-002 owns feedback / tactile / audio-state / correction CSS only. SPEC-003 owns `.player-top`, `.action-bar`, and the mobile dock — those were not redesigned here. Both specs touch `shared/worksheet.css` and `shared/player/core.js` additively; merge carefully if parallel.

## Follow-ups

- Activity templates (SPEC-004 / SPEC-005) should call `HebrewFeedback.showMiss` / `showCorrect` at check time.
- Shell may surface a visible `[data-sfx-toggle]` control using the existing binder.

## Eval

See `SPEC-002-eval-report.md`.
