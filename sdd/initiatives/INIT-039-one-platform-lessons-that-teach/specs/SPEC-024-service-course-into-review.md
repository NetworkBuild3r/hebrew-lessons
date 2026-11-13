# SPEC-024 · service: course words go into the lexicon and into review

## Metadata

```yaml
spec_id: SPEC-024
initiative_id: INIT-039
title: "Course words into the lexicon and into review"
domain: service
status: complete
primary_prompt: none
complexity: medium
complexity_reason: one additive migration, an insert-only sync at startup, and a new first source for review's new words
model: inherit
autonomy_level: auto_tests
gate_actions: []
reversibility_class: revertible
security_review: required
estimated_effort: 5 hours
blocked_by: [SPEC-022]
blocks: [SPEC-025, SPEC-026, SPEC-027]
```

## Description

Review already adds new words in `seedNewFromDeck` (`internal/session/supply.go`). It runs on a full build, which is what the review queue in `internal/web/learn.go` starts through `BuildSessionWithSeed`. When recent accuracy is at least `TargetSuccess` (0.85, and also with no history), it tops up due new cards to the accuracy tier: 5 from 0.80 through 0.90, and 8 above 0.90. It picks from the placement deck. This spec puts the ready course words into the shared lexicon and makes them the first source for those new words, in course order. The placement deck stays as the fallback. It also fills the frequency rank that new-card ordering already reads (`Candidate.HasFreq` and `FreqRank` in `internal/session/mix.go`). Nothing fills those fields today.

This spec works without SPEC-023. With no signed rows, `CourseReady()` is empty and nothing changes. The tests use their own rows.

## What to build

### 1. Migration `migrations/026_course_words.sql`

It is additive. Follow the comment style of `025_story_slots.sql`.

```sql
-- Words of the common-word course (INIT-039/SPEC-024). Additive. Shared content, not learner data.
-- There is no down migration. To roll back, restore the pre-migration SQLite file.
CREATE TABLE course_words (
    lexeme_id INTEGER PRIMARY KEY REFERENCES lexemes (id) ON DELETE RESTRICT, -- The lexicon row this course word uses.
    course_order INTEGER NOT NULL CHECK (course_order >= 1), -- Position in internal/speech/course.tsv.
    list_rank INTEGER NOT NULL CHECK (list_rank >= 1), -- Rank of the source lemma in the frequency list.
    lemma TEXT NOT NULL, -- The source lemma, unpointed, as the list spells it.
    linked INTEGER NOT NULL DEFAULT 0 CHECK (linked IN (0, 1)), -- 1 when the course reused a lexeme row that already existed.
    added_at TEXT NOT NULL -- UTC RFC3339 instant the course first used this row.
);
CREATE INDEX course_words_order ON course_words (course_order);
```

Add `internal/db/migrate_026_test.go` in the style of `migrate_025` or `migrate_024`. It checks that the table exists after migrate, and that an existing database with lexemes and cards migrates with every row kept.

### 2. `internal/db/course.go`

- `type CourseRow struct { Order, Rank int; Lemma, Pointed, Translit, Gloss, POS, Gender, Root, Binyan string }`
- `type CourseSync struct { Added, Linked, Skipped, Moved int }`
- `func (s *Store) SyncCourse(ctx context.Context, rows []CourseRow) (CourseSync, error)` runs in one transaction:
  1. Load every lexeme's id and consonant spelling: `bare` when it is not empty, else `units.StripNiqqud(text_nfc)`. Build `map[spelling][]id`.
  2. For each row, the spelling is `units.StripNiqqud(units.Normalize(Pointed))`.
     - **No stored row has that spelling.** Validate the row with `lexicon.Prepare`, then insert it with the same INSERT as `insertLexemeRow`. Set `verified` to 0 and fill translit. Add it to the map, then add a `course_words` row with `linked = 0`. Count it as Added.
     - **Exactly one stored row has it.** Add a `course_words` row for that id with `linked = 1`, unless one exists. Count it as Linked. Do not change the lexeme row.
     - **Two or more rows have it.** Count it as Skipped and log the pointed form at debug level.
     - **A `course_words` row already exists for the id.** Update only `course_order`, `list_rank`, and `lemma` when they differ, and count Moved.
  3. Never call `UpsertLexemes`. Never UPDATE `lexemes`. Never delete a row. Do not rebuild `lesson_items`.
- `type CourseLexeme struct { LexemeID int64; Order, Rank int }`
- `func (s *Store) CourseLexemes(ctx context.Context) ([]CourseLexeme, error)`, ordered by `course_order`, then `lexeme_id`.

### 3. `internal/content/course.go`

