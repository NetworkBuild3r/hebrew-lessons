# SPEC-011 · service: the "hear the difference" drill in the review mix

## Metadata

```yaml
spec_id: SPEC-011
initiative_id: INIT-036
title: "The \"hear the difference\" drill in the review mix"
domain: service
status: planned
primary_prompt: none
complexity: high
complexity_reason: a new drill kind woven into the review mix, with its own grading and a guidance rewrite
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 6 hours
blocked_by: [SPEC-009, SPEC-010]
blocks: [SPEC-012]
```

## Description

A new drill kind, `sound`, beside the letter and pattern drills (`internal/session/doc.go:5-12`, `internal/session/drill.go`). The learner hears one word and picks which of two written words it was.

- **When it appears.** After a listening miss on a word that has an approved pair, and as a seeded step for a learner with approved pairs and none answered. It takes the drill slots the letter drills use, inside `DrillCap` (6), and never takes a known or struggle slot.
- **Grading.** Like letter drills: a drill is not an FSRS card, answers go to a drill table, and three right, non-slow answers clear the pair. A wrong answer shows both clips and is not re-asked in the same sitting (ADR-0024 D3).
- **Which pairs.** Approved pairs only, whose two words the learner has seen. Each trial picks one of the two words as the clip at random from the sitting seed, so the same clock and rows build the same queue.
- **Both voices.** The clip alternates between the two voices across trials, since several-voice training is the research's direction (RSCH-003 line 42; the many-voices part is Low).
- **Guidance.** Rewrite C1 and C2 in `internal/units/rules.go:234-235` to name true contrasts only, and drop the letter pairs the inventory rejects. The validator still does not enforce guidance.
- Evidence rows record `activity: sound` so SPEC-006's later-day results by kind can read it.

**Scope (out):** the screen (SPEC-012), recording, and automatic checking.

## Acceptance criteria

- [ ] A learner with no approved pairs gets no sound drills and an unchanged queue.
- [ ] A listening miss on a word with an approved pair adds up to the configured drill steps, inside `DrillCap`.
- [ ] Study-card order and the known and struggle counts are unchanged by sound drills.
- [ ] Three right non-slow answers clear a pair; a wrong answer is not re-asked in the same sitting.
- [ ] The same clock, seed and rows give the same queue and the same clip choice.
- [ ] Guidance C1 and C2 no longer list a rejected letter pair.
- [ ] Sound answers are stored with `activity` set to `sound`, and they do not change any FSRS card or the known state.

## Verification Strategy

- **Claim:** the drill is served only for approved pairs, inside the existing caps, deterministically, and affects no card.
- **Check + executor:** mechanized. `go test ./internal/session/... ./internal/units/...` with seeded learners, and a determinism test that builds the same sitting twice.
- **Pass condition:** every acceptance case has a test; the card table and `review_log` are identical before and after a sitting of only sound drills.
