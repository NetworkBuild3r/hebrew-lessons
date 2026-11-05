package db

import (
	"context"
	"fmt"
	"math"
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

func finishLessonScore(t *testing.T, s *Store, userID, unitID int64, lessonIx int, at time.Time, score float64) error {
	t.Helper()
	_, err := s.exec(context.Background(), `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
		VALUES (?, ?, ?, ?, ?, ?)`, userID, unitID, lessonIx, FormatTime(at), FormatTime(at), score)
	return err
}

func startLesson(t *testing.T, s *Store, userID, unitID int64, lessonIx int, at time.Time) error {
	t.Helper()
	_, err := s.exec(context.Background(), `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at)
		VALUES (?, ?, ?, ?)`, userID, unitID, lessonIx, FormatTime(at))
	return err
}

// TestUnitProgressStates covers untouched, started, and finished. INIT-030/SPEC-002.
func TestUnitProgressStates(t *testing.T) {
	when := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "up-states@example.com", "sub-up-states", "learner")

	untouched := insertScopedUnit(t, s, 1, "shared", `{"lessons":[{},{}]}`)
	started := insertScopedUnit(t, s, 2, "shared", `{"lessons":[{},{}]}`)
	finished := insertScopedUnit(t, s, 3, "shared", `{"lessons":[{},{}]}`)
	zero := insertScopedUnit(t, s, 4, "shared", `{"title":"empty"}`)
	for _, id := range []int64{untouched, started, finished, zero} {
		if _, err := s.exec(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, id); err != nil {
			t.Fatal(err)
		}
	}

	if err := startLesson(t, s, userID, started, 0, when); err != nil {
		t.Fatal(err)
	}
	if err := finishLessonScore(t, s, userID, finished, 0, when, 1); err != nil {
		t.Fatal(err)
	}
	if err := finishLessonScore(t, s, userID, finished, 1, when.Add(time.Hour), 0.5); err != nil {
		t.Fatal(err)
	}

	got, err := s.UnitProgress(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int64]UnitProgress{}
	for _, p := range got {
		byID[p.UnitID] = p
	}
	if p := byID[untouched]; p.State != UnitUntouched || p.LessonsFinished != 0 || p.LessonsTotal != 2 {
		t.Fatalf("untouched = %+v", p)
	}
	if p := byID[started]; p.State != UnitStarted || p.LessonsFinished != 0 || p.LessonsTotal != 2 {
		t.Fatalf("started = %+v", p)
	}
	if p := byID[finished]; p.State != UnitFinished || p.LessonsFinished != 2 || p.LessonsTotal != 2 {
		t.Fatalf("finished = %+v", p)
	}
	if p := byID[zero]; p.State != UnitUntouched || p.LessonsTotal != 0 || p.State == UnitFinished {
		t.Fatalf("zero-lesson unit = %+v", p)
	}
}

// TestUnitProgressCountsAndMean checks lesson counts, last finish, and mean score. INIT-030/SPEC-002.
func TestUnitProgressCountsAndMean(t *testing.T) {
	when := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "up-mean@example.com", "sub-up-mean", "learner")

	unit := insertScopedUnit(t, s, 1, "shared", `{"lessons":[{},{},{}]}`)
	if _, err := s.exec(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, unit); err != nil {
		t.Fatal(err)
	}
	t0 := when
	t1 := when.Add(time.Hour)
	if err := finishLessonScore(t, s, userID, unit, 0, t0, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := finishLessonScore(t, s, userID, unit, 1, t1, 1.0); err != nil {
		t.Fatal(err)
	}
	// Lesson 2 unfinished: unit is started, mean over the two finished scores.

	got, err := s.UnitProgress(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("progress = %+v", got)
	}
	p := got[0]
	if p.UnitID != unit || p.State != UnitStarted || p.LessonsFinished != 2 || p.LessonsTotal != 3 {
		t.Fatalf("progress = %+v", p)
	}
	if !p.LastFinishedAt.Equal(t1) {
		t.Fatalf("LastFinishedAt = %v want %v", p.LastFinishedAt, t1)
	}
	wantMean := 0.75
	if math.Abs(p.MeanScore-wantMean) > 1e-9 {
		t.Fatalf("MeanScore = %v want %v", p.MeanScore, wantMean)
	}
}

// TestUnitProgressLearnerIsolation ensures another learner's runs do not change state. INIT-030/SPEC-002.
func TestUnitProgressLearnerIsolation(t *testing.T) {
	when := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "up-iso@example.com", "sub-up-iso", "learner")
	otherID := insertUser(t, s, "up-iso-other@example.com", "sub-up-iso-other", "learner")

	shared := insertScopedUnit(t, s, 1, "shared", `{"lessons":[{}]}`)
	if _, err := s.exec(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, shared); err != nil {
		t.Fatal(err)
	}
	if err := finishLessonScore(t, s, otherID, shared, 0, when, 1); err != nil {
		t.Fatal(err)
	}

	got, err := s.UnitProgress(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].State != UnitUntouched || got[0].LessonsFinished != 0 {
		t.Fatalf("isolation = %+v", got)
	}
	ok, err := s.IsUntouched(ctx, userID, shared)
	if err != nil || !ok {
		t.Fatalf("IsUntouched = %v, %v", ok, err)
	}
	ok, err = s.IsUntouched(ctx, otherID, shared)
	if err != nil || ok {
		t.Fatalf("other IsUntouched = %v, %v", ok, err)
	}
}

// TestIsUntouched covers the store check and the same check on a *sql.Tx. INIT-030/SPEC-002.
func TestIsUntouched(t *testing.T) {
	when := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "up-tx@example.com", "sub-up-tx", "learner")
	unit := insertScopedUnit(t, s, 1, "shared", `{"lessons":[{}]}`)

	ok, err := s.IsUntouched(ctx, userID, unit)
	if err != nil || !ok {
		t.Fatalf("before = %v, %v", ok, err)
	}

	conn, err := s.connOrErr()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	ok, err = IsUntouchedTx(ctx, tx, userID, unit)
	if err != nil || !ok {
		t.Fatalf("tx before = %v, %v", ok, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at)
		VALUES (?, ?, 0, ?)`, userID, unit, FormatTime(when)); err != nil {
		t.Fatal(err)
	}
	ok, err = IsUntouchedTx(ctx, tx, userID, unit)
	if err != nil || ok {
		t.Fatalf("tx after insert = %v, %v", ok, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	ok, err = s.IsUntouched(ctx, userID, unit)
	if err != nil || ok {
		t.Fatalf("after commit = %v, %v", ok, err)
	}
}
