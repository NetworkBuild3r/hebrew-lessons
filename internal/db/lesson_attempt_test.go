package db

import (
	"context"
	"testing"
	"time"
)

// HasLessonAttempt is true for any lesson run, finished or not, any lesson
// completion, and any attempt whose source is not placement. A placement run
// or a placement attempt, finished or abandoned, is not a lesson.
func TestHasLessonAttempt(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	unit := insertUnit(t, s, "published")
	lex := insertLexeme(t, s, "שָׁלוֹם")

	t.Run("NoData", func(t *testing.T) {
		user := insertUser(t, s, "empty@example.com", "sub-empty", "learner")
		assertLessonAttempt(t, s, ctx, user, false)
	})

	t.Run("FinishedPlacement", func(t *testing.T) {
		user := insertUser(t, s, "placed@example.com", "sub-placed", "learner")
		mustExec(t, s, `INSERT INTO placement_runs (user_id, started_at, finished_at, level, result, seed, sitting_no)
			VALUES (?, ?, ?, 1, '{"bands":[]}', 1, 1)`, user, ts, ts)
		insertAttempt(t, s, user, lex, "placement")
		assertLessonAttempt(t, s, ctx, user, false)
	})

	t.Run("AbandonedPlacement", func(t *testing.T) {
		user := insertUser(t, s, "left@example.com", "sub-left", "learner")
		mustExec(t, s, `INSERT INTO placement_runs (user_id, started_at, seed, sitting_no)
			VALUES (?, ?, 2, 1)`, user, ts)
		insertAttempt(t, s, user, lex, "placement")
		assertLessonAttempt(t, s, ctx, user, false)
	})

	t.Run("UnfinishedLessonRun", func(t *testing.T) {
		user := insertUser(t, s, "open@example.com", "sub-open", "learner")
		mustExec(t, s, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 0, ?)`, user, unit, ts)
		assertLessonAttempt(t, s, ctx, user, true)
	})

	t.Run("LessonAttempt", func(t *testing.T) {
		user := insertUser(t, s, "tried@example.com", "sub-tried", "learner")
		insertAttempt(t, s, user, lex, "lesson")
		assertLessonAttempt(t, s, ctx, user, true)
	})

	t.Run("FinishedLesson", func(t *testing.T) {
		user := insertUser(t, s, "done@example.com", "sub-done", "learner")
		mustExec(t, s, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
			VALUES (?, ?, 0, ?, ?, 1)`, user, unit, ts, ts)
		assertLessonAttempt(t, s, ctx, user, true)
	})

	t.Run("OtherLearner", func(t *testing.T) {
		owner := insertUser(t, s, "owner@example.com", "sub-owner", "learner")
		bystander := insertUser(t, s, "by@example.com", "sub-by", "learner")
		mustExec(t, s, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
			VALUES (?, ?, 0, ?, ?, 1)`, owner, unit, ts, ts)
		insertAttempt(t, s, owner, lex, "lesson")
		assertLessonAttempt(t, s, ctx, bystander, false)
		assertLessonAttempt(t, s, ctx, owner, true)
	})
}

func assertLessonAttempt(t *testing.T, s *Store, ctx context.Context, userID int64, want bool) {
	t.Helper()
	got, err := s.HasLessonAttempt(ctx, userID)
	if err != nil || got != want {
		t.Fatalf("HasLessonAttempt(%d) = %v, %v; want %v", userID, got, err, want)
	}
}

func insertAttempt(t *testing.T, s *Store, userID, lexemeID int64, source string) {
	t.Helper()
	mustExec(t, s, `INSERT INTO attempts (
		user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at
	) VALUES (?, ?, 'read', 'pick', ?, 'p', 'r', 1, 0, 0, 10, ?)`, userID, lexemeID, source, ts)
}
