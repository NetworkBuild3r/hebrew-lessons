# SPEC-025 · service: never the same kind of question twice in a row

## Metadata

```yaml
spec_id: SPEC-025
initiative_id: INIT-039
title: "Never the same kind of question twice in a row"
domain: service
status: complete
primary_prompt: none
complexity: low
complexity_reason: one constant changes, and the tests that pinned the old value are updated
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: none
estimated_effort: 2 hours
blocked_by: []
blocks: []
```

## Description

Brian said on 2026-11-12 that lessons that are the same every time bore him fast. Review already spreads question kinds: `limitKindRuns` and `spreadKinds` in `internal/session/mix.go` stop a run of one activity kind at `MaxKindRun`, which is 3, while another kind is waiting. They never drop a card. This spec sets `MaxKindRun` to 1, so two study questions in a row are not the same kind while another kind is still waiting. Words from the common-word list are read and typed until clips exist, so a sitting mixes `read_pick`, `word_match`, and `type_it`.

No report on file tests whether varying the kind of question improves recall. This change is for the boredom Brian named. Spaced recall, which review already does, is what makes words stay (RSCH-003). The change does not alter which cards are chosen, how many, their skills, or their grades. Only the seat order changes.

## What to change

- `internal/session/mix.go`: `MaxKindRun = 1`. Update its comment: "the longest run of one activity kind while another kind is still waiting".
- `internal/session/doc.go`: add one sentence under Algorithm saying that two study visits in a row are not the same activity kind while another kind is waiting. Drills keep their seats.
- `limitKindRuns` runs before `separateLexemes` in `mixGateCap`. Leave that order. Separating the same word matters more than separating kinds, so after `separateLexemes` a kind may still repeat. The test below allows for that.
- Find every test that assumed runs of 3, with `rg -n "MaxKindRun|run of 3|three in a row" internal/session`, and update the expected queues. Do not weaken a test that checks something else, such as struggle runs or the new-card order.

## Tests

In `internal/session/mix_test.go`:

- `TestKindsAlternateWhenTheyCan`. Twelve due study candidates on twelve different words: six asked by `read_pick` and six by `type_it`. Give them facts so `ChooseSkill` picks those kinds. In the built queue, no two adjacent study visits share an activity kind.
- `TestKindRunWhenOnlyOneKindIsLeft`. Four `type_it` candidates and one `read_pick`. The queue keeps all five, with the `read_pick` placed as early as the rule allows.
- `TestKindSpreadKeepsTheCards`. For a mixed set, the cards in the queue (by `CardID`) are the same set before and after spreading.

Then run `go test ./internal/session/ ./internal/web/`.

## Acceptance criteria

- [ ] `MaxKindRun` is 1.
- [ ] The three tests pass, and every existing session and web test passes after any expected-queue updates.
- [ ] No card is added or dropped by the change. The third test proves it.

## Verification Strategy

- **Claim:** a sitting does not ask the same kind twice in a row while another kind is waiting.
- **Check + executor:** mechanized. `go test ./internal/session/ ./internal/web/` in Docker `golang:1.26`.
- **Pass condition:** all of them pass.
