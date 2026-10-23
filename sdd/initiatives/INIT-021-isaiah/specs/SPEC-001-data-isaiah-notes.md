# SPEC-001 · data: Isaiah verse notes (migration 015)

## Metadata

```yaml
spec_id: SPEC-001
initiative_id: INIT-021
title: "Isaiah verse notes"
domain: data
status: planned
primary_prompt: .claude/agents/database-architect/AGENT.md
supplement_prompts: []
model: sonnet
autonomy_level: auto_tests
gate_actions: [db_migration]
reversibility_class: remote
security_review: standard
estimated_effort: 3 hours
blocked_by: [INIT-016/SPEC-001]
blocks: [SPEC-002]
```

## Description

**Context:** `tanakh_verses` holds the pointed text and JPS 1917 English. Isaiah study layers four kinds of notes on each verse; they are drafted by an agent and checked by a person (D10).

**Scope (in):** `migrations/015_verse_notes.sql` and store methods:

- `verse_notes`: `book`, `chapter`, `verse` (referencing `tanakh_verses`), `layer` (`gloss` | `roots` | `manner` | `bridge`), `body` (restricted markdown from INIT-017/SPEC-002's grammar, ≤ 8 000 bytes), `status` (`draft` | `checked`), `drafted_by`, `checked_by` (user id, nullable), `updated_at`. Primary key (book, chapter, verse, layer).
- `verse_word_glosses`: (`book`, `chapter`, `verse`, `position`) → `word_he`, `gloss_en`, `root` (nullable), `lexeme_id` (nullable), `status`.
- Store: `UpsertVerseNotes`, `CheckVerseNote(…, checkerID)`, `VerseNotes(book, chapter, verse, checkedOnly)`, `UpsertVerseGlosses`, `VerseGlosses(…, checkedOnly)`.

**Scope (out):** Tools (SPEC-002), reader (SPEC-003).

## Acceptance Criteria

- [ ] ac-1: The migration applies to a copy of the current schema; a note for a verse not in `tanakh_verses` is refused. (REQ-013)
- [ ] ac-2: Changing a checked note's body sets it back to `draft`. (REQ-013)
- [ ] ac-3: `checkedOnly` reads return only checked rows. (REQ-013)

## Deliverables

- [ ] `migrations/015_verse_notes.sql`
- [ ] `internal/db/versenotes.go` and tests

## Gates & Controls

- **Autonomy level:** auto_tests. **Gate actions:** `db_migration` — additive; back up first. **Rollback:** revert the app.

## Security

- **Sensitivity:** standard — public content with a human gate encoded in the schema.

## Verification Strategy

- **Claim:** notes are stored per verse and layer, and only checked notes are readable as checked.
- **Check + executor:** mechanized — `go test ./internal/db/...`.
- **Pass condition:** ac-1 to ac-3 tests pass.

## Integration Points

- SPEC-002 writes; SPEC-003 reads.
- **Provenance tag:** `INIT-021/SPEC-001`
