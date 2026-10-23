# SPEC-002 · data: Roots and verb patterns in the lexicon

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-016
title: "Roots and verb patterns in the lexicon"
domain: data
status: planned
primary_prompt: .claude/agents/database-architect/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: []
reversibility_class: branch
security_review: standard
estimated_effort: 3 hours
blocked_by: [SPEC-001]
blocks: [SPEC-003, INIT-018/SPEC-001]
```

## Description

**Context:** `lexemes.root` and `lexemes.binyan` exist since `001_init.sql`, but `lexicon.Input` and `lexicon.Row` have no fields for them, so nothing can write them. The roots hub, verb tables and the class backfill all need them.

**Scope (in):**

- `lexicon.Input` gains `Root *string` and `Binyan *string`, with the same nil-keeps-stored rule as `POS`.
- Validation: a root is 2 to 4 Hebrew consonants, written with a separator (`שׁ־מ־ע` or `ש-מ-ע`), stored normalized to letters joined by U+05BE maqaf, dots for shin and sin kept, niqqud removed. A binyan is one of `paal`, `nifal`, `piel`, `pual`, `hifil`, `hufal`, `hitpael`, or empty.
- `UpsertLexemes` writes both. `LexemeView` and the lookup reads return them.
- `Store.LexemesByRoot(ctx, root) ([]LexemeView, error)` and `Store.RootCounts(ctx) (map[string]int, error)`.

**Scope (out):** MCP (SPEC-003). Pages (INIT-018/SPEC-002).

## Acceptance Criteria

- [ ] ac-1: Upserting with a root and binyan stores them normalized; upserting the same row again with the fields omitted keeps them; sending an empty string clears them. (REQ-001)
- [ ] ac-2: A root with a non-Hebrew letter, fewer than 2 or more than 4 letters, or niqqud the normalizer cannot remove is rejected with a JSON-pointer error, and the batch writes nothing. (REQ-001)
- [ ] ac-3: `LexemesByRoot` returns every lexeme with that root, in a stable order, and `RootCounts` counts them.

## Deliverables

- [ ] `internal/lexicon/prepare.go` (fields, validation, normalization)
- [ ] `internal/db` upsert and read changes, `LexemesByRoot`, `RootCounts`
- [ ] Tests: `internal/lexicon/prepare_test.go` (table test for roots and binyanim), `internal/db` upsert and root tests

## Technical Requirements

- [ ] No migration: the columns exist.
- [ ] The rebuild of `lesson_items` after an upsert is unchanged.

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** none. **Rollback:** revert; stored roots remain and are harmless.

## Security

- **Sensitivity:** standard — admin write path. Input is length-capped (root ≤ 16 runes) and character-checked before storage.

## Verification Strategy

- **Claim:** roots and binyanim can be written, normalized, kept, cleared and read.
- **Check + executor:** mechanized — `go test ./internal/lexicon/... ./internal/db/...`.
- **Pass condition:** the named table tests for ac-1 to ac-3 pass.

## Integration Points

- SPEC-003 exposes the fields over MCP. INIT-018/SPEC-001 reads `LexemesByRoot`.
- **Provenance tag:** `INIT-016/SPEC-002`
