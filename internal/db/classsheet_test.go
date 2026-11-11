package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lessons"
)

type sheetClock struct{ t time.Time }

func (c sheetClock) Now() time.Time { return c.t }

func TestSaveClassSheetTwiceChangesNothing(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), sheetClock{time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('sheet@example.com', 'sub-sheet', 'learner', '2026-11-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	var userID int64
	if err := conn.QueryRowContext(ctx, `SELECT id FROM users`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	in, err := lessons.ImportSheet(lessons.Sheet{
		Ref: "class-1", ClassOn: "2026-11-01", Kind: "class", Title: "Morning",
		Body: "tutor: שָׁלוֹם || peace\n",
	}, time.Time{})
	if err != nil || !in.Passed {
		t.Fatal(err, in.Note)
	}
	id, unchanged, err := store.SaveClassSheet(ctx, userID, in)
	if err != nil || unchanged || id == 0 {
		t.Fatalf("id %d unchanged %v err %v", id, unchanged, err)
	}
	again, unchanged, err := store.SaveClassSheet(ctx, userID, in)
	if err != nil || !unchanged || again != id {
		t.Fatalf("again %d unchanged %v err %v", again, unchanged, err)
	}
	sheets, err := store.ClassSheets(ctx, userID)
	if err != nil || len(sheets) != 1 || sheets[0].Status != "passed" || sheets[0].Lines[0].Tutor == "" {
		t.Fatalf("%+v %v", sheets, err)
	}
}
