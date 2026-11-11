# SPEC-015 · service: a full spelling counts as the word

## Metadata

```yaml
spec_id: SPEC-015
initiative_id: INIT-038
title: "A full spelling counts as the word"
domain: service
status: complete
primary_prompt: none
complexity: medium
complexity_reason: the fold must accept the tutor's extra vowel letters and must not accept a different word
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: []
```

## Description

Review grades a typed answer with `fsrs.GradeTyped` (`internal/fsrs/typed.go:19`). Vowel marks are stripped, then distance 0 is Good, distance 1 is Hard, and distance 2 or more is Again. A Hard answer still counts as right (`internal/activity/check.go:191`, `rating != Again`) and is stored as a near miss. An Again is a wrong mark.

**Design:** [Lesson types — Nov 2026](https://www.figma.com/design/NJ8HfTPCQgrFty2VVQBMhY?node-id=255-182), file `NJ8HfTPCQgrFty2VVQBMhY`, Write column `255:182` on board `255:82`. The screen shows the typed word and "This question is wrong".

The lexicon stores the pointed form. Its consonants often omit the ו and י that everyday unpointed spelling adds. One extra letter is a near miss. Two are a wrong mark. Brian named that as a reason he stops trusting a lesson ([BRAIN-002](../../../brainstorms/BRAIN-002-lesson-types-done-right/decision-document.md)). He asked for this fix inside INIT-038 on 2026-11-10.

- **ו and י that carry no new consonant count as the same word.** After the existing normalization, drop ו and י from both strings. If what remains is equal, at least 2 letters, and no other lexicon entry's consonant form folds to that same remainder, the rating is Good. Not Hard.
- **A fold that hits two entries does not apply.** `בית` and `בת` fold to the same letters, so neither is accepted as the other. The existing distance grade stands. The comparison uses every lexeme's consonant form, not only words Brian has been taught, so a later taught word cannot turn a previously accepted fold into a collision after the fact without the check seeing it.
- **Nothing else gets easier.** A one-letter typo that is not only ו or י stays Hard. Two unrelated letters stay Again. Final letters stay distinct (`ך` is not `כ`). Niqqud stripping stays as it is.
- **Callers pass the other forms.** `GradeTyped` keeps its current meaning for a single expected string. The review path (`gradeTyped`) and the placement path (`internal/placement/service.go:362`, which accepts Good only) call a new function that also receives the other consonant forms. An empty other-set means the fold is unique.
- **Past answers stay put.** `hebrew backfill-spell` is not run again. Rows already written keep their rating.

**What Brian will see differently:** when he types the everyday spelling of a word the lexicon stores without those vowel letters, review marks it right. A spelling that is also a different word is still marked wrong.

**Removes:** the false Again, and the false near miss, on a spelling that differs from the stored consonants only by ו and י, when that folded form belongs to one lexicon entry.

**Scope (out):** a stored list of extra spellings on the lexeme (Stage 2, if the fold is not enough); lesson `produce`, which is still three Hebrew buttons; re-grading old attempts; any tutor sentence committed into the test file.

## Acceptance criteria

- [ ] A spelling that adds only ו, only י, or both, to the stored consonants is Good, when no other lexeme folds to the same remainder.
- [ ] `בית` typed for `בת`, and `בת` typed for `בית`, are not Good.
- [ ] A one-letter change that is not ו or י is still Hard. Two unrelated letters are still Again. Final `ך` against `כ` is still Hard.
- [ ] The existing cases in `internal/fsrs/typed_test.go` still pass.
- [ ] A synthetic table of at least 40 rows covers each case above, including generated extra-ו and extra-י pairs. The table contains no tutor text.
- [ ] Review stores a fold-accepted answer as correct and not a near miss. Placement accepts it. `hebrew backfill-spell` is not invoked by this spec.

## Verification Strategy

- **Claim:** a full spelling is marked right, and a different word is not.
- **Check + executor:** mechanized. `go test ./internal/fsrs/... ./internal/activity/... ./internal/placement/...`
- **Pass condition:** every acceptance case passes.
