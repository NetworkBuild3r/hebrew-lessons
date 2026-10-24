# SPEC-003 completion summary

**Initiative:** INIT-020 · **Spec:** SPEC-003 (frontend) · **Status:** complete · **Date:** 2026-10-23

## Change

A say step in the lesson player shows the English cue large, with "Say it in Hebrew" and one button, "I said it". No Hebrew and no audio are on that screen. After "I said it", the model is word cards, the line plays once, and "Got it" / "Not yet" write one self-check (source `self`, activity `say`). Without script, the buttons are ordinary form posts and the clip is the browser's own audio bar.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestSayStepHidesHebrewUntilTheLearnerSpeaks`: the ask screen has the cue and "I said it", and no Hebrew and no audio element. |
| ac-2 | PASS | The same test: after "I said it" the model, its clip, "Got it", and "Not yet" are on screen. "Got it" writes one attempt (`say` / `self`) and finishes the step. A second post does not write another. |
| ac-3 | PASS | `TestSayStepInBrowser`: 390 and 1440, light and dark, keyboard-only, and script off. Screenshots of the ask and the reveal. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. The step adds no microphone and no new stored field. The self-check is the attempt SPEC-002 already defined.
- A published say step now renders this screen. SPEC-002's note that the player ignored say is superseded by this spec.
- The validator still refuses a say step that has no clip. New clips wait on SPEC-001. The player renders a say step that already has audio.
- SPEC-001 stays planned. The initiative stays planned.
