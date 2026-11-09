# SPEC-002 · docs: ADR-0032, a lesson asks only about words the learner was taught

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-037
title: "ADR-0032: a lesson asks only about words the learner was taught"
domain: docs
status: complete
primary_prompt: none
complexity: medium
complexity_reason: sets four rules that the word list, buttons, validator and brief all follow
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 2 hours
blocked_by: []
blocks: [SPEC-003, SPEC-004, SPEC-005, SPEC-006, SPEC-007]
```

## Description

Write `docs/decisions/ADR-0032-taught-words-only.md`. The number was taken by hand, with no reservation. Decisions:

- **D1 Taught.** A word is taught for a learner when any of these holds:
  - they have a card for it;
  - it is a new word this unit introduces before the ask;
  - it is in a unit they finished.

  An earlier published unit they never opened does not count.
- **D2 A wrong form is never a unit word.** On a pattern line, the text after the first " — " is the wrong side: shown, never tokenized into `lesson_items`. Decide whether new units also get a structured `wrong` field. If yes, say which later spec adds it; this initiative only handles the stored form.
- **D3 Learner English carries no authoring notes.** A translation or prompt says what the Hebrew means, not how the author built the step.
- **D4 Consolidation is review first.** At most one consolidation unit is queued ahead. Beyond that, the brief points to review instead of writing a unit.
- **D5 A copy warns and a copied story blocks.** `unit.same_shape` warns at 80% or more the same, as INIT-030 SPEC-004 line 31 specified.
- **D6 Rebuild forward, one or two units at a time.** Old units are never edited in place; their revisions are kept.

Cite ASMT-004 and the lesson 12 complaint. Name what it amends: ADR-0026 D3 and D4 (pace), and INIT-030 SPEC-004 (shape rules).

**Scope (out):** code.

## Acceptance criteria

- [x] `docs/decisions/ADR-0032-taught-words-only.md` exists with decisions D1 to D6, each with its reason and a rejected alternative.
- [x] D2 states the stored form ("right — wrong" in a pattern `text`) and the decision on a structured field.
- [x] The ADR names the files each decision governs: `internal/db/put_unit.go`, `internal/web/lesson.go`, `internal/units/shape_rules.go`, `internal/brief/build.go`, `internal/brief/progression.go`, and the reviser skill.

## Verification Strategy

- **Claim:** the ADR states each rule so a spec can be checked against it.
- **Check + executor:** human. Brian reads it before SPEC-003 starts.
- **Pass condition:** every later spec's acceptance criteria cite a decision number in this ADR, and Brian approves.
