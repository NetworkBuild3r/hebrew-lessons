# SPEC-004 completion summary

**Initiative:** INIT-020 · **Spec:** SPEC-004 (frontend) · **Status:** complete · **Date:** 2026-10-23

## Change

A dialogue can name the learner's side with `play` (`a` or `b`). The validator requires an English cue on each of that side's lines (`dialogue.play`). The player then walks the lines: the other speaker's line shows and plays, and the learner's line is the cue until "I said it", then the cards, the clip, and one self-check. Leaving and coming back resumes the line. Migration 016 adds `lesson_runs.line_ix` and `lesson_runs.side` for that. A dialogue with no `play` still shows the whole conversation, then offers "Say Dana's lines" and "Say Avi's lines".

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestDialogueSideHidesTheLearnersLine`: the learner's line has the cue and no Hebrew and no audio until "I said it". Leaving and opening the lesson again is still that line. |
| ac-2 | PASS | The same test: one attempt (`say` / `self`) for the learner's line, then the lesson is complete. `TestDialoguePlayNeedsACue` refuses a side with no cue and accepts one with a cue. |
| ac-3 | PASS | `TestDialogueSideInBrowser`: 390 and 1440, light and dark, keyboard-only, and script off, with screenshots. `TestDialogueInBrowser` still requires the full conversation to stay silent until the learner presses play, and it saves a screenshot of that screen. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. `play` is `a` or `b`. A learner-line cue follows the same length and markup rules as `prompt_en`.
- Migration 016 adds two columns to `lesson_runs`. It does not copy or rebuild a table. It ships in the same deploy as migration 015. No production backup was taken.
- Speaker b uses the male voice. Until SPEC-001 sets that voice, a line for speaker b has no clip and is not played in the female voice. The offer buttons are on every full dialogue, including ones already published.
- INIT-021's planned migration number moved from 016 to 017 so the two specs do not share a file.
- SPEC-001 stays planned. The initiative stays planned. Dehydration tooling is not in this repo, so the specs stay on disk.
