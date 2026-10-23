# SPEC-002 completion summary

**Initiative:** INIT-019 · **Spec:** SPEC-002 (api) · **Status:** complete · **Date:** 2026-10-23

## Change

`record_mistakes` writes the caller's own mistakes from a learner token, and an admin must name the learner. `upsert_common_mistakes` and `upsert_verb_forms` are admin. A missing lemma writes nothing. `get_learner_model` returns the top 10 personal mistakes. `get_lesson_brief` carries up to three as `fixMistakes`, and `brief.fix_contrast` blocks a unit that never uses the right form. The authoring page and the lesson-author skill both say so.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestMistakeToolsScopeAndVerbBatch`: a learner token records its own row; the other learner's pair is absent from the model and the brief. |
| ac-2 | PASS | The same test: an admin `record_mistakes` with no learner id is refused. |
| ac-3 | PASS | The same test: the brief JSON has `fixMistakes` and the right form. `TestBriefCarriesThreeFixMistakes` keeps three. `brief.fix_contrast` is in the golden set. |
| ac-4 | PASS | The same test: a missing lemma returns the batch error and leaves the table empty. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. The learner id comes from the token. Lengths are capped in Go before the store. Personal mistakes are not copied into shared tool output.
- The brief payload version stays 1. Older briefs unmarshal with no `fixMistakes`.
