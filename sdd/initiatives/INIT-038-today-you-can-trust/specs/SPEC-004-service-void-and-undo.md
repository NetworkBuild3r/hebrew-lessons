# SPEC-004 · service: void with a true undo, and the one-time void command

## Metadata

```yaml
spec_id: SPEC-004
initiative_id: INIT-038
title: "Void with a true undo, and the one-time void command"
domain: service
status: complete
primary_prompt: none
complexity: high
complexity_reason: the replay must reproduce every scheduler field exactly, and the command writes to production evidence
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: full
estimated_effort: 6 hours
blocked_by: [SPEC-002]
blocks: [SPEC-009, SPEC-014]
```

## Description

`review_log` keeps each answer's rating and time but not the card's state before it (`migrations/001_init.sql:107-115`). So the undo is a replay.

- **Void.** `internal/evidence` gains a void that:
  - writes SPEC-002's void rows;
  - voids the answer's confusion row;
  - replays the card from its remaining `review_log` rows through `internal/fsrs`.

  Fuzz is off (`internal/fsrs/review.go:30`), so the replay is deterministic. The card's state, stability, difficulty, due date, reps, and lapses come out as they would have been without the voided answer.
- **Undo.** Undoing a batch marks it undone and replays the affected cards again.
- **Unit void.** Voids one learner's wrong answers in one unit, with reason `report`, for SPEC-009's Report a problem.
- **`hebrew void-evidence`** in `cmd/hebrew`, modeled on `cmd/hebrew/backfill_spell.go`:
  - It dry-runs by default and prints counts by word and skill, never answer text.
  - `--apply` works only with `--backup-taken`, after `hebrew backup --to`.
  - `--undo <batch>` reverses a batch.
  - It selects every answer from learner 1's units at positions 12 to 34 (reason `unit_12_34`), and every answer from a lesson word-choice blank whose step had no English cue (reason `no_cue_blank`).
- **Matching blanks to steps.** Attempts store `unit_id`, `activity`, and `prompt`, but no step index, so the command matches each lesson cloze answer to its step in the unit revision. If a match is ambiguous, it falls back to voiding every wrong lesson cloze answer, and the dry run says which method it used and how many answers each covered.

**What Brian will see differently:** nothing on its own. It is the engine behind the "also right" tap and Report a problem (SPEC-009) and the cleanup in SPEC-014.

**Removes:** nothing. It is the only way evidence is taken back, so no one edits answer rows by hand.

**Scope (out):** the tap and the report button (SPEC-009); running the command on production (SPEC-014).

## Acceptance criteria

- [ ] After a void, the card's state, stability, difficulty, due date, reps, and lapses equal their values before that answer, on fixtures with the voided answer first, in the middle, and last.
- [ ] The voided answer's confusion row no longer counts.
- [ ] A void followed by its undo restores the card exactly.
- [ ] A unit void covers only that learner's wrong answers in that unit.
- [ ] The dry run on a production copy prints counts by word, skill, and reason, names the blank-matching method, and writes nothing.
- [ ] `--apply` without `--backup-taken` refuses.

## Verification Strategy

- **Claim:** a void takes an answer back exactly, and the command is safe to run.
- **Check + executor:** mechanized. `go test ./internal/evidence/... ./cmd/hebrew/...` with replay fixtures, then the dry run on a production copy through the local Docker check.
- **Pass condition:** every field matches in every fixture, and the dry run leaves the copy's checksums unchanged.
