# SPEC-003 · data: the word list leaves out the wrong side of a contrast

## Metadata

```yaml
spec_id: SPEC-003
initiative_id: INIT-037
title: "The word list leaves out the wrong side of a contrast"
domain: data
status: complete
primary_prompt: none
complexity: medium
complexity_reason: one derivation change plus a rebuild of every unit's lesson_items
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-002]
blocks: [SPEC-004, SPEC-011]
```

## Description

`deriveLessonItems` (`internal/db/put_unit.go:214-252`) tokenizes every string `Activity.Texts()` returns (`internal/units/doc.go:241-250`). On a `pattern` activity that is the whole "right — wrong" line. So the wrong form, such as לַעֲשׂוֹת in unit 12, becomes a unit word and then a button.

- For a `pattern` activity, tokenize only the text before the first " — " (U+2014 with a space either side). Other activity types are unchanged.
- If ADR-0032 D2 adds a structured `wrong` field, `deriveLessonItems` never tokenizes it.
- Rebuild `lesson_items` for every stored unit, published or draft, in one transaction per unit. `rebuildReferencedUnits` (`internal/db/lexemes.go:234-262`) already re-derives a unit; reuse it with an all-units entry point.

**Scope (out):** buttons (SPEC-004); the validator (SPEC-005, SPEC-006); the player's display of the line.

## Acceptance criteria

- [x] A pattern "שְׁלוֹשָׁה יָמִים — שָׁלוֹשׁ יָמִים" puts שְׁלוֹשָׁה and יָמִים in `lesson_items`, not שָׁלוֹשׁ (ADR-0032 D2).
- [x] After the rebuild, unit 12's `lesson_items` has no לַעֲשׂוֹת row, and every right-side word is still present.
- [x] A pattern line with no " — " is tokenized whole, as today.
- [x] Running the rebuild twice changes nothing on the second run.
- [x] A unit `newWord` that appears only on a wrong side loses its `new` role, and a test pins that case.

## Verification Strategy

- **Claim:** no wrong-side token enters a unit's word list, and every other token is unchanged.
- **Check + executor:** mechanized. `go test ./internal/db/...` with fixtures for pattern, dialogue and plain lines. The rebuild test diffs every unit's `lesson_items` before and after.
- **Pass condition:** every removed row is a wrong-side token; no other row is added or removed; the second run's diff is empty.
