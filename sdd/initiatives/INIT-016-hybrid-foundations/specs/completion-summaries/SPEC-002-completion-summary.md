# SPEC-002 completion summary

**Initiative:** INIT-016 · **Spec:** SPEC-002 (data) · **Status:** complete · **Date:** 2026-10-23

## Change

`lexicon.Input` and `lexicon.Row` gain `Root` and `Binyan`, with the nil-keeps-stored rule. A root is stored as 2–4 consonants joined by a maqaf, shin and sin dots kept. A binyan is one of the seven patterns or empty. `UpsertLexemes` writes both. `LexemeView` and the lookup reads return them. `Store.LexemesByRoot` and `Store.RootCounts` are new. No migration: the columns already existed.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestUpsertLexemesRootAndBinyan`: stores normalized, an omitted field keeps the value, an empty string clears it. |
| ac-2 | PASS | `TestPrepareRootAndBinyan`: a non-Hebrew letter, one letter, five letters, and an unknown binyan are rejected with a JSON pointer, and the batch writes nothing. |
| ac-3 | PASS | `LexemesByRoot` returns the matching lexeme in pointed-text order, and `RootCounts` counts it. |

## Deviations and disclosures

- No personas and no trace, as in SPEC-001.
- The MCP lookup already selected `l.root`. This spec adds `binyan` beside it and the write path.
