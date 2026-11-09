# SPEC-006 · content: a graded word must be taught, the English must be clean, and checkpoints are covered

## Metadata

```yaml
spec_id: SPEC-006
initiative_id: INIT-037
title: "A graded word must be taught, the English must be clean, and checkpoints are covered"
domain: content
status: complete
primary_prompt: none
complexity: medium
complexity_reason: three new validator rules, one of which reads the brief's learner words
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 5 hours
blocked_by: [SPEC-002]
blocks: [SPEC-008, SPEC-009]
```

## Description

Three gaps let units 12 to 34 through:

- **A graded word the learner never had.** The checkpoint contrasts graded and showed words such as לְהָכִין.
- **Authoring notes in the learner's English.** "Correct form first; incorrect form second." appears in every one of the 23 units.
- **Only stories are coverage-checked.** `coverage.story` and `brief.coverage` check stories and listen-picks, not checkpoint or pattern lines.

Add three rules. Each blocks in `ModeWrite` and warns in `ModePublished`, and each has a failing fixture in `internal/units/testdata`:

- **`unit.target_untaught`** (ADR-0032 D1). The graded word of a recall, produce, cloze, say or listen_pick must be one of:
  - this unit's `newWords`, introduced earlier in the unit;
  - in the brief's known or learning words;
  - a new word of an earlier unit in the brief's previous units.

  It runs with a brief. Without one, it is skipped and the reply says so.
- **`unit.meta_text`** (ADR-0032 D3). A `translation`, `prompt_en` or dialogue translation contains authoring vocabulary. The list lives in one place in `internal/units` and starts with: "correct form", "incorrect form", "wrong form", "first;", "second.", "target", "distractor", "checkpoint". It runs with and without a brief.
- **`brief.checkpoint_coverage`.** In a checkpoint lesson, the share of Hebrew tokens on pattern and graded lines that are known, learning, in the brief, or one of this unit's newWords meets the brief's tier figure. It uses `brief.coverage`'s counting rules and runs only with a brief.

Add the three rows to the catalog (`internal/units/rules.go`), `doc.go`, `authoring.md` and the author skill's rule table.

**Scope (out):** `unit.same_shape` and `unit.story_repeat` (SPEC-005); the brief's mistake choice (SPEC-007).

## Acceptance criteria

- [x] Unit 12's steak checkpoint, as a fixture with a brief whose learner has no card for לְהָכִין, blocks with `unit.target_untaught`.
- [x] Every one of the 23 units' English, as fixtures, gives `unit.meta_text`. "I live." gives none.
- [x] A checkpoint whose pattern lines are mostly untaught words blocks with `brief.checkpoint_coverage`.
- [x] Without a brief, `unit.target_untaught` and `brief.checkpoint_coverage` do not run, and the validate reply lists them as skipped.
- [x] The catalog-parity tests pass with the three new rows.

## Verification Strategy

- **Claim:** each rule blocks its bad shape on a new write, and nothing in the 11 finished units' good lines trips `unit.meta_text`.
- **Check + executor:** mechanized. `go test ./internal/units/... ./internal/mcp/...` with the fixtures; a test that runs `unit.meta_text` over every translation in units 1 to 11 as fixtures.
- **Pass condition:** every acceptance case passes, and the units 1 to 11 run has no `unit.meta_text` finding, or every hit is listed and justified in the test.
