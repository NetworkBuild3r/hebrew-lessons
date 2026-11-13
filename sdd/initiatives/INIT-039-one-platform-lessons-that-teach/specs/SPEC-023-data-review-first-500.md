# SPEC-023 · data: a second agent checks the first 500 course words

## Metadata

```yaml
spec_id: SPEC-023
initiative_id: INIT-039
title: "Check the first 500 course words"
domain: data
status: complete
primary_prompt: none
complexity: medium
complexity_reason: about 500 judgments against dictionary senses, recorded row by row
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 4 hours
blocked_by: [SPEC-022]
blocks: []
```

## Who runs this

This must be run by an agent session that did not build SPEC-022. ADR-0034 D3, as amended in SPEC-021, needs a second agent to sign every graded meaning. The same rule was used for class sheets (INIT-039 decision A4).

## Description

SPEC-022 builds `internal/speech/course.tsv` and leaves every row unsigned, so nothing is gradable. This spec checks every non-dropped row with list rank 1 to 500 and signs it in `internal/speech/course_fixes.tsv`. At five new words a day, 500 words is about 100 study days of course.

The meaning must be the one an adult speaking everyday modern Hebrew needs. A machine pick gets some of these wrong. Examples: טוב picked as "well" when it means "good", and איש as "anyone" when it means "man".

## How to check one row

1. Run `go run ./cmd/coursegen -dict <file> -show <lemma>`. It prints every Wiktionary entry and sense for that lemma.
2. Compare the row's pointed form, English, part of speech, gender, and root and pattern for verbs with those senses. Choose the everyday modern sense. For a verb, the gloss is the infinitive in English, such as "to say".
3. Write or edit that row's line in `course_fixes.tsv`:
   - `keep` when every field is right.
   - `fix` with the corrected fields when any field is wrong or empty. A verb with no root or pattern gets them here.
   - `drop` with a reason for a name, a parser artifact, or a form that duplicates another course word. A drop is not signed.
   - For SPEC-022's `prefix` and `split` rows, sign each line that is right, and correct the ones that are not.
4. Fill `reviewed_by` with `agent:<model name>` and `reviewed_on` with the date (YYYY-MM-DD).
5. When the right meaning is not among the Wiktionary senses, a `fix` may still give it. Its reason must then begin `not in Wiktionary senses:`, followed by why.

## Rules

- The gloss is 1 to 4 English words, with no Hebrew and no angle brackets. Parentheses are allowed, as in `you (f.)`.
- Do not sign a row that you changed only to make the check pass when you are not sure of the meaning. Leave it unsigned, with a reason.
- Do not sign the row `אַתְּ` "you (f.)". It stays `course.duplicate` until a later spec solves typed grading of two words with one spelling.
- Do not edit `course.tsv` by hand. Rerun the generator.

## Acceptance criteria

- [x] Every row with rank 1 to 500 is `ready`, `dropped`, or unsigned with a reason in `course_fixes.tsv`.
- [x] No more than 25 of those rows are unsigned.
- [x] The rows named in SPEC-022 section 3 read as given there, or carry a `fix` with a reason.
- [x] `go test ./cmd/coursegen/ ./internal/speech/` passes after regenerating.
- [x] `sdd/initiatives/INIT-039-one-platform-lessons-that-teach/review-first-500.md` gives the counts of keep, fix, drop, and unsigned, lists every `not in Wiktionary senses` row, and names the reviewing model.

## Verification Strategy

- **Claim:** the first 500 course words have a meaning signed by a second agent.
- **Check + executor:** mechanized for the counts. A test in `internal/speech/course_test.go`, `TestFirstFiveHundredAreSigned`, counts ready rows with rank ≤ 500 and fails when ready plus dropped is fewer than (rows with rank ≤ 500) − 25. The meanings themselves are checked by the second agent, as recorded in the review file.
- **Pass condition:** that test and the rest of the package pass.
