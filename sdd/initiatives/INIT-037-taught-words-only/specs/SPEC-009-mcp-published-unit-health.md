# SPEC-009 · mcp: published-unit block counts and a read-only sweep

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-037
title: "Published-unit block counts and a read-only sweep"
domain: mcp
status: complete
primary_prompt: none
complexity: medium
complexity_reason: runs write-mode validation per published unit inside an existing read tool
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: standard
estimated_effort: 3 hours
blocked_by: [SPEC-005, SPEC-006]
blocks: [SPEC-010]
```

## Description

A published unit is checked only when it is saved, and newer rules only warn on it (`internal/units/validate_cloze.go:12-20`). Nothing lists the published units that today's rules block. ASMT-004 found all 34 on learner 1's path did, using an ad hoc script.

- **`get_context`.** Each outline row gains `blocks`: the number of block issues the unit's published document gets under `ModeWrite`, with no brief. Cache by unit revision and rule-catalog version, so the outline stays fast.
- **Sweep.** A read-only sweep, `cmd/unit-health`, or a test-only helper that the ops spec runs. For every published unit in a scope, it writes id, position, state, block count and rule ids. It never calls a write tool, and it compares shapes without a brief: same activity sequence as the previous unit, or same first story.

**Scope (out):** Today's behaviour (SPEC-010); fixing units (SPEC-012).

## Acceptance criteria

- [x] `get_context` for learner 1 returns `blocks` on every outline row, and 0 for a unit that passes.
- [x] A learner token sees `blocks` only for its own and shared units.
- [x] The sweep on a fixture store lists a copied unit and a one-word-cloze unit, and nothing else.
- [x] The sweep has no code path that calls `put_unit`, `publish_unit`, `unpublish_unit` or any `upsert_*`, and a test checks the tool allowlist.

## Verification Strategy

- **Claim:** every published unit's block count is visible to the author, and the sweep is read-only.
- **Check + executor:** mechanized. `go test ./internal/mcp/... ./internal/mcpstore/...` and the sweep's fixture test.
- **Pass condition:** the acceptance cases pass, and the allowlist test fails if a write tool is added.
