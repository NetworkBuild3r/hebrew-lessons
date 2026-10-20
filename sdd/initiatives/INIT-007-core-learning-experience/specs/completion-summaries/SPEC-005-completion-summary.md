# SPEC-005 completion summary

**Spec:** SPEC-005 · Interactive drills restyle (Match, Sentence Builder, Speed Quiz, Type-It)  
**Initiative:** INIT-007  
**Completed:** 2026-10-11  
**Status:** complete (orchestrator closed 2026-10-11 — 855 pass; learner-support smokes ok)  
**Provenance:** `INIT-007/SPEC-005`

## What shipped

- `shared/activities/match.js` — clearer selection / wrong flash (120ms shake), pair checkmarks, optional `HebrewFeedback` chimes/complete affirmation. Preserved `pair-c0`…`pair-c7` and `data-match`.
- `shared/activities/build-sentence.js` — magnetic tiles (tap bank → slot; tap placed tile → bank), dashed slot affordance, model-audio speaker still via `playTarget(ctx.Speech, entry)` (never TTS of partial tiles).
- `shared/activities/speed-quiz.js` — calm timer bar + non-alarm clock; choice cards show selected / correct / wrong / dimmed contrast.
- `shared/activities/fill.js` — auto-focus first blank; bank chips unchanged; reveal uses `HebrewFeedback.showMiss` with pointed form + speak while keeping retry-before-reveal on first miss.
- `shared/activities/type-it.js` — RTL scaled input, soft keyboard hint, miss correction via `HebrewFeedback.showMiss` (fallback HTML), preserves `data-action="type-check"` / `type-hint` / `type-next` and `.type-answer`.
- `shared/activities/pattern.js` — `.pattern-box` grammar highlight; reveal/correct via feedback kit; first miss still “Try again” only.
- `shared/activities/ask-friend.js` — side-by-side model vs learner recorder slots; recording pulse retained (`.recording`).
- `shared/worksheet.css` — additive drill styles under `INIT-007/SPEC-005` (timer bar, build slot/tiles, pattern box, friend compare, match shake). Extended existing bank/choice/match rules; did not delete SPEC-002 tactile/correction or path-btn ink.

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass | Match selected borders + `.pair-c0`…`.pair-c7` + `.wrong` shake; `test/learner-support.test.js` + chrome smoke |
| ac-2 | Pass | Build tiles `min-height: 44px`; bank ↔ slot via `build-tile` / `build-remove` |
| ac-3 | Pass | Type-It miss shows pointed form + Next; hooks preserved; chrome-learner-support-smoke `typeIt: ok` |
| ac-4 | Pass | Speed Quiz `.selected` / `.correct` / `.wrong` / `.dimmed` |
| ac-5 | Pass | Ask-Friend model+you compare + `.recording` pulse ring |

## Understood

- und-1: Eight pair colour classes preserved for learner-support tests.
- und-2: Inputs / selection cards ≥44px.
- und-3: Pointed correction on Type-It / reveal paths; fill/pattern keep retry-before-reveal on first miss.

## Tests

- `npm test` — 855 pass, 0 fail, exit 0
- `node test/chrome-learner-support-smoke.js` — ok (`match: 3 pairs coloured`, `typeIt: ok`)
- `node --test test/retry-before-reveal-smoke.test.js test/learner-support.test.js test/build-sentence-audio.test.js` — pass

## CSS ownership / conflicts

- Owns drill activity CSS + bank-word / choice-card / word-match-card states only.
- Left vocab-card / story-body (SPEC-004), review/mastery (SPEC-006), `.player-top` / `.action-bar` (SPEC-003) alone.
- Preserved path button ink (`color: inherit` / `var(--ink)`).

## Follow-ups

- SPEC-007 can add visual/a11y smokes for timer bar, build-remove, and recording pulse.

## Eval

See `SPEC-005-eval-report.md`.
