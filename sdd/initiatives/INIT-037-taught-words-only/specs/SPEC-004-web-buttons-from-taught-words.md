# SPEC-004 · web: buttons come only from taught words

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-037
title: "Buttons come only from taught words"
domain: web
status: complete
primary_prompt: none
complexity: medium
complexity_reason: changes the pool every graded ask reads, with a reveal fallback already in place
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-002, SPEC-003]
blocks: [SPEC-011]
```

## Description

Every graded ask builds its buttons from two pools:
- `UnitLexemes`: every word on any line of the unit (`internal/db/lesson.go:209-225`);
- `EarlierUnitLexemes`: every published unit at a lower position, whether opened or not (`:227-251`).

Callers: `askChoiceCount` (`internal/web/lesson.go:1339-1352`), `askChoices`, `wideChoices` (`:1391-1441`) and `lessonChoices` (`:1562-1598`). Neither pool checks the learner (ADR-0032 D1).

- **Unit tier.** This unit's `newWords`, plus unit words the learner has a card for (`LearnerCardLexemeIDs`, `internal/db/cards_new.go:126`).
- **Earlier tier.** Words of units the learner finished (`UnitProgress`, `internal/db/scope.go:209`), plus any other words the learner has a card for.
- Apply this to every graded ask (recall, cloze, listen_pick, produce), on the render path and the post path, so `askChoiceCount` still agrees with what is shown.
- Keep the existing rules: line-mates excluded (`choiceExclude`, `:1600-1608`), no shared gloss, and same part of speech first. Fewer than three buttons gives the existing reveal (`:1437-1439`).
- Load the learner's card set once per request, not once per tier.

**Scope (out):** the word list itself (SPEC-003); INIT-032 SPEC-005's `accept` list, which removes also-right words and composes with this.

## Acceptance criteria

- [x] On a rebuilt copy of unit 12's steak checkpoint, no button is לְהָכִין or עוֹשֶׂה for a learner with no card for them (ADR-0032 D1).
- [x] A word from an earlier published unit the learner has not finished, and has no card for, is never a button.
- [x] A new word of this unit can be a button in a later step of the same unit.
- [x] With fewer than three taught candidates, the step reveals and continues, and the post path grades nothing.
- [x] Across 50 seeds per graded step of a fixture path, every button is the target or a taught word.
- [x] The render path and the post path agree on the button count for every fixture step.

## Verification Strategy

- **Claim:** every button is the target or a word the learner was taught, and the count stays consistent between render and grade.
- **Check + executor:** mechanized. `go test ./internal/web/... ./internal/db/...` with a fixture learner who has cards for some words, one finished unit and one unfinished earlier unit.
- **Pass condition:** the 50-seed sweep finds no untaught button, and every acceptance case passes.
