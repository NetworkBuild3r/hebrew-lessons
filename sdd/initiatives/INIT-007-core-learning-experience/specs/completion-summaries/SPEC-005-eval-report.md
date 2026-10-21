# SPEC-005 eval report

**Spec:** SPEC-005  
**Initiative:** INIT-007  
**Date:** 2026-10-11  
**Provenance:** `INIT-007/SPEC-005`  
**Verifier:** implementing session (not a fresh subagent)  
**Verdict:** PASS on mechanized checks · independent confirmation not recorded here

## Per AC

| AC | Verdict | Evidence |
|---|---|---|
| ac-1 | PASS | `learner-support.test.js` asserts eight distinct `.match-pair.pair-cN` edges; chrome smoke matches coloured pairs; CSS `match-wrong-shake` 0.12s |
| ac-2 | PASS | Source: `build-tile` / `build-remove`; CSS `.build-tiles .bank-word` / `.build-placed` min-height 44px |
| ac-3 | PASS | chrome-learner-support-smoke Type-It ladder + miss + Next; `type-check` / `type-hint` hooks intact; pointed `.type-answer` / `.correction-word` |
| ac-4 | PASS | speed-quiz adds `.selected` + `.correct`/`.wrong` + `.dimmed` on peers |
| ac-5 | PASS | ask-friend `.friend-audio-compare` model/you slots; `.recording` uses `rec-pulse-ring` |

## Tests run

- `npm test` — 855 pass, 0 fail, exit 0
- `node test/chrome-learner-support-smoke.js` — exit 0
- `node --test test/retry-before-reveal-smoke.test.js test/learner-support.test.js test/build-sentence-audio.test.js test/player-feedback.test.js` — pass

## Gate notes

- No new deps, no gamification, no `worksheets/*/index.html` edits, no commit.
- `playTarget(ctx.Speech, entry)` preserved for build-sentence speaker.
- `pair-c0`…`pair-c7` colour values unchanged.
- Fill/pattern first-miss still “Try again” only (retry-before-reveal smoke).
- Path-btn ink contrast untouched.

## Final

Mechanized blocking checks pass. Spec status set to `in_review` for orchestrator close.
