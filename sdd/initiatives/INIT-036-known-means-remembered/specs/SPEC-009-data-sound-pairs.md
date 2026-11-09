# SPEC-009 · data: the sound-pair bank

## Metadata

```yaml
spec_id: SPEC-009
initiative_id: INIT-036
title: "The sound-pair bank"
domain: data
status: planned
primary_prompt: none
complexity: low
complexity_reason: one small table and a store, patterned on existing lexicon tables
model: inherit
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: revertible
security_review: light
estimated_effort: 3 hours
blocked_by: [SPEC-008]
blocks: [SPEC-010, SPEC-011]
```

## Description

Add migration 023, `sound_pairs`:

- `id`, `contrast` (the slug from `docs/sound-contrasts.md`), `lexeme_a_id`, `lexeme_b_id` (both reference `lexemes`), `approved` (0 or 1), `created_at`.
- A pair is stored once whichever order it is given in, so `(a, b)` and `(b, a)` are one row. `lexeme_a_id` and `lexeme_b_id` differ.
- `internal/db` gets `UpsertSoundPair`, `ListSoundPairs(contrast, approvedOnly)` and `PairsForLexeme`.

The table is shared reference data, like the lexicon. It holds nothing about a learner. A learner's answers go to the existing drill tables in SPEC-011.

**Scope (out):** the MCP tool (SPEC-010), the drill (SPEC-011).

## Acceptance criteria

- [ ] Migration 023 applies cleanly on top of 022 and the migration test counts it.
- [ ] Upserting `(a, b)` then `(b, a)` leaves one row.
- [ ] A pair of a word with itself, or with a missing lexeme, is rejected.
- [ ] `ListSoundPairs` with `approvedOnly` omits unapproved rows.
- [ ] The table has no learner column.

## Gates & Controls

- **Gate actions triggered:** `db_migration`. Rollback: the migration is additive; drop `sound_pairs`.

## Verification Strategy

- **Claim:** pairs are stored once, valid, and filtered by approval.
- **Check + executor:** mechanized. `go test ./internal/db/...`, including the migration test.
- **Pass condition:** all acceptance cases pass on a fresh database and on one migrated from 022.
