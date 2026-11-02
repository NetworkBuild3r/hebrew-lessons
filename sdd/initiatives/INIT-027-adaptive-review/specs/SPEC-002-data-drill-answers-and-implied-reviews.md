# SPEC-002 · data: Migration 021, drill answers, cleared pairs, implied reviews

## Metadata

```yaml
spec_id: SPEC-002
initiative_id: INIT-027
title: "Migration 021: drill answers, cleared pairs, implied reviews"
domain: data
status: complete
primary_prompt: none
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: local
security_review: light
estimated_effort: 2 hours
blocked_by: []
blocks: [SPEC-003, SPEC-004]
```

Expertise: `.claude/agents/` is absent. Nothing was manufactured. Execute inline.

## Description

**Context:**
- `confusion_misses` holds one row per miss (`kind`, `key`, `lexeme_id`, `at`). `confusionPairs` (`internal/db/struggle.go:199`) and `RecentConfusionPairs` (`internal/db/learner.go:600`, which the session drills read) count rows since a cutoff. Nothing records a drill answer.
- `review_log` (`migrations/001_init.sql:108`, later columns added by `ALTER TABLE`) has no way to mark a review that came from another skill's answer.
- Migrations are embedded with `//go:embed *.sql` (`migrations/embed.go`). Migration 020 is the latest. Companion tests pin the version list (`internal/db/db_test.go`, `inspect_test.go`, `learning_test.go`, `migrate_006/008/009/010/011_test.go`, `migrate_upgrade_test.go`, `cmd/hebrew/ops_test.go`). INIT-026/SPEC-002 bumped them 19→20; this bumps 20→21 the same way.

**Scope (in):**
- `migrations/021_drill_answers_and_implied.sql`, additive, no down migration. Header says so and how to roll back (restore the pre-migration file). Comment `INIT-027/SPEC-002`.
  - `drill_answers`: `id INTEGER PRIMARY KEY`, `learner_id` → users CASCADE, `kind TEXT NOT NULL CHECK (kind IN ('letter_pair','agreement'))`, `key TEXT NOT NULL CHECK (length(key) BETWEEN 1 AND 64)`, `correct INTEGER NOT NULL CHECK (correct IN (0,1))`, `latency_ms INTEGER NOT NULL DEFAULT 0 CHECK (latency_ms >= 0)`, `answered_at TEXT NOT NULL`. Index `(learner_id, kind, key, answered_at)`.
  - `confusion_clears`: `id INTEGER PRIMARY KEY`, `learner_id` → users CASCADE, `kind` (same check), `key` (same check), `cleared_at TEXT NOT NULL`. Index `(learner_id, kind, key, cleared_at)`. Many rows per pair are allowed. The latest counts.
  - `review_log.implied INTEGER NOT NULL DEFAULT 0 CHECK (implied IN (0,1))` by `ALTER TABLE ... ADD COLUMN`, the pattern migrations 002 and 003 use for `review_log`. Existing rows become 0.
- Store calls in `internal/db/drills.go`, each scoped by learner id:
  - `InsertDrillAnswer(ctx, DrillAnswer{LearnerID, Kind, Key, Correct, LatencyMS, At})`.
  - `DrillAnswersSince(ctx, learnerID, kind, key, since time.Time) ([]DrillAnswer, error)`, oldest first.
  - `CorrectDrillLatencies(ctx, learnerID, limit) ([]int64, error)`: newest timed right answers (latency > 0), at most `limit`.
  - `InsertConfusionClear(ctx, learnerID, kind, key, at)`.
  - `LatestConfusionClear(ctx, learnerID, kind, key) (time.Time, bool, error)`.
  - `LastConfusionMiss(ctx, learnerID, kind, key) (time.Time, bool, error)`.
- `confusionPairs` and `RecentConfusionPairs` count only misses after that pair's latest clear (a correlated `NOT EXISTS`/`MAX(cleared_at)` filter on the same learner, kind, and key). A pair with no clear counts as today.
- `review_log` writers keep `implied = 0` unless told otherwise: add `Implied bool` to `db.ReviewLog` (`internal/db/learning.go:57`) and write it in the three inserts (`learning.go:248`, `evidence.go:460`, `placement.go:292`), reading it back in `ReviewLogs`. Every query that counts ratings for accuracy (`RecentReviewRatings` at `internal/db/learning.go:542`, which the session mix reads, `CoachCount` / `Reviews7d` / `LastReviews` in `internal/db/coach.go`, retention in the progress and digest queries) adds `AND implied = 0`. List the queries changed in the completion summary.
- Bump the migration version lists 20→21. Extend the "nothing else changed" helpers the way `m20Aside` did, if a table dump compares `review_log`.

**Scope (out):** Any caller of the new store calls (SPEC-003, SPEC-004). Rewriting old `confusion_misses` rows.

## Acceptance Criteria

- [ ] ac-1: A database at 020 with rows in every table migrates to 021 with every row kept, `review_log.implied = 0` on old rows, and both new tables empty.
- [ ] ac-2: The checks refuse a bad `kind`, an empty or 65-character `key`, `correct = 2`, a negative latency, and `implied = 2`.
- [ ] ac-3: Deleting a user deletes their `drill_answers` and `confusion_clears` rows.
- [ ] ac-4: A fresh database and a migrated one have the same schema for the three changed tables.
- [ ] ac-5: With two misses of ב/כ, a clear, then one miss, `RecentConfusionPairs` reports a count of 1 for ב/כ. Without the clear it reports 3.
- [ ] ac-6: An implied review row is left out of every accuracy count listed in the summary, and a normal row is still counted.
- [ ] ac-7: Learner A's calls never return learner B's drill answers or clears.

## Deliverables

- [ ] `migrations/021_drill_answers_and_implied.sql`
- [ ] `internal/db/drills.go` and `internal/db/drills_test.go`
- [ ] `internal/db/migrate_021_test.go`
- [ ] `internal/db/struggle.go`, `internal/db/coach.go`, and the other accuracy queries
- [ ] the review_log writer and `db.ReviewLog`
- [ ] the version-list tests, including `cmd/hebrew/ops_test.go`

## Verification Strategy

- **Claim:** The schema can store drill answers, clears, and implied reviews without changing any existing row, and implied reviews never count as accuracy.
- **Check + executor:** mechanized. `/tmp/go/bin/go test -count=1 ./internal/db` and `/tmp/go/bin/go test -count=1 -run 'Migrat|Ops' ./cmd/hebrew`.
- **Pass condition:** ac-1 to ac-7 pass. Dropping `AND implied = 0` from one accuracy query fails ac-6. Dropping the clear filter fails ac-5.

## Integration Points

- **Provenance tag:** `INIT-027/SPEC-002`
- SPEC-003 writes drill answers and clears. SPEC-004 writes implied reviews.
