# SPEC-001 completion summary

**Initiative:** INIT-021 · **Spec:** SPEC-001 (data) · **Status:** complete · **Date:** 2026-10-24

## Change

Migration 017 replaces the navigation placeholder `verse_notes` with a note per verse and layer, and adds `verse_word_glosses`. A note for a verse that is not in `tanakh_verses` is refused. Writing a checked note again stores it as a draft. `checkedOnly` reads leave drafts out.

## Acceptance criteria

| AC | Result | Where to verify |
| --- | --- | --- |
| ac-1 | PASS | `TestVerseNoteRefusesAMissingVerse`: a note for a missing verse writes nothing. The migration applies with the rest of the chain in `go test ./internal/db/`. |
| ac-2 | PASS | `TestVerseNotesStayDraftUntilChecked`: rewriting a checked note's body sets it back to draft and clears `checked_by`. |
| ac-3 | PASS | The same test, and `TestVerseGlossCheckedOnly`: `checkedOnly` returns only checked rows. |

## Deviations and disclosures

- No personas, no timer, and no trace. This repo has no `.claude/agents/` and no `scripts/dev/`, so the spec ran inline and no trace was emitted.
- No security-review registry is available. The human gate is the `status` column. A write from the store is always `draft`.
- The placeholder table from migration 014 had no verse to keep. Those rows are dropped. Production had none: Isaiah was still hidden.
- No production backup was taken before this migration. It is additive beside every other table. There is no down migration.
