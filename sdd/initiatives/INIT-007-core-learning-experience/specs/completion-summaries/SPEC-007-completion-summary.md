# SPEC-007 completion summary

**Spec:** SPEC-007 · Player visual regression, a11y, touch-target, and Chrome smoke verification  
**Initiative:** INIT-007  
**Completed:** 2026-10-11  
**Status:** complete (orchestrator closed 2026-10-11 — player-ui-a11y 4/4; npm test 859; chrome smokes ok)  
**Provenance:** `INIT-007/SPEC-007`

## What shipped

- `test/player-ui-a11y.test.js` — mechanized checks for primary target ≥44px, pointed Hebrew / ink contrast ≥4.5:1 (light+dark) and UI chrome ≥3:1 (reusing `spikes/look/check.js` luminance math), reduced-motion CSS audit across INIT-007 blocks + SPEC-013 global reduce, and ADR-0006 hook contract (70 `data-*` + 50 classes = 120 kept; retired `set-dir` documented).
- `test/chrome-review-smoke.js` — extend to grade Knew/Missed-family (`data-signal`) through `#review-finished` with Back to Today.
- `test/chrome-learner-support-smoke.js` — extend my-morning walkthrough with audio play trigger, story read-aloud highlight (`.reading-now`), and mastery ceremony after full-round progress seed.
- `test/chrome-navigation-smoke.js` — provenance note pointing ac-4 lesson/review coverage to the above smokes.
- `test/design-tokens.test.js` — snapshot update for new smoke-queried classes (`icon-btn`, `reading-now`, `story`, `story-body`).

## Acceptance criteria

| Id | Result | Evidence |
|---|---|---|
| ac-1 | Pass | Primary selectors (`.btn`, `.icon-btn`, `.path-btn`, `.review-grade`, action-bar, etc.) declare ≥44px; `--target-primary` 48px / `--size-target-primary` 44px. |
| ac-2 | Pass | ADR-0006 `tokens.json` contrast rows + player ink/paper/story pairs ≥4.5:1 / UI ≥3:1. |
| ac-3 | Pass | SPEC-013 `*` reduce + INIT-007/002–006 reduce blocks clear animation/transition; `--motion-fast` / `--motion-step` zeroed. |
| ac-4 | Pass | `chrome-learner-support-smoke.js` (match, story, type-it, audio, highlight, mastery) + `chrome-review-smoke.js` (reveal → grade → finished). |
| ac-5 | Pass | Parsed ADR-0006 tables: 70 attrs + 50 classes present in shared/+tests corpus. |

## Remediations

None. SPECs 002–006 CSS already cleared target, contrast, and reduced-motion floors; no production CSS edits required.

## Preserved

- `.path-btn` / `.path-label` use `var(--ink)`.
- `build-sentence.js` `playTarget` retained.

## Tests

- `node --test test/player-ui-a11y.test.js` — **4/4 pass**
- `npm test` — **859 pass / 0 fail**, exit 0
- `node test/chrome-review-smoke.js` — ok (`sessionComplete: true`, graded 2)
- `node test/chrome-learner-support-smoke.js` — ok (audioPlay, storyRead, mastery)

## Follow-ups

- Optional: retire remaining `data-action="set-dir"` in `words.js` if product direction no longer needs the alternate bar (ADR already marks it retired for production-recall markup).
