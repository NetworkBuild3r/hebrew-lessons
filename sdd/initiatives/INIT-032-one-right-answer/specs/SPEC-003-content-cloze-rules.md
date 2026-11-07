# SPEC-003 · content: a cloze needs a sentence and its English, and a lesson cannot repeat

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-032
title: "A cloze needs a sentence and its English, and a lesson cannot repeat"
domain: content
status: built, uncommitted
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: none
estimated_effort: 2 hours
blocked_by: [SPEC-001]
blocks: [SPEC-005]
```

## Description

Three validator rules in `internal/units/validate_cloze.go`, called from `ValidateIn`. Each is a block in `ModeWrite` and a warning in `ModePublished`, the same split as `activity.target`.

- `unit.cloze_context`: a cloze line has fewer than `MinClozeTokens` (2) tokens. Pointer `/lessons/i/activities/j/text`. The message says to use `produce` for one word.
- `unit.cloze_translation`: a cloze has an empty `translation`. Pointer `.../translation`. The message says the learner sees it as the cue.
- `unit.lesson_repeat`: a `lesson` whose activities marshal to the same JSON as an earlier `lesson` of the unit. Checkpoints are skipped. Pointer `/lessons/i`.

Catalog rows in `internal/units/rules.go` (sorted), the list in `internal/units/doc.go`, and rows in `internal/mcp/resources/authoring.md`. The `cloze` row of the activity table says at least two words, and that `translation` is the cue. Both worked examples in the guide give their cloze a translation. `.cursor/skills/hebrew-lesson-author/SKILL.md` is the guide after its front matter, byte for byte.

Built in the working tree, with `internal/units/validate_cloze_test.go`.

Still to do: name the three rules in `.cursor/skills/hebrew-lesson-reviser/SKILL.md`. That file has local edits that are not part of this work, so change only the rules lines and leave the rest as it is.

## Acceptance criteria

- [x] A one-word cloze is `unit.cloze_context`, block, on a new write.
- [x] A cloze with no translation is `unit.cloze_translation`, block, on a new write.
- [x] A lesson equal to an earlier lesson is `unit.lesson_repeat`, block. An equal checkpoint is not flagged.
- [x] All three warn in `ModePublished`.
- [x] `go test ./internal/units/... ./internal/mcp/...` passes, including the skill mirror test and the goldens.
- [ ] The reviser skill names the three rules.
