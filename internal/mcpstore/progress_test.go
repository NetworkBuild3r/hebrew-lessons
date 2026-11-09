package mcpstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// TestUnitProgressMapsStates covers mcpstore.UnitProgress for get_context.
// INIT-030/SPEC-005.
func TestUnitProgressMapsStates(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	raw, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock{now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	user, err := raw.InsertUser(ctx, "p@example.com", "sub-p", "learner")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := raw.DB()
	if err != nil {
		t.Fatal(err)
	}
	res, err := conn.ExecContext(ctx, `
		INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (0, 't', 's', 'c', 'published', 1, '{"lessons":[{},{}]}', 'shared')`)
	if err != nil {
		t.Fatal(err)
	}
	unitID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 0, ?)`,
		user, unitID, db.FormatTime(now)); err != nil {
		t.Fatal(err)
	}

	store := New(raw)
	got, err := store.UnitProgress(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("progress = %+v", got)
	}
	row := got[0]
	if row.UnitID != unitID || row.State != db.UnitStarted || row.LessonsFinished != 0 || row.LessonsTotal != 2 {
		t.Fatalf("row %+v", row)
	}
	_ = mcp.UnitProgressRow{}
}
