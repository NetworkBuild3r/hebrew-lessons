# SPEC-001 completion summary

**Initiative:** INIT-019 · **Spec:** SPEC-001 (data) · **Status:** complete · **Date:** 2026-10-23

## Change

Migration 014 adds `learner_mistakes`, `common_mistakes`, `verb_forms`, and a small `verse_notes` table. A repeated mistake for one learner stays one row and increments `times`. The same pair for another learner is a different row. Nothing copies a personal row into the shared list. Verb forms upsert by lemma, tense, person, number, and gender, and read back in that order. A missing lemma writes nothing.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestMistakesMigrationKeepsExistingRows`: existing rows stay, and the new tables are empty. |
| ac-2 | PASS | `TestMistakeRepeatStaysOneRowPerLearner`: one learner's repeat is `times = 2`; the other learner's identical pair is its own row. |
| ac-3 | PASS | `TestCommonMistakesHaveNoLearnerAndVerbFormsOrder`: `common_mistakes` has no learner column, and no store method copies a personal row across. |
| ac-4 | PASS | The same test: forms read back present, past, future, imperative, infinitive, then person. A missing lemma rolls the batch back. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. The store takes a learner id on every personal read and write, and no method lists mistakes across learners.
- `verse_notes` is only a ref and a status, so Isaiah can appear in navigation once a note is checked. The reader itself is INIT-021. The page is a placeholder until then.
- These writes do not add `publish_audit` rows. The spec does not require them.
- The migration is additive. Rollback is reverting the app. The tables stay unused. No production backup was taken before this lands.
