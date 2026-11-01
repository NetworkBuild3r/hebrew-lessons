# SPEC-005 · service: Taught words schedule cards; corrections carry their day

## Metadata

```yaml
spec_id: SPEC-005
initiative_id: INIT-026
title: "Taught words schedule cards; corrections carry their day"
domain: service
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: required
estimated_effort: 4 hours
blocked_by: [SPEC-001, SPEC-003]
blocks: [SPEC-008]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- The evidence package is the only card writer (`internal/evidence/doc.go`). `Expose` records words shown with no question (`internal/evidence/recorder.go:319`).
- A finished lesson inserts `new` read and recall cards, plus listen when the word has a clip, with `INSERT OR IGNORE` and due now (`internal/db/lesson.go:675`).
- `RecordMistakes` merges a repeat of the same pair into `times` and stores server instants only (`internal/db/mistakes.go:75`).

**Scope (in):**
- `Recorder.Taught(ctx, learnerID, lexemeIDs, on string, z studyday.Zone) (TaughtResult, error)`, in one transaction:
  - **Checks:** `on` is a valid `YYYY-MM-DD`. It is not after `studyday.Today(now, z)` and not more than 30 days before it. The list holds 1 to 50 ids, each positive. Duplicates count once.
  - **Rows:** inserts a `taught_words` row per word. If a row already exists for the same learner, word, and day, the result says `already` and nothing changes.
  - **Cards:** inserts `new` cards for read and recall, plus listen when the lexeme has a stored clip (the same fact `lookup` reports as `hasAudio`), with `INSERT OR IGNORE`. `due` is `studyday.DueCutoff(now, z)`. The row's `cards_created` lists the skills actually inserted.
  - **Never:** writes `attempts` or `review_log`, or changes an existing card.
  - **Result:** per word, `created` skills, `hadCard` skills, and the `due` instant.
- `Recorder.RemoveTaught(ctx, learnerID, on string, lexemeIDs)`, in one transaction:
  - Deletes the matching `taught_words` rows.
  - Deletes only the cards named in each row's `cards_created` that still have `reps = 0` and no `review_log` row.
  - Reports per word which cards were `removed` and which were `kept` because they had been reviewed.
- `db.MistakeInput` gains `On string`. A new pair stores `first_on = last_on = On`. A repeat sets `first_on` to the earlier day and `last_on` to the later one. An old row with null days takes `On` for both. `On` is checked the same way as `Taught`. The caller supplies `On`, defaulting to the learner's today in SPEC-008.
- `internal/evidence/doc.go`: one paragraph. Outside teaching schedules `new` cards through `Taught` and never grades. ADR-0022.

**Scope (out):** MCP tools (SPEC-008). Resolving Hebrew text to a lexeme (SPEC-008). Undoing corrections.

## Acceptance Criteria

- [ ] ac-1: `Taught` for three words with no cards, at 2026-11-01T17:00:00Z, Denver, `on = 2026-11-01`:
  - Inserts read and recall cards for each, in state `new`, due 2026-11-02T04:00 Denver (11:00Z).
  - Inserts listen only for the word that has a clip.
  - Writes no `attempts` or `review_log` row.
- [ ] ac-2: `Taught` for a word that already has a read card in review leaves that card byte-for-byte unchanged and reports `hadCard: [read]`.
- [ ] ac-3: `on` the next day, `on` 31 days back, 51 ids, or a zero id each return an error and write nothing.
- [ ] ac-4: `RemoveTaught` after one of the new recall cards was reviewed removes the other cards that row created, keeps the reviewed one, and reports it as `kept`.
- [ ] ac-5: `RemoveTaught` never deletes a card the taught row did not create, for example a card a lesson created earlier the same day.
- [ ] ac-6: Recording the same pair on `2026-10-28` and then `2026-10-25` gives `first_on = 2026-10-25`, `last_on = 2026-10-28`, and `times = 2`.
- [ ] ac-7: Learner A's `Taught` and `RemoveTaught` never touch learner B's cards or rows.

## Deliverables

- [ ] `internal/evidence/taught.go` and test
- [ ] `internal/evidence/doc.go`
- [ ] `internal/db/taught.go` (transactional store calls) and test
- [ ] `internal/db/mistakes.go` and test

## Verification Strategy

- **Claim:**
  - Outside teaching creates only `new` cards due at the learner's next study-day cutoff.
  - Undo removes exactly what that teaching created and nothing reviewed.
  - Corrections keep their learner day.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 -run 'TestTaught|TestRemoveTaught' ./internal/evidence ./internal/db` and `/tmp/go/bin/go test -count=1 -run 'TestRecordMistakes' ./internal/db`. Store-backed tests with the fixed clock, as the evidence package's tests are today.
- **Pass condition:** ac-1 to ac-7 pass. Planting an `UPDATE cards` in `Taught` fails ac-2. Dropping the `reps = 0` guard fails ac-4.

## Integration Points

- **Provenance tag:** `INIT-026/SPEC-005`
- Uses `studyday.Today` and `studyday.DueCutoff` from SPEC-003 and `taught_words` from SPEC-002.
