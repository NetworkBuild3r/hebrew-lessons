package db

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestScopeUnits(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "scope@example.com", "sub-scope", "learner")
	otherID := insertUser(t, s, "scope-other@example.com", "sub-scope-other", "learner")

	shared := insertScopedUnit(t, s, 2, "shared", `{"title":"t","lessons":[{},{}]}`)
	if _, err := s.exec(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, shared); err != nil {
		t.Fatal(err)
	}
	private := insertScopedUnit(t, s, 0, fmt.Sprintf("learner:%d", userID), `{"lessons":[{}]}`)
	otherPrivate := insertScopedUnit(t, s, 1, fmt.Sprintf("learner:%d", otherID), `{"lessons":[{}]}`)
	empty := insertScopedUnit(t, s, 3, "shared", `{"title":"empty"}`)
	partial := insertScopedUnit(t, s, 4, "shared", `{"lessons":[{},{}]}`)
	_ = otherPrivate
	_ = empty

	later := when.Add(time.Hour)
	if err := finishLesson(t, s, userID, shared, 0, when); err != nil {
		t.Fatal(err)
	}
	if err := finishLesson(t, s, userID, shared, 1, later); err != nil {
		t.Fatal(err)
	}
	// An unfinished rerun does not erase the finish, and another learner does not complete it.
	if err := startLesson(t, s, userID, shared, 0, later); err != nil {
		t.Fatal(err)
	}
	if err := finishLesson(t, s, otherID, shared, 0, later); err != nil {
		t.Fatal(err)
	}
	if err := finishLesson(t, s, userID, otherPrivate, 0, when); err != nil {
		t.Fatal(err)
	}
	if err := finishLesson(t, s, userID, private, 0, when); err != nil {
		t.Fatal(err)
	}
	if err := finishLesson(t, s, userID, partial, 0, when); err != nil {
		t.Fatal(err)
	}

	done, err := s.CompletedSharedUnits(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 1 || done[0].UnitID != shared || done[0].Position != 2 || !done[0].CompletedAt.Equal(later) {
		t.Fatalf("completed = %+v", done)
	}

	rows, err := s.UnitsInScope(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("in scope = %+v", rows)
	}
	if rows[0].ID != private || rows[0].Scope != fmt.Sprintf("learner:%d", userID) || rows[0].Position != 0 {
		t.Fatalf("first = %+v", rows[0])
	}
	if rows[1].ID != shared || rows[1].Scope != "shared" || rows[1].Position != 2 {
		t.Fatalf("second = %+v", rows[1])
	}
	for _, row := range rows {
		if row.ID == otherPrivate {
			t.Fatalf("other learner's unit was included: %+v", rows)
		}
	}

	if _, err := s.exec(ctx, `UPDATE units SET scope = 'learner:' WHERE id = ?`, empty); err == nil {
		t.Fatal("scope learner: was accepted")
	}
	if _, err := s.exec(ctx, `UPDATE units SET scope = 'private' WHERE id = ?`, empty); err == nil {
		t.Fatal("scope private was accepted")
	}
	if _, err := s.exec(ctx, `UPDATE units SET scope = ? WHERE id = ?`, fmt.Sprintf("learner:%d", userID), empty); err != nil {
		t.Fatal(err)
	}
}

func insertScopedUnit(t *testing.T, s *Store, position int, scope, doc string) int64 {
	t.Helper()
	res, err := s.exec(context.Background(), `
		INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (?, 't', 's', 'c', 'draft', 1, ?, ?)`, position, doc, scope)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func finishLesson(t *testing.T, s *Store, userID, unitID int64, lessonIx int, at time.Time) error {
	t.Helper()
	_, err := s.exec(context.Background(), `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?)`, userID, unitID, lessonIx, FormatTime(at), FormatTime(at))
	return err
}

func startLesson(t *testing.T, s *Store, userID, unitID int64, lessonIx int, at time.Time) error {
	t.Helper()
	_, err := s.exec(context.Background(), `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at)
		VALUES (?, ?, ?, ?)`, userID, unitID, lessonIx, FormatTime(at))
	return err
}
