package db

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

func TestLessonRunsOneOpen(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	userID := insertUser(t, s, "a@example.com", "sub-a", "learner")
	unitID := insertUnit(t, s, "published")
	mustExec(t, s, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 0, ?)`, userID, unitID, ts)
	if _, err := s.exec(ctx, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 0, ?)`, userID, unitID, ts); !isUniqueConstraint(err) {
		t.Fatalf("second open run = %v, want unique constraint", err)
	}
	mustExec(t, s, `UPDATE lesson_runs SET finished_at = ? WHERE user_id = ?`, ts, userID)
	mustExec(t, s, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 0, ?)`, userID, unitID, ts)
}

func TestConcurrentLessonStartOneRun(t *testing.T) {
	dir := t.TempDir()
	s1 := openAt(t, dir, nil)
	s2, err := Open(context.Background(), filepath.Join(dir, "hebrew.sqlite"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })

	ctx := context.Background()
	userID := insertUser(t, s1, "a@example.com", "sub-a", "learner")
	unitID := insertUnit(t, s1, "published")

	var wg sync.WaitGroup
	type got struct {
		id  int64
		err error
	}
	out := make([]got, 2)
	stores := []*Store{s1, s2}
	wg.Add(2)
	for i := range stores {
		go func() {
			defer wg.Done()
			run, err := stores[i].StartPlayerRun(ctx, userID, unitID, 0)
			out[i] = got{id: run.ID, err: err}
		}()
	}
	wg.Wait()
	for i, item := range out {
		if item.err != nil || item.id == 0 {
			t.Fatalf("start %d = %d, %v", i, item.id, item.err)
		}
	}
	if out[0].id != out[1].id {
		t.Fatalf("concurrent starts created %d and %d", out[0].id, out[1].id)
	}
	var n int
	if err := s1.queryRow(ctx, `SELECT COUNT(*) FROM lesson_runs WHERE user_id = ? AND finished_at IS NULL`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("open runs = %d, want 1", n)
	}
}
