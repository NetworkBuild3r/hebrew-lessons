# SPEC-002 completion summary

**Initiative:** INIT-020 · **Spec:** SPEC-002 (service) · **Status:** complete · **Date:** 2026-10-23

## Change

A lesson can include a `say` step: an English cue (`prompt_en`, at most 200 characters, no Hebrew and no markup) and the pointed Hebrew the learner should say. The validator blocks a missing cue (`say.prompt_required`) and a missing clip (`say.audio_required`). A self-check writes one attempt with activity `say`, skill `recall`, and source `self`, and a peeked review log (Good for "got it", Again for "not yet"). Migration 015 rebuilds `attempts` so that source is allowed. The MCP schema, the activity catalog, and the authoring page all name the ninth type.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestSayStepNeedsPromptAndAudio`: a cued say step with a clip validates; a missing cue, a long cue, a Hebrew cue, and a missing clip fail on `say.prompt_required` or `say.audio_required`. |
| ac-2 | PASS | `TestSayCheckWritesAttemptAndReview`: one attempt (`say` / `self` / `recall`) and one peeked review log. A second check with "not yet" writes `correct = 0`. |
| ac-3 | PASS | `TestDialogueIsInTheAuthoringResources` wants nine catalog types. `TestResourcesAuthoringNamesEveryActivityType` and `TestResourcesAuthoringSkillMirror` cover `say`, `prompt_en`, and the two rule rows. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. `prompt_en` is capped at 200 characters. Markup is `unit.no_markup`. A Hebrew cue is `say.prompt_required`.
- The player does not render a say step yet. That is SPEC-003. A published say unit would fall through to the existing lesson screen until that spec ships. Nothing in production is a say step.
- Migration 015 is in this commit and is not deployed. Production stays on the previous image until a later spec with a deploy gate. The rebuild copies every existing attempts row. No production backup was taken, because production has not run it.
- INIT-021's planned migration number moved from 015 to 016 so the two specs do not share a file.
- SPEC-001, SPEC-003, and SPEC-004 stay planned. The initiative stays planned.
