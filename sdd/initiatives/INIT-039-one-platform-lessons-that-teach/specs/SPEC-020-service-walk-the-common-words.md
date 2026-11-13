# SPEC-020 · service: walk the most common words

## Metadata

```yaml
spec_id: SPEC-020
initiative_id: INIT-039
title: "Walk the most common words"
domain: service
status: complete
primary_prompt: none
complexity: medium
complexity_reason: orders lessons by the frequency list and refuses a sentence under the coverage line
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 3 hours
blocked_by: [SPEC-019]
blocks: []
```

## Description

Brian asked on 2026-11-11 to build the lessons from the most common words. The first slice is the first six lemmas on the list. The next slice is the six after those. Each word shows how often it appears in every 1,000 subtitle words and the running share. A sentence is kept only when about 95 of every 100 words are ones he already knows, plus one token of the new word. A shorter line is refused. He types the new word. The spoken line is not graded. Today still links here when nothing is due.

## Acceptance criteria

- [x] The second slice is ranks 7 through 12 on the list.
- [x] A sentence of 19 already-known words plus one new list word is kept.
- [x] A short line, and a line with a word off the list, are refused.
- [x] The page shows that slice, the 95 percent line, and an ungraded line to say.

## Verification Strategy

- **Claim:** lessons walk the list in order, and a sentence under 95 percent known is refused.
- **Check + executor:** mechanized. `go test` on `./internal/speech/` and `TestSpeechLessonAfterReview`.
- **Pass condition:** those tests pass.
