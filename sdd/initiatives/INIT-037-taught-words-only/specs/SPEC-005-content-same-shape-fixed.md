# SPEC-005 · content: `unit.same_shape` warns on a copy, and a copied story blocks

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-037
title: "unit.same_shape warns on a copy, and a copied story blocks"
domain: content
status: complete
primary_prompt: none
complexity: low
complexity_reason: inverts one comparison and adds one story check, with text fixes in five places
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 2 hours
blocked_by: [SPEC-002]
blocks: [SPEC-008, SPEC-009]
```

## Description

INIT-030 SPEC-004 line 31 specified a warning when a unit's activity types are at least 80% the same as the previous unit's. `sameShapeIssues` (`internal/units/shape_rules.go:235-248`) warns when they are under 80%, and `shape_rules_test.go:145-163` pins that. Units 20, 27 and 34 have identical sequences, and their stories differ by one phrase.

- Warn when similarity is at least `SameShapeMin` (0.80), not under it (ADR-0032 D5).
- New block, `unit.story_repeat`: a story or dialogue whose normalized text equals the first story of an earlier unit on the same path. "Normalized" means NFC, niqqud kept, whitespace collapsed. It runs with and without a brief, and blocks in `ModeWrite` and warns in `ModePublished`, like `unit.lesson_repeat`.
- Fix every place that describes the old rule:
  - the test;
  - the catalog row (`internal/units/rules.go:187`);
  - the doc comment (`internal/units/doc.go:48`);
  - the guide (`internal/mcp/resources/authoring.md:108`, `:204`);
  - the author skill (`.cursor/skills/hebrew-lesson-author/SKILL.md:119`, `:215`).

**Scope (out):** other new rules (SPEC-006); the health sweep (SPEC-009).

## Acceptance criteria

- [x] Units 27 and 34's activity sequences, as fixtures, give `unit.same_shape` against each other. A varied sequence gives none.
- [x] Unit 34's story against unit 27's, both fixtures, gives a `unit.story_repeat` block in `ModeWrite`.
- [x] The rule catalog, `doc.go`, `authoring.md` and the author skill all say "at least 80% the same", and a grep finds no "under 80%".
- [x] `go test ./internal/units/... ./internal/mcp/...` passes, including the catalog-parity tests.

## Verification Strategy

- **Claim:** a copied shape warns, a copied story blocks, and every description matches the code.
- **Check + executor:** mechanized. `go test ./internal/units/... ./internal/mcp/...`, plus `rg "under 80"` over `internal/` and `.cursor/skills/`.
- **Pass condition:** all tests pass and the grep has no hits.
