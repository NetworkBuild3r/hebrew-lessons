# SPEC-005 · web: a word that also fits is never a wrong button

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-032
title: "A word that also fits is never a wrong button"
domain: web
status: planned
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-003, SPEC-004]
blocks: [SPEC-006]
```

## Description

The English cue settles most gaps. Two cases remain:

- **Near meanings.** "You say" and "you tell" are both right for "אַתָּה __" when the cue is loose. The gloss check in `wideChoices` drops only an identical gloss.
- **Author knowledge.** Only the author knows that לָגוּר and גָּר could both stand in a given line.

**Content.** A cloze may set `accept`: pointed Hebrew tokens that are also right in the gap. The field already exists on `Activity` for `order`. On a cloze, the validator (`internal/units/validate_cloze.go`) checks each entry:

- One token, with an exact pointed lexicon entry. Otherwise it is `unit.cloze_accept`, block.
- Different from `target`. Otherwise it is `unit.cloze_accept`, block.

Add the catalog row and the guide row, and mirror the skill.

**Player.** `wideChoices` takes the accept list through `choiceExclude` and never offers those entries. `lineMateExclude` already carries the words on the line. Also skip a distractor whose gloss shares its first word with the target's gloss after `glossKey` folding ("says" and "says, tells").

If an accepted word reaches the post handler anyway, for example from an old form, grade it right and log the answer as the target.

**Out of scope.** Telling, by grammar, whether a word fits the line. The lexicon does not hold enough to do that safely.

## Acceptance criteria

- [ ] `unit.cloze_accept` blocks an entry that is not one lexicon token, or that equals `target`.
- [ ] A cloze whose `accept` names a word in the unit never shows that word as a button, across 50 seeds.
- [ ] A distractor whose gloss starts with the target's first gloss word is skipped.
- [ ] Posting an accepted word grades right.
- [ ] The guide, the skill mirror, and the catalog name `unit.cloze_accept`.
- [ ] `go test ./internal/units/... ./internal/web/... ./internal/mcp/...` passes.