- `func EnsureCourse(ctx context.Context, store *db.Store) error`: read `speech.CourseReady()`, map each word to `db.CourseRow`, and call `SyncCourse`.
  - When Added, Linked, or Moved is above 0, call `store.AuditContent(ctx, "course", "sync_course", "shared")` once.
  - Log one `slog.Info("course", "added", …, "linked", …, "skipped", …, "moved", …)`.
- Call it in `internal/app/app.go` right after `content.Ensure(ctx, store)`, with the same error handling (close the store and return).

### 4. Review takes course words first

In `internal/session/supply.go`:

- Add `func (s *Service) pickCourseLexemes(ctx context.Context, userID int64, need int, haveCard map[int64]struct{}) ([]int64, error)`. It walks `CourseLexemes` in order and skips:
  - a lexeme in `haveCard` (this also skips a flagged word, because a flag suspends its cards and the card still exists);
  - a lexeme whose `LexemeByID` gloss is empty;
  - a lexeme that is a `new` word of a published unit this learner has not finished. Review hides such a card (`introNotFinished` in `internal/db/learning.go`), so seeding it would make a card nobody sees. Add `func (s *Store) LexemesBehindUnfinishedIntro(ctx context.Context, userID int64) (map[int64]bool, error)` to `internal/db/course.go`, using the same conditions as `introNotFinished`, with a test.

  It stops at `need`.
- In `seedNewFromDeck`, after `need` is known, load `haveCard` once and take course words first. Only when fewer than `need` were found, call `pickSeedLexemes` for the rest, excluding ids already picked. Change `pickSeedLexemes` to accept the already-loaded `haveCard` and an exclude set, instead of loading `haveCard` again.
- Keep the accuracy gate, the tier, and the skills function exactly as they are.

In `internal/session/session.go`, where candidates are built (near `HasIntro: card.HasIntro`): load `CourseLexemes` once per build into `map[lexemeID]rank`, and set `HasFreq: true, FreqRank: rank` for a card whose lexeme is in it. Update the sentence in `internal/session/doc.go` that says "The lexicon does not store a rank, so ListStudyCards leaves that key unset". It should now say the rank comes from the course list for course words.

## Tests

The `internal/db` tests use a temp database:

- `TestSyncCourseAddsOnceAndKeepsStoredWords`.
  - Seed one lexeme `בַּיִת` with gloss `home` and translit.
  - Sync three rows: `בַּיִת` "house", `אִם` "if", and `עִם` "with".
  - The result is Added 2, Linked 1, and the stored `בַּיִת` gloss is still `home`.
  - A second sync gives Added 0, Linked 0, Moved 0.
- `TestSyncCourseSkipsAmbiguousSpelling`. Two stored lexemes share a consonant spelling, and the course row is Skipped.
- `TestSyncCourseMovesOrder`. A second sync with a new order updates `course_order` and counts Moved 1.

The `internal/session` tests use the real temp DB helpers in `supply_test.go`, such as `seedAccuracy`:

- `TestSeedTakesCourseWordsInOrder`. Ten course words and six deck words. At accuracy 0.95, a build introduces the first 8 course words in course order and no deck word.
- `TestSeedTierFollowsAccuracy`. At 0.87 it introduces 5. At 0.80 it introduces 0. With no history it introduces 5.
- `TestSeedFallsBackToDeck`. Two course words and accuracy 0.95: both course words plus 6 deck words.
- `TestSeedSkipsSuspendedCourseWord`. A course word with a suspended card is not introduced again, and the next word is.
- `TestSeedSkipsWordBehindUnfinishedIntro`. A course word that is a `new` item of a published, unfinished unit is skipped, and the next word is introduced.
- `TestNewCardsOrderByCourseRank`. Three new course cards with ranks 30, 5, and 12 are served in the order 5, 12, 30.

Run the existing packages too: `go test ./internal/db/ ./internal/session/ ./internal/content/ ./internal/app/`.

## Acceptance criteria

- [ ] Migration 026 exists and its test passes. Every earlier migration test still passes.
- [ ] A stored lexeme is never changed by the sync, and the test proves it.
- [ ] Review picks course words first, in order, at the existing tiers. It falls back to the deck.
- [ ] New course cards are ordered by list rank.
- [ ] The app starts locally with `docker run … go run ./cmd/hebrew` against a copy of `data\local-dev` (never the original), and logs one `course` line.

## Verification Strategy

- **Claim:** course words enter review in order, at the accuracy tiers, and nothing stored is changed.
- **Check + executor:** mechanized. The tests above in Docker `golang:1.26`.
- **Pass condition:** all of them pass. `go vet ./...` is clean.

## Security note

The sync writes shared content at startup from an embedded file. No request reaches it. The review in SPEC-016 must still cover it, because it writes to the lexicon on every start.
