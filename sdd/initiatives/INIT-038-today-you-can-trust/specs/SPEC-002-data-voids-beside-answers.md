# SPEC-002 · data: voids beside answers, and the study-day span

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-038
title: "Voids beside answers, and the study-day span"
domain: data
status: in_review
primary_prompt: none
complexity: medium
complexity_reason: an additive table, plus a filter in every evidence read, where a missed reader would keep counting a voided answer
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: light
estimated_effort: 4 hours
blocked_by: [SPEC-001]
blocks: [SPEC-004, SPEC-005]
```

## Description

- **The table.** `migrations/023_answer_voids.sql`, additive. One table, `answer_voids`, with:
  - the voided attempt;
  - the `review_log` row and the confusion row it covers, when there is one;
  - the learner and the reason (`unit_12_34`, `no_cue_blank`, `also_right`, `report`);
  - the actor, a batch id, the time, and `undone_at`.

  No existing row is edited or removed. A separate table avoids rebuilding `attempts`, whose `source` column is a fixed CHECK list (`migrations/015_attempt_source_self.sql:12`).
- **Store functions.** Insert a batch of voids, mark a batch undone, and list voids by learner or batch. There is no scheduling logic here; SPEC-004 owns the replay.
- **Evidence reads skip voided answers.** A void whose `undone_at` is set counts again. Every learner-facing reader is covered:
  - the evidence reader that sets `SuccessDays` (`internal/db/learner.go`, about line 421);
  - `internal/db/evidence.go`;
  - the later-day queries and the confusion queries.

  An include-voided option serves audits and SPEC-010's one-time comparison.
- **The study-day span.** The same reader returns the first and last checked-success study day per word and skill, which SPEC-005's "spanning 7 days" needs.

**What Brian will see differently:** nothing on its own. It lets a wrong grade be taken back without deleting a row, which SPEC-004, SPEC-009, and SPEC-014 build on.

**Removes:** nothing from the schema; it is additive by rule. It removes the readers' assumption that every attempt counts.

**Scope (out):** the replay and the void command (SPEC-004); the ladder (SPEC-005).

## Acceptance criteria

- [ ] With no void rows, every reader returns exactly what it returns today, by a fixture comparison.
- [ ] A voided fixture answer is invisible to every learner-facing reader, and visible with the include-voided option.
- [ ] A void marked undone counts again.
- [ ] The reader returns the first and last checked-success study day per word and skill on a fixture with answers on three days.
- [ ] Migration 023 applies to a copy of the production database, and the row counts and checksums of `attempts`, `review_log`, and `confusion_misses` are unchanged.

## Verification Strategy

- **Claim:** a void hides an answer from every reader without touching the answer, and nothing changes while there are no voids.
- **Check + executor:** mechanized. `go test ./internal/db/...` with a fixture learner, and the migration run on a production copy by the local Docker check.
- **Pass condition:** every acceptance case passes, and the checksums match before and after the migration.
