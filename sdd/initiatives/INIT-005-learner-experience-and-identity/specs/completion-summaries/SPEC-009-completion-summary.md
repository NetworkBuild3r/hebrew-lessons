# SPEC-009 completion summary

**Spec:** SPEC-009 · Feedback kit and route transition  
**Initiative:** INIT-005  
**Completed:** 2026-10-12  
**Status:** in_review  
**Provenance:** `INIT-005/SPEC-009`

## Deviation: module path

SPEC-009 assumed `shared/ui/feedback.js`. The kit already exists at `shared/player/feedback.js` (INIT-007/SPEC-002) and is loaded by the player and the routes that need it. It was not moved. The assumptions ledger row aud-1 now says so.

## What shipped

Most of the kit was in place. This pass checked it against ac-1..3, added the one missing piece, and wrote the tests.

- **Already satisfied.** `showCorrect`, `showMiss` and `clear`; escaped markup; a wrong answer shows the correct pointed form under a "The correct form" label with an optional highlight and one retry button; sound off by default (`hebrew-sfx-enabled`); chimes skipped while a word is loading or playing; Space and `r` replay; audio-playing classes; reduced-motion overrides for the kit surfaces.
- **Gap closed: no visible sound control (REQ-014).** `shared/player/core.js` now renders `#btn-sfx` in `.player-top-end`: a `<button>` with `data-sfx-toggle`, `aria-pressed`, a Hebrew label with English gloss ("צלילים (Sounds)"), and a Hebrew title. It is painted from stored state when the player builds its markup. The existing document click handler in `feedback.js` does the toggling. The shell has no sound control because no sound plays outside the player.
- **CSS.** The delimited block at the end of `shared/worksheet.css` holds the control's styles: 44 px target, state shown by `aria-pressed`, colour, and a ✓/✕ mark (not colour alone), English gloss hidden under 600 px. The same block holds the route transition. The "Light by default" and Words blocks were not touched.

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass | Kit exists. Reduced motion: every kit surface and `#app-main.route-enter` appear in reduce blocks (`test/feedback-kit.test.js`); the module starts no timers or animations; nothing loops forever outside a reduce override; Chrome checks the transition with emulated reduced motion. |
| ac-2 | Pass | Default off, and no `AudioContext` is created while off. Playing a clip is unaffected by `showCorrect`, `showMiss` or `chime`, with sound off or on; chimes use Web Audio and never the word `<audio>` element. With sound on, chimes are skipped while a word plays. The control exists in the player header and works in Chrome (`aria-pressed` false, then true and stored `1`, then false; 140 by 44 px). |
| ac-3 | Pass | Markup includes the correct form and label. Every caller string (word, highlight, retry action, labels) is escaped; script and image injection tests pass. |

## Not built

- **Lesson-complete moment.** The spec's scope lists it, but no acceptance criterion needs it, and ADR-0010 forbids confetti. The path screen's step ticks and the review summary card already mark completion. Not built here.
- **Nothing runs on its own for more than 5 s without a stop.** The only looping animations are the audio loading and playing pulses (bounded by the clip) and the recording ring (user-started, stop button present). All three are off under reduced motion.
- **blocked_by SPEC-008** (ADR-0008, new look) is still open. The kit follows ADR-0010 and the existing tokens.

## Tests

- `npm test` (`SRS_SCHEDULER=ladder`): 1010 pass, 0 fail.
- `test/feedback-kit.test.js`: 21 pass. `test/player-feedback.test.js` still passes.
- `node test/chrome-navigation-smoke.js`: PASS, now includes the sound control and reduced-motion transition. `node test/chrome-audio-smoke.js`: ok.