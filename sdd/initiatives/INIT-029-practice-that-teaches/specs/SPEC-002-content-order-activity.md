# SPEC-002 · content: the `order` activity in the unit schema and validator

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-029
title: "The order activity in the unit schema and validator"
domain: content
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003, SPEC-008]
```

## Description

`internal/units` gains `ActivityOrder = "order"`. An order activity has `text` (a pointed Hebrew line), `translation` (the English cue), and an optional `accept`, which is a list of other full orders of the same words that are also right.

Every existing text rule applies to `text`: it must be pointed, have no markup, be found in the lexicon, have an audio clip, and fit the brief's coverage. New rules, with stable ids listed in `doc.go`:

- `unit.order_tokens`: the line has 3 to 8 tokens (block).
- `unit.order_translation`: `translation` is present, is English, and is no longer than 120 characters (block).
- `unit.order_accept`: each `accept` entry uses exactly the same tokens as `text`, in a different order (block).
- `brief.order_known`: every token is a known word, or a new word that an earlier step in the same unit introduced (block).

## Acceptance criteria

- [ ] A valid order activity passes `Validate` and `ValidateWithBrief`.
- [ ] Each new rule has a passing case and a failing case in table tests.
- [ ] An `accept` entry that adds, drops, or repeats a token is blocked.
- [ ] An order line that uses a new word before that word is introduced is blocked by `brief.order_known`.
- [ ] `hebrew://schema/unit` describes the new fields.
